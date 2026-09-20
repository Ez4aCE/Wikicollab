package handlers_test

import (
	"bytes"
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

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"

	"github.com/wikicollab/backend/handlers"
	"github.com/wikicollab/backend/middleware"
	"github.com/wikicollab/backend/services"
)

// testDB opens a connection to the test database specified by TEST_DATABASE_URL.
// If the env var is not set, the test is skipped.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Retry a few times — the DB may not be ready immediately.
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	runMigrations(t, db)
	cleanTables(t, db)

	t.Cleanup(func() {
		cleanTables(t, db)
		db.Close()
	})

	return db
}

// runMigrations applies SQL files from the migrations directory.
func runMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	dir := migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir %s: %v", dir, err)
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
		// handlers/setup_test.go → backend/ → repo root → migrations/
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
		candidate := filepath.Join(repoRoot, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("..", "..", "migrations")
}

// cleanTables truncates all tables in reverse FK order.
func cleanTables(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{"revisions", "pages", "wikis", "users"}
	for _, tbl := range tables {
		if _, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl)); err != nil {
			t.Logf("truncate %s: %v", tbl, err)
		}
	}
}

// testServer builds an httptest server with a full router — same setup as main.go.
func testServer(t *testing.T, db *sql.DB) (*httptest.Server, sessions.Store) {
	t.Helper()

	store := sessions.NewCookieStore([]byte("test-secret-key"))

	authSvc := services.NewAuthService(db)
	wikiSvc := services.NewWikiService(db)
	pageSvc := services.NewPageService(db)
	revSvc := services.NewRevisionService(db)

	authHandler := handlers.NewAuthHandler(authSvc, store)
	wikiHandler := handlers.NewWikiHandler(wikiSvc)
	pageHandler := handlers.NewPageHandler(pageSvc, wikiSvc)
	revHandler := handlers.NewRevisionHandler(revSvc, pageSvc, wikiSvc)

	requireAuth := middleware.RequireAuth(store, db)

	r := mux.NewRouter()

	r.HandleFunc("/health", handlers.HealthHandler).Methods(http.MethodGet)

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	api.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	api.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost)

	protected := api.NewRoute().Subrouter()
	protected.Use(requireAuth)

	protected.HandleFunc("/me", authHandler.Me).Methods(http.MethodGet)
	protected.HandleFunc("/wikis", wikiHandler.ListWikis).Methods(http.MethodGet)
	protected.HandleFunc("/wikis", wikiHandler.CreateWiki).Methods(http.MethodPost)
	protected.HandleFunc("/wikis/{id}", wikiHandler.GetWiki).Methods(http.MethodGet)
	protected.HandleFunc("/wikis/{id}/pages", pageHandler.ListPages).Methods(http.MethodGet)
	protected.HandleFunc("/wikis/{id}/pages", pageHandler.CreatePage).Methods(http.MethodPost)
	protected.HandleFunc("/pages/{id}", pageHandler.GetPage).Methods(http.MethodGet)
	protected.HandleFunc("/pages/{id}", pageHandler.UpdatePage).Methods(http.MethodPut)
	protected.HandleFunc("/pages/{id}/revisions", revHandler.ListRevisions).Methods(http.MethodGet)

	return httptest.NewServer(r), store
}

// jsonBody encodes v as JSON and returns a bytes.Buffer.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return bytes.NewBuffer(b)
}

// doRequest sends an HTTP request to the test server and returns the response.
// It optionally attaches a session cookie for authenticated requests.
func doRequest(t *testing.T, srv *httptest.Server, method, path string, body *bytes.Buffer, cookie *http.Cookie) *http.Response {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(context.Background(), method, srv.URL+path, body)
	} else {
		req, err = http.NewRequestWithContext(context.Background(), method, srv.URL+path, nil)
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// decodeBody decodes a JSON response body into v.
func decodeBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

// loginUser registers and logs in a user, returning the session cookie.
func loginUser(t *testing.T, srv *httptest.Server, username, email, password string) *http.Cookie {
	t.Helper()

	// Register
	regResp := doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}), nil)
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("register failed: %d", regResp.StatusCode)
	}
	regResp.Body.Close()

	// Login
	loginResp := doRequest(t, srv, http.MethodPost, "/api/login", jsonBody(t, map[string]string{
		"email":    email,
		"password": password,
	}), nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: %d", loginResp.StatusCode)
	}
	loginResp.Body.Close()

	for _, c := range loginResp.Cookies() {
		if c.Name == "wikicollab-session" {
			return c
		}
	}
	t.Fatal("no session cookie returned after login")
	return nil
}
