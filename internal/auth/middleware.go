package auth

import (
	"context"
	"net/http"

	"github.com/piligrim/pushkinlib/internal/storage"
)

type contextKey string

const userContextKey contextKey = "auth_user"

// Middleware provides authentication middleware that validates session cookies.
// When auth is disabled, it passes requests through without checking.
type Middleware struct {
	repo        *storage.Repository
	authEnabled bool
	cookieName  string
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(repo *storage.Repository, authEnabled bool) *Middleware {
	return &Middleware{
		repo:        repo,
		authEnabled: authEnabled,
		cookieName:  "pushkinlib_session",
	}
}

// IsEnabled returns whether authentication is enabled.
func (m *Middleware) IsEnabled() bool {
	return m.authEnabled
}

// CookieName returns the session cookie name.
func (m *Middleware) CookieName() string {
	return m.cookieName
}

// RequireAuth is middleware that requires a valid session when auth is enabled.
// When auth is disabled, requests pass through with no user in context.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(m.cookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		session, err := m.repo.GetSession(cookie.Value)
		if err != nil || session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := m.repo.GetUserByID(session.UserID)
		if err != nil || user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that extracts user from session if present,
// but does not reject unauthenticated requests. Use this for endpoints
// that work both with and without auth.
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(m.cookieName)
		if err == nil && cookie.Value != "" {
			session, err := m.repo.GetSession(cookie.Value)
			if err == nil && session != nil {
				user, err := m.repo.GetUserByID(session.UserID)
				if err == nil && user != nil {
					ctx := context.WithValue(r.Context(), userContextKey, user)
					r = r.WithContext(ctx)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAdmin is middleware that requires admin privileges.
func (m *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is authenticated (auth disabled or no session).
func UserFromContext(ctx context.Context) *storage.User {
	user, _ := ctx.Value(userContextKey).(*storage.User)
	return user
}

// UserIDFromContext returns the user ID from context, or empty string if no user.
// Empty string is the correct value for no-auth mode (matches DB convention).
func UserIDFromContext(ctx context.Context) string {
	user := UserFromContext(ctx)
	if user == nil {
		return ""
	}
	return user.ID
}

// RequireBasicAuth is middleware that requires HTTP Basic Auth when auth is enabled.
// This is designed for OPDS clients (e-readers) that support Basic Auth but not cookies.
// Credentials are validated against the users table (same bcrypt passwords).
// When auth is disabled, requests pass through without checking.
func (m *Middleware) RequireBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.authEnabled {
			next.ServeHTTP(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || username == "" || password == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Pushkinlib OPDS"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := m.repo.AuthenticateUser(username, password)
		if err != nil || user == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="Pushkinlib OPDS"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
