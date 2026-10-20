package services

import (
	"context"
	"encoding/json"
	"errors"
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

// podMemberServices returns the other project services attached to this pod,
// using the same matching rules as the runtime member reconciliation.
func (d *Deployer) podMemberServices(ctx context.Context, s Service) []Service {
	if d.repo == nil {
		return nil
	}
	allSvcs, err := d.repo.List(ctx, s.ProjectID)
	if err != nil {
		return nil
	}
	expectedPod := PodNameForService(s)
	legacyNames := legacyPodCandidates(s)
	var members []Service
	for _, m := range allSvcs {
		if m.ID == s.ID {
			continue
		}
		isMember := m.InPod && (m.PodID == s.ID || m.PodID == s.Name || m.PodID == expectedPod ||
			(strings.TrimSpace(m.PodID) != "" && PodNameFor(m.PodID) == PodNameFor(expectedPod)))
		if !isMember && m.InPod {
			for _, leg := range legacyNames {
				if m.PodID == leg {
					isMember = true
					break
				}
			}
		}
		if isMember {
			members = append(members, m)
		}
	}
	return members
}

// hasAttachedMemberServices checks if other services in the project are attached to this pod.
func (d *Deployer) hasAttachedMemberServices(ctx context.Context, s Service) bool {
	return len(d.podMemberServices(ctx, s)) > 0
}

// hasK8sWorkloadContainers checks whether the Kubernetes manifest defines at least one non-infra workload container.
func hasK8sWorkloadContainers(k8sYaml string) bool {
	trimmed := strings.TrimSpace(k8sYaml)
	if trimmed == "" {
		return false
	}
	var doc struct {
		Spec struct {
			Containers []struct {
				Name  string `yaml:"name"`
				Image string `yaml:"image"`
			} `yaml:"containers"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(trimmed), &doc); err != nil {
		return false
	}
	for _, c := range doc.Spec.Containers {
		name := strings.ToLower(strings.TrimSpace(c.Name))
		img := strings.ToLower(strings.TrimSpace(c.Image))
		if name == "infra" || strings.Contains(img, "pause") || (img == "" && name == "") {
			continue
		}
		return true
	}
	return false
}

// prepareK8sPodManifest injects canonical pod name, GOPOD tracing labels, and synchronizes desired ports into a K8s Pod manifest.
func prepareK8sPodManifest(raw string, podName, projectID, serviceID string, desiredPorts []string) (string, int, error) {
	var doc struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
		Spec struct {
			Containers []struct {
				Name string `yaml:"name"`
			} `yaml:"containers"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return "", 0, fmt.Errorf("invalid Kubernetes YAML: %w", err)
	}

	containerCount := len(doc.Spec.Containers)
	if containerCount == 0 {
		return "", 0, fmt.Errorf("no containers declared in spec.containers")
	}

	var generic map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &generic); err == nil {
		meta, _ := generic["metadata"].(map[string]interface{})
		if meta == nil {
			meta = make(map[string]interface{})
		}
		meta["name"] = podName
		labels, _ := meta["labels"].(map[string]interface{})
		if labels == nil {
			labels = make(map[string]interface{})
		}
		labels["io.gopod.project"] = projectID
		labels["io.gopod.service"] = serviceID
		meta["labels"] = labels
		generic["metadata"] = meta

		// Synchronize desired ports (from compose/visual view) into the primary container if not already declared
		if specMap, ok := generic["spec"].(map[string]interface{}); ok {
			if containersList, ok := specMap["containers"].([]interface{}); ok && len(containersList) > 0 {
				existingPortKeys := make(map[string]bool)
				for _, cRaw := range containersList {
					if cMap, ok := cRaw.(map[string]interface{}); ok {
						if portsRaw, ok := cMap["ports"].([]interface{}); ok {
							for _, pItem := range portsRaw {
								if pMap, ok := pItem.(map[string]interface{}); ok {
									cp, _ := pMap["containerPort"].(int)
									hp, _ := pMap["hostPort"].(int)
									ip, _ := pMap["hostIP"].(string)
									if ip == "" {
										ip = "0.0.0.0"
									}
									proto, _ := pMap["protocol"].(string)
									if strings.ToLower(proto) == "udp" {
										existingPortKeys[fmt.Sprintf("%s:%d:%d/udp", ip, hp, cp)] = true
									} else {
										existingPortKeys[fmt.Sprintf("%s:%d:%d", ip, hp, cp)] = true
									}
								}
							}
						}
					}
				}

				if primaryMap, ok := containersList[0].(map[string]interface{}); ok {
					var pList []interface{}
					if rawPorts, ok := primaryMap["ports"].([]interface{}); ok {
						pList = rawPorts
					}
					for _, dp := range desiredPorts {
						ip, hp, cp, udp, ok := parsePortBinding(dp)
						if !ok {
							continue
						}
						normIp := ip
						if normIp == "" {
							normIp = "0.0.0.0"
						}
						protoStr := ""
						if udp {
							protoStr = "/udp"
						}
						key := fmt.Sprintf("%s:%d:%d%s", normIp, hp, cp, protoStr)
						if !existingPortKeys[key] {
							newBinding := map[string]interface{}{
								"containerPort": cp,
								"hostPort":      hp,
							}
							if udp {
								newBinding["protocol"] = "UDP"
							}
							if ip != "" && ip != "0.0.0.0" {
								newBinding["hostIP"] = ip
							}
							pList = append(pList, newBinding)
							existingPortKeys[key] = true
						}
					}
					primaryMap["ports"] = pList
					containersList[0] = primaryMap
					specMap["containers"] = containersList
					generic["spec"] = specMap
				}
			}
		}

		out, err := yaml.Marshal(generic)
		if err == nil {
			return string(out), containerCount, nil
		}
	}

	doc.Metadata.Name = podName
	if doc.Metadata.Labels == nil {
		doc.Metadata.Labels = make(map[string]string)
	}
	doc.Metadata.Labels["io.gopod.project"] = projectID
	doc.Metadata.Labels["io.gopod.service"] = serviceID

	out, err := yaml.Marshal(&doc)
	return string(out), containerCount, err
}

// deployPodKube deploys a declarative multi-container Kubernetes manifest directly using podman play kube.
func (d *Deployer) deployPodKube(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	podName := PodNameForService(s)
	logLine(fmt.Sprintf("Reconciling Kubernetes Pod Manifest for '%s' (service: %s)...", podName, s.Name))

	ports := extractPodPorts(s)
	content, containerCount, err := prepareK8sPodManifest(s.K8sYaml, podName, s.ProjectID, s.ID, ports)
	if err != nil {
		logLine(fmt.Sprintf("❌ Failed to parse Kubernetes manifest: %v", err))
		return nil, err
	}

	// Pre-flight validation & collision check
	valRes := ValidateKubernetesPodManifest(ctx, content)
	if !valRes.Valid {
		errMsg := strings.Join(valRes.Errors, "; ")
		logLine(fmt.Sprintf("❌ Manifest validation failed: %s", errMsg))
		return nil, fmt.Errorf("manifest validation failed: %s", errMsg)
	}
	for _, w := range valRes.Warnings {
		logLine(fmt.Sprintf("⚠️ %s", w))
	}

	kubeDir := filepath.Join(d.dataDir, "kube", s.ProjectID, podName)
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		logLine(fmt.Sprintf("❌ Failed to create kube directory: %v", err))
		return nil, err
	}

	kubeFile := filepath.Join(kubeDir, "manifest.yaml")
	if err := os.WriteFile(kubeFile, []byte(content), 0644); err != nil {
		logLine(fmt.Sprintf("❌ Failed to write manifest: %v", err))
		return nil, err
	}

	if d.podmanClient != nil {
		_ = d.podmanClient.EnsureNetwork(ctx, "gopod-net")
	}

	args := []string{"play", "kube", "--network", "gopod-net", "--replace", kubeFile}
	logLine(fmt.Sprintf("⏳ Executing: podman %s...", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "podman", args...)
	d.setupCmdEnv(cmd)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		logLine(fmt.Sprintf("❌ podman play kube start failed: %v", err))
		return nil, err
	}

	errLines := d.streamPipes(stdout, stderr, logLine)
	if err := cmd.Wait(); err != nil {
		errMsg := fmt.Sprintf("podman play kube failed: %v", err)
		if len(errLines) > 0 {
			errMsg = fmt.Sprintf("podman play kube failed: %s", strings.Join(errLines, " "))
		}
		logLine(fmt.Sprintf("❌ %s", errMsg))
		return nil, errors.New(errMsg)
	}

	// Verify outcome
	hasFatal := false
	var sampleErr string
	for _, l := range errLines {
		lower := strings.ToLower(l)
		if strings.HasPrefix(l, "Error:") || strings.Contains(lower, "failed to start") || strings.Contains(lower, "address already in use") {
			hasFatal = true
			if sampleErr == "" {
				sampleErr = l
			}
		}
	}

	if hasFatal {
		errMsg := fmt.Sprintf("Kubernetes deployment failed: %s", sampleErr)
		logLine(fmt.Sprintf("❌ %s", errMsg))
		return nil, errors.New(errMsg)
	}

	logLine(fmt.Sprintf("✅ Kubernetes Pod '%s' played successfully (%d workload containers running).", podName, containerCount))

	// Remove leftover pods from the legacy naming scheme.
	if d.podmanClient != nil {
		if pods, err := d.podmanClient.GetPods(ctx); err == nil {
			for _, legacy := range legacyPodCandidates(s) {
				if legacy == "" || legacy == podName {
					continue
				}
				for _, p := range pods {
					if p.Name != legacy || !podLegacyCandidateOwned(p, s) {
						continue
					}
					_ = d.podmanClient.DeletePod(ctx, legacy, true)
					logLine(fmt.Sprintf("🧹 Legacy pod cleanup: removed '%s'.", legacy))
					break
				}
			}
		}
	}
	d.removeLegacyPodQuadlets(s, podName)

	return &DeployResult{
		ContainerName: podName,
		CommitMessage: fmt.Sprintf("Kubernetes Pod active: %s (%d containers)", podName, containerCount),
		Version:       "kube",
	}, nil
}

// deployPod manages Podman Pod lifecycle, infra container creation, and port publishing.
func (d *Deployer) deployPod(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	if !d.hasAttachedMemberServices(ctx, s) && hasK8sWorkloadContainers(s.K8sYaml) {
		return d.deployPodKube(ctx, s, depID, logLine)
	}

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

	// Sweep leftover pods from the legacy naming scheme (pod_<name>): they
	// hold the same host ports as the canonical pod and would shadow its
	// bindings. Ownership-checked: a candidate is removed only when its labels
	// prove this gopod project owns it, so same-named services across projects
	// can never delete each other's pods.
	if d.podmanClient != nil {
		if pods, err := d.podmanClient.GetPods(ctx); err == nil {
			for _, legacy := range legacyPodCandidates(s) {
				if legacy == "" || legacy == podName {
					continue
				}
				for _, p := range pods {
					if p.Name != legacy || !podLegacyCandidateOwned(p, s) {
						continue
					}
					logLine(fmt.Sprintf("Removing legacy pod '%s' (renamed to '%s')...", legacy, podName))
					delErr := d.podmanClient.DeletePod(ctx, legacy, true)
					if delErr == nil {
						logLine(fmt.Sprintf("🧹 Legacy pod cleanup: removed '%s' (superseded by '%s').", legacy, podName))
					} else {
						logLine(fmt.Sprintf("🧹 Legacy pod cleanup: '%s' removal attempted (superseded by '%s'): %v", legacy, podName, delErr))
					}
					break
				}
			}
		}
	}
	d.removeLegacyPodQuadlets(s, podName)

	// Quadlet persistence/recovery: the runtime path stays podman, but the
	// declarative bundle (plus one .container unit per attached child) is
	// persisted so host systemd can re-create the pod after reboot. The exact
	// on-disk bundle is recorded on the service row for the delete cleanup.
	if s.Type == "pod" || s.Type == "quadlet" || s.RuntimeTarget == "quadlet" {
		d.writePodQuadletArtifacts(ctx, s, podName, ports, logLine)
	}

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
// the pod so systemd Quadlet can re-create it after reboot. Best-effort: never fails
// deploy. The exact bundle is recorded on the service record so the delete path can
// remove every unit it wrote (member units otherwise leak and reconnect on boot).
func (d *Deployer) writePodQuadletArtifacts(ctx context.Context, s Service, podName string, ports []string, logLine func(string)) {
	defer func() { _ = recover() }()
	var members []Service
	if d.repo != nil {
		members = d.podMemberServices(ctx, s)
	}
	content := buildPodQuadletBundle(s, podName, ports, members)
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
	// Record what was actually written so the service-delete path (which parses
	// this exact field) can remove every unit, member units included.
	if d.repo != nil && strings.TrimSpace(s.QuadletConfig) != strings.TrimSpace(content) {
		if stored, err := d.repo.Get(ctx, s.ID); err == nil && stored != nil {
			stored.QuadletConfig = content
			_ = d.repo.Update(ctx, *stored)
		}
	}
}

// buildPodQuadletBundle renders a .pod unit plus one .container unit per known member.
// Ports live only on the .pod (Podman Quadlet spec); members reference Pod= and omit Network/PublishPort.
func buildPodQuadletBundle(s Service, podName string, ports []string, members []Service) string {
	content := buildPodQuadletUnit(s, podName, ports)
	for _, m := range members {
		if m.ID == s.ID {
			continue
		}
		content += "\n---\n" + podMemberQuadletUnit(podName, m)
	}
	return content
}

// buildPodQuadletUnit renders only the .pod unit.
func buildPodQuadletUnit(s Service, podName string, ports []string) string {
	var publishes strings.Builder
	for _, p := range ports {
		// Quadlet PublishPort accepts [[ip:]host:]container[/proto]:
		// preserve a non-default bind IP, omit 0.0.0.0 (default).
		if n := canonicalPortBinding(p); n != "" {
			publishes.WriteString(fmt.Sprintf("PublishPort=%s\n", n))
		}
	}
	var podLabels strings.Builder
	if s.ID != "" {
		podLabels.WriteString(fmt.Sprintf("Label=io.gopod.service=%s\n", s.ID))
		if s.ProjectID != "" {
			podLabels.WriteString(fmt.Sprintf("Label=io.gopod.project=%s\n", s.ProjectID))
		}
	}
	// NOTE: [Pod] has no InfraImage=/Share= keys in the Quadlet generator; an
	// unknown key aborts conversion of the whole unit. podman's builtin infra
	// image (registry.k8s.io/pause) is used implicitly, and namespace sharing is
	// requested explicitly via PodmanArgs so the pod keeps sharing net/uts/ipc.
	podUnit := fmt.Sprintf("# File: %s.pod\n[Unit]\nDescription=GoPod %s Pod\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Pod]\nPodName=%s\nNetwork=gopod-net\nPodmanArgs=--share=net,uts,ipc\n%s%s[Install]\nWantedBy=default.target\n",
		podName, podName, podName, podLabels.String(), publishes.String())
	return podUnit
}

// podMemberQuadletUnit renders one .container unit for a child service joined to
// the pod. The member carries env, volumes, and the Advanced-mapped directives;
// ports live on the .pod (PublishPort in [Pod]) because a container joined to a
// pod publishes through the pod in the Quadlet generator.
func podMemberQuadletUnit(podName string, m Service) string {
	memberSlug := SanitizeQuadletSlug(m.ID)
	if memberSlug == "" {
		memberSlug = SanitizeQuadletSlug(m.Name)
	}
	image := m.Image
	if image == "" || ValidateContainerImage(image) != nil {
		if ext := ExtractFirstImageFromCompose(m.ComposeYaml); ext != "" && ValidateContainerImage(ext) == nil {
			image = ext
		}
		if image == "" || ValidateContainerImage(image) != nil {
			image = "docker.io/library/nginx:alpine"
		}
	}
	restart := MapDockerRestartToSystemd(m.RestartPolicy)
	if restart == "" {
		restart = "always"
	}
	var labels strings.Builder
	labels.WriteString(fmt.Sprintf("Label=io.gopod.service=%s\n", m.ID))
	if m.ProjectID != "" {
		labels.WriteString(fmt.Sprintf("Label=io.gopod.project=%s\n", m.ProjectID))
	}
	return fmt.Sprintf("# File: %s.container\n[Unit]\nDescription=GoPod Pod Member %s\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nPod=%s.pod\n%s%s[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
		memberSlug, m.Name, memberSlug, image, podName, labels.String(), podMemberExtraDirectives(m), restart)
}

// podMemberExtraDirectives renders a member's env, volumes, and Advanced-mapped
// keys — the same mapping the single-container generator applies.
func podMemberExtraDirectives(m Service) string {
	var b strings.Builder
	for _, env := range m.EnvVars {
		if env.Key != "" && !strings.HasPrefix(env.Key, "#") {
			b.WriteString(fmt.Sprintf("Environment=\"%s=%s\"\n", env.Key, env.Value))
		}
	}
	if a := m.Advanced; a != nil {
		if a.Runtime.UserNamespace != "" && a.Runtime.UserNamespace != "default" && a.Runtime.UserNamespace != "none" {
			b.WriteString(fmt.Sprintf("UserNS=%s\n", a.Runtime.UserNamespace))
		}
		for _, v := range a.Storage.Volumes {
			if v.Source != "" && v.Target != "" {
				volSource := SanitizeQuadletSlug(v.Source)
				if v.Options != "" {
					b.WriteString(fmt.Sprintf("Volume=%s:%s:%s\n", volSource, v.Target, v.Options))
				} else {
					b.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", volSource, v.Target))
				}
			}
		}
		for _, cap := range a.Security.CapAdd {
			if cap != "" {
				b.WriteString(fmt.Sprintf("AddCapability=%s\n", cap))
			}
		}
		for _, cap := range a.Security.CapDrop {
			if cap != "" {
				b.WriteString(fmt.Sprintf("DropCapability=%s\n", cap))
			}
		}
		for _, dev := range a.Runtime.Devices {
			if dev != "" {
				b.WriteString(fmt.Sprintf("AddDevice=%s\n", dev))
			}
		}
		if a.Security.Privileged || a.Security.SELinuxLabel == "disable" {
			b.WriteString("SecurityLabelDisable=true\n")
		} else if a.Security.SELinuxLabel != "" && a.Security.SELinuxLabel != "container_file_t" {
			b.WriteString(fmt.Sprintf("SecurityLabelType=%s\n", a.Security.SELinuxLabel))
		}
		if a.Security.AppArmorProfile != "" {
			b.WriteString(fmt.Sprintf("AppArmorProfile=%s\n", a.Security.AppArmorProfile))
		}
	}
	return b.String()
}
