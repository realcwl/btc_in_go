package main

import (
	"log"

	"github.com/Luismorlan/btc_in_go/model"
	"github.com/Luismorlan/btc_in_go/utils"
)

func main() {
	l := model.NewLedger()
	l.L[model.UTXOLite{PrevTxHash: "prevhash", Index: 0}] = &model.Output{Value: 1.0, PublicKey: []byte{1, 1, 1}}
	nl := utils.GetLedgerDeepCopy(l)
	nnl := l
	log.Println("prev: ", l)
	log.Println("new: ", nl)
	log.Println("new new: ", nnl)
	log.Println("change something===")
	delete(l.L, model.UTXOLite{PrevTxHash: "prevhash", Index: 0})
	log.Println("prev: ", l)
	log.Println("new: ", nl)
	log.Println("new new: ", nnl)
}
