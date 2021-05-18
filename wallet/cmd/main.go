package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Luismorlan/btc_in_go/commands"
	"github.com/Luismorlan/btc_in_go/wallet"
)

var (
	keyPath *string
	ipAddr  *string
	port    *string
)

func init() {
	keyPath = flag.String("key_path", "/tmp/mykey.pem", "RSA file path for your private key")
	ipAddr = flag.String("ip_addr", "127.0.0.1", "Fullnode's IPv4 address")
	port = flag.String("port", "10010", "Fullnode's TCP port number for connection")
}

func main() {
	flag.Parse()
	fmt.Println("keyPath is", *keyPath)

	wallet := wallet.NewWallet(*keyPath, *ipAddr, *port)
	fmt.Println("connected to fullnode endpoint: ", *ipAddr+":"+*port)

	cmd := make(chan commands.ClientCommand)
	go ParseCommand(cmd)
	go HandleCommand(cmd, wallet)

	c := make(chan int)
	<-c
}

func ParseCommand(cmd chan commands.ClientCommand) {
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		// convert CRLF to LF
		text = strings.Replace(text, "\n", "", -1)
		c, err := commands.CreateClientCommand(text)
		if err != nil {
			log.Println(err)
			continue
		}
		cmd <- c
	}
}

func HandleCommand(cmd chan commands.ClientCommand, wallet *wallet.Wallet) {
	for {
		c := <-cmd
		switch c.Op {
		case commands.TRANSFER:
			receiverPK := c.Args[0]
			value, _ := strconv.ParseFloat(c.Args[1], 64)
			err := wallet.TransferMoney(receiverPK, value)
			if err != nil {
				log.Println("fail to transfer money: ", err)
				continue
			}
			log.Printf("successfully send transaction to fullnode, receiver: %s, value: %f", receiverPK, value)
		case commands.MY_PK:
			fmt.Println(wallet.GetPublicKey())
			/*
				case commands.CONNECT_FULL_NODE:

						ipAddr := c.Args[0]
						port := c.Args[1]
						wallet.SetFullNodeConnection(ipAddr, port)
						if err != nil {
							log.Printf("failed to connect to full node address %s", ipAddr+":"+port)
							continue
						}
						log.Printf("Full node %s connected", ipAddr+":"+port)
			*/
		case commands.GET_BALANCE:
			v, err := wallet.GetTotalDeposit()
			if err != nil {
				log.Println("fail to get balance: " + err.Error())
				continue
			}
			log.Println("your total balance is: ", v)
		default:
			log.Println("Unhandled command:", c)
		}
	}
}
