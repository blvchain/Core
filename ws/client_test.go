package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"blvchain/core/config"
	"blvchain/core/db"

	"go.mongodb.org/mongo-driver/bson"
)

func newClientManager() *ClientManager {
	return &ClientManager{servers: make(map[string]*safeConn)}
}

func TestClientManagerSnapshotIsIsolated(t *testing.T) {
	cm := newClientManager()

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	cm.set("node_2_test", newSafeConn(conn))

	snapshot := cm.snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot has %d connections, want 1", len(snapshot))
	}

	// Removing from the manager must not mutate an existing snapshot.
	cm.remove("node_2_test")
	if len(snapshot) != 1 {
		t.Fatalf("snapshot was mutated: %d connections", len(snapshot))
	}
	if len(cm.snapshot()) != 0 {
		t.Fatal("the connection was not removed from the manager")
	}
}

func TestClientManagerSetReplacesConnection(t *testing.T) {
	cm := newClientManager()

	first := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer second.Close()

	connA := dialPeer(t, first.URL, "node_2_test")
	defer connA.Close()
	connB := dialPeer(t, second.URL, "node_2_test")
	defer connB.Close()

	cm.set("node_2_test", newSafeConn(connA))
	cm.set("node_2_test", newSafeConn(connB))

	if got := len(cm.snapshot()); got != 1 {
		t.Fatalf("manager has %d connections, want 1", got)
	}
}

func TestClientManagerOperationsWithoutPeers(t *testing.T) {
	cm := newClientManager()

	if cm.SendMessageToOneServer("missing", map[string]string{"a": "b"}) {
		t.Fatal("SendMessageToOneServer reported success without a connection")
	}
	if responses := cm.AddBlockToServers(db.Block{}); len(responses) != 0 {
		t.Fatalf("AddBlockToServers returned %d responses, want 0", len(responses))
	}
	if blocks := cm.GetBlockFromServers("abc"); len(blocks) != 0 {
		t.Fatalf("GetBlockFromServers returned %d blocks, want 0", len(blocks))
	}
}

func TestConnectToOneServerFailure(t *testing.T) {
	cm := newClientManager()

	// Port 1 is not listening; the dial must fail fast.
	ok := cm.ConnectToOneServer(config.Dns_seed_config{UID: "node_2_test", Address: "ws://127.0.0.1:1"})
	if ok {
		t.Fatal("ConnectToOneServer reported success for an unreachable address")
	}
	if len(cm.snapshot()) != 0 {
		t.Fatal("a failed dial left a connection behind")
	}
}

func TestConnectToOneServerSkipsSelf(t *testing.T) {
	cm := newClientManager()

	if cm.ConnectToOneServer(config.Dns_seed_config{UID: config.SELF_UID, Address: "ws://127.0.0.1:1"}) {
		t.Fatal("a node must not connect to itself")
	}
}

func TestDisconnectFromServers(t *testing.T) {
	cm := newClientManager()

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	cm.set("node_2_test", newSafeConn(conn))

	cm.DisconnectFromServers()
	if len(cm.snapshot()) != 0 {
		t.Fatal("DisconnectFromServers left connections behind")
	}
}

func TestAddBlockToServersEndToEnd(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "replicated", 1700000001000)

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	cm := newClientManager()
	cm.set("node_2_test", newSafeConn(conn))

	responses := cm.AddBlockToServers(block)
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}
	if !responses[0].IsSuccess {
		t.Fatal("the peer rejected the replicated block")
	}

	var stored db.Block
	if err := db.FindOneBlock(block.ID, &stored); err != nil {
		t.Fatalf("the block was not stored on the peer: %v", err)
	}
	if stored.BlockData.Data != "replicated" {
		t.Fatalf("stored data = %q", stored.BlockData.Data)
	}

	// Reading it back through the peer must work too.
	blocks := cm.GetBlockFromServers(block.ID)
	if len(blocks) != 1 || blocks[0].ID != block.ID {
		t.Fatalf("GetBlockFromServers returned %+v", blocks)
	}
}

func TestSendMessageToOneServerEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	cm := newClientManager()
	cm.set("node_2_test", newSafeConn(conn))

	if !cm.SendMessageToOneServer("node_2_test", WS_Req{Method: MethodGet, Block: db.Block{ID: "abc"}}) {
		t.Fatal("SendMessageToOneServer failed")
	}
}

func TestSafeConnPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	safe := newSafeConn(conn)
	if err := safe.ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestLatestBlock(t *testing.T) {
	resetStore(t)

	if _, err := latestBlock(); err != errNoBlocks {
		t.Fatalf("latestBlock on an empty chain = %v, want errNoBlocks", err)
	}

	sender := newIdentity(t)
	receiver := newIdentity(t)
	older := makeBlock(t, sender, receiver, "older", 1700000002000)
	newer := makeBlock(t, sender, receiver, "newer", 1700000003000)
	for _, block := range []db.Block{older, newer} {
		if ok, err := db.InsertOneBlock(block); !ok || err != nil {
			t.Fatalf("insert: %v %v", ok, err)
		}
	}

	tip, err := latestBlock()
	if err != nil {
		t.Fatalf("latestBlock: %v", err)
	}
	if tip.ID != newer.ID {
		t.Fatalf("tip = %q, want %q", tip.ID, newer.ID)
	}
}

func TestSyncRoundWithoutPeers(t *testing.T) {
	resetStore(t)

	cm := newClientManager()
	if !cm.syncRound() {
		t.Fatal("syncRound with no peers should report the chain as up to date")
	}
}

func TestSyncRoundWithPeer(t *testing.T) {
	resetStore(t)

	sender := newIdentity(t)
	receiver := newIdentity(t)
	block := makeBlock(t, sender, receiver, "sync-me", 1700000004000)
	if ok, err := db.InsertOneBlock(block); !ok || err != nil {
		t.Fatalf("insert: %v %v", ok, err)
	}

	server := httptest.NewServer(http.HandlerFunc(WS_Server_Handler))
	defer server.Close()

	conn := dialPeer(t, server.URL, "node_2_test")
	defer conn.Close()

	cm := newClientManager()
	cm.set("node_2_test", newSafeConn(conn))

	// The peer has the same block, so there is nothing new to pull.
	if !cm.syncRound() {
		t.Fatal("syncRound should be a no-op when the peer has no newer blocks")
	}
	if got := len(mustFindAll(t)); got != 1 {
		t.Fatalf("sync duplicated a block: %d stored", got)
	}
}

func mustFindAll(t *testing.T) []db.Block {
	t.Helper()

	blocks, err := db.FindAllBlocks(bson.M{})
	if err != nil {
		t.Fatalf("FindAllBlocks: %v", err)
	}
	return blocks
}
