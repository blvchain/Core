package config

// DO NOT CHANGE THESE CONSTS!

const (
	ZERO_STRING string = "0"
	ONE_STRING  string = "1"

	// Node
	NODE_ID_LEN int = 16

	// Cryption
	DELIUM_SEED_PATH  string = "BLV#5/chain#6/StRoNg#7/SAFE#8"
	MNEMONIC_STRENGTH int    = 128

	// DB
	DATABASE_NAME string = "BLVchain"

	SELF_AUTH_COLLECTION_NAME   string = "selfauth"
	CLIENT_AUTH_COLLECTION_NAME string = "clientauth"
	DATA_COLLECTION_NAME        string = "data"

	// DATA type
	GENESIS_DATA_TYPE int = 1
	NODE_DATA_TYPE    int = 2
	NORMAL_DATA_TYPE  int = 3

	// NODE message type
	NODE_GET_AUTH      int = 0
	NODE_VERIFY_AUTH   int = 1
	NODE_NEW_DATA      int = 2
	NODE_GET_ALL_DATAS int = 3
	NODE_SYNC_DATAS    int = 4
	NODE_NEW_DNSSEED   int = 5

	DELIMITER string = "#"
)
