package client

import (
	"blvchain/core/logger"
	"blvchain/core/utils"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
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

var ServerManagerVar = ServerManager{
	clients: make(map[string]*websocket.Conn),
}

var ClientManagerVar = ClientManager{
	servers: make(map[string]*websocket.Conn),
}

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
