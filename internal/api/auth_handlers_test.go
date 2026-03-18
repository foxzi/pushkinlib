package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
