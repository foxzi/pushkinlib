package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/piligrim/pushkinlib/internal/storage"
)

func setupTestRepo(t *testing.T) *storage.Repository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return storage.NewRepository(db)
}

// TestRequireAuth_Disabled verifies that when auth is disabled, requests pass through.
func TestRequireAuth_Disabled(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, false)

	called := false
	handler := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called when auth is disabled")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestRequireAuth_NoCookie returns 401 when no cookie is present.
func TestRequireAuth_NoCookie(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	handler := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestRequireAuth_InvalidSession returns 401 for invalid session token.
func TestRequireAuth_InvalidSession(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	handler := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "pushkinlib_session", Value: "invalid-token"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestRequireAuth_ValidSession passes through with user in context.
func TestRequireAuth_ValidSession(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	// Create a user and session
	user, err := repo.CreateUser("testuser", "password123", "Test User", false)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	session, err := repo.CreateSession(user.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	var ctxUser *storage.User
	handler := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "pushkinlib_session", Value: session.Token})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxUser == nil {
		t.Fatal("user not found in context")
	}
	if ctxUser.Username != "testuser" {
		t.Errorf("username = %s, want testuser", ctxUser.Username)
	}
}

// TestOptionalAuth_NoSession passes through without user.
func TestOptionalAuth_NoSession(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	var ctxUser *storage.User
	handler := mw.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxUser != nil {
		t.Error("expected nil user in context for optional auth with no session")
	}
}

// TestRequireAdmin_Disabled passes through when auth disabled.
func TestRequireAdmin_Disabled(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, false)

	called := false
	handler := mw.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called when auth is disabled")
	}
}

// TestRequireAdmin_NonAdmin returns 403 for non-admin user.
func TestRequireAdmin_NonAdmin(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	user, _ := repo.CreateUser("regular", "pass", "Regular", false)

	handler := mw.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-admin")
	}))

	// Inject user into context manually (simulating RequireAuth already ran)
	ctx := context.WithValue(context.Background(), userContextKey, user)
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestRequireAdmin_Admin passes through for admin user.
func TestRequireAdmin_Admin(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	admin, _ := repo.CreateUser("admin", "pass", "Admin", true)

	called := false
	handler := mw.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), userContextKey, admin)
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called for admin user")
	}
}

// TestUserIDFromContext_NoUser returns empty string.
func TestUserIDFromContext_NoUser(t *testing.T) {
	id := UserIDFromContext(context.Background())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// TestUserIDFromContext_WithUser returns user ID.
func TestUserIDFromContext_WithUser(t *testing.T) {
	user := &storage.User{ID: "test-id-123", Username: "test"}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	id := UserIDFromContext(ctx)
	if id != "test-id-123" {
		t.Errorf("expected test-id-123, got %q", id)
	}
}

// TestRequireBasicAuth_Disabled passes through when auth disabled.
func TestRequireBasicAuth_Disabled(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, false)

	called := false
	handler := mw.RequireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/opds/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called when auth is disabled")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestRequireBasicAuth_NoCredentials returns 401 with WWW-Authenticate header.
func TestRequireBasicAuth_NoCredentials(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	handler := mw.RequireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without credentials")
	}))

	req := httptest.NewRequest("GET", "/opds/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	wwwAuth := w.Header().Get("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("expected WWW-Authenticate header")
	}
	if wwwAuth != `Basic realm="Pushkinlib OPDS"` {
		t.Errorf("unexpected WWW-Authenticate: %s", wwwAuth)
	}
}

// TestRequireBasicAuth_WrongPassword returns 401.
func TestRequireBasicAuth_WrongPassword(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	// Create a user
	_, err := repo.CreateUser("opdsuser", "correctpass", "OPDS User", false)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	handler := mw.RequireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with wrong password")
	}))

	req := httptest.NewRequest("GET", "/opds/", nil)
	req.SetBasicAuth("opdsuser", "wrongpass")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestRequireBasicAuth_ValidCredentials passes through with user in context.
func TestRequireBasicAuth_ValidCredentials(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	_, err := repo.CreateUser("opdsuser", "correctpass", "OPDS User", false)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	var ctxUser *storage.User
	handler := mw.RequireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/opds/", nil)
	req.SetBasicAuth("opdsuser", "correctpass")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxUser == nil {
		t.Fatal("user not found in context")
	}
	if ctxUser.Username != "opdsuser" {
		t.Errorf("username = %s, want opdsuser", ctxUser.Username)
	}
}

// TestRequireBasicAuth_UnknownUser returns 401.
func TestRequireBasicAuth_UnknownUser(t *testing.T) {
	repo := setupTestRepo(t)
	mw := NewMiddleware(repo, true)

	handler := mw.RequireBasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for unknown user")
	}))

	req := httptest.NewRequest("GET", "/opds/", nil)
	req.SetBasicAuth("nonexistent", "somepass")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
