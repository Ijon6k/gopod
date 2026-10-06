package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Service encapsulates storage backups and snapshot lifecycle business logic.
type Service struct {
	repo Repository
}

// NewService creates a new storage domain service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ── Snapshots ──

func (s *Service) ListSnapshots(ctx context.Context, projectID string) ([]VolumeSnapshot, error) {
	if s.repo == nil {
		return []VolumeSnapshot{}, nil
	}
	return s.repo.ListSnapshots(ctx, projectID)
}

func (s *Service) CreateSnapshot(ctx context.Context, snap VolumeSnapshot) (*VolumeSnapshot, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if snap.VolumeName == "" {
		return nil, errors.New("volumeName is required")
	}

	if snap.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		snap.ID = fmt.Sprintf("snap-%s", hex.EncodeToString(b))
	}
	if snap.Filename == "" {
		snap.Filename = fmt.Sprintf("%s-%d.tar.zst", snap.VolumeName, time.Now().Unix())
	}
	if snap.Size == "" {
		snap.Size = "Pending"
	}
	if snap.Status == "" {
		snap.Status = "completed"
	}
	if snap.Compression == "" {
		snap.Compression = "zstd"
	}
	snap.CreatedAt = time.Now()

	if err := s.repo.CreateSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteSnapshot(ctx, id)
}

// ── Schedules ──

func (s *Service) ListSchedules(ctx context.Context, projectID string) ([]VolumeSchedule, error) {
	if s.repo == nil {
		return []VolumeSchedule{}, nil
	}
	return s.repo.ListSchedules(ctx, projectID)
}

func (s *Service) CreateSchedule(ctx context.Context, sched VolumeSchedule) (*VolumeSchedule, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if sched.VolumeName == "" || sched.Cron == "" {
		return nil, errors.New("volumeName and cron expression are required")
	}

	if sched.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		sched.ID = fmt.Sprintf("sched-%s", hex.EncodeToString(b))
	}
	if sched.RetentionCount <= 0 {
		sched.RetentionCount = 7
	}
	sched.CreatedAt = time.Now()

	if err := s.repo.CreateSchedule(ctx, sched); err != nil {
		return nil, err
	}
	return &sched, nil
}

func (s *Service) DeleteSchedule(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteSchedule(ctx, id)
}
