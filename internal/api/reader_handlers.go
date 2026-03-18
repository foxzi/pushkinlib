package api

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/auth"
	"github.com/piligrim/pushkinlib/internal/reader"
	"github.com/piligrim/pushkinlib/internal/storage"
)

// openBookFromArchive locates and opens the FB2 file for a given book.
// Returns the opened reader, a cleanup function, and any error.
func (h *Handlers) openBookFromArchive(book *storage.Book) (io.ReadCloser, func(), error) {
	archiveName := book.ArchivePath
	if archiveName == "" {
		return nil, nil, fmt.Errorf("book archive path is empty")
	}
	if !strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		archiveName += ".zip"
	}
	archivePath := filepath.Join(h.booksDir, archiveName)

	// Path traversal check
	cleanArchivePath := filepath.Clean(archivePath)
	cleanBooksDir := filepath.Clean(h.booksDir)
	if !strings.HasPrefix(cleanArchivePath, cleanBooksDir+string(os.PathSeparator)) && cleanArchivePath != cleanBooksDir {
		return nil, nil, fmt.Errorf("invalid archive path")
	}

	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open archive %s: %w", archivePath, err)
	}

	format := strings.ToLower(book.Format)
	if format == "" {
		format = "fb2"
	}
	expectedFileName := book.ID + "." + format

	// Also try zero-padded filename (e.g., "000024.fb2" for book ID "24")
	var paddedFileName string
	if _, err := fmt.Sscanf(book.ID, "%d", new(int)); err == nil {
		paddedFileName = fmt.Sprintf("%06s", book.ID) + "." + format
	}

	var bookFile *zip.File
	for _, file := range archive.File {
		if strings.EqualFold(file.Name, expectedFileName) {
			bookFile = file
			break
		}
		if paddedFileName != "" && strings.EqualFold(file.Name, paddedFileName) {
			bookFile = file
			break
		}
	}

	if bookFile == nil {
		archive.Close()
		return nil, nil, fmt.Errorf("file %s not found in archive", expectedFileName)
	}

	rc, err := bookFile.Open()
	if err != nil {
		archive.Close()
		return nil, nil, fmt.Errorf("open file in archive: %w", err)
	}

	cleanup := func() {
		rc.Close()
		archive.Close()
	}

	return rc, cleanup, nil
}

// parseBookFB2 fetches and parses a book's FB2 content.
func (h *Handlers) parseBookFB2(book *storage.Book) (*reader.FB2Book, error) {
	rc, cleanup, err := h.openBookFromArchive(book)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	fb2Book, err := reader.ParseFB2(rc)
	if err != nil {
		return nil, fmt.Errorf("parse FB2: %w", err)
	}

	return fb2Book, nil
}

// GetBookTOC returns the table of contents for a book.
// GET /api/v1/books/{id}/toc
func (h *Handlers) GetBookTOC(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	book, err := h.repo.GetBookByID(bookID)
	if err != nil {
		log.Printf("GetBookTOC: book_id=%s database error: %v", bookID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	fb2Book, err := h.parseBookFB2(book)
	if err != nil {
		log.Printf("GetBookTOC: book_id=%s parse error: %v", bookID, err)
		http.Error(w, "Failed to parse book", http.StatusInternalServerError)
		return
	}

	flat := reader.FlattenSections(fb2Book)
	toc := reader.BuildTOC(flat)

	response := map[string]interface{}{
		"book_id":        bookID,
		"title":          book.Title,
		"total_sections": len(flat),
		"toc":            toc,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("GetBookTOC: failed to encode response: %v", err)
	}
}

// GetBookContent returns HTML content for a specific section of a book.
// GET /api/v1/books/{id}/content?section=0
func (h *Handlers) GetBookContent(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	sectionIdx := parseInt(r.URL.Query().Get("section"), 0)

	book, err := h.repo.GetBookByID(bookID)
	if err != nil {
		log.Printf("GetBookContent: book_id=%s database error: %v", bookID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	fb2Book, err := h.parseBookFB2(book)
	if err != nil {
		log.Printf("GetBookContent: book_id=%s parse error: %v", bookID, err)
		http.Error(w, "Failed to parse book", http.StatusInternalServerError)
		return
	}

	flat := reader.FlattenSections(fb2Book)

	if sectionIdx < 0 || sectionIdx >= len(flat) {
		http.Error(w, "Section index out of range", http.StatusBadRequest)
		return
	}

	sec := flat[sectionIdx]
	htmlContent := reader.SectionToHTML(sec.Section, bookID)

	response := map[string]interface{}{
		"book_id":        bookID,
		"section":        sectionIdx,
		"title":          sec.Title,
		"level":          sec.Level,
		"body_name":      sec.BodyName,
		"total_sections": len(flat),
		"has_prev":       sectionIdx > 0,
		"has_next":       sectionIdx < len(flat)-1,
		"html":           htmlContent,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("GetBookContent: failed to encode response: %v", err)
	}
}

// GetBookImage serves an embedded image from an FB2 book.
// GET /api/v1/books/{id}/image/{name}
func (h *Handlers) GetBookImage(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	imageName := chi.URLParam(r, "name")

	if bookID == "" || imageName == "" {
		http.Error(w, "Book ID and image name are required", http.StatusBadRequest)
		return
	}

	book, err := h.repo.GetBookByID(bookID)
	if err != nil {
		log.Printf("GetBookImage: book_id=%s database error: %v", bookID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	fb2Book, err := h.parseBookFB2(book)
	if err != nil {
		log.Printf("GetBookImage: book_id=%s parse error: %v", bookID, err)
		http.Error(w, "Failed to parse book", http.StatusInternalServerError)
		return
	}

	// Find the binary with matching ID
	var found *reader.FB2Binary
	for i := range fb2Book.Binaries {
		if fb2Book.Binaries[i].ID == imageName {
			found = &fb2Book.Binaries[i]
			break
		}
	}

	if found == nil {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(found.Data)
	if err != nil {
		log.Printf("GetBookImage: book_id=%s image=%s decode error: %v", bookID, imageName, err)
		http.Error(w, "Failed to decode image", http.StatusInternalServerError)
		return
	}

	// Set content type and cache headers
	contentType := found.ContentType
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24h

	if _, err := w.Write(data); err != nil {
		log.Printf("GetBookImage: book_id=%s image=%s write error: %v", bookID, imageName, err)
	}
}

// GetReadingPosition returns the saved reading position for a book.
// GET /api/v1/books/{id}/position
func (h *Handlers) GetReadingPosition(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	pos, err := h.repo.GetReadingPosition(userID, bookID)
	if err != nil {
		log.Printf("GetReadingPosition: book_id=%s error: %v", bookID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if pos == nil {
		// No saved position — return default
		pos = &storage.ReadingPosition{
			BookID:  bookID,
			Section: 0,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pos); err != nil {
		log.Printf("GetReadingPosition: failed to encode response: %v", err)
	}
}

// SaveReadingPosition saves the reading position for a book.
// PUT /api/v1/books/{id}/position
func (h *Handlers) SaveReadingPosition(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Book ID is required", http.StatusBadRequest)
		return
	}

	var pos storage.ReadingPosition
	if err := json.NewDecoder(r.Body).Decode(&pos); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	pos.BookID = bookID
	pos.UserID = auth.UserIDFromContext(r.Context())

	if err := h.repo.SaveReadingPosition(&pos); err != nil {
		log.Printf("SaveReadingPosition: book_id=%s error: %v", bookID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("SaveReadingPosition: failed to encode response: %v", err)
	}
}

// GetReadingHistory returns the reading history (books with progress).
// GET /api/v1/reading-history?status=reading&limit=30&offset=0
func (h *Handlers) GetReadingHistory(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // "", "reading", "finished"
	limit := parseInt(r.URL.Query().Get("limit"), 30)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	userID := auth.UserIDFromContext(r.Context())
	items, total, err := h.repo.GetReadingHistory(userID, status, limit, offset)
	if err != nil {
		log.Printf("GetReadingHistory: error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []storage.ReadingHistoryItem{}
	}

	response := map[string]interface{}{
		"items":    items,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": offset+limit < total,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("GetReadingHistory: failed to encode response: %v", err)
	}
}
