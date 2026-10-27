package storage

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"gopod/internal/activity"
	"gopod/pkg/crypto"
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

// Sentinel errors for precise HTTP status mapping.
var (
	// ErrBackupInProgress reports an overlapping backup creation attempt.
	ErrBackupInProgress = errors.New("a backup operation is already in progress")
	// ErrInconsistentDatabase reports that a transactionally consistent copy of
	// the live database could not be captured. The backup aborts instead of
	// shipping a torn archive under a "completed" status.
	ErrInconsistentDatabase = errors.New("could not capture a consistent database copy")
	// ErrSnapshotNotFound reports a snapshot id that does not resolve.
	ErrSnapshotNotFound = errors.New("snapshot not found")
	// ErrScheduleNotFound reports a schedule id that does not resolve.
	ErrScheduleNotFound = errors.New("schedule not found")
	// ErrSystemRestoreUnsupported rejects restore attempts for instance-level
	// disaster recovery archives; their home is an offline host recovery.
	ErrSystemRestoreUnsupported = errors.New("instance disaster recovery archives cannot be restored into container volumes; download the archive and extract during host recovery")
)

// AuditRecorder records storage and backup audit events.
type AuditRecorder interface {
	Record(ctx context.Context, action, actor, target, category, ip, status string) error
}

// Service encapsulates storage backups and snapshot lifecycle business logic.
type Service struct {
	repo     Repository
	cron     *cron.Cron
	sqlDB    *sql.DB
	dataDir  string
	audit    AuditRecorder
	mu       sync.Mutex
	backupMu sync.Mutex
}

func (s *Service) getDataDir() string {
	if s.dataDir != "" {
		return s.dataDir
	}
	return "./data"
}

// backupDir is the single source of truth for where local archives live.
// Hardcoding "./data/backups" in individual callers desynchronizes writes
// (create) from reads (download/restore/delete) whenever the configured data
// directory differs from the process working directory.
func (s *Service) backupDir() string {
	return filepath.Join(s.getDataDir(), "backups")
}

// SetAuditRecorder configures audit recording for storage lifecycle operations.
func (s *Service) SetAuditRecorder(ar AuditRecorder) {
	s.audit = ar
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
	if snap.VolumeName == "system:gopod" {
		return s.CreateSystemBackup(ctx, snap.DestinationID)
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

	// Perform real Podman volume export with standard gzip compression.
	// Archives round-trip potentially confidential volume data, so both the
	// directory and the files keep owner-only permissions.
	backupDir := s.backupDir()
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("prepare snapshot directory: %w", err)
	}
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
			snap.Size = formatSnapshotSize(fi.Size())
			snap.Status = "completed"
			_ = os.Chmod(filePath, 0600)
		} else {
			snap.Size = formatSnapshotSize(0)
			snap.Status = "failed"
			_ = os.Remove(filePath)
		}

		// Remote mirror push (S3-compatible destination), part of the same
		// operation. PrincFunnyPLES: a remote failure must never destroy the
		// local hot copy — instead the snapshot is marked "local-only" so the
		// operator can see that the off-host copy is missing.
		if snap.Status == "completed" && snap.DestinationID != "" {
			state := s.mirrorSnapshotToDestination(ctx, &snap, filePath)
			snap.RemoteStatus = state.Status
			snap.RemoteProvider = state.Provider
			snap.RemoteKey = state.RemoteKey
			snap.Checksum = state.Checksum
			snap.RemoteError = state.ErrText
		}
	} else {
		snap.Size = formatSnapshotSize(0)
		snap.Status = "failed"
	}

	// The database row is written before the mirror update so a crash between
	// the two steps always leaves a locally recorded snapshot rather than an
	// orphaned remote object with no metadata to clean it up later.
	if err := s.repo.CreateSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	if snap.RemoteStatus != "" {
		if err := s.repo.UpdateSnapshotRemote(ctx, snap.ID, SnapshotRemoteState{
			Status:    snap.RemoteStatus,
			Provider:  snap.RemoteProvider,
			RemoteKey: snap.RemoteKey,
			Checksum:  snap.Checksum,
			ErrText:   snap.RemoteError,
		}); err != nil {
			// The local archive and its row are intact; only metadata about the
			// mirror is missing. Operators delete by snapshot id, so silently
			// dropping the error would leave stale remote objects behind.
			log.Printf("[storage] WARNING: remote mirror state not recorded for snapshot %s: %v", snap.ID, err)
		}
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
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}

	if snap.VolumeName == "system:gopod" || snap.ProjectID == "system" {
		return ErrSystemRestoreUnsupported
	}

	// Stored rows are replayed against the shell/daemon below; re-validate the
	// volume name so a legacy host-path row can never be bind-mounted again.
	if err := validateVolumeName(snap.VolumeName); err != nil {
		return err
	}

	cleanFilename := filepath.Base(snap.Filename)
	filePath := filepath.Join(s.getDataDir(), "backups", cleanFilename)
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
		return "", "", fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}
	cleanFilename := filepath.Base(snap.Filename)
	filePath := filepath.Join(s.backupDir(), cleanFilename)
	if _, err := os.Stat(filePath); err != nil {
		return "", "", fmt.Errorf("snapshot file not found on disk: %w", err)
	}
	return filePath, cleanFilename, nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	snap, err := s.repo.GetSnapshot(ctx, id)
	if err != nil {
		return err
	}
	if snap == nil {
		// Only the row-delete side effect may run (a concurrent retention pass
		// may have removed the row moments ago); report the miss accurately so
		// callers can honor 404 semantics instead of a phantom success.
		if deleteErr := s.repo.DeleteSnapshot(ctx, id); deleteErr != nil {
			return deleteErr
		}
		return fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}
	if snap.Filename != "" {
		cleanFilename := filepath.Base(snap.Filename)
		_ = os.Remove(filepath.Join(s.backupDir(), cleanFilename))
	}
	// Best-effort remote cleanup: when the local archive is gone, the mirror
	// should go too. Failure here only logs — the operator is not stranded by
	// an unreachable bucket when trying to clean their history.
	if snap.RemoteStatus == "synced" && snap.RemoteKey != "" && snap.RemoteDestinationID != "" {
		if dest, _ := s.repo.GetDestination(ctx, snap.RemoteDestinationID); dest != nil {
			if err := deleteFromDestination(ctx, *dest, snap.RemoteKey); err != nil {
				fmt.Printf("[storage] mirror cleanup for %s failed: %v (stale remote object kept)\n", snap.RemoteKey, err)
			}
		}
	}
	err = s.repo.DeleteSnapshot(ctx, id)
	if err == nil {
		log.Printf("[backup] Snapshot deleted: %s", id)
		if s.audit != nil {
			_ = s.audit.Record(ctx, "system.backup_delete", "admin", id, "storage", "", "success")
		}
		activity.EmitEngineEvent(ctx, "info", "system", "", "", "admin", "system.backup_delete", "Backup snapshot deleted: "+id, nil)
	}
	return err
}

// pruneSnapshotsByRetention keeps the newest keepCount snapshots and deletes
// the rest. volSnaps must be sorted newest-first (the repository's
// ListSnapshots contract orders by created_at DESC), so the excess-if-any
// lives at the TAIL of the slice — deleting from the head would silently
// destroy the freshest backups this retention run just created.
func (s *Service) pruneSnapshotsByRetention(ctx context.Context, volSnaps []VolumeSnapshot, keepCount int) {
	if s.repo == nil || keepCount <= 0 || len(volSnaps) <= keepCount {
		return
	}
	for _, snap := range volSnaps[keepCount:] {
		_ = s.DeleteSnapshot(ctx, snap.ID)
	}
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
		return nil, fmt.Errorf("%w: %s", ErrScheduleNotFound, id)
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
// The caller's request context is deliberately detached: an HTTP client that
// disconnects right after saving a schedule would otherwise cancel the
// re-registration query and silently leave every backup schedule unregistered
// until the next process restart.
func (s *Service) RestartScheduler(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
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
			ProjectID:     sched.ProjectID,
			VolumeName:    sched.VolumeName,
			DestinationID: sched.DestinationID,
		}
		_, err := s.CreateSnapshot(ctx, snap)
		if err != nil {
			// Scheduled backups run in the background where no one looks at a
			// return value: emit the failure into the engine feed and system
			// log so a broken backup policy cannot fail silently for weeks.
			log.Printf("[backup] scheduled backup failed for volume %s: %v", sched.VolumeName, err)
			if sched.VolumeName != "system:gopod" {
				// Self-backups already emit system.backup_failed inside
				// CreateSystemBackup; only volume backups need this event.
				activity.EmitEngineEvent(ctx, "error", sched.ProjectID, "", sched.VolumeName, "admin", "backup_failed",
					fmt.Sprintf("Scheduled backup for volume %s failed: %v", sched.VolumeName, err), nil)
			}
		} else if sched.RetentionCount > 0 && s.repo != nil {
			allSnaps, _ := s.repo.ListSnapshots(ctx, sched.ProjectID)
			var volSnaps []VolumeSnapshot
			for _, sn := range allSnaps {
				if sn.VolumeName == sched.VolumeName {
					volSnaps = append(volSnaps, sn)
				}
			}
			s.pruneSnapshotsByRetention(ctx, volSnaps, sched.RetentionCount)
		}
	})
}

// ── Destinations (S3-compatible mirrors) ──

// mirrorSnapshotToDestination pushes a completed local snapshot to its
// configured S3-compatible destination and records the outcome. Semantics are
// "local snapshot survives": a remote failure never removes the local archive
// — it downgrades the snapshot to "local-only" so the operator can see that
// the off-host copy is missing.
func (s *Service) mirrorSnapshotToDestination(ctx context.Context, snap *VolumeSnapshot, filePath string) SnapshotRemoteState {
	base := SnapshotRemoteState{
		Provider:      "s3",
		DestinationID: snap.DestinationID,
	}
	if s.repo == nil {
		base.Status = "local-only"
		base.ErrText = "repository unavailable"
		return base
	}
	dest, err := s.repo.GetDestination(ctx, snap.DestinationID)
	if err != nil || dest == nil {
		base.Status = "local-only"
		base.ErrText = "destination not found"
		return base
	}
	key := remoteKeyFor(*dest, *snap)
	size, checksum, err := uploadToDestination(ctx, *dest, filePath, key)
	if err != nil {
		base.Status = "local-only"
		base.ErrText = err.Error()
		return base
	}
	base.Status = "synced"
	base.RemoteKey = key
	base.Checksum = checksum
	fmt.Printf("[storage] mirrored %s (%s, %d bytes) to destination %q at %s\n",
		snap.Filename, dest.Name, size, dest.ID, key)
	return base
}

// ListDestinations returns every configured S3-compatible destination with
// its secret stripped (write-only field).
func (s *Service) ListDestinations(ctx context.Context) ([]BackupDestination, error) {
	if s.repo == nil {
		return []BackupDestination{}, nil
	}
	dests, err := s.repo.ListDestinations(ctx)
	if err != nil {
		return nil, err
	}
	for i := range dests {
		dests[i] = RedactDestination(dests[i])
	}
	return dests, nil
}

// CreateDestination validates and stores a new destination. The secret key is
// encrypted at rest with the master key (pkg/crypto) before it reaches the
// data layer; every read path redacts it.
func (s *Service) CreateDestination(ctx context.Context, d BackupDestination) (*BackupDestination, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	d.Name = strings.TrimSpace(d.Name)
	d.Endpoint = strings.TrimSpace(d.Endpoint)
	d.Bucket = strings.TrimSpace(d.Bucket)
	d.Prefix = strings.Trim(strings.TrimSpace(d.Prefix), "/")
	d.Region = strings.TrimSpace(d.Region)
	d.Endpoint = strings.TrimSuffix(d.Endpoint, "/")
	if d.Name == "" {
		return nil, errors.New("name is required")
	}
	if d.Endpoint == "" || d.Bucket == "" || d.AccessKey == "" || d.SecretKey == "" {
		return nil, errors.New("endpoint, bucket, accessKey and secretKey are required")
	}
	if !strings.Contains(d.Endpoint, "://") || !strings.HasPrefix(d.Endpoint, "http") {
		return nil, errors.New("endpoint must be an http(s) URL")
	}
	if d.Region == "" {
		d.Region = "us-east-1"
	}
	if d.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		d.ID = fmt.Sprintf("dest-%s", hex.EncodeToString(b))
	}
	secretEnc, err := crypto.Encrypt(d.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	if err := s.repo.CreateDestination(ctx, d, secretEnc); err != nil {
		return nil, err
	}
	created := RedactDestination(d)
	return &created, nil
}

// DeleteDestination removes a destination. Schedules still referencing it
// keep running in local-only mode (their snapshots stay on this host), which
// is the documented degradation instead of a hard failure.
func (s *Service) DeleteDestination(ctx context.Context, id string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	return s.repo.DeleteDestination(ctx, id)
}

// TestDestination performs a read probe against the configured bucket. It
// takes the redacted-safe struct plus the raw secret from the request when
// present (create-then-test flow); otherwise it looks the destination up.
func (s *Service) TestDestination(ctx context.Context, id string, secretOverride string) error {
	if s.repo == nil {
		return errors.New("repository not initialized")
	}
	dest, err := s.repo.GetDestination(ctx, id)
	if err != nil || dest == nil {
		return fmt.Errorf("destination not found: %s", id)
	}
	if secretOverride != "" {
		dest.SecretKey = secretOverride
	}
	return testDestination(ctx, *dest)
}

// TestDestinationStruct probes a not-yet-saved destination (create form).
func (s *Service) TestDestinationStruct(ctx context.Context, d BackupDestination) error {
	return testDestination(ctx, d)
}
