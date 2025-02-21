package config

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	// Global ENV
	MONGO_URI string = GetEnv("MONGO_URI")

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
	SELF_UID       string = DefineENV("SELF_UID", GetEnv("SELF_UID"))
	DATABASE_NAME         = DefineENV("DB", "BLVchain")
	WEBSOCKET_PORT        = DefineENV("WP", ":8080")
	GRPC_PORT             = DefineENV("GP", ":50051")
)
