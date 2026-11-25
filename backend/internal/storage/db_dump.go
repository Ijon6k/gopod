package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DatabaseDumpRequest contains parameters to trigger a database dump.
type DatabaseDumpRequest struct {
	ProjectID     string `json:"projectId"`
	ServiceID     string `json:"serviceId"`
	Engine        string `json:"engine"`        // postgres, mysql, mariadb, mongodb, redis, etc.
	Database      string `json:"database"`      // Target DB name
	Username      string `json:"username"`      // DB user
	Password      string `json:"password"`      // DB password
	DestinationID string `json:"destinationId"` // Optional remote mirror
}

// DetectDatabaseEngine identifies the engine from image string and environment keys.
func DetectDatabaseEngine(image string, env map[string]string) string {
	img := strings.ToLower(image)
	if strings.Contains(img, "postgres") || strings.Contains(img, "pgsql") || strings.Contains(img, "psql") {
		return "postgres"
	}
	if strings.Contains(img, "mariadb") {
		return "mariadb"
	}
	if strings.Contains(img, "mysql") {
		return "mysql"
	}
	if strings.Contains(img, "redis") || strings.Contains(img, "valkey") {
		return "redis"
	}
	if strings.Contains(img, "mongo") {
		return "mongodb"
	}

	for k := range env {
		kUpper := strings.ToUpper(k)
		if strings.HasPrefix(kUpper, "POSTGRES_") {
			return "postgres"
		}
		if strings.HasPrefix(kUpper, "MARIADB_") {
			return "mariadb"
		}
		if strings.HasPrefix(kUpper, "MYSQL_") {
			return "mysql"
		}
		if strings.HasPrefix(kUpper, "REDIS_") {
			return "redis"
		}
		if strings.HasPrefix(kUpper, "MONGO_") {
			return "mongodb"
		}
	}
	return "custom"
}

// FindServiceContainer looks up running container name for the service.
func FindServiceContainer(ctx context.Context, serviceID string) (string, error) {
	if serviceID == "" {
		return "", errors.New("serviceID is required")
	}

	// 1. Try exact label match: io.gopod.service=<serviceID>
	cmd := exec.CommandContext(ctx, "podman", "ps", "--filter", "label=io.gopod.service="+serviceID, "--format", "{{.Names}}")
	setupCmdEnv(cmd)
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			if name := strings.TrimSpace(l); name != "" {
				return name, nil
			}
		}
	}

	// 2. Fallback: exact match or hyphen-delimited suffix/prefix
	cmd = exec.CommandContext(ctx, "podman", "ps", "--format", "{{.Names}}")
	setupCmdEnv(cmd)
	out, err = cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, l := range lines {
			name := strings.TrimSpace(l)
			if name == serviceID || strings.HasPrefix(name, serviceID+"-") || strings.HasSuffix(name, "-"+serviceID) {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("no running container found for service '%s'", serviceID)
}

// CreateDatabaseDump runs an engine-native dump command streaming into a .sql.gz or .rdb.gz archive.
func (s *Service) CreateDatabaseDump(ctx context.Context, req DatabaseDumpRequest) (*VolumeSnapshot, error) {
	if s.repo == nil {
		return nil, errors.New("repository not initialized")
	}
	if req.ServiceID == "" {
		return nil, errors.New("serviceId is required")
	}
	if req.ProjectID == "" {
		return nil, errors.New("projectId is required")
	}

	containerName, err := FindServiceContainer(ctx, req.ServiceID)
	if err != nil {
		return nil, err
	}

	engine := strings.ToLower(strings.TrimSpace(req.Engine))
	if engine == "" || engine == "custom" {
		engine = "postgres" // fallback default
	}

	ext := "sql.gz"
	if engine == "redis" {
		ext = "rdb.gz"
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	snapID := fmt.Sprintf("dump-%s", hex.EncodeToString(b))
	filename := fmt.Sprintf("db-%s-%s-%d.%s", engine, req.ServiceID, time.Now().Unix(), ext)
	backupDir := s.backupDir()
	_ = os.MkdirAll(backupDir, 0700)
	filePath := filepath.Join(backupDir, filename)

	outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("create backup file: %w", err)
	}
	defer outFile.Close()

	if strings.HasPrefix(strings.TrimSpace(req.Username), "-") {
		return nil, errors.New("database username must not start with '-'")
	}
	if strings.HasPrefix(strings.TrimSpace(req.Database), "-") {
		return nil, errors.New("database name must not start with '-'")
	}

	var dumpErr error
	var stderrBuf bytes.Buffer
	runDump := func(args []string, extraEnv ...string) error {
		gw := gzip.NewWriter(outFile)
		cmd := exec.CommandContext(ctx, "podman", args...)
		setupCmdEnv(cmd)
		if len(extraEnv) > 0 {
			cmd.Env = append(cmd.Env, extraEnv...)
		}
		cmd.Stdout = gw
		cmd.Stderr = &stderrBuf
		runErr := cmd.Run()
		_ = gw.Close()
		if runErr != nil && stderrBuf.Len() > 0 {
			msg := strings.TrimSpace(stderrBuf.String())
			if len(msg) > 512 {
				msg = msg[:512]
			}
			return fmt.Errorf("%w: %s", runErr, msg)
		}
		return runErr
	}

	switch engine {
	case "postgres":
		user := req.Username
		if user == "" {
			user = "postgres"
		}
		dbName := req.Database
		if dbName == "" {
			dbName = user
		}
		args := []string{"exec", "-i"}
		var extraEnv []string
		if req.Password != "" {
			args = append(args, "-e", "PGPASSWORD")
			extraEnv = append(extraEnv, "PGPASSWORD="+req.Password)
		}
		args = append(args, containerName, "pg_dump", "-U", user, "-h", "localhost", "--no-acl", "--no-owner", "--", dbName)
		dumpErr = runDump(args, extraEnv...)

	case "mysql", "mariadb":
		user := req.Username
		if user == "" {
			user = "root"
		}
		dbName := req.Database
		bin := "mysqldump"
		if engine == "mariadb" {
			bin = "mariadb-dump"
		}
		args := []string{"exec", "-i"}
		var extraEnv []string
		if req.Password != "" {
			args = append(args, "-e", "MYSQL_PWD")
			extraEnv = append(extraEnv, "MYSQL_PWD="+req.Password)
		}
		args = append(args, containerName, bin, "-u", user, "--single-transaction", "--quick")
		if dbName != "" {
			args = append(args, "--", dbName)
		} else {
			args = append(args, "--all-databases")
		}
		dumpErr = runDump(args, extraEnv...)

	case "mongodb":
		args := []string{"exec", "-i"}
		var extraEnv []string
		mongoArgs := []string{"mongodump"}
		if req.Username != "" {
			mongoArgs = append(mongoArgs, "-u", req.Username)
		}
		if req.Database != "" {
			mongoArgs = append(mongoArgs, "-d", req.Database)
		}
		mongoArgs = append(mongoArgs, "--archive", "--gzip")
		if req.Password != "" {
			args = append(args, "-e", "GOPOD_MONGO_PASS")
			extraEnv = append(extraEnv, "GOPOD_MONGO_PASS="+req.Password)
			args = append(args, containerName, "sh", "-c", `exec "$@" -p "$GOPOD_MONGO_PASS"`, "sh")
			args = append(args, mongoArgs...)
		} else {
			args = append(args, containerName)
			args = append(args, mongoArgs...)
		}
		dumpErr = runDump(args, extraEnv...)

	case "redis":
		args := []string{"exec", "-i"}
		var extraEnv []string
		if req.Password != "" {
			args = append(args, "-e", "REDISCLI_AUTH")
			extraEnv = append(extraEnv, "REDISCLI_AUTH="+req.Password)
		}
		args = append(args, containerName, "redis-cli", "--rdb", "-")
		dumpErr = runDump(args, extraEnv...)

	default:
		dumpErr = fmt.Errorf("unsupported database engine '%s'", engine)
	}

	snap := VolumeSnapshot{
		ID:          snapID,
		ProjectID:   req.ProjectID,
		ServiceID:   req.ServiceID,
		VolumeName:  "db:" + engine,
		Filename:    filename,
		Compression: "gzip",
		CreatedAt:   time.Now(),
	}

	fi, statErr := os.Stat(filePath)
	if dumpErr == nil && statErr == nil && fi.Size() > 0 {
		snap.SizeBytes = fi.Size()
		snap.Size = formatSnapshotSize(fi.Size())
		snap.Status = "completed"

		// Remote mirror push if destination specified
		if req.DestinationID != "" {
			state := s.mirrorSnapshotToDestination(ctx, &snap, filePath)
			snap.RemoteStatus = state.Status
			snap.RemoteProvider = state.Provider
			snap.RemoteDestinationID = state.DestinationID
			snap.RemoteKey = state.RemoteKey
			snap.Checksum = state.Checksum
			snap.RemoteError = state.ErrText
		}
	} else {
		snap.Size = formatSnapshotSize(0)
		snap.Status = "failed"
		_ = os.Remove(filePath)
		if dumpErr != nil {
			return nil, fmt.Errorf("database dump failed: %w", dumpErr)
		}
		return nil, errors.New("dump produced 0 bytes")
	}

	if err := s.repo.CreateSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	if snap.RemoteStatus != "" {
		_ = s.repo.UpdateSnapshotRemote(ctx, snap.ID, SnapshotRemoteState{
			Status:        snap.RemoteStatus,
			Provider:      snap.RemoteProvider,
			DestinationID: snap.RemoteDestinationID,
			RemoteKey:     snap.RemoteKey,
			Checksum:      snap.Checksum,
			ErrText:       snap.RemoteError,
		})
	}

	return &snap, nil
}
