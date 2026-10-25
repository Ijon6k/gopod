package db

import (
	"fmt"
	"strings"
)

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
		description TEXT DEFAULT '',
		quadlet_config TEXT DEFAULT '',
		compose_yaml TEXT DEFAULT '',
		k8s_yaml TEXT DEFAULT '',
		runtime_target TEXT DEFAULT '',
		env_vars TEXT DEFAULT '[]',
		advanced TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS domains (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		target_container TEXT DEFAULT '',
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
		remote_provider TEXT DEFAULT '',
		remote_destination_id TEXT DEFAULT '',
		remote_key TEXT DEFAULT '',
		checksum TEXT DEFAULT '',
		remote_status TEXT DEFAULT '',
		remote_error TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS volume_schedules (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		cron TEXT NOT NULL,
		label TEXT DEFAULT '',
		retention_count INTEGER DEFAULT 7,
		enabled BOOLEAN DEFAULT 1,
		last_run DATETIME,
		destination_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS backup_destinations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		region TEXT DEFAULT 'us-east-1',
		access_key TEXT DEFAULT '',
		secret_enc TEXT DEFAULT '',
		bucket TEXT NOT NULL,
		prefix TEXT DEFAULT '',
		path_style INTEGER DEFAULT 1,
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

	CREATE TABLE IF NOT EXISTS ssh_keys (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		public_key TEXT NOT NULL,
		private_key TEXT DEFAULT '',
		fingerprint TEXT DEFAULT '',
		type TEXT DEFAULT 'ed25519',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS container_registries (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		username TEXT NOT NULL,
		token TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS git_tokens (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		provider TEXT NOT NULL,
		username TEXT DEFAULT '',
		token TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS secrets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		value TEXT DEFAULT '',
		driver TEXT DEFAULT 'file',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		name TEXT DEFAULT '',
		role TEXT DEFAULT 'admin',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		actor TEXT NOT NULL,
		target TEXT NOT NULL,
		category TEXT NOT NULL,
		ip TEXT DEFAULT '',
		status TEXT DEFAULT 'success',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS engine_events (
		id TEXT PRIMARY KEY,
		ts DATETIME DEFAULT CURRENT_TIMESTAMP,
		level TEXT DEFAULT 'info',
		scope TEXT DEFAULT 'system',
		service_id TEXT DEFAULT '',
		project_id TEXT DEFAULT '',
		actor TEXT DEFAULT '',
		action TEXT DEFAULT '',
		message TEXT DEFAULT '',
		data TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS activity_log (
		id TEXT PRIMARY KEY,
		ts DATETIME DEFAULT CURRENT_TIMESTAMP,
		actor TEXT DEFAULT '',
		action TEXT NOT NULL,
		entity TEXT DEFAULT '',
		entity_id TEXT DEFAULT '',
		project_id TEXT DEFAULT '',
		service_id TEXT DEFAULT '',
		status TEXT DEFAULT 'success',
		detail TEXT DEFAULT '',
		ip TEXT DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_services_project ON services(project_id);
	CREATE INDEX IF NOT EXISTS idx_domains_project ON domains(project_id);
	CREATE INDEX IF NOT EXISTS idx_deployments_service ON deployments(service_id);
	CREATE INDEX IF NOT EXISTS idx_deployments_project ON deployments(project_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
	CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_logs(category);
	CREATE INDEX IF NOT EXISTS idx_engine_events_service ON engine_events(service_id);
	CREATE INDEX IF NOT EXISTS idx_engine_events_project ON engine_events(project_id);
	CREATE INDEX IF NOT EXISTS idx_engine_events_level ON engine_events(level);
	CREATE INDEX IF NOT EXISTS idx_engine_events_ts ON engine_events(ts);
	CREATE INDEX IF NOT EXISTS idx_activity_project ON activity_log(project_id);
	CREATE INDEX IF NOT EXISTS idx_activity_service ON activity_log(service_id);
	CREATE INDEX IF NOT EXISTS idx_activity_action ON activity_log(action);
	CREATE INDEX IF NOT EXISTS idx_activity_ts ON activity_log(ts);
	`

	if _, err := d.Exec(schema); err != nil {
		return err
	}

	// Safe column additions for service Git, Quadlet, and Webhook support
	newCols := []string{
		"ALTER TABLE services ADD COLUMN webhook_token TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN git_repo TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN git_branch TEXT DEFAULT 'main';",
		"ALTER TABLE services ADD COLUMN dockerfile_path TEXT DEFAULT 'Dockerfile';",
		"ALTER TABLE services ADD COLUMN ssh_key_id TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN description TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN quadlet_config TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN compose_yaml TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN k8s_yaml TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN runtime_target TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN in_pod BOOLEAN DEFAULT 0;",
		"ALTER TABLE services ADD COLUMN host_port INTEGER DEFAULT 0;",
		"ALTER TABLE services ADD COLUMN host_ip TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN advanced TEXT DEFAULT '{}';",
		"ALTER TABLE services ADD COLUMN source_type TEXT DEFAULT 'git';",
		"ALTER TABLE services ADD COLUMN build_type TEXT DEFAULT 'dockerfile';",
		"ALTER TABLE services ADD COLUMN build_path TEXT DEFAULT '/';",
		"ALTER TABLE domains ADD COLUMN target_container TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN publish_dir TEXT DEFAULT 'dist';",
		"ALTER TABLE services ADD COLUMN pod_id TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN registry_id TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN auto_deploy BOOLEAN DEFAULT 1;",
		"ALTER TABLE services ADD COLUMN watch_paths TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN command TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN entrypoint TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN git_credential_id TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN build_args TEXT DEFAULT '[]';",
		"ALTER TABLE services ADD COLUMN webhook_provider TEXT DEFAULT '';",
		"ALTER TABLE services ADD COLUMN webhook_secret TEXT DEFAULT '';",
		"ALTER TABLE deployments ADD COLUMN commit_hash TEXT DEFAULT '';",
		"ALTER TABLE deployments ADD COLUMN commit_message TEXT DEFAULT '';",
		"ALTER TABLE deployments ADD COLUMN trigger TEXT DEFAULT 'manual';",
		"ALTER TABLE deployments ADD COLUMN number INTEGER DEFAULT 1;",
		"ALTER TABLE deployments ADD COLUMN image TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN remote_provider TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN remote_destination_id TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN remote_key TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN checksum TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN remote_status TEXT DEFAULT '';",
		"ALTER TABLE volume_snapshots ADD COLUMN remote_error TEXT DEFAULT '';",
		"ALTER TABLE volume_schedules ADD COLUMN destination_id TEXT DEFAULT '';",
		"ALTER TABLE volume_schedules ADD COLUMN service_id TEXT DEFAULT '';",
		"CREATE TABLE IF NOT EXISTS backup_destinations (" +
			" id TEXT PRIMARY KEY," +
			" name TEXT NOT NULL," +
			" endpoint TEXT NOT NULL," +
			" region TEXT DEFAULT 'us-east-1'," +
			" access_key TEXT DEFAULT ''," +
			" secret_enc TEXT DEFAULT ''," +
			" bucket TEXT NOT NULL," +
			" prefix TEXT DEFAULT ''," +
			" path_style INTEGER DEFAULT 1," +
			" created_at DATETIME DEFAULT CURRENT_TIMESTAMP" +
			");",
		"CREATE INDEX IF NOT EXISTS idx_services_webhook ON services(webhook_token);",
		"CREATE INDEX IF NOT EXISTS idx_deployments_project ON deployments(project_id);",
		"CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);",
	}
	for _, q := range newCols {
		if _, err := d.Exec(q); err != nil {
			// Schema migrations are re-run on every boot; "duplicate column
			// name" means the statement already succeeded on a previous run
			// and must not abort startup. Any other failure is fatal: a
			// partially migrated schema must never boot silently (REL-7).
			msg := err.Error()
			if !strings.Contains(msg, "duplicate column name") {
				return fmt.Errorf("schema migration %q: %w", q, err)
			}
		}
	}

	if err := d.migrateData(); err != nil {
		return err
	}
	return nil
}

// Data migrations are keyed by PRAGMA user_version; each entry runs once.
var dataMigrations = []string{
	// auto_deploy existed as an inert column, so every stored false value is
	// the zero value rather than an operator choice. Start everyone enabled.
	"UPDATE services SET auto_deploy = 1",
}

func (d *DB) migrateData() error {
	var version int
	if err := d.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read PRAGMA user_version: %w", err)
	}
	for i := version; i < len(dataMigrations); i++ {
		if _, err := d.Exec(dataMigrations[i]); err != nil {
			return fmt.Errorf("data migration %d %q: %w", i, dataMigrations[i], err)
		}
		if _, err := d.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return fmt.Errorf("advance PRAGMA user_version to %d: %w", i+1, err)
		}
	}
	return nil
}
