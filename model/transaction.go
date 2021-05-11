package model

type Input struct {
	// Hash of the transaction that outputs this coin.
	PrevTxHash string
	// The index of the output in that transaction. Together with PrevTxHash, it identifies the unique output.
	Index int64
	// Signature using the previous owner's PK.
	Signature []byte
}

type Output struct {
	// how much value to transfer.
	Value float64
	// Public key of the receiver, in the form of bytes.
	PublicKey []byte
}

type Transaction struct {
	// Hash of this transaction. We use this to uniquely identify the transaction.
	Hash string
	// All inputs of this transaction.
	Inputs []Input
	// All outputs of this transaction.
	Outputs []Output
}

type TransactionPool struct {
	// TransactionPool contains all pending transactions that haven't be checked in the blockchain.
	// Key is the hex of transaction's hash, value is the transaction.
	TxPool map[string]Transaction
}

// NewTransactionPool creates a new transaction pool with no transaction at all.
func NewTransactionPool() TransactionPool {
	return TransactionPool{
		TxPool: make(map[string]Transaction),
	}
}
