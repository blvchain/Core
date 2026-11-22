package config

import (
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"blvchain/core/logger"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// snapshot returns a copy of the mutable package configuration.
type configSnapshot struct {
	dir          string
	mongoURI     string
	selfUID      string
	databaseName string
	maxDataKB    string
	delium       Delium_json_config
	apiKeys      map[string]bool
	seeds        []Dns_seed_config
}

func snapshot(t *testing.T) configSnapshot {
	t.Helper()
	return configSnapshot{
		dir:          configDir,
		mongoURI:     MONGO_URI,
		selfUID:      SELF_UID,
		databaseName: DATABASE_NAME,
		maxDataKB:    MAX_DATA_SIZE_KB,
		delium:       DELIUM_CONFIG,
		apiKeys:      API_KEY_LIST,
		seeds:        DNS_SEED_LIST,
	}
}

func restore(t *testing.T, s configSnapshot) {
	t.Helper()
	t.Cleanup(func() {
		configDir = s.dir
		MONGO_URI = s.mongoURI
		SELF_UID = s.selfUID
		DATABASE_NAME = s.databaseName
		MAX_DATA_SIZE_KB = s.maxDataKB
		DELIUM_CONFIG = s.delium
		API_KEY_LIST = s.apiKeys
		DNS_SEED_LIST = s.seeds
		loadDotEnvOnce = sync.Once{}
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestGetDeliumConfigFile(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	dir := t.TempDir()
	writeFile(t, dir, "delium_config.json", `{
		"BLOCK_HASHING_PATH": "a#5/b#6",
		"UID_MAKER_PATH": "u#7",
		"MESSAGE_HASH_PATH": "m#8"
	}`)
	SetConfigDir(dir)

	got := GetDeliumConfigFile()
	if got.BLOCK_HASHING_PATH != "a#5/b#6" || got.UID_MAKER_PATH != "u#7" || got.MESSAGE_HASH_PATH != "m#8" {
		t.Fatalf("GetDeliumConfigFile() = %+v", got)
	}
}

func TestGetDeliumConfigFileMissing(t *testing.T) {
	s := snapshot(t)
	restore(t, s)
	SetConfigDir(t.TempDir())

	got := GetDeliumConfigFile()
	if got != (Delium_json_config{}) {
		t.Fatalf("a missing file must yield the zero value, got %+v", got)
	}
}

func TestGetDnsSeedListFile(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	dir := t.TempDir()
	writeFile(t, dir, "dns_seed.json", `[
		{"UID": "node_1_abc", "Address": "ws://localhost:8081"},
		{"UID": "node_2_def", "Address": "ws://localhost:8082"}
	]`)
	SetConfigDir(dir)

	got := GetDnsSeedListFile()
	if len(got) != 2 {
		t.Fatalf("got %d seeds, want 2", len(got))
	}
	if got[0].UID != "node_1_abc" || got[1].Address != "ws://localhost:8082" {
		t.Fatalf("unexpected seeds: %+v", got)
	}
}

func TestGetApiKeyFile(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	dir := t.TempDir()
	writeFile(t, dir, "api_key.json", `{"enabled-key": true, "disabled-key": false}`)
	SetConfigDir(dir)

	got := GetApiKeyFile()
	if len(got) != 2 {
		t.Fatalf("got %d keys, want 2", len(got))
	}
	if !got["enabled-key"] || got["disabled-key"] {
		t.Fatalf("unexpected flags: %+v", got)
	}
}

func TestGetEnvReadsDotEnv(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	dir := t.TempDir()
	writeFile(t, dir, ".env", "BLVCHAIN_TEST_ENV_VALUE=from-file\n")
	SetConfigDir(dir)
	ReloadDotEnv()

	if got := GetEnv("BLVCHAIN_TEST_ENV_VALUE"); got != "from-file" {
		t.Fatalf("GetEnv = %q, want from-file", got)
	}
}

func TestGetEnvDoesNotOverrideShell(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	t.Setenv("BLVCHAIN_SHELL_VALUE", "from-shell")

	dir := t.TempDir()
	writeFile(t, dir, ".env", "BLVCHAIN_SHELL_VALUE=from-file\n")
	SetConfigDir(dir)
	ReloadDotEnv()

	if got := GetEnv("BLVCHAIN_SHELL_VALUE"); got != "from-shell" {
		t.Fatalf("GetEnv = %q, want from-shell (godotenv must not override)", got)
	}
}

func TestDefineHelpers(t *testing.T) {
	t.Setenv("BLVCHAIN_STR", "value")
	t.Setenv("BLVCHAIN_INT", "42")
	t.Setenv("BLVCHAIN_ZERO", "0")
	t.Setenv("BLVCHAIN_FLOAT", "0.5")

	if got := DefineENV("BLVCHAIN_STR", "fallback"); got != "value" {
		t.Errorf("DefineENV = %q", got)
	}
	if got := DefineENV("BLVCHAIN_MISSING", "fallback"); got != "fallback" {
		t.Errorf("DefineENV fallback = %q", got)
	}
	if got := DefineENVInt("BLVCHAIN_INT", 7); got != 42 {
		t.Errorf("DefineENVInt = %d", got)
	}
	if got := DefineENVInt("BLVCHAIN_ZERO", 7); got != 7 {
		t.Errorf("DefineENVInt with 0 should fall back, got %d", got)
	}
	if got := DefineENVInt("BLVCHAIN_MISSING", 7); got != 7 {
		t.Errorf("DefineENVInt fallback = %d", got)
	}
	if got := DefineENVFloat64("BLVCHAIN_FLOAT", 1.5); got != 0.5 {
		t.Errorf("DefineENVFloat64 = %v", got)
	}
	if got := DefineENVFloat64("BLVCHAIN_MISSING", 1.5); got != 1.5 {
		t.Errorf("DefineENVFloat64 fallback = %v", got)
	}
}

func TestStringHelpers(t *testing.T) {
	if got := StringToInt("123"); got != 123 {
		t.Errorf("StringToInt = %d", got)
	}
	if got := StringToFloat64("1.25"); got != 1.25 {
		t.Errorf("StringToFloat64 = %v", got)
	}
}

func TestGenesisConstantsHaveValidLengths(t *testing.T) {
	// The genesis block is validated like any other block, so a mis-sized
	// constant would make the very first block unacceptable.
	cases := map[string]struct {
		value string
		want  int
	}{
		"GENESIS_NODE_UID":       {GENESIS_NODE_UID, 32},
		"GENESIS_SENDER_UID":     {GENESIS_SENDER_UID, 32},
		"GENESIS_RECEIVER_UID":   {GENESIS_RECEIVER_UID, 32},
		"GENESIS_BLOCK_PREHASH":  {GENESIS_BLOCK_PREHASH, 64},
		"GENESIS_PUBKEY":         {GENESIS_PUBKEY, 66},
		"GENESIS_SIGNATURE":      {GENESIS_SIGNATURE, 128},
		"GENESIS_NODE_SIGNATURE": {GENESIS_NODE_SIGNATURE, 128},
		"GENESIS_SELF_UID":       {GENESIS_SELF_UID, 32},
	}

	for name, tc := range cases {
		if len(tc.value) != tc.want {
			t.Errorf("%s has length %d, want %d", name, len(tc.value), tc.want)
		}
		if _, err := hex.DecodeString(tc.value); err != nil {
			t.Errorf("%s is not valid hex: %v", name, err)
		}
	}
}

func TestValidateDeliumPath(t *testing.T) {
	valid := []string{"a#1", "BLV#5/chain#6/StRoNg#7/SAFE#8"}
	for _, path := range valid {
		if err := ValidateDeliumPath(path); err != nil {
			t.Errorf("ValidateDeliumPath(%q) = %v, want nil", path, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"nohash",
		"a#",
		"#5",
		"a#0",
		"a#-1",
		"a#abc",
		"a#5/",
	}
	for _, path := range invalid {
		if err := ValidateDeliumPath(path); err == nil {
			t.Errorf("ValidateDeliumPath(%q) = nil, want an error", path)
		}
	}
}

func TestValidateAcceptsShippedConfig(t *testing.T) {
	// The repository ships a config/.env and valid JSON files; running the test
	// from the package directory means those files are the ones being read.
	if err := Validate(); err != nil {
		t.Fatalf("Validate() on the shipped configuration = %v", err)
	}
}

func TestValidateRejectsMissingSelfUID(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	SELF_UID = ""
	err := Validate()
	if err == nil || !strings.Contains(err.Error(), "SELF_UID") {
		t.Fatalf("Validate() = %v, want an error mentioning SELF_UID", err)
	}
}

func TestValidateRejectsEmptyMongoURI(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	MONGO_URI = ""
	err := Validate()
	if err == nil || !strings.Contains(err.Error(), "MONGO_URI") {
		t.Fatalf("Validate() = %v, want an error mentioning MONGO_URI", err)
	}
}

func TestValidateRejectsInvalidDeliumPath(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	DELIUM_CONFIG = Delium_json_config{BLOCK_HASHING_PATH: "broken"}
	err := Validate()
	if err == nil || !strings.Contains(err.Error(), "BLOCK_HASHING_PATH") {
		t.Fatalf("Validate() = %v, want an error mentioning BLOCK_HASHING_PATH", err)
	}
}

func TestValidateRejectsNoEnabledAPIKey(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	API_KEY_LIST = map[string]bool{"disabled": false}
	err := Validate()
	if err == nil || !strings.Contains(err.Error(), "api_key.json") {
		t.Fatalf("Validate() = %v, want an error mentioning api_key.json", err)
	}
}

func TestValidateRejectsBadDataSize(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	MAX_DATA_SIZE_KB = "0"
	err := Validate()
	if err == nil || !strings.Contains(err.Error(), "MAX_DATA_SIZE_KB") {
		t.Fatalf("Validate() = %v, want an error mentioning MAX_DATA_SIZE_KB", err)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	s := snapshot(t)
	restore(t, s)

	SELF_UID = ""
	MONGO_URI = ""
	err := Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "SELF_UID") || !strings.Contains(err.Error(), "MONGO_URI") {
		t.Fatalf("Validate() should list both problems, got: %v", err)
	}
}

func TestDefaultsAreSane(t *testing.T) {
	// These constants are what the node falls back to; a bad default would be
	// invisible until production.
	if DEFAULT_MAX_MEMORY_PAGES == 0 {
		t.Error("DEFAULT_MAX_MEMORY_PAGES must not be zero")
	}
	if DEFAULT_EXECUTION_TIMEOUT <= 0 {
		t.Error("DEFAULT_EXECUTION_TIMEOUT must be positive")
	}
	if MAX_TIMESTAMP_MS <= MIN_TIMESTAMP_MS {
		t.Error("MAX_TIMESTAMP_MS must be greater than MIN_TIMESTAMP_MS")
	}
	if WS_READ_LIMIT_BYTES <= 0 {
		t.Error("WS_READ_LIMIT_BYTES must be positive")
	}
}
