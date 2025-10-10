package utils

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/url"
	"os"
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

func StringToInt(strNum string) int {
	num, _ := strconv.Atoi(strNum)
	return num
}

func StringToFloat64(strNum string) float64 {
	num, _ := strconv.ParseFloat(strNum, 64)
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
	hash := D512(pubkey_str, config.DELIUM_CONFIG.MESSAGE.DELETE_STEP, config.DELIUM_CONFIG.MESSAGE.REPEAT).String
	return hash[:32]
}

func StringSizeInKB(s string) float64 {
	bytes := len(s)
	kbSize := float64(bytes) / 1024

	return kbSize
}

// validators
// Between
func Bt_int(inputData int, gt int, ls int) bool {
	if inputData >= gt && inputData <= ls {
		return false
	} else {
		return true
	}
}

func Bt_int64(inputData int64, gt int64, ls int64) bool {
	if inputData >= gt && inputData <= ls {
		return false
	} else {
		return true
	}
}

func Bt_str(inputData string, gt int, ls int) bool {
	if len(inputData) >= gt && len(inputData) <= ls {
		return false
	} else {
		return true
	}
}

// Greater than
func Gt_int(inputData int, gt int) bool {
	if inputData > gt {
		return false
	} else {
		return true
	}
}

func Gt_int64(inputData int64, gt int64) bool {
	if inputData > gt {
		return false
	} else {
		return true
	}
}

func Gt_str(inputData string, gt int) bool {
	if len(inputData) > gt {
		return false
	} else {
		return true
	}
}

// Lesser than
func Lt_float(inputData float64, ls float64) bool {
	if inputData < ls {
		return false
	} else {
		return true
	}
}

func Lt_int(inputData int, ls int) bool {
	if inputData < ls {
		return false
	} else {
		return true
	}
}

func Lt_int64(inputData int64, ls int64) bool {
	if inputData < ls {
		return false
	} else {
		return true
	}
}

func Lt_str(inputData string, ls int) bool {
	if len(inputData) < ls {
		return false
	} else {
		return true
	}
}

// Equal

func E_str(inputData string, e int) bool {
	if len(inputData) == e {
		return false
	} else {
		return true
	}
}

func BoolCheck(inputData bool) bool {
	var data interface{} = inputData
	if _, ok := data.(bool); ok {
		return false
	} else {
		return true
	}
}

func FileCheckSumSHA256(fileName string) bool {
	path := config.SMART_CONTRACT_FILES_PATH + fileName
	f, err := os.Open(path)
	if err != nil {
		logger.SC_F_LOGGER.Printf("Error: Error in opening file %v, in path %v for checksum sha256: %v", fileName, path, err)
		return false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logger.SC_F_LOGGER.Printf("Error: Error in opening file %v, in path %v for checksum sha256: %v", fileName, path, err)
		return false
	}

	return hex.EncodeToString(h.Sum(nil)) == fileName
}
