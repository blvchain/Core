package ws

import (
	"blvchain/core/db"
	"sync"

	"github.com/gorilla/websocket"
)

type ServerManager struct {
	clients map[string]*websocket.Conn // Map of client UIDs to WebSocket connections
	mutex   sync.RWMutex
}

type ClientManager struct {
	servers map[string]*websocket.Conn // Map of server UIDs to WebSocket connections
	mutex   sync.RWMutex
}

type WS_Req struct {
	Method string   `json:"method"`
	Block  db.Block `json:"block"`
}

type WS_Res struct {
	IsSuccess bool     `json:"isSuccess"`
	Block     db.Block `json:"block"`
}

type WS_Sync_Res struct {
	IsSuccess bool       `json:"isSuccess"`
	Blocks    []db.Block `json:"blocks"`
}
