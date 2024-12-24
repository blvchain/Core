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

var Manager = ClientManager{
	clients: make(map[string]*websocket.Conn),
}

func NodeServer(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade:", err)
		return
	}
	defer conn.Close()

	// Node connection
	clientUID := r.URL.Query().Get("uid")
	Manager.AddClient(clientUID, conn)
	defer Manager.RemoveClient(clientUID)
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

		// Handle specific message types
		switch msg.ReqType {

		// 1. Send new data to this node to add it
		case config.WS_SEND_NEW_DATA:

			// err = conn.WriteMessage(websocket.TextMessage, messageData)
			// if err != nil {
			// 	log.Println("Error writing message:", err)
			// }

		// 2. Request to get new data
		case config.WS_GET_DATA:

			// Manager.Broadcast(messageData)

		default:
			log.Printf("Request type from node %v is not valid. \n %v", clientUID, msg.ReqType)
		}

	}
}
