package auth

import (
	"context"
	"database/sql"
	"time"
)

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

func (r *SQLiteRepository) CreateSession(ctx context.Context, s Session) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		s.Token, s.UserID, s.ExpiresAt,
	)
	return err
}

func (r *SQLiteRepository) GetSession(ctx context.Context, token string) (*Session, *User, error) {
	var s Session
	var u User
	query := `
		SELECT s.token, s.user_id, s.expires_at, s.created_at,
		       u.id, u.email, u.name, u.role, u.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > ?
	`
	err := r.db.QueryRowContext(ctx, query, token, time.Now()).
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

func (r *SQLiteRepository) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}
