package utils

import (
	"testing"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/stretchr/testify/assert"
)

func createTestBlock() model.Block {
	return model.Block{
		Hash:     "sdcdskcsdk",
		PrevHash: "dsweweq",
		Txs: []model.Transaction{
			{
				Hash: "sdcsdcsd",
			},
		},
		Nounce: 3,
		Coinbase: model.Transaction{
			Hash: "sdcsdcsd",
		},
	}
}

func TestGetBlockBytes(t *testing.T) {
	testBlock := createTestBlock()

	var expectedBlockBytes []byte

	actualBlockBytes, actualErr := GetBlockBytes(&testBlock)

	expectedBlockBytes = append(expectedBlockBytes, Int64ToBytes(testBlock.Nounce)...)
	preHashBytes, hashErr := HexToBytes(testBlock.PrevHash)
	if hashErr == nil {
		expectedBlockBytes = append(expectedBlockBytes, preHashBytes...)

		txBytes, txsErr := GetTransactionBytes(&testBlock.Txs[0])
		if txsErr == nil {
			expectedBlockBytes = append(expectedBlockBytes, txBytes...)

			coinbaseBytes, cbErr := GetTransactionBytes(&testBlock.Coinbase)
			if cbErr == nil {
				expectedBlockBytes = append(expectedBlockBytes, coinbaseBytes...)
				assert.Equal(t, expectedBlockBytes, actualBlockBytes)
				assert.Nil(t, actualErr)
			} else {
				assert.Nil(t, actualBlockBytes)
				assert.Equal(t, hashErr, actualErr)
			}
		} else {
			assert.Nil(t, actualBlockBytes)
			assert.Equal(t, hashErr, actualErr)
		}

	} else {
		assert.Nil(t, actualBlockBytes)
		assert.Equal(t, hashErr, actualErr)
	}
}
