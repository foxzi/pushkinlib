package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/piligrim/pushkinlib/internal/auth"
)

const sessionDuration = 30 * 24 * time.Hour // 30 days

// GetAuthInfo returns whether auth is enabled. Public endpoint, no auth required.
// GET /api/v1/auth/info
func (h *Handlers) GetAuthInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"auth_enabled": h.authMw.IsEnabled(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("GetAuthInfo: failed to encode response: %v", err)
	}
}

// Login authenticates a user and creates a session cookie.
// POST /api/v1/auth/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if !h.authMw.IsEnabled() {
		http.Error(w, "Authentication is not enabled", http.StatusNotFound)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.repo.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		log.Printf("Login: authentication error for user %s: %v", req.Username, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Неверное имя пользователя или пароль", http.StatusUnauthorized)
		return
	}

	session, err := h.repo.CreateSession(user.ID, sessionDuration)
	if err != nil {
		log.Printf("Login: failed to create session for user %s: %v", user.Username, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.authMw.CookieName(),
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"user": map[string]interface{}{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"is_admin":     user.IsAdmin,
		},
	}); err != nil {
		log.Printf("Login: failed to encode response: %v", err)
	}
}

// Logout destroys the current session.
// POST /api/v1/auth/logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if !h.authMw.IsEnabled() {
		http.Error(w, "Authentication is not enabled", http.StatusNotFound)
		return
	}

	cookie, err := r.Cookie(h.authMw.CookieName())
	if err == nil && cookie.Value != "" {
		if err := h.repo.DeleteSession(cookie.Value); err != nil {
			log.Printf("Logout: failed to delete session: %v", err)
		}
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.authMw.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Logout: failed to encode response: %v", err)
	}
}

// GetMe returns the currently authenticated user's info.
// GET /api/v1/auth/me
func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	if !h.authMw.IsEnabled() {
		http.Error(w, "Authentication is not enabled", http.StatusNotFound)
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"is_admin":     user.IsAdmin,
	}); err != nil {
		log.Printf("GetMe: failed to encode response: %v", err)
	}
}
