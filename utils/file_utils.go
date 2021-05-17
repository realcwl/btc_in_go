package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"os"
)

func ParseKeyFile(fPath string, createNewKey string) (*rsa.PrivateKey, error) {
	var userKey *rsa.PrivateKey
	var err error
	if fPath == "" {
		return nil, errors.New("file path is missing")
	}
	// Generate new key and save to given path
	if createNewKey == "t" {
		log.Println("Generating a new key")
		userKey, err = rsa.GenerateKey(rand.Reader, 304)
		if err != nil {
			log.Fatal("Got error when generating new key", err)
			return nil, err
		}
		err2 := SavePrivateKeyToFile(userKey, fPath)
		if err2 != nil {
			log.Fatal("Got error when saving new key", err2)
			return nil, err2
		}
		return userKey, nil
	}
	// Read key from exsiting rsa file
	userKey, err = ReadKeyFromFPath(fPath)
	if err != nil {
		log.Printf("Failed to read your key from path %s with error %s", fPath, err)
		return nil, err
	}
	return userKey, nil
}

func SavePrivateKeyToFile(privkey *rsa.PrivateKey, fpath string) error {
	f, err := os.Create(fpath)
	if err != nil {
		log.Println("failed to open file", fpath, err)
		return err
	}
	defer f.Close()
	_, err2 := f.WriteString(string(PrivateKeyToBytes(privkey)))
	if err2 != nil {
		log.Println("failed to save key in", fpath, err2)
		return err2
	}
	log.Println("Saved private key in file", fpath)

	return nil
}

func ReadKeyFromFPath(fPath string) (*rsa.PrivateKey, error) {
	fileContent, err := ioutil.ReadFile(fPath)
	if err != nil {
		return nil, err
	}
	if len(fileContent) == 0 {
		log.Println("File is empty, please check filepath.")
		return nil, nil
	}
	block, _ := pem.Decode(fileContent)
	key, _ := x509.ParsePKCS1PrivateKey(block.Bytes)
	return key, nil
}
