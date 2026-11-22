package utils

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PrintError prints an error in red. Kept for the console output style used by
// the node.
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

// MakeUID derives the 32-character node/client identifier from a compressed
// public key. It is deterministic: the same key always maps to the same UID.
func Make_UID(pubkey_str string) string {
	hash := D512C(pubkey_str, config.DELIUM_CONFIG.UID_MAKER_PATH).String
	if len(hash) < 32 {
		return hash
	}
	return hash[:32]
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

// NodeUidChecker reports whether nodeUID belongs to a *different* seed node than
// this one. It is used to authorise inbound WebSocket peers.
func NodeUidChecker(nodeUID string) bool {
	for _, item := range config.DNS_SEED_LIST {
		if item.UID == nodeUID && item.UID != config.SELF_UID {
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
	return float64(len(s)) / 1024
}

// SHA256Hex returns the lower-case hex SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IsHex reports whether s is a non-empty, even-length hex string.
func IsHex(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// FileSHA256 returns the hex SHA-256 digest of the file at path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileCheckSumSHA256 reports whether the file named `checksum` inside the
// smart-contract directory hashes to that name.
func FileCheckSumSHA256(fileName string) bool {
	path := filepath.Join(config.SMART_CONTRACT_DIR, fileName)
	sum, err := FileSHA256(path)
	if err != nil {
		logger.SC_F_LOGGER.Printf("Error: could not read file %v at %v for sha256 checksum: %v", fileName, path, err)
		return false
	}
	return strings.EqualFold(sum, fileName)
}

// ---------------------------------------------------------------------------
// Tiny validation helpers
//
// Every helper returns true when the value is INVALID, which keeps the call
// sites readable:
//
//	if utils.E_str(value, 32) { return errors.New("...") }
// ---------------------------------------------------------------------------

// Bt_int reports whether inputData is outside [gt, ls] (inclusive).
func Bt_int(inputData int, gt int, ls int) bool { return !(inputData >= gt && inputData <= ls) }

// Bt_int64 reports whether inputData is outside [gt, ls] (inclusive).
func Bt_int64(inputData int64, gt int64, ls int64) bool { return !(inputData >= gt && inputData <= ls) }

// Bt_str reports whether len(inputData) is outside [gt, ls] (inclusive).
func Bt_str(inputData string, gt int, ls int) bool {
	return !(len(inputData) >= gt && len(inputData) <= ls)
}

// Gt_int reports whether inputData is not greater than gt.
func Gt_int(inputData int, gt int) bool { return !(inputData > gt) }

// Gt_int64 reports whether inputData is not greater than gt.
func Gt_int64(inputData int64, gt int64) bool { return !(inputData > gt) }

// Gt_str reports whether len(inputData) is not greater than gt.
func Gt_str(inputData string, gt int) bool { return !(len(inputData) > gt) }

// Lt_float reports whether inputData is not smaller than ls.
func Lt_float(inputData float64, ls float64) bool { return !(inputData < ls) }

// Lt_int reports whether inputData is not smaller than ls.
func Lt_int(inputData int, ls int) bool { return !(inputData < ls) }

// Lt_int64 reports whether inputData is not smaller than ls.
func Lt_int64(inputData int64, ls int64) bool { return !(inputData < ls) }

// Lt_str reports whether len(inputData) is not smaller than ls.
func Lt_str(inputData string, ls int) bool { return !(len(inputData) < ls) }

// E_str reports whether len(inputData) differs from e.
func E_str(inputData string, e int) bool { return len(inputData) != e }
