package websocket_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	gorillaws "github.com/gorilla/websocket"
	_ "github.com/lib/pq"

	redisPkg "github.com/wikicollab/backend/redis"
	"github.com/wikicollab/backend/services"
	wsPkg "github.com/wikicollab/backend/websocket"
)

// ---------------------------------------------------------------------------
// Integration test infrastructure
// ---------------------------------------------------------------------------

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("ping db: %v", err)
	}
	runMigrations(t, db)
	cleanTables(t, db)
	t.Cleanup(func() { cleanTables(t, db); db.Close() })
	return db
}

func testRedis(t *testing.T) *redisPkg.LockStore {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = os.Getenv("REDIS_URL")
	}
	if url == "" {
		t.Skip("TEST_REDIS_URL not set — skipping integration test")
	}
	client, err := redisPkg.NewClient(url)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	client.FlushDB(context.Background())
	t.Cleanup(func() { client.Close() })
	return redisPkg.NewLockStore(client)
}

func runMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	dir := migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}
}

func migrationsDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// websocket/handler_test.go → backend/ → repo root → migrations/
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
		candidate := filepath.Join(repoRoot, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("..", "..", "migrations")
}

func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, tbl := range []string{"revisions", "pages", "wikis", "users"} {
		db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl))
	}
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func seedUser(t *testing.T, db *sql.DB, username, email string) string {
	t.Helper()
	authSvc := services.NewAuthService(db)
	user, err := authSvc.Register(context.Background(), services.RegisterInput{
		Username: username,
		Email:    email,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user.ID
}

func seedWiki(t *testing.T, db *sql.DB, ownerID, name string) string {
	t.Helper()
	wikiSvc := services.NewWikiService(db)
	wiki, err := wikiSvc.CreateWiki(context.Background(), services.CreateWikiInput{
		Name:    name,
		OwnerID: ownerID,
	})
	if err != nil {
		t.Fatalf("seed wiki: %v", err)
	}
	return wiki.ID
}

func seedPage(t *testing.T, db *sql.DB, wikiID, createdBy string) string {
	t.Helper()
	pageSvc := services.NewPageService(db)
	page, err := pageSvc.CreatePage(context.Background(), services.CreatePageInput{
		WikiID:    wikiID,
		Title:     "Test Page",
		Content:   "# Test\n\nContent.",
		CreatedBy: createdBy,
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}
	return page.ID
}

// ---------------------------------------------------------------------------
// Server helpers
// ---------------------------------------------------------------------------

func buildServer(t *testing.T, db *sql.DB, lockStore *redisPkg.LockStore) (*httptest.Server, sessions.Store) {
	t.Helper()
	store := sessions.NewCookieStore([]byte("test-secret"))
	pageSvc := services.NewPageService(db)
	wikiSvc := services.NewWikiService(db)
	hub := wsPkg.NewHub()
	wsHandler := wsPkg.NewHandler(hub, lockStore, pageSvc, wikiSvc, store, db, 30*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/pages/", func(w http.ResponseWriter, r *http.Request) {
		// Extract pageID from path manually (no gorilla/mux in test).
		r = setPathVar(r, r.URL.Path[len("/ws/pages/"):])
		wsHandler.ServeWS(w, r)
	})
	return httptest.NewServer(mux), store
}

// setPathVar injects the page ID into the request using gorilla/mux vars.
// We use a simplified approach for test purposes.
func setPathVar(r *http.Request, pageID string) *http.Request {
	// gorilla/mux vars won't be set in a plain http.ServeMux handler.
	// We fake it by passing pageID as a query param and patching handler.
	// Instead, since gorilla/mux.Vars reads from context, use a test shim.
	return r.WithContext(wsPkg.WithTestPageID(r.Context(), pageID))
}

// ---------------------------------------------------------------------------
// WebSocket dial helpers
// ---------------------------------------------------------------------------

// dialWS connects to the test server's WebSocket endpoint with the given session cookie.
func dialWS(t *testing.T, srv *httptest.Server, pageID string, cookie *http.Cookie) *gorillaws.Conn {
	t.Helper()
	url := "ws" + srv.URL[len("http"):] + "/ws/pages/" + pageID
	headers := http.Header{}
	if cookie != nil {
		headers.Add("Cookie", cookie.String())
	}
	conn, resp, err := gorillaws.DefaultDialer.Dial(url, headers)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial ws (status=%d): %v", resp.StatusCode, err)
		}
		t.Fatalf("dial ws: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// sessionCookie creates a session cookie for the given user ID.
func sessionCookie(t *testing.T, store sessions.Store, userID string) *http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	session, err := store.Get(req, "wikicollab-session")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	session.Values["user_id"] = userID
	if err := store.Save(req, w, session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	return cookies[0]
}

func readMsg(t *testing.T, conn *gorillaws.Conn) wsPkg.OutMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	var msg wsPkg.OutMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	return msg
}

func sendMsg(t *testing.T, conn *gorillaws.Conn, msg wsPkg.InMessage) {
	t.Helper()
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write ws message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestWS_Unauthenticated_Rejected(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, _ := buildServer(t, db, locks)
	defer srv.Close()

	userID := seedUser(t, db, "alice", "alice@example.com")
	wikiID := seedWiki(t, db, userID, "Wiki")
	pageID := seedPage(t, db, wikiID, userID)

	url := "ws" + srv.URL[len("http"):] + "/ws/pages/" + pageID
	_, resp, err := gorillaws.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWS_LockAcquire_Broadcast(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, store := buildServer(t, db, locks)
	defer srv.Close()

	userID := seedUser(t, db, "alice", "alice@example.com")
	wikiID := seedWiki(t, db, userID, "Wiki")
	pageID := seedPage(t, db, wikiID, userID)

	cookie := sessionCookie(t, store, userID)

	// Two clients join the same page.
	conn1 := dialWS(t, srv, pageID, cookie)
	conn2 := dialWS(t, srv, pageID, cookie)

	// Client 1 acquires a lock.
	sendMsg(t, conn1, wsPkg.InMessage{Type: wsPkg.MsgLock, BlockID: "block-1"})

	// Both clients should receive lock_acquired.
	msg1 := readMsg(t, conn1)
	msg2 := readMsg(t, conn2)

	for _, msg := range []wsPkg.OutMessage{msg1, msg2} {
		if msg.Type != wsPkg.MsgLockAcquired {
			t.Errorf("expected lock_acquired, got %s", msg.Type)
		}
		if msg.BlockID != "block-1" {
			t.Errorf("expected block_id=block-1, got %s", msg.BlockID)
		}
	}
}

func TestWS_LockDenied_WhenAlreadyHeld(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, store := buildServer(t, db, locks)
	defer srv.Close()

	aliceID := seedUser(t, db, "alice", "alice@example.com")
	bobID := seedUser(t, db, "bob", "bob@example.com")
	wikiID := seedWiki(t, db, aliceID, "Wiki")
	// Give Bob ownership too — for this test we share the wiki.
	// (In a real scenario, both would be owners or viewers.)
	// Simplification: reuse alice's cookie for both connections.
	pageID := seedPage(t, db, wikiID, aliceID)

	_ = bobID
	aliceCookie := sessionCookie(t, store, aliceID)

	conn1 := dialWS(t, srv, pageID, aliceCookie)
	conn2 := dialWS(t, srv, pageID, aliceCookie)

	// conn1 acquires lock on block-1.
	sendMsg(t, conn1, wsPkg.InMessage{Type: wsPkg.MsgLock, BlockID: "block-1"})
	readMsg(t, conn1) // lock_acquired (self)
	readMsg(t, conn2) // lock_acquired (broadcast to conn2)

	// conn2 tries to acquire the same block.
	sendMsg(t, conn2, wsPkg.InMessage{Type: wsPkg.MsgLock, BlockID: "block-1"})

	msg := readMsg(t, conn2)
	if msg.Type != wsPkg.MsgLockDenied {
		t.Errorf("expected lock_denied, got %s", msg.Type)
	}
	if msg.BlockID != "block-1" {
		t.Errorf("expected block_id=block-1, got %s", msg.BlockID)
	}
}

func TestWS_LockRelease_Broadcast(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, store := buildServer(t, db, locks)
	defer srv.Close()

	userID := seedUser(t, db, "alice", "alice@example.com")
	wikiID := seedWiki(t, db, userID, "Wiki")
	pageID := seedPage(t, db, wikiID, userID)
	cookie := sessionCookie(t, store, userID)

	conn1 := dialWS(t, srv, pageID, cookie)
	conn2 := dialWS(t, srv, pageID, cookie)

	// Acquire.
	sendMsg(t, conn1, wsPkg.InMessage{Type: wsPkg.MsgLock, BlockID: "block-2"})
	readMsg(t, conn1)
	readMsg(t, conn2)

	// Release.
	sendMsg(t, conn1, wsPkg.InMessage{Type: wsPkg.MsgUnlock, BlockID: "block-2"})

	msg1 := readMsg(t, conn1)
	msg2 := readMsg(t, conn2)

	for _, msg := range []wsPkg.OutMessage{msg1, msg2} {
		if msg.Type != wsPkg.MsgLockReleased {
			t.Errorf("expected lock_released, got %s", msg.Type)
		}
	}
}

func TestWS_PageUpdate_Broadcast(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, store := buildServer(t, db, locks)
	defer srv.Close()

	userID := seedUser(t, db, "alice", "alice@example.com")
	wikiID := seedWiki(t, db, userID, "Wiki")
	pageID := seedPage(t, db, wikiID, userID)
	cookie := sessionCookie(t, store, userID)

	conn1 := dialWS(t, srv, pageID, cookie)
	conn2 := dialWS(t, srv, pageID, cookie)

	sendMsg(t, conn1, wsPkg.InMessage{
		Type:            wsPkg.MsgPageUpdate,
		Title:           "Test Page",
		Content:         "# Updated content",
		ExpectedVersion: 1,
	})

	// Both clients should receive page_updated.
	msg1 := readMsg(t, conn1)
	msg2 := readMsg(t, conn2)

	for _, msg := range []wsPkg.OutMessage{msg1, msg2} {
		if msg.Type != wsPkg.MsgPageUpdated {
			t.Errorf("expected page_updated, got %s (message: %s)", msg.Type, msg.Message)
		}
		if msg.Version != 2 {
			t.Errorf("expected version=2, got %d", msg.Version)
		}
	}
}

func TestWS_PageUpdate_VersionConflict(t *testing.T) {
	db := testDB(t)
	locks := testRedis(t)
	srv, store := buildServer(t, db, locks)
	defer srv.Close()

	userID := seedUser(t, db, "alice", "alice@example.com")
	wikiID := seedWiki(t, db, userID, "Wiki")
	pageID := seedPage(t, db, wikiID, userID)
	cookie := sessionCookie(t, store, userID)

	conn1 := dialWS(t, srv, pageID, cookie)

	// First update succeeds (version 1 → 2).
	sendMsg(t, conn1, wsPkg.InMessage{
		Type:            wsPkg.MsgPageUpdate,
		Title:           "Page",
		Content:         "v2",
		ExpectedVersion: 1,
	})
	readMsg(t, conn1) // consume page_updated

	// Second update with stale version 1 should get an error.
	sendMsg(t, conn1, wsPkg.InMessage{
		Type:            wsPkg.MsgPageUpdate,
		Title:           "Page",
		Content:         "stale",
		ExpectedVersion: 1,
	})
	msg := readMsg(t, conn1)
	if msg.Type != wsPkg.MsgError {
		t.Errorf("expected error, got %s", msg.Type)
	}
}
