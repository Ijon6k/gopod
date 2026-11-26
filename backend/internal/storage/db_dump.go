package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// DatabaseRestoreRequest holds optional overrides when restoring a DB dump snapshot.
type DatabaseRestoreRequest struct {
	ServiceID string `json:"serviceId"`
	Database  string `json:"database"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

func firstNonEmptyEnv(env map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(env[k]); v != "" {
			return v
		}
	}
	return ""
}

// RestoreDatabaseDump streams a saved .sql.gz or .rdb.gz dump back into the
// target service's database container.
func (s *Service) RestoreDatabaseDump(ctx context.Context, snap *VolumeSnapshot, archivePath string, req DatabaseRestoreRequest) error {
	if snap == nil {
		return ErrSnapshotNotFound
	}
	engine := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(snap.VolumeName, "db:")))
	switch engine {
	case "postgres", "mysql", "mariadb", "mongodb", "redis":
	default:
		return fmt.Errorf("unsupported database snapshot target %q", snap.VolumeName)
	}

	serviceID := strings.TrimSpace(req.ServiceID)
	if serviceID == "" {
		serviceID = snap.ServiceID
	}
	if serviceID == "" {
		return errors.New("serviceId is required to restore a database dump")
	}

	containerName, err := FindServiceContainer(ctx, serviceID)
	if err != nil {
		return err
	}

	var env map[string]string
	if s.envResolver != nil {
		env, _ = s.envResolver(ctx, serviceID)
	}

	user := strings.TrimSpace(req.Username)
	dbName := strings.TrimSpace(req.Database)
	pass := req.Password

	switch engine {
	case "postgres":
		if user == "" {
			user = firstNonEmptyEnv(env, "POSTGRES_USER")
			if user == "" {
				user = "postgres"
			}
		}
		if dbName == "" {
			dbName = firstNonEmptyEnv(env, "POSTGRES_DB")
			if dbName == "" {
				dbName = user
			}
		}
		if pass == "" {
			pass = firstNonEmptyEnv(env, "POSTGRES_PASSWORD")
		}
	case "mysql", "mariadb":
		if user == "" {
			user = firstNonEmptyEnv(env, "MYSQL_USER", "MARIADB_USER")
			if user == "" {
				user = "root"
			}
		}
		if dbName == "" {
			dbName = firstNonEmptyEnv(env, "MYSQL_DATABASE", "MARIADB_DATABASE")
		}
		if pass == "" {
			if user == "root" {
				pass = firstNonEmptyEnv(env, "MYSQL_ROOT_PASSWORD", "MARIADB_ROOT_PASSWORD", "MYSQL_PASSWORD", "MARIADB_PASSWORD")
			} else {
				pass = firstNonEmptyEnv(env, "MYSQL_PASSWORD", "MARIADB_PASSWORD", "MYSQL_ROOT_PASSWORD", "MARIADB_ROOT_PASSWORD")
			}
		}
	case "mongodb":
		if user == "" {
			user = firstNonEmptyEnv(env, "MONGO_INITDB_ROOT_USERNAME")
		}
		if dbName == "" {
			dbName = firstNonEmptyEnv(env, "MONGO_INITDB_DATABASE")
		}
		if pass == "" {
			pass = firstNonEmptyEnv(env, "MONGO_INITDB_ROOT_PASSWORD")
		}
	}

	if strings.HasPrefix(user, "-") {
		return errors.New("database username must not start with '-'")
	}
	if strings.HasPrefix(dbName, "-") {
		return errors.New("database name must not start with '-'")
	}

	inFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer inFile.Close()

	runExecStream := func(stdin io.Reader, args []string, extraEnv ...string) error {
		var stderrBuf bytes.Buffer
		cmd := exec.CommandContext(ctx, "podman", args...)
		setupCmdEnv(cmd)
		if len(extraEnv) > 0 {
			cmd.Env = append(cmd.Env, extraEnv...)
		}
		cmd.Stdin = stdin
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderrBuf.String())
			if len(msg) > 512 {
				msg = msg[:512]
			}
			if msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}
		return nil
	}

	switch engine {
	case "postgres":
		gr, err := gzip.NewReader(inFile)
		if err != nil {
			return fmt.Errorf("invalid gzip database archive: %w", err)
		}
		defer gr.Close()

		args := []string{"exec", "-i"}
		var extraEnv []string
		if pass != "" {
			args = append(args, "-e", "PGPASSWORD")
			extraEnv = append(extraEnv, "PGPASSWORD="+pass)
		}
		args = append(args, containerName, "psql", "-v", "ON_ERROR_STOP=1", "-U", user, "-h", "localhost", "-d", dbName)
		return runExecStream(gr, args, extraEnv...)

	case "mysql", "mariadb":
		gr, err := gzip.NewReader(inFile)
		if err != nil {
			return fmt.Errorf("invalid gzip database archive: %w", err)
		}
		defer gr.Close()

		bin := "mysql"
		if engine == "mariadb" {
			bin = "mariadb"
		}
		args := []string{"exec", "-i"}
		var extraEnv []string
		if pass != "" {
			args = append(args, "-e", "MYSQL_PWD")
			extraEnv = append(extraEnv, "MYSQL_PWD="+pass)
		}
		args = append(args, containerName, bin, "-u", user)
		if dbName != "" {
			args = append(args, "--", dbName)
		}
		return runExecStream(gr, args, extraEnv...)

	case "mongodb":
		args := []string{"exec", "-i"}
		var extraEnv []string
		mongoArgs := []string{"mongorestore", "--archive", "--gzip", "--drop"}
		if user != "" {
			mongoArgs = append(mongoArgs, "-u", user)
		}
		if dbName != "" {
			mongoArgs = append(mongoArgs, "-d", dbName)
		}
		if pass != "" {
			args = append(args, "-e", "GOPOD_MONGO_PASS")
			extraEnv = append(extraEnv, "GOPOD_MONGO_PASS="+pass)
			args = append(args, containerName, "sh", "-c", `exec "$@" -p "$GOPOD_MONGO_PASS"`, "sh")
			args = append(args, mongoArgs...)
		} else {
			args = append(args, containerName)
			args = append(args, mongoArgs...)
		}
		return runExecStream(inFile, args, extraEnv...)

	case "redis":
		gr, err := gzip.NewReader(inFile)
		if err != nil {
			return fmt.Errorf("invalid gzip redis archive: %w", err)
		}
		defer gr.Close()

		volName, err := findContainerDataVolume(ctx, containerName)
		if err != nil {
			return err
		}

		stopCmd := exec.CommandContext(ctx, "podman", "stop", "-t", "10", "--", containerName)
		setupCmdEnv(stopCmd)
		if out, err := stopCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("stop redis container for restore: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		defer func() {
			startCmd := exec.CommandContext(context.Background(), "podman", "start", "--", containerName)
			setupCmdEnv(startCmd)
			_ = startCmd.Run()
		}()

		args := []string{"run", "--rm", "-i", "-v", fmt.Sprintf("%s:/data:Z", volName), "docker.io/library/alpine:latest", "sh", "-c", "cat > /data/dump.rdb"}
		return runExecStream(gr, args)
	}
	return nil
}

func findContainerDataVolume(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", "inspect", "--format", "json", "--", containerName)
	setupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("inspect redis container mounts: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	var inspected []struct {
		Mounts []struct {
			Type        string `json:"Type"`
			Name        string `json:"Name"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(out, &inspected); err != nil || len(inspected) == 0 {
		return "", fmt.Errorf("parse redis mounts failed: %v", err)
	}
	for _, m := range inspected[0].Mounts {
		if m.Destination == "/data" && m.Type == "volume" && m.Name != "" {
			if err := validateVolumeName(m.Name); err == nil {
				return m.Name, nil
			}
		}
	}
	for _, m := range inspected[0].Mounts {
		if m.Type == "volume" && m.Name != "" {
			if err := validateVolumeName(m.Name); err == nil {
				return m.Name, nil
			}
		}
	}
	return "", errors.New("no named volume mount found on redis container for RDB restore")
}
