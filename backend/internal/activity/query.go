package activity

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Store provides filtered reads for the HTTP handlers.
type Store struct {
	db *sql.DB
}

// NewStore creates a query store. May hold a nil DB (returns empty lists).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func parseTimeFlexible(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func scanEngineEvent(rows *sql.Rows) (EngineEvent, error) {
	var e EngineEvent
	var tsRaw any
	err := rows.Scan(&e.ID, &tsRaw, &e.Level, &e.Scope, &e.ServiceID, &e.ProjectID, &e.Actor, &e.Action, &e.Message, &e.Data)
	if err != nil {
		return e, err
	}
	e.TS = coerceTime(tsRaw)
	return e, nil
}

func scanActivity(rows *sql.Rows) (ActivityEntry, error) {
	var a ActivityEntry
	var tsRaw any
	err := rows.Scan(&a.ID, &tsRaw, &a.Actor, &a.Action, &a.Entity, &a.EntityID, &a.ProjectID, &a.ServiceID, &a.Status, &a.Detail, &a.IP)
	if err != nil {
		return a, err
	}
	a.TS = coerceTime(tsRaw)
	return a, nil
}

func coerceTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		if parsed, ok := parseTimeFlexible(t); ok {
			return parsed
		}
	}
	return time.Time{}
}

// ListEngineEvents returns filtered engine events newest-first plus total count.
func (s *Store) ListEngineEvents(ctx context.Context, level, serviceID, projectID, since string, limit int) ([]EngineEvent, int, error) {
	if s == nil || s.db == nil {
		return []EngineEvent{}, 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var conds []string
	var args []any
	if v := strings.TrimSpace(level); v != "" && v != "all" {
		conds = append(conds, "level = ?")
		args = append(args, strings.ToLower(v))
	}
	if v := strings.TrimSpace(serviceID); v != "" {
		conds = append(conds, "service_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(projectID); v != "" {
		conds = append(conds, "project_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(since); v != "" {
		if t, ok := parseTimeFlexible(v); ok {
			conds = append(conds, "ts >= ?")
			args = append(args, t.UTC())
		}
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	total := 0
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM engine_events "+where, args...).Scan(&total)

	q := "SELECT id, ts, level, scope, service_id, project_id, actor, action, message, data FROM engine_events " + where + " ORDER BY ts DESC, rowid DESC LIMIT ?"
	qargs := append(append([]any{}, args...), limit)
	rows, err := s.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return []EngineEvent{}, total, err
	}
	defer rows.Close()
	items := []EngineEvent{}
	for rows.Next() {
		e, err := scanEngineEvent(rows)
		if err != nil {
			return items, total, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []EngineEvent{}
	}
	return items, total, rows.Err()
}

// ListActivity returns filtered activity entries newest-first plus total count.
func (s *Store) ListActivity(ctx context.Context, projectID, serviceID, action, status string, limit, offset int) ([]ActivityEntry, int, error) {
	if s == nil || s.db == nil {
		return []ActivityEntry{}, 0, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	var conds []string
	var args []any
	if v := strings.TrimSpace(projectID); v != "" {
		conds = append(conds, "project_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(serviceID); v != "" {
		conds = append(conds, "service_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(action); v != "" && v != "all" {
		conds = append(conds, "action = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(status); v != "" && v != "all" {
		conds = append(conds, "status = ?")
		args = append(args, v)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	total := 0
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM activity_log "+where, args...).Scan(&total)

	q := "SELECT id, ts, actor, action, entity, entity_id, project_id, service_id, status, detail, ip FROM activity_log " + where + " ORDER BY ts DESC, rowid DESC LIMIT ? OFFSET ?"
	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return []ActivityEntry{}, total, err
	}
	defer rows.Close()
	items := []ActivityEntry{}
	for rows.Next() {
		a, err := scanActivity(rows)
		if err != nil {
			return items, total, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []ActivityEntry{}
	}
	return items, total, rows.Err()
}
