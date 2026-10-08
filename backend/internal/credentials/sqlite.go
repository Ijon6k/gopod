package credentials

import (
	"context"
	"database/sql"

	"gopod/pkg/crypto"
)

// SQLiteRepository implements credentials.Repository using SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository for credentials.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// ── SSH Keys ──

func (r *SQLiteRepository) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, public_key, private_key, fingerprint, type, created_at FROM ssh_keys ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SSHKey
	for rows.Next() {
		var k SSHKey
		if err := rows.Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.Type, &k.CreatedAt); err != nil {
			return nil, err
		}
		if dec, err := crypto.Decrypt(k.PrivateKey); err == nil {
			k.PrivateKey = dec
		}
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []SSHKey{}
	}
	return keys, rows.Err()
}

func (r *SQLiteRepository) GetSSHKey(ctx context.Context, id string) (*SSHKey, error) {
	var k SSHKey
	err := r.db.QueryRowContext(ctx, "SELECT id, name, public_key, private_key, fingerprint, type, created_at FROM ssh_keys WHERE id = ?", id).
		Scan(&k.ID, &k.Name, &k.PublicKey, &k.PrivateKey, &k.Fingerprint, &k.Type, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dec, err := crypto.Decrypt(k.PrivateKey); err == nil {
		k.PrivateKey = dec
	}
	return &k, nil
}

func (r *SQLiteRepository) CreateSSHKey(ctx context.Context, k SSHKey) error {
	encKey, err := crypto.Encrypt(k.PrivateKey)
	if err != nil {
		encKey = k.PrivateKey
	}
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO ssh_keys (id, name, public_key, private_key, fingerprint, type) VALUES (?, ?, ?, ?, ?, ?)",
		k.ID, k.Name, k.PublicKey, encKey, k.Fingerprint, k.Type,
	)
	return err
}

func (r *SQLiteRepository) DeleteSSHKey(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ssh_keys WHERE id = ?", id)
	return err
}

// ── Registries ──

func (r *SQLiteRepository) ListRegistries(ctx context.Context) ([]ContainerRegistry, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, url, username, token, created_at FROM container_registries ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ContainerRegistry
	for rows.Next() {
		var reg ContainerRegistry
		if err := rows.Scan(&reg.ID, &reg.Name, &reg.URL, &reg.Username, &reg.Token, &reg.CreatedAt); err != nil {
			return nil, err
		}
		if dec, err := crypto.Decrypt(reg.Token); err == nil {
			reg.Token = dec
		}
		list = append(list, reg)
	}
	if list == nil {
		list = []ContainerRegistry{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) GetRegistry(ctx context.Context, id string) (*ContainerRegistry, error) {
	var reg ContainerRegistry
	err := r.db.QueryRowContext(ctx, "SELECT id, name, url, username, token, created_at FROM container_registries WHERE id = ?", id).
		Scan(&reg.ID, &reg.Name, &reg.URL, &reg.Username, &reg.Token, &reg.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dec, err := crypto.Decrypt(reg.Token); err == nil {
		reg.Token = dec
	}
	return &reg, nil
}

func (r *SQLiteRepository) CreateRegistry(ctx context.Context, reg ContainerRegistry) error {
	encToken, err := crypto.Encrypt(reg.Token)
	if err != nil {
		encToken = reg.Token
	}
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO container_registries (id, name, url, username, token) VALUES (?, ?, ?, ?, ?)",
		reg.ID, reg.Name, reg.URL, reg.Username, encToken,
	)
	return err
}

func (r *SQLiteRepository) DeleteRegistry(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM container_registries WHERE id = ?", id)
	return err
}

// ── Secrets ──

func (r *SQLiteRepository) ListSecrets(ctx context.Context) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, value, driver, created_at FROM secrets ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.Name, &s.Value, &s.Driver, &s.CreatedAt); err != nil {
			return nil, err
		}
		if dec, err := crypto.Decrypt(s.Value); err == nil {
			s.Value = dec
		}
		list = append(list, s)
	}
	if list == nil {
		list = []Secret{}
	}
	return list, rows.Err()
}

func (r *SQLiteRepository) GetSecret(ctx context.Context, id string) (*Secret, error) {
	var s Secret
	err := r.db.QueryRowContext(ctx, "SELECT id, name, value, driver, created_at FROM secrets WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Value, &s.Driver, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dec, err := crypto.Decrypt(s.Value); err == nil {
		s.Value = dec
	}
	return &s, nil
}

func (r *SQLiteRepository) CreateSecret(ctx context.Context, s Secret) error {
	encVal, err := crypto.Encrypt(s.Value)
	if err != nil {
		encVal = s.Value
	}
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO secrets (id, name, value, driver) VALUES (?, ?, ?, ?)",
		s.ID, s.Name, encVal, s.Driver,
	)
	return err
}

func (r *SQLiteRepository) DeleteSecret(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?", id)
	return err
}

