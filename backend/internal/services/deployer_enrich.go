package services

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gopod/internal/activity"
	"gopod/internal/podman"
)

// engineLevelForStatus maps deployment status to an engine event level.
func engineLevelForStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "ready", "running":
		return "info"
	case "cancelled", "interrupted":
		return "warn"
	case "failed":
		return "error"
	default:
		return "info"
	}
}

// emitEngineEventSafe forwards to activity.EmitEngineEvent best-effort.
func emitEngineEventSafe(ctx context.Context, level, scope, serviceID, projectID, actor, action, message string, data any) {
	defer func() { _ = recover() }()
	activity.EmitEngineEvent(ctx, level, scope, serviceID, projectID, actor, action, message, data)
}

// sortedKeys returns sorted map keys for deterministic log output.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// redactSecretValue masks likely-secret env values when echoing podman args.
func redactSecretValue(key, value string) string {
	upper := strings.ToUpper(strings.TrimSpace(key))
	for _, token := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "PRIVATE_KEY", "ACCESS_KEY", "API_KEY", "CREDENTIALS"} {
		if strings.Contains(upper, token) {
			return "***redacted***"
		}
	}
	if strings.HasPrefix(upper, "AWS_") && (strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY")) {
		return "***redacted***"
	}
	return value
}

// sanitizedPodmanRunArgs renders the `podman run` equivalent for logging with
// secret env values redacted. Keys the service stores as secret (exact match)
// are always masked; the name heuristic remains as a fallback for
// compose-derived values. Mirrors podman.Client.RunContainer arg order.
func sanitizedPodmanRunArgs(opts podman.RunContainerOptions, secretKeys map[string]bool) string {
	var args []string
	args = append(args, "podman", "run", "-d", "--name", opts.Name)
	if opts.Pod != "" {
		args = append(args, "--pod", opts.Pod)
	} else {
		for _, p := range opts.Ports {
			args = append(args, "-p", p)
		}
		if opts.Network != "" {
			args = append(args, "--network", opts.Network)
		}
	}
	if opts.RestartPolicy != "" {
		args = append(args, "--restart", opts.RestartPolicy)
	}
	if opts.CPULimit > 0 {
		args = append(args, fmt.Sprintf("--cpus=%.2f", opts.CPULimit))
	}
	if opts.MemoryLimit > 0 {
		args = append(args, fmt.Sprintf("--memory=%dm", opts.MemoryLimit))
	}
	if opts.PidsLimit > 0 {
		args = append(args, fmt.Sprintf("--pids-limit=%d", opts.PidsLimit))
	}
	if opts.MemorySwap != "" {
		args = append(args, fmt.Sprintf("--memory-swap=%s", opts.MemorySwap))
	}
	if opts.UserNS != "" && opts.UserNS != "default" && opts.UserNS != "none" {
		args = append(args, "--userns", opts.UserNS)
	}
	if opts.Privileged {
		args = append(args, "--privileged")
	}
	for _, cap := range opts.CapAdd {
		if cap != "" {
			args = append(args, "--cap-add", cap)
		}
	}
	for _, cap := range opts.CapDrop {
		if cap != "" {
			args = append(args, "--cap-drop", cap)
		}
	}
	for _, dev := range opts.Devices {
		if dev != "" {
			args = append(args, "--device", dev)
		}
	}
	for _, vol := range opts.Volumes {
		if vol != "" {
			args = append(args, "-v", vol)
		}
	}
	for _, sec := range opts.SecurityOpt {
		if sec != "" {
			args = append(args, "--security-opt", sec)
		}
	}
	keys := make([]string, 0, len(opts.Labels))
	for k := range opts.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != "" {
			args = append(args, "--label", fmt.Sprintf("%s=%s", k, opts.Labels[k]))
		}
	}
	envKeys := make([]string, 0, len(opts.Env))
	for k := range opts.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		if k != "" {
			value := opts.Env[k]
			if secretKeys[k] {
				value = "***redacted***"
			} else {
				value = redactSecretValue(k, value)
			}
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, value))
		}
	}
	args = append(args, opts.Image)
	args = append(args, opts.Command...)
	return strings.Join(args, " ")
}

// redactCloneURL strips credentials embedded in a clone URL
// (https://user:token@host/repo) so they never reach the deployment log.
func redactCloneURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

// detectRunExitCode best-effort extracts a podman/OCI exit code (125/126/127)
// from a run failure error string. Returns 0 when unknown.
func detectRunExitCode(runErr error) int {
	if runErr == nil {
		return 0
	}
	msg := strings.ToLower(runErr.Error())
	for _, code := range []int{125, 126, 127} {
		needle := fmt.Sprintf("exit status %d", code)
		if strings.Contains(msg, needle) {
			return code
		}
		needle2 := fmt.Sprintf("exit code %d", code)
		if strings.Contains(msg, needle2) {
			return code
		}
	}
	// Podman CLI surfaces raw output; look for "exit 126" style fragments.
	for _, code := range []int{125, 126, 127} {
		if strings.Contains(msg, fmt.Sprintf(" %d", code)) && (strings.Contains(msg, "permission") || strings.Contains(msg, "conflict") || strings.Contains(msg, "not found") || strings.Contains(msg, "exit")) {
			return code
		}
	}
	return 0
}

// enrichRunFailure appends best-effort `podman logs --tail 30` output and
// likely-cause hints to the deployment log. Never fails the deployment itself.
func (d *Deployer) enrichRunFailure(ctx context.Context, containerName string, runErr error, logLine func(string)) {
	defer func() { _ = recover() }()
	if logLine == nil || strings.TrimSpace(containerName) == "" {
		return
	}
	// Best-effort recent container logs (5s timeout).
	func() {
		defer func() { _ = recover() }()
		lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(lctx, "podman", "logs", "--tail", "30", containerName)
		d.setupCmdEnv(cmd)
		if out, err := cmd.CombinedOutput(); err == nil {
			text := strings.TrimSpace(string(out))
			if text != "" {
				logLine(fmt.Sprintf("📋 Recent logs for '%s' (podman logs --tail 30):", containerName))
				for _, line := range strings.Split(text, "\n") {
					line = strings.TrimRight(line, "\r")
					if strings.TrimSpace(line) != "" {
						logLine(fmt.Sprintf("   │ %s", line))
					}
				}
			} else {
				logLine(fmt.Sprintf("📋 No recent logs available for '%s' (empty output).", containerName))
			}
		} else if lctx.Err() == nil {
			logLine(fmt.Sprintf("📋 Could not fetch logs for '%s': container may not exist yet.", containerName))
		}
	}()

	code := detectRunExitCode(runErr)
	switch code {
	case 126:
		logLine("💡 Likely cause (exit 126): command invoked cannot execute — permission denied, bad interpreter, or image arch mismatch (e.g. amd64 image on arm64 host).")
		logLine("💡 Check: image architecture vs host (`podman image inspect --format {{.Architecture}} <image>` + `uname -m`), entrypoint/command executable bit, and shebang line.")
	case 125:
		logLine("💡 Likely cause (exit 125): podman-level failure — name already in use, port already allocated, or invalid run flags.")
		d.logPortConflictHolder(ctx, logLine)
	case 127:
		logLine("💡 Likely cause (exit 127): command not found inside the container — check Command/Entrypoint override and image PATH.")
		logLine("💡 Check: run the image interactively (`podman run --rm -it <image> sh`) and verify the binary exists.")
	default:
		msg := ""
		if runErr != nil {
			msg = strings.ToLower(runErr.Error())
		}
		switch {
		case strings.Contains(msg, "port") && (strings.Contains(msg, "already") || strings.Contains(msg, "in use") || strings.Contains(msg, "allocated")):
			logLine("💡 Likely cause: host port conflict — another container/pod already publishes the requested port.")
			d.logPortConflictHolder(ctx, logLine)
		case strings.Contains(msg, "already exists") || strings.Contains(msg, "already in use"):
			logLine("💡 Likely cause: container/pod name collision — a stale container with the same name still exists. Pre-flight cleanup normally removes it; check `podman ps -a`.")
		case strings.Contains(msg, "no such image") || strings.Contains(msg, "image not known"):
			logLine("💡 Likely cause: image not present locally and pull failed — check registry credentials and image reference.")
		case strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted"):
			logLine("💡 Likely cause: permission denied — check rootless socket access, SELinux/AppArmor profile, and volume mount permissions.")
		}
	}
}

// logPortConflictHolder cheaply looks up which container holds published ports.
func (d *Deployer) logPortConflictHolder(ctx context.Context, logLine func(string)) {
	defer func() { _ = recover() }()
	lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(lctx, "podman", "ps", "--format", "{{.Names}} {{.Ports}}")
	d.setupCmdEnv(cmd)
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	shown := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		logLine(fmt.Sprintf("   ├─ holder: %s", line))
		shown++
		if shown >= 10 {
			break
		}
	}
}
