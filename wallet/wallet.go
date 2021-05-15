package client

import (
	"crypto/rsa"
	"log"
	"os"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/utils"
)

// User signs and sends transactions to network.
type Wallet struct {
	keys       *rsa.PrivateKey
	fullNodeIp string
	utxos      map[model.UTXOLite]*model.Output
}

// Create a pending transaction to transfer money to users with public key
// wallet : a pointer to a wallet struct
// outputs : an array of struct Output
// READONLY:
// * wallet
func CreatePendingTransaction(wallet *Wallet, outputs []*model.Output) (*model.Transaction, error) {
	var inputs []*model.Input
	// Total money from all UTXOs
	var totalValue = 0.0
	// building inputs for pending transaction
	for utxo := range wallet.utxos {
		input := &model.Input{
			PrevTxHash: utxo.PrevTxHash,
			Index:      utxo.Index,
		}

		inputs = append(inputs, input)
		totalValue += float64(wallet.utxos[utxo].Value)
	}
	// Total amount of money will be transferred to others
	var totalTransferValue = 0.0
	for i := 0; i < len(outputs); i++ {
		totalTransferValue += float64(outputs[i].Value)
	}

	// Output with amount of money left after transfer
	selfOutput := model.Output{
		Value:     (totalValue - totalTransferValue),
		PublicKey: utils.PublicKeyToBytes(&wallet.keys.PublicKey),
	}
	outputs = append(outputs, &selfOutput)
	// build pending transaction with inputs and outputs
	pendingTransaction := model.Transaction{
		Inputs:  inputs,
		Outputs: outputs,
	}
	// sign inputs with own private key
	for i := 0; i < len(inputs); i++ {
		toSignMsg, err := utils.GetInputDataToSignByIndex(&pendingTransaction, i)
		if err != nil {
			return &model.Transaction{}, nil
		}
		inputs[i].Signature, err = utils.Sign(toSignMsg, wallet.keys)
		if err != nil {
			return &model.Transaction{}, nil
		}
	}
	transactionBytes, err := utils.GetTransactionBytes(&pendingTransaction, false)
	if err != nil {
		return &model.Transaction{}, nil
	}
	// get Hash for transaction
	pendingTransaction.Hash = string(utils.SHA256(transactionBytes))
	return &pendingTransaction, nil
}

func main() {
	log.Println(os.Args[1:])
}
