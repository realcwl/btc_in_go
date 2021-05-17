package full_node

import (
	"container/list"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/config"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/jinzhu/copier"
	uuid "github.com/satori/go.uuid"
)

// A full node should maintain the blockchain, and update the blockchain.
type FullNode struct {
	// The blockchain it needs to maintain.
	blockchain *model.Blockchain
	// Transaction pool it need to maintain. Incoming transaction are added to this pool.
	txPool *model.TransactionPool
	// keys contain private key and public key for this fullnode. Although we mostly care about public key.
	keys *rsa.PrivateKey
	// Peers in the network.
	// TODO(chenweilunster): Add this member.
	// Blockchain config.
	config config.AppConfig
	// A single mutex for changing internal state.
	m sync.RWMutex
	// A unique indentifier of this Fullnode, this doesn't impact consensus, only
	// used for easier implementation.
	uuid string
}

// Create a brand new full node, which contains a genesis block in the chain.
func NewFullNode(c config.AppConfig) *FullNode {
	myuuid := uuid.NewV4()
	sk, _ := utils.GenerateKeyPair(2048)
	return &FullNode{
		blockchain: model.NewBlockChain(),
		txPool:     model.NewTransactionPool(),
		keys:       sk,
		config:     c,
		m:          sync.RWMutex{},
		uuid:       myuuid.String(),
	}
}

func (f *FullNode) AddTransactionToPool(tx *model.Transaction) error {
	f.m.Lock()
	defer f.m.Unlock()

	if _, exist := f.txPool.TxPool[tx.Hash]; exist {
		return fmt.Errorf("existing transaction, will not process: %s", tx.Hash)
	}
	f.txPool.TxPool[tx.Hash] = tx
	return nil
}

// Return a deep copy of the ledger at tail.
func (f *FullNode) GetLedgerSnapshotAtDepth(depth int64) *model.Ledger {
	f.m.RLock()
	defer f.m.RUnlock()
	l := model.NewLedger()

	tail := f.blockchain.Tail
	for i := 0; i < int(depth); i++ {
		if tail.Parent == nil {
			break
		}
		tail = tail.Parent
	}
	copier.Copy(&l, tail.L)
	return l
}

// Create a new block with all transactions in the provided transaction pool. CreateNewBlock
// is a really long process and takes a long time to proccess.
// This block must be created after the tail block in the blockchain.
// cmd is a channel that interrupts the mining process at any time
func (f *FullNode) CreateNewBlock(ctl chan commands.Command, height int64) (*model.Block, commands.Command, error) {
	// Lock the transaction pool for reading.
	f.m.RLock()
	l := model.NewLedger()
	// Make a deepcopy of the ledger at tail.
	copier.Copy(l, f.blockchain.Tail.L)
	tail := f.blockchain.Tail
	txs := utils.GetAllTxsInPool(f.txPool)
	// Mining is a really heavy task
	f.m.RUnlock()
	block, c, err := utils.CreateNewBlock(txs, tail.B.Hash, f.config.COINBASE_REWARD, height, utils.PublicKeyToBytes(&f.keys.PublicKey), l, f.config.DIFFICULTY, ctl)
	return block, c, err
}

// Return the height of the tail block.
func (f *FullNode) GetHeight() int64 {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.blockchain.Tail.Height
}

// Return tail of the blockchain.
func (f *FullNode) GetTail() *model.BlockWrapper {
	f.m.RLock()
	defer f.m.RUnlock()
	return f.blockchain.Tail
}

// Create a snapshot of the given public key's ledger, return all UTXO it has.
// The snapshot must be obtained at the CONFIRMATION blocks ago, instead of directly
// snapshot at the tail. See bitcoin whitepaper for more details on block confirmation.
func (f *FullNode) GetUtxoForPublicKey(pk []byte) model.Ledger {
	l := f.GetLedgerSnapshotAtDepth(f.config.CONFIRMATION)
	res := model.NewLedger()
	for utxoLite, output := range l.L {
		if utils.IsSameBytes(pk, output.PublicKey) {
			res.L[utxoLite] = output
		}
	}
	return *res
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
//
// This function returns 3 things:
// 1. A boolean flag indicating whether a tail change happens. This is needed
// because depending on the config we might want to remine the blocks on tail change
// in order not to waste time mining on a deprecated tail.
// 2. A boolean flag indicating whether a block hanling failure could happen.
// TODO(chenweilunster): incoparate this flag into the error code.
// 3. An error indicating any error happened during mining.
func (f *FullNode) HandleNewBlock(pendingBlock *model.Block) (bool, bool, error) {
	// Lock mutex because we are changing the state of blockchain.
	f.m.Lock()
	defer f.m.Unlock()

	// Block should not already exist in blockchain.
	if _, ok := f.blockchain.Chain[pendingBlock.Hash]; ok {
		return false, false, fmt.Errorf("block already exist in the chain: %s", pendingBlock.Hash)
	}

	tailChange := false
	// Difficulty and hash should match.
	utils.MatchDifficulty(pendingBlock, f.config.DIFFICULTY)
	blockBytes, err := utils.GetBlockBytes(pendingBlock)
	if err != nil {
		return tailChange, false, err
	}
	if utils.BytesToHex(utils.SHA256(blockBytes)) != pendingBlock.Hash {
		return tailChange, false, errors.New("block hash is invalid")
	}

	// previous block should exist in blockchain.
	prevHash := pendingBlock.PrevHash
	prevBlockWrapper, ok := f.blockchain.Chain[prevHash]
	if !ok {
		log.Println("parent block not found in blockchain, parent block hash:", prevHash)
		// Parent not found in blockchain could signal that we're out of sync.
		return tailChange, true, errors.New("")
	}

	// Calculate its parent depth in the chain. If parent is greater than confirmation, it means
	// it already has a confirmed child. Thus, we should stop adding this block to the chain
	// because no matter what it will not win.
	parentDepth := f.blockchain.Tail.Height - prevBlockWrapper.Height
	if parentDepth > int64(f.config.CONFIRMATION) {
		return tailChange, false, errors.New("parent is buried too deep")
	}

	// Here we need to make a deep copy of the entire previous block's ledger because we are chaning it.
	l := model.NewLedger()
	copier.Copy(&l, &prevBlockWrapper.L)
	// Total transaction fee.
	fee, err := utils.CalcTxFee(pendingBlock.Txs, l)
	if err != nil {
		return tailChange, false, err
	}

	// Coinbase should be valid.
	err = utils.IsValidCoinbase(pendingBlock.Coinbase, fee+f.config.COINBASE_REWARD)
	if err != nil {
		return tailChange, false, err
	}

	// Handle all transactions and coinbase.
	err = utils.HandleTransactions(append(pendingBlock.Txs, pendingBlock.Coinbase), l)
	if err != nil {
		return tailChange, false, err
	}

	// Add block to blockchain and remove all transaction from the Tx pool.
	blockWrapper := model.BlockWrapper{
		B:      pendingBlock,
		Parent: prevBlockWrapper,
		Height: prevBlockWrapper.Height + 1,
		L:      l,
	}

	// Add the new block to the children of parent block.
	prevBlockWrapper.Children = append(prevBlockWrapper.Children, &blockWrapper)

	f.blockchain.Chain[pendingBlock.Hash] = &blockWrapper
	if blockWrapper.Height > f.blockchain.Tail.Height {
		f.blockchain.Tail = &blockWrapper
		tailChange = true
	}
	for i := 0; i < len(pendingBlock.Txs); i++ {
		tx := pendingBlock.Txs[i]
		delete(f.txPool.TxPool, tx.Hash)
	}

	return tailChange, false, nil
}

// GetBlocks returns a $number of blocks starting from the given hash. It only returns blocks from the longest chain.
func (f *FullNode) GetBlocks(hash string, number int) ([]*model.Block, bool) {
	f.m.RLock()
	dq := list.New()
	tail := f.blockchain.Tail
	f.m.RUnlock()

	synced := true
	for tail.B.Hash != hash {
		dq.PushFront(tail.B)
		if dq.Len() > number {
			// As long as we're poping anything, it means the chain is not synced
			// and will need another call to possibly other peers to fully sync.
			synced = false
			e := dq.Back()
			dq.Remove(e)
		}

		if tail.Parent == nil || tail.Parent.B.Hash == "00" {
			// Break when reaching genesis block.
			break
		}
		tail = tail.Parent
	}
	res := []*model.Block{}
	bw := dq.Front()
	for bw != nil {
		res = append(res, bw.Value.(*model.Block))
		bw = bw.Next()
	}
	return res, synced
}
