package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Get env from .env file in /config and return one value of the key
func GetEnv(key string) string {
	err := godotenv.Load("config/.env")
	if err != nil {
		log.Fatalf("Error in reading ENV file: %v", err)
	}

	return os.Getenv(key)
}
