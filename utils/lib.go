package utils

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

func Int64ToBytes(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
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
		if item.UID == nodeUID && !bytes.Equal([]byte(item.UID), config.SELF_UID.Data) {
			return true
		}
	}
	return false
}

func Data_to_JSON(data any) []byte {
	byte_data, _ := json.Marshal(data)
	return byte_data
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
	path := config.SMART_CONTRACT_UPLOAD_PATH + fileName
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

func ToMongoBinary(b []byte) primitive.Binary {
	if len(b) == 0 {
		return primitive.Binary{}
	}
	return primitive.Binary{
		Data:    b,
		Subtype: 0x00,
	}
}

func ByteToHexString(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}

func HexStringToBytes(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("hex string must be even length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		b1 := HexValue(s[i*2])
		b2 := HexValue(s[i*2+1])
		if b1 < 0 || b2 < 0 {
			return nil, errors.New("invalid hex")
		}
		out[i] = byte((b1 << 4) | b2)
	}
	return out, nil
}

func HexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}

func Make_UID(pubkey primitive.Binary) (D_hash, error) {
	// Step 1. Perform binary hashing
	h, err := D256C(pubkey, config.DELIUM_CONFIG.UID_MAKER_PATH)
	if err != nil {
		return D_hash{}, err
	}

	// Step 2. Convert to hex
	fullHex := h.String

	if len(fullHex) < 32 {
		return D_hash{}, errors.New("hash too short, unexpected")
	}

	// Step 3. First 32 hex characters (this is your address string)
	addrHex := fullHex[:32]

	// Step 4. Decode those 32 hex characters back into 16 raw bytes
	addrBytes, err := HexStringToBytes(addrHex)
	if err != nil {
		return D_hash{}, err
	}

	// Step 5. Wrap in D_hash like every other Delium output
	return D_hash{
		Byte_slice:       addrBytes,
		String:           addrHex,
		Primitive_binary: ToMongoBinary(addrBytes),
	}, nil
}
