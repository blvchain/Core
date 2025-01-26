package ws

import (
	"blvchain/core/config"
	"blvchain/core/db"
	"blvchain/core/logger"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToServers(dns_seeds []config.Dns_seed_config) {
	for _, dns_seed := range dns_seeds {
		conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
		if err != nil {
			logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
			continue
		}

		cm.mutex.Lock()
		cm.servers[dns_seed.UID] = conn
		cm.mutex.Unlock()
		logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)
	}
}

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToOneServer(dns_seed config.Dns_seed_config) bool {

	conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
	}

	cm.mutex.Lock()
	cm.servers[dns_seed.UID] = conn
	cm.mutex.Unlock()
	logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)

	return true
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
		time.Sleep(config.RECONNECT_SLEEP_TIME * time.Second) // Wait for 5 seconds

		cm.mutex.Lock()

		for _, dns_seed := range config.DNS_SEED_LIST {
			if cm.servers[dns_seed.UID] == nil { // If the server is disconnected
				logger.WS_F_LOGGER.Printf("Attempting to reconnect to server: %v", dns_seed.Address)

				conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
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
			BlockHash: blockHash,
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
