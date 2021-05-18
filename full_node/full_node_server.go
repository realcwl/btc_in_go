package full_node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/Luismorlan/btc_in_go/visualize"
	"github.com/jroimartin/gocui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// TODO(chenweilunster): Clean this up because this is too fking ugly.
const PEER_ALREADY_EXIST_ERR = "peer already exist"

type Peer struct {
	// A service client established to connect to other full node.
	client service.FullNodeServiceClient
	// Peer address
	addr Address
	// The connection for this peer. Each peer/client has a dedicated connection.
	conn *grpc.ClientConn
}

// Stringer function of peer.
func (p Peer) String() string {
	return p.addr.IpAddr + ":" + p.addr.Port
}

type Address struct {
	// What ip address peer fullnode is using.
	IpAddr string
	// What TCP Port peer full node is running on.
	Port string
}

// This server
type FullNodeServer struct {
	service.UnimplementedFullNodeServiceServer
	// A bunch of peers that we have grpc connection to.
	peers []Peer
	addr  Address

	// Create a mutex protect peers addition and deletion, as well as continous
	// block failure handlment accounting.
	m sync.RWMutex

	// FullNode manages all important states about blockchain.
	fullNode *FullNode

	// A counter counts how many failures we continously encountered for handle block.
	// A large number means very likely we're out of sync and need to sync the blockchain.
	// This number is incremented by handle failure, and reduced by successful handle.
	blockFailure int
	syncing      bool

	// A command channel to pass command to other part of the system.
	// For now, the only use is the interrupt mining process on tail change.
	cmd chan commands.Command
	// A command fancy place to put output.
	g *gocui.Gui
}

// Get self address.
func (sev *FullNodeServer) GetAddress() Address {
	return sev.addr
}

// Get public key in hex format.
func (sev *FullNodeServer) GetPublicKey() string {
	return sev.fullNode.GetPublicKey()
}

// Return all current peers.
func (sev *FullNodeServer) GetAllPeers() []Peer {
	sev.m.RLock()
	defer sev.m.RUnlock()
	return sev.peers
}

func (sev *FullNodeServer) GetPeer(idx int) (Peer, error) {
	sev.m.RLock()
	defer sev.m.RUnlock()
	if idx > len(sev.GetAllPeers()) {
		return Peer{}, fmt.Errorf("out of bound: %d", idx)
	}
	return sev.peers[idx], nil
}

// Set transaction should add transaction to pool and broad cast to peer.
func (sev *FullNodeServer) SetTransaction(con context.Context, req *service.SetTransactionRequest) (*service.SetTransactionResponse, error) {
	tx := req.GetTx()
	if tx == nil {
		return &service.SetTransactionResponse{}, nil
	}

	// First validate the transaction. This is totally optional but is a nice to have optimization.
	l := sev.fullNode.GetLedgerSnapshotAtDepth(0)
	err := utils.IsValidTransaction(tx, l)
	if err != nil {
		sev.Log("invalid incoming transaction: " + err.Error())
		return &service.SetTransactionResponse{}, nil
	}

	// Add the transaction to pool.
	err = sev.fullNode.AddTransactionToPool(tx)
	if err != nil {
		sev.Log("fail to add transaction to pool: " + err.Error())
		return &service.SetTransactionResponse{}, err
	}

	// Broadcast to all other nodes.
	sev.m.RLock()
	defer sev.m.RUnlock()
	for i := 0; i < len(sev.peers); i++ {
		peer := sev.peers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		peer.client.SetTransaction(ctx, &service.SetTransactionRequest{Tx: tx})
	}

	return &service.SetTransactionResponse{}, nil
}

// Return all utxo the public key owned.
func (sev *FullNodeServer) GetBalance(ctx context.Context, req *service.GetBalanceRequest) (*service.GetBalanceResponse, error) {
	pk := req.PublicKey
	l := sev.fullNode.GetUtxoForPublicKey(pk)
	sev.Log(fmt.Sprintf("read UTXO: %d", len(l.L)))
	res := service.GetBalanceResponse{}
	for utxoLite, output := range l.L {
		utxo := model.GetUtxo(&utxoLite)
		pair := service.UtxoOutputPair{
			Utxo:   &utxo,
			Output: output,
		}
		res.UtxoOutputPairs = append(res.UtxoOutputPairs, &pair)
	}
	sev.Log(fmt.Sprintf("returned UTXO size is: %d", len(res.GetUtxoOutputPairs())))
	return &res, nil
}

// Mine one block and set that block.
func (sev *FullNodeServer) Mine(ctl chan commands.Command) (commands.Command, error) {
	// We are mining a block at a new height.
	height := sev.fullNode.GetHeight() + 1

	b, c, err := sev.fullNode.CreateNewBlock(ctl, height)
	if err != nil {
		return c, err
	}
	// Not terminated by command nor mining failure, proceed to handle that block.
	// A tail change incurred by mining at local isn't cared.
	_, _, _, err = sev.SetBlockInternal(&service.SetBlockRequest{Block: b}, true /*broadcast=*/)
	if err == nil {
		sev.Log("successfully mined a new block: " + b.Hash)
	}
	return commands.NewDefaultCommand(), err
}

// Round-robin query peers to sync to the latest block.
func (sev *FullNodeServer) SyncToLatest() error {
	sev.Log("start syncing...")
	// boolean flag doesn't need mutex protection because it's a benign race.
	sev.syncing = true

	peerSize := len(sev.GetAllPeers())
	if peerSize == 0 {
		return errors.New("no peer to sync")
	}
	// TODO(chenweilunster): Make this configurable
	batch_size := 5
	tail := sev.fullNode.GetTail()
	i := 0
	for {
		// Peer size might change, we must get peer size every time.
		peerSize = len(sev.GetAllPeers())
		i = i % peerSize
		p, err := sev.GetPeer(i)
		if err != nil || p.conn.GetState() != connectivity.Ready {
			// Don't return error here. The peer might be just deleted or some other reason.
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := p.client.Sync(ctx, &service.SyncRequest{Hash: tail.B.Hash, Number: int64(batch_size)})
		if err != nil {
			return err
		}
		// Add blocks to blockchain.
		for i := 0; i < len(res.Block); i++ {
			b := res.Block[i]
			sev.SetBlockInternal(&service.SetBlockRequest{Block: b}, false /*broadcast=*/)
		}
		if res.Synced {
			sev.Log("fully synced")
			break
		}
		// Update tail and choose the next peer.
		tail = sev.fullNode.GetTail()
		i++
	}

	sev.m.Lock()
	defer sev.m.Unlock()
	// Although each of them is benign race, changing them together is not,
	// thus we need to use a mutex to protect them.
	sev.syncing = false
	sev.blockFailure = 0
	return nil
}

// Add a connection to peer, note that this is a best effort 2-way connection.
func (sev *FullNodeServer) AddPeer(ctx context.Context, req *service.AddPeerRequest) (*service.AddPeerResponse, error) {
	_, err := sev.AddPeerInternal(req)
	if err != nil {
		sev.Log("fail to add peer: " + err.Error())
	}
	return &service.AddPeerResponse{}, nil
}

func (sev *FullNodeServer) AddPeerInternal(req *service.AddPeerRequest) (service.FullNodeServiceClient, error) {
	sev.m.RLock()
	for _, p := range sev.peers {
		if p.addr.IpAddr == req.NodeAddr.IpAddr && p.addr.Port == req.NodeAddr.Port {
			return nil, errors.New(PEER_ALREADY_EXIST_ERR)
		}
	}
	sev.m.RUnlock()

	nodeAddr := req.NodeAddr
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// Create a connection to the incoming peer. Do not close the connection.
	// The ip is assumed to be a ipv4 address.
	conn, err := grpc.Dial(nodeAddr.IpAddr+":"+nodeAddr.Port, opts...)
	if err != nil {
		sev.Log(fmt.Sprintf("fail to dial peer when AddPeer: %v", err))
		return nil, err
	}
	client := service.NewFullNodeServiceClient(conn)

	// Convert from gRPC address proto to local address struct.
	addr := Address{
		IpAddr: nodeAddr.IpAddr,
		Port:   nodeAddr.Port,
	}

	sev.m.Lock()
	sev.peers = append(sev.peers, Peer{
		client: client,
		addr:   addr,
		conn:   conn,
	})
	sev.m.Unlock()

	// Spin up a process that GC idle connection, we GC the client in a
	// expo backoff way to avoid overloading any peer.
	go func() {
		// Retry 3 times in total.
		retry := 3
		// How many times we already tried.
		try := 0
		// Retry every 3 seconds.
		base := 3
		for {
			time.Sleep(time.Duration(base) * time.Second)
			if conn.GetState() == connectivity.Ready {
				// Reset on any successful retry.
				try = 0
				base = 3
				continue
			}
			try++
			// Exponential backoff for retry.
			base *= 2
			if try >= retry {
				// If we already tried enough times, we just break and reclaim the connection.
				break
			}
		}
		sev.Log(fmt.Sprintf("close dead peer: %s:%s", addr.IpAddr, addr.Port))
		conn.Close()
		sev.RemovePeer(addr)
	}()
	return client, nil
}

// Remove a peer from the peer list.
func (sev *FullNodeServer) RemovePeer(addr Address) {
	sev.m.Lock()
	defer sev.m.Unlock()

	for i := 0; i < len(sev.peers); i++ {
		if sev.peers[i].addr == addr {
			// Find the peer in peer list and remove it.
			sev.peers = append(sev.peers[:i], sev.peers[i+1:]...)
			return
		}
	}
}

// Add a mutual connection to a remote full node.
func (sev *FullNodeServer) AddMutualConnection(ipAddr string, port string) error {
	// Add peer node to self peer list.
	client, err := sev.AddPeerInternal(&service.AddPeerRequest{NodeAddr: &service.NodeAddr{IpAddr: ipAddr, Port: port}})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = client.AddPeer(ctx, &service.AddPeerRequest{NodeAddr: &service.NodeAddr{IpAddr: sev.addr.IpAddr, Port: sev.addr.Port}})
	// TODO(chenweilunster): This is very hacky..must be some better way to check
	// whether the peer error due to peer already exist.
	sev.m.Lock()
	defer sev.m.Unlock()
	if err != nil && err.Error() != PEER_ALREADY_EXIST_ERR {
		// Peer cannot add a new peer, prune this peer.
		sev.Log(err.Error())
		sev.peers = sev.peers[:len(sev.peers)-1]
		return err
	}
	return nil
}

// Handle the incoming block, this is the external RPC not intended to be called by
// internal functions. If the block is valid, just broadcast it to other nodes.
func (sev *FullNodeServer) SetBlock(con context.Context, req *service.SetBlockRequest) (*service.SetBlockResponse, error) {
	sev.Log(fmt.Sprintf("received a new block: %s", req.Block.Hash))
	res, tailChange, outOfSync, err := sev.SetBlockInternal(req, true /*broadcast=*/)
	// If there is a possible signal of out of sync, and we are not currently syncing,
	// we should try to sync with peer in a round robin manner.
	if err != nil && outOfSync && !sev.syncing {
		sev.m.Lock()
		sev.blockFailure++
		if sev.blockFailure >= int(sev.fullNode.config.CONFIRMATION) {
			sev.cmd <- commands.Command{
				Op: commands.SYNC,
			}
		}
		sev.m.Unlock()
	}

	// Only external block handling incurred tail change interrupts the mining process.
	if sev.fullNode.config.REMINE_ON_TAIL_CHANGE && tailChange {
		sev.cmd <- commands.Command{
			Op: commands.RESTART,
		}
	}
	if err != nil {
		sev.Log("fail to handle incoming block: " + req.Block.Hash + " err: " + err.Error())
	}
	return res, nil
}

// In sync mode we don't want to broadcast the block to other nodes, in all other cases we do.
func (sev *FullNodeServer) SetBlockInternal(req *service.SetBlockRequest, broadcast bool) (*service.SetBlockResponse, bool, bool, error) {
	block := req.Block
	tailChange, outOfSync, err := sev.fullNode.HandleNewBlock(block)
	if err != nil {
		return &service.SetBlockResponse{}, tailChange, outOfSync, err
	}

	// Broadcast to all other nodes.
	if !broadcast {
		return &service.SetBlockResponse{}, tailChange, outOfSync, err
	}

	sev.m.RLock()
	defer sev.m.RUnlock()
	for i := 0; i < len(sev.peers); i++ {
		peer := sev.peers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := peer.client.SetBlock(ctx, &service.SetBlockRequest{Block: block})
		if err != nil {
			sev.Log(err.Error())
		}
	}

	return &service.SetBlockResponse{}, tailChange, outOfSync, err
}

// Sync returns blocks it knows of starting from the given hash in request. If not found the block at all,
// return from the first non-genesis block in the blockchain. This function only returns blocks from the
// longest chain.
func (sev *FullNodeServer) Sync(ctx context.Context, req *service.SyncRequest) (*service.SyncResponse, error) {
	blocks, synced := sev.fullNode.GetBlocks(req.Hash, int(req.Number))
	return &service.SyncResponse{Block: blocks, Synced: synced}, nil
}

// Return all peers this full node knows of.
func (sev *FullNodeServer) GetPeers(ctx context.Context, req *service.GetPeersRequest) (*service.GetPeersResponse, error) {
	sev.m.RLock()
	defer sev.m.RUnlock()
	res := &service.GetPeersResponse{}
	for i := 0; i < len(sev.peers); i++ {
		p := &sev.peers[i]
		res.NodeAddrs = append(res.NodeAddrs, &service.NodeAddr{IpAddr: p.addr.IpAddr, Port: p.addr.Port})
	}
	return res, nil
}

// Ask the given full node to introduce his peers to me.
func (sev *FullNodeServer) Introduce(ip string, port string) ([]*service.NodeAddr, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// Create a connection to the incoming peer. Close this connection immediatly after return.
	conn, err := grpc.Dial(ip+":"+port, opts...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := service.NewFullNodeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := client.GetPeers(ctx, &service.GetPeersRequest{})
	if err != nil {
		return nil, err
	}
	return res.NodeAddrs, nil
}

func (sev *FullNodeServer) Show(d int) {
	tail := sev.fullNode.GetTail()
	visualize.RenderBlockChain(tail, d, sev.fullNode.uuid)
}

// Log the message. If the GUI is not nil, log to GUI, otherwise log to stdout.
func (sev *FullNodeServer) Log(s string) {
	if sev.g == nil {
		log.Println(s)
		return
	}
	sev.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("logger")
		if err != nil {
			log.Fatalln("fail to create logger, exit")
		}
		fmt.Fprintln(v, s)
		return nil
	})
}

// Create a new full node server with connection established. Exit if connection
// cannot be established.
func NewFullNodeServer(c config.AppConfig, ps []Peer, addr Address, keyPath string, cmd chan commands.Command, g *gocui.Gui) *FullNodeServer {
	sev := FullNodeServer{
		fullNode: NewFullNode(c, keyPath),
		peers:    ps,
		cmd:      cmd,
		addr:     addr,
		m:        sync.RWMutex{},
		g:        g,
	}
	for i := 0; i < len(ps); i++ {
		peer := ps[i]
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())

		conn, err := grpc.Dial(peer.addr.IpAddr+":"+peer.addr.Port, opts...)
		if err != nil {
			log.Fatalf("fail to dial: %v", err)
		}
		defer conn.Close()
		client := service.NewFullNodeServiceClient(conn)
		sev.peers[i].client = client
	}
	return &sev
}
