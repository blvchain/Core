package websocket

import (
	"blvchain/core/config"
	"blvchain/core/utils"
	"encoding/json"
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
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Node '%s' disconnected, %v\n", clientUID, err)
			break
		}
		log.Printf("Received: %s\n", messageData)

		var msg WSMessage
		if err := json.Unmarshal(messageData, &msg); err != nil {
			log.Println("Error parsing message:", err)
			break
		}

		// Process messages
		if msg.ReqType == config.WS_SEND_NEW_DATA {
			err = conn.WriteMessage(websocket.TextMessage, messageData)
			if err != nil {
				log.Println("Error writing message:", err)
				break
			}
		}

		if msg.ReqType == config.WS_GET_DATA {

		}

	}

}
