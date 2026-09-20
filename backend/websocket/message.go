package websocket

// Message types sent from client to server.
const (
	MsgLock       = "lock"        // acquire a block lock
	MsgLockRenew  = "lock_renew"  // renew own lock
	MsgUnlock     = "unlock"      // release a block lock
	MsgPageUpdate = "page_update" // save page content
)

// Message types sent from server to client.
const (
	MsgLockAcquired = "lock_acquired" // a block was locked (broadcast)
	MsgLockDenied   = "lock_denied"   // lock request refused (to requester only)
	MsgLockReleased = "lock_released" // a block was unlocked (broadcast)
	MsgPageUpdated  = "page_updated"  // page content changed (broadcast)
	MsgError        = "error"         // error response (to one client)
)

// InMessage is the generic shape of every client-to-server message.
// The handler switches on Type to decode further fields.
type InMessage struct {
	Type    string `json:"type"`
	BlockID string `json:"block_id,omitempty"`

	// page_update fields
	Title           string `json:"title,omitempty"`
	Content         string `json:"content,omitempty"`
	ExpectedVersion int    `json:"expected_version,omitempty"`
}

// OutMessage is the generic shape of every server-to-client message.
type OutMessage struct {
	Type    string `json:"type"`
	BlockID string `json:"block_id,omitempty"`

	// lock_acquired / lock_denied
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`

	// page_updated
	PageID  string `json:"page_id,omitempty"`
	Version int    `json:"version,omitempty"`
	Content string `json:"content,omitempty"`
	Title   string `json:"title,omitempty"`

	// error
	Message string `json:"message,omitempty"`
}
