package ws

import "blvchain/core/db"

// WebSocket message methods.
const (
	MethodGet  = "get"
	MethodAdd  = "add"
	MethodSync = "sync"
)

// WS_Req is a request sent by a peer node.
type WS_Req struct {
	Method string   `json:"method"`
	Block  db.Block `json:"block"`
}

// WS_Res is the reply to "get" and "add".
type WS_Res struct {
	IsSuccess bool     `json:"isSuccess"`
	Block     db.Block `json:"block"`
}

// WS_Sync_Res is the reply to "sync".
type WS_Sync_Res struct {
	IsSuccess bool       `json:"isSuccess"`
	Blocks    []db.Block `json:"blocks"`
}
