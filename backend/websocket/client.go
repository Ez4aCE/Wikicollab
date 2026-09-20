package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	gorillaws "github.com/gorilla/websocket"

	"github.com/wikicollab/backend/models"
)

const (
	// Maximum time to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum time to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from client.
	maxMessageSize = 64 * 1024 // 64 KB
)

// Client represents one active WebSocket connection.
type Client struct {
	conn   *gorillaws.Conn
	User   *models.User
	PageID string

	// outbound is a buffered channel of outbound messages.
	outbound chan OutMessage

	// ownedLocks maps blockID → lock token for blocks this client currently holds.
	// Protected by locksMu.
	ownedLocks map[string]string
	locksMu    sync.Mutex
}

// newClient creates a Client for the given authenticated connection.
func newClient(conn *gorillaws.Conn, user *models.User, pageID string) *Client {
	return &Client{
		conn:       conn,
		User:       user,
		PageID:     pageID,
		outbound:   make(chan OutMessage, 32),
		ownedLocks: make(map[string]string),
	}
}

// send attempts to queue a message for the client without blocking.
// If the send buffer is full the client is considered too slow and the message is dropped.
func (c *Client) send(msg OutMessage) {
	select {
	case c.outbound <- msg:
	default:
		log.Printf("ws: send buffer full for user %s, dropping message", c.User.ID)
	}
}

// addLock records that this client now holds the lock on blockID with the given token.
func (c *Client) addLock(blockID, token string) {
	c.locksMu.Lock()
	defer c.locksMu.Unlock()
	c.ownedLocks[blockID] = token
}

// removeLock removes the record of this client holding blockID.
func (c *Client) removeLock(blockID string) {
	c.locksMu.Lock()
	defer c.locksMu.Unlock()
	delete(c.ownedLocks, blockID)
}

// lockToken returns the token for blockID if owned by this client, or "" if not.
func (c *Client) lockToken(blockID string) string {
	c.locksMu.Lock()
	defer c.locksMu.Unlock()
	return c.ownedLocks[blockID]
}

// drainLocks returns a snapshot of all (blockID, token) pairs held by this client
// and clears the internal map. Used on disconnect for cleanup.
func (c *Client) drainLocks() map[string]string {
	c.locksMu.Lock()
	defer c.locksMu.Unlock()
	snapshot := make(map[string]string, len(c.ownedLocks))
	for k, v := range c.ownedLocks {
		snapshot[k] = v
	}
	c.ownedLocks = make(map[string]string)
	return snapshot
}

// readPump reads messages from the WebSocket connection and dispatches them to handler.
// It runs in its own goroutine. When it returns the connection is considered done.
func (c *Client) readPump(handler func(*Client, InMessage)) {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err, gorillaws.CloseGoingAway, gorillaws.CloseAbnormalClosure) {
				log.Printf("ws: unexpected close for user %s: %v", c.User.ID, err)
			}
			return
		}

		var msg InMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.send(OutMessage{Type: MsgError, Message: "invalid message format"})
			continue
		}

		handler(c, msg)
	}
}

// writePump drains the outbound channel to the WebSocket connection.
// It also sends periodic pings to keep the connection alive.
// Runs in its own goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.outbound:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed — send a close frame and stop.
				c.conn.WriteMessage(gorillaws.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("ws: write error for user %s: %v", c.User.ID, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gorillaws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
