package utils

import "github.com/Luismorlan/btc_in_go/model"

func CreateUtxoFromInput(input *model.Input) model.UTXO {
	return model.UTXO{
		PrevTxHash: input.PrevTxHash,
		Index:      input.Index,
	}
}

// Handle transaction:
// 1. Validate transaction.
// 2. Claim every input.
// 3. Store every output.
func HandleTransaction(tx *model.Transaction, l *model.Ledger) bool {
	// First validate the transaction.
	if !IsValidTransaction(tx, l) {
		return false
	}

	// Claim every input
	for i := 0; i < len(tx.Inputs); i++ {
		input := &tx.Inputs[i]
		utxo := CreateUtxoFromInput(input)
		delete(l.L, utxo)
	}

	// Store every output
	for i := 0; i < len(tx.Outputs); i++ {
		output := &tx.Outputs[i]
		utxo := model.UTXO{
			PrevTxHash: tx.Hash,
			Index:      int64(i),
		}
		l.L[utxo] = *output
	}

	return true
}

// Handle a bunch of transactions.
// Note that ledger will be changed directly, when passing ledger to this function, be sure to pass a deep copy.
func HandleTransactions(txs []model.Transaction, l *model.Ledger) bool {
	for i := 0; i < len(txs); i++ {
		tx := &txs[i]
		success := HandleTransaction(tx, l)
		if !success {
			return false
		}
	}
	return true
}
