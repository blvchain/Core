package websocket

import (
	"github.com/gorilla/websocket"
)

// AddClient adds a client to the map
func (cm *ClientManager) AddClient(id string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clients[id] = conn
}

// RemoveClient removes a client from the map
func (cm *ClientManager) RemoveClient(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.clients, id)
}
