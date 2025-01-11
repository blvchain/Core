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

func messanger(message any, conn *websocket.Conn, uid string) {
	messageByte, _ := json.Marshal(message)
	err := conn.WriteMessage(websocket.TextMessage, messageByte)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
	}
}

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

// End point functions
func AddNewBlock(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	ServerManagerVar.addClientToLocalServer(clientUID, conn)
	defer ServerManagerVar.removeClientFromLocalServer(clientUID)
	logger.WS_S_LOGGER.Printf("Node '%s' connected\n", clientUID)

	// Handle incoming messages
	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Node '%s' disconnected, %v\n", clientUID, err)
			break
		}

		var msg AddNewBlock_Req
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logger.WS_F_LOGGER.Println("Error parsing message:", err)
			break
		}

		// check all data validation
		structValidation_err := db.StructValidator(msg.Block)

		if structValidation_err != nil {

			// Structure failed
			logger.WS_F_LOGGER.Printf("Error in node '%v' message structure validation: , %v\n", clientUID, structValidation_err)
			var err AddNewBlock_Res = AddNewBlock_Res{
				Status: config.FAIL,
				Detail: structValidation_err.Error(),
			}
			messanger(err, conn, clientUID)

		} else {

			// check block validation
			validation_err := db.BlockValidator(msg.Block)

			if validation_err != nil {

				// Block validation failed
				logger.WS_F_LOGGER.Printf("Error in node '%v' block validation: , %v\n", clientUID, validation_err)
				var err AddNewBlock_Res = AddNewBlock_Res{
					Status: config.FAIL,
					Detail: validation_err.Error(),
				}
				messanger(err, conn, clientUID)

			} else {

				// check hash
				founded_blocks, _ := db.FindAllBlocks(bson.M{"blockHash": msg.Block.BlockHash})

				if len(founded_blocks) == 0 {

					// Block hash is unique
					Block_insert_result, Block_insert_result_err := db.InsertOneBlock(msg.Block)

					if !Block_insert_result {

						// Internal error to add data to DB
						logger.INTERNAL_LOGGER.Printf("Error in adding block '%v' to db from node '%v' : \n %v", msg.Block.BlockHash, clientUID, Block_insert_result_err)
						var err AddNewBlock_Res = AddNewBlock_Res{
							Status: config.FAIL,
							Detail: Block_insert_result_err.Error(),
						}
						messanger(err, conn, clientUID)

					} else {

						// Successfully added data to DB
						logger.WS_F_LOGGER.Printf("Block '%v' successfully added from '%v' ", msg.Block.BlockHash, clientUID)
						var message AddNewBlock_Res = AddNewBlock_Res{
							Status: config.SUCCESS,
							Detail: "Block '" + msg.Block.BlockHash + "' added to db.",
						}
						messanger(message, conn, clientUID)

					}

				} else {

					// Block hash is NOT unique
					logger.WS_F_LOGGER.Printf("Error in node '%v' block hash is not unique: , %v\n", clientUID, validation_err)
					var err AddNewBlock_Res = AddNewBlock_Res{
						Status: config.FAIL,
						Detail: "Block hash is not unique",
					}
					messanger(err, conn, clientUID)

				}

			}
		}

	}
}

func GetBlock(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	ServerManagerVar.addClientToLocalServer(clientUID, conn)
	defer ServerManagerVar.removeClientFromLocalServer(clientUID)
	logger.WS_S_LOGGER.Printf("Node '%s' connected\n", clientUID)

	// Handle incoming messages
	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Node '%s' disconnected, %v\n", clientUID, err)
			break
		}

		var msg GetBlock_Req
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logger.WS_F_LOGGER.Println("Error parsing message:", err)
			break
		}

		var block db.Block
		block_filter := bson.M{"blockHash": msg.BlockHash}
		founded_block_err := db.FindOneBlock(block_filter, &block)

		if founded_block_err == mongo.ErrNoDocuments {
			// Not found any block with this hash
			logger.WS_F_LOGGER.Printf("Not found block '%v'. Req from node '%v'", msg.BlockHash, clientUID)
			var err GetBlock_Res = GetBlock_Res{
				Status: config.FAIL,
				Detail: founded_block_err.Error(),
			}
			messanger(err, conn, clientUID)
		} else {
			// Send founded block
			logger.WS_S_LOGGER.Printf("Send data of block '%v' to node '%v'", msg.BlockHash, clientUID)
			var err GetBlock_Res = GetBlock_Res{
				Status: config.SUCCESS,
				Detail: "Block founded",
				Block:  block,
			}
			messanger(err, conn, clientUID)
		}

	}
}
