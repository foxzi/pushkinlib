package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGetBookTOC_NotFound(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books/nonexistent/toc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetBookTOC(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetBookTOC_NoArchive(t *testing.T) {
	h := setupTestHandlers(t)

	// test-001 exists in DB but has no real archive file, so parsing should fail
	req := httptest.NewRequest("GET", "/api/v1/books/test-001/toc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetBookTOC(w, req)

	// Should return 500 because the archive doesn't exist on disk
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetBookContent_NotFound(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books/nonexistent/content?section=0", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetBookContent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetBookImage_NotFound(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books/nonexistent/image/test.jpg", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	rctx.URLParams.Add("name", "test.jpg")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetBookImage(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetBookImage_EmptyParams(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/books//image/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	rctx.URLParams.Add("name", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetBookImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReadingPosition_SaveAndGet(t *testing.T) {
	h := setupTestHandlers(t)

	// Get position — should return default (section 0)
	req := httptest.NewRequest("GET", "/api/v1/books/test-001/position", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetReadingPosition(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET position: expected 200, got %d", w.Code)
	}

	var pos map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &pos); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if pos["book_id"] != "test-001" {
		t.Errorf("book_id = %v, want test-001", pos["book_id"])
	}
	if pos["section"].(float64) != 0 {
		t.Errorf("default section = %v, want 0", pos["section"])
	}

	// Save position
	body := bytes.NewBufferString(`{"section":7,"progress":0.35}`)
	req2 := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	req2.Header.Set("Content-Type", "application/json")
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "test-001")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()
	h.SaveReadingPosition(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("PUT position: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Get position again — should return saved values
	req3 := httptest.NewRequest("GET", "/api/v1/books/test-001/position", nil)
	rctx3 := chi.NewRouteContext()
	rctx3.URLParams.Add("id", "test-001")
	req3 = req3.WithContext(context.WithValue(req3.Context(), chi.RouteCtxKey, rctx3))

	w3 := httptest.NewRecorder()
	h.GetReadingPosition(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("GET position after save: expected 200, got %d", w3.Code)
	}

	var pos2 map[string]interface{}
	if err := json.Unmarshal(w3.Body.Bytes(), &pos2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if pos2["section"].(float64) != 7 {
		t.Errorf("saved section = %v, want 7", pos2["section"])
	}
	if pos2["progress"].(float64) != 0.35 {
		t.Errorf("saved progress = %v, want 0.35", pos2["progress"])
	}
}

func TestSaveReadingPosition_InvalidBody(t *testing.T) {
	h := setupTestHandlers(t)

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSaveReadingPosition_EmptyBookID(t *testing.T) {
	h := setupTestHandlers(t)

	body := bytes.NewBufferString(`{"section":1}`)
	req := httptest.NewRequest("PUT", "/api/v1/books//position", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
