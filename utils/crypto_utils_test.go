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

func TestHash(t *testing.T) {
	b := []byte{1, 2, 3}
	hash := SHA256(b)
	hs := BytesToHex(hash)
	assert.Equal(t, "039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81", hs)
}
