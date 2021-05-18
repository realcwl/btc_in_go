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
	"github.com/Luismorlan/btc_in_go/layout"
	"github.com/Luismorlan/btc_in_go/wallet"
	"github.com/jroimartin/gocui"
)

var (
	keyPath   *string
	debugMode *bool
)

func init() {
	keyPath = flag.String("key_path", "/tmp/mykey.pem", "RSA file path for your private key")
	debugMode = flag.Bool("debug_mode", false, "Using debug mode will disable fancy GUI.")
}

// Return a gui handle if not in debug mode.
func ListenOnInput(cmd chan commands.ClientCommand, debugMode bool) *gocui.Gui {
	// Choose a fancy GUI
	if debugMode {
		go ParseCommand(cmd)
		return nil
	}
	g, err := layout.CreateGui(cmd, "wallet/cmd/usage.txt")
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		if err := g.MainLoop(); err != nil {
			if err == gocui.ErrQuit {
				g.Close()
				os.Exit(0)
			}
			os.Exit(1)
		}
	}()
	return g

}

func main() {
	flag.Parse()
	fmt.Println("keyPath is", *keyPath)

	cmd := make(chan commands.ClientCommand)
	// Start listening on input.
	g := ListenOnInput(cmd, *debugMode)
	wallet := wallet.NewWallet(*keyPath, g)
	wallet.Log("Wallet public key: " + wallet.GetPublicKey())

	go HandleCommand(cmd, wallet)

	c := make(chan int)
	<-c
}

// Parse command from stdio.
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
				wallet.Log("fail to transfer money: " + err.Error())
				continue
			}
			wallet.Log(fmt.Sprintf("successfully send transaction to fullnode, receiver: %s, value: %f", receiverPK, value))
		case commands.MY_PK:
			wallet.Log("\n===============DO NOT COPY THIS LINE================\n" + wallet.GetPublicKey() + "\n===============DO NOT COPY THIS LINE================")
		case commands.CONNECT:
			ipAddr := c.Args[0]
			port := c.Args[1]
			err := wallet.SetFullNodeConnection(ipAddr, port)
			if err != nil {
				wallet.Log("failed to connect to full node endpoint " + ipAddr + ":" + port)
				continue
			}
			wallet.Log("connected full node endpoint " + ipAddr + ":" + port)
		case commands.GET_BALANCE:
			v, err := wallet.GetTotalDeposit()
			if err != nil {
				wallet.Log("fail to get balance: " + err.Error())
				continue
			}
			wallet.Log(fmt.Sprintf("your total balance is: %f", v))
		default:
			wallet.Log(fmt.Sprintf("Unimplemented command: %d", c.Op))
		}
	}
}
