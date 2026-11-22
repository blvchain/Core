package ws

import (
	"net/http"
	"sync"

	"blvchain/core/utils"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Only nodes listed in dns_seed.json may open an inbound connection.
	CheckOrigin: func(r *http.Request) bool {
		return utils.NodeUidChecker(r.Header.Get("X-UID"))
	},
	// The node is not a browser client; the default 4 KiB buffers are enough.
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// ServerManager tracks the peers that connected *to* this node.
type ServerManager struct {
	clients map[string]*safeConn
	mutex   sync.RWMutex
}

// ClientManager tracks the outbound connections to other nodes.
type ClientManager struct {
	servers map[string]*safeConn
	mutex   sync.RWMutex
}

var ServerManagerVar = ServerManager{
	clients: make(map[string]*safeConn),
}

var ClientManagerVar = ClientManager{
	servers: make(map[string]*safeConn),
}
