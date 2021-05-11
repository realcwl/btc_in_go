package model

type Block struct {
	// Hash of this entire block in the hex string format.
	Hash string
	// Hash of the previous block in the hex format.
	PrevHash string
	// Transactions for this block. The first transaction is the coinbase transaction.
	Txs []Transaction
	// Coinbase transaction as the miner's reward.
	Coinbase Transaction
	// Nouce is the miner's chanllenge for computing the block.
	Nounce int64
}

// BlockWrapper stores both the block information and it's metadata on blockchain.
type BlockWrapper struct {
	// The actual block
	B Block
	// There can be multiple children because we allow fork.
	Children []BlockWrapper
	// height in the blockchain.
	Height int64
	// Ledger at that node.
	L Ledger
}

type Blockchain struct {
	// The block with the maximum height
	Tail BlockWrapper
	// A map from hex string of the block hash to block wrapper.
	Chain map[string]BlockWrapper
}

// Create a new blockchain
func NewBlockChain() Blockchain {
	return Blockchain{
		Tail:  BlockWrapper{},
		Chain: make(map[string]BlockWrapper),
	}
}
