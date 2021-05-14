package full_node

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
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
}

// Set transaction should add transaction to pool and broad cast to peer.
func (sev *FullNodeServer) SetTransaction(con context.Context, req *service.SetTransactionRequest) (*service.SetTransactionResponse, error) {
	tx := req.GetTx()
	if tx == nil {
		return &service.SetTransactionResponse{}, errors.New("input transaction is nil")
	}

	// First validate the transaction. This is totally optional but is a nice to have optimization.
	l := sev.fullNode.GetTailLedgerSnapshot()
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

// Handle the incoming block, if the block is valid, just broad cast it to other nodes.
func (sev *FullNodeServer) SetBlock(con context.Context, req *service.SetBlockRequest) (*service.SetBlockResponse, error) {
	block := req.Block
	err := sev.fullNode.HandleNewBlock(block)
	if err != nil {
		return &service.SetBlockResponse{}, err
	}

	// Broadcast to all other nodes.
	for i := 0; i < len(sev.peers); i++ {
		peer := sev.peers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		peer.client.SetBlock(ctx, &service.SetBlockRequest{Block: block})
	}

	return &service.SetBlockResponse{}, err
}

// Create a new full node server with connection established. Exit if connection
// cannot be established.
func NewFullNodeServer(c config.AppConfig, ps []Peer) *FullNodeServer {
	sev := FullNodeServer{
		fullNode: NewFullNode(c),
		peers:    ps,
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
