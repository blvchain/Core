package ws

import (
	"blvchain/core/logger"
	"encoding/json"

	"github.com/gorilla/websocket"
)

func Messenger(message any, conn *websocket.Conn, uid string) {
	messageByte, _ := json.Marshal(message)
	err := conn.WriteMessage(websocket.TextMessage, messageByte)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
	}
}

func (cm *ServerManager) AddClient(uid string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.clients[uid] = conn
}

func (cm *ServerManager) RemoveClient(uid string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if conn, ok := cm.clients[uid]; ok {
		conn.Close()
		delete(cm.clients, uid)
	}
}

func (cm *ServerManager) BroadcastMessage(message any) {
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
