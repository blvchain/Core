package ws

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"blvchain/core/utils"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

func wsAddressMaker(address string) string {
	return address + "?uid=" + config.SELF_UID
}

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToServers(dns_seeds []config.Dns_seed_config) {
	for _, dns_seed := range dns_seeds {
		if dns_seed.UID != config.SELF_UID {
			conn, _, err := websocket.DefaultDialer.Dial(wsAddressMaker(dns_seed.Address), nil)

			if err != nil {
				logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
				continue
			}

			cm.mutex.Lock()
			cm.servers[dns_seed.UID] = conn
			cm.mutex.Unlock()
			logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)
		} else {
			continue
		}
	}
}

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToOneServer(dns_seed config.Dns_seed_config) bool {

	if dns_seed.UID != config.SELF_UID {
		conn, _, err := websocket.DefaultDialer.Dial(wsAddressMaker(dns_seed.Address), nil)
		if err != nil {
			logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
		}

		cm.mutex.Lock()
		cm.servers[dns_seed.UID] = conn
		cm.mutex.Unlock()
		logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)

		return true
	} else {
		return false
	}
}

// DisconnectFromServers disconnects from all servers.
func (cm *ClientManager) DisconnectFromServers() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for uid, conn := range cm.servers {
		conn.Close()
		logger.WS_S_LOGGER.Printf("Disconnected from server %v", uid)
		delete(cm.servers, uid)
	}
}

func MonitorAndReconnectToServers(cm *ClientManager) {
	for {
		time.Sleep(config.RECONNECT_SLEEP_TIME * time.Second)

		cm.mutex.Lock()

		for _, dns_seed := range config.DNS_SEED_LIST {

			// Check the connection
			var pingError error = nil
			conn, ok := cm.servers[dns_seed.UID]

			if ok {
				pingError = conn.WriteMessage(websocket.PingMessage, []byte{})
			}

			if (cm.servers[dns_seed.UID] == nil || pingError != nil) && dns_seed.UID != config.SELF_UID {
				// If the server is disconnected
				logger.WS_F_LOGGER.Printf("Attempting to reconnect to server: %v", dns_seed.Address)

				conn, _, err := websocket.DefaultDialer.Dial(wsAddressMaker(dns_seed.Address), nil)
				if err != nil {
					logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
				} else {
					cm.servers[dns_seed.UID] = conn
					logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)
				}
			}
		}

		cm.mutex.Unlock()
	}
}

func FirstTimeSyncData(cm *ClientManager) bool {
	syncComplete := false

	for !syncComplete { // Keep looping until sync is done
		time.Sleep(1 * time.Second)
		cm.mutex.Lock()
		syncComplete = true // Assume sync is done, but update if more data is found

		for uid, conn := range cm.servers {

			// Get last block in db
			founded_block, _ := db.FindManyBlocksLimited(config.NO_FILTER, 0, 1)

			var req WS_Req = WS_Req{
				Method: "sync",
				Block:  founded_block[0],
			}

			messageByte, messageByte_err := json.Marshal(req)
			if messageByte_err != nil {
				logger.INTERNAL_LOGGER.Printf("Error: Failed to marshal message \n %v", req)
			}

			if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
				logger.WS_F_LOGGER.Printf("Error: Error writing message '%v' to node '%v'", err, uid)
				continue
			} else {
				_, responseData, err := conn.ReadMessage()
				if err != nil {
					logger.WS_F_LOGGER.Printf("Error: Error reading response from node '%v': %v", uid, err)
					continue
				} else {
					var thisResponse WS_Sync_Res
					if err := json.Unmarshal(responseData, &thisResponse); err != nil {
						logger.WS_F_LOGGER.Println("Error: Error parsing message:", err)
						continue
					} else {
						if thisResponse.IsSuccess {
							if len(thisResponse.Blocks) > config.MIN_LIMIT_OF_DATA_SYNC {
								syncComplete = false // More data found, so keep syncing

								var newBlocks []interface{}
								for _, dbBlock := range thisResponse.Blocks {
									//* Valid data
									// check block validation
									message := dbBlock.BlockData.SenderUID +
										utils.Int64ToStr(dbBlock.BlockData.SenderRole) +
										dbBlock.BlockData.SenderPubKey +
										dbBlock.BlockData.ReceiverUID +
										utils.Int64ToStr(dbBlock.BlockData.ReceiverRole) +
										dbBlock.BlockData.Data +
										utils.Int64ToStr(dbBlock.BlockData.TimeStamp)

									valid, _ := utils.Verify(dbBlock.BlockData.SenderPubKey, dbBlock.BlockData.SenderUID, message, dbBlock.BlockData.Signature)

									if valid {
										newBlocks = append(newBlocks, db.Block{
											ID: dbBlock.ID,
											BlockMeta: db.BlockMeta{
												PreBlockHash: dbBlock.BlockMeta.PreBlockHash,
												NodeUID:      dbBlock.BlockMeta.NodeUID,
												TimeStamp:    dbBlock.BlockMeta.TimeStamp,
											},
											BlockData: db.BlockData{
												SenderUID:    dbBlock.BlockData.SenderUID,
												SenderRole:   dbBlock.BlockData.SenderRole,
												SenderPubKey: dbBlock.BlockData.SenderPubKey,
												Signature:    dbBlock.BlockData.Signature,
												ReceiverUID:  dbBlock.BlockData.ReceiverUID,
												ReceiverRole: dbBlock.BlockData.ReceiverRole,
												Data:         dbBlock.BlockData.Data,
												TimeStamp:    dbBlock.BlockData.TimeStamp,
											},
										})
									} else {
										logger.WS_F_LOGGER.Printf("WARNING!!! : Block validation error from node '%v'. Block data:\n%v", uid, dbBlock)
									}
								}

								result, err := db.InsertManyBlock(newBlocks)

								if result {
									logger.WS_S_LOGGER.Printf("Success: Added %v blocks made after %v from node '%v'", len(thisResponse.Blocks), founded_block[0].BlockMeta.TimeStamp, uid)
								}

								if err != nil {
									logger.INTERNAL_LOGGER.Printf("Error: Cannot add data of %v block(s) after %v from node '%v'", config.MAX_LIMIT_OF_DATA_SYNC, founded_block[0].BlockMeta.TimeStamp, uid)
								}

							}
						}
					}
				}
			}
		}
		cm.mutex.Unlock()
	}

	return syncComplete
}

func SyncData(cm *ClientManager) {
	for {
		time.Sleep(config.SYNC_DATA_SLEEP_TIME * time.Second)
		cm.mutex.Lock()

		// Send request to servers
		for uid, conn := range cm.servers {

			// Get last block in db
			founded_block, _ := db.FindManyBlocksLimited(config.NO_FILTER, 0, 1)

			var req WS_Req = WS_Req{
				Method: "sync",
				Block:  founded_block[0],
			}

			messageByte, messageByte_err := json.Marshal(req)
			if messageByte_err != nil {
				logger.INTERNAL_LOGGER.Printf("Failed to marshal message \n %v", req)
			}

			if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
				// Error in sending message
				logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
				continue
			} else {
				// Message sent
				_, responseData, err := conn.ReadMessage()
				if err != nil {
					// Error in reading response message
					logger.WS_F_LOGGER.Printf("Error reading response from node '%v': %v", uid, err)
					continue
				} else {
					// Successfully get the response
					var thisResponse WS_Sync_Res
					if err := json.Unmarshal(responseData, &thisResponse); err != nil {
						// Error in unmarshaling byte data
						logger.WS_F_LOGGER.Println("Error parsing message:", err)
						continue
					} else {
						if thisResponse.IsSuccess {
							if len(thisResponse.Blocks) > config.MIN_LIMIT_OF_DATA_SYNC {

								var newBlocks []interface{}

								for _, dbBlock := range thisResponse.Blocks {

									//* Valid data
									// check block validation
									message := dbBlock.BlockData.SenderUID +
										utils.Int64ToStr(dbBlock.BlockData.SenderRole) +
										dbBlock.BlockData.SenderPubKey +
										dbBlock.BlockData.ReceiverUID +
										utils.Int64ToStr(dbBlock.BlockData.ReceiverRole) +
										dbBlock.BlockData.Data +
										utils.Int64ToStr(dbBlock.BlockData.TimeStamp)

									valid, _ := utils.Verify(dbBlock.BlockData.SenderPubKey, dbBlock.BlockData.SenderUID, message, dbBlock.BlockData.Signature)

									if valid {
										newBlocks = append(newBlocks, db.Block{
											ID: dbBlock.ID,
											BlockMeta: db.BlockMeta{
												PreBlockHash: dbBlock.BlockMeta.PreBlockHash,
												NodeUID:      dbBlock.BlockMeta.NodeUID,
												TimeStamp:    dbBlock.BlockMeta.TimeStamp,
											},
											BlockData: db.BlockData{
												SenderUID:    dbBlock.BlockData.SenderUID,
												SenderRole:   dbBlock.BlockData.SenderRole,
												SenderPubKey: dbBlock.BlockData.SenderPubKey,
												Signature:    dbBlock.BlockData.Signature,
												ReceiverUID:  dbBlock.BlockData.ReceiverUID,
												ReceiverRole: dbBlock.BlockData.ReceiverRole,
												Data:         dbBlock.BlockData.Data,
												TimeStamp:    dbBlock.BlockData.TimeStamp,
											},
										})
									} else {
										logger.WS_F_LOGGER.Printf("WARNING!!! : Block validation error from node '%v'. Block data:\n%v", uid, dbBlock)
									}
								}

								result, err := db.InsertManyBlock(newBlocks)

								if err != nil {
									logger.INTERNAL_LOGGER.Printf("Error: CanNot add data of %v block made after %v from node '%v'", config.MAX_LIMIT_OF_DATA_SYNC, founded_block[0].BlockMeta.TimeStamp, uid)
								}

								if result {
									logger.WS_S_LOGGER.Printf("Success: Added data of %v block made after %v to node '%v'", len(thisResponse.Blocks), founded_block[0].BlockMeta.TimeStamp, uid)
									continue
								}

							} else {
								continue
							}
						} else {
							continue
						}
					}
				}
			}
		}

		cm.mutex.Unlock()
	}
}

// SendMessage sends a message to the server identified by its UID.
func (cm *ClientManager) SendMessageToOneServer(uid string, message any) bool {
	cm.mutex.RLock()
	conn, ok := cm.servers[uid]
	cm.mutex.RUnlock()

	if !ok {
		logger.WS_F_LOGGER.Printf("No connection found for server UID: %s", uid)
		return false
	}

	messageByte, messageByte_err := json.Marshal(message)
	if messageByte_err != nil {
		logger.INTERNAL_LOGGER.Printf("Failed to marshal message \n %v", message)
	}

	if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
		logger.WS_F_LOGGER.Printf("Failed to send message to server %s: %v", uid, err)
		return false
	}

	logger.WS_S_LOGGER.Printf("Message sent to server %s: %s\n", uid, message)
	return true

}

// SendMessage sends a message to the server identified by its UID.
func (cm *ClientManager) GetBlockFromServers(blockHash string) []db.Block {

	var req WS_Req = WS_Req{
		Method: "get",
		Block: db.Block{
			ID: blockHash,
		},
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	messageByte, messageByte_err := json.Marshal(req)
	if messageByte_err != nil {
		logger.INTERNAL_LOGGER.Printf("Failed to marshal message \n %v", req)
	}

	var blocks []db.Block

	// Send request to servers
	for uid, conn := range cm.servers {
		if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
			// Error in sending message
			logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
			continue
		} else {
			// Message sent
			_, responseData, err := conn.ReadMessage()
			if err != nil {
				// Error in reading response message
				logger.WS_F_LOGGER.Printf("Error reading response from node '%v': %v", uid, err)
				continue
			} else {
				// Successfully get the response
				var thisResponse WS_Res
				if err := json.Unmarshal(responseData, &thisResponse); err != nil {
					// Error in unmarshaling byte data
					logger.WS_F_LOGGER.Println("Error parsing message:", err)
					continue
				} else {
					if thisResponse.IsSuccess {
						// Adding block to Blocks if exists
						blocks = append(blocks, thisResponse.Block)
					} else {
						continue
					}
				}
			}
		}
	}

	return blocks
}

// SendMessage sends a message to the server identified by its UID.
func (cm *ClientManager) AddBlockToServers(block db.Block) []WS_Res {

	var req WS_Req = WS_Req{
		Method: "add",
		Block:  block,
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	messageByte, messageByte_err := json.Marshal(req)
	if messageByte_err != nil {
		logger.INTERNAL_LOGGER.Printf("Failed to marshal message \n %v", req)
	}

	var responses []WS_Res

	// Send request to servers
	for uid, conn := range cm.servers {
		if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
			// Error in sending message
			logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
			continue
		} else {
			// Message sent
			_, responseData, err := conn.ReadMessage()
			if err != nil {
				// Error in reading response message
				logger.WS_F_LOGGER.Printf("Error reading response from node '%v': %v", uid, err)
				continue
			} else {
				// Successfully get the response
				var thisResponse WS_Res
				if err := json.Unmarshal(responseData, &thisResponse); err != nil {
					// Error in unmarshaling byte data
					logger.WS_F_LOGGER.Println("Error parsing message:", err)
					continue
				} else {
					responses = append(responses, thisResponse)
				}
			}
		}
	}

	return responses
}
