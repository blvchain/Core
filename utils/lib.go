package utils

import (
	"blvchain/core/config"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"strconv"
	"time"
)

// Custom print for errors with red color
func PrintError(format error, a ...interface{}) {
	redColor := "\033[31m"
	resetColor := "\033[0m"
	fmt.Printf(redColor+format.Error()+resetColor+"\n", a...)
}

func NowTimeInt64UnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func Int64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

func StringToInt64(strNum string) int64 {
	num, _ := strconv.ParseInt(strNum, 10, 64)
	return num
}

func privkeyHexToECDSA(privkey string) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = curve
	privateKeyBigInt, _ := new(big.Int).SetString(privkey, 16)
	privateKey.D = privateKeyBigInt

	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(privateKey.D.Bytes())

	return privateKey, nil
}

func AddQueryParams(baseURL string, params map[string]string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()

	for key, value := range params {
		q.Set(key, value)
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}

func NodeUidChecker(nodeUID string) bool {
	for _, item := range config.DNS_SEED_LIST {
		if item.UID == nodeUID {
			return true
		}
	}
	return false
}

func Data_to_JSON(data any) []byte {
	byte_data, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error marshalling to JSON: %v", err)
	}
	return byte_data
}

func Make_UID(pubkey_str string) string {
	hash := D512(pubkey_str, config.DELIUM_CONFIG.MESSAGE.DELETE_STEP, config.DELIUM_CONFIG.HASH.REPEAT).String
	return hash[:32]
}
