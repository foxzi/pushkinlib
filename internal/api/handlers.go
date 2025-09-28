package api

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/indexer"
	"github.com/piligrim/pushkinlib/internal/storage"
)

// Handlers contains all API handlers
type Handlers struct {
	repo     *storage.Repository
	booksDir string
	inpxPath string
}

// NewHandlers creates new API handlers
func NewHandlers(repo *storage.Repository, booksDir, inpxPath string) *Handlers {
	return &Handlers{
		repo:     repo,
		booksDir: booksDir,
		inpxPath: inpxPath,
	}
}

// ReindexLibrary clears database and re-imports data from INPX
func (h *Handlers) ReindexLibrary(w http.ResponseWriter, r *http.Request) {
	result, err := indexer.ReindexFromINPX(h.repo, h.inpxPath)
	if err != nil {
		switch {
		case errors.Is(err, indexer.ErrINPXPathEmpty):
			http.Error(w, "INPX path is not configured", http.StatusInternalServerError)
		case errors.Is(err, indexer.ErrINPXNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	collectionName := ""
	collectionVersion := ""
	if result.Collection != nil {
		collectionName = result.Collection.Name
		collectionVersion = result.Collection.Version
	}

	response := map[string]interface{}{
		"status":             "ok",
		"imported":           result.Imported,
		"collection":         collectionName,
		"version":            collectionVersion,
		"duration_ms":        result.Duration.Milliseconds(),
		"parse_duration_ms":  result.ParseDuration.Milliseconds(),
		"clear_duration_ms":  result.ClearDuration.Milliseconds(),
		"insert_duration_ms": result.InsertDuration.Milliseconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SearchBooks handles book search requests
func (h *Handlers) SearchBooks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := storage.BookFilter{
		Query:     query.Get("q"),
		Limit:     parseInt(query.Get("limit"), 30),
		Offset:    parseInt(query.Get("offset"), 0),
		SortBy:    query.Get("sort_by"),
		SortOrder: query.Get("sort_order"),
		YearFrom:  parseInt(query.Get("year_from"), 0),
		YearTo:    parseInt(query.Get("year_to"), 0),
	}

	// Parse array parameters
	if authors := query["authors"]; len(authors) > 0 {
		filter.Authors = authors
	}
	if series := query["series"]; len(series) > 0 {
		filter.Series = series
	}
	if genres := query["genres"]; len(genres) > 0 {
		filter.Genres = genres
	}
	if languages := query["languages"]; len(languages) > 0 {
		filter.Languages = languages
	}
	if formats := query["formats"]; len(formats) > 0 {
		filter.Formats = formats
	}

	result, err := h.repo.SearchBooks(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetBookByID handles getting a single book by ID
func (h *Handlers) GetBookByID(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	book, err := h.repo.GetBookByID(bookID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

// DownloadBook handles book download requests
func (h *Handlers) DownloadBook(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	// Get book info from database
	book, err := h.repo.GetBookByID(bookID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// Build path to archive
	archivePath := filepath.Join(h.booksDir, book.ArchivePath+".zip")

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		http.Error(w, "Book archive not found", http.StatusNotFound)
		return
	}

	// Open archive
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		http.Error(w, "Failed to open archive", http.StatusInternalServerError)
		return
	}
	defer archive.Close()

	// Find book file in archive
	var bookFile *zip.File
	expectedFileName := book.FileNum + "." + book.Format

	for _, file := range archive.File {
		if file.Name == expectedFileName {
			bookFile = file
			break
		}
	}

	if bookFile == nil {
		http.Error(w, "Book file not found in archive", http.StatusNotFound)
		return
	}

	// Open book file
	rc, err := bookFile.Open()
	if err != nil {
		http.Error(w, "Failed to open book file", http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	// Set headers for download
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(book.Title), book.Format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", getContentType(book.Format))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", bookFile.UncompressedSize64))

	// Stream file to response
	_, err = io.Copy(w, rc)
	if err != nil {
		// Can't send error response after starting to stream
		return
	}
}

// HealthCheck handles health check requests
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "ok",
		"service": "pushkinlib",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sanitizeFilename removes invalid characters from filename
func sanitizeFilename(filename string) string {
	// Replace invalid characters
	replacements := map[rune]string{
		'/':  "_",
		'\\': "_",
		':':  "_",
		'*':  "_",
		'?':  "_",
		'"':  "_",
		'<':  "_",
		'>':  "_",
		'|':  "_",
	}

	result := make([]rune, 0, len(filename))
	for _, r := range filename {
		if replacement, exists := replacements[r]; exists {
			result = append(result, []rune(replacement)...)
		} else {
			result = append(result, r)
		}
	}

	// Limit length
	if len(result) > 100 {
		result = result[:100]
	}

	return string(result)
}

// getContentType returns MIME type for file format
func getContentType(format string) string {
	switch format {
	case "fb2":
		return "application/x-fictionbook+xml"
	case "epub":
		return "application/epub+zip"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// parseInt helper function to parse integer from string with default
func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return defaultValue
}
