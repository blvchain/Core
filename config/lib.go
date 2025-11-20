package config

import (
	"blvchain/core/logger"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func pathMaker(fileName string) string {
	return CONFIG_FILE_PATH + fileName
}

// Get env from .env file in /config and return one value of the key
func GetEnv(key string) string {
	err := godotenv.Load(pathMaker(".env"))
	if err != nil {
		logger.INTERNAL_LOGGER.Println("Error in reading '.env' file")
		fmt.Println("Error: see log/internal folder for details.")
	}

	return os.Getenv(key)
}

func GetDeliumConfigFile() Delium_json_config {
	file, err := os.Open(pathMaker("delium_config.json"))
	if err != nil {
		logger.INTERNAL_LOGGER.Println("Error opening 'delium_config.json' file")
		fmt.Println("Error: see log/internal folder for details.")
	}
	defer file.Close()

	var jsonConfig Delium_json_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jsonConfig); err != nil {
		logger.INTERNAL_LOGGER.Println("Error decoding 'delium_config.json'")
		fmt.Println("Error: see log/internal folder for details.")
	}

	return jsonConfig
}

func GetDnsSeedListFile() []Dns_seed_config {
	file, err := os.Open(pathMaker("dns_seed.json"))
	if err != nil {
		logger.INTERNAL_LOGGER.Println("Error opening 'dns_seed.json' file")
		fmt.Println("Error: see log/internal folder for details.")
	}
	defer file.Close()

	var dns_seed []Dns_seed_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&dns_seed); err != nil {
		logger.INTERNAL_LOGGER.Println("Error decoding 'dns_seed.json'")
		fmt.Println("Error: see log/internal folder for details.")
	}

	return dns_seed
}

func GetApiKeyFile() map[string]bool {
	file, err := os.Open(pathMaker("api_key.json"))
	if err != nil {
		logger.INTERNAL_LOGGER.Println("Error opening 'api_key.json' file")
		fmt.Println("Error: see log/internal folder for details.")
	}
	defer file.Close()

	var allowedClients map[string]bool
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&allowedClients); err != nil {
		logger.INTERNAL_LOGGER.Println("Error decoding 'api_key.json'")
		fmt.Println("Error: see log/internal folder for details.")
	}

	return allowedClients
}

func StringToInt(strNum string) int {
	num, _ := strconv.Atoi(strNum)
	return num
}

func StringToFloat64(strNum string) float64 {
	num, _ := strconv.ParseFloat(strNum, 64)
	return num
}

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
