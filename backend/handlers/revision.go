package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wikicollab/backend/middleware"
	"github.com/wikicollab/backend/models"
	"github.com/wikicollab/backend/services"
)

// RevisionHandler handles revision endpoints.
type RevisionHandler struct {
	revisionService *services.RevisionService
	pageService     *services.PageService
	wikiService     *services.WikiService
}

// NewRevisionHandler creates a RevisionHandler.
func NewRevisionHandler(
	revisionService *services.RevisionService,
	pageService *services.PageService,
	wikiService *services.WikiService,
) *RevisionHandler {
	return &RevisionHandler{
		revisionService: revisionService,
		pageService:     pageService,
		wikiService:     wikiService,
	}
}

// ListRevisions handles GET /api/pages/:id/revisions.
func (h *RevisionHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	pageID := mux.Vars(r)["id"]

	page, err := h.pageService.GetPage(r.Context(), pageID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.wikiService.AssertAccess(r.Context(), page.WikiID, user.ID); err != nil {
		handleServiceError(w, err)
		return
	}

	revisions, err := h.revisionService.ListRevisions(r.Context(), pageID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if revisions == nil {
		revisions = []models.Revision{}
	}

	writeJSON(w, http.StatusOK, revisions)
}
