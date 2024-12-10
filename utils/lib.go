package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
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

func ReplaceHTTPWithWS(url string) string {
	url = strings.Replace(url, "https://", "ws://", 1)
	url = strings.Replace(url, "http://", "ws://", 1)
	return url
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
