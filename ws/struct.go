package ws

import (
	"blvchain/core/db"
	"sync"

	"github.com/gorilla/websocket"
)

type ServerManager struct {
	clients map[string]*websocket.Conn // Map of client UIDs to WebSocket connections
	mutex   sync.RWMutex               // To handle concurrent access
}

type ClientManager struct {
	servers map[string]*websocket.Conn // Map of server UIDs to WebSocket connections
	mutex   sync.RWMutex               // To handle concurrent access
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
