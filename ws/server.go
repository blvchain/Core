package ws

import (
	"encoding/json"
	"net/http"

	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/time/rate"
)

// WS_Server_Handler upgrades the request, authenticates the peer and then serves
// one peer connection.
func WS_Server_Handler(w http.ResponseWriter, r *http.Request) {
	// Bound the number of concurrent inbound peers before doing any work.
	select {
	case inboundSem <- struct{}{}:
		defer func() { <-inboundSem }()
	default:
		logger.WS_F_LOGGER.Println("Refusing inbound peer: connection limit reached")
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Error: Failed to upgrade:", err)
		return
	}

	safe := newSafeConn(conn)
	defer safe.Close()

	// NET-2: the X-UID header is only a pre-upgrade filter. The real identity is
	// established by a challenge/response signature below.
	clientUID, err := authenticatePeer(safe)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error: peer authentication failed: %v", err)
		return
	}

	ServerManagerVar.addClient(clientUID, safe)
	defer ServerManagerVar.removeClient(clientUID)
	logger.WS_S_LOGGER.Printf("Success: Node '%s' connected", clientUID)

	// NET-4: per-peer token bucket. A peer that floods the connection is closed.
	limiter := rate.NewLimiter(rate.Limit(config.WS_MESSAGES_PER_SECOND), config.WS_MESSAGE_BURST)

	for {
		_, messageData, err := safe.read()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Error: Node '%s' disconnected: %v", clientUID, err)
			return
		}

		if !limiter.Allow() {
			logger.WS_F_LOGGER.Printf("Error: Node '%s' exceeded the message rate limit; closing", clientUID)
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
		case MethodInventory:
			reply = handleInventory(msg.Cursor)
		case MethodGetBlocks:
			reply = handleGetBlocks(msg.IDs)
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
//
// A block that fails validation is dropped, not quarantined. Signature-invalid
// records are never persisted (NET-4): keeping them would let any authenticated
// peer exhaust storage with garbage.
func handleAddBlock(clientUID string, block db.Block) WS_Res {
	res := WS_Res{IsSuccess: false}

	if err := db.BlockStructValidator(block); err != nil {
		logger.WS_F_LOGGER.Printf("Error: malformed block from node '%v': %v", clientUID, err)
		return res
	}

	if err := db.BlockValidator(block); err != nil {
		logger.WS_F_LOGGER.Printf("Error: dropping invalid block '%v' from node '%v': %v", block.ID, clientUID, err)
		return res
	}

	// The predecessor must already be known. A block whose link cannot be
	// resolved is not stored here; the inventory sync will deliver it once the
	// predecessor is present.
	if block.BlockMeta.PreBlockHash != config.FIRST_BLOCK_HASH {
		known, err := db.BlockExists(block.BlockMeta.PreBlockHash)
		if err != nil {
			logger.INTERNAL_LOGGER.Printf("Error: could not check predecessor of '%v': %v", block.ID, err)
			return res
		}
		if !known {
			logger.WS_F_LOGGER.Printf("Error: node '%v' sent block '%v' with an unknown predecessor", clientUID, block.ID)
			return res
		}
	}

	ok, insertErr := storeBlock(block)
	if insertErr != nil {
		logger.INTERNAL_LOGGER.Printf("Error: could not store block '%v' from node '%v': %v", block.ID, clientUID, insertErr)
		return res
	}
	if !ok {
		// Already present: replication is idempotent, so this is success.
		res.IsSuccess = true
		res.Block = block
		return res
	}

	logger.WS_S_LOGGER.Printf("Success: stored block '%v' from node '%v'", block.ID, clientUID)
	res.IsSuccess = true
	res.Block = block
	return res
}

// handleInventory answers an "inventory" request with one page of block IDs in
// ascending _id order. Paging on the immutable _id is stable even while other
// blocks are being inserted, so a peer can never be skipped past a page
// boundary (NET-1).
func handleInventory(cursor string) WS_Inventory_Res {
	res := WS_Inventory_Res{IsSuccess: false}

	ids, err := db.FindBlockIDs(cursor, config.SYNC_INVENTORY_PAGE_SIZE)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error: could not read the block inventory: %v", err)
		return res
	}

	res.IsSuccess = true
	res.IDs = ids
	if int64(len(ids)) >= config.SYNC_INVENTORY_PAGE_SIZE {
		res.HasMore = true
		res.NextCursor = ids[len(ids)-1]
	}
	return res
}

// handleGetBlocks answers a "getblocks" request. The number of IDs is bounded so
// a peer cannot ask for an unbounded result set.
func handleGetBlocks(ids []string) WS_Blocks_Res {
	res := WS_Blocks_Res{IsSuccess: false}

	if len(ids) > config.WS_MAX_GETBLOCKS_IDS {
		ids = ids[:config.WS_MAX_GETBLOCKS_IDS]
	}

	blocks, err := db.FindBlocksByIDs(ids)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error: could not read blocks for a peer: %v", err)
		return res
	}

	res.IsSuccess = true
	res.Blocks = blocks
	return res
}
