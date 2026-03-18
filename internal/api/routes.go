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

	authMw := handlers.authMw

	// Health check
	r.Get("/health", handlers.HealthCheck)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Get("/auth/info", handlers.GetAuthInfo)
		r.Post("/auth/login", handlers.Login)

		// Auth-protected auth endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Post("/auth/logout", handlers.Logout)
			r.Get("/auth/me", handlers.GetMe)
		})

		// Public book endpoints (search, details, reader content, images, download)
		r.Get("/books", handlers.SearchBooks)
		r.Get("/books/{id}", handlers.GetBookByID)
		r.Get("/books/{id}/toc", handlers.GetBookTOC)
		r.Get("/books/{id}/content", handlers.GetBookContent)
		r.Get("/books/{id}/image/{name}", handlers.GetBookImage)

		// Reading position and history — require auth when enabled
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Get("/books/{id}/position", handlers.GetReadingPosition)
			r.Put("/books/{id}/position", handlers.SaveReadingPosition)
			r.Get("/reading-history", handlers.GetReadingHistory)
		})

		// TTS proxy endpoints (public — no auth needed)
		r.Get("/tts/status", handlers.GetTTSStatus)
		r.Get("/tts/voices", handlers.GetTTSVoices)
		r.Post("/tts/speech", handlers.SynthesizeSpeech)

		// Admin endpoints — require auth + admin role
		r.Group(func(r chi.Router) {
			r.Use(authMw.RequireAuth)
			r.Use(authMw.RequireAdmin)
			r.Post("/admin/reindex", handlers.ReindexLibrary)
			r.Get("/admin/users", handlers.ListUsers)
			r.Post("/admin/users", handlers.CreateUser)
			r.Delete("/admin/users/{id}", handlers.DeleteUser)
			r.Put("/admin/users/{id}/password", handlers.UpdateUserPassword)
		})
	})

	// Legacy admin endpoint (also protected)
	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Use(authMw.RequireAdmin)
		r.Post("/admin/reindex", handlers.ReindexLibrary)
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
