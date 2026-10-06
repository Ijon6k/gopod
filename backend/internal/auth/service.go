package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AuditRecorder records security and operational audit logs.
type AuditRecorder interface {
	Record(ctx context.Context, action, actor, target, category, ip, status string) error
}

// Service encapsulates authentication and user session business logic.
type Service struct {
	repo  Repository
	audit AuditRecorder
}

// NewService creates a new auth domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetAuditRecorder configures audit recording.
func (s *Service) SetAuditRecorder(ar AuditRecorder) {
	s.audit = ar
}

// GenerateRandomToken generates cryptographically secure hex tokens.
func GenerateRandomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ExtractToken retrieves session token from Cookie, Bearer header, or query param.
func ExtractToken(r *http.Request) string {
	if cookie, err := r.Cookie("gopod_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

// GetSessionUser retrieves the active session and associated user.
func (s *Service) GetSessionUser(r *http.Request) (*Session, *User, error) {
	if s.repo == nil {
		return nil, nil, errors.New("repository not initialized")
	}
	token := ExtractToken(r)
	if token == "" {
		return nil, nil, nil
	}
	return s.repo.GetSession(r.Context(), token)
}

// CountUsers returns the total count of registered users.
func (s *Service) CountUsers(ctx context.Context) (int, error) {
	if s.repo == nil {
		return 0, errors.New("repository not initialized")
	}
	return s.repo.CountUsers(ctx)
}

// SetupAdmin creates the initial administrator user and session.
func (s *Service) SetupAdmin(ctx context.Context, name, email, password string) (*User, string, error) {
	count, err := s.CountUsers(ctx)
	if err != nil {
		return nil, "", err
	}
	if count > 0 {
		return nil, "", errors.New("cluster is already initialized")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	randHex, _ := GenerateRandomToken(8)
	user := User{
		ID:           "usr_" + randHex,
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		Role:         "admin",
		CreatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, "", err
	}

	token, err := GenerateRandomToken(32)
	if err != nil {
		return nil, "", err
	}

	session := Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// Login verifies user credentials and creates a new authenticated session.
func (s *Service) Login(ctx context.Context, email, password string) (*User, string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("invalid email or password")
	}

	token, err := GenerateRandomToken(32)
	if err != nil {
		return nil, "", err
	}

	session := Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, "", err
	}

	if s.audit != nil {
		_ = s.audit.Record(ctx, "user.login", user.Email, "auth.session", "security", "", "success")
	}

	return user, token, nil
}

// Logout deletes an active session.
func (s *Service) Logout(ctx context.Context, token string) error {
	if s.repo == nil || token == "" {
		return nil
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "user.logout", "authenticated-user", "auth.session", "security", "", "success")
	}
	return s.repo.DeleteSession(ctx, token)
}
