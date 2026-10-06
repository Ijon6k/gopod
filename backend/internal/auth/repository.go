package auth

import "context"

// Repository defines data access contract for users and sessions.
type Repository interface {
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	CreateUser(ctx context.Context, user User) error
	CountUsers(ctx context.Context) (int, error)
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, token string) (*Session, *User, error)
	DeleteSession(ctx context.Context, token string) error
}
