package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestPragmaInt_AllowedNames verifies allowed PRAGMA names work (#14).
func TestPragmaInt_AllowedNames(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	for name := range allowedPragmas {
		if name == "journal_mode" {
			continue // journal_mode returns string, not int
		}
		_, err := repo.pragmaInt(name)
		if err != nil {
			t.Errorf("pragmaInt(%q) should succeed for allowed name: %v", name, err)
		}
	}
}

// TestPragmaInt_DisallowedName verifies disallowed PRAGMA names are rejected (#14).
func TestPragmaInt_DisallowedName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	_, err = repo.pragmaInt("table_info")
	if err == nil {
		t.Fatal("expected error for disallowed PRAGMA name")
	}
	if !strings.Contains(err.Error(), "disallowed") {
		t.Errorf("expected 'disallowed' in error, got: %v", err)
	}
}

// TestSetPragmaInt_DisallowedName verifies setPragmaInt rejects disallowed names (#14).
func TestSetPragmaInt_DisallowedName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	err = repo.setPragmaInt("user_version; DROP TABLE books;--", 0)
	if err == nil {
		t.Fatal("expected error for SQL injection attempt")
	}
	if !strings.Contains(err.Error(), "disallowed") {
		t.Errorf("expected 'disallowed' in error, got: %v", err)
	}
}

// TestSetPragmaJournalMode_AllowedModes verifies valid journal modes are accepted (#14).
func TestSetPragmaJournalMode_AllowedModes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// WAL should be accepted
	result, err := repo.setPragmaJournalMode("wal")
	if err != nil {
		t.Fatalf("setPragmaJournalMode(wal) failed: %v", err)
	}
	if result != "WAL" {
		t.Errorf("expected WAL, got %s", result)
	}
}

// TestSetPragmaJournalMode_DisallowedMode verifies invalid journal modes are rejected (#14).
func TestSetPragmaJournalMode_DisallowedMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	_, err = repo.setPragmaJournalMode("EVIL; DROP TABLE books;--")
	if err == nil {
		t.Fatal("expected error for disallowed journal mode")
	}
	if !strings.Contains(err.Error(), "disallowed") {
		t.Errorf("expected 'disallowed' in error, got: %v", err)
	}
}

// TestPragmaString_DisallowedName verifies pragmaString rejects disallowed names (#14).
func TestPragmaString_DisallowedName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	_, err = repo.pragmaString("compile_options")
	if err == nil {
		t.Fatal("expected error for disallowed PRAGMA name")
	}
}
