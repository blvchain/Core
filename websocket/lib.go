package websocket

import (
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
