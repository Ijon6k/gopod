package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EngineEventsCap bounds the engine_events table (ring buffer).
const EngineEventsCap = 20000

var (
	mu        sync.RWMutex
	defaultDB *sql.DB
)

// Init registers the shared database handle used by the best-effort
// package-level helpers. Safe to call once at startup; nil disables emits.
func Init(db *sql.DB) {
	mu.Lock()
	defer mu.Unlock()
	defaultDB = db
}

func sharedDB() *sql.DB {
	mu.RLock()
	defer mu.RUnlock()
	return defaultDB
}

// EmitEngineEvent inserts a best-effort engine event and enforces the ring
// buffer cap with a single delete statement. Never returns an error to the
// caller and never panics.
func EmitEngineEvent(ctx context.Context, level, scope, serviceID, projectID, actor, action, message string, data any) {
	defer func() { _ = recover() }()
	db := sharedDB()
	if db == nil {
		return
	}
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug", "info", "warn", "error":
	default:
		if level == "" {
			level = "info"
		} else {
			level = "info"
		}
	}
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case "service", "project", "system":
	default:
		if scope == "" {
			scope = "system"
		} else {
			scope = "system"
		}
	}

	dataStr := ""
	if data != nil {
		switch v := data.(type) {
		case string:
			dataStr = v
		case []byte:
			dataStr = string(v)
		default:
			if b, err := json.Marshal(v); err == nil {
				dataStr = string(b)
			}
		}
		if len(dataStr) > 8000 {
			dataStr = dataStr[:8000]
		}
	}
	if len(message) > 4000 {
		message = message[:4000]
	}

	id := "evt-" + uuid.New().String()[:8]
	now := time.Now().UTC()
	if ctx == nil {
		ctx = context.Background()
	}
	qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _ = db.ExecContext(qctx,
		`INSERT INTO engine_events (id, ts, level, scope, service_id, project_id, actor, action, message, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, now, level, scope, serviceID, projectID, actor, action, message, dataStr,
	)
	// Ring-buffer cap: single statement, best-effort.
	_, _ = db.ExecContext(qctx,
		`DELETE FROM engine_events WHERE id NOT IN (SELECT id FROM engine_events ORDER BY ts DESC, rowid DESC LIMIT ?)`,
		EngineEventsCap,
	)
}

// RecordActivity inserts a best-effort user-facing activity record.
// Never fails the caller; no-op when the store is not initialized.
func RecordActivity(ctx context.Context, actor, action, entity, entityID, projectID, serviceID, status, detail, ip string) {
	defer func() { _ = recover() }()
	db := sharedDB()
	if db == nil {
		return
	}
	if strings.TrimSpace(action) == "" {
		return
	}
	if actor == "" {
		actor = "operator"
	}
	if status == "" {
		status = "success"
	}
	if len(detail) > 4000 {
		detail = detail[:4000]
	}
	id := "act-" + uuid.New().String()[:8]
	now := time.Now().UTC()
	if ctx == nil {
		ctx = context.Background()
	}
	qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _ = db.ExecContext(qctx,
		`INSERT INTO activity_log (id, ts, actor, action, entity, entity_id, project_id, service_id, status, detail, ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, now, actor, action, entity, entityID, projectID, serviceID, status, detail, ip,
	)
}
