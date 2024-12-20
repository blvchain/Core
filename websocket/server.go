package websocket

import (
	"blvchain/core/utils"
	"log"
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

var manager = ClientManager{
	clients: make(map[string]*websocket.Conn),
}

func Server(w http.ResponseWriter, r *http.Request) {

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	manager.AddClient(clientUID, conn)
	defer manager.RemoveClient(clientUID)
	log.Printf("Node '%s' connected\n", clientUID)

	// Handle incoming messages
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Node '%s' disconnected, %v\n", clientUID, err)
			break
		}
		log.Printf("Received: %s\n", message)

		// Echo the message back to the client
		err = conn.WriteMessage(messageType, message)
		if err != nil {
			log.Println("Error writing message:", err)
			break
		}
	}

}
