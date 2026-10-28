package services

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

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

		switch env := svc.Environment.(type) {
		case []interface{}:
			for _, item := range env {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					result.Environment = append(result.Environment, strings.TrimSpace(s))
				}
			}
		case map[string]interface{}:
			for k, v := range env {
				result.Environment = append(result.Environment, fmt.Sprintf("%s=%v", k, v))
			}
		}

		switch cmd := svc.Command.(type) {
		case string:
			result.Exec = strings.TrimSpace(cmd)
		case []interface{}:
			var parts []string
			for _, part := range cmd {
				if s, ok := part.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				result.Exec = strings.Join(parts, " ")
			}
		}

		if result.Image != "" {
			break
		}
	}

	return result
}

// CountComposeServices returns the number of services declared in a Compose YAML.
func CountComposeServices(composeYaml string) int {
	return len(ExtractAllComposeServices(composeYaml))
}

// ExtractAllComposeServices parses every service in Compose YAML.
func ExtractAllComposeServices(composeYaml string) []ExtractedCompose {
	var results []ExtractedCompose
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
		var item ExtractedCompose
		item.Image = strings.TrimSpace(svc.Image)
		if svc.ContainerName != "" {
			item.ContainerName = SanitizeQuadletSlug(svc.ContainerName)
		} else {
			item.ContainerName = SanitizeQuadletSlug(name)
		}
		for _, p := range svc.Ports {
			if strings.TrimSpace(p) != "" {
				item.Ports = append(item.Ports, strings.TrimSpace(p))
			}
		}
		for _, v := range svc.Volumes {
			if strings.TrimSpace(v) != "" {
				item.Volumes = append(item.Volumes, strings.TrimSpace(v))
			}
		}
		if svc.Restart != "" {
			item.Restart = MapDockerRestartToSystemd(svc.Restart)
		}

		switch env := svc.Environment.(type) {
		case []interface{}:
			for _, el := range env {
				if s, ok := el.(string); ok && strings.TrimSpace(s) != "" {
					item.Environment = append(item.Environment, strings.TrimSpace(s))
				}
			}
		case map[string]interface{}:
			for k, v := range env {
				item.Environment = append(item.Environment, fmt.Sprintf("%s=%v", k, v))
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
