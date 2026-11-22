package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"blvchain/core/config"
	"blvchain/core/db"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
)

func TestUpgraderRejectsUnknownAndSelfNodes(t *testing.T) {
	cases := []struct {
		name string
		uid  string
		want bool
	}{
		{"listed peer", "node_2_test", true},
		{"self", config.SELF_UID, false},
		{"unknown", "node_99", false},
		{"missing", "", false},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.uid != "" {
			req.Header.Set("X-UID", tc.uid)
		}
		if got := upgrader.CheckOrigin(req); got != tc.want {
			t.Errorf("%s: CheckOrigin = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestHandleGetBlock(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "payload", 1700000000001)
	if ok, err := db.InsertOneBlock(block); !ok || err != nil {
		t.Fatalf("insert: %v %v", ok, err)
	}

	got := handleGetBlock(db.Block{ID: block.ID})
	if !got.IsSuccess {
		t.Fatalf("handleGetBlock failed for an existing block")
	}
	if got.Block.ID != block.ID {
		t.Fatalf("handleGetBlock returned %q, want %q", got.Block.ID, block.ID)
	}

	if missing := handleGetBlock(db.Block{ID: strings.Repeat("f", 64)}); missing.IsSuccess {
		t.Fatal("handleGetBlock reported success for a missing block")
	}
	if empty := handleGetBlock(db.Block{}); empty.IsSuccess {
		t.Fatal("handleGetBlock reported success for an empty query")
	}
}

func TestHandleAddBlock(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "payload", 1700000000002)

	res := handleAddBlock("node_2_test", block)
	if !res.IsSuccess {
		t.Fatalf("handleAddBlock rejected a valid block")
	}
	if res.Block.Boycott {
		t.Fatal("a valid block must not be boycotted")
	}

	blocks, err := db.FindAllBlocks(bson.M{"_id": block.ID})
	if err != nil || len(blocks) != 1 {
		t.Fatalf("block was not stored: %v (%d)", err, len(blocks))
	}

	// A duplicate must be rejected.
	if duplicate := handleAddBlock("node_2_test", block); duplicate.IsSuccess {
		t.Fatal("handleAddBlock accepted a duplicate")
	}
}

func TestHandleAddBlockQuarantinesInvalidSignatures(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "payload", 1700000000003)
	block.BlockData.Signature = strings.Repeat("0", 128)
	db.BlockHashMaker(&block, block.BlockMeta.NodeUID)

	res := handleAddBlock("node_2_test", block)
	if !res.IsSuccess {
		t.Fatal("handleAddBlock should store the block with Boycott=true")
	}
	if !res.Block.Boycott {
		t.Fatal("a block with an invalid signature must be boycotted")
	}
}

func TestHandleAddBlockRejectsMalformed(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "payload", 1700000000004)
	block.ID = "not-a-hash"

	if res := handleAddBlock("node_2_test", block); res.IsSuccess {
		t.Fatal("handleAddBlock accepted a malformed block")
	}
	if blocks, _ := db.FindAllBlocks(bson.M{}); len(blocks) != 0 {
		t.Fatalf("a malformed block was stored: %d", len(blocks))
	}
}

func TestHandleSyncBlocks(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	genesis := db.MakeGenesisBlock()
	old := makeBlock(t, sender, receiver, "old", 1700000000010)
	recent := makeBlock(t, sender, receiver, "recent", 1700000000020)
	for _, block := range []db.Block{genesis, old, recent} {
		if ok, err := db.InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	res := handleSyncBlocks("node_2_test", db.Block{BlockMeta: db.BlockMeta{TimeStamp: old.BlockMeta.TimeStamp}})
	if !res.IsSuccess {
		t.Fatal("handleSyncBlocks should succeed")
	}

	// The genesis block (preBlockHash == zero hash) is never replicated.
	for _, block := range res.Blocks {
		if block.ID == genesis.ID {
			t.Fatal("handleSyncBlocks returned the genesis block")
		}
	}
	if len(res.Blocks) == 0 {
		t.Fatal("handleSyncBlocks returned no blocks")
	}

	// Nothing newer than the newest block.
	tip := db.Block{ID: recent.ID, BlockMeta: db.BlockMeta{TimeStamp: recent.BlockMeta.TimeStamp}}
	if latest := handleSyncBlocks("node_2_test", tip); latest.IsSuccess {
		t.Fatalf("handleSyncBlocks reported success with no new blocks")
	}
}

func TestFilterNewBlocks(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	valid := makeBlock(t, sender, receiver, "valid", 1700000000100)
	known := makeBlock(t, sender, receiver, "known", 1700000000101)
	badSignature := makeBlock(t, sender, receiver, "bad", 1700000000102)
	badSignature.BlockData.Signature = strings.Repeat("0", 128)
	db.BlockHashMaker(&badSignature, badSignature.BlockMeta.NodeUID)

	malformed := makeBlock(t, sender, receiver, "malformed", 1700000000103)
	malformed.ID = "broken"

	contract := makeBlock(t, sender, receiver, "contract", 1700000000104)
	contract.BlockData.UseContract = strings.Repeat("a", 66)
	contract.BlockData.ContractData = db.Contract{Name: "counter", Checksum: strings.Repeat("b", 64)}
	db.BlockHashMaker(&contract, contract.BlockMeta.NodeUID)

	if ok, err := db.InsertOneBlock(known); !ok || err != nil {
		t.Fatalf("insert: %v %v", ok, err)
	}

	received := []db.Block{valid, known, badSignature, malformed, contract}
	got := filterNewBlocks(received, "node_2_test")

	if len(got) != 3 {
		t.Fatalf("filterNewBlocks returned %d blocks, want 3", len(got))
	}

	byID := map[string]db.Block{}
	for _, block := range got {
		byID[block.ID] = block
	}

	if _, ok := byID[known.ID]; ok {
		t.Error("filterNewBlocks returned a block the node already has")
	}
	if block, ok := byID[badSignature.ID]; !ok || !block.Boycott {
		t.Error("a block with an invalid signature should be kept and boycotted")
	}
	if block, ok := byID[contract.ID]; !ok {
		t.Fatal("the contract block was dropped")
	} else if block.BlockData.UseContract == "" || block.BlockData.ContractData.Name != "counter" {
		t.Fatalf("sync dropped the contract metadata: %+v", block.BlockData)
	}
}

func TestWSHandlerEndToEnd(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "e2e", 1700000000200)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	// --- add ---
	if err := writeRequest(conn, WS_Req{Method: MethodAdd, Block: block}); err != nil {
		t.Fatalf("write add: %v", err)
	}
	var addRes WS_Res
	if err := readResponse(conn, &addRes); err != nil {
		t.Fatalf("read add response: %v", err)
	}
	if !addRes.IsSuccess {
		t.Fatalf("server rejected the block")
	}

	// --- get ---
	if err := writeRequest(conn, WS_Req{Method: MethodGet, Block: db.Block{ID: block.ID}}); err != nil {
		t.Fatalf("write get: %v", err)
	}
	var getRes WS_Res
	if err := readResponse(conn, &getRes); err != nil {
		t.Fatalf("read get response: %v", err)
	}
	if !getRes.IsSuccess || getRes.Block.ID != block.ID {
		t.Fatalf("get response = %+v", getRes)
	}

	// --- sync ---
	tip := db.Block{BlockMeta: db.BlockMeta{TimeStamp: block.BlockMeta.TimeStamp - 1}}
	if err := writeRequest(conn, WS_Req{Method: MethodSync, Block: tip}); err != nil {
		t.Fatalf("write sync: %v", err)
	}
	var syncRes WS_Sync_Res
	if err := readResponse(conn, &syncRes); err != nil {
		t.Fatalf("read sync response: %v", err)
	}
	if !syncRes.IsSuccess || len(syncRes.Blocks) != 1 {
		t.Fatalf("sync response = %+v", syncRes)
	}
}

func TestWSHandlerClosesOnMalformedJSON(t *testing.T) {
	resetStore(t)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("{not json")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server must close the connection rather than crash the process.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func TestServerManagerBroadcast(t *testing.T) {
	// A broadcast to zero clients must be a no-op rather than a panic.
	ServerManagerVar.BroadcastMessageFromLocalServer(map[string]string{"hello": "world"})
}

func dialPeer(t *testing.T, httpURL, uid string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(httpURL, "http")

	header := http.Header{}
	header.Set("X-UID", uid)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	return conn
}

func writeRequest(conn *websocket.Conn, req WS_Req) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func readResponse(conn *websocket.Conn, out any) error {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
