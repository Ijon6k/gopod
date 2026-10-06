package credentials

import "time"

// SSHKey represents an SSH deploy key for Git repositories.
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"publicKey"`
	PrivateKey  string    `json:"privateKey,omitempty"`
	Fingerprint string    `json:"fingerprint"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ContainerRegistry represents credentials for pulling/pushing private container images.
type ContainerRegistry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Username  string    `json:"username"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Secret represents a Podman secret.
type Secret struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Value     string    `json:"value,omitempty"`
	Driver    string    `json:"driver"`
	CreatedAt time.Time `json:"createdAt"`
}
