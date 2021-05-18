package utils

import (
	"errors"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/model"
)

// TODO(CX): Create a block from the provided transactions and the previous hash, miner's reward, current
// 1. Fill in previous hash.
// 2. Create coinbase transactions (Reward + Tx fee).
// 3. Fill in transactions provided.
// 4. Mine the block.
// Also, **input ledger must be a deep copy because it will be change permanently.**
func CreateNewBlock(txs []*model.Transaction, prevHash string, reward float64, height int64, pk []byte, l *model.Ledger, difficulty int, ctl chan commands.Command) (*model.Block, commands.Command, error) {
	// First calculate transaction fee.
	fee, err := CalcTxFee(txs, l)
	if err != nil {
		return nil, commands.NewDefaultCommand(), err
	}

	err = HandleTransactions(txs, l)
	if err != nil {
		return nil, commands.NewDefaultCommand(), err
	}

	block := model.Block{
		PrevHash: prevHash,
		Txs:      txs,
		Coinbase: CreateCoinbaseTx(reward+fee, pk, height),
	}

	c, err := Mine(&block, difficulty, ctl)
	return &block, c, err
}

// TODO(chenweilunster): Mine a block, fill the nounce and hash given the current difficulty setting.
// difficulty - how many leading zeros
// Always listen for command interruption and stop mining at any time.
func Mine(block *model.Block, difficulty int, ctl chan commands.Command) (commands.Command, error) {
	for i := 0; i < int(^uint(0)>>1); i++ {
		//time.Sleep(time.Second)
		select {
		case c := <-ctl:
			return c, errors.New("terminated by command")
		default:
			block.Nounce = int64(i)
			isMatched, digest := MatchDifficulty(block, difficulty)
			if isMatched {
				block.Hash = digest
				return commands.NewDefaultCommand(), nil
			}
		}
	}
	return commands.NewDefaultCommand(), errors.New("failed to find any nounce")
}

// Get block bytes without its hash.
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
	for i := 0; i < len(block.Txs); i++ {
		tx := block.Txs[i]
		txBytes, err := GetTransactionBytes(tx, true /*withHash*/)
		if err != nil {
			return nil, err
		}
		rawBlock = append(rawBlock, txBytes...)
	}

	// covert coinbase to bytes
	coinbaseBytes, err := GetTransactionBytes(block.Coinbase, true /*withHash*/)
	if err != nil {
		return nil, err
	}
	rawBlock = append(rawBlock, coinbaseBytes...)

	return rawBlock, nil
}

func MatchDifficulty(block *model.Block, difficulty int) (bool, string) {
	blockBytes, err := GetBlockBytes(block)
	if err != nil {
		return false, ""
	}
	digest := SHA256(blockBytes)
	return ByteHasLeadingZeros(digest, difficulty), BytesToHex(digest)
}

func ByteHasLeadingZeros(bytes []byte, difficulty int) bool {
	numOfZeroBytes := difficulty / 8
	numOfZeroBits := difficulty % 8

	totalBytes := numOfZeroBytes
	if numOfZeroBits > 0 {
		totalBytes += 1
	}
	if totalBytes > len(bytes) {
		return false
	}
	for i := 0; i < numOfZeroBytes; i++ {
		if bytes[i] != 0 {
			return false
		}
	}
	nextByte := bytes[numOfZeroBytes]

	return (nextByte>>byte(8-numOfZeroBits))&0xFF == 0
}
