package websocket

import (
	"log"

	"github.com/gorilla/websocket"
)

func (cm *ClientManager) AddClient(uid string, conn *websocket.Conn) {
	cm.clients[uid] = conn
}

func (cm *ClientManager) RemoveClient(uid string) {
	if conn, ok := cm.clients[uid]; ok {
		conn.Close()
		delete(cm.clients, uid)
	}
}

func (cm *ClientManager) Broadcast(message []byte) {
	for uid, conn := range cm.clients {
		err := conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Failed to send message to client '%s': %v\n", uid, err)
			conn.Close()
			delete(cm.clients, uid) // Remove client if sending fails
		}
	}
}
