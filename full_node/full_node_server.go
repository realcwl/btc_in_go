package full_node

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/Luismorlan/btc_in_go/visualize"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type Peer struct {
	// A service client established to connect to other full node.
	client service.FullNodeServiceClient
	// Peer address
	addr Address
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

	// Create a mutex protect peers addition and deletion.
	pm sync.RWMutex

	fullNode *FullNode
	// A command channel to pass command to other part of the system.
	// For now, the only use is the interrupt mining process on tail change.
	cmd chan commands.Command
}

// Return all current peers.
func (sev *FullNodeServer) GetAllPeers() []Peer {
	sev.pm.RLock()
	defer sev.pm.RUnlock()
	return sev.peers
}

// Set transaction should add transaction to pool and broad cast to peer.
func (sev *FullNodeServer) SetTransaction(con context.Context, req *service.SetTransactionRequest) (*service.SetTransactionResponse, error) {
	tx := req.GetTx()
	if tx == nil {
		return &service.SetTransactionResponse{}, errors.New("input transaction is nil")
	}

	// First validate the transaction. This is totally optional but is a nice to have optimization.
	l := sev.fullNode.GetLedgerSnapshotAtDepth(0)
	err := utils.IsValidTransaction(tx, l)
	if err != nil {
		return &service.SetTransactionResponse{}, err
	}

	// Add the transaction to pool.
	err = sev.fullNode.AddTransactionToPool(tx)
	if err != nil {
		return &service.SetTransactionResponse{}, err
	}

	// Broadcast to all other nodes.
	sev.pm.RLock()
	defer sev.pm.RUnlock()
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
	res := service.GetBalanceResponse{}
	for utxoLite, output := range l.L {
		utxo := model.GetUtxo(&utxoLite)
		pair := service.UtxoOutputPair{
			Utxo:   &utxo,
			Output: output,
		}
		res.UtxoOutputPairs = append(res.UtxoOutputPairs, &pair)
	}
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
	_, _, err = sev.SetBlockInternal(&service.SetBlockRequest{Block: b})
	return commands.NewDefaultCommand(), err
}

// Add a connection to peer, note that this is a best effort 2-way connection.
func (sev *FullNodeServer) AddPeer(ctx context.Context, req *service.AddPeerRequest) (*service.AddPeerResponse, error) {
	_, err := sev.AddPeerInternal(req)
	return &service.AddPeerResponse{}, err
}

func (sev *FullNodeServer) AddPeerInternal(req *service.AddPeerRequest) (service.FullNodeServiceClient, error) {
	sev.pm.RLock()
	for _, p := range sev.peers {
		if p.addr.IpAddr == req.NodeAddr.IpAddr && p.addr.Port == req.NodeAddr.Port {
			return nil, errors.New("peer already exist")
		}
	}
	sev.pm.RUnlock()

	nodeAddr := req.NodeAddr
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	// Create a connection to the incoming peer. Do not close the connection.
	// The ip is assumed to be a ipv4 address.
	conn, err := grpc.Dial(nodeAddr.IpAddr+":"+nodeAddr.Port, opts...)
	if err != nil {
		log.Printf("fail to dial peer when AddPeer: %v", err)
		return nil, err
	}
	client := service.NewFullNodeServiceClient(conn)

	// Convert from gRPC address proto to local address struct.
	addr := Address{
		IpAddr: nodeAddr.IpAddr,
		Port:   nodeAddr.Port,
	}

	sev.pm.Lock()
	sev.peers = append(sev.peers, Peer{
		client: client,
		addr:   addr,
	})
	sev.pm.Unlock()

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
		log.Println("close dead peer:", addr)
		conn.Close()
		sev.RemovePeer(addr)
	}()
	return client, nil
}

// Remove a peer from the peer list.
func (sev *FullNodeServer) RemovePeer(addr Address) {
	sev.pm.Lock()
	defer sev.pm.Unlock()
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
	sev.pm.Lock()
	defer sev.pm.Unlock()
	if err != nil && err.Error() != "peer already exist" {
		// Peer cannot add a new peer, prune this peer.
		log.Println(err)
		sev.peers = sev.peers[:len(sev.peers)-1]
		return err
	}
	return nil
}

// Handle the incoming block, this is the external RPC not intended to be called by
// internal functions. If the block is valid, just broadcast it to other nodes.
func (sev *FullNodeServer) SetBlock(con context.Context, req *service.SetBlockRequest) (*service.SetBlockResponse, error) {
	log.Println("Received a new block: ", req.Block.Hash)
	res, tailChange, err := sev.SetBlockInternal(req)
	// Only external block handling incurred tail change interrupts the mining process.
	if sev.fullNode.config.REMINE_ON_TAIL_CHANGE && tailChange {
		sev.cmd <- commands.Command{
			Op: commands.RESTART,
		}
	}
	return res, err
}

func (sev *FullNodeServer) SetBlockInternal(req *service.SetBlockRequest) (*service.SetBlockResponse, bool, error) {
	block := req.Block
	tailChange, err := sev.fullNode.HandleNewBlock(block)
	if err != nil {
		return &service.SetBlockResponse{}, tailChange, err
	}

	// Broadcast to all other nodes.
	sev.pm.RLock()
	defer sev.pm.RUnlock()
	for i := 0; i < len(sev.peers); i++ {
		peer := sev.peers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := peer.client.SetBlock(ctx, &service.SetBlockRequest{Block: block})
		if err != nil {
			log.Println(err)
		}
	}

	return &service.SetBlockResponse{}, tailChange, err
}

func (sev *FullNodeServer) Show(d int) {
	tail := sev.fullNode.GetTail()
	visualize.Render(tail, d, sev.fullNode.uuid)
}

// Create a new full node server with connection established. Exit if connection
// cannot be established.
func NewFullNodeServer(c config.AppConfig, ps []Peer, addr Address, cmd chan commands.Command) *FullNodeServer {
	sev := FullNodeServer{
		fullNode: NewFullNode(c),
		peers:    ps,
		cmd:      cmd,
		addr:     addr,
		pm:       sync.RWMutex{},
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
