package utils

import (
	"encoding/binary"
	"encoding/hex"
	"math"
)

func BytesToHex(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func HexToBytes(str string) ([]byte, error) {
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func Int64ToBytes(i int64) []byte {
	b := make([]byte, 8)
	binary.PutVarint(b, i)
	return b
}

func Float64ToBytes(f float64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b[:], math.Float64bits(f))
	return b
}
