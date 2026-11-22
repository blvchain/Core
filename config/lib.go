package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"blvchain/core/logger"

	"github.com/joho/godotenv"
)

// configDir is the directory that holds .env and the JSON configuration files.
// It is resolved once at start-up. Call SetConfigDir before touching any of the
// loaders if you need a different location (tests do exactly that).
var configDir = detectConfigDir()

var loadDotEnvOnce sync.Once

// loadDotEnv reads the .env file once. godotenv never overrides variables that
// are already present in the process environment, so shell-provided values keep
// winning (that is what s1.sh/s2.sh/s3.sh rely on).
func loadDotEnv() {
	loadDotEnvOnce.Do(func() {
		if err := godotenv.Load(pathMaker(".env")); err != nil {
			// A missing .env is fine: the deployment may inject every variable.
			if !errors.Is(err, os.ErrNotExist) {
				logger.INTERNAL_LOGGER.Printf("Warning: could not load .env: %v", err)
			}
		}
	})
}

// detectConfigDir looks for ./config next to the process. When the binary is run
// from the repository root that directory exists. When `go test ./config` runs,
// the working directory already *is* the config package, so the fallback "." is
// correct there as well.
func detectConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv(CONFIG_DIR_ENV)); dir != "" {
		return dir
	}
	if info, err := os.Stat("config"); err == nil && info.IsDir() {
		return "config"
	}
	return "."
}

// ConfigDir returns the directory the loaders read from.
func ConfigDir() string { return configDir }

// SetConfigDir overrides the configuration directory.
func SetConfigDir(dir string) { configDir = dir }

func pathMaker(fileName string) string {
	return filepath.Join(configDir, fileName)
}

// LoadDotEnv forces a (re)load of the .env file. It is mostly useful in tests.
func LoadDotEnv() { loadDotEnv() }

// ReloadDotEnv resets the load-once guard and reads the .env file again.
func ReloadDotEnv() {
	loadDotEnvOnce = sync.Once{}
	loadDotEnv()
}

// GetEnv returns the value of key from the process environment.
func GetEnv(key string) string {
	loadDotEnv()
	return os.Getenv(key)
}

// GetDeliumConfigFile returns the hashing pipeline configuration. A missing or
// malformed file yields the zero value; call Validate to get a precise error.
func GetDeliumConfigFile() Delium_json_config {
	var cfg Delium_json_config
	if err := readJSONFile(pathMaker("delium_config.json"), &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.INTERNAL_LOGGER.Printf("Warning: delium_config.json could not be loaded: %v", err)
	}
	return cfg
}

// GetDnsSeedListFile returns the seed nodes used for peer discovery.
func GetDnsSeedListFile() []Dns_seed_config {
	var seeds []Dns_seed_config
	if err := readJSONFile(pathMaker("dns_seed.json"), &seeds); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.INTERNAL_LOGGER.Printf("Warning: dns_seed.json could not be loaded: %v", err)
	}
	return seeds
}

// GetApiKeyFile returns the map of allowed API keys. The boolean value is the
// enabled flag; a key set to false is present but rejected.
func GetApiKeyFile() map[string]bool {
	allowed := make(map[string]bool)
	if err := readJSONFile(pathMaker("api_key.json"), &allowed); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.INTERNAL_LOGGER.Printf("Warning: api_key.json could not be loaded: %v", err)
	}
	return allowed
}

func readJSONFile(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func StringToInt(strNum string) int {
	num, _ := strconv.Atoi(strNum)
	return num
}

func StringToFloat64(strNum string) float64 {
	num, _ := strconv.ParseFloat(strNum, 64)
	return num
}

// DefineENV returns the environment variable name, or defaultValue when it is
// unset or empty.
func DefineENV(name string, defaultValue string) string {
	envVar := os.Getenv(name)
	if envVar == "" {
		envVar = defaultValue
	}
	return envVar
}

func DefineENVFloat64(name string, defaultValue float64) float64 {
	envVar := os.Getenv(name)
	if envVar == "" || envVar == "0" {
		return defaultValue
	}
	return StringToFloat64(envVar)
}

func DefineENVInt(name string, defaultValue int) int {
	envVar := os.Getenv(name)
	if envVar == "" || envVar == "0" {
		return defaultValue
	}
	return StringToInt(envVar)
}

// Validate checks that the values the node cannot run without are present and
// well formed. main calls it before opening the database.
func Validate() error {
	var problems []string

	if MONGO_URI == "" {
		problems = append(problems, "MONGO_URI is required")
	}
	if SELF_UID == "" {
		problems = append(problems, "SELF_UID is required")
	}
	if DATABASE_NAME == "" {
		problems = append(problems, "DB is required")
	}
	if GRPC_PORT == "" {
		problems = append(problems, "GP (gRPC port) is required")
	}
	if WEBSOCKET_PORT == "" {
		problems = append(problems, "WP (WebSocket port) is required")
	}
	if StringToFloat64(MAX_DATA_SIZE_KB) <= 0 {
		problems = append(problems, "MAX_DATA_SIZE_KB must be a positive number")
	}
	if ADD_DATA_BURST <= 0 || READ_DATA_BURST <= 0 {
		problems = append(problems, "rate-limit bursts must be greater than zero")
	}
	if err := ValidateDeliumPath(DELIUM_CONFIG.BLOCK_HASHING_PATH); err != nil {
		problems = append(problems, "delium_config.json BLOCK_HASHING_PATH: "+err.Error())
	}
	if err := ValidateDeliumPath(DELIUM_CONFIG.UID_MAKER_PATH); err != nil {
		problems = append(problems, "delium_config.json UID_MAKER_PATH: "+err.Error())
	}
	if err := ValidateDeliumPath(DELIUM_CONFIG.MESSAGE_HASH_PATH); err != nil {
		problems = append(problems, "delium_config.json MESSAGE_HASH_PATH: "+err.Error())
	}
	enabledKeys := 0
	for _, enabled := range API_KEY_LIST {
		if enabled {
			enabledKeys++
		}
	}
	if enabledKeys == 0 {
		problems = append(problems, "api_key.json must contain at least one enabled key")
	}

	for _, seed := range DNS_SEED_LIST {
		if seed.UID == "" || seed.Address == "" {
			problems = append(problems, "dns_seed.json entries need both UID and Address")
			break
		}
	}

	if len(problems) > 0 {
		return errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return nil
}

// ValidateDeliumPath checks the "addon#deleteStep/addon#deleteStep" syntax used
// by delium_config.json. An empty path is rejected because it would silently
// degrade to a plain SHA-256.
func ValidateDeliumPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	for _, part := range strings.Split(path, "/") {
		fields := strings.Split(part, "#")
		if len(fields) != 2 {
			return fmt.Errorf("segment %q must look like addon#deleteStep", part)
		}
		if fields[0] == "" {
			return fmt.Errorf("segment %q has an empty addon", part)
		}
		step, err := strconv.Atoi(fields[1])
		if err != nil {
			return fmt.Errorf("segment %q has a non-numeric deleteStep", part)
		}
		if step < 1 {
			return fmt.Errorf("segment %q has a deleteStep smaller than 1", part)
		}
	}
	return nil
}
