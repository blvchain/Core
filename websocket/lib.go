package websocket

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

var Manager = ClientManager{
	clients: make(map[string]*websocket.Conn),
}

func Messenger(message any, conn *websocket.Conn, uid string) {
	messageByte, _ := json.Marshal(message)
	err := conn.WriteMessage(websocket.TextMessage, messageByte)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Error writing message '%v' to node '%v'", err, uid)
	}
}

func (cm *ClientManager) AddClient(uid string, conn *websocket.Conn) {
	cm.clients[uid] = conn
}

func (cm *ClientManager) RemoveClient(uid string) {
	if conn, ok := cm.clients[uid]; ok {
		conn.Close()
		delete(cm.clients, uid)
	}
}
