package ws

import (
	"errors"
	"net/http"
	"time"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"

	"github.com/gorilla/websocket"
)

// errNoBlocks is returned when the local chain has not produced a block yet.
var errNoBlocks = errors.New("no local blocks")

// addClient registers an authenticated inbound peer, replacing any previous
// connection from the same node.
func (cm *ServerManager) addClient(uid string, conn *safeConn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if previous, ok := cm.clients[uid]; ok {
		_ = previous.Close()
	}
	cm.clients[uid] = conn
}

// removeClient forgets an inbound peer.
func (cm *ServerManager) removeClient(uid string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	delete(cm.clients, uid)
}

// BroadcastMessageFromLocalServer sends a message to every connected peer.
func (cm *ServerManager) BroadcastMessageFromLocalServer(message any) {
	cm.mutex.RLock()
	conns := make([]*safeConn, 0, len(cm.clients))
	for _, conn := range cm.clients {
		conns = append(conns, conn)
	}
	cm.mutex.RUnlock()

	for _, conn := range conns {
		if err := conn.writeJSON(message); err != nil {
			logger.WS_F_LOGGER.Printf("Error: could not broadcast message: %v", err)
		}
	}
}

// dial opens an outbound WebSocket connection to a seed node and authenticates
// it with a challenge/response handshake before returning.
func dial(seed config.Dns_seed_config) (*safeConn, error) {
	header := http.Header{}
	header.Set("X-UID", config.SELF_UID)

	conn, _, err := websocket.DefaultDialer.Dial(seed.Address, header)
	if err != nil {
		return nil, err
	}

	safe := newSafeConn(conn)
	if err := authenticateClient(safe, seed); err != nil {
		_ = safe.Close()
		return nil, err
	}
	return safe, nil
}

// snapshot returns a copy of the current server connections so callers can do
// network I/O without holding the manager lock.
func (cm *ClientManager) snapshot() map[string]*safeConn {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	out := make(map[string]*safeConn, len(cm.servers))
	for uid, conn := range cm.servers {
		out[uid] = conn
	}
	return out
}

func (cm *ClientManager) set(uid string, conn *safeConn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if previous, ok := cm.servers[uid]; ok && previous != conn {
		_ = previous.Close()
	}
	cm.servers[uid] = conn
}

func (cm *ClientManager) remove(uid string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if conn, ok := cm.servers[uid]; ok {
		_ = conn.Close()
		delete(cm.servers, uid)
	}
}

// ConnectToServers connects to every seed node except this one.
func (cm *ClientManager) ConnectToServers(dnsSeeds []config.Dns_seed_config) {
	for _, seed := range dnsSeeds {
		if seed.UID == config.SELF_UID {
			continue
		}

		conn, err := dial(seed)
		if err != nil {
			logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v", seed.UID, seed.Address, err)
			continue
		}

		cm.set(seed.UID, conn)
		logger.WS_S_LOGGER.Printf("Connected to server %s (%s)", seed.UID, seed.Address)
	}
}

// ConnectToOneServer connects to a single seed node. It reports whether the
// connection was established.
func (cm *ClientManager) ConnectToOneServer(seed config.Dns_seed_config) bool {
	if seed.UID == config.SELF_UID {
		return false
	}

	conn, err := dial(seed)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v", seed.UID, seed.Address, err)
		return false
	}

	cm.set(seed.UID, conn)
	logger.WS_S_LOGGER.Printf("Connected to server %s (%s)", seed.UID, seed.Address)
	return true
}

// DisconnectFromServers closes every outbound connection.
func (cm *ClientManager) DisconnectFromServers() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for uid, conn := range cm.servers {
		_ = conn.Close()
		logger.WS_S_LOGGER.Printf("Disconnected from server %v", uid)
		delete(cm.servers, uid)
	}
}

// MonitorAndReconnectToServers pings the peers periodically and re-dials the
// ones that are gone. Network I/O happens outside the manager lock.
func MonitorAndReconnectToServers(cm *ClientManager) {
	for {
		time.Sleep(config.RECONNECT_SLEEP_TIME * time.Second)

		connections := cm.snapshot()

		for _, seed := range config.DNS_SEED_LIST {
			if seed.UID == config.SELF_UID {
				continue
			}

			if conn, ok := connections[seed.UID]; ok {
				if err := conn.ping(); err == nil {
					continue
				}
				// Stale connection: drop it before re-dialing.
				cm.remove(seed.UID)
				logger.WS_F_LOGGER.Printf("Connection to %v is stale, reconnecting", seed.Address)
			}

			conn, err := dial(seed)
			if err != nil {
				logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v", seed.UID, seed.Address, err)
				continue
			}
			cm.set(seed.UID, conn)
			logger.WS_S_LOGGER.Printf("Connected to server %s (%s)", seed.UID, seed.Address)
		}
	}
}

// FirstTimeSyncData blocks until every peer reports that it has nothing new.
func FirstTimeSyncData(cm *ClientManager) bool {
	for {
		time.Sleep(1 * time.Second)
		if cm.syncRound() {
			return true
		}
	}
}

// SyncData runs forever, asking the peers for new blocks every few seconds.
func SyncData(cm *ClientManager) {
	for {
		time.Sleep(config.SYNC_DATA_SLEEP_TIME * time.Second)
		cm.syncRound()
	}
}

// syncRound performs one pass over every peer. It reports true when every peer
// is up to date (no missing blocks and no deferred blocks).
func (cm *ClientManager) syncRound() bool {
	upToDate := true

	for uid, conn := range cm.snapshot() {
		peerUpToDate, err := syncPeer(conn, uid)
		if err != nil {
			logger.WS_F_LOGGER.Printf("Error: sync request to node '%v' failed: %v", uid, err)
			// Drop the dead connection so the reconnect monitor can replace it.
			cm.remove(uid)
			continue
		}
		if !peerUpToDate {
			upToDate = false
		}
	}

	return upToDate
}

// syncPeer reconciles one peer using an inventory exchange (NET-1).
//
// Instead of asking "what is newer than my timestamp?", which can permanently
// strand a block whose timestamp is older than the requester's tip, the
// requester pages through the peer's complete, stable, _id-ordered inventory
// and explicitly asks for the IDs it is missing. The exchange is idempotent:
// already-known IDs are ignored and re-running it is a no-op.
func syncPeer(conn *safeConn, uid string) (bool, error) {
	upToDate := true
	cursor := ""
	fetched := 0
	pages := 0

	for {
		if pages >= config.SYNC_MAX_INVENTORY_PAGES_PER_ROUND {
			// Bound the work done for one peer in one round; resume next round.
			return false, nil
		}
		pages++

		var inventory WS_Inventory_Res
		if err := conn.request(WS_Req{Method: MethodInventory, Cursor: cursor}, &inventory); err != nil {
			return false, err
		}
		if !inventory.IsSuccess {
			return false, errors.New("inventory request failed")
		}

		missing := missingIDs(inventory.IDs)
		if len(missing) > 0 {
			upToDate = false
		}

		for start := 0; start < len(missing); start += config.SYNC_BLOCK_BATCH_SIZE {
			if fetched >= config.SYNC_MAX_BLOCKS_PER_ROUND {
				// The rest arrives on the next round; report "not up to date".
				return false, nil
			}

			end := start + config.SYNC_BLOCK_BATCH_SIZE
			if end > len(missing) {
				end = len(missing)
			}
			batch := missing[start:end]

			var blocksRes WS_Blocks_Res
			if err := conn.request(WS_Req{Method: MethodGetBlocks, IDs: batch}, &blocksRes); err != nil {
				return false, err
			}
			if !blocksRes.IsSuccess {
				return false, errors.New("getblocks request failed")
			}

			fetched += len(batch)
			inserted := mergeBlocks(blocksRes.Blocks, uid)
			if inserted > 0 {
				logger.WS_S_LOGGER.Printf("Success: stored %v block(s) from node '%v'", inserted, uid)
			}
		}

		if !inventory.HasMore {
			break
		}
		if inventory.NextCursor == "" || inventory.NextCursor == cursor {
			// Malformed paginator; stop instead of looping forever.
			return false, errors.New("inventory pagination did not advance")
		}
		cursor = inventory.NextCursor
	}

	return upToDate, nil
}

// missingIDs returns the subset of ids that the local node does not have.
func missingIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	existing, err := db.FindExistingIDs(ids)
	if err != nil {
		// If the lookup fails, treat everything as missing; the insert path is
		// idempotent, so this is safe (just wasteful).
		return ids
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !existing[id] {
			out = append(out, id)
		}
	}
	return out
}

// mergeBlocks validates a batch of replicated blocks and stores the ones whose
// predecessor is available. Blocks are retried across passes so a batch whose
// predecessors are also in the batch is inserted in dependency order. Anything
// still blocked is left for the next sync round rather than stored with a
// dangling link. It returns the number of newly stored blocks.
func mergeBlocks(blocks []db.Block, peerUID string) int {
	if len(blocks) == 0 {
		return 0
	}

	seen := make(map[string]bool, len(blocks))
	pending := make([]db.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.ID == "" || seen[block.ID] {
			continue
		}
		seen[block.ID] = true

		if err := db.BlockValidator(block); err != nil {
			// Structurally invalid or bad signature: drop, never persist.
			logger.WS_F_LOGGER.Printf("Dropping invalid block '%v' from node '%v': %v", block.ID, peerUID, err)
			continue
		}
		pending = append(pending, block)
	}

	inserted := 0
	genesisID := config.FIRST_BLOCK_HASH

	for pass := 0; pass < config.SYNC_MAX_PENDING_PASSES && len(pending) > 0; pass++ {
		progress := false
		remaining := make([]db.Block, 0, len(pending))

		for _, block := range pending {
			// The chain link must resolve: either it points at the shared genesis
			// block or at a block we already hold.
			if block.BlockMeta.PreBlockHash != genesisID {
				known, err := db.BlockExists(block.BlockMeta.PreBlockHash)
				if err != nil || !known {
					remaining = append(remaining, block)
					continue
				}
			}

			stored, err := storeBlock(block)
			if err != nil {
				logger.INTERNAL_LOGGER.Printf("Error: could not store block '%v': %v", block.ID, err)
				remaining = append(remaining, block)
				continue
			}
			if stored {
				inserted++
				progress = true
			}
		}

		pending = remaining
		if !progress {
			break
		}
	}

	for _, block := range pending {
		logger.WS_F_LOGGER.Printf("Deferring block '%v' from node '%v': predecessor not available yet", block.ID, peerUID)
	}
	return inserted
}

// storeBlock inserts a block unless it is already present. It is the idempotent
// primitive shared by replication and sync.
func storeBlock(block db.Block) (bool, error) {
	exists, err := db.BlockExists(block.ID)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	return db.InsertOneBlock(block)
}

func latestBlock() (db.Block, error) {
	blocks, err := db.FindManyBlocksLimited(config.NO_FILTER, 0, 1)
	if err != nil {
		return db.Block{}, err
	}
	if len(blocks) == 0 {
		return db.Block{}, errNoBlocks
	}
	return blocks[0], nil
}

// SendMessageToOneServer sends a payload to a single peer.
func (cm *ClientManager) SendMessageToOneServer(uid string, message any) bool {
	conn, ok := cm.snapshot()[uid]
	if !ok {
		logger.WS_F_LOGGER.Printf("No connection found for server UID: %s", uid)
		return false
	}

	if err := conn.writeJSON(message); err != nil {
		logger.WS_F_LOGGER.Printf("Failed to send message to server %s: %v", uid, err)
		cm.remove(uid)
		return false
	}

	logger.WS_S_LOGGER.Printf("Message sent to server %s: %s", uid, message)
	return true
}

// GetBlockFromServers asks every peer for a block by hash.
func (cm *ClientManager) GetBlockFromServers(blockHash string) []db.Block {
	req := WS_Req{Method: MethodGet, Block: db.Block{ID: blockHash}}

	var blocks []db.Block
	for uid, conn := range cm.snapshot() {
		var response WS_Res
		if err := conn.request(req, &response); err != nil {
			logger.WS_F_LOGGER.Printf("Error: get request to node '%v' failed: %v", uid, err)
			cm.remove(uid)
			continue
		}
		if response.IsSuccess {
			blocks = append(blocks, response.Block)
		}
	}
	return blocks
}

// AddBlockToServers replicates a block to every peer and collects the replies.
func (cm *ClientManager) AddBlockToServers(block db.Block) []WS_Res {
	req := WS_Req{Method: MethodAdd, Block: block}

	var responses []WS_Res
	for uid, conn := range cm.snapshot() {
		var response WS_Res
		if err := conn.request(req, &response); err != nil {
			logger.WS_F_LOGGER.Printf("Error: could not replicate block '%v' to node '%v': %v", block.ID, uid, err)
			cm.remove(uid)
			continue
		}
		if !response.IsSuccess {
			logger.WS_F_LOGGER.Printf("Error: node '%v' rejected block '%v'", uid, block.ID)
		}
		responses = append(responses, response)
	}
	return responses
}
