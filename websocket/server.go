package websocket

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {

		// Check UID for node connection
		clientUID := r.URL.Query().Get("uid")
		if clientUID != "" {
			if utils.NodeUidChecker(clientUID) {
				return true
			} else {
				return false
			}
		} else {
			return false
		}

	},
}

var Manager = ClientManager{
	clients: make(map[string]*websocket.Conn),
}

func NodeServer(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WS_F_LOGGER.Println("Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	Manager.AddClient(clientUID, conn)
	defer Manager.RemoveClient(clientUID)
	logger.WS_S_LOGGER.Printf("Node '%s' connected\n", clientUID)

	// Handle incoming messages
	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			logger.WS_F_LOGGER.Printf("Node '%s' disconnected, %v\n", clientUID, err)
			break
		}
		logger.WS_S_LOGGER.Printf("Received: %s\n", messageData)

		var msg WSMessage
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logger.WS_F_LOGGER.Println("Error parsing message:", err)
			break
		}

		// Handle specific message types
		switch msg.ReqType {

		// 1. Request to add new data to db
		case config.WS_NEW_DATA:

			// check all data validation
			structValidation_err := db.StructValidator(msg.Block)

			if structValidation_err != nil {
				logger.WS_F_LOGGER.Printf("Error in node '%v' message structure validation: , %v\n", clientUID, structValidation_err)
				var err WSResponse = WSResponse{
					Status: "fail",
					Data:   structValidation_err.Error(),
				}
				Messenger(err, conn, clientUID)
			} else {

				// check block validation
				validation_err := db.BlockValidator(msg.Block)

				if validation_err != nil {
					logger.WS_F_LOGGER.Printf("Error in node '%v' block validation: , %v\n", clientUID, validation_err)
					var err WSResponse = WSResponse{
						Status: "fail",
						Data:   validation_err.Error(),
					}
					Messenger(err, conn, clientUID)
				} else {

					// check hash
					founded_blocks, _ := db.FindAllBlocks(bson.M{"blockHash": msg.Block.BlockHash})

					if len(founded_blocks) == 0 {
						Block_insert_result, Block_insert_result_err := db.InsertOne(config.DATA_COLL, msg.Block, "hash")
						if !Block_insert_result {
							logger.INTERNAL_LOGGER.Printf("Error in adding block '%v' to db from node '%v' : \n %v", msg.Block.BlockHash, clientUID, Block_insert_result_err)
							var err WSResponse = WSResponse{
								Status: "fail",
								Data:   Block_insert_result_err.Error(),
							}
							Messenger(err, conn, clientUID)
						} else {
							logger.WS_F_LOGGER.Printf("Block '%v' successfully added from '%v' ", msg.Block.BlockHash, clientUID)
							var message WSResponse = WSResponse{
								Status: "success",
								Data:   "Block '" + msg.Block.BlockHash + "' added to db.",
							}
							Messenger(message, conn, clientUID)
						}

					} else {
						logger.WS_F_LOGGER.Printf("Error in node '%v' block hash is not unique: , %v\n", clientUID, validation_err)
						var err WSResponse = WSResponse{
							Status: "fail",
							Data:   "Block hash is not unique",
						}
						Messenger(err, conn, clientUID)
					}

				}
			}

		// 2. Request to sync data and get missing data
		case config.WS_SYNC_DATA:

			// err = conn.WriteMessage(websocket.TextMessage, messageData)
			// if err != nil {
			// 	logger.WS_F_LOGGER.Println("Error writing message:", err)
			// }

		// 3. Request to get all data
		case config.WS_GET_ALL_DATA:

			// err = conn.WriteMessage(websocket.TextMessage, messageData)
			// if err != nil {
			// 	logger.WS_F_LOGGER.Println("Error writing message:", err)
			// }

		default:
			logger.WS_F_LOGGER.Printf("Request type from node '%v' is not valid. \n Message: %v", clientUID, msg.ReqType)
		}

	}
}

func Messenger(message WSResponse, conn *websocket.Conn, uid string) {
	messageByte, _ := json.Marshal(message)
	err := conn.WriteMessage(websocket.TextMessage, messageByte)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
	}
}
