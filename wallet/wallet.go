package wallet

import (
	"context"
	"crypto/rsa"
	"log"
	"time"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
	"google.golang.org/grpc"
)

// User signs and sends transactions to network.
type Wallet struct {
	Keys           *rsa.PrivateKey
	FullNodeClient service.FullNodeServiceClient
	UTXOs          map[model.UTXOLite]*model.Output
}

func (w *Wallet) SetFullNodeConnection(ipAddr string, port string) error {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	serverAddr := ipAddr + ":" + port
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		log.Fatal("failed to dial", serverAddr, err)
		return err
	}
	// TODO : Add a callback to terminate connection at end
	w.FullNodeClient = service.NewFullNodeServiceClient(conn)
	return nil
}

func (w *Wallet) GetBalance() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pk := utils.PublicKeyToBytes(&w.Keys.PublicKey)
	response, err := w.FullNodeClient.GetBalance(ctx, &service.GetBalanceRequest{PublicKey: pk})
	if err != nil {
		return err
	}
	for _, pair := range response.GetUtxoOutputPairs() {
		utxoLite := model.UTXOLite{
			PrevTxHash: pair.Utxo.PrevTxHash,
			Index:      pair.Utxo.Index,
		}
		w.UTXOs[utxoLite] = pair.Output
	}
	return nil
}

func (w *Wallet) TransferMoney(receiverPK string, value float64) error {
	err1 := w.GetBalance()
	if err1 != nil {
		log.Println("failed to get balance from full node", err1)
		return err1
	}
	pk, err2 := utils.HexToBytes(receiverPK)
	if err2 != nil {
		log.Println("failed to parse receiverPK", err2)
		return err2
	}
	output := &model.Output{
		PublicKey: pk,
		Value:     value,
	}
	tx, err3 := CreatePendingTransaction(w, []*model.Output{output})
	if err3 != nil {
		log.Println("failed to create new transaction", err3)
		return err3
	}
	err4 := w.SendTransaction(tx)
	if err4 != nil {
		log.Println("failed to send transaction to full node", err4)
		return err4
	}
	return nil
}

func (w *Wallet) SendTransaction(tx *model.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := w.FullNodeClient.SetTransaction(ctx, &service.SetTransactionRequest{Tx: tx})
	if err != nil {
		return err
	}
	return nil
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
	for utxo := range wallet.UTXOs {
		input := &model.Input{
			PrevTxHash: utxo.PrevTxHash,
			Index:      utxo.Index,
		}

		inputs = append(inputs, input)
		totalValue += float64(wallet.UTXOs[utxo].Value)
	}
	// Total amount of money will be transferred to others
	var totalTransferValue = 0.0
	for i := 0; i < len(outputs); i++ {
		totalTransferValue += float64(outputs[i].Value)
	}

	// Output with amount of money left after transfer
	selfOutput := model.Output{
		Value:     (totalValue - totalTransferValue),
		PublicKey: utils.PublicKeyToBytes(&wallet.Keys.PublicKey),
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
		inputs[i].Signature, err = utils.Sign(toSignMsg, wallet.Keys)
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
