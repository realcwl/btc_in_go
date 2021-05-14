package main

import (
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"

	"github.com/Luismorlan/btc_in_go/utils"
	"github.com/Luismorlan/btc_in_go/wallet"
)

const LOAD_OP string = "load"
const SEND_OP string = "send"
const DEFAULT_FN_IP string = "127.0.0.1"
const DEFAULT_FN_PORT string = "8280"

var (
	fPathFlag  *string
	ipAddrFlag *string
	portFlag   *string
)

func init() {
	fPathFlag = flag.String("fPath", "", "RSA file path for your private key")
	ipAddrFlag = flag.String("ip", "127.0.0.1", "ip address of full node")
	portFlag = flag.String("port", "8280", "Port number for connection")
}

func main() {
	flag.Parse()
	fmt.Println("fPath is", *fPathFlag)
	fmt.Println("ipAddr is", *ipAddrFlag)
	fmt.Println("port is", *portFlag)

	wallet, err := parseToWallet(*fPathFlag, *ipAddrFlag, *portFlag)
	if err != nil {
		log.Println("[ERROR]Failed to get your wallet. Please check the input.")
		return
	}
	log.Println(wallet)

	// reader := bufio.NewReader(os.Stdin)

}

func parseToWallet(fPath string, ipAddr string, port string) (*wallet.Wallet, error) {
	var wallet wallet.Wallet
	var err error
	wallet.Keys, err = parseKeyFile(fPath)
	if err != nil {
		return nil, err
	}
	wallet.FullNodeIp = ipAddr
	wallet.FullNodePort = port
	return &wallet, nil
}

func parseKeyFile(fPath string) (*rsa.PrivateKey, error) {
	var userKey *rsa.PrivateKey
	var err error
	if fPath == "" || fPath == "\n" {
		log.Println("No key file detected. Will generate a new key for you")
		userKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal("Got error when generating new key", err)
			return nil, err
		}
	} else {
		// read from rsa file
		userKey, err = readKeyFromFPath(fPath)
		if err != nil {
			log.Fatal("Failed to read your key from path", fPath)
			return nil, err
		}
	}
	key := utils.BytesToHex(utils.PublicKeyToBytes(&userKey.PublicKey))

	fmt.Println(key)
	return userKey, err

}

func readKeyFromFPath(fPath string) (*rsa.PrivateKey, error) {
	return nil, nil
}
