package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/inpx"
	"github.com/piligrim/pushkinlib/internal/storage"
)

// setupTestHandlers creates a Handlers instance with an in-memory database for testing.
func setupTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := storage.NewRepository(db)

	// Insert a test book
	book := inpx.Book{
		ID:          "test-001",
		Title:       "Test Book Title",
		Authors:     []string{"Test Author"},
		Series:      "Test Series",
		SeriesNum:   1,
		Genre:       "fiction",
		Year:        2024,
		Language:    "ru",
		FileSize:    12345,
		ArchivePath: "test-archive",
		FileNum:     "001",
		Format:      "fb2",
		Date:        time.Now(),
		Rating:      5,
		Annotation:  "Test annotation text",
	}
	if err := repo.InsertBooks([]inpx.Book{book}); err != nil {
		t.Fatalf("failed to insert test book: %v", err)
	}

	return NewHandlers(repo, t.TempDir(), "")
}

// TestSearchBooks_LimitCapped verifies that limit parameter is capped at maxLimit (#11).
func TestSearchBooks_LimitCapped(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books?limit=999999", nil)
	w := httptest.NewRecorder()

	h.SearchBooks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result storage.BookList
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Limit > maxLimit {
		t.Errorf("expected limit <= %d, got %d", maxLimit, result.Limit)
	}
}

// TestSearchBooks_DefaultLimit verifies default limit when not specified.
func TestSearchBooks_DefaultLimit(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books", nil)
	w := httptest.NewRecorder()

	h.SearchBooks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result storage.BookList
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Limit != 30 {
		t.Errorf("expected default limit 30, got %d", result.Limit)
	}
}

// TestHealthCheck verifies the health endpoint returns valid JSON (#5).
func TestHealthCheck(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

// TestGetBookByID verifies book retrieval returns valid JSON (#5).
func TestGetBookByID(t *testing.T) {
	h := setupTestHandlers(t)

	// Use chi router context to inject URL param
	req := httptest.NewRequest("GET", "/api/v1/books/test-001", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetBookByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var book storage.Book
	if err := json.NewDecoder(w.Body).Decode(&book); err != nil {
		t.Fatalf("failed to decode book: %v", err)
	}

	if book.ID != "test-001" {
		t.Errorf("expected book ID test-001, got %s", book.ID)
	}
	if book.Title != "Test Book Title" {
		t.Errorf("expected title 'Test Book Title', got '%s'", book.Title)
	}
}

// TestGetBookByID_NotFound verifies 404 for missing book.
func TestGetBookByID_NotFound(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books/nonexistent", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetBookByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestDownloadBook_PathTraversal verifies path traversal protection (#13).
func TestDownloadBook_PathTraversal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db)

	// Insert a book with a malicious archive path
	book := inpx.Book{
		ID:          "evil-001",
		Title:       "Evil Book",
		Authors:     []string{"Hacker"},
		Genre:       "exploit",
		Year:        2024,
		Language:    "en",
		FileSize:    100,
		ArchivePath: "../../etc/passwd",
		FileNum:     "001",
		Format:      "fb2",
		Date:        time.Now(),
	}
	if err := repo.InsertBooks([]inpx.Book{book}); err != nil {
		t.Fatalf("failed to insert book: %v", err)
	}

	booksDir := t.TempDir()
	h := NewHandlers(repo, booksDir, "")

	req := httptest.NewRequest("GET", "/download/evil-001", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "evil-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.DownloadBook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDownloadBook_ArchiveNotFound verifies 404 when archive doesn't exist (#12).
func TestDownloadBook_ArchiveNotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db)

	book := inpx.Book{
		ID:          "missing-001",
		Title:       "Missing Archive Book",
		Authors:     []string{"Author"},
		Genre:       "fiction",
		Year:        2024,
		Language:    "en",
		FileSize:    100,
		ArchivePath: "nonexistent-archive",
		FileNum:     "001",
		Format:      "fb2",
		Date:        time.Now(),
	}
	if err := repo.InsertBooks([]inpx.Book{book}); err != nil {
		t.Fatalf("failed to insert book: %v", err)
	}

	booksDir := t.TempDir()
	h := NewHandlers(repo, booksDir, "")

	req := httptest.NewRequest("GET", "/download/missing-001", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "missing-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.DownloadBook(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing archive, got %d: %s", w.Code, w.Body.String())
	}
}

// TestReindexLibrary_ConcurrentProtection verifies mutex prevents concurrent reindex (#9).
func TestReindexLibrary_ConcurrentProtection(t *testing.T) {
	h := setupTestHandlers(t)

	// Lock the mutex manually to simulate an in-progress reindex
	h.reindexMu.Lock()

	req := httptest.NewRequest("POST", "/admin/reindex", nil)
	w := httptest.NewRecorder()

	h.ReindexLibrary(w, req)

	h.reindexMu.Unlock()

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when reindex is already running, got %d: %s", w.Code, w.Body.String())
	}
}
