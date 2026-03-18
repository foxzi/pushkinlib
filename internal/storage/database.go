package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	// Migrate reading_positions table BEFORE running schema.sql,
	// because schema.sql now defines the new composite PK table.
	// If the old table exists (without user_id), we must recreate it first.
	if err := d.migrateReadingPositionsPK(); err != nil {
		return fmt.Errorf("failed to migrate reading_positions PK: %w", err)
	}

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

// migrateReadingPositionsPK migrates reading_positions from old schema (book_id-only PK)
// to new schema with composite PK (user_id, book_id).
// This runs BEFORE schema.sql so the CREATE TABLE IF NOT EXISTS won't conflict.
func (d *Database) migrateReadingPositionsPK() error {
	// Check if table exists at all
	if !d.tableExists("reading_positions") {
		return nil // fresh DB, schema.sql will create the correct table
	}

	// Check if user_id column already exists — already migrated
	if d.columnExists("reading_positions", "user_id") {
		return nil
	}

	// Old table exists without user_id — need to recreate with composite PK.
	// Use a transaction for atomicity.
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback()

	// Create new table with composite PK
	_, err = tx.Exec(`CREATE TABLE reading_positions_new (
		user_id TEXT NOT NULL DEFAULT '',
		book_id TEXT NOT NULL,
		section INTEGER NOT NULL DEFAULT 0,
		progress REAL NOT NULL DEFAULT 0.0,
		total_sections INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'reading',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, book_id),
		FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("create reading_positions_new: %w", err)
	}

	// Build column list from old table (may or may not have total_sections, status, started_at)
	oldCols := []string{"book_id", "section", "progress", "updated_at"}
	if d.columnExists("reading_positions", "total_sections") {
		oldCols = append(oldCols, "total_sections")
	}
	if d.columnExists("reading_positions", "status") {
		oldCols = append(oldCols, "status")
	}
	if d.columnExists("reading_positions", "started_at") {
		oldCols = append(oldCols, "started_at")
	}

	colList := strings.Join(oldCols, ", ")
	copySQL := fmt.Sprintf(
		"INSERT INTO reading_positions_new (user_id, %s) SELECT '', %s FROM reading_positions",
		colList, colList,
	)
	if _, err := tx.Exec(copySQL); err != nil {
		return fmt.Errorf("copy reading_positions data: %w", err)
	}

	if _, err := tx.Exec("DROP TABLE reading_positions"); err != nil {
		return fmt.Errorf("drop old reading_positions: %w", err)
	}

	if _, err := tx.Exec("ALTER TABLE reading_positions_new RENAME TO reading_positions"); err != nil {
		return fmt.Errorf("rename reading_positions_new: %w", err)
	}

	return tx.Commit()
}

// tableExists checks whether a table exists in the database.
func (d *Database) tableExists(table string) bool {
	var count int
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&count)
	return err == nil && count > 0
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
