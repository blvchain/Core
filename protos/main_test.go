package protos

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"os"
	"testing"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/db/dbtest"
	"blvchain/core/logger"
	"blvchain/core/utils"

	"google.golang.org/grpc/metadata"
)

const testAPIKey = "test-api-key"

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)

	config.SetConfigDir("../config")
	config.ReloadDotEnv()
	config.DELIUM_CONFIG = config.GetDeliumConfigFile()
	config.SELF_UID = "node_1_test"
	config.API_KEY_LIST = map[string]bool{testAPIKey: true}
	config.MAX_DATA_SIZE_KB = "2048"
	config.MAX_WASM_SIZE_KB = 1024
	config.MAX_WASM_SIZE = 1024 * 1024
	config.FIRST_BLOCK_HASH = ""
	config.SMART_CONTRACT_DIR = "./smart_contracts"

	db.ResetCollection()

	code := m.Run()
	os.Exit(code)
}

// identity mirrors db's test helper: a key pair plus the derived UID.
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

// authContext returns a gRPC context carrying a valid API key.
func authContext(key string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", key))
}

// signedRequest builds a BlockData request with a valid signature.
func signedRequest(t *testing.T, sender identity, receiver identity, data string, timestamp int64) *BlockData {
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

	return &BlockData{
		SenderUID:    sender.uid,
		SenderRole:   1,
		SenderPubKey: sender.pubHex,
		Signature:    signature,
		ReceiverUID:  receiver.uid,
		ReceiverRole: 1,
		Data:         data,
		TimeStamp:    timestamp,
	}
}

func resetStore(t *testing.T) {
	t.Helper()
	db.ResetCollection()
	db.SetCollection(dbtest.New())
	t.Cleanup(db.ResetCollection)
}
