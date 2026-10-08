package db

import (
	"log"
	"os"
)

func (d *DB) seedIfEmpty() error {
	if os.Getenv("GOPOD_SEED_DEMO") != "true" {
		return nil
	}

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

	// Seed Audit Logs
	_, err = tx.Exec(`
		INSERT INTO audit_logs (id, action, actor, target, category, ip, status)
		VALUES
		('aud-1', 'system.daemon.init', 'system', 'podman.socket', 'runtime', '127.0.0.1', 'success'),
		('aud-2', 'caddy.ingress.init', 'caddy', 'reverse-proxy', 'security', '127.0.0.1', 'success'),
		('aud-3', 'service.deploy', 'admin', 'aerochat-web', 'deployment', '127.0.0.1', 'success'),
		('aud-4', 'system.security.audit', 'system', 'cgroups-v2', 'security', '127.0.0.1', 'success')
	`)
	if err != nil {
		return err
	}

	return tx.Commit()
}
