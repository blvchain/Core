package config

// DO NOT CHANGE THESE CONSTS!

const (
	ZERO_STRING string = "0"
	ONE_STRING  string = "1"

	// Node
	NODE_ID_LEN int = 16

	// Cryption
	DELIUM_SEED_PATH  string = "matin#5/ramz#6/negar#7/SAFE#8"
	MNEMONIC_STRENGTH int    = 128

	// DB
	DATABASE_NAME string = "matinramznegar"

	NODE_ID_COLLECTION_NAME     string = "nodeid"
	SELF_AUTH_COLLECTION_NAME   string = "selfauth"
	CLIENT_AUTH_COLLECTION_NAME string = "clientauth"
	Data_COLLECTION_NAME        string = "data"
	DNS_SEED_COLLECTION_NAME    string = "dnsseed"

	BLVCHAIN_URL    string = "https://blvchain.org"
	MAKER_WALLET    string = "bd1465212ba7bb2c3ed20e02594b1d49"
	BLVCHAIN_WALLET string = "bd1465212ba7bb2c3ed20e02594b1d49"
	BLVCHAIN_PUBKEY string = "dfd"

	// NODE message type
	NODE_GET_AUTH      int = 0
	NODE_VERIFY_AUTH   int = 1
	NODE_NEW_Data      int = 2
	NODE_GET_ALL_DataS int = 3
	NODE_SYNC_DataS    int = 4
	NODE_NEW_DNSSEED   int = 5

	DELIMITER string = "#"
)
