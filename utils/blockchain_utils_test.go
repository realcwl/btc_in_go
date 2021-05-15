package utils

import (
	"testing"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/model"
	"github.com/stretchr/testify/assert"
)

func createTestBlock() model.Block {
	return model.Block{
		PrevHash: "00ab",
		Txs: []*model.Transaction{
			{
				Hash: "887d",
			},
		},
		Nounce: 3,
		Coinbase: &model.Transaction{
			Hash: "00cd",
		},
	}
}

func TestGetBlockBytes(t *testing.T) {
	testBlock := createTestBlock()

	var expectedBlockBytes []byte

	actualBlockBytes, _ := GetBlockBytes(&testBlock)

	expectedBlockBytes = append(expectedBlockBytes, Int64ToBytes(testBlock.Nounce)...)
	preHashBytes, _ := HexToBytes(testBlock.PrevHash)
	expectedBlockBytes = append(expectedBlockBytes, preHashBytes...)
	txBytes, _ := GetTransactionBytes(testBlock.Txs[0], true)
	expectedBlockBytes = append(expectedBlockBytes, txBytes...)
	coinbaseBytes, _ := GetTransactionBytes(testBlock.Coinbase, true)
	expectedBlockBytes = append(expectedBlockBytes, coinbaseBytes...)
	assert.Equal(t, expectedBlockBytes, actualBlockBytes)
}

func TestMine(t *testing.T) {
	testDifficulty := 1
	testBlock := createTestBlock()
	testChan := make(chan commands.Command)

	_, actualErr := Mine(&testBlock, testDifficulty, testChan)
	assert.Nil(t, actualErr)
	expectedMatched, _ := MatchDifficulty(&testBlock, testDifficulty)
	assert.True(t, expectedMatched)
}

func TestMineInterruption(t *testing.T) {
	// Make a really difficult hash difficulty that's impossible to solve.
	testDifficulty := 100
	testBlock := createTestBlock()
	testChan := make(chan commands.Command)

	go func() {
		testChan <- commands.Command{
			Op: commands.STOP,
		}
	}()

	c, actualErr := Mine(&testBlock, testDifficulty, testChan)
	assert.Equal(t, c, commands.Command{
		Op: commands.STOP,
	})
	assert.NotNil(t, actualErr)
	assert.Equal(t, c, commands.Command{
		Op: commands.STOP,
	})
}

func TestMatchDifficulty(t *testing.T) {
	testDifficulty := 8
	testBlock := createTestBlock()
	actualMatched, actualDigest := MatchDifficulty(&testBlock, testDifficulty)
	blockBytes, expectedErr := GetBlockBytes(&testBlock)
	if expectedErr != nil {
		assert.Equal(t, "", actualDigest)
		assert.False(t, actualMatched)
	}
	digestBytes := SHA256(blockBytes)
	expectedDigest := BytesToHex(digestBytes)

	expectedRes := ByteHasLeadingZeros(digestBytes, testDifficulty)
	assert.Equal(t, expectedRes, actualMatched)
	assert.Equal(t, expectedDigest, actualDigest)
}

func TestByteHasLeadingZeros(t *testing.T) {
	testByte := []byte{2, 45, 40}
	assert.True(t, ByteHasLeadingZeros(testByte, 6))
	assert.False(t, ByteHasLeadingZeros(testByte, 9))
	assert.False(t, ByteHasLeadingZeros(testByte, 25))
}
