package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/wikicollab/backend/config"
	"github.com/wikicollab/backend/database"
	"github.com/wikicollab/backend/handlers"
	"github.com/wikicollab/backend/middleware"
	redisPkg "github.com/wikicollab/backend/redis"
	"github.com/wikicollab/backend/services"
	wsPkg "github.com/wikicollab/backend/websocket"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// PostgreSQL
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer db.Close()

	migrationsDir := migrationsPath()
	if err := database.RunMigrations(db, migrationsDir); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	// Redis
	redisClient, err := redisPkg.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis connection error: %v", err)
	}
	defer redisClient.Close()
	lockStore := redisPkg.NewLockStore(redisClient)

	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))

	// Services
	authSvc := services.NewAuthService(db)
	wikiSvc := services.NewWikiService(db)
	pageSvc := services.NewPageService(db)
	revSvc := services.NewRevisionService(db)

	// WebSocket hub
	hub := wsPkg.NewHub()

	// Handlers
	authHandler := handlers.NewAuthHandler(authSvc, store)
	wikiHandler := handlers.NewWikiHandler(wikiSvc)
	pageHandler := handlers.NewPageHandler(pageSvc, wikiSvc)
	revHandler := handlers.NewRevisionHandler(revSvc, pageSvc, wikiSvc)
	wsHandler := wsPkg.NewHandler(hub, lockStore, pageSvc, wikiSvc, store, db, cfg.LockTTL)

	// Auth middleware (for protected routes)
	requireAuth := middleware.RequireAuth(store, db)

	r := mux.NewRouter()

	// CORS middleware
	r.Use(corsMiddleware(cfg.CORSOrigin))

	// Health
	r.HandleFunc("/health", handlers.HealthHandler).Methods(http.MethodGet, http.MethodOptions)

	// WebSocket — no CORS middleware needed; auth is done inside the handler
	r.HandleFunc("/ws/pages/{id}", wsHandler.ServeWS)

	// Auth
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost, http.MethodOptions)
	api.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost, http.MethodOptions)
	api.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost, http.MethodOptions)

	// Protected routes
	protected := api.NewRoute().Subrouter()
	protected.Use(requireAuth)

	protected.HandleFunc("/me", authHandler.Me).Methods(http.MethodGet, http.MethodOptions)

	// Wiki routes
	protected.HandleFunc("/wikis", wikiHandler.ListWikis).Methods(http.MethodGet, http.MethodOptions)
	protected.HandleFunc("/wikis", wikiHandler.CreateWiki).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/wikis/{id}", wikiHandler.GetWiki).Methods(http.MethodGet, http.MethodOptions)
	protected.HandleFunc("/wikis/{id}/share", wikiHandler.ShareWiki).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/join", wikiHandler.JoinWiki).Methods(http.MethodPost, http.MethodOptions)

	// Page routes
	protected.HandleFunc("/wikis/{id}/pages", pageHandler.ListPages).Methods(http.MethodGet, http.MethodOptions)
	protected.HandleFunc("/wikis/{id}/pages", pageHandler.CreatePage).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/pages/{id}", pageHandler.GetPage).Methods(http.MethodGet, http.MethodOptions)
	protected.HandleFunc("/pages/{id}", pageHandler.UpdatePage).Methods(http.MethodPut, http.MethodOptions)

	// Revision routes
	protected.HandleFunc("/pages/{id}/revisions", revHandler.ListRevisions).Methods(http.MethodGet, http.MethodOptions)

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine so we can listen for shutdown signals.
	go func() {
		log.Printf("WikiCollab backend listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}

// corsMiddleware sets CORS headers, allowing the React frontend to call the API.
func corsMiddleware(allowedOrigin string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// migrationsPath returns the absolute path to the migrations directory.
// It resolves relative to the binary location so it works both locally and in Docker.
func migrationsPath() string {
	// Try environment variable override first (useful in Docker).
	if p := os.Getenv("MIGRATIONS_DIR"); p != "" {
		return p
	}

	// Walk up from the current file to find the repo root.
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// backend/main.go → go up one level to repo root, then migrations/
		repoRoot := filepath.Dir(filepath.Dir(filename))
		candidate := filepath.Join(repoRoot, "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback: relative to working directory (works when running from repo root).
	return filepath.Join("..", "migrations")
}
