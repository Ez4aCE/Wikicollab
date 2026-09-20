package handlers

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/wikicollab/backend/middleware"
	"github.com/wikicollab/backend/services"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService *services.AuthService
	store       sessions.Store
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(authService *services.AuthService, store sessions.Store) *AuthHandler {
	return &AuthHandler{authService: authService, store: store}
}

// Register handles POST /api/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	user, err := h.authService.Register(r.Context(), services.RegisterInput{
		Username: input.Username,
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// Login handles POST /api/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	user, err := h.authService.Login(r.Context(), services.LoginInput{
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	session, err := h.store.Get(r, "wikicollab-session")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	session.Values["user_id"] = user.ID
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Set Secure: true in production (HTTPS)
	}

	if err := h.store.Save(r, w, session); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// Logout handles POST /api/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "wikicollab-session")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Expire the session immediately.
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	delete(session.Values, "user_id")

	if err := h.store.Save(r, w, session); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Me handles GET /api/me.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
