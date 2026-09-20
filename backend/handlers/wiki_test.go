package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCreateWiki_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")

	resp := doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
		"name": "Computer Science",
	}), cookie)

	var body map[string]any
	decodeBody(t, resp, &body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if body["name"] != "Computer Science" {
		t.Errorf("expected name=Computer Science, got %v", body["name"])
	}
	if body["id"] == nil {
		t.Error("expected id in response")
	}
}

func TestCreateWiki_EmptyName(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")

	resp := doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
		"name": "",
	}), cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty wiki name, got %d", resp.StatusCode)
	}
}

func TestListWikis_OnlyOwn(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	aliceCookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	bobCookie := loginUser(t, srv, "bob", "bob@example.com", "password456")

	// Alice creates 2 wikis
	for i := 0; i < 2; i++ {
		doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
			"name": fmt.Sprintf("Alice Wiki %d", i),
		}), aliceCookie).Body.Close()
	}

	// Bob creates 1 wiki
	doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
		"name": "Bob Wiki",
	}), bobCookie).Body.Close()

	// Alice should see only her 2 wikis
	resp := doRequest(t, srv, http.MethodGet, "/api/wikis", nil, aliceCookie)
	var aliceWikis []map[string]any
	decodeBody(t, resp, &aliceWikis)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(aliceWikis) != 2 {
		t.Errorf("expected 2 wikis for Alice, got %d", len(aliceWikis))
	}

	// Bob should see only his 1 wiki
	resp2 := doRequest(t, srv, http.MethodGet, "/api/wikis", nil, bobCookie)
	var bobWikis []map[string]any
	decodeBody(t, resp2, &bobWikis)

	if len(bobWikis) != 1 {
		t.Errorf("expected 1 wiki for Bob, got %d", len(bobWikis))
	}
}

func TestListWikis_Unauthenticated(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodGet, "/api/wikis", nil, nil)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetWiki_NotOwner(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	aliceCookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	bobCookie := loginUser(t, srv, "bob", "bob@example.com", "password456")

	// Alice creates a wiki
	wikiResp := doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
		"name": "Alice's Wiki",
	}), aliceCookie)
	var wiki map[string]any
	decodeBody(t, wikiResp, &wiki)

	wikiID := wiki["id"].(string)

	// Bob tries to access Alice's wiki
	resp := doRequest(t, srv, http.MethodGet, "/api/wikis/"+wikiID, nil, bobCookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
