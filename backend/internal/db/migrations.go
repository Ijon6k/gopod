package db

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

	CREATE INDEX IF NOT EXISTS idx_services_project ON services(project_id);
	CREATE INDEX IF NOT EXISTS idx_domains_project ON domains(project_id);
	CREATE INDEX IF NOT EXISTS idx_deployments_service ON deployments(service_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_audit_category ON audit_logs(category);
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
		"ALTER TABLE deployments ADD COLUMN commit_hash TEXT DEFAULT '';",
		"ALTER TABLE deployments ADD COLUMN commit_message TEXT DEFAULT '';",
		"ALTER TABLE deployments ADD COLUMN trigger TEXT DEFAULT 'manual';",
		"ALTER TABLE deployments ADD COLUMN number INTEGER DEFAULT 1;",
		"ALTER TABLE deployments ADD COLUMN image TEXT DEFAULT '';",
	}
	for _, q := range newCols {
		_, _ = d.Exec(q)
	}

	return nil
}
