package wallet

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/stretchr/testify/assert"
)

func GetTestWallet() Wallet {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	utxos := model.UTXOLite{
		PrevTxHash: "2334ad",
		Index:      5,
	}
	output := model.Output{
		Value:     50,
		PublicKey: utils.PublicKeyToBytes(&privateKey.PublicKey),
	}

	return Wallet{
		Keys: privateKey,
		UTXOs: map[model.UTXOLite]*model.Output{
			utxos: &output,
		},
	}
}

func TestCreatePendingTransaction(t *testing.T) {
	testWallet := GetTestWallet()
	receiverPK, _ := rsa.GenerateKey(rand.Reader, 2048)
	testOutputs := []*model.Output{
		{
			Value:     10,
			PublicKey: utils.PublicKeyToBytes(&receiverPK.PublicKey),
		},
	}

	actualTx, _ := CreatePendingTransaction(&testWallet, testOutputs)

	actualSignature := actualTx.Inputs[0].Signature

	expectedInput := &model.Input{
		PrevTxHash: "2334ad",
		Index:      5,
	}
	selfOutput := &model.Output{
		Value:     40,
		PublicKey: utils.PublicKeyToBytes(&testWallet.Keys.PublicKey),
	}
	expectedOutputs := testOutputs
	expectedOutputs = append(expectedOutputs, selfOutput)

	expectedPendingTx := model.Transaction{
		Inputs:  []*model.Input{expectedInput},
		Outputs: expectedOutputs,
	}
	expectedMsg, _ := utils.GetInputDataToSignByIndex(&expectedPendingTx, 0)

	assert.True(t, utils.Verify(expectedMsg, &testWallet.Keys.PublicKey, actualSignature))
}
