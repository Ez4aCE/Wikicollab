package handlers_test

import (
	"net/http"
	"testing"
)

func TestRegister_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "password123",
	}), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body map[string]any
	decodeBody(t, resp, &body)

	if body["id"] == nil {
		t.Error("expected id in response")
	}
	if body["username"] != "alice" {
		t.Errorf("expected username=alice, got %v", body["username"])
	}
	if body["password_hash"] != nil {
		t.Error("password_hash must not be returned")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	body := map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "password123",
	}

	doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, body), nil).Body.Close()

	body["username"] = "alice2"
	resp := doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, body), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", resp.StatusCode)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	body := map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "password123",
	}
	doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, body), nil).Body.Close()

	body["email"] = "alice2@example.com"
	resp := doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, body), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate username, got %d", resp.StatusCode)
	}
}

func TestRegister_Validation(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cases := []map[string]string{
		{"username": "", "email": "a@b.com", "password": "password123"},
		{"username": "alice", "email": "not-an-email", "password": "password123"},
		{"username": "alice", "email": "a@b.com", "password": "short"},
	}

	for _, c := range cases {
		resp := doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, c), nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for input %v, got %d", c, resp.StatusCode)
		}
	}
}

func TestLogin_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	// Register
	doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "password123",
	}), nil).Body.Close()

	// Login
	resp := doRequest(t, srv, http.MethodPost, "/api/login", jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "password123",
	}), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "wikicollab-session" {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie must be HttpOnly")
			}
			break
		}
	}
	if !found {
		t.Error("no session cookie in login response")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	doRequest(t, srv, http.MethodPost, "/api/register", jsonBody(t, map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "password123",
	}), nil).Body.Close()

	resp := doRequest(t, srv, http.MethodPost, "/api/login", jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "wrongpassword",
	}), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "/api/login", jsonBody(t, map[string]string{
		"email":    "nobody@example.com",
		"password": "password123",
	}), nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")

	// Confirm /api/me works before logout.
	resp := doRequest(t, srv, http.MethodGet, "/api/me", nil, cookie)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 before logout, got %d", resp.StatusCode)
	}

	// Logout.
	logoutResp := doRequest(t, srv, http.MethodPost, "/api/logout", nil, cookie)
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", logoutResp.StatusCode)
	}

	// After logout the session cookie should be expired (MaxAge < 0).
	for _, c := range logoutResp.Cookies() {
		if c.Name == "wikicollab-session" && c.MaxAge < 0 {
			return // success
		}
	}
}

func TestProtectedEndpoint_Unauthenticated(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/api/wikis"},
		{http.MethodPost, "/api/wikis"},
	}

	for _, e := range endpoints {
		resp := doRequest(t, srv, e.method, e.path, jsonBody(t, map[string]string{}), nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", e.method, e.path, resp.StatusCode)
		}
	}
}

func TestMe_ReturnsCurrentUser(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")

	resp := doRequest(t, srv, http.MethodGet, "/api/me", nil, cookie)

	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["email"] != "alice@example.com" {
		t.Errorf("expected email=alice@example.com, got %v", body["email"])
	}
	if _, hasHash := body["password_hash"]; hasHash {
		t.Error("password_hash must not be returned from /api/me")
	}
}
