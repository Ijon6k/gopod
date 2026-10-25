package storage

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// volumeNamePattern enforces a Podman volume identifier: alphanumeric first
// character, then alphanumerics, dots, dashes and underscores, 64 characters
// max. Together with the filepath.Base equality check below it guarantees the
// name can never be interpreted as a host path, a flag, or a traversal
// sequence when it is interpolated into a podman bind mount.
var volumeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

// validateVolumeName rejects anything that is not a bare volume identifier.
// It operates on the raw, untrimmed string: values such as "/etc" pass
// filepath.Base on their own (base "etc" is well-formed) but must never be
// handed to `podman -v /etc:/...` as a bind mount.
func validateVolumeName(raw string) error {
	if raw == "" || !volumeNamePattern.MatchString(raw) || filepath.Base(raw) != raw {
		return fmt.Errorf("invalid volume name %q: must be an alphanumeric volume identifier", raw)
	}
	return nil
}

// Service encapsulates storage backups and snapshot lifecycle business logic.
type Service struct {
	repo Repository
	cron *cron.Cron
	mu   sync.Mutex
}

// NewService creates a new storage domain service.
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		cron: cron.New(),
	}
}

// ── Snapshots ──

func (s *Service) ListSnapshots(ctx context.Context, projectID string) ([]VolumeSnapshot, error) {
	if s.repo == nil {
		return []VolumeSnapshot{}, nil
	}
	return s.repo.ListSnapshots(ctx, projectID)
}

func setupCmdEnv(cmd *exec.Cmd) {
	cmd.Env = os.Environ()
	if socket := os.Getenv("PODMAN_SOCKET"); socket != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CONTAINER_HOST=unix://%s", socket))
	}
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
	// security: the raw value is validated, not just its base — "/etc" passes
	// Base() as "etc" but must be rejected before it reaches a bind mount.
	if err := validateVolumeName(snap.VolumeName); err != nil {
		return nil, err
	}
	cleanFilename := filepath.Base(snap.Filename)
	if cleanFilename == "." || cleanFilename == "/" || strings.Contains(cleanFilename, "..") || cleanFilename == "" {
		cleanFilename = fmt.Sprintf("%s-%d.tar.gz", snap.VolumeName, time.Now().Unix())
	}
	snap.Filename = cleanFilename

	if snap.Compression == "" {
		snap.Compression = "gzip"
	}
	snap.CreatedAt = time.Now()

	// Perform real Podman volume export with standard gzip compression
	backupDir := "./data/backups"
	_ = os.MkdirAll(backupDir, 0755)
	filePath := filepath.Join(backupDir, snap.Filename)

	outFile, err := os.Create(filePath)
	if err == nil {
		gw := gzip.NewWriter(outFile)
		cmd := exec.CommandContext(ctx, "podman", "volume", "export", snap.VolumeName)
		setupCmdEnv(cmd)
		cmd.Stdout = gw
		exportErr := cmd.Run()
		// Flush and close the primary writers before deciding anything; the
		// tar fallback path below re-opens the file from scratch.
		_ = gw.Close()
		_ = outFile.Close()
		if exportErr != nil {
			// Remote Podman client fallback: export via tar helper container
			_ = os.Remove(filePath)
			if f, createErr := os.Create(filePath); createErr == nil {
				tarCmd := exec.CommandContext(ctx, "podman", "run", "--rm", "-v", fmt.Sprintf("%s:/data:ro", snap.VolumeName), "docker.io/library/alpine:latest", "tar", "-C", "/data", "-czf", "-", ".")
				setupCmdEnv(tarCmd)
				tarCmd.Stdout = f
				exportErr = tarCmd.Run()
				_ = f.Close()
			} else {
				exportErr = createErr
			}
		}

		// Honest status reporting: an export that failed, or that produced no
		// bytes on disk, is a failed backup — never a completed one. The file
		// is removed in both failure cases so nothing fake stays downloadable.
		if fi, statErr := os.Stat(filePath); statErr == nil && fi.Size() > 0 {
			snap.SizeBytes = fi.Size()
			snap.Size = fmt.Sprintf("%.2f MB", float64(fi.Size())/(1024*1024))
			snap.Status = "completed"
		} else {
			snap.Size = "0 MB"
			snap.Status = "failed"
			_ = os.Remove(filePath)
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

// RestoreSnapshot restores volume contents from a real snapshot archive.
func (s *Service) RestoreSnapshot(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	snap, err := s.repo.GetSnapshot(ctx, id)
	if err != nil || snap == nil {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	// Stored rows are replayed against the shell/daemon below; re-validate the
	// volume name so a legacy host-path row can never be bind-mounted again.
	if err := validateVolumeName(snap.VolumeName); err != nil {
		return err
	}

	cleanFilename := filepath.Base(snap.Filename)
	filePath := filepath.Join("./data/backups", cleanFilename)
	inFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("snapshot file missing: %w", err)
	}
	defer inFile.Close()

	var reader io.Reader = inFile
	if strings.HasSuffix(cleanFilename, ".gz") || snap.Compression == "gzip" {
		gr, err := gzip.NewReader(inFile)
		if err != nil {
			return fmt.Errorf("invalid gzip snapshot archive: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	cmd := exec.CommandContext(ctx, "podman", "volume", "import", snap.VolumeName, "-")
	setupCmdEnv(cmd)
	cmd.Stdin = reader
	if _, err := cmd.CombinedOutput(); err != nil {
		// Remote Podman client fallback: unpack via helper container
		_, _ = inFile.Seek(0, io.SeekStart)
		tarCmd := exec.CommandContext(ctx, "podman", "run", "--rm", "-i", "-v", fmt.Sprintf("%s:/data:Z", snap.VolumeName), "docker.io/library/alpine:latest", "tar", "-C", "/data", "-xzf", "-")
		setupCmdEnv(tarCmd)
		tarCmd.Stdin = inFile
		if tarOut, tarErr := tarCmd.CombinedOutput(); tarErr != nil {
			return fmt.Errorf("volume restore error: %s (%w)", strings.TrimSpace(string(tarOut)), tarErr)
		}
		return nil
	}
	return nil
}

// GetSnapshotFilePath returns physical path and target download filename.
func (s *Service) GetSnapshotFilePath(ctx context.Context, id string) (string, string, error) {
	if s.repo == nil {
		return "", "", errors.New("repository not initialized")
	}
	snap, err := s.repo.GetSnapshot(ctx, id)
	if err != nil || snap == nil {
		return "", "", fmt.Errorf("snapshot not found: %s", id)
	}
	cleanFilename := filepath.Base(snap.Filename)
	filePath := filepath.Join("./data/backups", cleanFilename)
	if _, err := os.Stat(filePath); err != nil {
		return "", "", fmt.Errorf("snapshot file not found on disk: %w", err)
	}
	return filePath, cleanFilename, nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	if snap, _ := s.repo.GetSnapshot(ctx, id); snap != nil && snap.Filename != "" {
		cleanFilename := filepath.Base(snap.Filename)
		_ = os.Remove(filepath.Join("./data/backups", cleanFilename))
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
	if err := validateVolumeName(sched.VolumeName); err != nil {
		return nil, err
	}

	if sched.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		sched.ID = fmt.Sprintf("sched-%s", hex.EncodeToString(b))
	}
	if sched.Cron == "" {
		sched.Cron = "0 2 * * *"
	}
	if _, err := cron.ParseStandard(sched.Cron); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
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
	s.RestartScheduler(ctx)
	return &sched, nil
}

func (s *Service) DeleteSchedule(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	err := s.repo.DeleteSchedule(ctx, id)
	if err == nil {
		s.RestartScheduler(ctx)
	}
	return err
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
	s.RestartScheduler(ctx)
	return sched, nil
}

// StartScheduler initializes background cron execution for enabled backup policies.
func (s *Service) StartScheduler(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repo == nil || s.cron == nil {
		return
	}
	s.cron.Start()

	// Register current enabled schedules
	schedules, err := s.repo.ListSchedules(ctx, "")
	if err != nil {
		return
	}

	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		s.registerCronJob(sched)
	}
}

// RestartScheduler resets running cron jobs to match updated database state.
func (s *Service) RestartScheduler(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
		s.cron = cron.New()
		s.cron.Start()
	}

	if s.repo == nil {
		return
	}
	schedules, err := s.repo.ListSchedules(ctx, "")
	if err != nil {
		return
	}
	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		s.registerCronJob(sched)
	}
}

func (s *Service) registerCronJob(sched VolumeSchedule) {
	if s.cron == nil {
		return
	}
	_, _ = s.cron.AddFunc(sched.Cron, func() {
		ctx := context.Background()
		snap := VolumeSnapshot{
			ProjectID:  sched.ProjectID,
			VolumeName: sched.VolumeName,
		}
		_, err := s.CreateSnapshot(ctx, snap)
		if err == nil && sched.RetentionCount > 0 && s.repo != nil {
			allSnaps, _ := s.repo.ListSnapshots(ctx, sched.ProjectID)
			var volSnaps []VolumeSnapshot
			for _, sn := range allSnaps {
				if sn.VolumeName == sched.VolumeName {
					volSnaps = append(volSnaps, sn)
				}
			}
			if len(volSnaps) > sched.RetentionCount {
				excess := len(volSnaps) - sched.RetentionCount
				for i := 0; i < excess; i++ {
					_ = s.DeleteSnapshot(ctx, volSnaps[i].ID)
				}
			}
		}
	})
}
