package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func PrintError(err string) {
	redColor := "\033[31m"
	resetColor := "\033[0m"
	fmt.Printf("%s", redColor+err+resetColor+"\n")
	os.Exit(1)
}

func pathMaker(fileName string) string {
	return CONFIG_FILE_PATH + fileName
}

// Get env from .env file in /config and return one value of the key
func GetEnv(key string) string {
	err := godotenv.Load(pathMaker(".env"))
	if err != nil {
		PrintError("Error in reading '.env' file")
	}

	return os.Getenv(key)
}

func GetDeliumConfigFile() Delium_json_config {
	file, err := os.Open(pathMaker("delium_config.json"))
	if err != nil {
		PrintError("Error opening 'delium_config.json' file")
	}
	defer file.Close()

	var jsonConfig Delium_json_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jsonConfig); err != nil {
		PrintError("Error decoding 'delium_config.json'")
	}

	return jsonConfig
}

func GetDnsSeedListFile() []Dns_seed_config {
	file, err := os.Open(pathMaker("dns_seed.json"))
	if err != nil {
		PrintError("Error opening 'dns_seed.json' file")
	}
	defer file.Close()

	var dns_seed []Dns_seed_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&dns_seed); err != nil {
		PrintError("Error decoding 'dns_seed.json'")
	}

	return dns_seed
}

func GetApiKeyFile() map[string]bool {
	file, err := os.Open(pathMaker("api_key.json"))
	if err != nil {
		PrintError("Error opening 'api_key.json' file")
	}
	defer file.Close()

	var allowedClients map[string]bool
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&allowedClients); err != nil {
		PrintError("Error decoding 'api_key.json'")
	}

	return allowedClients
}

func DefineENV(name string, defaultValue string) string {
	port := os.Getenv(name)
	if port == "" {
		port = defaultValue
	}

	return port
}
