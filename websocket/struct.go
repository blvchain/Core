package websocket

import (
	"blvchain/core/db"
	"sync"

	"github.com/gorilla/websocket"
)

type ClientManager struct {
	clients map[string]*websocket.Conn // Map of client UIDs to WebSocket connections
}

type WSMessage struct {
	ReqType int      `json:"reqType"`
	Block   db.Block `json:"block"`
}

type WebSocketClient struct {
	conn      *websocket.Conn
	mutex     sync.Mutex // To ensure thread-safe writes
	serverURL string
}

type WSResponse struct {
	Status string
	Data   string
}
