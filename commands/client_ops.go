package commands

import (
	"errors"
	"net"
	"regexp"
	"strings"
)

const PK_REGEX = "[0-9|a-f]{136}"
const VALUE_REGEX = "[0-9]+[.]?[0-9]*"

const (
	// do nothing operation
	NOOP = iota
	// Initiate a money transfer from wallet
	TRANSFER
	// Print user public key
	MY_PUBLIC_KEY
	// Connect a full node with ip address and port
	CONNECT_FULL_NODE
)

type ClientCommand struct {
	Op   Operation
	Args []string
}

func (c ClientCommand) IsValid() bool {
	switch c.Op {
	case TRANSFER:
		if len(c.Args) != 2 {
			return false
		}
		receiverPK := c.Args[0]
		value := c.Args[1]
		pkRegex, _ := regexp.Compile(PK_REGEX)
		valueRegex, _ := regexp.Compile(VALUE_REGEX)
		return pkRegex.Match([]byte(receiverPK)) && valueRegex.Match([]byte(value))
	case MY_PUBLIC_KEY:
		return len(c.Args) == 0
	case CONNECT_FULL_NODE:
		if len(c.Args) != 2 {
			return false
		}
		ipAddr := c.Args[0]
		port := c.Args[1]
		ip := net.ParseIP(ipAddr)

		portRegex, _ := regexp.Compile(PORT_REGEX)
		return ip != nil && ip.To4() != nil && portRegex.Match([]byte(port))
	default:
		return false
	}
}

func CreateClientCommand(s string) (ClientCommand, error) {
	// split command by space.
	ss := strings.Split(s, " ")
	if len(ss) == 0 {
		return ClientCommand{}, errors.New("command is empty")
	}
	cmd := ClientCommand{}
	switch ss[0] {
	case "transfer":
		cmd.Op = TRANSFER
	case "mypk":
		cmd.Op = MY_PUBLIC_KEY
	case "connect":
		cmd.Op = CONNECT_FULL_NODE
	default:
		cmd.Op = NOOP
	}
	cmd.Args = ss[1:]
	if !cmd.IsValid() {
		return ClientCommand{}, errors.New("invalid command")
	}
	return cmd, nil
}
