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

// Node identities used by the handshake tests. node1 is this node (SELF_UID);
// node2 is the peer that dials in.
var (
	node1Priv string
	node1Pub  string
	node2Priv string
	node2Pub  string
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.SELF_UID = "node_1_test"
	config.MAX_DATA_SIZE_KB = "2048"

	node1Priv, node1Pub = mustNodeKey()
	node2Priv, node2Pub = mustNodeKey()
	config.NODE_PRIVATE_KEY = node1Priv
	config.NODE_PUBLIC_KEY = node1Pub
	config.DNS_SEED_LIST = []config.Dns_seed_config{
		{UID: "node_1_test", Address: "ws://localhost:8081", PubKey: node1Pub},
		{UID: "node_2_test", Address: "ws://localhost:8082", PubKey: node2Pub},
	}

	db.ResetCollection()
	db.SetCollection(dbtest.New())
	config.FIRST_BLOCK_HASH = db.MakeGenesisBlock().ID

	code := m.Run()
	os.Exit(code)
}

func mustNodeKey() (privHex, pubHex string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return utils.PrivateKeyHex(priv), utils.PubKeyHex(&priv.PublicKey)
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
			PreBlockHash: preBlockHash(timestamp),
			NodeUID:      config.SELF_UID,
			TimeStamp:    timestamp,
		},
		BlockData: blockData,
	}
	if err := db.SignBlock(&block, config.NODE_PRIVATE_KEY); err != nil {
		t.Fatalf("sign block: %v", err)
	}
	return block
}

// preBlockHash returns the configured genesis ID when it is known, so blocks
// built by the tests link to a predecessor the node actually has.
func preBlockHash(timestamp int64) string {
	if config.FIRST_BLOCK_HASH != "" {
		return config.FIRST_BLOCK_HASH
	}
	return fmt.Sprintf("%064x", timestamp)
}
