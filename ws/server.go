package ws

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (cm *ServerManager) addClientToLocalServer(uid string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.clients[uid] = conn
}

func (cm *ServerManager) removeClientFromLocalServer(uid string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if conn, ok := cm.clients[uid]; ok {
		conn.Close()
		delete(cm.clients, uid)
	}
}

func (cm *ServerManager) BroadcastMessageFromLocalServer(message any) {
	messageByte, messageByte_err := json.Marshal(message)
	if messageByte_err != nil {
		logger.INTERNAL_LOGGER.Printf("Error: Failed to marshal message \n %v", message)
	}

	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for uid, conn := range cm.clients {
		if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
			logger.WS_F_LOGGER.Printf("Error: Error writing message '%v' to node '%v'", err, uid)
		}
	}
}

func messanger(message any, conn *websocket.Conn, uid string) {
	messageByte, _ := json.Marshal(message)
	err := conn.WriteMessage(websocket.TextMessage, messageByte)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error: Error writing message '%v' to node '%v'", err, uid)
	}
}

func WS_Server_Handler(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Error: Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	ServerManagerVar.addClientToLocalServer(clientUID, conn)
	defer ServerManagerVar.removeClientFromLocalServer(clientUID)
	logger.WS_S_LOGGER.Printf("Success: Node '%s' connected\n", clientUID)

	// Handle incoming messages
	for {

		// Handle disconnected clients
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Error: Node '%s' disconnected, %v\n", clientUID, err)
			break
		}

		var msg WS_Req
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logger.WS_F_LOGGER.Println("Error: Error parsing message:", err)
			break
		}

		//* Get block data
		if msg.Method == "get" {

			// Make response var
			var res WS_Res = WS_Res{
				IsSuccess: false,
			}

			filter := []bson.M{}

			if msg.Block.BlockData.SenderUID != "" {
				filter = append(filter, bson.M{"blockData.senderUid": msg.Block.BlockData.SenderUID})
			}
			if msg.Block.BlockData.SenderRole != 0 {
				filter = append(filter, bson.M{"blockData.senderRole": msg.Block.BlockData.SenderRole})
			}
			if msg.Block.BlockData.SenderPubKey != "" {
				filter = append(filter, bson.M{"blockData.senderPubKey": msg.Block.BlockData.SenderPubKey})
			}
			if msg.Block.BlockData.ReceiverUID != "" {
				filter = append(filter, bson.M{"blockData.receiverUid": msg.Block.BlockData.ReceiverUID})
			}
			if msg.Block.BlockData.ReceiverRole != 0 {
				filter = append(filter, bson.M{"blockData.receiverRole": msg.Block.BlockData.ReceiverRole})
			}
			if msg.Block.ID != "" {
				filter = append(filter, bson.M{"_id": msg.Block.ID})
			}
			if msg.Block.BlockMeta.PreBlockHash != "" {
				filter = append(filter, bson.M{"blockMeta.preBlockHash": msg.Block.BlockMeta.PreBlockHash})
			}
			if msg.Block.BlockMeta.NodeUID != "" {
				filter = append(filter, bson.M{"blockMeta.nodeUid": msg.Block.BlockMeta.NodeUID})
			}

			founded_block, founded_block_err := db.FindManyBlocksLimited(bson.M{"$and": filter}, 0, 1)

			if founded_block_err == mongo.ErrNoDocuments || len(founded_block) == 0 {
				// Not found any block with this hash
				logger.WS_F_LOGGER.Printf("Error: Not found block '%v'. Req from node '%v'", msg.Block.ID, clientUID)
			} else {
				// Send founded block
				logger.WS_S_LOGGER.Printf("Success: Send data of block '%v' to node '%v'", msg.Block.ID, clientUID)
				res.IsSuccess = true
				res.Block = founded_block[0]
			}

			// Send response to client
			messanger(res, conn, clientUID)
		}

		//* Add new block
		if msg.Method == "add" {

			// Make response var
			var res WS_Res = WS_Res{
				IsSuccess: false,
			}

			// check all data validation
			structValidation_err := db.StructValidator(msg.Block)

			if structValidation_err != nil {
				// Structure failed
				logger.WS_F_LOGGER.Printf("Error: Error in node '%v' message structure validation: , %v\n", clientUID, structValidation_err)
			} else {

				// check block validation
				validation_err := db.BlockValidator(msg.Block)

				if validation_err != nil {
					// Block validation failed
					logger.WS_F_LOGGER.Printf("WARNING!!!: Error in node '%v' block validation: , %v\n", clientUID, validation_err)
				} else {

					// check hash
					founded_blocks, _ := db.FindAllBlocks(bson.M{"_id": msg.Block.ID})

					if len(founded_blocks) == 0 {
						// Block hash is unique
						// preBlockHash found in db
						Block_insert_result, Block_insert_result_err := db.InsertOneBlock(msg.Block)

						if !Block_insert_result {
							// Internal error to add data to DB
							logger.INTERNAL_LOGGER.Printf("Error: Error in adding block '%v' to db from node '%v' : \n %v", msg.Block.ID, clientUID, Block_insert_result_err)
						} else {
							// Successfully added data to DB
							logger.WS_S_LOGGER.Printf("Success: Block '%v' successfully added from '%v' ", msg.Block.ID, clientUID)
							res.IsSuccess = true
						}

					} else {
						// Block hash is NOT unique
						logger.WS_F_LOGGER.Printf("Error: Error in node '%v' block hash is not unique: , %v\n", clientUID, validation_err)
					}
				}
			}

			// Send response to client
			messanger(res, conn, clientUID)
		}

		//* Add new block
		if msg.Method == "sync" {

			// Make response var
			var res WS_Sync_Res = WS_Sync_Res{
				IsSuccess: false,
			}

			filter := bson.M{"blockMeta.timeStamp": bson.M{"$gt": msg.Block.BlockMeta.TimeStamp}}

			founded_block, founded_block_err := db.FindManyBlocksLimitedASE(filter, 0, config.MAX_LIMIT_OF_DATA_SYNC)

			if (founded_block_err == nil || len(founded_block) != 0) && len(founded_block) > config.MIN_LIMIT_OF_DATA_SYNC {
				// Send founded block
				logger.WS_S_LOGGER.Printf("Success: Send data of %v block(s) made after %v to node '%v'", len(founded_block), msg.Block.BlockMeta.TimeStamp, clientUID)
				res.IsSuccess = true
				res.Blocks = founded_block
			} else {
				if founded_block_err != mongo.ErrNoDocuments && founded_block_err != nil {
					logger.WS_F_LOGGER.Printf("Error: Error in finding blocks made after %v for node %v. \n %v", msg.Block.BlockMeta.TimeStamp, clientUID, founded_block_err)
				}
			}
			// Send response to client
			messanger(res, conn, clientUID)
		}

	}
}
