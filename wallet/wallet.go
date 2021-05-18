package wallet

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/service"
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/jroimartin/gocui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// User signs and sends transactions to network.
// We don't need any mutex to protect the private members of this Wallet because all
// operations are linear. No concurrent operation is supported.
type Wallet struct {
	// The credential (sk, pk) of this wallet.
	keys *rsa.PrivateKey
	// The client to connect to FullNode server.
	client service.FullNodeServiceClient
	// gRPC connection this client has.
	conn *grpc.ClientConn
	// The balance. Updated every Transfer and GetBalance.
	UTXOs map[model.UTXOLite]*model.Output

	// A command fancy place to put output.
	g *gocui.Gui
}

// Return my public key in hex string.
func (w *Wallet) GetPublicKey() string {
	return utils.BytesToHex(utils.PublicKeyToBytes(&w.keys.PublicKey))
}

func (w *Wallet) GetTotalDeposit() (float64, error) {
	err := w.GetBalance()
	var v float64 = 0
	if err != nil {
		return v, err
	}
	for _, output := range w.UTXOs {
		v += output.GetValue()
	}
	return v, nil
}

func (w *Wallet) SetFullNodeConnection(ipAddr string, port string) error {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	serverAddr := ipAddr + ":" + port
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return err
	}
	if w.conn != nil {
		w.conn.Close()
	}
	w.conn = conn
	w.client = service.NewFullNodeServiceClient(conn)

	// Spin up a process that GC idle connection, we GC the client in a
	// expo backoff way to avoid overloading any peer.
	go func() {
		// Retry 3 times in total.
		retry := 3
		// How many times we already tried.
		try := 0
		// Retry every 3 seconds.
		base := 3
		for {
			time.Sleep(time.Duration(base) * time.Second)
			if conn.GetState() == connectivity.Ready {
				// Reset on any successful retry.
				try = 0
				base = 3
				continue
			}
			try++
			// Exponential backoff for retry.
			base *= 2
			if try >= retry {
				// If we already tried enough times, we just break and reclaim the connection.
				break
			}
		}
		w.Log(fmt.Sprintf("close dead fullnode connection: %s:%s", ipAddr, port))
	}()

	return nil
}

// Blocking call to get balance of current public key. The balance is represented
// as a list of UTXO and corresponding outputs.
func (w *Wallet) GetBalance() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pk := utils.PublicKeyToBytes(&w.keys.PublicKey)
	if w.client == nil || w.conn.GetState() != connectivity.Ready {
		return errors.New("no available connection to fullnode, fullnode might shutdown or unstable network")
	}
	res, err := w.client.GetBalance(ctx, &service.GetBalanceRequest{PublicKey: pk})
	if err != nil {
		return err
	}
	// Create an entire new balance to overwrite the current balance.
	balance := make(map[model.UTXOLite]*model.Output)
	for _, pair := range res.GetUtxoOutputPairs() {
		utxoLite := model.UTXOLite{
			PrevTxHash: pair.Utxo.PrevTxHash,
			Index:      pair.Utxo.Index,
		}
		balance[utxoLite] = pair.Output
	}
	w.UTXOs = balance
	return nil
}

func (w *Wallet) TransferMoney(receiver string, value float64) error {
	err := w.GetBalance()
	if err != nil {
		return err
	}
	receiverPk, err := utils.HexToBytes(receiver)
	if err != nil {
		return err
	}
	output := &model.Output{
		PublicKey: receiverPk,
		Value:     value,
	}
	tx, err := utils.CreatePendingTransaction(w.keys, w.UTXOs, []*model.Output{output})
	if err != nil {
		return err
	}
	err = w.SendTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wallet) SendTransaction(tx *model.Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if w.client == nil || w.conn.GetState() != connectivity.Ready {
		return errors.New("no available connection to fullnode, fullnode might shutdown or unstable network")
	}
	_, err := w.client.SetTransaction(ctx, &service.SetTransactionRequest{Tx: tx})
	if err != nil {
		return err
	}
	return nil
}

// Log the message. If the GUI is not nil, log to GUI, otherwise log to stdout.
func (w *Wallet) Log(s string) {
	if w.g == nil {
		log.Println(s)
		return
	}
	w.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("logger")
		if err != nil {
			log.Fatalln("fail to create logger, exit")
		}
		fmt.Fprintln(v, s)
		return nil
	})
}

// Create a new wallet from given credentials.
func NewWallet(path string, g *gocui.Gui) *Wallet {
	wallet := &Wallet{
		UTXOs: make(map[model.UTXOLite]*model.Output),
		// TODO: refactor this into a client config.
		keys: utils.ParseKeyFile(path, 304),
		g:    g,
	}

	return wallet
}
