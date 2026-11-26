package updater

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopod/internal/podman"
)

var (
	ErrUpdateInProgress  = errors.New("an update is already in progress")
	ErrDevBuildProtected = errors.New("instance is running a local dev source build; pass confirmSwitchFromDev=true to switch to GHCR release image")
	ErrUpdateUnsupported = errors.New("in-panel update unavailable in current runtime environment")
)

var safeContainerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

// StartUpdate validates permissions/mode/target tag and performs a safe,
// rollback-guarded self-update for Quadlet, Compose/Container, or Native binary installs.
func (s *Service) StartUpdate(ctx context.Context, requestedTag, channel string, confirmSwitchFromDev bool, actorEmail, clientIP string) (*UpdateStatus, error) {
	s.updateMu.Lock()
	if s.isUpdating {
		s.updateMu.Unlock()
		return nil, ErrUpdateInProgress
	}
	s.isUpdating = true
	s.updateMu.Unlock()

	releaseLock := func() {
		s.updateMu.Lock()
		s.isUpdating = false
		s.updateMu.Unlock()
	}

	targetTag := strings.TrimSpace(requestedTag)
	if targetTag == "" {
		ch := ValidateChannel(channel)
		if ch == "dev" {
			targetTag = "dev"
		} else if rel, err := s.fetchLatestRelease(ctx); err == nil && rel != nil && rel.TagName != "" {
			targetTag = rel.TagName
		} else {
			targetTag = "latest"
		}
	}
	cleanTag, err := ValidateTargetTag(targetTag)
	if err != nil {
		releaseLock()
		return nil, err
	}

	self := s.InspectSelf(ctx)
	if self.Mode == ModeDevLocal && !confirmSwitchFromDev {
		releaseLock()
		return nil, ErrDevBuildProtected
	}
	if !self.CanAutoUpdate && !(self.Mode == ModeDevLocal && confirmSwitchFromDev && self.PodmanSocketOK && self.ContainerName != "") {
		releaseLock()
		reason := self.AutoUpdateReason
		if reason == "" {
			reason = ErrUpdateUnsupported.Error()
		}
		return nil, errors.New(reason)
	}

	fullImage := GHCRImageBase + ":" + cleanTag
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system.self_update", actorEmail, fullImage, "system", clientIP, "started")
	}

	startedAt := time.Now().UTC().Format(time.RFC3339)
	st := UpdateStatus{
		State:     "updating",
		TargetTag: cleanTag,
		TargetRef: fullImage,
		Message:   fmt.Sprintf("Preparing update to %s (%s)...", cleanTag, self.Mode),
		StartedAt: startedAt,
	}
	s.writeStatus(st)

	// Perform Mode-specific update asynchronously after pre-flight checks so HTTP 202 returns cleanly.
	go func() {
		defer releaseLock()
		runCtx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()

		var execErr error
		switch self.Mode {
		case ModeQuadletPod, ModeQuadletStandalone:
			execErr = s.executeQuadletUpdate(runCtx, self, cleanTag, fullImage, startedAt)
		case ModeCompose, ModeContainer, ModeDevLocal:
			execErr = s.executeContainerSwapUpdate(runCtx, self, cleanTag, fullImage, startedAt)
		case ModeNative:
			execErr = s.executeNativeBinaryUpdate(runCtx, self, cleanTag, startedAt)
		default:
			execErr = ErrUpdateUnsupported
		}

		if execErr != nil {
			failSt := UpdateStatus{
				State:      "failed",
				TargetTag:  cleanTag,
				TargetRef:  fullImage,
				Message:    execErr.Error(),
				StartedAt:  startedAt,
				FinishedAt: time.Now().UTC().Format(time.RFC3339),
			}
			s.writeStatus(failSt)
			if s.notifier != nil {
				s.notifier.NotifyAsync("deploy_failed", "GOPOD Update Failed",
					fmt.Sprintf("Update to %s failed: %s", cleanTag, execErr.Error()),
					map[string]string{"target": fullImage, "mode": string(self.Mode)})
			}
		}
	}()

	return &st, nil
}

func (s *Service) executeQuadletUpdate(ctx context.Context, self SelfInspection, targetTag, fullImage, startedAt string) error {
	if s.podmanClient == nil {
		return errors.New("podman client unavailable")
	}
	s.writeStatus(UpdateStatus{
		State:     "updating",
		TargetTag: targetTag,
		TargetRef: fullImage,
		Message:   "Pulling " + fullImage + " via Podman socket...",
		StartedAt: startedAt,
	})
	if err := s.podmanClient.PullImage(ctx, fullImage); err != nil {
		return fmt.Errorf("image pull failed: %w", err)
	}

	// Update Image= line cleanly in gopod-server.container if the unit directory is writable.
	if self.QuadletDir != "" {
		unitPath := filepath.Join(self.QuadletDir, "gopod-server.container")
		if raw, err := os.ReadFile(unitPath); err == nil {
			updated, changed := rewriteQuadletImageDirective(string(raw), fullImage)
			if changed {
				_ = os.WriteFile(unitPath+".bak", raw, 0600)
				if werr := os.WriteFile(unitPath, []byte(updated), 0600); werr == nil {
					_ = callUserDbus(ctx, "Reload")
				}
			}
		}
	}

	unit := self.UnitName
	if unit == "" {
		unit = "gopod-pod.service"
	}

	s.writeStatus(UpdateStatus{
		State:      "succeeded",
		TargetTag:  targetTag,
		TargetRef:  fullImage,
		Message:    fmt.Sprintf("Image %s pulled; restarting %s via systemd D-Bus.", fullImage, unit),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// Delay slightly so status persists and any active UI poll reads it before SIGTERM.
	time.Sleep(400 * time.Millisecond)
	return callUserDbus(ctx, "RestartUnit", unit, "replace")
}

func rewriteQuadletImageDirective(content, newImage string) (string, bool) {
	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Image=") {
			if strings.TrimPrefix(trimmed, "Image=") != newImage {
				lines[i] = "Image=" + newImage
				changed = true
			}
			break
		}
	}
	return strings.Join(lines, "\n"), changed
}

func (s *Service) executeContainerSwapUpdate(ctx context.Context, self SelfInspection, targetTag, fullImage, startedAt string) error {
	if s.podmanClient == nil || !safeContainerNameRe.MatchString(self.ContainerName) {
		return errors.New("valid container identity and Podman socket required")
	}

	s.writeStatus(UpdateStatus{
		State:     "updating",
		TargetTag: targetTag,
		TargetRef: fullImage,
		Message:   "Pulling " + fullImage + "...",
		StartedAt: startedAt,
	})
	if err := s.podmanClient.PullImage(ctx, fullImage); err != nil {
		return fmt.Errorf("image pull failed: %w", err)
	}

	inspectTarget := self.ContainerID
	if inspectTarget == "" {
		inspectTarget = self.ContainerName
	}
	fullCfg, err := s.podmanClient.InspectContainerFull(ctx, inspectTarget)
	if err != nil {
		return fmt.Errorf("inspect current container: %w", err)
	}

	ts := time.Now().UTC().Format("20060102150405")
	stagingName := fmt.Sprintf("%s-staging-%s", self.ContainerName, ts)
	backupName := fmt.Sprintf("%s-rollback-%s", self.ContainerName, ts)
	helperName := fmt.Sprintf("%s-updater-%s", self.ContainerName, ts)

	// Pre-create replacement container BEFORE stopping current container so config
	// errors abort safely with zero downtime.
	createArgs, helperMounts := buildReplacementCreateArgs(stagingName, fullImage, fullCfg)
	if _, lookErr := exec.LookPath("podman"); lookErr != nil {
		return errors.New("podman CLI required in container image to stage replacement")
	}
	s.writeStatus(UpdateStatus{
		State:     "updating",
		TargetTag: targetTag,
		TargetRef: fullImage,
		Message:   "Staging replacement container with preserved mounts and network...",
		StartedAt: startedAt,
	})
	createCmd := exec.CommandContext(ctx, "podman", createArgs...)
	s.podmanClient.SetupCmdEnv(createCmd)
	if out, cerr := createCmd.CombinedOutput(); cerr != nil {
		return fmt.Errorf("stage replacement container: %w (%s)", cerr, strings.TrimSpace(string(out)))
	}

	helperImage := self.RunningImage
	if helperImage == "" {
		helperImage = fullImage
	}

	healthPort := "8085"
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		healthPort = p
	}

	// Detached static sibling helper swaps containers, checks /api/health, and auto-rolls back on failure.
	// Zero user-supplied strings are interpolated into this script; variables arrive via env.
	const swapScript = `
set -e
cleanup_fail() {
  echo "{\"state\":\"rolled_back\",\"targetTag\":\"$TARGET_TAG\",\"targetRef\":\"$TARGET_REF\",\"message\":\"New version failed startup health verification; automatically rolled back to previous container.\",\"startedAt\":\"$STARTED_AT\",\"finishedAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" > "$STATUS_FILE"
  podman rm -f "$OLD_NAME" >/dev/null 2>&1 || true
  podman rename "$BACKUP_NAME" "$OLD_NAME" >/dev/null 2>&1 || true
  podman start "$OLD_NAME" >/dev/null 2>&1 || true
  exit 1
}

podman stop -t 15 "$OLD_NAME" >/dev/null 2>&1 || true
podman rename "$OLD_NAME" "$BACKUP_NAME"
if ! podman rename "$STAGING_NAME" "$OLD_NAME"; then
  podman rename "$BACKUP_NAME" "$OLD_NAME" >/dev/null 2>&1 || true
  podman start "$OLD_NAME" >/dev/null 2>&1 || true
  exit 1
fi

if ! podman start "$OLD_NAME"; then
  cleanup_fail
fi

i=0
healthy=0
while [ "$i" -lt 25 ]; do
  state=$(podman inspect --format '{{.State.Running}}' "$OLD_NAME" 2>/dev/null || echo "false")
  if [ "$state" != "true" ]; then
    break
  fi
  if podman exec "$OLD_NAME" wget -q -O /dev/null "http://127.0.0.1:${HEALTH_PORT}/api/health" 2>/dev/null; then
    healthy=1
    break
  fi
  i=$((i + 1))
  sleep 1
done

if [ "$healthy" != "1" ]; then
  state=$(podman inspect --format '{{.State.Running}}' "$OLD_NAME" 2>/dev/null || echo "false")
  if [ "$state" != "true" ]; then
    cleanup_fail
  fi
fi

podman rm -f "$BACKUP_NAME" >/dev/null 2>&1 || true
echo "{\"state\":\"succeeded\",\"targetTag\":\"$TARGET_TAG\",\"targetRef\":\"$TARGET_REF\",\"message\":\"Updated cleanly to $TARGET_REF and verified live healthcheck.\",\"startedAt\":\"$STARTED_AT\",\"finishedAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" > "$STATUS_FILE"
`

	sockHost := os.Getenv("PODMAN_SOCKET")
	if sockHost == "" {
		sockHost = "/run/podman/podman.sock"
	}
	helperArgs := []string{
		"run", "-d", "--rm",
		"--name", helperName,
		"--entrypoint", "/bin/sh",
	}
	if hostSock, dataMount := findHostMounts(fullCfg); hostSock != "" {
		helperArgs = append(helperArgs, "-v", hostSock+":/run/podman/podman.sock:z")
		if dataMount != "" {
			helperArgs = append(helperArgs, "-v", dataMount+":/app/data:Z")
		}
	} else {
		_ = sockHost
		for _, hm := range helperMounts {
			helperArgs = append(helperArgs, "-v", hm)
		}
	}
	helperArgs = append(helperArgs,
		"--security-opt", "label=disable",
		"-e", "CONTAINER_HOST=unix:///run/podman/podman.sock",
		"-e", "DOCKER_HOST=unix:///run/podman/podman.sock",
		"-e", "OLD_NAME="+self.ContainerName,
		"-e", "BACKUP_NAME="+backupName,
		"-e", "STAGING_NAME="+stagingName,
		"-e", "HEALTH_PORT="+healthPort,
		"-e", "TARGET_TAG="+targetTag,
		"-e", "TARGET_REF="+fullImage,
		"-e", "STARTED_AT="+startedAt,
		"-e", "STATUS_FILE=/app/data/update-status.json",
		"--", helperImage, "-c", swapScript,
	)

	s.writeStatus(UpdateStatus{
		State:     "updating",
		TargetTag: targetTag,
		TargetRef: fullImage,
		Message:   "Handing off container swap and health verification to updater helper...",
		StartedAt: startedAt,
	})

	runHelper := exec.CommandContext(ctx, "podman", helperArgs...)
	s.podmanClient.SetupCmdEnv(runHelper)
	if out, herr := runHelper.CombinedOutput(); herr != nil {
		// Clean up staged container since helper never launched.
		cleanCmd := exec.Command("podman", "rm", "-f", "--", stagingName)
		s.podmanClient.SetupCmdEnv(cleanCmd)
		_ = cleanCmd.Run()
		return fmt.Errorf("launch updater helper: %w (%s)", herr, strings.TrimSpace(string(out)))
	}
	return nil
}

func findHostMounts(cfg *podman.ContainerFullInspect) (sockSrc, dataSrc string) {
	if cfg == nil {
		return "", ""
	}
	for _, m := range cfg.Mounts {
		if m.Destination == "/run/podman/podman.sock" && m.Source != "" {
			sockSrc = m.Source
		}
		if m.Destination == "/app/data" && m.Source != "" {
			dataSrc = m.Source
		}
	}
	return sockSrc, dataSrc
}

func buildReplacementCreateArgs(stagingName, targetImage string, cfg *podman.ContainerFullInspect) (args []string, helperMounts []string) {
	args = []string{"create", "--name", stagingName}
	if cfg == nil {
		args = append(args, "--", targetImage)
		return args, nil
	}

	if rp := strings.TrimSpace(cfg.HostConfig.RestartPolicy.Name); rp != "" && rp != "no" {
		args = append(args, "--restart", rp)
	}
	for _, sec := range cfg.HostConfig.SecurityOpt {
		if sec != "" && !strings.ContainsAny(sec, "\r\n") {
			args = append(args, "--security-opt", sec)
		}
	}
	if cfg.HostConfig.Privileged {
		args = append(args, "--privileged")
	}

	// Preserve container networks
	var nets []string
	for netName := range cfg.NetworkSettings.Networks {
		if netName != "" && netName != "none" && netName != "host" {
			nets = append(nets, netName)
		}
	}
	if len(nets) > 0 {
		for _, n := range nets {
			args = append(args, "--network", n)
		}
	} else if cfg.HostConfig.NetworkMode != "" && cfg.HostConfig.NetworkMode != "default" {
		args = append(args, "--network", cfg.HostConfig.NetworkMode)
	}

	// Preserve port bindings
	for cPortProto, bindings := range cfg.HostConfig.PortBindings {
		cPort := strings.Split(cPortProto, "/")[0]
		proto := ""
		if parts := strings.Split(cPortProto, "/"); len(parts) == 2 && parts[1] != "tcp" {
			proto = "/" + parts[1]
		}
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			spec := ""
			if b.HostIP != "" && b.HostIP != "0.0.0.0" {
				spec = fmt.Sprintf("%s:%s:%s%s", b.HostIP, b.HostPort, cPort, proto)
			} else {
				spec = fmt.Sprintf("%s:%s%s", b.HostPort, cPort, proto)
			}
			args = append(args, "-p", spec)
		}
	}

	// Preserve mounts (both bind mounts & named volumes)
	for _, m := range cfg.Mounts {
		if m.Destination == "" {
			continue
		}
		src := m.Source
		if m.Type == "volume" && m.Name != "" {
			src = m.Name
		}
		if src == "" {
			continue
		}
		mode := m.Mode
		if mode == "" && !m.RW {
			mode = "ro"
		}
		spec := src + ":" + m.Destination
		if mode != "" {
			spec += ":" + mode
		}
		args = append(args, "-v", spec)
		if m.Destination == "/run/podman/podman.sock" || m.Destination == "/app/data" {
			helperMounts = append(helperMounts, spec)
		}
	}

	// Preserve env vars (skip container-specific HOSTNAME so Podman refreshes cleanly)
	for _, kv := range cfg.Config.Env {
		if kv == "" || strings.HasPrefix(kv, "HOSTNAME=") || strings.ContainsRune(kv, 0) {
			continue
		}
		args = append(args, "-e", kv)
	}

	// Preserve key orchestrator labels (exclude transient builder tag)
	for k, v := range cfg.Config.Labels {
		if k == "" || k == "com.docker.compose.image.builder" || strings.ContainsAny(k+v, "\r\n") {
			continue
		}
		args = append(args, "--label", k+"="+v)
	}

	args = append(args, "--", targetImage)
	return args, helperMounts
}

func (s *Service) executeNativeBinaryUpdate(ctx context.Context, self SelfInspection, targetTag, startedAt string) error {
	if self.ExecutablePath == "" {
		return errors.New("cannot locate running executable path")
	}
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported host architecture %q for binary self-update", arch)
	}
	binName := fmt.Sprintf("gopod-linux-%s", arch)
	baseDL := fmt.Sprintf("%s/%s/%s/releases/download/%s", strings.TrimRight(s.githubDownloadURL, "/"), GitHubOwner, GitHubRepo, urlPathSegment(targetTag))

	s.writeStatus(UpdateStatus{
		State:     "updating",
		TargetTag: targetTag,
		TargetRef: baseDL + "/" + binName,
		Message:   "Downloading checksums.txt and verifying SHA256...",
		StartedAt: startedAt,
	})

	sumsRaw, err := s.downloadSmallText(ctx, baseDL+"/checksums.txt", 512<<10)
	if err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	expectedSha, ok := parseChecksumForFile(sumsRaw, binName)
	if !ok {
		return fmt.Errorf("SHA256 entry for %s not found in checksums.txt", binName)
	}

	tmpPath := self.ExecutablePath + ".new." + targetTag
	if err := s.downloadAndVerifyBinary(ctx, baseDL+"/"+binName, tmpPath, expectedSha); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	bakPath := self.ExecutablePath + ".bak"
	_ = os.Remove(bakPath)
	if err := copyFileBackup(self.ExecutablePath, bakPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup existing binary: %w", err)
	}
	if err := os.Rename(tmpPath, self.ExecutablePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	if self.UnitName != "" {
		s.writeStatus(UpdateStatus{
			State:      "succeeded",
			TargetTag:  targetTag,
			TargetRef:  binName,
			Message:    fmt.Sprintf("Binary verified and swapped (%s); restarting systemd unit %s.", expectedSha[:12], self.UnitName),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC().Format(time.RFC3339),
		})
		time.Sleep(300 * time.Millisecond)
		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.CommandContext(ctx, "systemctl", "--user", "restart", self.UnitName).Run()
		} else if self.DbusReachable {
			_ = callUserDbus(ctx, "RestartUnit", self.UnitName, "replace")
		}
		return nil
	}

	s.writeStatus(UpdateStatus{
		State:      "succeeded",
		TargetTag:  targetTag,
		TargetRef:  binName,
		Message:    fmt.Sprintf("Binary verified (%s) and staged on disk; restart the GOPOD process to activate.", expectedSha[:12]),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func urlPathSegment(s string) string {
	return strings.ReplaceAll(s, "/", "")
}

func parseChecksumForFile(content, filename string) (string, bool) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(strings.TrimSpace(fields[0]))
		name := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		if name == filename && len(sum) == 64 {
			return sum, true
		}
	}
	return "", false
}

func (s *Service) downloadSmallText(ctx context.Context, rawURL string, maxBytes int64) (string, error) {
	if err := s.validateOutboundURL(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	return string(data), err
}

func (s *Service) downloadAndVerifyBinary(ctx context.Context, rawURL, destPath, expectedHex string) error {
	if err := s.validateOutboundURL(rawURL); err != nil {
		return err
	}
	dlClient := newSafeUpdateHTTPClient(s.allowLocalHTTP)
	dlClient.Timeout = 4 * time.Minute
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binary download HTTP %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	const maxBinarySize = 200 << 20 // 200 MiB ceiling prevents disk DoS
	h := sha256.New()
	limited := io.LimitReader(resp.Body, maxBinarySize+1)
	written, err := io.Copy(io.MultiWriter(out, h), limited)
	if err != nil {
		return err
	}
	if written > maxBinarySize {
		return errors.New("downloaded binary exceeds maximum safe size limit")
	}

	actualHex := hex.EncodeToString(h.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(actualHex), []byte(strings.ToLower(expectedHex))) != 1 {
		return fmt.Errorf("SHA256 verification failed (expected %s, got %s)", expectedHex, actualHex)
	}
	return nil
}

func copyFileBackup(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
