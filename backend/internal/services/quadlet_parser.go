package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// ParsedQuadlet contains extracted directives from a Quadlet unit.
type ParsedQuadlet struct {
	Image         string
	ContainerName string
	Command       string
	Ports         []string
	HostPort      int
	Port          int
	Volumes       []string
	Env           map[string]string
	Network       string
	RestartPolicy string
	UserNS        string
	CapAdd        []string
	CapDrop       []string
	Devices       []string
	Privileged    bool
	Pod           string
}

// ValidateContainerImage validates that the image identifier is an OCI image reference
// and explicitly rejects compose files, file paths, empty strings, and whitespace.
func ValidateContainerImage(image string) error {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return fmt.Errorf("container image cannot be empty")
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return fmt.Errorf("invalid container image %q: OCI image references must not contain whitespace", trimmed)
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") ||
		strings.Contains(lower, "compose.yaml") || strings.Contains(lower, "compose.yml") ||
		strings.Contains(lower, "docker-compose") {
		return fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
	}
	if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") || (strings.HasPrefix(trimmed, "/") && !strings.Contains(trimmed, ":")) {
		return fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
	}
	return nil
}

// ParseQuadlet parses raw .container unit text and extracts container runtime parameters.
func ParseQuadlet(content string) ParsedQuadlet {
	p := ParsedQuadlet{
		Env:     make(map[string]string),
		Network: "gopod-net",
	}

	lines := strings.Split(content, "\n")
	inContainer := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inContainer = strings.EqualFold(trimmed, "[Container]")
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		if inContainer {
			switch {
			case strings.EqualFold(key, "Image"):
				p.Image = val
			case strings.EqualFold(key, "ContainerName"):
				p.ContainerName = val
			case strings.EqualFold(key, "Exec"):
				p.Command = val
			case strings.EqualFold(key, "Network"):
				p.Network = val
			case strings.EqualFold(key, "Pod"):
				p.Pod = val
			case strings.EqualFold(key, "PublishPort") || strings.EqualFold(key, "Port"):
				p.Ports = append(p.Ports, val)
				portParts := strings.Split(val, ":")
				if len(portParts) == 2 {
					var hp, cp int
					_, _ = fmt.Sscanf(portParts[0], "%d", &hp)
					_, _ = fmt.Sscanf(portParts[1], "%d", &cp)
					if p.HostPort == 0 && hp > 0 {
						p.HostPort = hp
					}
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				} else if len(portParts) == 3 {
					var hp, cp int
					_, _ = fmt.Sscanf(portParts[1], "%d", &hp)
					_, _ = fmt.Sscanf(portParts[2], "%d", &cp)
					if p.HostPort == 0 && hp > 0 {
						p.HostPort = hp
					}
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				} else if len(portParts) == 1 {
					var cp int
					_, _ = fmt.Sscanf(portParts[0], "%d", &cp)
					if p.Port == 0 && cp > 0 {
						p.Port = cp
					}
				}
			case strings.EqualFold(key, "Volume"):
				p.Volumes = append(p.Volumes, val)
			case strings.EqualFold(key, "Environment"):
				envParts := strings.SplitN(val, "=", 2)
				if len(envParts) == 2 {
					p.Env[envParts[0]] = envParts[1]
				}
			case strings.EqualFold(key, "UserNS"):
				p.UserNS = val
			case strings.EqualFold(key, "AddCapability"):
				p.CapAdd = append(p.CapAdd, val)
			case strings.EqualFold(key, "DropCapability"):
				p.CapDrop = append(p.CapDrop, val)
			case strings.EqualFold(key, "AddDevice"):
				p.Devices = append(p.Devices, val)
			case strings.EqualFold(key, "SecurityLabelDisable"):
				if strings.EqualFold(val, "true") || strings.EqualFold(val, "yes") {
					p.Privileged = true
				}
			case strings.EqualFold(key, "Restart"):
				p.RestartPolicy = val
			}
		} else {
			if strings.EqualFold(key, "Restart") && p.RestartPolicy == "" {
				p.RestartPolicy = val
			}
		}
	}

	return p
}

// SanitizeQuadletSlug ensures systemd unit names, container names, and pod names contain no spaces or invalid characters.
func SanitizeQuadletSlug(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "..", "")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	clean := strings.Trim(b.String(), "-")
	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}
	return clean
}

// SplitQuadletUnits splits single or multi-unit YAML/INI Quadlet configurations into named units.
func SplitQuadletUnits(content string, defaultName string) map[string]string {
	units := make(map[string]string)
	cleanDefault := SanitizeQuadletSlug(defaultName)
	if cleanDefault == "" {
		cleanDefault = "workload"
	}

	chunks := strings.Split(content, "\n---")
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(strings.TrimPrefix(chunk, "---"))
		if trimmed == "" {
			continue
		}

		// Detect explicit file directive comment: # File: <filename>
		var explicitFileName string
		lines := strings.Split(trimmed, "\n")
		for _, line := range lines {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "#") && strings.Contains(strings.ToLower(l), "file:") {
				parts := strings.SplitN(l, ":", 2)
				if len(parts) == 2 {
					explicitFileName = strings.TrimSpace(parts[1])
					break
				}
			}
		}

		if explicitFileName != "" {
			units[filepath.Base(explicitFileName)] = trimmed
			continue
		}

		// Determine unit type from systemd section headers
		if strings.Contains(trimmed, "[Container]") {
			cName := extractDirective(trimmed, "ContainerName")
			if cName != "" {
				cleanCName := SanitizeQuadletSlug(cName)
				units[fmt.Sprintf("%s.container", cleanCName)] = trimmed
			} else {
				units[fmt.Sprintf("%s.container", cleanDefault)] = trimmed
			}
		} else if strings.Contains(trimmed, "[Pod]") {
			podName := extractDirective(trimmed, "PodName")
			if podName != "" {
				cleanPodName := SanitizeQuadletSlug(podName)
				units[fmt.Sprintf("%s.pod", cleanPodName)] = trimmed
			} else {
				units[fmt.Sprintf("pod_%s.pod", cleanDefault)] = trimmed
			}
		} else if strings.Contains(trimmed, "[Network]") {
			units[fmt.Sprintf("%s.network", cleanDefault)] = trimmed
		} else if strings.Contains(trimmed, "[Volume]") {
			units[fmt.Sprintf("%s.volume", cleanDefault)] = trimmed
		} else if strings.Contains(trimmed, "PodName=") {
			units[fmt.Sprintf("pod_%s.pod", cleanDefault)] = trimmed
		} else {
			units[fmt.Sprintf("%s.container", cleanDefault)] = trimmed
		}
	}
	return units
}

func extractDirective(content string, key string) string {
	lines := strings.Split(content, "\n")
	prefix := strings.ToLower(key) + "="
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			val := strings.TrimPrefix(trimmed, trimmed[:len(prefix)])
			return strings.Trim(strings.TrimSpace(val), "\"'")
		}
	}
	return ""
}

// ValidateQuadletFiles runs official podman quadlet dryrun validation in an isolated directory.
func ValidateQuadletFiles(files map[string]string) (bool, string, error) {
	if len(files) == 0 {
		return false, "", fmt.Errorf("no Quadlet unit files provided")
	}

	// 1. Structural AST validation
	for fileName, content := range files {
		trimmed := strings.TrimSpace(content)
		if !strings.Contains(trimmed, "[Unit]") {
			return false, "", fmt.Errorf("unit file '%s' is missing [Unit] section", fileName)
		}
		if strings.HasSuffix(fileName, ".pod") {
			if !strings.Contains(trimmed, "[Pod]") {
				return false, "", fmt.Errorf("pod unit file '%s' is missing [Pod] section", fileName)
			}
		} else if strings.HasSuffix(fileName, ".container") {
			if !strings.Contains(trimmed, "[Container]") {
				return false, "", fmt.Errorf("container unit file '%s' is missing [Container] section", fileName)
			}
			parsed := ParseQuadlet(trimmed)
			if parsed.Image == "" {
				return false, "", fmt.Errorf("container unit file '%s' is missing Image= directive", fileName)
			}
			if err := ValidateContainerImage(parsed.Image); err != nil {
				return false, "", fmt.Errorf("container unit file '%s' has invalid image '%s': %w", fileName, parsed.Image, err)
			}
		}
	}

	// 2. Binary dry-run validator if available
	quadletBin := "/usr/libexec/podman/quadlet"
	if _, err := os.Stat(quadletBin); err != nil {
		if path, err := exec.LookPath("quadlet"); err == nil {
			quadletBin = path
		} else if _, err := os.Stat("/usr/lib/podman/quadlet"); err == nil {
			quadletBin = "/usr/lib/podman/quadlet"
		} else {
			return true, "Syntax valid (AST verified). Quadlet dryrun binary not present.", nil
		}
	}

	tempDir, err := os.MkdirTemp("", "gopod-quadlet-val-*")
	if err != nil {
		return true, "Syntax valid (AST)", nil
	}
	defer os.RemoveAll(tempDir)

	unitDir := filepath.Join(tempDir, "units")
	outDir := filepath.Join(tempDir, "out")
	_ = os.MkdirAll(unitDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	for fname, content := range files {
		base := filepath.Base(fname)
		_ = os.WriteFile(filepath.Join(unitDir, base), []byte(content), 0644)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, quadletBin, "-dryrun", "-user", outDir)
	cmd.Env = append(os.Environ(), fmt.Sprintf("QUADLET_UNIT_DIRS=%s", unitDir))
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		return false, outStr, fmt.Errorf("quadlet dryrun error: %s", strings.TrimSpace(outStr))
	}
	if strings.Contains(outStr, "converting") && strings.Contains(outStr, "error") {
		return false, outStr, fmt.Errorf("quadlet conversion failed: %s", strings.TrimSpace(outStr))
	}

	return true, strings.TrimSpace(outStr), nil
}

// QuadletUnitName produces a consistent, sanitized systemd unit name for a service.
func QuadletUnitName(s Service) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = s.ID
	}
	clean := SanitizeQuadletSlug(name)
	if clean == "" {
		clean = fmt.Sprintf("service-%s", s.ID)
	}
	return clean
}

// CheckSystemdLinger checks if systemd lingering is enabled for user session persistence.
func CheckSystemdLinger() (bool, string) {
	currUser := os.Getenv("USER")
	if currUser == "" {
		if u, err := user.Current(); err == nil {
			currUser = u.Username
		}
	}
	if currUser == "" {
		currUser = "unknown"
	}

	if _, err := os.Stat(filepath.Join("/var/lib/systemd/linger", currUser)); err == nil {
		return true, currUser
	}

	if _, err := exec.LookPath("loginctl"); err == nil {
		// `loginctl` talks to the D-Bus PAM layer, which can stall on a stuck
		// bus; this endpoint is reachable from request paths, so the call is
		// bounded and fails open (to the /var/lib/systemd/linger stat done
		// above) instead of hanging the request forever.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "loginctl", "show-user", currUser, "--property=Linger")
		if out, err := cmd.Output(); err == nil {
			str := strings.TrimSpace(string(out))
			if strings.EqualFold(str, "Linger=yes") {
				return true, currUser
			}
		}
	}

	return false, currUser
}
