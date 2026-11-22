package ws

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"testing"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/db/dbtest"
	"blvchain/core/logger"
	"blvchain/core/utils"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.SELF_UID = "node_1_test"
	config.DNS_SEED_LIST = []config.Dns_seed_config{
		{UID: "node_1_test", Address: "ws://localhost:8081"},
		{UID: "node_2_test", Address: "ws://localhost:8082"},
	}
	config.MAX_DATA_SIZE_KB = "2048"
	config.FIRST_BLOCK_HASH = ""

	db.ResetCollection()

	code := m.Run()
	os.Exit(code)
}

type identity struct {
	privHex string
	pubHex  string
	uid     string
}

func newIdentity(t *testing.T) identity {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubHex := utils.PubKeyHex(&priv.PublicKey)
	return identity{
		privHex: utils.PrivateKeyHex(priv),
		pubHex:  pubHex,
		uid:     utils.Make_UID(pubHex),
	}
}

func resetStore(t *testing.T) *dbtest.Memory {
	t.Helper()

	store := dbtest.New()
	db.SetCollection(store)
	t.Cleanup(db.ResetCollection)
	return store
}

// makeBlock builds a valid, signed block.
func makeBlock(t *testing.T, sender identity, receiver identity, data string, timestamp int64) db.Block {
	t.Helper()

	blockData := db.BlockData{
		SenderUID:    sender.uid,
		SenderRole:   1,
		SenderPubKey: sender.pubHex,
		ReceiverUID:  receiver.uid,
		ReceiverRole: 1,
		Data:         data,
		TimeStamp:    timestamp,
	}

	signature, err := utils.Sign(sender.privHex, db.MessageMaker(blockData))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	blockData.Signature = signature

	block := db.Block{
		BlockMeta: db.BlockMeta{
			PreBlockHash: fmt.Sprintf("%064x", timestamp),
			TimeStamp:    timestamp,
		},
		BlockData: blockData,
	}
	db.BlockHashMaker(&block, config.SELF_UID)
	return block
}
