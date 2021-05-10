package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const KEY_BITS = 2048

func TestSignatureAndVerify(t *testing.T) {
	sk, pk := GenerateKeyPair(KEY_BITS)

	message := []byte("Hello World!")
	sig, err := Sign(message, sk)
	assert.Nil(t, err)
	valid := Verify(message, pk, sig)
	assert.True(t, valid)
}
