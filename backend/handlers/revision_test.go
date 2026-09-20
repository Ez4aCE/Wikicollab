package handlers_test

import (
	"net/http"
	"testing"
)

func TestListRevisions_InitialRevisionExists(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page", "Initial content")

	resp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, cookie)
	var revisions []map[string]any
	decodeBody(t, resp, &revisions)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(revisions) != 1 {
		t.Fatalf("expected 1 initial revision, got %d", len(revisions))
	}
	if int(revisions[0]["version"].(float64)) != 1 {
		t.Errorf("expected revision version=1, got %v", revisions[0]["version"])
	}
}

func TestListRevisions_AfterUpdate(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page", "Initial content")

	// 3 updates → 4 revisions total (1 initial + 3)
	for i := 1; i <= 3; i++ {
		doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
			"title":            "Page",
			"content":          "content v" + string(rune('0'+i+1)),
			"expected_version": i,
		}), cookie).Body.Close()
	}

	resp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, cookie)
	var revisions []map[string]any
	decodeBody(t, resp, &revisions)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(revisions) != 4 {
		t.Fatalf("expected 4 revisions (1 initial + 3 updates), got %d", len(revisions))
	}

	// Revisions should be ordered newest first (version DESC).
	if int(revisions[0]["version"].(float64)) != 4 {
		t.Errorf("expected newest revision version=4 first, got %v", revisions[0]["version"])
	}
}

func TestListRevisions_Unauthenticated(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodGet, "/api/pages/some-id/revisions", nil, nil)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListRevisions_WrongUser(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	aliceCookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	bobCookie := loginUser(t, srv, "bob", "bob@example.com", "password456")

	wikiID := createWikiHelper(t, srv, aliceCookie, "Alice Wiki")
	pageID := createPageHelper(t, srv, aliceCookie, wikiID, "Page", "Content")

	// Bob tries to list revisions of Alice's page
	resp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, bobCookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRevision_ContainsExpectedFields(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page", "Initial content")

	resp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, cookie)
	var revisions []map[string]any
	decodeBody(t, resp, &revisions)

	if len(revisions) == 0 {
		t.Fatal("expected at least one revision")
	}
	rev := revisions[0]
	for _, field := range []string{"id", "page_id", "content", "version", "created_by", "created_at"} {
		if rev[field] == nil {
			t.Errorf("revision missing field: %s", field)
		}
	}
}
