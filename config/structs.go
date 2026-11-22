package config

type Delium_json_config struct {
	BLOCK_HASHING_PATH string
	UID_MAKER_PATH     string
	MESSAGE_HASH_PATH  string
}

type Dns_seed_config struct {
	UID     string
	Address string
	// PubKey is the node's compressed P-256 public key (66 hex chars). It is the
	// trust anchor for the WebSocket peer-authentication handshake (NET-2): a
	// peer must prove possession of the private key for this value. It is public
	// information and safe to commit.
	PubKey string
}
