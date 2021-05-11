package utils

import (
	"errors"
	"log"

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
func GetTransactionBytes(t *model.Transaction) ([]byte, error) {
	var data []byte
	for i := 0; i < len(t.Inputs); i++ {
		input := &t.Inputs[i]
		inputData, err := GetInputBytes(input, true /*withSig=*/)
		if err != nil {
			return nil, err
		}
		data = append(data, inputData...)
	}

	for i := 0; i < len(t.Outputs); i++ {
		output := &t.Outputs[i]
		outputData := GetOutputBytes(output)
		data = append(data, outputData...)
	}
	return data, nil
}

// GetInputDataToSign fetches the input from the transaction and return it in byte stream.
func GetInputDataToSignByIndex(t *model.Transaction, index int) ([]byte, error) {
	var data []byte

	if len(t.Inputs)-1 < index {
		return nil, errors.New("index is out of the range")
	}
	input := &t.Inputs[index]
	// Don't include signature since we haven't signed it yet.
	inputData, err := GetInputBytes(input, false /*withSig=*/)
	if err != nil {
		return nil, err
	}
	data = append(data, inputData...)

	for i := 0; i < len(t.Outputs); i++ {
		output := &t.Outputs[i]
		outputData := GetOutputBytes(output)
		data = append(data, outputData...)
	}
	return data, nil
}

// A transaction is valid if:
// 0. Is a valid coinbase transaction.
// 1. Total outputs are smaller or equal to inputs.
// 2. All inputs are UTXO.
// 3. Outputs are non-negative number.
// 4. Signatures are valid.
// 5. No double spending.
func IsValidTransaction(t *model.Transaction, ledger *model.Ledger) bool {
	var totalInput = 0.0
	var totalOutput = 0.0

	// Store all seen UTXOs to avoid double spending.
	seenUtxo := make(map[model.UTXO]bool)

	for i := 0; i < len(t.Inputs); i++ {
		// Verify the input is using UTXO.
		input := &t.Inputs[i]
		inputUtxo := CreateUtxoFromInput(input)
		output, ok := ledger.L[inputUtxo]
		if !ok {
			log.Println("Transaction input has been spent: ", *t)
			return false
		}
		totalInput += output.Value

		// Verify signature.
		inputData, err := GetInputBytes(input, false /*withSig=*/)
		if err != nil {
			log.Println(err)
			return false
		}
		pk := BytesToPublicKey(output.PublicKey)
		if pk == nil {
			log.Println("Invalid bytes when reconstructing public key.")
			return false
		}
		if isValid := Verify(inputData, pk, input.Signature); !isValid {
			log.Println("The input's signature doesn't match Tx data", *input)
			return false
		}

		// No double spending.
		if _, exist := seenUtxo[inputUtxo]; exist {
			log.Println("The input is a double spending", *input)
			return false
		}
		seenUtxo[inputUtxo] = true
	}

	for i := 0; i < len(t.Outputs); i++ {
		// Output should be non-negative number.
		output := t.Outputs[i]
		if output.Value < 0 {
			log.Println("Invalid output", output)
			return false
		}
		totalOutput += output.Value
	}

	return totalInput >= totalOutput
}
