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
	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/Luismorlan/btc_in_go/wallet"
)

const DEFAULT_FN_IP string = "127.0.0.1"
const DEFAULT_FN_PORT string = "8280"

var (
	fPathFlag        *string
	createNewKeyFlag *string
	ipAddrFlag       *string
	portFlag         *string
)

func init() {
	fPathFlag = flag.String("fPath", "", "RSA file path for your private key")
	createNewKeyFlag = flag.String("newkey", "f", "whether to create a new private key")
	ipAddrFlag = flag.String("ip", DEFAULT_FN_IP, "ip address of full node")
	portFlag = flag.String("port", DEFAULT_FN_PORT, "Port number for connection")
}

func main() {
	flag.Parse()
	fmt.Println("fPath is", *fPathFlag)
	fmt.Println("createNewKey is", *createNewKeyFlag)
	fmt.Println("ipAddr is", *ipAddrFlag)
	fmt.Println("port is", *portFlag)

	wallet, err := parseToWallet(*fPathFlag, *createNewKeyFlag, *ipAddrFlag, *portFlag)
	if err != nil {
		log.Println("[ERROR]Failed to log in your wallet. Please check the input.")
		return
	}
	log.Println("Wallet logged in")
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
			if wallet.FullNodeClient == nil {
				log.Println("Full node connection is missing. Please use command connect to set up connection first")
				continue
			}
			receiverPK := c.Args[0]
			value, err1 := strconv.ParseFloat(c.Args[1], 64)
			if err1 != nil {
				log.Println("cannot transfer with value", c.Args[1], err1)
				continue
			}
			err2 := wallet.TransferMoney(receiverPK, value)
			if err2 != nil {
				log.Println("Money transfer failed", err2)
				continue
			}
			log.Println("Money transfer successfully sent to full node")
		case commands.MY_PUBLIC_KEY:
			fmt.Println(utils.BytesToHex((utils.PublicKeyToBytes(&wallet.Keys.PublicKey))))
		case commands.CONNECT_FULL_NODE:
			ipAddr := c.Args[0]
			port := c.Args[1]
			err := wallet.SetFullNodeConnection(ipAddr, port)
			if err != nil {
				log.Printf("failed to connect to full node address %s", ipAddr+":"+port)
				continue
			}
			log.Printf("Full node %s connected", ipAddr+":"+port)
		default:
			log.Println("Unhandled command:", c)
		}
		fmt.Print("> ")
	}
}

func parseToWallet(fPath string, createNewKey string, ipAddr string, port string) (*wallet.Wallet, error) {
	var wallet wallet.Wallet
	var err error
	err = wallet.SetFullNodeConnection(ipAddr, port)
	if err != nil {
		return &wallet, err
	}
	wallet.Keys, err = utils.ParseKeyFile(fPath, createNewKey)
	return &wallet, err
}
