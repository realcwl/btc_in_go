package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCoinbase(t *testing.T) {
	_, pk := GenerateKeyPair(2048)
	cb := CreateCoinbaseTx(1.0, PublicKeyToBytes(pk))
	assert.Nil(t, IsValidCoinbase(cb, 1.0))
}
