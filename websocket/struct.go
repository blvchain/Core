package websocket

import (
	"blvchain/core/db"
	"sync"

	"github.com/gorilla/websocket"
)

type ClientManager struct {
	clients map[string]*websocket.Conn // Map of client UIDs to WebSocket connections
}

type WebSocketClient struct {
	conn      *websocket.Conn
	mutex     sync.Mutex // To ensure thread-safe writes
	serverURL string
}

// * Request
type AddNewBlock_Req struct {
	Block db.Block `json:"block"`
}

type GetBlock_Req struct {
	BlockHash string `json:"blockHash"`
}

// * Response
type AddNewBlock_Res struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type GetBlock_Res struct {
	Status string   `json:"status"`
	Detail string   `json:"detail"`
	Block  db.Block `json:"block"`
}
