package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/wikicollab/backend/models"
)

type contextKey string

const userContextKey contextKey = "user"

// RequireAuth is middleware that enforces authentication.
// It reads the session cookie, looks up the user, and stores the user in the request context.
// Unauthenticated requests receive 401 Unauthorized.
func RequireAuth(store sessions.Store, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Browsers send an OPTIONS preflight before POST/PUT/DELETE.
			// Cookies are not included in preflight requests, so let them pass
			// through to be handled by the CORS middleware.
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			user, err := GetSessionUser(store, db, r)
			if err != nil || user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is present.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userContextKey).(*models.User)
	return user
}

// GetSessionUser reads the user ID from the session and fetches the user from the database.
// Exported so the WebSocket handler can reuse the same auth mechanism without duplication.
func GetSessionUser(store sessions.Store, db *sql.DB, r *http.Request) (*models.User, error) {
	session, err := store.Get(r, "wikicollab-session")
	if err != nil {
		return nil, err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil, nil
	}

	var user models.User
	err = db.QueryRowContext(r.Context(),
		`SELECT id, username, email, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}
