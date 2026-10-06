package audit

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestAuditLogCreateAndList(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE audit_logs (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		actor TEXT NOT NULL,
		target TEXT NOT NULL,
		category TEXT NOT NULL,
		ip TEXT DEFAULT '',
		status TEXT DEFAULT 'success',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to run schema: %v", err)
	}

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)

	ctx := context.Background()
	if err := svc.Record(ctx, "service.deploy", "admin", "aerochat-web", "deployment", "127.0.0.1", "success"); err != nil {
		t.Fatalf("failed to record audit log: %v", err)
	}

	if err := svc.Record(ctx, "user.login", "admin", "session", "security", "127.0.0.1", "success"); err != nil {
		t.Fatalf("failed to record second audit log: %v", err)
	}

	// Test list all
	logs, err := svc.List(ctx, "all", 10)
	if err != nil {
		t.Fatalf("failed to list audit logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 audit logs, got %d", len(logs))
	}

	// Test category filter
	deployLogs, err := svc.List(ctx, "deployment", 10)
	if err != nil {
		t.Fatalf("failed to list deployment audit logs: %v", err)
	}
	if len(deployLogs) != 1 {
		t.Fatalf("expected 1 deployment log, got %d", len(deployLogs))
	}
	if deployLogs[0].Target != "aerochat-web" {
		t.Errorf("expected target 'aerochat-web', got %q", deployLogs[0].Target)
	}
}
