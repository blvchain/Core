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
	return FILE_PATH + fileName
}

// Get env from .env file in /config and return one value of the key
func GetEnv(key string) string {
	err := godotenv.Load(pathMaker(".env"))
	if err != nil {
		PrintError("Error in reading ENV file")
	}

	return os.Getenv(key)
}

func GetDeliumConfigFile() Delium_json_config {
	file, err := os.Open(pathMaker("delium_config.json"))
	if err != nil {
		PrintError("Error opening delium_config.json file")
	}
	defer file.Close()

	var jsonConfig Delium_json_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jsonConfig); err != nil {
		PrintError("Error decoding delium_config.json")
	}

	return jsonConfig
}

func GetBlvInfoFile() Blv_info_json {
	file, err := os.Open(pathMaker("blv_info.json"))
	if err != nil {
		PrintError("Error opening blv_info.json file")
	}
	defer file.Close()

	var blv_info Blv_info_json
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&blv_info); err != nil {
		PrintError("Error decoding blv_info.json")
	}

	return blv_info
}

func GetDnsSeedListFile() Dns_seed_list {
	file, err := os.Open(pathMaker("dns_seed.json"))
	if err != nil {
		PrintError("Error opening dns_seed.json file")
	}
	defer file.Close()

	var dns_seed Dns_seed_list
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&dns_seed); err != nil {
		PrintError("Error decoding dns_seed.json")
	}

	return dns_seed
}
