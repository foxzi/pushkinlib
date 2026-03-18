package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/auth"
	"github.com/piligrim/pushkinlib/internal/opds"
)

// SetupOPDSRoutes configures OPDS routes with optional BasicAuth protection.
// When auth is enabled, OPDS clients must authenticate via HTTP Basic Auth.
func SetupOPDSRoutes(r chi.Router, opdsHandler *opds.Handler, authMw *auth.Middleware) {
	r.Route("/opds", func(r chi.Router) {
		// Apply BasicAuth middleware for OPDS clients (e-readers)
		r.Use(authMw.RequireBasicAuth)

		// Root catalog
		r.Get("/", opdsHandler.Root)

		// Search
		r.Get("/search", opdsHandler.SearchBooks)
		r.Get("/opensearch.xml", opdsHandler.OpenSearch)

		// Navigation catalogs
		r.Get("/authors", opdsHandler.Authors)
		r.Get("/series", opdsHandler.Series)
		r.Get("/genres", opdsHandler.Genres)

		// Books
		r.Get("/books/new", opdsHandler.NewBooks)
		r.Get("/authors/{id}", opdsHandler.BooksByAuthor)
		r.Get("/series/{id}", opdsHandler.BooksBySeries)
		r.Get("/genres/{id}", opdsHandler.BooksByGenre)
	})
}
