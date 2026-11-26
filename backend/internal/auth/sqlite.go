package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
)

// sessionTokenPrefix marks hashed session tokens at rest. Self-describing so
// the start-up migration is idempotent (raw tokens and hashes are both 64-char
// hex and cannot be told apart otherwise).
const sessionTokenPrefix = "sha256:"

// hashSessionToken maps a raw client token to its at-rest form.
func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return sessionTokenPrefix + hex.EncodeToString(sum[:])
}

// SQLiteRepository implements Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite auth repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func hydrateUserPermissions(u *User, rawPerms sql.NullString) {
	if u == nil {
		return
	}
	if u.IsAdmin() {
		u.Permissions = AllPermissions()
		return
	}
	if rawPerms.Valid {
		u.Permissions = ParsePermissions(rawPerms.String)
	} else {
		u.Permissions = DefaultMemberPermissions()
	}
}

func (r *SQLiteRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	var rawPerms sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, role, COALESCE(permissions_json, '{}'), created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &rawPerms, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hydrateUserPermissions(&u, rawPerms)
	return &u, nil
}

func (r *SQLiteRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	var rawPerms sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, role, COALESCE(permissions_json, '{}'), created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &rawPerms, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hydrateUserPermissions(&u, rawPerms)
	return &u, nil
}

func (r *SQLiteRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	return err
}

func (r *SQLiteRepository) CreateUser(ctx context.Context, u User) error {
	permsJSON := u.Permissions.MarshalJSONString()
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password_hash, name, role, permissions_json) VALUES (?, ?, ?, ?, ?, ?)",
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role, permsJSON,
	)
	return err
}

func (r *SQLiteRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// CreateSession stores the session with the token hashed: a database dump must
// not hand out working sessions.
func (r *SQLiteRepository) CreateSession(ctx context.Context, s Session) error {
	sid := strings.TrimSpace(s.ID)
	if sid == "" {
		if rnd, err := GenerateRandomToken(12); err == nil {
			sid = "ses_" + rnd
		}
	}
	lastSeen := s.LastSeenAt
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (id, token, user_id, ip_address, user_agent, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		sid, hashSessionToken(s.Token), s.UserID, s.IPAddress, s.UserAgent, lastSeen, s.ExpiresAt,
	)
	return err
}

func (r *SQLiteRepository) getSessionByStoredToken(ctx context.Context, stored string) (*Session, *User, error) {
	var s Session
	var u User
	var rawPerms, sid, ipAddr, ua sql.NullString
	var lastSeen sql.NullTime
	query := `
		SELECT COALESCE(s.id, ''), s.token, s.user_id, COALESCE(s.ip_address, ''), COALESCE(s.user_agent, ''), s.last_seen_at, s.expires_at, s.created_at,
		       u.id, u.email, u.name, u.role, COALESCE(u.permissions_json, '{}'), u.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?
	`
	err := r.db.QueryRowContext(ctx, query, stored, time.Now()).
		Scan(&sid, &s.Token, &s.UserID, &ipAddr, &ua, &lastSeen, &s.ExpiresAt, &s.CreatedAt,
			&u.ID, &u.Email, &u.Name, &u.Role, &rawPerms, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	s.ID = sid.String
	s.IPAddress = ipAddr.String
	s.UserAgent = ua.String
	if lastSeen.Valid {
		s.LastSeenAt = lastSeen.Time
	} else {
		s.LastSeenAt = s.CreatedAt
	}
	hydrateUserPermissions(&u, rawPerms)
	return &s, &u, nil
}

// GetSession resolves a raw client token (hashed lookup) with a plaintext-row
// fallback for rows a failed migration may have left behind.
func (r *SQLiteRepository) GetSession(ctx context.Context, token string) (*Session, *User, error) {
	s, u, err := r.getSessionByStoredToken(ctx, hashSessionToken(token))
	if err != nil || s != nil {
		return s, u, err
	}
	if strings.HasPrefix(token, sessionTokenPrefix) {
		return nil, nil, nil
	}
	return r.getSessionByStoredToken(ctx, token)
}

// DeleteSession removes the session matching a raw client token.
func (r *SQLiteRepository) DeleteSession(ctx context.Context, token string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", hashSessionToken(token)); err != nil {
		return err
	}
	if !strings.HasPrefix(token, sessionTokenPrefix) {
		_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
		return err
	}
	return nil
}

// PruneExpiredSessions deletes sessions past their expiry. Returns the number
// of removed rows.
func (r *SQLiteRepository) PruneExpiredSessions(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteOtherSessions invalidates every session of a user except the provided
// client token. Both the hashed form and any legacy plaintext twin of the kept
// token survive, while every other session — hashed or not — is removed, so a
// password change kicks out stolen session tokens.
func (r *SQLiteRepository) DeleteOtherSessions(ctx context.Context, userID, keepToken string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE user_id = ? AND token NOT IN (?, ?)",
		userID, hashSessionToken(keepToken), keepToken)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *SQLiteRepository) TouchSessionLastSeen(ctx context.Context, token string, at time.Time) error {
	if token == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		"UPDATE sessions SET last_seen_at = ? WHERE token IN (?, ?)",
		at, hashSessionToken(token), token,
	)
	return err
}

// BackfillSessionMeta fills empty ip_address or user_agent fields on existing sessions
// (e.g. sessions created prior to schema metadata columns).
func (r *SQLiteRepository) BackfillSessionMeta(ctx context.Context, token, ipAddr, userAgent string) error {
	if token == "" {
		return nil
	}
	ipAddr = strings.TrimSpace(ipAddr)
	userAgent = SanitizeUserAgent(userAgent)
	if ipAddr == "" && userAgent == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions
		 SET ip_address = COALESCE(NULLIF(ip_address, ''), ?),
		     user_agent = COALESCE(NULLIF(user_agent, ''), ?)
		 WHERE token IN (?, ?)`,
		ipAddr, userAgent, hashSessionToken(token), token,
	)
	return err
}

func (r *SQLiteRepository) GetSessionIDByToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	var id sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM sessions WHERE token IN (?, ?) AND expires_at > ? LIMIT 1",
		hashSessionToken(token), token, time.Now(),
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id.String, err
}

func (r *SQLiteRepository) ListUserSessions(ctx context.Context, userID string) ([]SessionSummary, error) {
	return r.querySessions(ctx, userID)
}

func (r *SQLiteRepository) ListAllSessions(ctx context.Context) ([]SessionSummary, error) {
	return r.querySessions(ctx, "")
}

func (r *SQLiteRepository) querySessions(ctx context.Context, userID string) ([]SessionSummary, error) {
	base := `
		SELECT COALESCE(s.id, ''), s.user_id, u.name, u.email, u.role,
		       COALESCE(s.ip_address, ''), COALESCE(s.user_agent, ''),
		       s.last_seen_at, s.expires_at, s.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.expires_at > ?`
	args := []any{time.Now()}
	if strings.TrimSpace(userID) != "" {
		base += " AND s.user_id = ?"
		args = append(args, userID)
	}
	base += " ORDER BY COALESCE(s.last_seen_at, s.created_at) DESC"

	rows, err := r.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SessionSummary, 0)
	for rows.Next() {
		var item SessionSummary
		var lastSeen sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.UserName, &item.UserEmail, &item.UserRole,
			&item.IPAddress, &item.UserAgent, &lastSeen, &item.ExpiresAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			item.LastSeenAt = lastSeen.Time
		} else {
			item.LastSeenAt = item.CreatedAt
		}
		item.Device = DescribeUserAgent(item.UserAgent)
		out = append(out, item)
	}
	return out, rows.Err()
}

// DeleteSessionByID revokes a session by surrogate ID. When ownerUserID is
// non-empty, ownership is strictly enforced in the SQL predicate (OWASP A01).
func (r *SQLiteRepository) DeleteSessionByID(ctx context.Context, sessionID, ownerUserID string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" {
		return false, nil
	}
	var res sql.Result
	var err error
	if strings.TrimSpace(ownerUserID) != "" {
		res, err = r.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ? AND user_id = ?", sessionID, ownerUserID)
	} else {
		res, err = r.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID)
	}
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MigrateSessionIDs ensures every pre-existing session row has a unique surrogate id.
func (r *SQLiteRepository) MigrateSessionIDs(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT token FROM sessions WHERE id IS NULL OR id = ''")
	if err != nil {
		return 0, err
	}
	var tokens []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err == nil {
			tokens = append(tokens, tok)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	updated := 0
	for _, tok := range tokens {
		sid, err := GenerateRandomToken(12)
		if err != nil {
			return updated, err
		}
		res, err := r.db.ExecContext(ctx, "UPDATE sessions SET id = ? WHERE token = ? AND (id IS NULL OR id = '')", "ses_"+sid, tok)
		if err != nil {
			return updated, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			updated += int(n)
		}
	}
	return updated, nil
}

// MigrateSessionTokens rewrites plaintext tokens to hashed form.
// Idempotent: rows already carrying the prefix are skipped.
func (r *SQLiteRepository) MigrateSessionTokens(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT token FROM sessions")
	if err != nil {
		return 0, err
	}
	var tokens []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			rows.Close()
			return 0, err
		}
		tokens = append(tokens, tok)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	migrated := 0
	for _, tok := range tokens {
		if strings.HasPrefix(tok, sessionTokenPrefix) {
			continue
		}
		res, err := r.db.ExecContext(ctx, "UPDATE sessions SET token = ? WHERE token = ?", hashSessionToken(tok), tok)
		if err != nil {
			return migrated, err
		}
		// Count affected rows, not scanned rows: duplicate plaintext tokens
		// (the UPDATE rewrites every row with that value) must not be
		// double-counted.
		if n, _ := res.RowsAffected(); n > 0 {
			migrated += int(n)
		}
	}
	return migrated, nil
}
