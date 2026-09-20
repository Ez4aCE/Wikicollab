package websocket_test

import (
	"testing"
	"time"

	"github.com/wikicollab/backend/models"
	wsPkg "github.com/wikicollab/backend/websocket"
)

// makeTestClient builds a minimal Client for hub tests using the exported constructor.
// We use the package-level newClient via the exported NewTestClient helper.
func makeTestClient(t *testing.T, pageID, userID string) *wsPkg.TestClient {
	t.Helper()
	return wsPkg.NewTestClient(pageID, userID)
}

func TestHub_JoinAndLeave(t *testing.T) {
	hub := wsPkg.NewHub()

	c1 := makeTestClient(t, "page-1", "user-1")
	hub.JoinRoom("page-1", c1.Client)

	// Broadcast should reach c1.
	hub.Broadcast("page-1", wsPkg.OutMessage{Type: "test", Message: "hello"})

	select {
	case msg := <-c1.Out:
		if msg.Message != "hello" {
			t.Errorf("expected 'hello', got %q", msg.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: expected broadcast message")
	}

	// Leave and broadcast — no message should arrive.
	hub.LeaveRoom("page-1", c1.Client)
	hub.Broadcast("page-1", wsPkg.OutMessage{Type: "test", Message: "after-leave"})

	select {
	case msg := <-c1.Out:
		t.Errorf("expected no message after leave, got %+v", msg)
	case <-time.After(50 * time.Millisecond):
		// Correct: no message received.
	}
}

func TestHub_BroadcastReachesAll(t *testing.T) {
	hub := wsPkg.NewHub()

	c1 := makeTestClient(t, "page-1", "user-1")
	c2 := makeTestClient(t, "page-1", "user-2")
	c3 := makeTestClient(t, "page-1", "user-3")

	hub.JoinRoom("page-1", c1.Client)
	hub.JoinRoom("page-1", c2.Client)
	hub.JoinRoom("page-1", c3.Client)

	hub.Broadcast("page-1", wsPkg.OutMessage{Type: "test", Message: "all"})

	for _, tc := range []*wsPkg.TestClient{c1, c2, c3} {
		select {
		case msg := <-tc.Out:
			if msg.Message != "all" {
				t.Errorf("expected 'all', got %q", msg.Message)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timeout: client %s did not receive broadcast", tc.Client.User.ID)
		}
	}
}

func TestHub_BroadcastExcept(t *testing.T) {
	hub := wsPkg.NewHub()

	c1 := makeTestClient(t, "page-1", "user-1")
	c2 := makeTestClient(t, "page-1", "user-2")

	hub.JoinRoom("page-1", c1.Client)
	hub.JoinRoom("page-1", c2.Client)

	hub.BroadcastExcept("page-1", wsPkg.OutMessage{Type: "test", Message: "except-c1"}, c1.Client)

	// c2 should receive.
	select {
	case msg := <-c2.Out:
		if msg.Message != "except-c1" {
			t.Errorf("expected 'except-c1', got %q", msg.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: c2 should have received broadcast")
	}

	// c1 should NOT receive.
	select {
	case msg := <-c1.Out:
		t.Errorf("c1 should not receive its own broadcast, got %+v", msg)
	case <-time.After(50 * time.Millisecond):
		// Correct.
	}
}

func TestHub_RoomDeletedWhenEmpty(t *testing.T) {
	hub := wsPkg.NewHub()

	c1 := makeTestClient(t, "page-lonely", "user-1")
	hub.JoinRoom("page-lonely", c1.Client)
	hub.LeaveRoom("page-lonely", c1.Client)

	// Broadcast to now-empty room should not panic.
	hub.Broadcast("page-lonely", wsPkg.OutMessage{Type: "test"})
}

func TestHub_MultipleRooms(t *testing.T) {
	hub := wsPkg.NewHub()

	c1 := makeTestClient(t, "page-A", "user-1")
	c2 := makeTestClient(t, "page-B", "user-2")

	hub.JoinRoom("page-A", c1.Client)
	hub.JoinRoom("page-B", c2.Client)

	hub.Broadcast("page-A", wsPkg.OutMessage{Type: "test", Message: "room-A"})

	// c1 in page-A should receive.
	select {
	case msg := <-c1.Out:
		if msg.Message != "room-A" {
			t.Errorf("expected 'room-A', got %q", msg.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: c1 should receive room-A broadcast")
	}

	// c2 in page-B should NOT receive.
	select {
	case msg := <-c2.Out:
		t.Errorf("c2 in page-B should not receive page-A broadcast, got %+v", msg)
	case <-time.After(50 * time.Millisecond):
		// Correct.
	}
}

// Keep the models import used (for TestClient construction that embeds models.User).
var _ = models.User{}
