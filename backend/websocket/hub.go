package websocket

import (
	"sync"
)

// Hub manages all active page collaboration rooms.
// One Hub per backend process — we assume a single Go instance.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*Room // keyed by pageID
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
	}
}

// JoinRoom adds a client to the room for the given pageID, creating the room if needed.
func (h *Hub) JoinRoom(pageID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[pageID]
	if !ok {
		room = newRoom(pageID)
		h.rooms[pageID] = room
	}
	room.add(client)
}

// LeaveRoom removes a client from its room.
// If the room becomes empty it is deleted to avoid unbounded map growth.
func (h *Hub) LeaveRoom(pageID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[pageID]
	if !ok {
		return
	}
	room.remove(client)
	if room.empty() {
		delete(h.rooms, pageID)
	}
}

// Broadcast sends msg to every client in the room.
func (h *Hub) Broadcast(pageID string, msg OutMessage) {
	h.mu.RLock()
	room, ok := h.rooms[pageID]
	h.mu.RUnlock()
	if ok {
		room.broadcast(msg, nil)
	}
}

// BroadcastExcept sends msg to every client in the room except the excluded one.
func (h *Hub) BroadcastExcept(pageID string, msg OutMessage, except *Client) {
	h.mu.RLock()
	room, ok := h.rooms[pageID]
	h.mu.RUnlock()
	if ok {
		room.broadcast(msg, except)
	}
}

// GetRoomLocks returns a snapshot of all block locks held by existing clients
// in the room (excluding the given client). Used to sync a new joiner.
// Returns map[blockID]→{UserID, Username}.
func (h *Hub) GetRoomLocks(pageID string, exclude *Client) []OutMessage {
	h.mu.RLock()
	room, ok := h.rooms[pageID]
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	room.mu.RLock()
	defer room.mu.RUnlock()
	var msgs []OutMessage
	for c := range room.clients {
		if c == exclude {
			continue
		}
		c.locksMu.Lock()
		for blockID := range c.ownedLocks {
			msgs = append(msgs, OutMessage{
				Type:     MsgLockAcquired,
				BlockID:  blockID,
				UserID:   c.User.ID,
				Username: c.User.Username,
			})
		}
		c.locksMu.Unlock()
	}
	return msgs
}

// SendTo delivers msg to a single client only.
func (h *Hub) SendTo(client *Client, msg OutMessage) {
	client.send(msg)
}

// Room represents one active page collaboration room.
type Room struct {
	pageID  string
	clients map[*Client]struct{}
	mu      sync.RWMutex
}

func newRoom(pageID string) *Room {
	return &Room{
		pageID:  pageID,
		clients: make(map[*Client]struct{}),
	}
}

func (r *Room) add(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
}

func (r *Room) remove(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

func (r *Room) empty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients) == 0
}

// broadcast sends msg to all clients, skipping except (may be nil to send to all).
// Non-blocking: a slow client's buffered channel is dropped rather than blocking others.
func (r *Room) broadcast(msg OutMessage, except *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		if c == except {
			continue
		}
		c.send(msg)
	}
}
