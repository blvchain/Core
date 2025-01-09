package ws

import (
	"blvchain/core/utils"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {

		// Check UID for node connection
		clientUID := r.URL.Query().Get("uid")
		if clientUID != "" {
			if utils.NodeUidChecker(clientUID) {
				return true
			} else {
				return false
			}
		} else {
			return false
		}

	},
}

var ServerManagerVar = ServerManager{
	clients: make(map[string]*websocket.Conn),
}

var ClientManagerVar = ClientManager{
	servers: make(map[string]*websocket.Conn),
}
