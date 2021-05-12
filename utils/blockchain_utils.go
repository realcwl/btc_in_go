package utils

import (
	"log"

	"github.com/Luismorlan/btc_in_go/model"
)

// TODO(CX): Create a block from the provided transactions and the previous hash.
// 1. Fill in previous hash.
// 2. Create coinbase transactions (Reward + Tx fee).
// 3. Fill in transactions provided.
// Note that, the returned block is not ready to be sent on wire because it lacks nounce and hash,
// which will be filled by mining.
func CreateNewBlock(txs []model.Transaction, prevHash string) model.Block {
	return model.Block{}
}

// TODO(chenweilunster): Mine a block, fill the nounce and hash given the current difficulty setting.
// difficulty - how many leading zeros
func Mine(block *model.Block, difficulty int) error {
	return nil
}

// TODO(chenweilunster): Handle the new block received.
// This function should:
// 1. Validate the block.
// 2. Add to blockchain.
func HandleNewBlock(pendingBlock *model.Block, blockchain *model.Blockchain) bool {
	return false
}

// TODO(chenweilunster): Validate the block.
// 1. Parent exist in the blockchain.
// 2. Difficulty matches.
// 3. Each transaction in the block is valid.
// 4. Block is not too deep.
func ValidateBlock(pendingBlock *model.Block, blockchain *model.Blockchain, difficulty int) bool {
	return false
}

// TODO : get block in bytes format
func GetBlockBytes(block *model.Block) ([]byte, error) {
	var rawBlock []byte

	// convert nounce to bytes
	nounceBytes := Int64ToBytes(block.Nounce)
	rawBlock = append(rawBlock, nounceBytes...)

	// convert preHash to bytes
	preHashBytes, err := HexToBytes(block.PrevHash)
	if err != nil {
		return nil, err
	}
	rawBlock = append(rawBlock, preHashBytes...)

	// convert transactions to bytes
	for _, tx := range block.Txs {
		txBytes, err := GetTransactionBytes(&tx)
		if err == nil {
			return nil, err
		}
		rawBlock = append(rawBlock, txBytes...)
	}

	// covert coinbase to bytes
	coinbaseBytes, err := GetTransactionBytes(&block.Coinbase)
	if err != nil {
		return nil, err
	}
	rawBlock = append(rawBlock, coinbaseBytes...)

	return rawBlock, nil
}

func MatchDifficulty(block *model.Block, difficulty int) bool {
	blockBytes, err := GetBlockBytes(block)
	if err != nil {
		log.Println(err)
		return false
	}
	return ByteHasLeadingZeros(blockBytes, difficulty)
}

func ByteHasLeadingZeros(bytes []byte, difficulty int) bool {
	return true
}
