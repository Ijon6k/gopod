package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// quadletIdentPattern accepts the base of a systemd unit name: alphanumeric
// first character, then alphanumerics, dots, dashes and underscores, 64
// characters max. A dash-leading value would be parsed as a CLI flag by
// systemctl/podman, so it is rejected before any command is built.
var quadletIdentPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

// QuadletUnit represents a systemd Quadlet declarative file and its live status.
type QuadletUnit struct {
	Name     string `json:"name"`     // e.g. "nginx.container"
	UnitName string `json:"unitName"` // e.g. "nginx.service"
	Type     string `json:"type"`     // "container", "pod", "volume", "network", "kube"
	Path     string `json:"path"`     // full file path on disk
	Status   string `json:"status"`   // "active", "inactive", "failed", "unmanaged"
	Content  string `json:"content"`  // raw unit configuration
}

// quadletSearchDirs returns directories scanned for Quadlet files.
func quadletSearchDirs() []string {
	var dirs []string
	if envDir := os.Getenv("QUADLET_DIR"); envDir != "" {
		dirs = append(dirs, envDir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".config", "containers", "systemd"))
	}
	dirs = append(dirs, "/etc/containers/systemd")
	dirs = append(dirs, "./data/quadlets")
	return dirs
}

// ListQuadlets scans systemd quadlet search paths and returns active status.
func (s *Service) ListQuadlets(ctx context.Context) ([]QuadletUnit, error) {
	hasSystemctl := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		hasSystemctl = true
	}

	seen := make(map[string]bool)
	var units []QuadletUnit
	secretKeys := s.secretEnvKeys()

	for _, dir := range quadletSearchDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fileName := entry.Name()
			ext := filepath.Ext(fileName)
			var unitType string
			switch strings.ToLower(ext) {
			case ".container":
				unitType = "container"
			case ".pod":
				unitType = "pod"
			case ".volume":
				unitType = "volume"
			case ".network":
				unitType = "network"
			case ".kube":
				unitType = "kube"
			default:
				continue
			}

			if seen[fileName] {
				continue
			}
			seen[fileName] = true

			fullPath := filepath.Join(dir, fileName)
			baseName := strings.TrimSuffix(fileName, ext)
			serviceUnit := fmt.Sprintf("%s.service", baseName)

			contentBytes, _ := os.ReadFile(fullPath)
			content := maskEnvironmentValues(string(contentBytes), secretKeys)

			status := "unmanaged"
			if hasSystemctl {
				cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", serviceUnit)
				out, err := cmd.CombinedOutput()
				st := strings.TrimSpace(string(out))
				if err == nil && st == "active" {
					status = "active"
				} else if st == "failed" {
					status = "failed"
				} else if st == "activating" {
					status = "activating"
				} else if st == "inactive" {
					status = "inactive"
				} else {
					status = "inactive"
				}
			} else if s.client != nil {
				// Fallback: check if matching container is running in Podman
				containers, _ := s.client.GetContainers(ctx)
				for _, c := range containers {
					for _, name := range c.Names {
						trimmed := strings.TrimPrefix(name, "/")
						if trimmed == baseName || trimmed == fmt.Sprintf("systemd-%s", baseName) {
							if strings.ToLower(c.State) == "running" {
								status = "active"
							} else {
								status = "inactive"
							}
							break
						}
					}
					if status != "unmanaged" {
						break
					}
				}
			}

			units = append(units, QuadletUnit{
				Name:     fileName,
				UnitName: serviceUnit,
				Type:     unitType,
				Path:     fullPath,
				Status:   status,
				Content:  content,
			})
		}
	}

	return units, nil
}

// GetQuadletContent reads a quadlet unit by file name.
func (s *Service) GetQuadletContent(name string) (string, error) {
	name = filepath.Base(name)
	for _, dir := range quadletSearchDirs() {
		p := filepath.Join(dir, name)
		if data, err := os.ReadFile(p); err == nil {
			return maskEnvironmentValues(string(data), s.secretEnvKeys()), nil
		}
	}
	return "", fmt.Errorf("quadlet unit file not found: %s", name)
}

// validateQuadletName derives and allowlists the unit base identifier used
// in systemctl/podman invocations. A dash-leading base (e.g. "--all.container"
// stripping to "--all") would otherwise land in flag position of
// `systemctl start` or `podman rm -f --all`, so every lifecycle verb funnels
// through this gate.
func validateQuadletName(name string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if !quadletIdentPattern.MatchString(base) {
		return "", fmt.Errorf("invalid quadlet identifier %q: must be an alphanumeric unit name", name)
	}
	return base, nil
}

// StartQuadlet starts a quadlet unit via systemctl or podman container.
func (s *Service) StartQuadlet(ctx context.Context, name string) error {
	base, err := validateQuadletName(name)
	if err != nil {
		return err
	}
	serviceUnit := fmt.Sprintf("%s.service", base)

	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "--user", "start", serviceUnit)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl start error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	// Fallback to Podman start if container exists
	if s.client != nil {
		return s.client.StartContainer(ctx, base)
	}
	return fmt.Errorf("systemctl not available on host to control %s", serviceUnit)
}

// StopQuadlet stops a quadlet unit via systemctl and podman.
func (s *Service) StopQuadlet(ctx context.Context, name string) error {
	base, err := validateQuadletName(name)
	if err != nil {
		return err
	}
	serviceUnit := fmt.Sprintf("%s.service", base)

	var stopErr error
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "--user", "stop", serviceUnit)
		if out, err := cmd.CombinedOutput(); err != nil {
			stopErr = fmt.Errorf("systemctl stop error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
	}

	// Also ensure Podman container is stopped
	if s.client != nil {
		_ = s.client.StopContainer(ctx, base)
		_ = s.client.StopContainer(ctx, fmt.Sprintf("systemd-%s", base))
	}

	return stopErr
}

// RestartQuadlet restarts a quadlet unit via systemctl.
func (s *Service) RestartQuadlet(ctx context.Context, name string) error {
	base, err := validateQuadletName(name)
	if err != nil {
		return err
	}
	serviceUnit := fmt.Sprintf("%s.service", base)

	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "--user", "restart", serviceUnit)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl restart error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		return nil
	}

	if s.client != nil {
		_ = s.client.StopContainer(ctx, base)
		return s.client.StartContainer(ctx, base)
	}
	return fmt.Errorf("systemctl not available on host to control %s", serviceUnit)
}

// ReloadQuadlets executes systemctl --user daemon-reload.
func (s *Service) ReloadQuadlets(ctx context.Context) error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("daemon-reload error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	return fmt.Errorf("systemctl not available (containerized runtime: unit files are mounted to host ~/.config/containers/systemd/)")
}

// DeleteQuadlet thoroughly stops systemd supervision, removes unit files, reloads daemon, and forces Podman container/pod removal.
func (s *Service) DeleteQuadlet(ctx context.Context, name string) error {
	// The derived identifier is interpolated into systemctl/podman invocations
	// below. A name like "--all.container" would otherwise strip to "--all"
	// and turn `podman rm -f --all` into a host-wide cleanup. Only honest
	// unit identifiers are accepted.
	base, err := validateQuadletName(name)
	if err != nil {
		return err
	}
	serviceUnit := fmt.Sprintf("%s.service", base)
	podUnit := fmt.Sprintf("%s-pod.service", base)

	// 1. Force Stop, SIGKILL, and Reset Systemd units while files are still on disk
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", serviceUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", podUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "kill", "--kill-who=all", "--signal=SIGKILL", serviceUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "kill", "--kill-who=all", "--signal=SIGKILL", podUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", serviceUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", podUnit).Run()
	}

	// 2. Remove all matching unit files across search dirs
	for _, dir := range quadletSearchDirs() {
		_ = os.Remove(filepath.Join(dir, name))
		_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%s.container", base)))
		_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%s.pod", base)))
		_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%s.volume", base)))
		_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%s.network", base)))
	}

	// 3. Daemon reload and clean reset
	if _, err := exec.LookPath("systemctl"); err == nil {
		_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", serviceUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", podUnit).Run()
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed").Run()
	}

	// 4. Force remove container and pod in Podman
	if s.client != nil {
		_ = s.client.StopContainer(ctx, base)
		_ = s.client.DeleteContainer(ctx, base, true)
		_ = s.client.DeletePod(ctx, base, true)
	}
	_ = exec.CommandContext(ctx, "podman", "rm", "-f", "--", base).Run()
	_ = exec.CommandContext(ctx, "podman", "pod", "rm", "-f", "--", base).Run()

	return nil
}
