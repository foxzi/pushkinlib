package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/opds"
)

// SetupOPDSRoutes configures OPDS routes
func SetupOPDSRoutes(r chi.Router, opdsHandler *opds.Handler) {
	r.Route("/opds", func(r chi.Router) {
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
	})
}