package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gopod/internal/podman"
)

// PodNameForService returns the canonical runtime pod name for a pod service.
// Format: pod_<sanitized service ID>. Service IDs already embed
// <project>-<service>-<randhex>, so this is unique per project AND per
// service, deterministic across deploys (stable Netavark DNS + Quadlet unit),
// and safe against two projects owning a pod with the same display name.
func PodNameForService(s Service) string {
	if strings.TrimSpace(s.ID) != "" {
		slug := strings.ToLower(SanitizeQuadletSlug(s.ID))
		slug = strings.Trim(slug, "-")
		if len(slug) > 48 {
			slug = slug[:48]
			slug = strings.Trim(slug, "-")
		}
		if slug != "" {
			return fmt.Sprintf("pod_%s", slug)
		}
	}
	raw := s.PodID
	if raw == "" {
		raw = s.Name
	}
	return PodNameFor(raw)
}

// legacyPodCandidates lists runtime pod names from older naming schemes so
// deploy can garbage-collect them after migrating to the canonical name.
func legacyPodCandidates(s Service) []string {
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	add(PodNameFor(s.Name))
	if strings.TrimSpace(s.PodID) != "" {
		add(s.PodID)
		add(PodNameFor(s.PodID))
	}
	return out
}

// PodNameFor returns the canonical Netavark/podman pod name for a raw display
// name. Single source of truth for ad-hoc names: lowercase slug prefixed with
// pod_. Prefer PodNameForService when a Service record is available.
func PodNameFor(raw string) string {
	slug := SanitizeQuadletSlug(strings.TrimPrefix(strings.TrimSpace(raw), "pod_"))
	slug = strings.ToLower(slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "pod-service"
	}
	return fmt.Sprintf("pod_%s", slug)
}

// splitPortProto strips an optional /udp or /tcp suffix.
// Returns the remainder plus true when the protocol is UDP (TCP suffix is
// dropped — it is the podman default).
func splitPortProto(p string) (clean string, udp bool) {
	clean = strings.TrimSpace(p)
	lower := strings.ToLower(clean)
	if strings.HasSuffix(lower, "/udp") {
		return clean[:len(clean)-4], true
	}
	if strings.HasSuffix(lower, "/tcp") {
		return clean[:len(clean)-4], false
	}
	return clean, false
}

// parsePortBinding parses "[IP:]host:container[/proto]" into its parts.
// IP is "" when absent (meaning 0.0.0.0 — bind on all interfaces).
// ok is false when either port is missing or non-numeric.
func parsePortBinding(p string) (ip string, host, container int, udp bool, ok bool) {
	clean, udp := splitPortProto(p)
	parts := strings.Split(clean, ":")
	var hStr, cStr string
	switch {
	case len(parts) == 2:
		hStr, cStr = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	case len(parts) >= 3:
		ip = strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))
		hStr, cStr = strings.TrimSpace(parts[len(parts)-2]), strings.TrimSpace(parts[len(parts)-1])
	default:
		return "", 0, 0, false, false
	}
	var hp, cp int
	if _, _ = fmt.Sscanf(hStr, "%d", &hp); hp <= 0 {
		return "", 0, 0, false, false
	}
	if _, _ = fmt.Sscanf(cStr, "%d", &cp); cp <= 0 {
		return "", 0, 0, false, false
	}
	return ip, hp, cp, udp, true
}

// canonicalPortString renders "[IP:]host:container[/udp]".
// Empty or 0.0.0.0 IP is omitted (it is the default bind address).
func canonicalPortString(ip string, host, container int, udp bool) string {
	ip = strings.TrimSpace(ip)
	suffix := ""
	if udp {
		suffix = "/udp"
	}
	if ip != "" && ip != "0.0.0.0" {
		return fmt.Sprintf("%s:%d:%d%s", ip, host, container, suffix)
	}
	return fmt.Sprintf("%d:%d%s", host, container, suffix)
}

// canonicalPortBinding parses p and re-renders the canonical backend port
// string "[IP:]host:container[/proto]", preserving a non-default bind IP.
// Returns "" when p is not a valid binding.
func canonicalPortBinding(p string) string {
	ip, h, c, udp, ok := parsePortBinding(p)
	if !ok {
		return ""
	}
	return canonicalPortString(ip, h, c, udp)
}

// servicePortBinding renders the service-level "[HostIP:]host:container"
// binding. HostIP "" (or 0.0.0.0) means bind on all interfaces and is omitted.
func servicePortBinding(hostIP string, hostPort, containerPort int) string {
	return canonicalPortString(hostIP, hostPort, containerPort, false)
}

// normalizePortBinding canonicalizes "[IP:]h:c[/proto]" into the drift key
// "{ip}:{h}:{c}{/udp if udp}", normalizing an absent IP to 0.0.0.0.
// 0.0.0.0 and 127.0.0.1 bindings are DIFFERENT keys (drift!). TCP suffix is
// dropped (podman default), UDP is preserved.
func normalizePortBinding(p string) string {
	ip, h, c, udp, ok := parsePortBinding(p)
	if !ok {
		return ""
	}
	if strings.TrimSpace(ip) == "" {
		ip = "0.0.0.0"
	}
	suffix := ""
	if udp {
		suffix = "/udp"
	}
	return fmt.Sprintf("%s:%d:%d%s", ip, h, c, suffix)
}

func portBindingSet(ports []string) map[string]bool {
	set := make(map[string]bool, len(ports))
	for _, p := range ports {
		if n := normalizePortBinding(p); n != "" {
			set[n] = true
		}
	}
	return set
}

// extractPodPorts gathers all published host-to-container port bindings from service spec and compose YAML.
func extractPodPorts(s Service) []string {
	seen := make(map[string]bool)
	var result []string

	addPort := func(p string) {
		n := canonicalPortBinding(p)
		if n == "" {
			return
		}
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}

	// 1. Primary Service HostPort / Port (+ service-level HostIP bind address)
	if s.Port > 0 {
		hPort := s.HostPort
		if hPort <= 0 {
			hPort = s.Port
		}
		addPort(servicePortBinding(s.HostIP, hPort, s.Port))
	}

	// 2. Parse from Compose YAML if present
	if strings.TrimSpace(s.ComposeYaml) != "" {
		var composeData struct {
			Services map[string]struct {
				Ports []interface{} `yaml:"ports"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal([]byte(s.ComposeYaml), &composeData); err == nil {
			for _, svcDef := range composeData.Services {
				for _, p := range svcDef.Ports {
					if pStr, ok := p.(string); ok {
						addPort(pStr)
					} else if pNum, ok := p.(int); ok {
						addPort(fmt.Sprintf("%d:%d", pNum, pNum))
					}
				}
			}
		}
	}

	// 3. Parse from Kubernetes Pod YAML if present
	if strings.TrimSpace(s.K8sYaml) != "" {
		var k8sData struct {
			Spec struct {
				Containers []struct {
					Ports []struct {
						ContainerPort int    `yaml:"containerPort"`
						HostPort      int    `yaml:"hostPort"`
						Protocol      string `yaml:"protocol"`
					} `yaml:"ports"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(s.K8sYaml), &k8sData); err == nil {
			for _, c := range k8sData.Spec.Containers {
				for _, p := range c.Ports {
					if p.ContainerPort > 0 {
						hp := p.HostPort
						if hp <= 0 {
							hp = p.ContainerPort
						}
						proto := strings.ToLower(strings.TrimSpace(p.Protocol))
						if proto == "udp" {
							addPort(fmt.Sprintf("%d:%d/udp", hp, p.ContainerPort))
						} else {
							addPort(fmt.Sprintf("%d:%d", hp, p.ContainerPort))
						}
					}
				}
			}
		}
	}

	return result
}

// deployPod manages Podman Pod lifecycle, infra container creation, and port publishing.
func (d *Deployer) deployPod(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	podName := PodNameForService(s)
	logLine(fmt.Sprintf("Reconciling Podman Pod '%s' (service: %s)...", podName, s.Name))

	ports := extractPodPorts(s)
	desiredSet := portBindingSet(ports)
	logLine(fmt.Sprintf("🔍 Desired pod ports for '%s': %v", podName, ports))

	if d.podmanClient != nil {
		_ = d.podmanClient.EnsureNetwork(ctx, "gopod-net")

		existing, err := d.podmanClient.InspectPod(ctx, podName)
		needsRecreate := false
		if existing != nil {
			cmd := exec.CommandContext(ctx, "podman", "pod", "inspect", podName, "--format", "{{json .InfraConfig.PortBindings}}")
			d.podmanClient.SetupCmdEnv(cmd)
			if out, err := cmd.Output(); err == nil {
				bindingsStr := strings.TrimSpace(string(out))
				if bindingsStr == "{}" || bindingsStr == "null" || bindingsStr == "" {
					logLine(fmt.Sprintf("🔍 Infra actual bindings for '%s': (none)", podName))
					if len(desiredSet) > 0 {
						needsRecreate = true
						logLine(fmt.Sprintf("   ├─ verdict: recreate — infra has no bindings but %d port(s) desired", len(desiredSet)))
					} else {
						logLine("   ├─ verdict: keep — no ports desired and none bound")
					}
				} else {
					var existingBindings map[string][]struct {
						HostPort string `json:"HostPort"`
						HostIP   string `json:"HostIp"`
					}
					if err := json.Unmarshal([]byte(bindingsStr), &existingBindings); err == nil {
						actualSet := make(map[string]bool, len(existingBindings))
						for contPort, binds := range existingBindings {
							// contPort looks like "80/tcp" or "6379/udp"
							proto := ""
							clean := contPort
							if strings.HasSuffix(strings.ToLower(clean), "/udp") {
								proto = "/udp"
								clean = clean[:len(clean)-4]
							} else if strings.HasSuffix(strings.ToLower(clean), "/tcp") {
								clean = clean[:len(clean)-4]
							}
							var cp int
							_, _ = fmt.Sscanf(strings.TrimSpace(clean), "%d", &cp)
							if cp <= 0 {
								continue
							}
							for _, b := range binds {
								var hp int
								_, _ = fmt.Sscanf(strings.TrimSpace(b.HostPort), "%d", &hp)
								if hp > 0 {
									ip := strings.TrimSpace(b.HostIP)
									if ip == "" {
										ip = "0.0.0.0"
									}
									actualSet[fmt.Sprintf("%s:%d:%d%s", ip, hp, cp, proto)] = true
								}
							}
						}
						if len(actualSet) != len(desiredSet) {
							needsRecreate = true
						} else {
							for k := range desiredSet {
								if !actualSet[k] {
									needsRecreate = true
									break
								}
							}
						}
						logLine(fmt.Sprintf("🔍 Infra actual bindings for '%s': %v", podName, sortedKeys(actualSet)))
						for _, k := range sortedKeys(desiredSet) {
							if actualSet[k] {
								logLine(fmt.Sprintf("   ├─ port %s: keep (matches infra binding)", k))
							} else {
								logLine(fmt.Sprintf("   ├─ port %s: differs — triggers recreate", k))
							}
						}
						for _, k := range sortedKeys(actualSet) {
							if !desiredSet[k] {
								logLine(fmt.Sprintf("   ├─ port %s: stale infra binding (not desired) — triggers recreate", k))
							}
						}
						if needsRecreate {
							logLine("   ├─ verdict: recreate (binding set differs)")
						} else {
							logLine("   ├─ verdict: keep (binding set matches)")
						}
					}
				}
			}
		}

		if err != nil || existing == nil || needsRecreate {
			if existing != nil && needsRecreate {
				logLine(fmt.Sprintf("🔄 Port bindings changed, recreating Podman Pod '%s' with ports %v...", podName, ports))
				_ = d.podmanClient.DeletePod(ctx, podName, true)
			} else {
				logLine(fmt.Sprintf("🚀 Creating Podman Pod '%s' on network 'gopod-net'...", podName))
			}
			_, err = d.podmanClient.CreatePod(ctx, podman.CreatePodOptions{
				Name:    podName,
				Ports:   ports,
				Network: "gopod-net",
				Labels: map[string]string{
					"io.gopod.service": s.ID,
					"io.gopod.project": s.ProjectID,
				},
			})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				logLine(fmt.Sprintf("❌ Failed to create pod '%s': %v", podName, err))
				emitEngineEventSafe(ctx, "error", "service", s.ID, s.ProjectID, "engine", "pod.create_failed",
					fmt.Sprintf("Pod '%s' create failed: %v (ports %v)", podName, err, ports), map[string]any{"pod": podName, "ports": ports})
				return nil, err
			}
			if existing != nil && needsRecreate {
				logLine(fmt.Sprintf("✅ Pod '%s' recreated with ports %v.", podName, ports))
				emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.recreate",
					fmt.Sprintf("Pod '%s' recreated with ports %v", podName, ports), map[string]any{"pod": podName, "ports": ports})
			} else {
				logLine(fmt.Sprintf("✅ Pod '%s' created with ports %v.", podName, ports))
				emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.create",
					fmt.Sprintf("Pod '%s' created with ports %v", podName, ports), map[string]any{"pod": podName, "ports": ports})
			}
		} else {
			logLine(fmt.Sprintf("⚙️ Starting existing Podman Pod '%s'...", podName))
			_ = d.podmanClient.StartPod(ctx, podName)
			logLine(fmt.Sprintf("✅ Pod '%s' kept (bindings match); start issued.", podName))
			emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.reconcile_keep",
				fmt.Sprintf("Pod '%s' kept; bindings match desired %v", podName, ports), map[string]any{"pod": podName, "ports": ports})
		}
	}

	logLine(fmt.Sprintf("Podman Pod '%s' is active and running on network 'gopod-net'.", podName))

	// Migrate away from legacy name-scheme pods (pod_<name>): they would hold
	// the same host ports and shadow the canonical pod. Remove them now that
	// the canonical pod owns the bindings.
	if d.podmanClient != nil {
		for _, legacy := range legacyPodCandidates(s) {
			if legacy == "" || legacy == podName {
				continue
			}
			if _, err := d.podmanClient.InspectPod(ctx, legacy); err == nil {
				logLine(fmt.Sprintf("Removing legacy pod '%s' (renamed to '%s')...", legacy, podName))
				delErr := d.podmanClient.DeletePod(ctx, legacy, true)
				if delErr == nil {
					logLine(fmt.Sprintf("🧹 Legacy pod cleanup: removed '%s' (superseded by '%s').", legacy, podName))
				} else {
					logLine(fmt.Sprintf("🧹 Legacy pod cleanup: '%s' removal attempted (superseded by '%s'): %v", legacy, podName, delErr))
				}
			}
		}
	}
	d.removeLegacyPodQuadlets(s, podName)

	d.writePodQuadletArtifacts(s, podName, ports, logLine)

	return &DeployResult{
		ContainerName: podName,
		CommitMessage: fmt.Sprintf("Pod active: %s", podName),
		Version:       "pod",
	}, nil
}

// removeLegacyPodQuadlets deletes unit files from previous naming schemes so
// systemd never resurrects a shadow pod after reboot.
func (d *Deployer) removeLegacyPodQuadlets(s Service, canonicalPod string) {
	defer func() { _ = recover() }()
	dirs := []string{filepath.Join(d.dataDir, "quadlets")}
	if qd := os.Getenv("QUADLET_DIR"); qd != "" {
		dirs = append(dirs, qd)
	} else if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		dirs = append(dirs, filepath.Join(homeDir, ".config", "containers", "systemd"))
	}
	for _, legacy := range legacyPodCandidates(s) {
		if legacy == "" || legacy == canonicalPod {
			continue
		}
		for _, dir := range dirs {
			_ = os.Remove(filepath.Join(dir, legacy+".pod"))
			_ = os.Remove(filepath.Join(dir, legacy+".container"))
		}
	}
}

// writePodQuadletArtifacts persists declarative .pod (+ member .container) units for
// the pod so systemd Quadlet can re-create it after reboot. Best-effort: never fails deploy.
func (d *Deployer) writePodQuadletArtifacts(s Service, podName string, ports []string, logLine func(string)) {
	defer func() { _ = recover() }()
	content := buildPodQuadletBundle(s, podName, ports)
	if strings.TrimSpace(content) == "" {
		return
	}
	files := SplitQuadletUnits(content, strings.TrimPrefix(podName, "pod_"))
	if len(files) == 0 {
		return
	}
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	_ = os.MkdirAll(quadletDir, 0755)
	for fname, uContent := range files {
		dst := filepath.Join(quadletDir, fname)
		_ = os.WriteFile(dst, []byte(uContent), 0644)
		if logLine != nil {
			logLine(fmt.Sprintf("📄 Quadlet artifact staged: %s", dst))
		}
	}
	systemdDir := os.Getenv("QUADLET_DIR")
	if systemdDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	if systemdDir != "" {
		_ = os.MkdirAll(systemdDir, 0755)
		for fname, uContent := range files {
			dst := filepath.Join(systemdDir, fname)
			_ = os.WriteFile(dst, []byte(uContent), 0644)
			if logLine != nil {
				logLine(fmt.Sprintf("📄 Quadlet artifact written: %s", dst))
			}
		}
		if logLine != nil {
			logLine(fmt.Sprintf("Wrote %d Quadlet unit(s) for pod '%s' (.pod + members).", len(files), podName))
		}
	}
}

// buildPodQuadletBundle renders a .pod unit plus one .container unit per known member.
// Ports live only on the .pod (Podman Quadlet spec); members reference Pod= and omit Network/PublishPort.
func buildPodQuadletBundle(s Service, podName string, ports []string) string {
	var publishes strings.Builder
	for _, p := range ports {
		// Quadlet PublishPort accepts [[ip:]host:]container[/proto]:
		// preserve a non-default bind IP, omit 0.0.0.0 (default).
		if n := canonicalPortBinding(p); n != "" {
			publishes.WriteString(fmt.Sprintf("PublishPort=%s\n", n))
		}
	}
	podUnit := fmt.Sprintf("# File: %s.pod\n[Unit]\nDescription=GoPod %s Pod\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Pod]\nPodName=%s\nNetwork=gopod-net\nInfraImage=registry.k8s.io/pause:3.9\nShare=net,uts,ipc\n%s[Install]\nWantedBy=default.target\n",
		podName, podName, podName, publishes.String())
	return podUnit
}
