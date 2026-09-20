package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wikicollab/backend/middleware"
	"github.com/wikicollab/backend/models"
	"github.com/wikicollab/backend/services"
)

// PageHandler handles page endpoints.
type PageHandler struct {
	pageService *services.PageService
	wikiService *services.WikiService
}

// NewPageHandler creates a PageHandler.
func NewPageHandler(pageService *services.PageService, wikiService *services.WikiService) *PageHandler {
	return &PageHandler{pageService: pageService, wikiService: wikiService}
}

// ListPages handles GET /api/wikis/:id/pages.
func (h *PageHandler) ListPages(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	wikiID := mux.Vars(r)["id"]

	if err := h.wikiService.AssertAccess(r.Context(), wikiID, user.ID); err != nil {
		handleServiceError(w, err)
		return
	}

	pages, err := h.pageService.ListPages(r.Context(), wikiID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if pages == nil {
		pages = []models.Page{}
	}

	writeJSON(w, http.StatusOK, pages)
}

// CreatePage handles POST /api/wikis/:id/pages.
func (h *PageHandler) CreatePage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	wikiID := mux.Vars(r)["id"]

	if err := h.wikiService.AssertAccess(r.Context(), wikiID, user.ID); err != nil {
		handleServiceError(w, err)
		return
	}

	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	page, err := h.pageService.CreatePage(r.Context(), services.CreatePageInput{
		WikiID:    wikiID,
		Title:     input.Title,
		Content:   input.Content,
		CreatedBy: user.ID,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, page)
}

// GetPage handles GET /api/pages/:id.
func (h *PageHandler) GetPage(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, page)
}

// UpdatePage handles PUT /api/pages/:id.
func (h *PageHandler) UpdatePage(w http.ResponseWriter, r *http.Request) {
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

	var input struct {
		Title           string `json:"title"`
		Content         string `json:"content"`
		ExpectedVersion int    `json:"expected_version"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	if input.ExpectedVersion <= 0 {
		writeError(w, http.StatusBadRequest, "expected_version is required")
		return
	}

	updated, err := h.pageService.UpdatePage(r.Context(), services.UpdatePageInput{
		PageID:          pageID,
		Title:           input.Title,
		Content:         input.Content,
		ExpectedVersion: input.ExpectedVersion,
		UpdatedBy:       user.ID,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
