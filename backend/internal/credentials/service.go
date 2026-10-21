package credentials

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
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

// GenerateSSHKeyPair generates a real OpenSSH Ed25519 keypair and SHA256 fingerprint.
func (s *Service) GenerateSSHKeyPair(ctx context.Context, name string) (*SSHKey, error) {
	if strings.TrimSpace(name) == "" {
		name = "deploy-key"
	}
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh public key: %w", err)
	}

	fingerprint := ssh.FingerprintSHA256(sshPub)
	pubKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + fmt.Sprintf(" %s@gopod", name)

	// Format private key into OpenSSH PEM block
	privPemBlock, err := ssh.MarshalPrivateKey(privKey, "gopod-generated-key")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privKeyPEM := string(pem.EncodeToMemory(privPemBlock))

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	keyID := fmt.Sprintf("key-%s", hex.EncodeToString(b))

	key := &SSHKey{
		ID:          keyID,
		Name:        name,
		Type:        "ed25519",
		PublicKey:   pubKeyStr,
		PrivateKey:  privKeyPEM,
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}

	return key, nil
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

// ── Git tokens ──

func (s *Service) ListGitTokens(ctx context.Context) ([]GitToken, error) {
	if s.repo == nil {
		return []GitToken{}, nil
	}
	tokens, err := s.repo.ListGitTokens(ctx)
	if err != nil {
		return nil, err
	}
	sanitized := make([]GitToken, len(tokens))
	for i, t := range tokens {
		t.Token = ""
		sanitized[i] = t
	}
	return sanitized, nil
}

func (s *Service) CreateGitToken(ctx context.Context, t GitToken) (*GitToken, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if strings.TrimSpace(t.Name) == "" {
		return nil, errors.New("token name is required")
	}
	if strings.TrimSpace(t.Token) == "" {
		return nil, errors.New("access token is required")
	}
	t.Provider = strings.ToLower(strings.TrimSpace(t.Provider))
	switch t.Provider {
	case "github":
		if strings.TrimSpace(t.Username) == "" {
			t.Username = "x-access-token"
		}
	case "gitlab", "gitea":
		if strings.TrimSpace(t.Username) == "" {
			t.Username = "oauth2"
		}
	case "custom":
		if strings.TrimSpace(t.Username) == "" {
			return nil, errors.New("username is required for a custom Git provider")
		}
	case "":
		return nil, errors.New("Git provider is required")
	default:
		return nil, fmt.Errorf("unsupported Git provider %q", t.Provider)
	}
	t.Name = strings.TrimSpace(t.Name)
	t.Username = strings.TrimSpace(t.Username)
	t.Token = strings.TrimSpace(t.Token)

	if t.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		t.ID = fmt.Sprintf("git-%s", hex.EncodeToString(b))
	}
	t.CreatedAt = time.Now()

	if err := s.repo.CreateGitToken(ctx, t); err != nil {
		return nil, err
	}
	t.Token = ""
	return &t, nil
}

func (s *Service) DeleteGitToken(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteGitToken(ctx, id)
}

// GetGitToken retrieves the unmasked token and its clone identity.
func (s *Service) GetGitToken(ctx context.Context, id string) (*GitToken, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	t, err := s.repo.GetGitToken(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errors.New("git token not found")
	}
	return t, nil
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

// GetRegistry returns credentials for a single registry including unredacted token.
func (s *Service) GetRegistry(ctx context.Context, id string) (*ContainerRegistry, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	return s.repo.GetRegistry(ctx, id)
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

	// Real Podman secret creation
	if _, err := exec.LookPath("podman"); err == nil {
		cmd := exec.CommandContext(ctx, "podman", "secret", "create", sec.Name, "-")
		cmd.Stdin = strings.NewReader(sec.Value)
		_ = cmd.Run()
	}

	sec.Value = ""
	return &sec, nil
}

func (s *Service) DeleteSecret(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	sec, _ := s.repo.GetSecret(ctx, id)
	if sec != nil && sec.Name != "" {
		if _, err := exec.LookPath("podman"); err == nil {
			_ = exec.CommandContext(ctx, "podman", "secret", "rm", sec.Name).Run()
		}
	}
	return s.repo.DeleteSecret(ctx, id)
}
