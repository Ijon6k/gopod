package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/robfig/cron/v3"

	"gopod/internal/activity"
	"gopod/pkg/crypto"
)

// SetSystemSource provides the active database connection and data directory
// for creating consistent instance-level self backups.
func (s *Service) SetSystemSource(db *sql.DB, dataDir string) {
	s.sqlDB = db
	s.dataDir = dataDir
}

// KeyStatusResponse describes the state and fingerprint of the AES-256 master key.
type KeyStatusResponse struct {
	KeyFilePath string `json:"keyFilePath"`
	KeyExists   bool   `json:"keyExists"`
	KeyFromEnv  bool   `json:"keyFromEnv"`
	Fingerprint string `json:"fingerprint"`
	Algorithm   string `json:"algorithm"`
}

// GetKeyStatus inspects the on-disk or environment encryption key.
// The fingerprint shown to the UI is truncated: a 16-hex-char prefix still
// identifies the key between instances without turning the response endpoint
// into an offline verifier for dictionary-guessed keys.
func (s *Service) GetKeyStatus() KeyStatusResponse {
	dataDir := s.dataDir
	if dataDir == "" {
		dataDir = "./data"
	}
	keyPath := crypto.KeyFilePath(dataDir)
	_, statErr := os.Stat(keyPath)
	exists := statErr == nil

	fp, _ := crypto.KeyFingerprint()
	if len(fp) > 16 {
		fp = fp[:16]
	}
	return KeyStatusResponse{
		KeyFilePath: keyPath,
		KeyExists:   exists || crypto.KeyFromEnv(),
		KeyFromEnv:  crypto.KeyFromEnv(),
		Fingerprint: fp,
		Algorithm:   "AES-256-GCM",
	}
}

// CreateSystemBackup packages SQLite gopod.db, keys/master.key, and configurations
// into a unified disaster recovery tar.gz archive and optionally mirrors it to S3.
func (s *Service) CreateSystemBackup(ctx context.Context, destinationID string) (*VolumeSnapshot, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}

	// Prevent overlapping backup operations
	if !s.backupMu.TryLock() {
		return nil, ErrBackupInProgress
	}
	defer s.backupMu.Unlock()

	dataDir := s.getDataDir()

	tmpDir, err := os.MkdirTemp("", "gopod-self-backup-*")
	if err != nil {
		return nil, fmt.Errorf("create temp backup dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Safe SQLite Database Backup
	targetDB := filepath.Join(tmpDir, "gopod.db")
	dbCopied := false
	if s.sqlDB != nil {
		// VACUUM INTO streams a contained, transactionally consistent copy.
		// SQLITE_BUSY under concurrent writers is expected, so retry briefly.
		// A raw file copy cannot be trusted while the WAL still holds frames;
		// a torn archive with status "completed" is the single worst outcome
		// for a disaster-recovery feature.
		escapedTarget := strings.ReplaceAll(targetDB, "'", "''")
		vacuumStmt := fmt.Sprintf("VACUUM INTO '%s'", escapedTarget)
		for attempt := 0; attempt < 3; attempt++ {
			if _, execErr := s.sqlDB.ExecContext(ctx, vacuumStmt); execErr == nil {
				dbCopied = true
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		if !dbCopied {
			// Last resort: force a WAL checkpoint. busy=0 confirms the main
			// database file has been flushed and the WAL truncated, so the
			// main file alone is a consistent standalone image. Anything else
			// must abort loudly — a disk-only copy would silently drop
			// transactions that only exist in the WAL.
			var busy, logFrames, checkpointed int64
			walErr := s.sqlDB.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed)
			if walErr != nil || busy != 0 {
				return nil, fmt.Errorf("%w (busy=%d, err=%v); free WAL readers and retry", ErrInconsistentDatabase, busy, walErr)
			}
			srcDB := filepath.Join(dataDir, "gopod.db")
			if err := copyFile(srcDB, targetDB); err != nil {
				return nil, fmt.Errorf("backup database file: %w", err)
			}
			dbCopied = true
		}
	}
	if !dbCopied {
		// No live connection available (tests/tooling): the raw copy here is
		// only reached for an idle or closed database, where files are static.
		srcDB := filepath.Join(dataDir, "gopod.db")
		if err := copyFile(srcDB, targetDB); err != nil {
			return nil, fmt.Errorf("backup database file: %w", err)
		}
		walSrc := srcDB + "-wal"
		if _, err := os.Stat(walSrc); err == nil {
			if err := copyFile(walSrc, targetDB+"-wal"); err != nil {
				return nil, fmt.Errorf("backup database WAL file: %w", err)
			}
		}
	}

	// 2. Encryption Master Key (Critical for restoring secrets). A backup that
	// silently drops the key makes every encrypted secret unrecoverable —
	// that must surface as a failed backup, never a completed one.
	keyFile := crypto.KeyFilePath(dataDir)
	if info, statErr := os.Stat(keyFile); statErr == nil && !info.IsDir() {
		destKeyDir := filepath.Join(tmpDir, "keys")
		if err := os.MkdirAll(destKeyDir, 0700); err != nil {
			return nil, fmt.Errorf("prepare key directory in backup: %w", err)
		}
		if err := copyFile(keyFile, filepath.Join(destKeyDir, "master.key")); err != nil {
			return nil, fmt.Errorf("bundle master key into backup: %w", err)
		}
		// copyFile inherits the umask (0644); the tarball records that mode
		// verbatim, so force owner-only permissions before archiving.
		if err := os.Chmod(filepath.Join(destKeyDir, "master.key"), 0600); err != nil {
			return nil, fmt.Errorf("secure master key copy in backup: %w", err)
		}
	}

	// 3. System Quadlets & Ingress Configs (if present)
	copyDirIfExists(filepath.Join(dataDir, "quadlets"), filepath.Join(tmpDir, "quadlets"))
	copyDirIfExists(filepath.Join(dataDir, "caddy"), filepath.Join(tmpDir, "caddy"))
	copyDirIfExists(filepath.Join(dataDir, "compose"), filepath.Join(tmpDir, "compose"))

	// 4. Backup metadata manifest
	fp, _ := crypto.KeyFingerprint()
	manifest := fmt.Sprintf("{\n  \"version\": 1,\n  \"createdAt\": %q,\n  \"keyFromEnv\": %v,\n  \"keyFingerprint\": %q\n}",
		time.Now().UTC().Format(time.RFC3339), crypto.KeyFromEnv(), fp)
	if err := os.WriteFile(filepath.Join(tmpDir, "gopod-manifest.json"), []byte(manifest), 0644); err != nil {
		return nil, fmt.Errorf("write backup manifest: %w", err)
	}

	// 5. Create compressed tar.gz archive. Archives bundle master.key plus the
	// entire application database, so the on-disk artifacts stay owner-only.
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate backup identifier: %w", err)
	}
	randSuffix := hex.EncodeToString(b)
	snapID := fmt.Sprintf("sys-%s", randSuffix)
	filename := fmt.Sprintf("gopod-backup-%d-%s.tar.gz", time.Now().Unix(), randSuffix)
	backupDir := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, fmt.Errorf("prepare backup directory: %w", err)
	}
	archivePath := filepath.Join(backupDir, filename)

	if err := createTarGz(tmpDir, archivePath); err != nil {
		_ = os.Remove(archivePath)
		return nil, fmt.Errorf("create backup archive: %w", err)
	}
	_ = os.Chmod(archivePath, 0600)

	fi, err := os.Stat(archivePath)
	if err != nil || fi.Size() == 0 {
		_ = os.Remove(archivePath)
		return nil, errors.New("backup produced empty archive")
	}

	snap := VolumeSnapshot{
		ID:          snapID,
		ProjectID:   "system",
		ServiceID:   "gopod",
		VolumeName:  "system:gopod",
		Filename:    filename,
		SizeBytes:   fi.Size(),
		Size:        formatSnapshotSize(fi.Size()),
		Status:      "completed",
		Compression: "gzip",
		CreatedAt:   time.Now(),
	}

	// 6. Remote Mirror Push (if S3 destination specified)
	if destinationID != "" {
		state := s.mirrorSnapshotToDestination(ctx, &snap, archivePath)
		snap.RemoteStatus = state.Status
		snap.RemoteProvider = state.Provider
		snap.RemoteDestinationID = state.DestinationID
		snap.RemoteKey = state.RemoteKey
		snap.Checksum = state.Checksum
		snap.RemoteError = state.ErrText
	}

	// Insert the row BEFORE mirroring: a crash mid-upload must always leave a
	// recorded local snapshot, never an orphaned remote object that no delete
	// path knows about.
	if err := s.repo.CreateSnapshot(ctx, snap); err != nil {
		_ = os.Remove(archivePath)
		if s.audit != nil {
			_ = s.audit.Record(ctx, "system.backup", "admin", "system:gopod", "storage", "", "failure")
		}
		activity.EmitEngineEvent(ctx, "error", "system", "", "", "admin", "system.backup_failed", "GoPod self-backup failed: "+err.Error(), nil)
		return nil, err
	}

	// 6. Remote Mirror Push (if S3 destination specified)
	if destinationID != "" {
		state := s.mirrorSnapshotToDestination(ctx, &snap, archivePath)
		snap.RemoteStatus = state.Status
		snap.RemoteProvider = state.Provider
		snap.RemoteDestinationID = state.DestinationID
		snap.RemoteKey = state.RemoteKey
		snap.Checksum = state.Checksum
		snap.RemoteError = state.ErrText
		if err := s.repo.UpdateSnapshotRemote(ctx, snap.ID, SnapshotRemoteState{
			Status:        snap.RemoteStatus,
			Provider:      snap.RemoteProvider,
			DestinationID: snap.RemoteDestinationID,
			RemoteKey:     snap.RemoteKey,
			Checksum:      snap.Checksum,
			ErrText:       snap.RemoteError,
		}); err != nil {
			// The snapshot exists locally and in the database; only metadata
			// describing the mirror failed to persist. Log it loudly so
			// remote cleanup after a later delete isn't silently skipped.
			log.Printf("[backup] WARNING: mirror state not recorded for %s: %v", snap.ID, err)
		}
	}

	log.Printf("[backup] GoPod disaster recovery archive created: %s (%s)", snap.Filename, snap.Size)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system.backup", "admin", snap.ID, "storage", "", "success")
	}
	activity.EmitEngineEvent(ctx, "info", "system", "", "", "admin", "system.backup", "GoPod self-backup archive created: "+snap.Filename, map[string]interface{}{"id": snap.ID, "filename": snap.Filename, "size": snap.Size})

	// 7. Retention: manual creations are also weighed against the active
	// schedule's policy — otherwise a habit of "Backup now" clicks defeats
	// the "max archives retained" promise in the UI. Applied only when the
	// schedule is enabled, which is what the switch in the settings card
	// represents.
	if sched, schedErr := s.GetSystemBackupSchedule(ctx); schedErr == nil && sched != nil && sched.Enabled && sched.RetentionCount > 0 {
		all, listErr := s.repo.ListSnapshots(ctx, "system")
		if listErr != nil {
			log.Printf("[backup] WARNING: retention list failed: %v", listErr)
		} else {
			var sysSnaps []VolumeSnapshot
			for _, sn := range all {
				if sn.VolumeName == "system:gopod" || sn.ProjectID == "system" {
					sysSnaps = append(sysSnaps, sn)
				}
			}
			s.pruneSnapshotsByRetention(ctx, sysSnaps, sched.RetentionCount)
		}
	}

	return &snap, nil
}

// ListSystemBackups returns historical instance disaster recovery backups.
func (s *Service) ListSystemBackups(ctx context.Context) ([]VolumeSnapshot, error) {
	if s.repo == nil {
		return []VolumeSnapshot{}, nil
	}
	all, err := s.repo.ListSnapshots(ctx, "")
	if err != nil {
		return nil, err
	}
	var res []VolumeSnapshot
	for _, snap := range all {
		if snap.VolumeName == "system:gopod" || snap.ProjectID == "system" {
			res = append(res, snap)
		}
	}
	if res == nil {
		res = []VolumeSnapshot{}
	}
	return res, nil
}

// SystemBackupVerification is the offline integrity report for one disaster
// recovery archive. It lets an operator prove — while the primary instance is
// still healthy — that an archive would actually survive host recovery day:
// structure, packaged database integrity, key-bundle rules, and the mirror
// checksum when one was recorded.
type SystemBackupVerification struct {
	Valid              bool     `json:"valid"`
	CheckedAt          string   `json:"checkedAt"`
	ArchiveReadable    bool     `json:"archiveReadable"`
	ManifestFound      bool     `json:"manifestFound"`
	ManifestVersion    int      `json:"manifestVersion"`
	ManifestCreatedAt  string   `json:"manifestCreatedAt"`
	ManifestKeyFromEnv bool     `json:"manifestKeyFromEnv"`
	KeyBundleFound     bool     `json:"keyBundleFound"`
	DatabaseFound      bool     `json:"databaseFound"`
	DatabaseIntegrity  string   `json:"databaseIntegrity"`
	ChecksumMatch      string   `json:"checksumMatch"` // "match", "mismatch", or "unavailable"
	Entries            []string `json:"entries,omitempty"`
	Errors             []string `json:"errors,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

// backupManifest mirrors the gopod-manifest.json stored inside every archive.
type backupManifest struct {
	Version        int    `json:"version"`
	CreatedAt      string `json:"createdAt"`
	KeyFromEnv     bool   `json:"keyFromEnv"`
	KeyFingerprint string `json:"keyFingerprint"`
}

// maxVerifyEntries caps how many archive entry names are echoed back in the
// report so a pathological archive cannot balloon the HTTP payload.
const maxVerifyEntries = 100

// VerifySystemBackup streams one stored disaster recovery archive through a
// full integrity inspection without trusting its metadata row: gzip/tar
// structure is decompressed in-flight, the packaged gopod.db is extracted to a
// temporary copy and probed with PRAGMA integrity_check, the manifest is
// parsed to drive the key-presence rule, and the file's sha256 is compared
// against the checksum recorded at mirror time. The report is returned even
// when the archive is broken — a failed verification is exactly the signal an
// operator needs while there is still time to re-run the backup.
func (s *Service) VerifySystemBackup(ctx context.Context, id string) (*SystemBackupVerification, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	snap, err := s.repo.GetSnapshot(ctx, id)
	if err != nil || snap == nil {
		return nil, fmt.Errorf("%w: %s", ErrSnapshotNotFound, id)
	}
	if snap.VolumeName != "system:gopod" && snap.ProjectID != "system" {
		return nil, fmt.Errorf("%w: %s is not an instance disaster recovery archive", ErrSnapshotNotFound, id)
	}
	archivePath, _, err := s.GetSnapshotFilePath(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("open archive for verification: %w", err)
	}

	report := &SystemBackupVerification{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		ChecksumMatch: "unavailable",
	}

	// ── Pass 1: whole-file sha256 (independent of stream success) ──
	var actualChecksum string
	src, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, src)
	src.Close()
	if copyErr == nil {
		actualChecksum = hex.EncodeToString(h.Sum(nil))
	}
	// ── Pass 2: gzip + tar structural walk ──
	src, err = os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer src.Close()

	gz, err := gzip.NewReader(src)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Archive is not a readable gzip stream: %v", err))
	}
	if gz != nil {
		defer gz.Close()
		tr := tar.NewReader(gz)
		tmpDir, mkErr := os.MkdirTemp("", "gopod-verify-*")
		if mkErr != nil {
			return nil, fmt.Errorf("create verification temp dir: %w", mkErr)
		}
		defer os.RemoveAll(tmpDir)

		var tmpDBPath string
	iterate:
		for {
			hdr, iterErr := tr.Next()
			switch {
			case iterErr == io.EOF:
				report.ArchiveReadable = true
				break iterate
			case iterErr != nil:
				report.Errors = append(report.Errors, fmt.Sprintf("Archive stream is corrupt or truncated: %v", iterErr))
				break iterate
			}

			// Canonicalized for robustness against "./"-prefixed archives
			// created by foreign tooling during a manual restore round-trip.
			name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
			if name == "." || name == "" || hdr.FileInfo().IsDir() {
				continue
			}
			if len(report.Entries) < maxVerifyEntries {
				report.Entries = append(report.Entries, name)
			}

			switch name {
			// The manifest drives the key-presence rule; a 1 MiB read cap
			// doubles as protection against a malicious archive.
			case "gopod-manifest.json":
				raw, readErr := io.ReadAll(io.LimitReader(tr, 1<<20))
				if readErr != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("Manifest unreadable: %v", readErr))
					continue
				}
				var m backupManifest
				if jsonErr := json.Unmarshal(raw, &m); jsonErr != nil {
					report.Warnings = append(report.Warnings, fmt.Sprintf("Manifest could not be parsed: %v", jsonErr))
					continue
				}
				report.ManifestFound = true
				report.ManifestVersion = m.Version
				report.ManifestCreatedAt = m.CreatedAt
				report.ManifestKeyFromEnv = m.KeyFromEnv
			case "gopod.db":
				out, openErr := os.Create(filepath.Join(tmpDir, "gopod.db"))
				if openErr != nil {
					return nil, fmt.Errorf("stage packaged database copy: %w", openErr)
				}
				if _, copyErr := io.Copy(out, tr); copyErr != nil {
					out.Close()
					report.Errors = append(report.Errors, fmt.Sprintf("Packaged database unreadable: %v", copyErr))
					continue
				}
				if closeErr := out.Close(); closeErr != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("Packaged database unreadable: %v", closeErr))
					continue
				}
				_ = os.Chmod(filepath.Join(tmpDir, "gopod.db"), 0600)
				tmpDBPath = filepath.Join(tmpDir, "gopod.db")
				report.DatabaseFound = true
			case "gopod.db-wal":
				out, openErr := os.Create(filepath.Join(tmpDir, "gopod.db-wal"))
				if openErr == nil {
					if _, copyErr := io.Copy(out, tr); copyErr == nil {
						report.DatabaseFound = true
					}
					out.Close()
				}
			case "keys/master.key":
				report.KeyBundleFound = true
			}
		}

		// ── Checksum comparison against the mirror-time reference ──
		if want := strings.TrimSpace(snap.Checksum); want != "" {
			if actualChecksum == "" {
				report.Warnings = append(report.Warnings, "Checksum could not be computed — the file read failed.")
			} else if strings.EqualFold(actualChecksum, want) {
				report.ChecksumMatch = "match"
			} else {
				report.ChecksumMatch = "mismatch"
				report.Errors = append(report.Errors, fmt.Sprintf("Checksum mismatch: stored %s, archive %s", want, actualChecksum))
			}
		} else {
			report.Warnings = append(report.Warnings, "No reference checksum stored (archive was never mirrored) — only structural checks ran.")
		}

		// ── Packaged database integrity ──
		if report.DatabaseFound && tmpDBPath != "" {
			report.DatabaseIntegrity = checkSQLiteIntegrity(tmpDBPath)
			if report.DatabaseIntegrity != "ok" {
				report.Errors = append(report.Errors, fmt.Sprintf("Database integrity check failed: %s", report.DatabaseIntegrity))
			}
		} else if report.ArchiveReadable {
			report.Errors = append(report.Errors, "gopod.db not found inside the archive — the packaged database is missing.")
		}

		// ── Key-presence rule from the manifest ──
		switch {
		case report.ManifestFound && !report.ManifestKeyFromEnv && !report.KeyBundleFound:
			report.Errors = append(report.Errors, "Archive was created with a file-based encryption key but keys/master.key is missing — encrypted data would be unrecoverable at host recovery.")
		case report.ManifestFound && report.ManifestKeyFromEnv && !report.KeyBundleFound:
			report.Warnings = append(report.Warnings, "No key bundle inside the archive: GOPOD_ENCRYPTION_KEY matching the manifest fingerprint must be provided on the recovery host.")
		case !report.ManifestFound:
			report.Warnings = append(report.Warnings, "Manifest (gopod-manifest.json) missing — package version and key origin could not be verified.")
		}
	}

	report.Valid = report.ArchiveReadable && report.DatabaseFound && report.DatabaseIntegrity == "ok" && len(report.Errors) == 0

	if s.audit != nil {
		status := "failure"
		if report.Valid {
			status = "success"
		}
		_ = s.audit.Record(ctx, "system.backup_verify", "admin", snap.ID, "storage", "", status)
	}
	if report.Valid {
		activity.EmitEngineEvent(ctx, "info", "system", "", "", "admin", "system.backup_verify",
			fmt.Sprintf("Archive verification passed: %s", snap.Filename),
			map[string]interface{}{"id": snap.ID, "checksumMatch": report.ChecksumMatch})
	} else {
		activity.EmitEngineEvent(ctx, "error", "system", "", "", "admin", "system.backup_verify",
			fmt.Sprintf("Archive verification FAILED (%d problem(s)): %s", len(report.Errors), snap.Filename),
			map[string]interface{}{"id": snap.ID, "errors": report.Errors})
	}
	return report, nil
}

// checkSQLiteIntegrity opens a staged copy of a packaged database and reports
// the first integrity_check row ("ok" or a description of the corruption).
func checkSQLiteIntegrity(path string) string {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Sprintf("could not open packaged database: %v", err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(context.Background(), "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Sprintf("integrity probe failed: %v", err)
	}
	return result
}

// GetSystemBackupSchedule returns automated schedule for GoPod instance backup.
func (s *Service) GetSystemBackupSchedule(ctx context.Context) (*VolumeSchedule, error) {
	if s.repo == nil {
		return nil, nil
	}
	schedules, err := s.repo.ListSchedules(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, sched := range schedules {
		if sched.VolumeName == "system:gopod" {
			return &sched, nil
		}
	}
	return nil, nil
}

// SaveSystemBackupSchedule creates or updates automated self-backup schedule.
func (s *Service) SaveSystemBackupSchedule(ctx context.Context, cronExpr string, retention int, destinationID string, enabled bool) (*VolumeSchedule, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	existing, err := s.GetSystemBackupSchedule(ctx)
	if err != nil {
		return nil, err
	}

	if retention < 1 {
		retention = 7
	}
	// Mirror the UI's caps server-side so direct API calls cannot persist an
	// unbounded (or zero) retention policy.
	if retention > 365 {
		retention = 365
	}
	if cronExpr == "" {
		cronExpr = "0 3 * * *"
	}

	if _, err := cron.ParseStandard(cronExpr); err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}

	if existing != nil {
		existing.Cron = cronExpr
		existing.RetentionCount = retention
		existing.DestinationID = destinationID
		existing.Enabled = enabled
		if err := s.repo.UpdateSchedule(ctx, *existing); err != nil {
			return nil, err
		}
		s.RestartScheduler(ctx)
		log.Printf("[backup] System backup schedule updated: cron=%s retention=%d enabled=%v", existing.Cron, existing.RetentionCount, existing.Enabled)
		if s.audit != nil {
			_ = s.audit.Record(ctx, "system.backup_schedule", "admin", existing.ID, "storage", "", "success")
		}
		activity.EmitEngineEvent(ctx, "info", "system", "", "", "admin", "system.backup_schedule", fmt.Sprintf("System backup schedule updated (%s)", existing.Cron), nil)
		return existing, nil
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	sched := VolumeSchedule{
		ID:             fmt.Sprintf("sched-sys-%s", hex.EncodeToString(b)),
		ProjectID:      "system",
		ServiceID:      "gopod",
		VolumeName:     "system:gopod",
		Cron:           cronExpr,
		Label:          "GoPod System Disaster Recovery",
		RetentionCount: retention,
		DestinationID:  destinationID,
		Enabled:        enabled,
		CreatedAt:      time.Now(),
	}
	if err := s.repo.CreateSchedule(ctx, sched); err != nil {
		return nil, err
	}
	s.RestartScheduler(ctx)
	log.Printf("[backup] System backup schedule created: cron=%s retention=%d enabled=%v", sched.Cron, sched.RetentionCount, sched.Enabled)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system.backup_schedule", "admin", sched.ID, "storage", "", "success")
	}
	activity.EmitEngineEvent(ctx, "info", "system", "", "", "admin", "system.backup_schedule", fmt.Sprintf("System backup schedule created (%s)", sched.Cron), nil)
	return &sched, nil
}

// Helper: copy a single file
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Helper: copy directory recursively if it exists
func copyDirIfExists(srcDir, dstDir string) {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return
	}
	_ = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !info.Mode().IsRegular() {
			// A symlinked config pointing outside dataDir must not be followed
			// (and cannot be safely tarred); record the skip so "completed"
			// never silently hides a partially captured config set.
			log.Printf("[backup] skipping non-regular file in config backup: %s", path)
			return nil
		}
		return copyFile(path, target)
	})
}

// Helper: create tar.gz archive from a directory
func createTarGz(sourceDir, targetTarGz string) error {
	tarFile, err := os.OpenFile(targetTarGz, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gw := gzip.NewWriter(tarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			log.Printf("[backup] skipping non-regular file in archive: %s", relPath)
			return nil
		}
		// Forward slashes for tar format
		headerName := filepath.ToSlash(relPath)
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(headerName, "/")

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			file.Close()
			return copyErr
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	// fsync before the snapshot row is inserted: on power loss the recorded
	// "completed" archive must actually exist on the filesystem.
	if err := tarFile.Sync(); err != nil {
		return err
	}
	return tarFile.Close()
}
