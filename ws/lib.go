package ws

import (
	"blvchain/core/logger"
	"encoding/json"

	"github.com/gorilla/websocket"
)

func (cm *ServerManager) BroadcastMessageFromLocalServer(message any) {
	messageByte, messageByte_err := json.Marshal(message)
	if messageByte_err != nil {
		logger.INTERNAL_LOGGER.Printf("Failed to marshal message \n %v", message)
	}

	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for uid, conn := range cm.clients {
		if err := conn.WriteMessage(websocket.TextMessage, messageByte); err != nil {
			logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
		}
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
