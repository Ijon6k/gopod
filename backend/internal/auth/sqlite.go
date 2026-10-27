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

func (r *SQLiteRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, role, created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *SQLiteRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, role, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *SQLiteRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	return err
}

func (r *SQLiteRepository) CreateUser(ctx context.Context, u User) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password_hash, name, role) VALUES (?, ?, ?, ?, ?)",
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role,
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
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		hashSessionToken(s.Token), s.UserID, s.ExpiresAt,
	)
	return err
}

func (r *SQLiteRepository) getSessionByStoredToken(ctx context.Context, stored string) (*Session, *User, error) {
	var s Session
	var u User
	query := `
		SELECT s.token, s.user_id, s.expires_at, s.created_at,
		       u.id, u.email, u.name, u.role, u.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?
	`
	err := r.db.QueryRowContext(ctx, query, stored, time.Now()).
		Scan(&s.Token, &s.UserID, &s.ExpiresAt, &s.CreatedAt,
			&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
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
