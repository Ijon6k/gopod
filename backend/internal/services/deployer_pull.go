package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopod/internal/credentials"
)

// resolveRegistry returns the configured registry credentials for a service
// (nil when the service has none, the entry has no username, or lookup fails).
func (d *Deployer) resolveRegistry(ctx context.Context, s Service) *credentials.ContainerRegistry {
	if s.RegistryID == "" || d.registryProvider == nil {
		return nil
	}
	reg, err := d.registryProvider.GetRegistry(ctx, s.RegistryID)
	if err != nil || reg == nil || reg.Username == "" || reg.Token == "" {
		return nil
	}
	return reg
}

// registryHost reduces a registry URL to the host used as the auth-file key
// ("https://ghcr.io/team/" -> "ghcr.io").
func registryHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	raw = strings.TrimSuffix(raw, "/")
	if i := strings.Index(raw, "/"); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return "docker.io"
	}
	return raw
}

// writeRegistryAuthFile writes a temporary podman auth file for the registry
// and returns its path plus a cleanup func. The file is mode 0600 and lives in
// the data directory so the credentials used for a pull never appear in the
// process argv (visible to any local user via `ps`).
func (d *Deployer) writeRegistryAuthFile(reg *credentials.ContainerRegistry) (string, func(), error) {
	auth := base64.StdEncoding.EncodeToString([]byte(reg.Username + ":" + reg.Token))
	payload := map[string]any{
		"auths": map[string]any{
			registryHost(reg.URL): map[string]string{"auth": auth},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", func() {}, fmt.Errorf("encode auth file: %w", err)
	}

	base := d.dataDir
	if base == "" {
		base = os.TempDir()
	}
	authDir := filepath.Join(base, "auth")
	if err := os.MkdirAll(authDir, 0700); err != nil {
		return "", func() {}, fmt.Errorf("create auth dir: %w", err)
	}
	f, err := os.CreateTemp(authDir, "registry-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("create auth file: %w", err)
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }

	if err := f.Chmod(0600); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("chmod auth file: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write auth file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close auth file: %w", err)
	}
	return path, cleanup, nil
}

// pullImage runs `podman pull <image>`, authenticating through a temporary
// auth file when registry credentials are configured. Returns the pull error;
// callers that can fall back to a locally cached image may ignore it.
func (d *Deployer) pullImage(ctx context.Context, image string, reg *credentials.ContainerRegistry, logLine func(string)) error {
	args := []string{"pull"}
	cleanup := func() {}
	if reg != nil {
		authFile, done, err := d.writeRegistryAuthFile(reg)
		if err != nil {
			if logLine != nil {
				logLine(fmt.Sprintf("⚠️ Could not prepare registry auth file: %v", err))
			}
		} else {
			args = append(args, "--authfile", authFile)
			cleanup = done
		}
	}
	defer cleanup()
	args = append(args, image)

	pullCmd := exec.CommandContext(ctx, "podman", args...)
	d.setupCmdEnv(pullCmd)
	pStdout, _ := pullCmd.StdoutPipe()
	pStderr, _ := pullCmd.StderrPipe()
	if err := pullCmd.Start(); err != nil {
		return err
	}
	d.streamPipes(pStdout, pStderr, logLine)
	return pullCmd.Wait()
}
