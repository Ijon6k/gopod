package podman

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// reservedVolumePrefixes are absolute host paths that must never be bound
// into a tenant workload: kernel and device-control virtual filesystems,
// verified boot artifacts, and the host container runtime sockets. /proc,
// /sys and /dev mounts break namespace confinement; runtime sockets hand
// the container the same engine privileges the user's Podman driver has.
var reservedVolumePrefixes = []string{
	"/proc", "/sys", "/dev", "/boot", "/run", "/var/run", "/etc", "/root",
}

// reservedVolumeBases identify runtime sockets regardless of how the
// installation path is overridden via PODMAN_SOCKET/CONTAINER_HOST.
var reservedVolumeBases = []string{
	"podman.sock", "docker.sock", "containerd.sock",
	"containerd-containerd.sock", "buildkit-containerd-sock",
}

// ValidateVolumeSource enforces the shared host-path policy for every bind
// mount that reaches the engine through this client. Named volumes (and
// relative project paths such as ./data) remain accepted; risky host paths
// are refused with a rationale operators can quote verbatim in audit logs.
func ValidateVolumeSource(source string) error {
	if strings.ContainsAny(source, "\x00\n\r:") {
		return errors.New("volume source contains invalid characters or colons")
	}
	src := strings.TrimSpace(source)
	if src == "" {
		return errors.New("volume source is empty")
	}
	if strings.HasPrefix(src, "~") {
		return fmt.Errorf("volume mount to %q is not allowed: home directory reference is not supported", src)
	}
	// Named volumes are safe: podman scopes them to the engine and never
	// exposes a host path. Any value with path separators or dot/tilde
	// traversal is evaluated as a filesystem path.
	if !strings.Contains(src, "/") && !strings.Contains(src, `\`) && src != "." && src != ".." {
		return nil
	}
	clean := filepath.Clean(src)
	if clean == "/" {
		return errors.New("volume mount must not bind the host root directory")
	}
	abs, _ := filepath.Abs(clean)
	if abs == "/" {
		return errors.New("volume mount must not bind the host root directory")
	}
	for _, prefix := range reservedVolumePrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") ||
			(abs != "" && (abs == prefix || strings.HasPrefix(abs, prefix+"/"))) {
			return fmt.Errorf("volume mount to %q is not allowed: it exposes a reserved host path", src)
		}
	}
	base := strings.ToLower(filepath.Base(clean))
	for _, name := range reservedVolumeBases {
		if base == name || strings.HasSuffix(base, "."+name) {
			return fmt.Errorf("volume mount to %q is not allowed: it exposes the container runtime socket", src)
		}
	}
	return nil
}

// ValidateRunVolumeSpec validates a raw podman volume spec of the form
// "source:target[:options]" (the shape compose files, Quadlet fallbacks and
// RunContainerOptions use). Only the source segment is security-relevant:
// the container target is inside the container namespace and the third
// segment carries podman mount options.
func ValidateRunVolumeSpec(spec string) error {
	if strings.ContainsAny(spec, "\x00\n\r") {
		return errors.New("volume spec contains control characters")
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return errors.New("volume spec is empty")
	}
	// Podman volume specs follow source:target[:options] (2 or 3 parts).
	// Exactly 1 or 2 colons are allowed.
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("volume spec %q has invalid format (expected source:target[:options])", spec)
	}
	source := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])
	if source == "" || target == "" {
		return fmt.Errorf("volume spec %q must specify both source and target", spec)
	}
	if !strings.HasPrefix(target, "/") {
		return fmt.Errorf("volume spec %q target must be an absolute path", spec)
	}
	if len(parts) == 3 {
		opts := strings.TrimSpace(parts[2])
		if opts == "" {
			return fmt.Errorf("volume spec %q has empty options segment", spec)
		}
		if strings.ContainsAny(opts, `/\`) {
			return fmt.Errorf("volume spec %q options segment contains invalid characters", spec)
		}
	}
	return ValidateVolumeSource(source)
}
