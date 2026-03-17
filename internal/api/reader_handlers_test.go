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

func TestSaveReadingPosition_WithTotalSections(t *testing.T) {
	h := setupTestHandlers(t)

	// Save position with total_sections
	body := bytes.NewBufferString(`{"section":3,"progress":0.15,"total_sections":20}`)
	req := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify position was saved with total_sections
	req2 := httptest.NewRequest("GET", "/api/v1/books/test-001/position", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "test-001")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()
	h.GetReadingPosition(w2, req2)

	var pos map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &pos); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if pos["total_sections"].(float64) != 20 {
		t.Errorf("total_sections = %v, want 20", pos["total_sections"])
	}
	if pos["status"] != "reading" {
		t.Errorf("status = %v, want reading", pos["status"])
	}
}

func TestSaveReadingPosition_AutoFinished(t *testing.T) {
	h := setupTestHandlers(t)

	// Save position at last section — should auto-set to "finished"
	body := bytes.NewBufferString(`{"section":19,"progress":0.95,"total_sections":20}`)
	req := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify status is "finished"
	req2 := httptest.NewRequest("GET", "/api/v1/books/test-001/position", nil)
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("id", "test-001")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()
	h.GetReadingPosition(w2, req2)

	var pos map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &pos); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if pos["status"] != "finished" {
		t.Errorf("status = %v, want finished (section 19 of 20 is last)", pos["status"])
	}
}

func TestGetReadingHistory_Empty(t *testing.T) {
	h := setupTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/reading-history", nil)
	w := httptest.NewRecorder()
	h.GetReadingHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	items := result["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
	if result["total"].(float64) != 0 {
		t.Errorf("expected total=0, got %v", result["total"])
	}
}

func TestGetReadingHistory_WithItems(t *testing.T) {
	h := setupTestHandlers(t)

	// Save a reading position first
	body := bytes.NewBufferString(`{"section":5,"progress":0.25,"total_sections":20}`)
	req := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save position: expected 200, got %d", w.Code)
	}

	// Get all history
	req2 := httptest.NewRequest("GET", "/api/v1/reading-history", nil)
	w2 := httptest.NewRecorder()
	h.GetReadingHistory(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if result["total"].(float64) != 1 {
		t.Errorf("total = %v, want 1", result["total"])
	}

	items := result["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0].(map[string]interface{})
	if item["book_id"] != "test-001" {
		t.Errorf("book_id = %v, want test-001", item["book_id"])
	}
	if item["status"] != "reading" {
		t.Errorf("status = %v, want reading", item["status"])
	}
	if item["progress_percent"].(float64) != 30 {
		// section 5, total 20, (5+1)*100/20 = 30
		t.Errorf("progress_percent = %v, want 30", item["progress_percent"])
	}
}

func TestGetReadingHistory_FilterByStatus(t *testing.T) {
	h := setupTestHandlers(t)

	// Save a "reading" position
	body := bytes.NewBufferString(`{"section":5,"progress":0.25,"total_sections":20}`)
	req := httptest.NewRequest("PUT", "/api/v1/books/test-001/position", body)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.SaveReadingPosition(w, req)

	// Filter by "finished" — should get 0
	req2 := httptest.NewRequest("GET", "/api/v1/reading-history?status=finished", nil)
	w2 := httptest.NewRecorder()
	h.GetReadingHistory(w2, req2)

	var result map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &result)
	if result["total"].(float64) != 0 {
		t.Errorf("finished filter: total = %v, want 0", result["total"])
	}

	// Filter by "reading" — should get 1
	req3 := httptest.NewRequest("GET", "/api/v1/reading-history?status=reading", nil)
	w3 := httptest.NewRecorder()
	h.GetReadingHistory(w3, req3)

	var result2 map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &result2)
	if result2["total"].(float64) != 1 {
		t.Errorf("reading filter: total = %v, want 1", result2["total"])
	}
}
