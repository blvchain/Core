package utils

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"blvchain/core/config"
	"blvchain/core/logger"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.DNS_SEED_LIST = config.GetDnsSeedListFile()

	code := m.Run()
	os.Exit(code)
}

func TestNumberHelpers(t *testing.T) {
	if got := Int64ToStr(-42); got != "-42" {
		t.Errorf("Int64ToStr(-42) = %q", got)
	}
	if got := StringToInt64("1720108080000"); got != 1720108080000 {
		t.Errorf("StringToInt64 = %d", got)
	}
	if got := StringToInt("not a number"); got != 0 {
		t.Errorf("StringToInt on garbage = %d, want 0", got)
	}
	if got := StringToFloat64("0.1"); got != 0.1 {
		t.Errorf("StringToFloat64 = %v", got)
	}
}

func TestNowTimeInt64UnixMilli(t *testing.T) {
	before := time.Now().UTC().UnixMilli()
	got := NowTimeInt64UnixMilli()
	after := time.Now().UTC().UnixMilli()

	if got < before || got > after {
		t.Fatalf("NowTimeInt64UnixMilli() = %d, want between %d and %d", got, before, after)
	}
	if got < config.MIN_TIMESTAMP_MS || got > config.MAX_TIMESTAMP_MS {
		t.Fatalf("%d is outside the accepted timestamp window", got)
	}
}

func TestStringSizeInKB(t *testing.T) {
	if got := StringSizeInKB(""); got != 0 {
		t.Errorf("empty string size = %v", got)
	}
	if got := StringSizeInKB(string(make([]byte, 2048))); got != 2 {
		t.Errorf("2048 bytes = %v KB, want 2", got)
	}
}

func TestSHA256Hex(t *testing.T) {
	// echo -n "abc" | sha256sum
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256Hex([]byte("abc")); got != want {
		t.Fatalf("SHA256Hex(abc) = %q, want %q", got, want)
	}
}

func TestIsHex(t *testing.T) {
	cases := map[string]bool{
		"":                 false,
		"0":                false, // odd length
		"zz":               false,
		"00":               true,
		"0123456789abcdef": true,
		"0123456789abcdeg": false,
	}
	for input, want := range cases {
		if got := IsHex(input); got != want {
			t.Errorf("IsHex(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFileHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.wasm")
	content := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatalf("FileSHA256: %v", err)
	}
	if sum != SHA256Hex(content) {
		t.Fatalf("FileSHA256 = %q, want %q", sum, SHA256Hex(content))
	}

	original := config.SMART_CONTRACT_DIR
	config.SMART_CONTRACT_DIR = dir
	defer func() { config.SMART_CONTRACT_DIR = original }()

	if err := os.WriteFile(filepath.Join(dir, sum), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !FileCheckSumSHA256(sum) {
		t.Fatal("FileCheckSumSHA256 rejected a file whose name matches its digest")
	}
	if FileCheckSumSHA256(SHA256Hex([]byte("something else"))) {
		t.Fatal("FileCheckSumSHA256 accepted a mismatched file")
	}
	if FileCheckSumSHA256("does-not-exist") {
		t.Fatal("FileCheckSumSHA256 accepted a missing file")
	}
}

func TestAddQueryParams(t *testing.T) {
	got, err := AddQueryParams("ws://localhost:8080/path", map[string]string{"uid": "abc", "n": "2"})
	if err != nil {
		t.Fatalf("AddQueryParams: %v", err)
	}
	if got != "ws://localhost:8080/path?n=2&uid=abc" {
		t.Fatalf("AddQueryParams = %q", got)
	}
}

func TestNodeUidChecker(t *testing.T) {
	originalSeeds := config.DNS_SEED_LIST
	originalSelf := config.SELF_UID
	defer func() {
		config.DNS_SEED_LIST = originalSeeds
		config.SELF_UID = originalSelf
	}()

	config.SELF_UID = "node_self"
	config.DNS_SEED_LIST = []config.Dns_seed_config{
		{UID: "node_self", Address: "ws://localhost:8081"},
		{UID: "node_other", Address: "ws://localhost:8082"},
	}

	if NodeUidChecker("node_self") {
		t.Error("a node must not accept itself as a peer")
	}
	if !NodeUidChecker("node_other") {
		t.Error("a listed peer must be accepted")
	}
	if NodeUidChecker("node_unknown") {
		t.Error("an unknown node must be rejected")
	}
}

func TestDataToJSON(t *testing.T) {
	got := string(Data_to_JSON(map[string]int{"a": 1}))
	if got != `{"a":1}` {
		t.Fatalf("Data_to_JSON = %q", got)
	}
}

func TestValidators(t *testing.T) {
	type check struct {
		name string
		got  bool // true means INVALID
		want bool
	}

	cases := []check{
		{"Bt_int in range", Bt_int(5, 1, 10), false},
		{"Bt_int below", Bt_int(0, 1, 10), true},
		{"Bt_int above", Bt_int(11, 1, 10), true},
		{"Bt_int64 in range", Bt_int64(5, 1, 10), false},
		{"Bt_int64 out", Bt_int64(50, 1, 10), true},
		{"Bt_str in range", Bt_str("abc", 1, 5), false},
		{"Bt_str out", Bt_str("abcdef", 1, 5), true},
		{"Gt_int above", Gt_int(6, 5), false},
		{"Gt_int equal", Gt_int(5, 5), true},
		{"Gt_int64 above", Gt_int64(6, 5), false},
		{"Gt_str above", Gt_str("abcdef", 5), false},
		{"Gt_str equal", Gt_str("abcde", 5), true},
		{"Lt_float below", Lt_float(0.5, 1), false},
		{"Lt_float equal", Lt_float(1, 1), true},
		{"Lt_int below", Lt_int(0, 1), false},
		{"Lt_int64 below", Lt_int64(0, 1), false},
		{"Lt_str below", Lt_str("ab", 3), false},
		{"Lt_str equal", Lt_str("abc", 3), true},
		{"E_str equal", E_str("abc", 3), false},
		{"E_str different", E_str("abcd", 3), true},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
