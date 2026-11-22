package ws

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
)

// errNoBlocks is returned when the local chain has not produced a block yet.
var errNoBlocks = errors.New("no local blocks")

// dial opens an outbound WebSocket connection to a seed node.
func dial(seed config.Dns_seed_config) (*safeConn, error) {
	header := http.Header{}
	header.Set("X-UID", config.SELF_UID)

	conn, _, err := websocket.DefaultDialer.Dial(seed.Address, header)
	if err != nil {
		return nil, err
	}
	return newSafeConn(conn), nil
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

// syncRound performs one pass over every peer. It reports true when no peer had
// new blocks (i.e. the local chain is up to date).
func (cm *ClientManager) syncRound() bool {
	upToDate := true

	for uid, conn := range cm.snapshot() {
		tip, err := latestBlock()
		if err != nil {
			logger.INTERNAL_LOGGER.Printf("Error: could not read the local chain tip: %v", err)
			continue
		}

		var response WS_Sync_Res
		if err := conn.request(WS_Req{Method: MethodSync, Block: tip}, &response); err != nil {
			logger.WS_F_LOGGER.Printf("Error: sync request to node '%v' failed: %v", uid, err)
			// Drop the dead connection so the reconnect monitor can replace it.
			cm.remove(uid)
			continue
		}

		if !response.IsSuccess || len(response.Blocks) == 0 {
			continue
		}

		newBlocks := filterNewBlocks(response.Blocks, uid)
		if len(newBlocks) < config.MIN_LIMIT_OF_DATA_SYNC {
			continue
		}

		upToDate = false

		blocks := make([]interface{}, len(newBlocks))
		for i, block := range newBlocks {
			blocks[i] = block
		}

		if _, err := db.InsertManyBlock(blocks); err != nil {
			logger.INTERNAL_LOGGER.Printf("Error: could not store %v block(s) from node '%v': %v", len(newBlocks), uid, err)
			continue
		}

		logger.WS_S_LOGGER.Printf("Success: stored %v block(s) made after %v from node '%v'", len(newBlocks), tip.BlockMeta.TimeStamp, uid)
	}

	return upToDate
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

// filterNewBlocks validates the blocks received from a peer and drops the ones
// the local node already has. Structurally invalid blocks are discarded;
// signature failures are kept with Boycott=true so the chain can audit them.
func filterNewBlocks(received []db.Block, peerUID string) []db.Block {
	if len(received) == 0 {
		return nil
	}

	ids := make([]string, 0, len(received))
	for _, block := range received {
		ids = append(ids, block.ID)
	}

	known := make(map[string]bool)
	if existing, err := db.FindAllBlocks(bson.M{"_id": bson.M{"$in": ids}}); err == nil {
		for _, block := range existing {
			known[block.ID] = true
		}
	}

	out := make([]db.Block, 0, len(received))
	for _, block := range received {
		if known[block.ID] {
			continue
		}

		candidate := db.Block{
			ID: block.ID,
			BlockMeta: db.BlockMeta{
				PreBlockHash: block.BlockMeta.PreBlockHash,
				NodeUID:      block.BlockMeta.NodeUID,
				TimeStamp:    block.BlockMeta.TimeStamp,
			},
			BlockData: db.BlockData{
				SenderUID:    block.BlockData.SenderUID,
				SenderRole:   block.BlockData.SenderRole,
				SenderPubKey: block.BlockData.SenderPubKey,
				Signature:    block.BlockData.Signature,
				ReceiverUID:  block.BlockData.ReceiverUID,
				ReceiverRole: block.BlockData.ReceiverRole,
				Data:         block.BlockData.Data,
				UseContract:  block.BlockData.UseContract,
				ContractData: block.BlockData.ContractData,
				TimeStamp:    block.BlockData.TimeStamp,
			},
		}

		if err := db.BlockStructValidator(candidate); err != nil {
			logger.WS_F_LOGGER.Printf("WARNING!!!: Dropping malformed block from node '%v': %v", peerUID, err)
			continue
		}

		if err := db.BlockValidator(candidate); err != nil {
			logger.WS_F_LOGGER.Printf("WARNING!!!: Quarantining block '%v' from node '%v': %v", candidate.ID, peerUID, err)
			candidate.Boycott = true
		}

		out = append(out, candidate)
	}

	return out
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
