package config

import "time"

// Zero-filled constants used by the genesis block. They are built from a single
// 32-character literal so the field lengths can never drift apart again (an
// earlier version of this file had a 62-character block pre-hash and a
// 142-character signature, which no validator could ever accept).
const (
	hexZero32  = "00000000000000000000000000000000"
	hexZero64  = hexZero32 + hexZero32
	hexZero66  = hexZero64 + "00"
	hexZero128 = hexZero64 + hexZero64
)

const (
	// CONFIG_DIR_ENV lets a deployment point the loader at a directory other
	// than ./config. It is also what lets `go test ./config` find the files
	// that live next to the package sources.
	CONFIG_DIR_ENV string = "BLVCHAIN_CONFIG_DIR"

	// SMART_CONTRACT_FILES_PATH is the default directory where uploaded WASM
	// contracts are persisted. It can be overridden with SMART_CONTRACT_DIR.
	SMART_CONTRACT_FILES_PATH string = "smart_contracts/"

	// Mongo
	ZERO_STRING string = "0"
	ONE_STRING  string = "1"

	BLOCK_COLLECTION_NAME string = "Block"

	// Genesis
	GENESIS_NODE_UID       string = hexZero32 // 32 hex characters
	GENESIS_SENDER_UID     string = hexZero32 // 32 hex characters
	GENESIS_RECEIVER_UID   string = hexZero32 // 32 hex characters
	GENESIS_SENDER_ROLE    int64  = 1
	GENESIS_RECEIVER_ROLE  int64  = 1
	GENESIS_BLOCK_PREHASH  string = hexZero64 // 64 hex characters
	GENESIS_TIMESTAMP      int64  = 1720108080000
	GENESIS_PUBKEY         string = hexZero66  // 66 hex characters (compressed P-256)
	GENESIS_SIGNATURE      string = hexZero128 // 128 hex characters
	GENESIS_DATA           string = "Genesis Block Data"
	GENESIS_SELF_UID       string = hexZero32
	GENESIS_NODE_SIGNATURE string = hexZero128

	// Timestamp bounds shared by block validation and gRPC request validation.
	// Both are expressed in milliseconds since the Unix epoch.
	MIN_TIMESTAMP_MS int64 = 1262304000000 // 2009-12-31T23:59:59Z
	MAX_TIMESTAMP_MS int64 = 9262304000000 // 2263-05-11T23:47:20Z

	// WebSocket / sync
	RECONNECT_SLEEP_TIME   = 10
	MAX_LIMIT_OF_DATA_SYNC = 100
	// MIN_LIMIT_OF_DATA_SYNC is the smallest batch worth transferring. Keeping
	// it at 1 means a single new block is replicated instead of being stranded.
	MIN_LIMIT_OF_DATA_SYNC = 1
	SYNC_DATA_SLEEP_TIME   = 5

	// Smart contract
	SMART_CONTRACT_FUNCTION_NAME string = "smart_contract"

	// Defaults used by DefineENV* helpers.
	DEFAULT_DB_NAME           = "BLVchain"
	DEFAULT_GRPC_PORT         = ":50051"
	DEFAULT_WS_PORT           = ":8080"
	DEFAULT_MAX_DATA_SIZE_KB  = "2048"
	DEFAULT_MAX_WASM_SIZE_KB  = 1024
	DEFAULT_EXECUTION_TIMEOUT = 10 * time.Second
	DEFAULT_MAX_MEMORY_PAGES  = 256
	DEFAULT_DB_OP_TIMEOUT     = 10 * time.Second

	// WS_READ_LIMIT_BYTES caps how much a single peer may send on one message.
	WS_READ_LIMIT_BYTES int64 = 4 << 20 // 4 MiB

	// WS_WRITE_TIMEOUT bounds a single WebSocket write.
	WS_WRITE_TIMEOUT = 10 * time.Second

	// WS_READ_TIMEOUT bounds how long a request/response exchange may wait for
	// its peer. It also stops idle peers from pinning a goroutine forever.
	WS_READ_TIMEOUT = 60 * time.Second
)
