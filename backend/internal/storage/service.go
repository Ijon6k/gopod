package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if snap.Compression == "" {
		snap.Compression = "zstd"
	}
	snap.CreatedAt = time.Now()

	// Perform real Podman volume export
	backupDir := "./data/backups"
	_ = os.MkdirAll(backupDir, 0755)
	filePath := filepath.Join(backupDir, snap.Filename)

	outFile, err := os.Create(filePath)
	if err == nil {
		cmd := exec.CommandContext(ctx, "podman", "volume", "export", snap.VolumeName)
		cmd.Stdout = outFile
		if exportErr := cmd.Run(); exportErr == nil {
			_ = outFile.Close()
			if fi, statErr := os.Stat(filePath); statErr == nil {
				snap.SizeBytes = fi.Size()
				snap.Size = fmt.Sprintf("%.2f MB", float64(fi.Size())/(1024*1024))
				snap.Status = "completed"
			} else {
				snap.Size = "0 MB"
				snap.Status = "completed"
			}
		} else {
			_ = outFile.Close()
			_ = os.Remove(filePath)
			snap.Size = "0 MB"
			snap.Status = "failed"
		}
	} else {
		snap.Size = "0 MB"
		snap.Status = "failed"
	}

	if err := s.repo.CreateSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	if snap, _ := s.repo.GetSnapshot(ctx, id); snap != nil && snap.Filename != "" {
		_ = os.Remove(filepath.Join("./data/backups", snap.Filename))
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
	if sched.VolumeName == "" {
		return nil, errors.New("volumeName is required")
	}

	if sched.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		sched.ID = fmt.Sprintf("sched-%s", hex.EncodeToString(b))
	}
	if sched.Cron == "" {
		sched.Cron = "0 2 * * *"
	}
	if sched.Label == "" {
		sched.Label = "Daily at 02:00"
	}
	if sched.RetentionCount == 0 {
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

func (s *Service) ToggleSchedule(ctx context.Context, id string) (*VolumeSchedule, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	sched, err := s.repo.GetSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if sched == nil {
		return nil, errors.New("schedule not found")
	}

	sched.Enabled = !sched.Enabled
	if err := s.repo.UpdateSchedule(ctx, *sched); err != nil {
		return nil, err
	}
	return sched, nil
}

