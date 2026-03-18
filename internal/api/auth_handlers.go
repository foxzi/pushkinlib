package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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

// ListUsers returns all users (admin only).
// GET /api/v1/admin/users
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.repo.ListUsers()
	if err != nil {
		log.Printf("ListUsers: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type userResponse struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
		CreatedAt   string `json:"created_at"`
	}
	result := make([]userResponse, 0, len(users))
	for _, u := range users {
		result = append(result, userResponse{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			IsAdmin:     u.IsAdmin,
			CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("ListUsers: failed to encode response: %v", err)
	}
}

// CreateUser creates a new user (admin only).
// POST /api/v1/admin/users
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Имя пользователя и пароль обязательны", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "Пароль должен быть не менее 6 символов", http.StatusBadRequest)
		return
	}

	// Check if username already exists
	existing, err := h.repo.GetUserByUsername(req.Username)
	if err != nil {
		log.Printf("CreateUser: check existing user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "Пользователь с таким именем уже существует", http.StatusConflict)
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user, err := h.repo.CreateUser(req.Username, req.Password, displayName, req.IsAdmin)
	if err != nil {
		log.Printf("CreateUser: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"is_admin":     user.IsAdmin,
		"created_at":   user.CreatedAt.Format(time.RFC3339),
	}); err != nil {
		log.Printf("CreateUser: failed to encode response: %v", err)
	}
}

// DeleteUser deletes a user (admin only, cannot delete yourself).
// DELETE /api/v1/admin/users/{id}
func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Prevent self-deletion
	currentUser := auth.UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == userID {
		http.Error(w, "Нельзя удалить самого себя", http.StatusBadRequest)
		return
	}

	if err := h.repo.DeleteUser(userID); err != nil {
		if err.Error() == "user not found" {
			http.Error(w, "Пользователь не найден", http.StatusNotFound)
			return
		}
		log.Printf("DeleteUser: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("DeleteUser: failed to encode response: %v", err)
	}
}

// UpdateUserPassword changes a user's password (admin only).
// PUT /api/v1/admin/users/{id}/password
func (h *Handlers) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "Пароль должен быть не менее 6 символов", http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateUserPassword(userID, req.Password); err != nil {
		if err.Error() == "user not found" {
			http.Error(w, "Пользователь не найден", http.StatusNotFound)
			return
		}
		log.Printf("UpdateUserPassword: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("UpdateUserPassword: failed to encode response: %v", err)
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
