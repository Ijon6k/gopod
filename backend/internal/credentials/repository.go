package credentials

import "context"

// Repository defines data access contract for SSH keys, registries, and secrets.
type Repository interface {
	ListSSHKeys(ctx context.Context) ([]SSHKey, error)
	GetSSHKey(ctx context.Context, id string) (*SSHKey, error)
	CreateSSHKey(ctx context.Context, k SSHKey) error
	DeleteSSHKey(ctx context.Context, id string) error

	ListRegistries(ctx context.Context) ([]ContainerRegistry, error)
	GetRegistry(ctx context.Context, id string) (*ContainerRegistry, error)
	CreateRegistry(ctx context.Context, r ContainerRegistry) error
	DeleteRegistry(ctx context.Context, id string) error

	ListSecrets(ctx context.Context) ([]Secret, error)
	GetSecret(ctx context.Context, id string) (*Secret, error)
	CreateSecret(ctx context.Context, s Secret) error
	DeleteSecret(ctx context.Context, id string) error
}
