package storage

import "time"

// VolumeSnapshot represents a .tar.zst archive of a named volume.
type VolumeSnapshot struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	ServiceID   string    `json:"serviceId"`
	VolumeName  string    `json:"volumeName"`
	Filename    string    `json:"filename"`
	Size        string    `json:"size"`
	SizeBytes   int64     `json:"sizeBytes"`
	Status      string    `json:"status"`
	Compression string    `json:"compression"`
	CreatedAt   time.Time `json:"createdAt"`

	// Remote mirror state (S3-compatible destination). Empty RemoteStatus
	// means the snapshot was never pushed; "local-only" records a failed or
	// skipped push so operators can see that the only copy today lives on
	// this host.
	RemoteStatus        string `json:"remoteStatus,omitempty"`
	RemoteProvider      string `json:"remoteProvider,omitempty"`
	RemoteDestinationID string `json:"remoteDestinationId,omitempty"`
	RemoteKey           string `json:"remoteKey,omitempty"`
	Checksum            string `json:"checksum,omitempty"`
	RemoteError         string `json:"remoteError,omitempty"`

	// DestinationID is a request-signal only (create-snapshot payload): which
	// destination this snapshot should mirror to. It is never persisted in
	// the snapshot row itself — the outcome is stored in RemoteDestinationID.
	DestinationID string `json:"destinationId,omitempty"`
}

// BackupDestination is a generic S3-compatible object store (AWS S3, Cloudflare
// R2, MinIO, Wasabi, B2, ...). SecretKey is decrypted transiently for API use
// only; at rest it is ciphertext (pkg/crypto).
type BackupDestination struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Region    string    `json:"region"`
	AccessKey string    `json:"accessKey"`
	SecretKey string    `json:"-"`
	Bucket    string    `json:"bucket"`
	Prefix    string    `json:"prefix"`
	PathStyle bool      `json:"pathStyle"`
	CreatedAt time.Time `json:"createdAt"`
}

// VolumeSchedule represents an automated backup cron policy.
type VolumeSchedule struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	ServiceID      string     `json:"serviceId,omitempty"`
	VolumeName     string     `json:"volumeName"`
	Cron           string     `json:"cron"`
	Label          string     `json:"label"`
	RetentionCount int        `json:"retentionCount"`
	Enabled        bool       `json:"enabled"`
	LastRun        *time.Time `json:"lastRun,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	// DestinationID optionally routes schedule-issued snapshots to an
	// S3-compatible mirror. Empty = local-only (hot copy on this host).
	DestinationID string `json:"destinationId,omitempty"`
}

// SnapshotRemoteState is the result of a remote push used for metadata update.
type SnapshotRemoteState struct {
	Status        string // "", "synced", "local-only", "failed"
	Provider      string
	DestinationID string
	RemoteKey     string
	Checksum      string
	ErrText       string
}
