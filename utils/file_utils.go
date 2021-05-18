package utils

import (
	"crypto/rsa"
	"io/ioutil"
	"log"
	"os"
)

// This function is called in startup time.
// ParseKeyFile returns key pair from the given path. This function will exit on any error
// because there's no need to continue if we cannot get key.
func ParseKeyFile(path string, rsaLen int) *rsa.PrivateKey {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		sk, _ := GenerateKeyPair(rsaLen)
		WritePrivateKeyToFile(sk, path)
		return sk
	}
	sk := ReadKeyFromPath(path)
	return sk
}

// Write the given private key into file in bytes. Exit if fail to write
func WritePrivateKeyToFile(sk *rsa.PrivateKey, path string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatalln("fail to write sk: " + err.Error())
	}
	defer f.Close()
	_, err = f.Write(PrivateKeyToBytes(sk))
	if err != nil {
		log.Fatalln("fail to write sk: " + err.Error())
	}
}

// Read key from the given path. If the key is invalid, exit the execution
// because there is no need to continue.
func ReadKeyFromPath(path string) *rsa.PrivateKey {
	data, err := ioutil.ReadFile(path)
	if err != nil || len(data) == 0 {
		log.Fatalln("fail to read private key from path: " + path)
	}
	sk := BytesToPrivateKey(data)
	return sk
}
