package utils

import (
	"errors"

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

func GetOutputBytes(output *model.Output) ([]byte, error) {
	var data []byte
	data = append(data, Float64ToBytes(output.Value)...)
	pkData := PublicKeyToBytes(&output.PublicKey)
	if pkData == nil {
		return nil, errors.New("fail to convert public kep provided")
	}
	data = append(data, pkData...)
	return data, nil
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
		outputData, err := GetOutputBytes(output)
		if err != nil {
			return nil, err
		}
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
		outputData, err := GetOutputBytes(output)
		if err != nil {
			return nil, err
		}
		data = append(data, outputData...)
	}
	return data, nil
}

// A transaction is valid if:
// 1. All outputs are smaller or equal to inputs.
// 2. All inputs are UTXO.
// 3. Outputs are non-negative number.
// 4. Signatures are valid.
// 5. No double spending.
func IsValidTransaction(t *model.Transaction, ledger *model.Ledger) bool {
	// TODO: IsValid transaction validates that a single transaction is valid.
	return false
}
