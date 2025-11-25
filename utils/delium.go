package utils

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
)

type D_hash struct {
	Byte_slice []byte
	String     string
}

func D256(strData string, deleteStep int, repeat int) *D_hash {

	dataHash := sha256.Sum256([]byte(strData))
	var strDataHash string = hex.EncodeToString(dataHash[:])

	for i := 0; i < repeat; i++ {

		var result string = ""
		for r := 0; r < len(strDataHash); r++ {
			if (r+1)%deleteStep != 0 {
				result += string(strDataHash[r])
			}
		}

		hashByte32 := sha256.Sum256([]byte(result))
		strDataHash = hex.EncodeToString(hashByte32[:])
	}

	return &D_hash{
		Byte_slice: []byte(strDataHash),
		String:     strDataHash,
	}
}

func D512(strData string, deleteStep int, repeat int) *D_hash {

	dataHash := sha512.Sum512([]byte(strData))
	var strDataHash string = hex.EncodeToString(dataHash[:])

	for i := 0; i < repeat; i++ {

		var result string = ""
		for r := 0; r < len(strDataHash); r++ {
			if (r+1)%deleteStep != 0 {
				result += string(strDataHash[r])
			}
		}

		hashByte32 := sha512.Sum512([]byte(result))
		strDataHash = hex.EncodeToString(hashByte32[:])
	}

	return &D_hash{
		Byte_slice: []byte(strDataHash),
		String:     strDataHash,
	}
}

func D256C(strData string, path string) *D_hash {

	dataHash := sha256.Sum256([]byte(strData))
	var strDataHash string = hex.EncodeToString(dataHash[:])
	parts := strings.Split(path, "/")

	for _, part := range parts {
		d := strings.Split(part, "#")

		addonString := d[0]
		newString := strDataHash + addonString

		deleteStep, strconvErr := strconv.Atoi(d[1])
		if strconvErr != nil {
			fmt.Println("Error: see log/internal folder for details.")
			log.Fatal("Hashing path is incorrect!")
		}

		strDataHash = D256(newString, deleteStep, 1).String
	}

	return &D_hash{
		Byte_slice: []byte(strDataHash),
		String:     strDataHash,
	}
}

func D512C(strData string, path string) *D_hash {

	dataHash := sha512.Sum512([]byte(strData))
	var strDataHash string = hex.EncodeToString(dataHash[:])
	parts := strings.Split(path, "/")

	for _, part := range parts {
		d := strings.Split(part, "#")

		addonString := d[0]
		newString := strDataHash + addonString

		deleteStep, strconvErr := strconv.Atoi(d[1])
		if strconvErr != nil {
			fmt.Println("Error: see log/internal folder for details.")
			log.Fatal("Hashing path is incorrect!")
		}

		strDataHash = D512(newString, deleteStep, 1).String
	}

	return &D_hash{
		Byte_slice: []byte(strDataHash),
		String:     strDataHash,
	}
}
