package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// CreateUser creates a new user with a bcrypt-hashed password.
func (r *Repository) CreateUser(username, password, displayName string, isAdmin bool) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate user id: %w", err)
	}

	now := time.Now()
	user := &User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		IsAdmin:      isAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = r.db.db.Exec(
		`INSERT INTO users (id, username, password_hash, display_name, is_admin, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.DisplayName, user.IsAdmin, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

// GetUserByUsername returns a user by username, or nil if not found.
func (r *Repository) GetUserByUsername(username string) (*User, error) {
	row := r.db.db.QueryRow(
		`SELECT id, username, password_hash, display_name, is_admin, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	)

	var user User
	var isAdmin int
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName,
		&isAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	user.IsAdmin = isAdmin != 0
	return &user, nil
}

// GetUserByID returns a user by ID, or nil if not found.
func (r *Repository) GetUserByID(id string) (*User, error) {
	row := r.db.db.QueryRow(
		`SELECT id, username, password_hash, display_name, is_admin, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	)

	var user User
	var isAdmin int
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.DisplayName,
		&isAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	user.IsAdmin = isAdmin != 0
	return &user, nil
}

// AuthenticateUser checks username/password and returns the user if valid.
func (r *Repository) AuthenticateUser(username, password string) (*User, error) {
	user, err := r.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil // wrong password
	}

	return user, nil
}

// CreateSession creates a new session for a user. Returns the session token.
func (r *Repository) CreateSession(userID string, duration time.Duration) (*Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	now := time.Now()
	session := &Session{
		Token:     token,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(duration),
	}

	_, err = r.db.db.Exec(
		`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		session.Token, session.UserID, session.CreatedAt, session.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return session, nil
}

// GetSession returns a valid (non-expired) session by token, or nil if not found/expired.
func (r *Repository) GetSession(token string) (*Session, error) {
	row := r.db.db.QueryRow(
		`SELECT token, user_id, created_at, expires_at
		 FROM sessions WHERE token = ? AND expires_at > ?`,
		token, time.Now(),
	)

	var session Session
	err := row.Scan(&session.Token, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session by token.
func (r *Repository) DeleteSession(token string) error {
	_, err := r.db.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes all expired sessions.
func (r *Repository) DeleteExpiredSessions() error {
	_, err := r.db.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now())
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// CountUsers returns the total number of users.
func (r *Repository) CountUsers() (int, error) {
	var count int
	err := r.db.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// generateID generates a random hex ID for users.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateToken generates a random hex token for sessions.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
