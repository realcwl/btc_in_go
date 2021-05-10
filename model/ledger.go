package model

// Unspent transaction output. All UTXO are aggregated as a ledger and snapshoted at each block in the blockchain.
type UTXO struct {
	// Hex string of the transaction.
	PrevTxHash string
	// The index of the output in that transaction. Together with PrevTxHash, it identifies the unique output.
	Index int64
}

// Ledger is simply a pool of UTXO.
type Ledger struct {
	L map[UTXO]Output
}

func NewLedger() Ledger {
	return Ledger{
		L: make(map[UTXO]Output),
	}
}
