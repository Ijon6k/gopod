package storage

import "context"

// Repository defines data access contract for volume snapshots and schedules.
type Repository interface {
	ListSnapshots(ctx context.Context, projectID string) ([]VolumeSnapshot, error)
	GetSnapshot(ctx context.Context, id string) (*VolumeSnapshot, error)
	CreateSnapshot(ctx context.Context, s VolumeSnapshot) error
	DeleteSnapshot(ctx context.Context, id string) error

	ListSchedules(ctx context.Context, projectID string) ([]VolumeSchedule, error)
	GetSchedule(ctx context.Context, id string) (*VolumeSchedule, error)
	CreateSchedule(ctx context.Context, s VolumeSchedule) error
	UpdateSchedule(ctx context.Context, s VolumeSchedule) error
	DeleteSchedule(ctx context.Context, id string) error

	// Remote destinations (S3-compatible mirrors).
	ListDestinations(ctx context.Context) ([]BackupDestination, error)
	GetDestination(ctx context.Context, id string) (*BackupDestination, error)
	CreateDestination(ctx context.Context, d BackupDestination, secretEnc string) error
	DeleteDestination(ctx context.Context, id string) error

	// UpdateSnapshotRemote records the remote-mirror state of a snapshot.
	UpdateSnapshotRemote(ctx context.Context, id string, result SnapshotRemoteState) error
}

// RedactDestination returns a copy safe for API serialization: the secret
// key is dropped from every read path (write-only field).
func RedactDestination(d BackupDestination) BackupDestination {
	d.SecretKey = ""
	return d
}
