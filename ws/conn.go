package ws

import (
	"encoding/json"
	"sync"
	"time"

	"blvchain/core/config"

	"github.com/gorilla/websocket"
)

// safeConn serialises access to a WebSocket connection.
//
// gorilla/websocket allows only one concurrent reader and one concurrent writer.
// The node writes to a peer from several goroutines (gRPC handlers, the sync
// loop, the reconnect monitor), so every operation goes through this wrapper.
type safeConn struct {
	conn *websocket.Conn

	writeMu sync.Mutex
	readMu  sync.Mutex
}

func newSafeConn(conn *websocket.Conn) *safeConn {
	conn.SetReadLimit(config.WS_READ_LIMIT_BYTES)
	return &safeConn{conn: conn}
}

func (c *safeConn) Close() error { return c.conn.Close() }

// writeJSON sends a single JSON message.
func (c *safeConn) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(config.WS_WRITE_TIMEOUT)); err != nil {
		return err
	}
	return c.conn.WriteJSON(v)
}

// ping sends a WebSocket ping frame.
func (c *safeConn) ping() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(config.WS_WRITE_TIMEOUT)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

// request sends payload and waits for one JSON response.
//
// The read lock is held for the whole exchange so two goroutines cannot
// interleave requests on the same connection and read each other's answers.
func (c *safeConn) request(payload any, out any) error {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if err := c.writeJSON(payload); err != nil {
		return err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(config.WS_READ_TIMEOUT)); err != nil {
		return err
	}
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// read blocks for the next message from the peer.
func (c *safeConn) read() (int, []byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if err := c.conn.SetReadDeadline(time.Now().Add(config.WS_READ_TIMEOUT)); err != nil {
		return 0, nil, err
	}
	return c.conn.ReadMessage()
}

// readJSON reads one message and decodes it into out, applying timeout as the
// read deadline. The handshake uses a much shorter timeout than a normal
// request so an unauthenticated peer cannot pin a goroutine.
func (c *safeConn) readJSON(out any, timeout time.Duration) error {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
