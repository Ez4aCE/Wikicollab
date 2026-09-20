package websocket

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	gorillaws "github.com/gorilla/websocket"

	"github.com/wikicollab/backend/middleware"
	redisPkg "github.com/wikicollab/backend/redis"
	"github.com/wikicollab/backend/services"
)

// pageIDKey is the context key used to pass the page ID to the WebSocket handler.
// In production it is populated by gorilla/mux. In tests it is injected manually.
type pageIDKey struct{}

// WithTestPageID injects a page ID into the context.
// Used by tests that cannot rely on gorilla/mux route variables.
func WithTestPageID(ctx context.Context, pageID string) context.Context {
	return context.WithValue(ctx, pageIDKey{}, pageID)
}

// pageIDFromRequest extracts the page ID first from gorilla/mux vars, then
// falls back to the context value set by WithTestPageID.
func pageIDFromRequest(r *http.Request) string {
	if id := mux.Vars(r)["id"]; id != "" {
		return id
	}
	id, _ := r.Context().Value(pageIDKey{}).(string)
	return id
}

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Accept any origin — CORS policy is enforced by the HTTP-level middleware.
	// Members 3 & 4 connect from localhost during development; adjust for production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler is the HTTP handler for GET /ws/pages/:id.
type Handler struct {
	hub          *Hub
	locks        *redisPkg.LockStore
	pageSvc      *services.PageService
	wikiSvc      *services.WikiService
	sessionStore sessions.Store
	db           *sql.DB
	lockTTL      time.Duration
}

// NewHandler creates a WebSocket Handler.
func NewHandler(
	hub *Hub,
	locks *redisPkg.LockStore,
	pageSvc *services.PageService,
	wikiSvc *services.WikiService,
	sessionStore sessions.Store,
	db *sql.DB,
	lockTTL time.Duration,
) *Handler {
	return &Handler{
		hub:          hub,
		locks:        locks,
		pageSvc:      pageSvc,
		wikiSvc:      wikiSvc,
		sessionStore: sessionStore,
		db:           db,
		lockTTL:      lockTTL,
	}
}

// ServeWS handles GET /ws/pages/:id.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	pageID := pageIDFromRequest(r)
	if pageID == "" {
		http.Error(w, `{"error":"missing page id"}`, http.StatusBadRequest)
		return
	}

	// 1. Authenticate using the existing session mechanism — no separate auth system.
	user, err := middleware.GetSessionUser(h.sessionStore, h.db, r)
	if err != nil || user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// 2. Verify the page exists and the user owns the wiki it belongs to.
	ctx := r.Context()
	page, err := h.pageSvc.GetPage(ctx, pageID)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	if err := h.wikiSvc.AssertAccess(ctx, page.WikiID, user.ID); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// 3. Upgrade HTTP → WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}

	// 4. Create the client and join the page's collaboration room.
	client := newClient(conn, user, pageID)
	h.hub.JoinRoom(pageID, client)
	log.Printf("ws: user %s (%s) joined page %s", user.Username, user.ID, pageID)

	// 5. writePump runs in its own goroutine, draining the outbound channel.
	//    Must start BEFORE SendTo so the channel is being consumed.
	go client.writePump()

	// 6. Send the CURRENT page state immediately so the new client is in sync
	//    with any changes made by other collaborators already in the room.
	h.hub.SendTo(client, OutMessage{
		Type:    MsgPageUpdated,
		PageID:  page.ID,
		Title:   page.Title,
		Content: page.Content,
		Version: page.Version,
	})

	// 7. Send existing lock states so the new client sees who is editing what.
	for _, lockMsg := range h.hub.GetRoomLocks(pageID, client) {
		h.hub.SendTo(client, lockMsg)
	}

	// 8. readPump blocks here until the client disconnects.
	client.readPump(h.handleMessage)

	// 9. Cleanup: release all locks and leave the room.
	h.handleDisconnect(client)
}

// handleMessage dispatches an inbound WebSocket message.
func (h *Handler) handleMessage(client *Client, msg InMessage) {
	ctx := context.Background()
	switch msg.Type {
	case MsgLock:
		h.handleLockAcquire(ctx, client, msg)
	case MsgLockRenew:
		h.handleLockRenew(ctx, client, msg)
	case MsgUnlock:
		h.handleUnlock(ctx, client, msg)
	case MsgPageUpdate:
		h.handlePageUpdate(ctx, client, msg)
	default:
		client.send(OutMessage{Type: MsgError, Message: "unknown message type: " + msg.Type})
	}
}

// handleLockAcquire processes a "lock" message.
func (h *Handler) handleLockAcquire(ctx context.Context, client *Client, msg InMessage) {
	if msg.BlockID == "" {
		client.send(OutMessage{Type: MsgError, Message: "block_id is required"})
		return
	}

	token := generateToken()
	acquired, err := h.locks.AcquireLock(ctx, client.PageID, msg.BlockID, token, h.lockTTL)
	if err != nil {
		log.Printf("ws: acquire lock error: %v", err)
		client.send(OutMessage{Type: MsgError, Message: "lock service unavailable"})
		return
	}

	if !acquired {
		// Lock is held by someone else — notify the requester only.
		client.send(OutMessage{
			Type:     MsgLockDenied,
			BlockID:  msg.BlockID,
			UserID:   client.User.ID,
			Username: client.User.Username,
		})
		return
	}

	// Track ownership locally, then broadcast to the room.
	client.addLock(msg.BlockID, token)
	h.hub.Broadcast(client.PageID, OutMessage{
		Type:     MsgLockAcquired,
		BlockID:  msg.BlockID,
		UserID:   client.User.ID,
		Username: client.User.Username,
	})
}

// handleLockRenew processes a "lock_renew" message.
func (h *Handler) handleLockRenew(ctx context.Context, client *Client, msg InMessage) {
	if msg.BlockID == "" {
		client.send(OutMessage{Type: MsgError, Message: "block_id is required"})
		return
	}

	token := client.lockToken(msg.BlockID)
	if token == "" {
		client.send(OutMessage{Type: MsgError, Message: "you do not hold the lock for block " + msg.BlockID})
		return
	}

	renewed, err := h.locks.RenewLock(ctx, client.PageID, msg.BlockID, token, h.lockTTL)
	if err != nil {
		log.Printf("ws: renew lock error: %v", err)
		client.send(OutMessage{Type: MsgError, Message: "lock service unavailable"})
		return
	}

	if !renewed {
		// TTL expired between client tracking and renewal.
		client.removeLock(msg.BlockID)
		client.send(OutMessage{Type: MsgError, Message: "lock expired for block " + msg.BlockID})
	}
	// Renewal is silent — no broadcast needed.
}

// handleUnlock processes an "unlock" message.
func (h *Handler) handleUnlock(ctx context.Context, client *Client, msg InMessage) {
	if msg.BlockID == "" {
		client.send(OutMessage{Type: MsgError, Message: "block_id is required"})
		return
	}

	token := client.lockToken(msg.BlockID)
	if token == "" {
		// Client doesn't think it owns the lock — silently ignore.
		return
	}

	released, err := h.locks.ReleaseLock(ctx, client.PageID, msg.BlockID, token)
	if err != nil {
		log.Printf("ws: release lock error: %v", err)
		client.send(OutMessage{Type: MsgError, Message: "lock service unavailable"})
		return
	}

	client.removeLock(msg.BlockID)

	if released {
		h.hub.Broadcast(client.PageID, OutMessage{
			Type:    MsgLockReleased,
			BlockID: msg.BlockID,
		})
	}
}

// handlePageUpdate processes a "page_update" message.
// It calls PageService.UpdatePage — the same service used by the REST API.
func (h *Handler) handlePageUpdate(ctx context.Context, client *Client, msg InMessage) {
	if msg.Title == "" || msg.Content == "" {
		client.send(OutMessage{Type: MsgError, Message: "title and content are required"})
		return
	}
	if msg.ExpectedVersion <= 0 {
		client.send(OutMessage{Type: MsgError, Message: "expected_version is required"})
		return
	}

	page, err := h.pageSvc.UpdatePage(ctx, services.UpdatePageInput{
		PageID:          client.PageID,
		Title:           msg.Title,
		Content:         msg.Content,
		ExpectedVersion: msg.ExpectedVersion,
		UpdatedBy:       client.User.ID,
	})
	if err != nil {
		if errors.Is(err, services.ErrVersionConflict) {
			// Resync the client with the latest server state so they can retry.
			latestPage, fetchErr := h.pageSvc.GetPage(ctx, client.PageID)
			if fetchErr == nil {
				h.hub.SendTo(client, OutMessage{
					Type:    MsgPageUpdated,
					PageID:  latestPage.ID,
					Title:   latestPage.Title,
					Content: latestPage.Content,
					Version: latestPage.Version,
				})
			} else {
				client.send(OutMessage{Type: MsgError, Message: "version conflict — please refresh"})
			}
		} else if errors.Is(err, services.ErrValidation) {
			client.send(OutMessage{Type: MsgError, Message: err.Error()})
		} else {
			log.Printf("ws: page update error: %v", err)
			client.send(OutMessage{Type: MsgError, Message: "page update failed"})
		}
		return
	}

	// Broadcast to all clients in the room so everyone sees the new content.
	h.hub.Broadcast(client.PageID, OutMessage{
		Type:    MsgPageUpdated,
		PageID:  page.ID,
		Title:   page.Title,
		Content: page.Content,
		Version: page.Version,
	})
}

// handleDisconnect releases all locks owned by the client and removes it from the room.
func (h *Handler) handleDisconnect(client *Client) {
	ctx := context.Background()

	locks := client.drainLocks()
	for blockID, token := range locks {
		released, err := h.locks.ReleaseLock(ctx, client.PageID, blockID, token)
		if err != nil {
			log.Printf("ws: disconnect cleanup release error (page=%s block=%s): %v",
				client.PageID, blockID, err)
			continue
		}
		if released {
			h.hub.Broadcast(client.PageID, OutMessage{
				Type:    MsgLockReleased,
				BlockID: blockID,
			})
		}
	}

	h.hub.LeaveRoom(client.PageID, client)
	log.Printf("ws: user %s left page %s", client.User.ID, client.PageID)
}

// generateToken creates a random 16-byte hex string as a lock ownership token.
func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
