package utils

import (
	"errors"
	"fmt"

	"github.com/Luismorlan/btc_in_go/model"
)

// GetInputBytes converts input to byte slice. With or without the signature.
func GetInputBytes(input *model.Input, withSig bool) ([]byte, error) {
	var data []byte
	prevHash, err := HexToBytes(input.PrevTxHash)
	if err != nil {
		return nil, err
	}
	data = append(data, prevHash...)
	data = append(data, Int64ToBytes(input.Index)...)
	return data, nil
}

func GetOutputBytes(output *model.Output) []byte {
	var data []byte
	data = append(data, Float64ToBytes(output.Value)...)

	data = append(data, output.PublicKey...)
	return data
}

// Concat all inputs (including signature) and outputs raw data in byte slices.
// withHash specifies whether TX hash should be included or not.
func GetTransactionBytes(tx *model.Transaction, withHash bool) ([]byte, error) {
	var data []byte

	for i := 0; i < len(tx.Inputs); i++ {
		input := tx.Inputs[i]
		inputData, err := GetInputBytes(input, true /*withSig=*/)
		if err != nil {
			return nil, err
		}
		data = append(data, inputData...)
	}

	for i := 0; i < len(tx.Outputs); i++ {
		output := tx.Outputs[i]
		outputData := GetOutputBytes(output)
		data = append(data, outputData...)
	}

	// This is needed for Coinbase transaction to avoid block with only CB tx has same txid.
	data = append(data, Int64ToBytes(tx.Height)...)

	if withHash {
		hashBytes, err := HexToBytes(tx.Hash)
		if err != nil {
			return nil, err
		}
		data = append(data, hashBytes...)
	}

	return data, nil
}

// GetInputDataToSign fetches the input from the transaction and return it in byte stream.
func GetInputDataToSignByIndex(t *model.Transaction, index int) ([]byte, error) {
	var data []byte

	if len(t.Inputs)-1 < index {
		return nil, errors.New("index is out of the range")
	}
	input := t.Inputs[index]
	// Don't include signature since we haven't signed it yet.
	inputData, err := GetInputBytes(input, false /*withSig=*/)
	if err != nil {
		return nil, err
	}
	data = append(data, inputData...)

	for i := 0; i < len(t.Outputs); i++ {
		output := t.Outputs[i]
		outputData := GetOutputBytes(output)
		data = append(data, outputData...)
	}
	return data, nil
}

// A transaction is valid if:
// 1. Total outputs are smaller or equal to inputs.
// 2. All inputs are UTXO.
// 3. Outputs are non-negative number.
// 4. Signatures are valid.
// 5. No double spending.
// 6. Hash matches.
func IsValidTransaction(tx *model.Transaction, l *model.Ledger) error {
	var totalInput = 0.0
	var totalOutput = 0.0

	// Tx hash should match.
	txBytes, err := GetTransactionBytes(tx, false /*withHash*/)
	if err != nil {
		return err
	}
	if BytesToHex(SHA256(txBytes)) != tx.Hash {
		return fmt.Errorf("transaction contains a invalid hash: %+v", tx.String())
	}

	// Store all seen UTXOs to avoid double spending.
	seenUtxo := make(map[model.UTXOLite]bool)

	for i := 0; i < len(tx.Inputs); i++ {
		// Verify the input is using UTXO.
		input := tx.Inputs[i]
		inputUtxo := CreateUtxoFromInput(input)
		output, ok := l.L[model.GetUtxoLite(&inputUtxo)]
		if !ok {
			return fmt.Errorf("transaction input has been spent: %+v", tx.String())
		}
		totalInput += output.Value

		// Verify signature.
		inputData, err := GetInputBytes(input, false /*withSig=*/)
		if err != nil {
			return err
		}
		pk := BytesToPublicKey(output.PublicKey)
		if pk == nil {
			return errors.New("invalid bytes when reconstructing public key")
		}
		if isValid := Verify(inputData, pk, input.Signature); !isValid {
			return errors.New("signature verification failed")
		}

		// No double spending.
		if _, exist := seenUtxo[model.GetUtxoLite(&inputUtxo)]; exist {
			return fmt.Errorf("the input is a double spending: %+v", input.String())
		}
		seenUtxo[model.GetUtxoLite(&inputUtxo)] = true
	}

	for i := 0; i < len(tx.Outputs); i++ {
		// Output should be non-negative number.
		output := tx.Outputs[i]
		if output.Value < 0 {
			return fmt.Errorf("invalid output: %+v", output)
		}
		totalOutput += output.Value
	}

	if totalInput >= totalOutput {
		return fmt.Errorf("total input %+v is greater than total output %+v", totalInput, totalOutput)
	}
	return nil
}

// Calculate total transaction fee given transaction and ledger. This function will not
// modify ledger.
func CalcTxFee(txs []*model.Transaction, l *model.Ledger) (float64, error) {
	var fee float64
	for i := 0; i < len(txs); i++ {
		tx := txs[i]

		var totalInput = 0.0
		var totalOutput = 0.0

		for j := 0; i < len(tx.Inputs); j++ {
			// Verify the input is using UTXO.
			input := tx.Inputs[j]
			inputUtxo := CreateUtxoFromInput(input)
			output, ok := l.L[model.GetUtxoLite(&inputUtxo)]
			if !ok {
				return 0.0, errors.New("unexpected error: doesn't find utxo in ledger")
			}
			totalInput += output.Value
		}

		for j := 0; i < len(tx.Outputs); j++ {
			output := tx.Outputs[j]
			totalOutput += output.Value
		}

		if totalOutput > totalInput {
			return 0.0, errors.New("total output is greater than total inputs")
		}

		fee += totalOutput - totalInput
	}

	return fee, nil
}

// Fill hash simply compute the SHA256 hash for the transaction raw data and set the hash.
func FillTxHash(tx *model.Transaction) error {
	if tx == nil {
		return errors.New("input transaction to hash cannot be nil")
	}
	data, err := GetTransactionBytes(tx, false /*withHash*/)
	if err != nil {
		return err
	}
	hash := SHA256(data)
	tx.Hash = BytesToHex(hash)
	return nil
}

// A valid coinbase transaction should contains 0 input and 1 output. And total reward should be
// smaller than transaction fee + default reward.
// READONLY:
// * tx
func IsValidCoinbase(tx *model.Transaction, maxFee float64) error {
	// Tx hash should match.
	txBytes, err := GetTransactionBytes(tx, false /*withHash*/)
	if err != nil {
		return err
	}
	if BytesToHex(SHA256(txBytes)) != tx.Hash {
		return fmt.Errorf("coinbase transaction contains a invalid hash: %+v", tx.String())
	}

	// Should contains 0 input and 1 output.
	if len(tx.Inputs) != 0 || len(tx.Outputs) != 1 {
		return fmt.Errorf("coinbase should contain 0 input and 1 output, actual: %d, %d", len(tx.Inputs), len(tx.Outputs))
	}

	// total fee should be smaller than maxFee.
	if tx.Outputs[0].Value > maxFee {
		return fmt.Errorf("total fee: %f is greater than allowed: %f", tx.Outputs[0].Value, maxFee)
	}

	return nil
}

//Create a transaction with a single output, which is the miner's public key.
// READONLY:
// *pk
func CreateCoinbaseTx(totalReward float64, pk []byte, height int64) *model.Transaction {
	tx := &model.Transaction{
		Outputs: []*model.Output{{
			Value:     totalReward,
			PublicKey: pk,
		}},
		Height: height,
	}
	// Ignore error because tx can never be nil.
	FillTxHash(tx)
	return tx
}

// Return all transactions in the pool
func GetAllTxsInPool(txPool *model.TransactionPool) []*model.Transaction {
	txs := make([]*model.Transaction, len(txPool.TxPool))
	for _, tx := range txPool.TxPool {
		txs = append(txs, tx)
	}
	return txs
}
