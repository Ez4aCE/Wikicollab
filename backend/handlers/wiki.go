package handlers

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/wikicollab/backend/middleware"
	"github.com/wikicollab/backend/models"
	"github.com/wikicollab/backend/services"
)

// WikiHandler handles wiki endpoints.
type WikiHandler struct {
	wikiService *services.WikiService
}

// NewWikiHandler creates a WikiHandler.
func NewWikiHandler(wikiService *services.WikiService) *WikiHandler {
	return &WikiHandler{wikiService: wikiService}
}

// ListWikis handles GET /api/wikis.
// Returns all wikis owned by or shared with the authenticated user.
func (h *WikiHandler) ListWikis(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	wikis, err := h.wikiService.ListWikis(r.Context(), user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Return empty array instead of null when there are no wikis.
	if wikis == nil {
		wikis = []models.Wiki{}
	}

	writeJSON(w, http.StatusOK, wikis)
}

// CreateWiki handles POST /api/wikis.
func (h *WikiHandler) CreateWiki(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	wiki, err := h.wikiService.CreateWiki(r.Context(), services.CreateWikiInput{
		Name:    input.Name,
		OwnerID: user.ID,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, wiki)
}

// GetWiki handles GET /api/wikis/:id.
// Accessible to owners and members.
func (h *WikiHandler) GetWiki(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	wikiID := mux.Vars(r)["id"]

	wiki, err := h.wikiService.GetWiki(r.Context(), wikiID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.wikiService.AssertAccess(r.Context(), wikiID, user.ID); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, wiki)
}

// ShareWiki handles POST /api/wikis/:id/share.
// Owner-only; returns (or creates) a 6-char invite code.
func (h *WikiHandler) ShareWiki(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	wikiID := mux.Vars(r)["id"]

	code, err := h.wikiService.GenerateShareCode(r.Context(), wikiID, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"code": code})
}

// JoinWiki handles POST /api/join.
// Any authenticated user may join a wiki by providing its share code.
func (h *WikiHandler) JoinWiki(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	var input struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	wiki, err := h.wikiService.JoinByCode(r.Context(), code, user.ID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, wiki)
}
