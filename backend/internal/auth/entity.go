package auth

import "time"

// User represents an administrator or team member.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Session represents an authenticated web session.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// LoginRequest defines credentials payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SetupRequest defines initial admin account creation payload.
type SetupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// AuthResponse returns session token and sanitized user details.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
