package websocket

import (
	"github.com/gorilla/websocket"
)

type ClientManager struct {
	clients map[string]*websocket.Conn // Map of client UIDs to WebSocket connections
}

type WSMessage struct {
	ReqType int         `json:"reqType"`
	Data    interface{} `json:"data"`
}
