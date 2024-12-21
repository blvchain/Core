package websocket

import (
	"sync"

	"github.com/gorilla/websocket"
)

type ClientManager struct {
	clients map[string]*websocket.Conn // Map of client ID to WebSocket connection
	mu      sync.Mutex                 // Mutex for safe concurrent access
}

type WSMessage struct {
	ReqType int         `json:"reqType"`
	Data    interface{} `json:"data"`
}
