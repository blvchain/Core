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
	fmt.Printf(redColor + err + resetColor + "\n")
	os.Exit(1)
}

// Get env from .env file in /config and return one value of the key
func GetEnv(key string) string {
	err := godotenv.Load("config/.env")
	if err != nil {
		PrintError("Error in reading ENV file")
	}

	return os.Getenv(key)
}

func GetConfigFile() Delium_json_config {
	file, err := os.Open("delium_config.json")
	if err != nil {
		PrintError("Error opening file")
	}
	defer file.Close()

	var jsonConfig Delium_json_config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&jsonConfig); err != nil {
		PrintError("Error decoding JSON")
	}

	return jsonConfig
}

func SaveBlvInfo(blv_info Blv_info_json) {
	file, err := os.Create("blv_info.json")
	if err != nil {
		PrintError("Error creating file")
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(blv_info); err != nil {
		PrintError("Error encoding JSON")
	}

}
