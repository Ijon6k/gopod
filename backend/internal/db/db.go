package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with application-specific helpers.
type DB struct {
	*sql.DB
	path string
}

// Open initializes SQLite connection with WAL mode and runs migrations.
func Open(customPath string) (*DB, error) {
	dbPath := customPath
	if dbPath == "" {
		dbPath = os.Getenv("GOPOD_DB_PATH")
	}
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dbPath = filepath.Join(home, ".local", "share", "gopod", "data", "gopod.db")
		} else {
			dbPath = "gopod.db"
		}
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory %s: %w", dir, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Optimize connection pool for SQLite: single writer in WAL mode
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	database := &DB{DB: conn, path: dbPath}
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	if err := database.seedIfEmpty(); err != nil {
		log.Printf("[WARN] Seeding initial data failed: %v", err)
	}

	log.Printf("✅ SQLite database ready at: %s (WAL mode)", dbPath)
	return database, nil
}

// Path returns the physical file location.
func (d *DB) Path() string {
	return d.path
}
