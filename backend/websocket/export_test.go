package websocket

import "github.com/wikicollab/backend/models"

// TestClient is a test helper that wraps a Client with a readable out channel.
// It allows hub tests to run without a real WebSocket connection.
type TestClient struct {
	Client *Client
	Out    chan OutMessage
}

// NewTestClient creates a Client whose outbound channel is also accessible via Out.
// The Client's send method writes to the same channel.
func NewTestClient(pageID, userID string) *TestClient {
	ch := make(chan OutMessage, 32)
	c := &Client{
		User:       &models.User{ID: userID, Username: userID},
		PageID:     pageID,
		outbound:   ch,
		ownedLocks: make(map[string]string),
	}
	return &TestClient{Client: c, Out: ch}
}
