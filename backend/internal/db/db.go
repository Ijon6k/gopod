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

	// Optimize connection pool for SQLite
	conn.SetMaxOpenConns(1) // Single writer in WAL mode ensures zero locking conflict
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

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT DEFAULT 'stopped',
		source TEXT DEFAULT '',
		branch TEXT DEFAULT 'main',
		image TEXT DEFAULT '',
		port INTEGER DEFAULT 3000,
		cpu_limit REAL DEFAULT 0,
		memory_limit INTEGER DEFAULT 0,
		restart_policy TEXT DEFAULT 'always',
		env_vars TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS domains (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		hostname TEXT NOT NULL UNIQUE,
		container_port INTEGER NOT NULL DEFAULT 3000,
		tls BOOLEAN DEFAULT 1,
		https_redirect BOOLEAN DEFAULT 1,
		path_prefix TEXT DEFAULT '/',
		strip_path_prefix BOOLEAN DEFAULT 0,
		websocket BOOLEAN DEFAULT 0,
		cors BOOLEAN DEFAULT 0,
		hsts BOOLEAN DEFAULT 1,
		basic_auth BOOLEAN DEFAULT 0,
		basic_auth_user TEXT DEFAULT '',
		basic_auth_pass TEXT DEFAULT '',
		dns_status TEXT DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS volume_snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		filename TEXT NOT NULL,
		size TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0,
		status TEXT DEFAULT 'completed',
		compression TEXT DEFAULT 'zstd',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS volume_schedules (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		volume_name TEXT NOT NULL,
		cron TEXT NOT NULL,
		label TEXT DEFAULT '',
		retention_count INTEGER DEFAULT 7,
		enabled BOOLEAN DEFAULT 1,
		last_run DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS deployments (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		version TEXT DEFAULT 'v1',
		commit_hash TEXT DEFAULT '',
		commit_message TEXT DEFAULT '',
		status TEXT DEFAULT 'running',
		duration TEXT DEFAULT '0s',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_services_project ON services(project_id);
	CREATE INDEX IF NOT EXISTS idx_domains_project ON domains(project_id);
	CREATE INDEX IF NOT EXISTS idx_deployments_service ON deployments(service_id);
	`

	_, err := d.Exec(schema)
	return err
}

func (d *DB) seedIfEmpty() error {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Println("🌱 Seeding initial AeroChat project and services in SQLite...")

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Seed Project
	_, err = tx.Exec(
		"INSERT INTO projects (id, name, description) VALUES (?, ?, ?)",
		"aerochat", "AeroChat", "Realtime chat application with web client, websocket gateway, and PostgreSQL.",
	)
	if err != nil {
		return err
	}

	// Seed Services
	_, err = tx.Exec(`
		INSERT INTO services (id, project_id, name, type, status, source, branch, port)
		VALUES 
		('aerochat-web', 'aerochat', 'web', 'application', 'running', 'github.com/aerochat/web', 'main', 3000),
		('aerochat-ws', 'aerochat', 'websocket', 'pod', 'running', 'quay.io/aerochat/ws:v3.2', 'main', 8080),
		('aerochat-db', 'aerochat', 'postgres', 'database', 'running', 'postgres:16-alpine', '', 5432)
	`)
	if err != nil {
		return err
	}

	// Seed Domain
	_, err = tx.Exec(`
		INSERT INTO domains (id, project_id, service_id, hostname, container_port, tls, https_redirect, websocket, cors, hsts, dns_status)
		VALUES 
		('d-1', 'aerochat', 'aerochat-web', 'chat.example.com', 3000, 1, 1, 0, 1, 1, 'valid'),
		('d-2', 'aerochat', 'aerochat-ws', 'ws.example.com', 8080, 1, 1, 1, 1, 1, 'valid')
	`)
	if err != nil {
		return err
	}

	// Seed Volume Snapshot
	_, err = tx.Exec(`
		INSERT INTO volume_snapshots (id, project_id, service_id, volume_name, filename, size, size_bytes, status, compression)
		VALUES
		('snap-1', 'aerochat', 'aerochat-db', 'aerochat-pgdata', 'aerochat-pgdata_2026-10-04_0200.tar.zst', '1.2 GB', 1288490188, 'completed', 'zstd')
	`)
	if err != nil {
		return err
	}

	// Seed Volume Schedule
	_, err = tx.Exec(`
		INSERT INTO volume_schedules (id, project_id, volume_name, cron, label, retention_count, enabled)
		VALUES
		('sched-1', 'aerochat', 'aerochat-pgdata', '0 2 * * *', 'Daily at 02:00 AM', 7, 1)
	`)
	if err != nil {
		return err
	}

	return tx.Commit()
}
