package full_node

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/Luismorlan/btc_in_go/visualize"
	"google.golang.org/grpc"
)

type Peer struct {
	// A service client established to connect to other full node.
	client service.FullNodeServiceClient
	// What ip address peer fullnode is using.
	ipAddr string
	// What TCP port peer full node is running on.
	port string
}

// This server
type FullNodeServer struct {
	service.UnimplementedFullNodeServiceServer
	// A bunch of peers that we have grpc connection to.
	peers    []Peer
	fullNode *FullNode
	// A command channel to pass command to other part of the system.
	// For now, the only use is the interrupt mining process on tail change.
	cmd chan commands.Command
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

// Handle the incoming block, this is the external RPC not intended to be called by
// internal functions. If the block is valid, just broadcast it to other nodes.
func (sev *FullNodeServer) SetBlock(con context.Context, req *service.SetBlockRequest) (*service.SetBlockResponse, error) {
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
func NewFullNodeServer(c config.AppConfig, ps []Peer, cmd chan commands.Command) *FullNodeServer {
	sev := FullNodeServer{
		fullNode: NewFullNode(c),
		peers:    ps,
		cmd:      cmd,
	}
	for i := 0; i < len(ps); i++ {
		peer := ps[i]
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())

		conn, err := grpc.Dial(peer.ipAddr+":"+peer.port, opts...)
		if err != nil {
			log.Fatalf("fail to dial: %v", err)
		}
		defer conn.Close()
		client := service.NewFullNodeServiceClient(conn)
		sev.peers[i].client = client
	}
	return &sev
}
