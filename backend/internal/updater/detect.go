package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"gopod/internal/podman"
)

func resolveBuildMetadata() (version, commit, buildDate string) {
	version = strings.TrimSpace(Version)
	if version == "" {
		version = "dev"
	}
	commit = strings.TrimSpace(BuildCommit)
	buildDate = strings.TrimSpace(BuildDate)

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if commit == "" {
					commit = s.Value
				}
			case "vcs.time":
				if buildDate == "" {
					buildDate = s.Value
				}
			}
		}
	}
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return version, commit, buildDate
}

func isRunningInContainer() bool {
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

func selfContainerCandidates() []string {
	var candidates []string
	seen := make(map[string]bool)
	add := func(v string) {
		clean := strings.TrimSpace(strings.Trim(v, `"`))
		if clean != "" && !seen[clean] {
			seen[clean] = true
			candidates = append(candidates, clean)
		}
	}

	// Podman stamps /run/.containerenv with `name="..."` and `id="..."`
	if data, err := os.ReadFile("/run/.containerenv"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
				if k == "name" || k == "id" {
					add(v)
				}
			}
		}
	}
	if host, err := os.Hostname(); err == nil {
		add(host)
	}
	add("gopod")
	add("gopod-dev")
	return candidates
}

func systemdBusPath() string {
	if p := strings.TrimSpace(os.Getenv("GOPOD_USER_BUS")); p != "" {
		return p
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); xdg != "" {
		return filepath.Join(xdg, "bus")
	}
	return filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "bus")
}

func probeUserDbus() bool {
	busPath := systemdBusPath()
	if _, err := os.Stat(busPath); err != nil {
		return false
	}
	type res struct {
		c   *dbus.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		conn, err := dbus.Connect(fmt.Sprintf("unix:path=%s", busPath))
		ch <- res{conn, err}
	}()
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.c != nil {
			_ = r.c.Close()
		}
		return r.err == nil
	case <-timer.C:
		go func() {
			if r := <-ch; r.c != nil {
				_ = r.c.Close()
			}
		}()
		return false
	}
}

func callUserDbus(ctx context.Context, method string, args ...any) error {
	conn, err := dbus.Connect(fmt.Sprintf("unix:path=%s", systemdBusPath()))
	if err != nil {
		return err
	}
	defer conn.Close()
	obj := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))
	call := obj.CallWithContext(ctx, "org.freedesktop.systemd1.Manager."+method, 0, args...)
	return call.Err
}

func findQuadletDir() (dir string, hasPod, hasServer bool) {
	var candidates []string
	if q := strings.TrimSpace(os.Getenv("QUADLET_DIR")); q != "" {
		candidates = append(candidates, q)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "containers", "systemd"))
	}
	candidates = append(candidates, "/root/.config/containers/systemd", "/etc/containers/systemd")

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "gopod-server.container")); err == nil {
			_, podErr := os.Stat(filepath.Join(c, "gopod.pod"))
			return c, podErr == nil, true
		}
	}
	return "", false, false
}

func extractShaDigest(raw string) string {
	if idx := strings.Index(raw, "sha256:"); idx != -1 {
		candidate := raw[idx:]
		if len(candidate) >= 71 {
			candidate = candidate[:71]
			if digestSha256.MatchString(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// InspectSelf detects deployment mode, running container/binary details, and auto-update support.
func (s *Service) InspectSelf(ctx context.Context) SelfInspection {
	curVer, _, _ := resolveBuildMetadata()
	info := SelfInspection{
		Containerized:  isRunningInContainer(),
		PodmanSocketOK: s.podmanClient != nil && s.podmanClient.Ping(),
		IsDevBuild:     strings.EqualFold(curVer, "dev"),
	}
	if exe, err := os.Executable(); err == nil {
		info.ExecutablePath = exe
	}

	if !info.Containerized {
		info.Mode = ModeNative
		info.DbusReachable = probeUserDbus()
		if _, err := exec.LookPath("systemctl"); err == nil {
			out, serr := exec.CommandContext(ctx, "systemctl", "--user", "list-unit-files", "gopod.service").CombinedOutput()
			if serr == nil && strings.Contains(string(out), "gopod.service") {
				info.UnitName = "gopod.service"
			}
		}
		// Check whether current executable directory allows atomic swap.
		if info.ExecutablePath != "" {
			dir := filepath.Dir(info.ExecutablePath)
			probePath := filepath.Join(dir, fmt.Sprintf(".gopod-perm-check-%d", time.Now().UnixNano()))
			if f, err := os.OpenFile(probePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600); err == nil {
				_ = f.Close()
				_ = os.Remove(probePath)
				info.CanAutoUpdate = true
			} else {
				info.AutoUpdateReason = "Binary directory is not writable by current user; run installer/package update with elevated privileges."
				info.ManualHint = "curl -fsSL https://raw.githubusercontent.com/ijon6k/gopod/main/scripts/install.sh | bash -s -- update"
			}
		}
		if info.ManualHint == "" {
			info.ManualHint = "gopod update"
		}
		return info
	}

	info.DbusReachable = probeUserDbus()
	var full *podman.ContainerFullInspect
	if s.podmanClient != nil {
		for _, cand := range selfContainerCandidates() {
			if res, err := s.podmanClient.InspectContainerFull(ctx, cand); err == nil && res != nil {
				full = res
				break
			}
		}
	}

	if full != nil {
		idShort := full.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}
		info.ContainerID = idShort
		info.ContainerName = full.Name
		info.RunningImage = full.ImageName
		if info.RunningImage == "" {
			info.RunningImage = full.Image
		}
		info.PodName = strings.TrimSpace(full.Pod)
		if info.PodName == "" && full.Config.Labels != nil {
			info.PodName = strings.TrimSpace(full.Config.Labels["PODNAME"])
		}

		// Collect local OCI digest from inspect or RepoDigests.
		info.LocalDigest = extractShaDigest(full.ImageDigest)
		if info.LocalDigest == "" && s.podmanClient != nil && info.RunningImage != "" {
			if repoDigests, d, err := s.podmanClient.InspectImageRepoDigests(ctx, info.RunningImage); err == nil {
				if sha := extractShaDigest(d); sha != "" {
					info.LocalDigest = sha
				} else {
					for _, rd := range repoDigests {
						if strings.Contains(rd, "ijon6k/gopod") {
							if sha := extractShaDigest(rd); sha != "" {
								info.LocalDigest = sha
								break
							}
						}
					}
				}
			}
		}
	}

	quadDir, hasPodFile, hasServerFile := findQuadletDir()
	info.QuadletDir = quadDir
	autoUpdateLabel := ""
	composeProject := ""
	composeBuilder := ""
	if full != nil && full.Config.Labels != nil {
		autoUpdateLabel = strings.ToLower(strings.TrimSpace(full.Config.Labels["io.containers.autoupdate"]))
		composeProject = strings.TrimSpace(full.Config.Labels["io.podman.compose.project"])
		if composeProject == "" {
			composeProject = strings.TrimSpace(full.Config.Labels["com.docker.compose.project"])
		}
		composeBuilder = strings.TrimSpace(full.Config.Labels["com.docker.compose.image.builder"])
	}

	if hasServerFile || autoUpdateLabel != "" || info.PodName != "" {
		if hasPodFile || info.PodName != "" {
			info.Mode = ModeQuadletPod
			info.UnitName = "gopod-pod.service"
		} else {
			info.Mode = ModeQuadletStandalone
			info.UnitName = "gopod-server.service"
		}
		if !info.PodmanSocketOK {
			info.AutoUpdateReason = "Podman socket is unavailable inside the GOPOD container."
			info.ManualHint = "podman auto-update && systemctl --user restart " + info.UnitName
			return info
		}
		if info.DbusReachable {
			info.CanAutoUpdate = true
		} else {
			info.CanAutoUpdate = false
			info.AutoUpdateReason = "Quadlet systemd unit detected without host user D-Bus socket (GOPOD_USER_BUS). Run 'gopod update' on host or re-run installer to enable 1-click panel updates."
			info.ManualHint = "gopod update"
		}
		return info
	}

	isLocalImage := info.RunningImage != "" && !strings.Contains(info.RunningImage, GHCRImageBase)
	if composeBuilder != "" || isLocalImage {
		info.IsDevBuild = true
		if composeProject != "" {
			info.Mode = ModeDevLocal
		} else {
			info.Mode = ModeDevLocal
		}
		info.CanAutoUpdate = false
		info.AutoUpdateReason = "Running local source-built image (" + info.RunningImage + "). Rebuild locally or explicitly switch to GHCR release."
		info.ManualHint = "podman compose -f compose.dev.yml up -d --build"
		return info
	}

	if composeProject != "" {
		info.Mode = ModeCompose
		info.ManualHint = "podman compose pull && podman compose up -d"
	} else {
		info.Mode = ModeContainer
		info.ManualHint = "podman pull " + GHCRImageBase + ":latest"
	}

	if info.PodmanSocketOK && info.ContainerName != "" {
		info.CanAutoUpdate = true
	} else {
		info.AutoUpdateReason = "Podman socket or container metadata unavailable for safe self-replacement."
	}
	return info
}

func (s *Service) statusFilePath() string {
	dir := strings.TrimSpace(s.dataDir)
	if dir == "" {
		dir = "./data"
	}
	return filepath.Join(dir, "update-status.json")
}

// ReadStatus reads the last persisted self-update status from dataDir.
func (s *Service) ReadStatus() UpdateStatus {
	raw, err := os.ReadFile(s.statusFilePath())
	if err != nil {
		return UpdateStatus{State: "idle"}
	}
	var st UpdateStatus
	if json.Unmarshal(raw, &st) != nil || st.State == "" {
		return UpdateStatus{State: "idle"}
	}
	return st
}

func (s *Service) writeStatus(st UpdateStatus) {
	if err := os.MkdirAll(filepath.Dir(s.statusFilePath()), 0750); err != nil {
		return
	}
	if b, err := json.MarshalIndent(st, "", "  "); err == nil {
		_ = os.WriteFile(s.statusFilePath(), b, 0600)
	}
}

