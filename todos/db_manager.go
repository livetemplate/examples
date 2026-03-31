package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/livetemplate/examples/todos/db"
	_ "modernc.org/sqlite"
)

var (
	database *sql.DB
	queries  *db.Queries
)

// InitDB initializes the SQLite database and runs migrations
func InitDB(dbPath string) (*db.Queries, error) {
	var err error

	// Open database connection
	database, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations (create tables)
	if err := runMigrations(database); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create queries instance
	queries = db.New(database)

	log.Printf("Database initialized at: %s", dbPath)
	return queries, nil
}

// runMigrations creates the database schema, handling upgrades from older versions.
func runMigrations(database *sql.DB) error {
	// Check if the todos table exists with an outdated schema (missing user_id column).
	// CREATE TABLE IF NOT EXISTS won't modify an existing table, so we must detect
	// and drop the old schema before recreating.
	if needsRecreate, err := hasOutdatedSchema(database); err != nil {
		return fmt.Errorf("checking schema: %w", err)
	} else if needsRecreate {
		log.Println("Detected outdated todos table (missing user_id column), recreating...")
		if _, err := database.Exec("DROP TABLE IF EXISTS todos"); err != nil {
			return fmt.Errorf("dropping outdated table: %w", err)
		}
	}

	schema := `
CREATE TABLE IF NOT EXISTS todos (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  text TEXT NOT NULL,
  completed BOOLEAN NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at);
CREATE INDEX IF NOT EXISTS idx_todos_completed ON todos(completed);
CREATE INDEX IF NOT EXISTS idx_todos_user_id ON todos(user_id);
`
	_, err := database.Exec(schema)
	return err
}

// hasOutdatedSchema returns true if the todos table exists but lacks the user_id column.
func hasOutdatedSchema(database *sql.DB) (bool, error) {
	rows, err := database.Query("PRAGMA table_info(todos)")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var hasTable, hasUserID bool
	for rows.Next() {
		hasTable = true
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == "user_id" {
			hasUserID = true
		}
	}

	return hasTable && !hasUserID, rows.Err()
}

// CloseDB closes the database connection
func CloseDB() {
	if database != nil {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			log.Println("Database connection closed")
		}
	}
}

// GetDBPath returns the database file path, using `:memory:` for tests
func GetDBPath() string {
	// Check if we're running in test mode
	if os.Getenv("TEST_MODE") == "1" {
		return ":memory:"
	}
	return "todos.db"
}
