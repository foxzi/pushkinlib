package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaFS embed.FS

// Database wraps SQLite database operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection and initializes schema
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure directory exists
	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{db: db}

	if err := database.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// DB returns the underlying sql.DB for advanced operations
func (d *Database) DB() *sql.DB {
	return d.db
}

// initSchema initializes the database schema
func (d *Database) initSchema() error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	_, err = d.db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	if err := d.migrateReadingPositions(); err != nil {
		return fmt.Errorf("failed to migrate reading_positions: %w", err)
	}

	return nil
}

// migrateReadingPositions adds new columns to reading_positions for existing databases.
func (d *Database) migrateReadingPositions() error {
	migrations := []struct {
		column string
		ddl    string
	}{
		{"total_sections", "ALTER TABLE reading_positions ADD COLUMN total_sections INTEGER NOT NULL DEFAULT 0"},
		{"status", "ALTER TABLE reading_positions ADD COLUMN status TEXT NOT NULL DEFAULT 'reading'"},
		{"started_at", "ALTER TABLE reading_positions ADD COLUMN started_at DATETIME DEFAULT CURRENT_TIMESTAMP"},
	}

	for _, m := range migrations {
		if !d.columnExists("reading_positions", m.column) {
			if _, err := d.db.Exec(m.ddl); err != nil {
				return fmt.Errorf("add column %s: %w", m.column, err)
			}
		}
	}
	return nil
}

// columnExists checks whether a column exists in a table.
func (d *Database) columnExists(table, column string) bool {
	rows, err := d.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// ensureDir creates directory if it doesn't exist
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
