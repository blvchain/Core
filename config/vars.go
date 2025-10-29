package config

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/time/rate"
)

var (
	// Global ENV
	MONGO_URI           string = GetEnv("MONGO_URI")
	SMART_CONTRACT_PATH string = GetEnv("SMART_CONTRACT_PATH")
	DEV_MODE            string = GetEnv("DEV_MODE")

	// Rate limit for gRPC
	READ_DATA_R     rate.Limit = rate.Limit(DefineENVFloat64("READ_DATA_R", 0.1))
	READ_DATA_BURST int        = DefineENVInt("READ_DATA_BURST", 30)
	ADD_DATA_R      rate.Limit = rate.Limit(DefineENVFloat64("ADD_DATA_R", 0.1))
	ADD_DATA_BURST  int        = DefineENVInt("ADD_DATA_BURST", 5)

	// Mongodb
	NO_FILTER primitive.M = bson.M{}
	DESC      primitive.M = bson.M{"blockMeta.timeStamp": -1}
	ASC       primitive.M = bson.M{"blockMeta.timeStamp": 1}

	BLOCK_COLL *mongo.Collection

	FIRST_BLOCK_HASH string

	// Get vars form files
	DELIUM_CONFIG = GetDeliumConfigFile()
	DNS_SEED_LIST = GetDnsSeedListFile()
	API_KEY_LIST  = GetApiKeyFile()

	// Terminal variables
	SELF_UID         string = DefineENV("SELF_UID", GetEnv("SELF_UID"))
	DATABASE_NAME           = DefineENV("DB", "BLVchain")
	WEBSOCKET_PORT          = DefineENV("WP", ":8080")
	GRPC_PORT               = DefineENV("GP", ":50051")
	MAX_DATA_SIZE_KB        = DefineENV("MAX_DATA_SIZE_KB", "2048")
)
