package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/piligrim/pushkinlib/internal/auth"
)

// setupAuthHandlers creates handlers with auth enabled and an admin user.
func setupAuthHandlers(t *testing.T) (*Handlers, string) {
	t.Helper()
	h := setupTestHandlers(t) // uses auth disabled

	// Create a version with auth enabled
	authMw := auth.NewMiddleware(h.repo, true)
	h.authMw = authMw

	// Create admin user
	user, err := h.repo.CreateUser("admin", "admin123", "Admin", true)
	if err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	return h, user.ID
}

// TestGetAuthInfo_Disabled returns auth_enabled=false.
func TestGetAuthInfo_Disabled(t *testing.T) {
	h := setupTestHandlers(t) // auth disabled by default

	req := httptest.NewRequest("GET", "/api/v1/auth/info", nil)
	w := httptest.NewRecorder()
	h.GetAuthInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["auth_enabled"] != false {
		t.Errorf("auth_enabled = %v, want false", resp["auth_enabled"])
	}
}

// TestGetAuthInfo_Enabled returns auth_enabled=true.
func TestGetAuthInfo_Enabled(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/info", nil)
	w := httptest.NewRecorder()
	h.GetAuthInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["auth_enabled"] != true {
		t.Errorf("auth_enabled = %v, want true", resp["auth_enabled"])
	}
}

// TestLogin_Success logs in and returns a session cookie.
func TestLogin_Success(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check response body
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	user := resp["user"].(map[string]interface{})
	if user["username"] != "admin" {
		t.Errorf("username = %v, want admin", user["username"])
	}
	if user["is_admin"] != true {
		t.Errorf("is_admin = %v, want true", user["is_admin"])
	}

	// Check cookie
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "pushkinlib_session" && c.Value != "" {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("session cookie not found in response")
	}
}

// TestLogin_WrongPassword returns 401.
func TestLogin_WrongPassword(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrong",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestLogin_EmptyFields returns 400.
func TestLogin_EmptyFields(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	body, _ := json.Marshal(map[string]string{
		"username": "",
		"password": "",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestLogin_AuthDisabled returns 404.
func TestLogin_AuthDisabled(t *testing.T) {
	h := setupTestHandlers(t)

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when auth disabled, got %d", w.Code)
	}
}

// TestLogout clears session cookie.
func TestLogout(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	// Login first
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	// Get session cookie
	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "pushkinlib_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie from login")
	}

	// Logout
	logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	h.Logout(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", logoutW.Code)
	}

	// Check cookie is cleared
	for _, c := range logoutW.Result().Cookies() {
		if c.Name == "pushkinlib_session" {
			if c.MaxAge != -1 {
				t.Errorf("session cookie MaxAge = %d, want -1 (cleared)", c.MaxAge)
			}
		}
	}
}

// TestGetMe_WithSession returns user info.
func TestGetMe_WithSession(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	// Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "pushkinlib_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie from login")
	}

	// Get /me through the middleware chain
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.AddCookie(sessionCookie)

	// Run through RequireAuth middleware first to populate context
	meW := httptest.NewRecorder()
	chain := h.authMw.RequireAuth(http.HandlerFunc(h.GetMe))
	chain.ServeHTTP(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("GetMe: expected 200, got %d: %s", meW.Code, meW.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(meW.Body.Bytes(), &resp)
	if resp["username"] != "admin" {
		t.Errorf("username = %v, want admin", resp["username"])
	}
	if resp["is_admin"] != true {
		t.Errorf("is_admin = %v, want true", resp["is_admin"])
	}
}

// ---- Admin User Management Tests ----

// loginAndGetCookie is a helper that logs in and returns the session cookie.
func loginAndGetCookie(t *testing.T, h *Handlers) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)
	for _, c := range w.Result().Cookies() {
		if c.Name == "pushkinlib_session" {
			return c
		}
	}
	t.Fatal("no session cookie from login")
	return nil
}

// serveAdmin runs a handler through RequireAuth + RequireAdmin middleware chain.
func serveAdmin(h *Handlers, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	chain := h.authMw.RequireAuth(h.authMw.RequireAdmin(handler))
	chain.ServeHTTP(w, req)
	return w
}

func TestListUsers(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	req.AddCookie(cookie)
	w := serveAdmin(h, h.ListUsers, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var users []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &users)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0]["username"] != "admin" {
		t.Errorf("username = %v, want admin", users[0]["username"])
	}
}

func TestCreateUser_Success(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	body, _ := json.Marshal(map[string]interface{}{
		"username":     "alice",
		"password":     "secret123",
		"display_name": "Alice",
		"is_admin":     false,
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := serveAdmin(h, h.CreateUser, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["username"] != "alice" {
		t.Errorf("username = %v, want alice", resp["username"])
	}
	if resp["is_admin"] != false {
		t.Errorf("is_admin = %v, want false", resp["is_admin"])
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	body, _ := json.Marshal(map[string]interface{}{
		"username": "admin",
		"password": "secret123",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := serveAdmin(h, h.CreateUser, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_ShortPassword(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	body, _ := json.Marshal(map[string]interface{}{
		"username": "bob",
		"password": "12345",
	})
	req := httptest.NewRequest("POST", "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := serveAdmin(h, h.CreateUser, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteUser_Success(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	// Create a user to delete
	user, err := h.repo.CreateUser("toremove", "secret123", "Remove Me", false)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+user.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.AddCookie(cookie)
	w := serveAdmin(h, h.DeleteUser, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify user is gone
	deleted, _ := h.repo.GetUserByID(user.ID)
	if deleted != nil {
		t.Error("user should be deleted")
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	h, adminID := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/users/"+adminID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", adminID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.AddCookie(cookie)
	w := serveAdmin(h, h.DeleteUser, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 (cannot delete self), got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUserPassword_Success(t *testing.T) {
	h, _ := setupAuthHandlers(t)
	cookie := loginAndGetCookie(t, h)

	// Create a user
	user, _ := h.repo.CreateUser("pwtest", "oldpass123", "PW Test", false)

	body, _ := json.Marshal(map[string]string{"password": "newpass789"})
	req := httptest.NewRequest("PUT", "/api/v1/admin/users/"+user.ID+"/password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.AddCookie(cookie)
	w := serveAdmin(h, h.UpdateUserPassword, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify new password works
	authed, _ := h.repo.AuthenticateUser("pwtest", "newpass789")
	if authed == nil {
		t.Error("new password should authenticate successfully")
	}

	// Verify old password fails
	authedOld, _ := h.repo.AuthenticateUser("pwtest", "oldpass123")
	if authedOld != nil {
		t.Error("old password should no longer work")
	}
}

func TestListUsers_Unauthorized(t *testing.T) {
	h, _ := setupAuthHandlers(t)

	// Create a non-admin user
	h.repo.CreateUser("normaluser", "secret123", "Normal", false)

	// Login as non-admin
	body, _ := json.Marshal(map[string]string{"username": "normaluser", "password": "secret123"})
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	h.Login(loginW, loginReq)

	var cookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "pushkinlib_session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	req.AddCookie(cookie)
	w := serveAdmin(h, h.ListUsers, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", w.Code)
	}
}
