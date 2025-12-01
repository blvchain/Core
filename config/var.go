package config

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/time/rate"
)

var (
	CONFIG_FILE_PATH           string = "./config/"
	SMART_CONTRACT_UPLOAD_PATH string = "../smart_contracts/"
	VC_UPLOAD_PATH             string = "../smart_contracts/"

	// Mongo
	ZERO_STRING string = "0"
	ONE_STRING  string = "1"

	BLOCK_COLLECTION_NAME string = "block"

	// Genesis
	GENESIS_NODE_UID       primitive.Binary = ToMongoBinary([]byte("00000000000000000000000000000000"))
	GENESIS_SENDER_UID     primitive.Binary = ToMongoBinary([]byte("00000000000000000000000000000000"))
	GENESIS_RECEIVER_UID   primitive.Binary = ToMongoBinary([]byte("00000000000000000000000000000000"))
	GENESIS_BLOCK_PREHASH  primitive.Binary = ToMongoBinary([]byte("00000000000000000000000000000000000000000000000000000000000000"))
	GENESIS_TIMESTAMP      int64            = 1720108080000
	GENESIS_PUBKEY         primitive.Binary = ToMongoBinary([]byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))
	GENESIS_SIGNATURE      primitive.Binary = ToMongoBinary([]byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))
	GENESIS_SELF_UID       primitive.Binary = ToMongoBinary([]byte("00000000000000000000000000000000"))
	GENESIS_NODE_SIGNATURE primitive.Binary = ToMongoBinary([]byte("0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))

	// Web socket request types
	RECONNECT_SLEEP_TIME   = 10
	MAX_LIMIT_OF_DATA_SYNC = 100
	MIN_LIMIT_OF_DATA_SYNC = 2
	SYNC_DATA_SLEEP_TIME   = 5

	// Smart contract
	SMART_CONTRACT_FUNCTION_NAME string        = "smart_contract"
	EXECUTION_TIMEOUT            time.Duration = 10 * time.Second
	MAX_MEMORY_PAGES             uint32        = 256 // 1 Page = 64KB ==> 64*256 = 16MB

	// Global ENV
	MONGO_URI string = GetEnv("MONGO_URI")

	// Rate limit for gRPC
	READ_DATA_R     rate.Limit = rate.Limit(DefineENVFloat64("READ_DATA_R", 0.1))
	READ_DATA_BURST int        = DefineENVInt("READ_DATA_BURST", 30)
	ADD_DATA_R      rate.Limit = rate.Limit(DefineENVFloat64("ADD_DATA_R", 0.1))
	ADD_DATA_BURST  int        = DefineENVInt("ADD_DATA_BURST", 5)

	// Mongodb
	NO_FILTER primitive.M = bson.M{}
	DESC      primitive.M = bson.M{"m.t": -1}
	ASC       primitive.M = bson.M{"m.t": 1}

	BLOCK_COLL *mongo.Collection

	FIRST_BLOCK_HASH primitive.Binary

	// Get vars form files
	DELIUM_CONFIG = GetDeliumConfigFile()
	DNS_SEED_LIST = GetDnsSeedListFile()
	API_KEY_LIST  = GetApiKeyFile()

	// Terminal variables
	SELF_UID         primitive.Binary = ToMongoBinary([]byte(DefineENV("SELF_UID", GetEnv("SELF_UID"))))
	DATABASE_NAME    string           = DefineENV("DB", "BLVchain")
	GRPC_PORT        string           = DefineENV("GP", ":50051")
	MAX_DATA_SIZE_KB string           = DefineENV("MAX_DATA_SIZE_KB", "2048")
)
