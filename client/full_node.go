package client

import (
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/network"
)

// A full node should maintain the blockchain, and update the blockchain.
type FullNode struct {
	// The blockchain it needs to maintain.
	blockchain model.Blockchain
	// Transaction pool it need to maintain. Incoming transaction are added to this pool.
	txPool model.TransactionPool
	// The network needed to interact with other nodes.
	network network.Network
}
