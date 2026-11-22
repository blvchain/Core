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
	"blvchain/core/db/dbtest"
	"blvchain/core/utils"

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

	blocks, err := db.FindAllBlocks(bson.M{"_id": block.ID})
	if err != nil || len(blocks) != 1 {
		t.Fatalf("block was not stored: %v (%d)", err, len(blocks))
	}

	// A duplicate must be accepted idempotently.
	if duplicate := handleAddBlock("node_2_test", block); !duplicate.IsSuccess {
		t.Fatal("handleAddBlock rejected a duplicate instead of treating it as success")
	}
}

func TestHandleAddBlockDropsInvalidSignatures(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "payload", 1700000000003)
	block.BlockData.Signature = strings.Repeat("0", 128)
	db.BlockHashMaker(&block, block.BlockMeta.NodeUID)

	res := handleAddBlock("node_2_test", block)
	if res.IsSuccess {
		t.Fatal("handleAddBlock accepted a block with an invalid signature")
	}
	if blocks, _ := db.FindAllBlocks(bson.M{}); len(blocks) != 0 {
		t.Fatalf("a signature-invalid block was persisted (%d blocks)", len(blocks))
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

func TestHandleInventoryAndGetBlocks(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	genesis := db.MakeGenesisBlock()
	b1 := makeBlock(t, sender, receiver, "one", 1700000000010)
	b2 := makeBlock(t, sender, receiver, "two", 1700000000020)
	for _, block := range []db.Block{genesis, b1, b2} {
		if ok, err := db.InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	first := handleInventory("")
	if !first.IsSuccess {
		t.Fatal("handleInventory failed")
	}
	if len(first.IDs) != 3 {
		t.Fatalf("inventory returned %d IDs, want 3", len(first.IDs))
	}
	if first.HasMore {
		t.Fatal("inventory reported more pages for a single page of results")
	}

	// IDs must be strictly ascending, which is what makes cursor paging stable.
	for i := 1; i < len(first.IDs); i++ {
		if first.IDs[i-1] >= first.IDs[i] {
			t.Fatalf("inventory IDs are not ascending: %q then %q", first.IDs[i-1], first.IDs[i])
		}
	}

	// Paging from the last ID returns nothing.
	next := handleInventory(first.IDs[len(first.IDs)-1])
	if !next.IsSuccess || len(next.IDs) != 0 || next.HasMore {
		t.Fatalf("inventory after the last ID = %+v", next)
	}

	blocks := handleGetBlocks(first.IDs)
	if !blocks.IsSuccess || len(blocks.Blocks) != 3 {
		t.Fatalf("getblocks returned %+v", blocks)
	}
}

func TestMergeBlocksDropsInvalid(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)

	valid := makeBlock(t, sender, receiver, "valid", 1700000000100)
	relink(t, &valid, config.FIRST_BLOCK_HASH)
	if err := db.BlockValidator(valid); err != nil {
		t.Fatalf("valid block does not validate: %v", err)
	}

	badSignature := makeBlock(t, sender, receiver, "bad", 1700000000101)
	badSignature.BlockData.Signature = strings.Repeat("0", 128)
	db.BlockHashMaker(&badSignature, badSignature.BlockMeta.NodeUID)

	malformed := makeBlock(t, sender, receiver, "malformed", 1700000000102)
	malformed.ID = "broken"

	inserted := mergeBlocks([]db.Block{valid, badSignature, malformed}, "node_2_test")
	if inserted != 1 {
		t.Fatalf("mergeBlocks inserted %d blocks, want 1", inserted)
	}
	if blocks, _ := db.FindAllBlocks(bson.M{}); len(blocks) != 1 || blocks[0].ID != valid.ID {
		t.Fatalf("unexpected stored blocks: %+v", blocks)
	}
}

func TestMergeBlocksDefersUnknownPredecessor(t *testing.T) {
	resetStore(t)

	genesis := db.MakeGenesisBlock()
	sender := newIdentity(t)
	receiver := newIdentity(t)

	b1 := makeBlock(t, sender, receiver, "one", 1700000000200)
	relink(t, &b1, genesis.ID)

	b2 := makeBlock(t, sender, receiver, "two", 1700000000210)
	relink(t, &b2, b1.ID)

	if ok, err := db.InsertOneBlock(genesis); !ok || err != nil {
		t.Fatalf("insert genesis: %v %v", ok, err)
	}

	// b2 arrives before b1: its predecessor is unknown, so it must be deferred.
	if inserted := mergeBlocks([]db.Block{b2}, "node_2_test"); inserted != 0 {
		t.Fatalf("mergeBlocks stored a block with an unknown predecessor")
	}
	if blocks, _ := db.FindAllBlocks(bson.M{}); len(blocks) != 1 {
		t.Fatalf("deferred block was persisted: %d blocks", len(blocks))
	}

	// Once b1 arrives, b2 becomes valid on a later round.
	if inserted := mergeBlocks([]db.Block{b1}, "node_2_test"); inserted != 1 {
		t.Fatalf("mergeBlocks did not store b1")
	}
	if inserted := mergeBlocks([]db.Block{b2}, "node_2_test"); inserted != 1 {
		t.Fatalf("mergeBlocks did not store b2 after its predecessor arrived")
	}
}

// TestSyncReconcilesMissingEarlierRecords is the regression test for NET-1.
//
// Node B holds a later block (b3) but is missing earlier ones (b1, b2). The old
// timestamp sync asked "what is newer than my tip?" and would therefore never
// return b1/b2, permanently diverging. The inventory exchange returns every ID,
// so B discovers b1/b2 and pulls them explicitly.
func TestSyncReconcilesMissingEarlierRecords(t *testing.T) {
	genesis := db.MakeGenesisBlock()
	sender := newIdentity(t)
	receiver := newIdentity(t)

	b1 := makeBlock(t, sender, receiver, "one", 1700000000300)
	relink(t, &b1, genesis.ID)

	b2 := makeBlock(t, sender, receiver, "two", 1700000000310)
	relink(t, &b2, b1.ID)

	b3 := makeBlock(t, sender, receiver, "three", 1700000000320)
	relink(t, &b3, b2.ID)

	// Node A has the full chain.
	storeA := dbtest.New()
	db.SetCollection(storeA)
	for _, block := range []db.Block{genesis, b1, b2, b3} {
		if ok, err := db.InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("seed A: %v %v", ok, err)
		}
	}

	// Node B has only the genesis and the *latest* block.
	storeB := dbtest.New()
	db.SetCollection(storeB)
	for _, block := range []db.Block{genesis, b3} {
		if ok, err := db.InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("seed B: %v %v", ok, err)
		}
	}
	t.Cleanup(db.ResetCollection)

	// While A is the active store, page its inventory and fetch what B lacks.
	db.SetCollection(storeA)
	inventory := handleInventory("")
	if !inventory.IsSuccess {
		t.Fatal("inventory failed")
	}

	bHas := map[string]bool{genesis.ID: true, b3.ID: true}
	var missing []string
	for _, id := range inventory.IDs {
		if !bHas[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) != 2 {
		t.Fatalf("B is missing %d blocks, want 2 (b1 and b2)", len(missing))
	}

	fetched := handleGetBlocks(missing)
	if !fetched.IsSuccess || len(fetched.Blocks) != 2 {
		t.Fatalf("getblocks returned %+v", fetched)
	}

	// Now merge into B.
	db.SetCollection(storeB)
	inserted := mergeBlocks(fetched.Blocks, "node_2_test")
	if inserted != 2 {
		t.Fatalf("mergeBlocks inserted %d blocks, want 2", inserted)
	}
	if blocks, _ := db.FindAllBlocks(bson.M{}); len(blocks) != 4 {
		t.Fatalf("B has %d blocks after sync, want 4", len(blocks))
	}

	// Re-running the same merge is a no-op (idempotent replication).
	if inserted := mergeBlocks(fetched.Blocks, "node_2_test"); inserted != 0 {
		t.Fatalf("a repeated merge inserted %d blocks, want 0", inserted)
	}
}

// relink points a block at a new predecessor and re-signs it, because the
// predecessor is covered by the node signature.
func relink(t *testing.T, block *db.Block, preHash string) {
	t.Helper()
	block.BlockMeta.PreBlockHash = preHash
	if err := db.SignBlock(block, config.NODE_PRIVATE_KEY); err != nil {
		t.Fatalf("re-sign block: %v", err)
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

	// --- inventory + getblocks (the sync path) ---
	if err := writeRequest(conn, WS_Req{Method: MethodInventory}); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	var invRes WS_Inventory_Res
	if err := readResponse(conn, &invRes); err != nil {
		t.Fatalf("read inventory response: %v", err)
	}
	if !invRes.IsSuccess || len(invRes.IDs) != 1 || invRes.IDs[0] != block.ID {
		t.Fatalf("inventory response = %+v", invRes)
	}

	if err := writeRequest(conn, WS_Req{Method: MethodGetBlocks, IDs: invRes.IDs}); err != nil {
		t.Fatalf("write getblocks: %v", err)
	}
	var blocksRes WS_Blocks_Res
	if err := readResponse(conn, &blocksRes); err != nil {
		t.Fatalf("read getblocks response: %v", err)
	}
	if !blocksRes.IsSuccess || len(blocksRes.Blocks) != 1 {
		t.Fatalf("getblocks response = %+v", blocksRes)
	}
}

func TestWSHandlerRejectsUnauthenticatedPeer(t *testing.T) {
	resetStore(t)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	// The X-UID header passes the cheap pre-upgrade filter, but the peer cannot
	// complete the handshake without node_2's private key.
	conn, err := dialPeerWithKey(server.URL, "node_2_test", node1Priv)
	if err == nil {
		t.Fatal("the server accepted a peer that could not sign the challenge")
	}
	if conn != nil {
		defer conn.Close()
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

func TestWSHandlerRateLimitsFlooding(t *testing.T) {
	resetStore(t)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	// The per-peer token bucket allows a burst and then closes the connection.
	closed := false
	for i := 0; i < 200; i++ {
		if err := writeRequest(conn, WS_Req{Method: MethodInventory}); err != nil {
			closed = true
			break
		}
		var res WS_Inventory_Res
		if err := readResponse(conn, &res); err != nil {
			closed = true
			break
		}
	}
	if !closed {
		t.Fatal("the server kept serving a peer that flooded it with requests")
	}
}

func TestWSHandlerRejectsOversizedMessage(t *testing.T) {
	resetStore(t)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	// One byte over the configured read limit.
	oversized := make([]byte, config.WS_READ_LIMIT_BYTES+1024)
	_ = conn.WriteMessage(websocket.TextMessage, oversized)

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

// ---------------------------------------------------------------------------
// Handshake helpers
// ---------------------------------------------------------------------------

func dialPeer(t *testing.T, httpURL, uid string) *websocket.Conn {
	t.Helper()

	priv := node2Priv
	if uid == config.SELF_UID {
		priv = node1Priv
	}
	conn, err := dialPeerWithKey(httpURL, uid, priv)
	if err != nil {
		t.Fatalf("handshake as %s: %v", uid, err)
	}
	return conn
}

func dialPeerWithKey(httpURL, uid, priv string) (*websocket.Conn, error) {
	wsURL := "ws" + strings.TrimPrefix(httpURL, "http")

	header := http.Header{}
	header.Set("X-UID", uid)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return nil, err
	}

	// Complete the challenge/response handshake as uid.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return conn, err
	}
	var challenge WS_Challenge
	if err := json.Unmarshal(data, &challenge); err != nil {
		return conn, err
	}
	if challenge.Method != MethodChallenge {
		return conn, errUnexpectedChallenge
	}

	pub, err := utils.DerivePubKeyHex(priv)
	if err != nil {
		return conn, err
	}
	signature, err := utils.Sign(priv, authMessage(challenge.Nonce, uid, pub))
	if err != nil {
		return conn, err
	}
	if err := writeJSON(conn, WS_Auth{Method: MethodAuth, UID: uid, PubKey: pub, Signature: signature}); err != nil {
		return conn, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err = conn.ReadMessage()
	if err != nil {
		return conn, err
	}
	var res WS_Auth_Res
	if err := json.Unmarshal(data, &res); err != nil {
		return conn, err
	}
	if !res.IsSuccess {
		return conn, errAuthenticationRejected
	}

	return conn, nil
}

var (
	errUnexpectedChallenge    = errTest("server did not send a challenge")
	errAuthenticationRejected = errTest("authentication rejected")
)

type errTest string

func (e errTest) Error() string { return string(e) }

func writeJSON(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func writeRequest(conn *websocket.Conn, req WS_Req) error { return writeJSON(conn, req) }

func readResponse(conn *websocket.Conn, out any) error {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
