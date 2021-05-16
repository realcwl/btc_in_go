package commands

import (
	"errors"
	"strings"
)

const PK_REGEX = ""
const VALUE_REGEX = ""

const (
	// Initiate a money transfer from wallet
	TRANSFER = iota
	// Print user public key
	MY_PUBLIC_KEY
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
		return true
		// receiverPK := c.Args[0]
		// value := c.Args[1]
		// pkRegex, _ := regexp.Compile(PK_REGEX)
		// valueRegex, _ := regexp.Compile(VALUE_REGEX)
		// return pkRegex.Match([]byte(receiverPK)) && valueRegex.Match([]byte(value))
	case MY_PUBLIC_KEY:
		return len(c.Args) == 0
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
	}
	cmd.Args = ss[1:]
	if !cmd.IsValid() {
		return ClientCommand{}, errors.New("invalid command")
	}
	return cmd, nil
}
