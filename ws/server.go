package ws

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"encoding/json"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
)

func (cm *ServerManager) addClient(uid string, conn *safeConn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Replace any previous connection from the same node.
	if previous, ok := cm.clients[uid]; ok {
		_ = previous.Close()
	}
	cm.clients[uid] = conn
}

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

// WS_Server_Handler upgrades the request and serves one peer.
func WS_Server_Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Error: Failed to upgrade:", err)
		return
	}

	safe := newSafeConn(conn)
	defer safe.Close()

	clientUID := r.Header.Get("X-UID")
	ServerManagerVar.addClient(clientUID, safe)
	defer ServerManagerVar.removeClient(clientUID)
	logger.WS_S_LOGGER.Printf("Success: Node '%s' connected", clientUID)

	for {
		_, messageData, err := safe.read()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Error: Node '%s' disconnected: %v", clientUID, err)
			return
		}

		var msg WS_Req
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logger.WS_F_LOGGER.Printf("Error: Node '%s' sent malformed JSON: %v", clientUID, err)
			return
		}

		var reply any
		switch msg.Method {
		case MethodGet:
			reply = handleGetBlock(msg.Block)
		case MethodAdd:
			reply = handleAddBlock(clientUID, msg.Block)
		case MethodSync:
			reply = handleSyncBlocks(clientUID, msg.Block)
		default:
			logger.WS_F_LOGGER.Printf("Error: Node '%s' used unknown method '%s'", clientUID, msg.Method)
			continue
		}

		if err := safe.writeJSON(reply); err != nil {
			logger.WS_F_LOGGER.Printf("Error: could not answer node '%s': %v", clientUID, err)
			return
		}
	}
}

// handleGetBlock answers a "get" request: look up one block by its fields.
func handleGetBlock(query db.Block) WS_Res {
	res := WS_Res{IsSuccess: false}

	filter := []bson.M{}
	if query.BlockData.SenderUID != "" {
		filter = append(filter, bson.M{"blockData.senderUid": query.BlockData.SenderUID})
	}
	if query.BlockData.SenderRole != 0 {
		filter = append(filter, bson.M{"blockData.senderRole": query.BlockData.SenderRole})
	}
	if query.BlockData.SenderPubKey != "" {
		filter = append(filter, bson.M{"blockData.senderPubKey": query.BlockData.SenderPubKey})
	}
	if query.BlockData.ReceiverUID != "" {
		filter = append(filter, bson.M{"blockData.receiverUid": query.BlockData.ReceiverUID})
	}
	if query.BlockData.ReceiverRole != 0 {
		filter = append(filter, bson.M{"blockData.receiverRole": query.BlockData.ReceiverRole})
	}
	if query.ID != "" {
		filter = append(filter, bson.M{"_id": query.ID})
	}
	if query.BlockMeta.PreBlockHash != "" {
		filter = append(filter, bson.M{"blockMeta.preBlockHash": query.BlockMeta.PreBlockHash})
	}
	if query.BlockMeta.NodeUID != "" {
		filter = append(filter, bson.M{"blockMeta.nodeUid": query.BlockMeta.NodeUID})
	}

	if len(filter) == 0 {
		logger.WS_F_LOGGER.Printf("Error: empty get request from node")
		return res
	}

	blocks, err := db.FindManyBlocksLimited(bson.M{"$and": filter}, 0, 1)
	if err != nil || len(blocks) == 0 {
		logger.WS_F_LOGGER.Printf("Error: block '%v' not found", query.ID)
		return res
	}

	logger.WS_S_LOGGER.Printf("Success: sent block '%v' to a peer", query.ID)
	res.IsSuccess = true
	res.Block = blocks[0]
	return res
}

// handleAddBlock answers an "add" request: validate and store a replicated block.
func handleAddBlock(clientUID string, block db.Block) WS_Res {
	res := WS_Res{IsSuccess: false}

	if err := db.BlockStructValidator(block); err != nil {
		logger.WS_F_LOGGER.Printf("Error: malformed block from node '%v': %v", clientUID, err)
		return res
	}

	existing, err := db.FindAllBlocks(bson.M{"_id": block.ID})
	if err != nil {
		logger.INTERNAL_LOGGER.Printf("Error: could not check block '%v': %v", block.ID, err)
		return res
	}
	if len(existing) > 0 {
		logger.WS_F_LOGGER.Printf("Error: node '%v' sent duplicate block '%v'", clientUID, block.ID)
		return res
	}

	if err := db.BlockValidator(block); err != nil {
		logger.WS_F_LOGGER.Printf("WARNING!!!: Quarantining block '%v' from node '%v': %v", block.ID, clientUID, err)
		block.Boycott = true
	} else {
		block.Boycott = false
	}

	ok, insertErr := db.InsertOneBlock(block)
	if !ok {
		logger.INTERNAL_LOGGER.Printf("Error: could not store block '%v' from node '%v': %v", block.ID, clientUID, insertErr)
		return res
	}

	logger.WS_S_LOGGER.Printf("Success: stored block '%v' from node '%v'", block.ID, clientUID)
	res.IsSuccess = true
	res.Block = block
	return res
}

// handleSyncBlocks answers a "sync" request with the blocks newer than the
// peer's tip. The tip block itself is excluded so a peer that is already up to
// date receives nothing and can stop syncing.
func handleSyncBlocks(clientUID string, tip db.Block) WS_Sync_Res {
	res := WS_Sync_Res{IsSuccess: false}

	newerThanTip := bson.M{
		"$or": []bson.M{
			{"blockMeta.timeStamp": bson.M{"$gt": tip.BlockMeta.TimeStamp}},
			{
				"$and": []bson.M{
					{"blockMeta.timeStamp": tip.BlockMeta.TimeStamp},
					{"_id": bson.M{"$ne": tip.ID}},
				},
			},
		},
	}

	filter := bson.M{
		"$and": []bson.M{
			newerThanTip,
			{"blockMeta.preBlockHash": bson.M{"$ne": config.GENESIS_BLOCK_PREHASH}},
		},
	}

	blocks, err := db.FindManyBlocksLimitedASE(filter, 0, config.MAX_LIMIT_OF_DATA_SYNC)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error: could not read blocks for node '%v': %v", clientUID, err)
		return res
	}
	if len(blocks) == 0 {
		return res
	}

	logger.WS_S_LOGGER.Printf("Success: sent %v block(s) made after %v to node '%v'", len(blocks), tip.BlockMeta.TimeStamp, clientUID)
	res.IsSuccess = true
	res.Blocks = blocks
	return res
}
