package commands

import (
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const (
	// do nothing operation
	NOOP = iota
	// Initiate a money transfer from wallet
	TRANSFER
	// Print user public key
	MY_PK
	// Connect a full node with ip address and port
	CONNECT
	// Get my own balance
	GET_BALANCE
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
		value := c.Args[1]
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return err == nil && v > 0
	case MY_PK, GET_BALANCE:
		return len(c.Args) == 0
	case CONNECT:
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
	case "my_pk":
		cmd.Op = MY_PK
	case "connect":
		cmd.Op = CONNECT
	case "get_balance":
		cmd.Op = GET_BALANCE
	default:
		cmd.Op = NOOP
	}
	cmd.Args = ss[1:]
	if !cmd.IsValid() {
		return ClientCommand{}, errors.New("invalid command")
	}
	return cmd, nil
}
