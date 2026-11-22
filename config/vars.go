package config

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/time/rate"
)

var (
	// Global ENV
	MONGO_URI           string = GetEnv("MONGO_URI")
	SMART_CONTRACT_PATH string = GetEnv("SMART_CONTRACT_PATH")

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
	SELF_UID         string = DefineENV("SELF_UID", "")
	DATABASE_NAME    string = DefineENV("DB", DEFAULT_DB_NAME)
	WEBSOCKET_PORT   string = DefineENV("WP", DEFAULT_WS_PORT)
	GRPC_PORT        string = DefineENV("GP", DEFAULT_GRPC_PORT)
	MAX_DATA_SIZE_KB string = DefineENV("MAX_DATA_SIZE_KB", DEFAULT_MAX_DATA_SIZE_KB)

	// Limits
	DB_OP_TIMEOUT     time.Duration = time.Duration(DefineENVInt("DB_OP_TIMEOUT_SECONDS", int(DEFAULT_DB_OP_TIMEOUT/time.Second))) * time.Second
	EXECUTION_TIMEOUT time.Duration = time.Duration(DefineENVInt("EXECUTION_TIMEOUT_SECONDS", int(DEFAULT_EXECUTION_TIMEOUT/time.Second))) * time.Second
	MAX_MEMORY_PAGES  uint32        = uint32(DefineENVInt("MAX_MEMORY_PAGES", DEFAULT_MAX_MEMORY_PAGES))
	MAX_WASM_SIZE_KB  int64         = int64(DefineENVInt("MAX_WASM_SIZE_KB", DEFAULT_MAX_WASM_SIZE_KB))
	MAX_WASM_SIZE     int64         = MAX_WASM_SIZE_KB * 1024

	SMART_CONTRACT_DIR string = DefineENV("SMART_CONTRACT_DIR", SMART_CONTRACT_FILES_PATH)

	// TIMESTAMP_SKEW_MS is the maximum tolerated difference between the node's
	// clock and a client-supplied block timestamp, in milliseconds. It is a
	// sanity bound on AddData submissions, never a synchronisation mechanism.
	TIMESTAMP_SKEW_MS int64 = int64(DefineENVInt("TIMESTAMP_SKEW_MS", int(DEFAULT_TIMESTAMP_SKEW_MS)))

	// Node identity used by the WebSocket peer-authentication handshake (NET-2).
	// NODE_PRIVATE_KEY is this node's P-256 private key (hex); NODE_PUBLIC_KEY is
	// its compressed public key. If NODE_PRIVATE_KEY is empty, LoadNodePrivateKey
	// falls back to config/node_keys.json keyed by SELF_UID.
	NODE_PRIVATE_KEY = DefineENV("NODE_PRIVATE_KEY", "")
	NODE_PUBLIC_KEY  = DefineENV("NODE_PUBLIC_KEY", "")
)

// Ensure the execution limits are usable even when an operator sets nonsense in
// .env (a zero timeout would kill every contract instantly).
func init() {
	if DB_OP_TIMEOUT <= 0 {
		DB_OP_TIMEOUT = DEFAULT_DB_OP_TIMEOUT
	}
	if EXECUTION_TIMEOUT <= 0 {
		EXECUTION_TIMEOUT = DEFAULT_EXECUTION_TIMEOUT
	}
	if MAX_MEMORY_PAGES == 0 {
		MAX_MEMORY_PAGES = DEFAULT_MAX_MEMORY_PAGES
	}
	if MAX_WASM_SIZE_KB <= 0 {
		MAX_WASM_SIZE_KB = DEFAULT_MAX_WASM_SIZE_KB
		MAX_WASM_SIZE = MAX_WASM_SIZE_KB * 1024
	}
	if MAX_DATA_SIZE_KB == "" {
		MAX_DATA_SIZE_KB = DEFAULT_MAX_DATA_SIZE_KB
	}
}
