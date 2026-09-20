package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/wikicollab/backend/services"
)

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleServiceError maps service-layer sentinel errors to HTTP status codes.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, services.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, services.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version conflict — refresh the page and try again")
	case errors.Is(err, services.ErrDuplicateUser):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, services.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, services.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// decodeJSON decodes the request body into v and returns a 400 on failure.
// Returns false if decoding failed (caller should return immediately).
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
