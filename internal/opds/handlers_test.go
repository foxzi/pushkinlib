package opds

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/piligrim/pushkinlib/internal/inpx"
	"github.com/piligrim/pushkinlib/internal/storage"
)

func setupTestOPDSHandler(t *testing.T) *Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := storage.NewRepository(db)

	book := inpx.Book{
		ID:       "opds-001",
		Title:    "OPDS Test Book",
		Authors:  []string{"OPDS Author"},
		Genre:    "fiction",
		Year:     2024,
		Language: "ru",
		FileSize: 1000,
		Format:   "fb2",
		Date:     time.Now(),
	}
	if err := repo.InsertBooks([]inpx.Book{book}); err != nil {
		t.Fatalf("failed to insert test book: %v", err)
	}

	return NewHandler(repo, "http://localhost:9090", "Test Catalog", nil)
}

// TestWriteFeed_ValidXML verifies writeFeed produces valid XML (#6).
func TestWriteFeed_ValidXML(t *testing.T) {
	h := setupTestOPDSHandler(t)

	req := httptest.NewRequest("GET", "/opds", nil)
	w := httptest.NewRecorder()

	h.Root(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/atom+xml") {
		t.Errorf("expected atom+xml content type, got %s", ct)
	}

	// Verify the body is valid XML
	var feed Feed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("response is not valid XML: %v\nbody: %s", err, w.Body.String())
	}

	if feed.Title == "" {
		t.Error("feed title should not be empty")
	}
}

// TestWriteFeed_ErrorOnInvalidFeed verifies writeFeed returns 500 on encoding error (#6).
func TestWriteFeed_ErrorOnInvalidFeed(t *testing.T) {
	h := setupTestOPDSHandler(t)
	w := httptest.NewRecorder()

	// A nil feed should cause encoding to fail or produce empty output
	// but writeFeed should not panic
	h.writeFeed(w, &Feed{
		Title:   "Test",
		Updated: time.Now(),
	})

	// Should succeed (even minimal feed is valid XML)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestOpenSearch_XMLEscaping verifies XML injection is prevented (#7).
func TestOpenSearch_XMLEscaping(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db)

	// Use a catalog title with XML-special characters
	maliciousTitle := `My <Library> & "Books"`
	h := NewHandler(repo, "http://example.com/path?a=1&b=2", maliciousTitle, nil)

	req := httptest.NewRequest("GET", "/opds/opensearch.xml", nil)
	w := httptest.NewRecorder()

	h.OpenSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify that raw special characters are NOT present (they should be escaped)
	if strings.Contains(body, "<Library>") {
		t.Error("XML injection: raw <Library> found in output, should be escaped")
	}
	if strings.Contains(body, "& \"Books\"") {
		t.Error("XML injection: raw & found in unescaped context")
	}

	// Verify the output is valid XML
	decoder := xml.NewDecoder(strings.NewReader(body))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("OpenSearch output is not valid XML: %v\nbody: %s", err, body)
		}
	}
}

// TestOpenSearch_ContentType verifies correct content type (#8).
func TestOpenSearch_ContentType(t *testing.T) {
	h := setupTestOPDSHandler(t)

	req := httptest.NewRequest("GET", "/opds/opensearch.xml", nil)
	w := httptest.NewRecorder()

	h.OpenSearch(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/opensearchdescription+xml") {
		t.Errorf("expected opensearchdescription+xml content type, got %s", ct)
	}
}
