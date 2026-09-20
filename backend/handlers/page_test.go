package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// createWikiHelper creates a wiki and returns its ID.
func createWikiHelper(t *testing.T, srv *httptest.Server, cookie *http.Cookie, name string) string {
	t.Helper()
	resp := doRequest(t, srv, http.MethodPost, "/api/wikis", jsonBody(t, map[string]string{
		"name": name,
	}), cookie)
	var body map[string]any
	decodeBody(t, resp, &body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create wiki: expected 201, got %d", resp.StatusCode)
	}
	return body["id"].(string)
}

// createPageHelper creates a page and returns its ID.
func createPageHelper(t *testing.T, srv *httptest.Server, cookie *http.Cookie, wikiID, title, content string) string {
	t.Helper()
	resp := doRequest(t, srv, http.MethodPost, "/api/wikis/"+wikiID+"/pages", jsonBody(t, map[string]string{
		"title":   title,
		"content": content,
	}), cookie)
	var body map[string]any
	decodeBody(t, resp, &body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create page: expected 201, got %d", resp.StatusCode)
	}
	return body["id"].(string)
}

func TestCreatePage_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "My Wiki")

	resp := doRequest(t, srv, http.MethodPost, "/api/wikis/"+wikiID+"/pages", jsonBody(t, map[string]string{
		"title":   "Operating Systems",
		"content": "# Operating Systems\n\nAn OS manages resources.",
	}), cookie)

	var page map[string]any
	decodeBody(t, resp, &page)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if page["title"] != "Operating Systems" {
		t.Errorf("expected title=Operating Systems, got %v", page["title"])
	}
	if int(page["version"].(float64)) != 1 {
		t.Errorf("expected version=1, got %v", page["version"])
	}
}

func TestGetPage_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page 1", "Content here")

	resp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID, nil, cookie)
	var page map[string]any
	decodeBody(t, resp, &page)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if page["id"] != pageID {
		t.Errorf("expected id=%s, got %v", pageID, page["id"])
	}
}

func TestUpdatePage_Success(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page 1", "Initial content")

	resp := doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
		"title":            "Page 1 Updated",
		"content":          "Updated content",
		"expected_version": 1,
	}), cookie)

	var updated map[string]any
	decodeBody(t, resp, &updated)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if int(updated["version"].(float64)) != 2 {
		t.Errorf("expected version=2 after update, got %v", updated["version"])
	}
	if updated["title"] != "Page 1 Updated" {
		t.Errorf("expected updated title, got %v", updated["title"])
	}
}

func TestUpdatePage_VersionConflict(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page 1", "Initial content")

	// First update: version 1 → 2
	doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
		"title":            "Updated",
		"content":          "Updated",
		"expected_version": 1,
	}), cookie).Body.Close()

	// Second update with stale version 1 — should conflict
	resp := doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
		"title":            "Stale",
		"content":          "Stale",
		"expected_version": 1,
	}), cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for stale version, got %d", resp.StatusCode)
	}
}

func TestUpdatePage_CreatesRevision(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page 1", "Initial content")

	// 1 initial revision should exist
	revResp := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, cookie)
	var revs []map[string]any
	decodeBody(t, revResp, &revs)
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision after creation, got %d", len(revs))
	}

	// Update the page
	doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
		"title":            "Page 1",
		"content":          "Updated content",
		"expected_version": 1,
	}), cookie).Body.Close()

	// 2 revisions now
	revResp2 := doRequest(t, srv, http.MethodGet, "/api/pages/"+pageID+"/revisions", nil, cookie)
	var revs2 []map[string]any
	decodeBody(t, revResp2, &revs2)
	if len(revs2) != 2 {
		t.Fatalf("expected 2 revisions after update, got %d", len(revs2))
	}
}

func TestListPages_WrongWikiRejected(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	aliceCookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	bobCookie := loginUser(t, srv, "bob", "bob@example.com", "password456")

	wikiID := createWikiHelper(t, srv, aliceCookie, "Alice Wiki")

	// Bob tries to list pages in Alice's wiki
	resp := doRequest(t, srv, http.MethodGet, "/api/wikis/"+wikiID+"/pages", nil, bobCookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreatePage_Validation(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")

	resp := doRequest(t, srv, http.MethodPost, "/api/wikis/"+wikiID+"/pages", jsonBody(t, map[string]string{
		"title":   "",
		"content": "some content",
	}), cookie)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", resp.StatusCode)
	}
}

func TestOptimisticConcurrency_SequentialSuccess(t *testing.T) {
	db := testDB(t)
	srv, _ := testServer(t, db)
	defer srv.Close()

	cookie := loginUser(t, srv, "alice", "alice@example.com", "password123")
	wikiID := createWikiHelper(t, srv, cookie, "Wiki")
	pageID := createPageHelper(t, srv, cookie, wikiID, "Page", "v1")

	// 5 sequential updates, version should increment each time.
	for expectedVersion := 1; expectedVersion <= 5; expectedVersion++ {
		resp := doRequest(t, srv, http.MethodPut, "/api/pages/"+pageID, jsonBody(t, map[string]any{
			"title":            "Page",
			"content":          "content",
			"expected_version": expectedVersion,
		}), cookie)
		var updated map[string]any
		decodeBody(t, resp, &updated)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("update %d: expected 200, got %d", expectedVersion, resp.StatusCode)
		}
		newVersion := int(updated["version"].(float64))
		if newVersion != expectedVersion+1 {
			t.Errorf("update %d: expected version=%d, got %d", expectedVersion, expectedVersion+1, newVersion)
		}
	}
}
