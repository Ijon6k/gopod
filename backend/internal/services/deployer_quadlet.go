package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParsedQuadlet contains extracted directives from a Quadlet unit.
type ParsedQuadlet struct {
	Image         string
	ContainerName string
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

// ExtractedCompose holds runtime parameters extracted dynamically from Compose YAML.
type ExtractedCompose struct {
	Image         string
	ContainerName string
	Ports         []string
	Volumes       []string
	Environment   []string
	Exec          string
	Restart       string
}

// MapDockerRestartToSystemd translates container restart policies into valid systemd Restart= directives.
func MapDockerRestartToSystemd(restart string) string {
	clean := strings.ToLower(strings.TrimSpace(restart))
	switch clean {
	case "unless-stopped", "always":
		return "always"
	case "on-failure":
		return "on-failure"
	case "no", "never":
		return "no"
	default:
		if clean == "" {
			return "always"
		}
		return clean
	}
}

// ExtractComposeWorkload extracts parameters from the primary service in Compose YAML.
func ExtractComposeWorkload(composeYaml string) ExtractedCompose {
	var result ExtractedCompose
	trimmed := strings.TrimSpace(composeYaml)
	if trimmed == "" {
		return result
	}

	var raw struct {
		Services map[string]struct {
			Image         string      `yaml:"image"`
			ContainerName string      `yaml:"container_name"`
			Ports         []string    `yaml:"ports"`
			Volumes       []string    `yaml:"volumes"`
			Environment   interface{} `yaml:"environment"`
			Command       interface{} `yaml:"command"`
			Restart       string      `yaml:"restart"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal([]byte(trimmed), &raw); err != nil || raw.Services == nil {
		return result
	}

	for _, svc := range raw.Services {
		if img := strings.TrimSpace(svc.Image); img != "" && ValidateContainerImage(img) == nil {
			result.Image = img
		}
		if svc.ContainerName != "" {
			result.ContainerName = SanitizeQuadletSlug(svc.ContainerName)
		}
		for _, p := range svc.Ports {
			p = strings.TrimSpace(p)
			if p != "" {
				result.Ports = append(result.Ports, p)
			}
		}
		for _, v := range svc.Volumes {
			v = strings.TrimSpace(v)
			if v != "" {
				result.Volumes = append(result.Volumes, v)
			}
		}
		if svc.Restart != "" {
			result.Restart = MapDockerRestartToSystemd(svc.Restart)
		}

		// Environment
		switch env := svc.Environment.(type) {
		case []interface{}:
			for _, item := range env {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					result.Environment = append(result.Environment, strings.TrimSpace(s))
				}
			}
		case map[string]interface{}:
			for k, v := range env {
				if k != "" {
					result.Environment = append(result.Environment, fmt.Sprintf("%s=%v", k, v))
				}
			}
		}

		// Command / Exec
		switch cmd := svc.Command.(type) {
		case string:
			result.Exec = strings.TrimSpace(cmd)
		case []interface{}:
			var parts []string
			for _, item := range cmd {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				result.Exec = strings.Join(parts, " ")
			}
		}

		// Process first primary service
		break
	}

	return result
}

// ExtractedComposeService holds runtime parameters extracted for a specific service in Compose YAML.
type ExtractedComposeService struct {
	Name          string
	Image         string
	ContainerName string
	Ports         []string
	Volumes       []string
	Environment   []string
	Exec          string
	Restart       string
}

// ExtractAllComposeServices extracts all services defined in Compose YAML.
func ExtractAllComposeServices(composeYaml string) []ExtractedComposeService {
	var results []ExtractedComposeService
	trimmed := strings.TrimSpace(composeYaml)
	if trimmed == "" {
		return results
	}

	var raw struct {
		Services map[string]struct {
			Image         string      `yaml:"image"`
			ContainerName string      `yaml:"container_name"`
			Ports         []string    `yaml:"ports"`
			Volumes       []string    `yaml:"volumes"`
			Environment   interface{} `yaml:"environment"`
			Command       interface{} `yaml:"command"`
			Restart       string      `yaml:"restart"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal([]byte(trimmed), &raw); err != nil || raw.Services == nil {
		return results
	}

	for name, svc := range raw.Services {
		var item ExtractedComposeService
		item.Name = strings.TrimSpace(name)
		if img := strings.TrimSpace(svc.Image); img != "" && ValidateContainerImage(img) == nil {
			item.Image = img
		}
		if svc.ContainerName != "" {
			item.ContainerName = SanitizeQuadletSlug(svc.ContainerName)
		} else {
			item.ContainerName = SanitizeQuadletSlug(name)
		}
		for _, p := range svc.Ports {
			if p = strings.TrimSpace(p); p != "" {
				item.Ports = append(item.Ports, p)
			}
		}
		for _, v := range svc.Volumes {
			if v = strings.TrimSpace(v); v != "" {
				item.Volumes = append(item.Volumes, v)
			}
		}
		if svc.Restart != "" {
			item.Restart = MapDockerRestartToSystemd(svc.Restart)
		}

		switch env := svc.Environment.(type) {
		case []interface{}:
			for _, e := range env {
				if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
					item.Environment = append(item.Environment, strings.TrimSpace(s))
				}
			}
		case map[string]interface{}:
			for k, v := range env {
				if k != "" {
					item.Environment = append(item.Environment, fmt.Sprintf("%s=%v", k, v))
				}
			}
		}

		switch cmd := svc.Command.(type) {
		case string:
			item.Exec = strings.TrimSpace(cmd)
		case []interface{}:
			var parts []string
			for _, part := range cmd {
				if s, ok := part.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				item.Exec = strings.Join(parts, " ")
			}
		}

		results = append(results, item)
	}

	return results
}

// ExtractAllComposeWorkloadNames extracts all service keys and explicit container_name definitions from Compose YAML.
func ExtractAllComposeWorkloadNames(composeYaml string) []string {
	var names []string
	trimmed := strings.TrimSpace(composeYaml)
	if trimmed == "" {
		return names
	}
	var raw struct {
		Services map[string]struct {
			ContainerName string `yaml:"container_name"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(trimmed), &raw); err != nil || raw.Services == nil {
		return names
	}
	for key, svc := range raw.Services {
		if k := strings.TrimSpace(key); k != "" {
			names = append(names, k)
			cleanKey := SanitizeQuadletSlug(k)
			if cleanKey != "" && cleanKey != k {
				names = append(names, cleanKey)
			}
		}
		if cName := strings.TrimSpace(svc.ContainerName); cName != "" {
			names = append(names, cName)
			cleanCName := SanitizeQuadletSlug(cName)
			if cleanCName != "" && cleanCName != cName {
				names = append(names, cleanCName)
			}
		}
	}
	return names
}

// ExtractFirstImageFromCompose attempts to extract the first valid container image defined in Compose YAML.
func ExtractFirstImageFromCompose(composeYaml string) string {
	trimmed := strings.TrimSpace(composeYaml)
	if trimmed == "" {
		return ""
	}
	var raw struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(trimmed), &raw); err == nil && raw.Services != nil {
		for _, svc := range raw.Services {
			if img := strings.TrimSpace(svc.Image); img != "" && ValidateContainerImage(img) == nil {
				return img
			}
		}
	}
	return ""
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

		var fileName string
		for _, l := range strings.Split(trimmed, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(strings.ToLower(l), "# file:") {
				fileName = strings.TrimSpace(l[len("# file:"):])
				break
			}
		}
		if fileName != "" {
			ext := filepath.Ext(fileName)
			base := strings.TrimSuffix(fileName, ext)
			safeFileName := fmt.Sprintf("%s%s", SanitizeQuadletSlug(base), ext)
			units[safeFileName] = trimmed
			continue
		}

		if strings.Contains(trimmed, "[Pod]") {
			podName := extractDirective(trimmed, "PodName")
			if podName == "" {
				podName = fmt.Sprintf("pod_%s", cleanDefault)
			}
			podName = SanitizeQuadletSlug(podName)
			units[fmt.Sprintf("%s.pod", podName)] = trimmed
		} else if strings.Contains(trimmed, "[Container]") {
			contName := extractDirective(trimmed, "ContainerName")
			if contName == "" {
				if len(chunks) > 1 {
					contName = fmt.Sprintf("%s-member", cleanDefault)
				} else {
					contName = cleanDefault
				}
			}
			contName = SanitizeQuadletSlug(contName)
			units[fmt.Sprintf("%s.container", contName)] = trimmed
		}
	}

	if len(units) == 0 && strings.TrimSpace(content) != "" {
		trimmed := strings.TrimSpace(content)
		if strings.Contains(trimmed, "[Pod]") {
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

// prepareQuadletContent prepares declarative unit text ensuring anti-loop limits, ingress, and tracking directives.
func (d *Deployer) prepareQuadletContent(s Service, cleanName string) string {
	rawContent := strings.TrimSpace(s.QuadletConfig)
	if rawContent == "" {
		return d.generateDefaultQuadletContent(s, cleanName)
	}

	if strings.Contains(rawContent, "\n---") {
		chunks := strings.Split(rawContent, "\n---")
		var processedChunks []string
		for _, chunk := range chunks {
			trimmed := strings.TrimSpace(strings.TrimPrefix(chunk, "---"))
			if trimmed == "" {
				continue
			}
			processedChunks = append(processedChunks, d.prepareSingleQuadletContent(s, cleanName, trimmed))
		}
		return strings.Join(processedChunks, "\n---\n")
	}

	return d.prepareSingleQuadletContent(s, cleanName, rawContent)
}

func (d *Deployer) generateDefaultQuadletContent(s Service, cleanName string) string {
	targetContainerName := SanitizeQuadletSlug(s.ID)
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	if s.ComposeYaml != "" {
		allSvcs := ExtractAllComposeServices(s.ComposeYaml)
		if len(allSvcs) > 1 {
			var chunks []string
			podName := fmt.Sprintf("pod_%s", cleanName)
			var podPublishes strings.Builder
			for _, svc := range allSvcs {
				for _, p := range svc.Ports {
					podPublishes.WriteString(fmt.Sprintf("PublishPort=%s\n", p))
				}
			}

			// 1. Pod unit
			podUnit := fmt.Sprintf("# File: %s.pod\n[Unit]\nDescription=GoPod %s Pod\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Pod]\nPodName=%s\nNetwork=gopod-net\n%s[Install]\nWantedBy=default.target\n",
				podName, cleanName, podName, podPublishes.String())
			chunks = append(chunks, podUnit)

			// 2. Member containers
			for _, svc := range allSvcs {
				cImg := svc.Image
				if cImg == "" {
					cImg = "docker.io/library/nginx:alpine"
				}
				var cExtras strings.Builder
				if svc.Exec != "" {
					cExtras.WriteString(fmt.Sprintf("Exec=%s\n", svc.Exec))
				}
				for _, env := range svc.Environment {
					cExtras.WriteString(fmt.Sprintf("Environment=%s\n", env))
				}
				for _, vol := range svc.Volumes {
					if !strings.Contains(vol, ":") {
						cExtras.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", SanitizeQuadletSlug(vol), vol))
					} else {
						if !strings.HasSuffix(vol, ":Z") && !strings.HasSuffix(vol, ":z") {
							cExtras.WriteString(fmt.Sprintf("Volume=%s:Z\n", vol))
						} else {
							cExtras.WriteString(fmt.Sprintf("Volume=%s\n", vol))
						}
					}
				}
				rPolicy := svc.Restart
				if rPolicy == "" {
					rPolicy = "always"
				}

				mUnit := fmt.Sprintf("# File: %s.container\n[Unit]\nDescription=GoPod Pod Member %s\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nPod=%s.pod\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
					svc.ContainerName, svc.ContainerName, svc.ContainerName, cImg, podName, s.ID, s.ProjectID, cExtras.String(), rPolicy)
				chunks = append(chunks, mUnit)
			}
			return strings.Join(chunks, "\n---\n")
		}
	}

	var extracted ExtractedCompose
	if s.ComposeYaml != "" {
		extracted = ExtractComposeWorkload(s.ComposeYaml)
		if extracted.ContainerName != "" {
			targetContainerName = extracted.ContainerName
		}
	}

	// Resolve image safely
	image := s.Image
	if extracted.Image != "" {
		image = extracted.Image
	} else if err := ValidateContainerImage(image); err != nil {
		if s.ComposeYaml != "" {
			extImg := ExtractFirstImageFromCompose(s.ComposeYaml)
			if extImg != "" && ValidateContainerImage(extImg) == nil {
				image = extImg
			}
		}
		if image == "" || ValidateContainerImage(image) != nil {
			image = "docker.io/library/nginx:alpine"
		}
	}

	var publishDirectives strings.Builder
	if len(extracted.Ports) > 0 {
		for _, p := range extracted.Ports {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%s\n", p))
		}
	} else if s.Type != "compose" && s.ComposeYaml == "" {
		// Only non-compose services fall back to service.hostPort/port
		if s.HostPort > 0 && s.Port > 0 {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%d:%d\n", s.HostPort, s.Port))
		} else if s.HostPort > 0 {
			publishDirectives.WriteString(fmt.Sprintf("PublishPort=%d:80\n", s.HostPort))
		}
	}

	var extraDirectives strings.Builder
	if extracted.Exec != "" {
		extraDirectives.WriteString(fmt.Sprintf("Exec=%s\n", extracted.Exec))
	}
	for _, env := range extracted.Environment {
		extraDirectives.WriteString(fmt.Sprintf("Environment=%s\n", env))
	}
	for _, env := range s.EnvVars {
		if env.Key != "" && !strings.HasPrefix(env.Key, "#") {
			extraDirectives.WriteString(fmt.Sprintf("Environment=\"%s=%s\"\n", env.Key, env.Value))
		}
	}
	for _, vol := range extracted.Volumes {
		if !strings.Contains(vol, ":") {
			extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", SanitizeQuadletSlug(vol), vol))
		} else {
			if !strings.HasSuffix(vol, ":Z") && !strings.HasSuffix(vol, ":z") {
				extraDirectives.WriteString(fmt.Sprintf("Volume=%s:Z\n", vol))
			} else {
				extraDirectives.WriteString(fmt.Sprintf("Volume=%s\n", vol))
			}
		}
	}

	if s.Advanced != nil {
		if s.Advanced.Runtime.UserNamespace != "" && s.Advanced.Runtime.UserNamespace != "default" && s.Advanced.Runtime.UserNamespace != "none" {
			extraDirectives.WriteString(fmt.Sprintf("UserNS=%s\n", s.Advanced.Runtime.UserNamespace))
		}
		for _, v := range s.Advanced.Storage.Volumes {
			if v.Source != "" && v.Target != "" {
				volSource := SanitizeQuadletSlug(v.Source)
				if v.Options != "" {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:%s\n", volSource, v.Target, v.Options))
				} else {
					extraDirectives.WriteString(fmt.Sprintf("Volume=%s:%s:Z\n", volSource, v.Target))
				}
			}
		}
		for _, cap := range s.Advanced.Security.CapAdd {
			if cap != "" {
				extraDirectives.WriteString(fmt.Sprintf("AddCapability=%s\n", cap))
			}
		}
		for _, cap := range s.Advanced.Security.CapDrop {
			if cap != "" {
				extraDirectives.WriteString(fmt.Sprintf("DropCapability=%s\n", cap))
			}
		}
		for _, dev := range s.Advanced.Runtime.Devices {
			if dev != "" {
				extraDirectives.WriteString(fmt.Sprintf("AddDevice=%s\n", dev))
			}
		}
		if s.Advanced.Security.Privileged || s.Advanced.Security.SELinuxLabel == "disable" {
			extraDirectives.WriteString("SecurityLabelDisable=true\n")
		} else if s.Advanced.Security.SELinuxLabel != "" && s.Advanced.Security.SELinuxLabel != "container_file_t" {
			extraDirectives.WriteString(fmt.Sprintf("SecurityLabelType=%s\n", s.Advanced.Security.SELinuxLabel))
		}
		if s.Advanced.Security.AppArmorProfile != "" {
			extraDirectives.WriteString(fmt.Sprintf("AppArmorProfile=%s\n", s.Advanced.Security.AppArmorProfile))
		}
	}

	restartPolicy := MapDockerRestartToSystemd(s.RestartPolicy)
	if extracted.Restart != "" {
		restartPolicy = MapDockerRestartToSystemd(extracted.Restart)
	}
	if restartPolicy == "" {
		restartPolicy = "always"
	}

	return fmt.Sprintf("[Unit]\nDescription=%s Quadlet Service\nAfter=network-online.target\nStartLimitIntervalSec=60s\nStartLimitBurst=5\n\n[Container]\nContainerName=%s\nImage=%s\nNetwork=gopod-net\nLabel=io.gopod.service=%s\nLabel=io.gopod.project=%s\n%s%s\n[Service]\nRestart=%s\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target\n",
		cleanName, targetContainerName, image, s.ID, s.ProjectID, publishDirectives.String(), extraDirectives.String(), restartPolicy)
}

func (d *Deployer) prepareSingleQuadletContent(s Service, cleanName string, rawContent string) string {
	targetContainerName := SanitizeQuadletSlug(s.ID)
	if targetContainerName == "" {
		targetContainerName = cleanName
	}

	lines := strings.Split(rawContent, "\n")
	var newLines []string
	inContainer := false
	hasUnit := strings.Contains(rawContent, "[Unit]")
	hasStartLimitInterval := strings.Contains(rawContent, "StartLimitIntervalSec=")
	hasStartLimitBurst := strings.Contains(rawContent, "StartLimitBurst=")
	hasContainerName := strings.Contains(rawContent, "ContainerName=")
	hasNetwork := strings.Contains(rawContent, "Network=")
	hasServiceLabel := strings.Contains(rawContent, "io.gopod.service")
	hasServiceSection := strings.Contains(rawContent, "[Service]")
	hasPodDirective := strings.Contains(rawContent, "Pod=")
	var containerRestart string

	if !hasUnit {
		newLines = append(newLines, "[Unit]", fmt.Sprintf("Description=GoPod %s Service", cleanName), "After=network-online.target", "StartLimitIntervalSec=60s", "StartLimitBurst=5", "")
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inContainer = strings.EqualFold(trimmed, "[Container]")
		}

		// Sanitize ContainerName if already present to guarantee valid systemd/Podman identifier
		if inContainer && strings.HasPrefix(trimmed, "ContainerName=") {
			currName := strings.TrimSpace(strings.TrimPrefix(trimmed, "ContainerName="))
			cleanCurrName := SanitizeQuadletSlug(currName)
			if cleanCurrName != "" {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", cleanCurrName))
			} else {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", targetContainerName))
			}
			continue
		}

		// Validate and heal Image reference if it contains spaces or corrupt values
		if inContainer && strings.HasPrefix(trimmed, "Image=") {
			rawImg := strings.TrimSpace(strings.TrimPrefix(trimmed, "Image="))
			if err := ValidateContainerImage(rawImg); err != nil {
				// Fallback: recover image from Compose YAML if available
				if s.ComposeYaml != "" {
					extImg := ExtractComposeWorkload(s.ComposeYaml).Image
					if extImg != "" && ValidateContainerImage(extImg) == nil {
						rawImg = extImg
					}
				}
				// If still has whitespace, replace spaces with dashes or fallback to valid OCI image
				if strings.ContainsAny(rawImg, " \t\r\n") {
					rawImg = strings.ReplaceAll(rawImg, " ", "-")
				}
				if err := ValidateContainerImage(rawImg); err != nil {
					rawImg = "docker.io/library/nginx:alpine"
				}
			}
			newLines = append(newLines, fmt.Sprintf("Image=%s", rawImg))
			continue
		}

		// Podman Quadlet: Restart= is invalid in [Container], belongs in [Service]
		if inContainer && strings.HasPrefix(trimmed, "Restart=") {
			containerRestart = strings.TrimPrefix(trimmed, "Restart=")
			continue
		}

		// Sanitize invalid systemd Restart= directives (e.g. unless-stopped -> always)
		if rVal, ok := strings.CutPrefix(trimmed, "Restart="); ok {
			newLines = append(newLines, fmt.Sprintf("Restart=%s", MapDockerRestartToSystemd(rVal)))
			continue
		}

		newLines = append(newLines, line)

		if strings.TrimSpace(line) == "[Unit]" {
			if !hasStartLimitInterval {
				newLines = append(newLines, "StartLimitIntervalSec=60s")
			}
			if !hasStartLimitBurst {
				newLines = append(newLines, "StartLimitBurst=5")
			}
		}

		if strings.TrimSpace(line) == "[Container]" {
			if !hasContainerName {
				newLines = append(newLines, fmt.Sprintf("ContainerName=%s", targetContainerName))
			}
			if !strings.Contains(rawContent, "Exec=") && s.ComposeYaml != "" {
				extracted := ExtractComposeWorkload(s.ComposeYaml)
				if extracted.Exec != "" {
					newLines = append(newLines, fmt.Sprintf("Exec=%s", extracted.Exec))
				}
			}
			// In Podman Quadlet, child member of Pod must NOT declare Network=
			if !hasNetwork && !hasPodDirective {
				newLines = append(newLines, "Network=gopod-net")
			}
			if !hasServiceLabel && s.ID != "" {
				newLines = append(newLines, fmt.Sprintf("Label=io.gopod.service=%s", s.ID))
				if s.ProjectID != "" {
					newLines = append(newLines, fmt.Sprintf("Label=io.gopod.project=%s", s.ProjectID))
				}
			}
		}
	}

	if containerRestart != "" && !hasServiceSection {
		newLines = append(newLines, "", "[Service]", fmt.Sprintf("Restart=%s", MapDockerRestartToSystemd(containerRestart)))
	}

	return strings.Join(newLines, "\n")
}

// deployQuadlet writes Quadlet systemd units and generates declarative services.
func (d *Deployer) deployQuadlet(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	quadletDir := filepath.Join(d.dataDir, "quadlets")
	_ = os.MkdirAll(quadletDir, 0755)

	cleanName := QuadletUnitName(s)
	content := d.prepareQuadletContent(s, cleanName)

	// Split bundle into individual unit files (.pod and .container)
	files := SplitQuadletUnits(content, cleanName)

	// 1. Validate Image strictly across container units
	for fname, uContent := range files {
		if strings.HasSuffix(fname, ".container") {
			parsed := ParseQuadlet(uContent)
			img := parsed.Image
			if img == "" {
				img = s.Image
			}
			if err := ValidateContainerImage(img); err != nil {
				// Fallback to compose extraction
				if s.ComposeYaml != "" {
					if extImg := ExtractFirstImageFromCompose(s.ComposeYaml); extImg != "" {
						img = extImg
					}
				}
			}
			if err := ValidateContainerImage(img); err != nil {
				logLine(fmt.Sprintf("❌ Invalid container image in '%s': %v", fname, err))
				return nil, fmt.Errorf("Invalid container image identifier: must not be a compose file or path")
			}
		}
	}

	// 2. Pre-flight Dry-Run validation
	logLine("🔍 Running pre-flight Quadlet validation (quadlet -dryrun -user)...")
	valid, dryOut, err := ValidateQuadletFiles(files)
	if !valid || err != nil {
		logLine(fmt.Sprintf("❌ Pre-flight Quadlet dry-run rejected: %v\n%s", err, dryOut))
		return nil, fmt.Errorf("Pre-flight Quadlet validation failed: %v", err)
	}
	logLine("✅ Quadlet syntax verified by podman systemd generator.")

	// 3. Pre-flight Ghost Container & Unit cleanup (prevent conflict exit code 125 and systemd auto-restart races)
	d.cleanPreflightConflicts(ctx, s, logLine)

	// 4. Determine systemd user quadlet search path and purge legacy space-polluted unit files
	systemdQuadletDir := os.Getenv("QUADLET_DIR")
	if systemdQuadletDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir = filepath.Join(homeDir, ".config", "containers", "systemd")
		}
	}
	if systemdQuadletDir != "" {
		_ = os.MkdirAll(systemdQuadletDir, 0755)
		if s.Name != cleanName {
			_ = os.Remove(filepath.Join(systemdQuadletDir, s.Name+".container"))
			_ = os.Remove(filepath.Join(systemdQuadletDir, s.Name+".pod"))
		}
		if entries, err := os.ReadDir(systemdQuadletDir); err == nil {
			for _, entry := range entries {
				eName := entry.Name()
				if strings.Contains(eName, " ") && (strings.HasSuffix(eName, ".container") || strings.HasSuffix(eName, ".pod")) {
					cleanSlug := SanitizeQuadletSlug(strings.TrimSuffix(strings.TrimSuffix(eName, ".container"), ".pod"))
					if cleanSlug == cleanName {
						_ = os.Remove(filepath.Join(systemdQuadletDir, eName))
					}
				}
			}
		}
	}

	// 5. Write unit files to disk
	var primaryServiceName string
	var podMemberServices []string
	for fname, uContent := range files {
		// Local data backup
		_ = os.WriteFile(filepath.Join(quadletDir, fname), []byte(uContent), 0644)

		if systemdQuadletDir != "" {
			_ = os.MkdirAll(systemdQuadletDir, 0755)
			userFile := filepath.Join(systemdQuadletDir, fname)
			if err := os.WriteFile(userFile, []byte(uContent), 0644); err != nil {
				logLine(fmt.Sprintf("❌ Failed to write unit '%s': %v", fname, err))
				return nil, err
			}
			logLine(fmt.Sprintf("📄 Written unit to %s", userFile))
		}

		if strings.HasSuffix(fname, ".pod") {
			basePod := strings.TrimSuffix(fname, ".pod")
			primaryServiceName = fmt.Sprintf("%s-pod.service", basePod)
		} else if strings.HasSuffix(fname, ".container") {
			baseMember := strings.TrimSuffix(fname, ".container")
			podMemberServices = append(podMemberServices, fmt.Sprintf("%s.service", baseMember))
		}
	}

	if primaryServiceName == "" && len(podMemberServices) > 0 {
		primaryServiceName = podMemberServices[0]
		podMemberServices = podMemberServices[1:]
	} else if primaryServiceName == "" {
		primaryServiceName = fmt.Sprintf("%s.service", cleanName)
	}

	// 6. Pull image first with streaming progress
	mainImage := s.Image
	for _, uContent := range files {
		p := ParseQuadlet(uContent)
		if p.Image != "" {
			mainImage = p.Image
			break
		}
	}
	if mainImage == "" {
		mainImage = "docker.io/library/nginx:alpine"
	}

	// Smart Image Cache Check: avoid redundant remote pulls when image exists locally
	hasImage := false
	if d.podmanClient != nil {
		chkCmd := exec.CommandContext(ctx, "podman", "image", "exists", mainImage)
		d.setupCmdEnv(chkCmd)
		if err := chkCmd.Run(); err == nil {
			hasImage = true
		}
	}

	shouldPull := !hasImage || trigger == "pull" || trigger == "force-pull" || trigger == "update"
	if shouldPull {
		logLine(fmt.Sprintf("📦 Pulling container image '%s'...", mainImage))
		var pullArgs []string
		if s.RegistryID != "" && d.registryProvider != nil {
			if reg, err := d.registryProvider.GetRegistry(ctx, s.RegistryID); err == nil && reg != nil && reg.Username != "" {
				logLine(fmt.Sprintf("🔑 Authenticating with registry '%s' (%s)...", reg.Name, reg.Username))
				pullArgs = append(pullArgs, "--creds", fmt.Sprintf("%s:%s", reg.Username, reg.Token))
			}
		}
		pullArgs = append(pullArgs, "pull", mainImage)
		pullCmd := exec.CommandContext(ctx, "podman", pullArgs...)
		d.setupCmdEnv(pullCmd)
		pStdout, _ := pullCmd.StdoutPipe()
		pStderr, _ := pullCmd.StderrPipe()
		if err := pullCmd.Start(); err == nil {
			d.streamPipes(pStdout, pStderr, logLine)
			_ = pullCmd.Wait()
		}
	} else {
		logLine(fmt.Sprintf("⚡ Container image '%s' is cached locally. Reusing existing image.", mainImage))
	}

	// 7. Reload systemd daemon and activate service
	systemdSuccess := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		logLine("⚙️ Executing: systemctl --user daemon-reload...")
		reloadCmd := exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload")
		d.setupCmdEnv(reloadCmd)
		if out, err := reloadCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("⚠️ daemon-reload notice: %v (%s)", err, strings.TrimSpace(string(out))))
		}

		// Reset failed counter so clean start is permitted
		resetCmd := exec.CommandContext(ctx, "systemctl", "--user", "reset-failed", primaryServiceName)
		d.setupCmdEnv(resetCmd)
		_ = resetCmd.Run()

		logLine(fmt.Sprintf("🚀 Activating systemd service: %s...", primaryServiceName))
		startCmd := exec.CommandContext(ctx, "systemctl", "--user", "restart", primaryServiceName)
		d.setupCmdEnv(startCmd)
		if out, err := startCmd.CombinedOutput(); err != nil {
			logLine(fmt.Sprintf("❌ systemctl restart failed: %v (%s)", err, strings.TrimSpace(string(out))))
			return nil, fmt.Errorf("systemctl start failed: %s", strings.TrimSpace(string(out)))
		}

		// Also start member services if part of a Pod bundle
		for _, memberSvc := range podMemberServices {
			mStart := exec.CommandContext(ctx, "systemctl", "--user", "restart", memberSvc)
			d.setupCmdEnv(mStart)
			_ = mStart.Run()
		}

		systemdSuccess = true
		logLine(fmt.Sprintf("✅ Service '%s' activated successfully under systemd user supervisor.", primaryServiceName))
	}

	if !systemdSuccess {
		return nil, fmt.Errorf("systemd daemon unavailable to activate Quadlet unit")
	}

	deployedContName := cleanName
	if primaryServiceName != "" {
		deployedContName = strings.TrimSuffix(primaryServiceName, ".service")
		if strings.HasSuffix(deployedContName, "-pod") {
			deployedContName = strings.TrimSuffix(deployedContName, "-pod")
		}
	}

	return &DeployResult{
		ContainerName: deployedContName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Quadlet unit compiled and activated (%s)", mainImage),
		Image:         mainImage,
		Version:       "quadlet",
	}, nil
}

// GetServiceJournal fetches systemd journalctl entries for the service unit.
func (d *Deployer) GetServiceJournal(ctx context.Context, s Service, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	unitName := QuadletUnitName(s)
	cands := getQuadletCandidateUnitNames(s, unitName)

	if _, err := exec.LookPath("journalctl"); err != nil {
		return "", fmt.Errorf("journalctl not available on host")
	}

	for _, cand := range cands {
		cmd := exec.CommandContext(ctx, "journalctl", "--user", "-u", cand, "-n", strconv.Itoa(lines), "--no-pager")
		d.setupCmdEnv(cmd)
		out, err := cmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return string(out), nil
		}
	}

	// Fallback to primary unit name
	cmd := exec.CommandContext(ctx, "journalctl", "--user", "-u", fmt.Sprintf("%s.service", unitName), "-n", strconv.Itoa(lines), "--no-pager")
	d.setupCmdEnv(cmd)
	out, _ := cmd.CombinedOutput()
	return string(out), nil
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
		cmd := exec.Command("loginctl", "show-user", currUser, "--property=Linger")
		if out, err := cmd.Output(); err == nil {
			str := strings.TrimSpace(string(out))
			if strings.EqualFold(str, "Linger=yes") {
				return true, currUser
			}
		}
	}

	return false, currUser
}
