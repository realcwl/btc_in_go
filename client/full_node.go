package client

import (
	"crypto/rsa"
	"errors"
	"sync"

	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/jinzhu/copier"
)

// A full node should maintain the blockchain, and update the blockchain.
type FullNode struct {
	// The blockchain it needs to maintain.
	blockchain model.Blockchain
	// Transaction pool it need to maintain. Incoming transaction are added to this pool.
	txPool model.TransactionPool
	// keys contain private key and public key for this fullnode. Although we mostly care about public key.
	keys rsa.PrivateKey
	// Peers in the network.
	// TODO(chenweilunster): Add this member.
	// Blockchain config.
	config config.AppConfig
	// A single mutex for changing internal state.
	m sync.RWMutex
}

// Create a brand new full node.
func NewFullNode(c config.AppConfig) *FullNode {
	return nil
}

// Create a new block with all transactions in the provided transaction pool.
func (f *FullNode) CreateNewBlock() error {
	return nil
}

// Handle the new block received.
// This function should:
// 1. Validate the block.
//   a. Parent exist in the blockchain.
//   b. Difficulty matches.
//   c. Each transaction in the block is valid.
//   d. Block is not too deep.
//   e. Total input should be <= reward + tx fee
// 2. Add to blockchain.
// Return false if the block is invalid.
func (f *FullNode) HandleNewBlock(pendingBlock *model.Block) error {
	// Lock mutex because we are changing the state of blockchain.
	f.m.Lock()
	defer f.m.Unlock()

	// Difficulty and hash should match.
	utils.MatchDifficulty(pendingBlock, f.config.DIFFICULTY)
	blockBytes, err := utils.GetBlockBytes(pendingBlock)
	if err != nil {
		return err
	}
	if utils.BytesToHex(utils.SHA256(blockBytes)) != pendingBlock.Hash {
		return errors.New("block hash is invalid")
	}

	// previous block should exist in blockchain.
	prevHash := pendingBlock.PrevHash
	prevBlockWrapper, ok := f.blockchain.Chain[prevHash]
	if !ok {
		return errors.New("parent block not found in blockchain")
	}

	// Calculate its parent depth in the chain. If parent is greater than confirmation, it means
	// it already has a confirmed child. Thus, we should stop adding this block to the chain
	// because no matter what it will not win.
	parentDepth := f.blockchain.Tail.Height - prevBlockWrapper.Height
	if parentDepth > int64(f.config.CONFIRMATION) {
		return errors.New("parent is buried too deep")
	}

	// Here we need to make a deep copy of the entire previous block's ledger because we are chaning it.
	l := model.NewLedger()
	copier.Copy(&l, &prevBlockWrapper.L)
	// Total transaction fee.
	fee, err := utils.CalcTxFee(pendingBlock.Txs, &l)
	if err != nil {
		return err
	}

	// Coinbase should be valid.
	err = utils.IsValidCoinbase(&pendingBlock.Coinbase, fee+f.config.COINBASE_REWARD)
	if err != nil {
		return err
	}

	// Each transaction should be able to add to blockchain.
	err = utils.HandleTransactions(pendingBlock.Txs, &l)
	if err != nil {
		return err
	}

	// Add block to blockchain and remove all transaction from the Tx pool.
	blockWrapper := model.BlockWrapper{
		B:      *pendingBlock,
		Parent: prevBlockWrapper,
		Height: prevBlockWrapper.Height + 1,
		L:      l,
	}
	f.blockchain.Chain[pendingBlock.Hash] = &blockWrapper
	if blockWrapper.Height > f.blockchain.Tail.Height {
		f.blockchain.Tail = &blockWrapper
	}
	for i := 0; i < len(pendingBlock.Txs); i++ {
		tx := pendingBlock.Txs[i]
		delete(f.txPool.TxPool, tx.Hash)
	}

	return nil
}
