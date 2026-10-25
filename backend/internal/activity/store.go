package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// EngineEventsCap bounds the engine_events table (ring buffer).
const EngineEventsCap = 20000

// Ring-buffer amortization: the cap DELETE scans the whole table, so running
// it on every insert turns each emit into an O(cap) SQLite write and holds
// the single-writer lock against unrelated endpoints. Both prunes therefore
// run one round every N inserts; the table may temporarily overshoot its cap
// by N-1 rows, which is fine for a diagnostic ring buffer.
const (
	engineEventPruneEvery = 250
	ActivityLogCap        = 50000
	activityLogPruneEvery = 500
)

// Per-package dashboards of how many inserts each table has seen. Touched
// exclusively via atomic ops, so it is safe across concurrent emitters.
var (
	engineEventSeq atomic.Uint64
	activityLogSeq atomic.Uint64
)

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

	// Full-uuid identifier: an 8-hex prefix collides at ~4.5% odds per 20k
	// rows (birthday bound) and a failed PK insert silently loses the event.
	id := "evt-" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	// Ring-buffer cap, amortized: with equal-second timestamps the ordering by
	// rowid is the honest insertion order. Best-effort by design.
	if engineEventSeq.Add(1)%engineEventPruneEvery == 0 {
		_, _ = db.ExecContext(qctx,
			`DELETE FROM engine_events WHERE rowid NOT IN (SELECT rowid FROM engine_events ORDER BY rowid DESC LIMIT ?)`,
			EngineEventsCap,
		)
	}
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
	// Same anti-collision rule as engine event ids.
	id := "act-" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	// activity_log previously grew without bound; cap and amortize like
	// engine_events.
	if activityLogSeq.Add(1)%activityLogPruneEvery == 0 {
		_, _ = db.ExecContext(qctx,
			`DELETE FROM activity_log WHERE rowid NOT IN (SELECT rowid FROM activity_log ORDER BY rowid DESC LIMIT ?)`,
			ActivityLogCap,
		)
	}
}
