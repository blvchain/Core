package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"blvchain/core/config"
	"github.com/tyler-smith/go-bip39"
)

// Custom print for errors with red color
func PrintError(format error, a ...interface{}) {
	redColor := "\033[31m"
	resetColor := "\033[0m"
	fmt.Printf(redColor+format.Error()+resetColor+"\n", a...)
}

// Generate random 12 words mnemonic
func Generate_mnemonic() string {
	entropy, _ := bip39.NewEntropy(config.MNEMONIC_STRENGTH)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	return mnemonic
}

func NowTimeInt64UnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func Rounder(f float64) int {
	return int(math.Ceil(f))
}

func Int64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

func Last24HoursUnixMilli() int64 {
	now := time.Now()
	oneHourAgo := now.Add(-24 * time.Hour)
	return oneHourAgo.UnixMilli()
}

func Next10DaysUnixMilli() int64 {
	now := time.Now()
	oneHourAgo := now.Add(10 * 24 * time.Hour)
	return oneHourAgo.UnixMilli()
}

func Past10MinsUnixMilli() int64 {
	now := time.Now()
	tenMinAgo := now.Add(-10 * time.Minute)
	return tenMinAgo.UnixMilli()
}

func StringToInt64(strNum string) int64 {
	num, _ := strconv.ParseInt(strNum, 10, 64)
	return num
}

func Float64ToString(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}

	// Convert the float to string with maximum precision
	s := strconv.FormatFloat(f, 'f', -1, 64)

	// If the float has more than 8 decimals, format it to 8 decimal places
	parts := strings.Split(s, ".")
	if len(parts) > 1 && len(parts[1]) > 8 {
		s = strconv.FormatFloat(f, 'f', 8, 64)
	}

	return s
}

func StringToFloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
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

func GenerateRandomHexString(len int) (string, error) {
	b := make([]byte, len)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
