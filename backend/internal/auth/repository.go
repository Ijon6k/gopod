package auth

import "context"

// Repository defines data access contract for users and sessions.
type Repository interface {
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	CreateUser(ctx context.Context, user User) error
	CountUsers(ctx context.Context) (int, error)
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, token string) (*Session, *User, error)
	DeleteSession(ctx context.Context, token string) error
	PruneExpiredSessions(ctx context.Context) (int64, error)
	DeleteOtherSessions(ctx context.Context, userID, keepToken string) (int64, error)
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
}
