package api

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// SetupRoutes configures all API routes
func SetupRoutes(handlers *Handlers) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// CORS for SPA
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Health check
	r.Get("/health", handlers.HealthCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/books", handlers.SearchBooks)
		r.Get("/books/{id}", handlers.GetBookByID)
	})

	// Serve static files
	staticDir := "./web/static"
	if absPath, err := filepath.Abs(staticDir); err == nil {
		staticDir = absPath
	}

	fileServer := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	// Download routes (must be before wildcard route)
	r.Get("/download/{id}", handlers.DownloadBook)

	// Serve SPA (index.html for all non-API routes)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	return r
}