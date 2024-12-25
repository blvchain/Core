package websocket

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func NewWebSocketClient(serverURL string) *WebSocketClient {
	return &WebSocketClient{serverURL: serverURL}
}

// Connect establishes a stable connection to the WebSocket server
func (client *WebSocketClient) Connect() error {
	var err error
	client.conn, _, err = websocket.DefaultDialer.Dial(client.serverURL, nil)
	if err != nil {
		return err
	}
	log.Println("Connected to WebSocket server:", client.serverURL)

	// Start listening for messages in a separate goroutine
	go client.listen()
	return nil
}

// SendMessage sends a message to the WebSocket server
func (client *WebSocketClient) SendMessage(message string) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if client.conn == nil {
		return websocket.ErrCloseSent
	}

	err := client.conn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("Error sending message:", err)
		return err
	}
	log.Println("Message sent:", message)
	return nil
}

// listen continuously listens for incoming messages from the WebSocket server
func (client *WebSocketClient) listen() {
	for {
		if client.conn == nil {
			break
		}

		_, message, err := client.conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			client.reconnect()
			return
		}

		log.Println("Received message:", string(message))
	}
}

// reconnect attempts to re-establish the WebSocket connection
func (client *WebSocketClient) reconnect() {
	log.Println("Attempting to reconnect...")
	time.Sleep(5 * time.Second) // Wait before reconnecting
	err := client.Connect()
	if err != nil {
		log.Println("Reconnect failed:", err)
		client.reconnect()
	}
}

// Close gracefully closes the WebSocket connection
func (client *WebSocketClient) Close() {
	if client.conn != nil {
		client.mutex.Lock()
		client.conn.Close()
		client.mutex.Unlock()
		log.Println("WebSocket connection closed")
	}
}
