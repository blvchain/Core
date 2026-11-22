package db

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"os"
	"testing"

	"blvchain/core/config"
	"blvchain/core/logger"
	"blvchain/core/utils"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.SELF_UID = "node_1_test"

	// A node key is required to sign blocks (NET-3). Pin it in dns_seed.json's
	// in-memory equivalent so BlockValidator can verify the node signature.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	nodePriv := utils.PrivateKeyHex(priv)
	nodePub := utils.PubKeyHex(&priv.PublicKey)
	config.NODE_PRIVATE_KEY = nodePriv
	config.NODE_PUBLIC_KEY = nodePub
	config.DNS_SEED_LIST = []config.Dns_seed_config{
		{UID: config.SELF_UID, Address: "ws://localhost:8081", PubKey: nodePub},
	}

	config.FIRST_BLOCK_HASH = ""
	config.MAX_DATA_SIZE_KB = "2048"

	ResetCollection()

	code := m.Run()
	os.Exit(code)
}
