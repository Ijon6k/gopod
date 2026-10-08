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
}
