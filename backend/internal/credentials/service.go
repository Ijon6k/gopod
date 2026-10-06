package credentials

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Service encapsulates credentials management business logic.
type Service struct {
	repo Repository
}

// NewService creates a new credentials domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ── SSH Keys ──

func (s *Service) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	if s.repo == nil {
		return []SSHKey{}, nil
	}
	keys, err := s.repo.ListSSHKeys(ctx)
	if err != nil {
		return nil, err
	}
	// Mask private keys in listing response
	sanitized := make([]SSHKey, len(keys))
	for i, k := range keys {
		k.PrivateKey = ""
		sanitized[i] = k
	}
	return sanitized, nil
}

func (s *Service) CreateSSHKey(ctx context.Context, k SSHKey) (*SSHKey, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if strings.TrimSpace(k.Name) == "" {
		return nil, errors.New("key name is required")
	}

	if k.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		k.ID = fmt.Sprintf("key-%s", hex.EncodeToString(b))
	}
	if k.Fingerprint == "" && k.PublicKey != "" {
		hash := sha256.Sum256([]byte(k.PublicKey))
		k.Fingerprint = "SHA256:" + hex.EncodeToString(hash[:16])
	}
	if k.Type == "" {
		k.Type = "ed25519"
	}
	k.CreatedAt = time.Now()

	if err := s.repo.CreateSSHKey(ctx, k); err != nil {
		return nil, err
	}
	// Do not return raw private key in response
	k.PrivateKey = ""
	return &k, nil
}

func (s *Service) DeleteSSHKey(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteSSHKey(ctx, id)
}

// GetPrivateKey retrieves raw unmasked private key for Git authentication.
func (s *Service) GetPrivateKey(ctx context.Context, id string) (string, error) {
	if s.repo == nil {
		return "", errors.New("repository not initialized")
	}
	k, err := s.repo.GetSSHKey(ctx, id)
	if err != nil {
		return "", err
	}
	if k == nil {
		return "", errors.New("ssh key not found")
	}
	return k.PrivateKey, nil
}

// ── Registries ──

func (s *Service) ListRegistries(ctx context.Context) ([]ContainerRegistry, error) {
	if s.repo == nil {
		return []ContainerRegistry{}, nil
	}
	registries, err := s.repo.ListRegistries(ctx)
	if err != nil {
		return nil, err
	}
	sanitized := make([]ContainerRegistry, len(registries))
	for i, r := range registries {
		r.Token = ""
		sanitized[i] = r
	}
	return sanitized, nil
}

func (s *Service) CreateRegistry(ctx context.Context, r ContainerRegistry) (*ContainerRegistry, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.URL) == "" {
		return nil, errors.New("registry name and URL are required")
	}

	if r.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		r.ID = fmt.Sprintf("reg-%s", hex.EncodeToString(b))
	}
	r.CreatedAt = time.Now()

	if err := s.repo.CreateRegistry(ctx, r); err != nil {
		return nil, err
	}
	r.Token = ""
	return &r, nil
}

func (s *Service) DeleteRegistry(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteRegistry(ctx, id)
}

// ── Secrets ──

func (s *Service) ListSecrets(ctx context.Context) ([]Secret, error) {
	if s.repo == nil {
		return []Secret{}, nil
	}
	secrets, err := s.repo.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	sanitized := make([]Secret, len(secrets))
	for i, sec := range secrets {
		sec.Value = ""
		sanitized[i] = sec
	}
	return sanitized, nil
}

func (s *Service) CreateSecret(ctx context.Context, sec Secret) (*Secret, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if strings.TrimSpace(sec.Name) == "" || strings.TrimSpace(sec.Value) == "" {
		return nil, errors.New("secret name and value are required")
	}

	if sec.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		sec.ID = fmt.Sprintf("sec-%s", hex.EncodeToString(b))
	}
	if sec.Driver == "" {
		sec.Driver = "file"
	}
	sec.CreatedAt = time.Now()

	if err := s.repo.CreateSecret(ctx, sec); err != nil {
		return nil, err
	}
	sec.Value = ""
	return &sec, nil
}

func (s *Service) DeleteSecret(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteSecret(ctx, id)
}
