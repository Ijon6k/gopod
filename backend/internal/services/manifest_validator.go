package services

import (
	"context"
	"fmt"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestWorkload represents an extracted workload container from K8s or Compose manifest.
type ManifestWorkload struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Ports   []string `json:"ports"`
	Role    string   `json:"role"` // "main", "sidecar", "init", "service"
	Volumes []string `json:"volumes,omitempty"`
	Memory  string   `json:"memory,omitempty"`
	CPU     string   `json:"cpu,omitempty"`
	Command string   `json:"command,omitempty"`
}

// ManifestValidationResult is the structured response for manifest pre-flight inspection.
type ManifestValidationResult struct {
	Valid     bool               `json:"valid"`
	Type      string             `json:"type"` // "kubernetes" | "compose"
	Errors    []string           `json:"errors"`
	Warnings  []string           `json:"warnings"`
	Workloads []ManifestWorkload `json:"workloads"`
	Volumes   []string           `json:"volumes"`
}

// checkPortAvailable tests if host port is currently bindable.
func checkPortAvailable(hostIP string, port int, udp bool) (bool, string) {
	if port <= 0 || port > 65535 {
		return false, fmt.Sprintf("Port %d is outside valid range (1-65535)", port)
	}

	// Always flag known host infrastructure ports (even when running in containerized engine)
	switch port {
	case 8080:
		return false, "Host port 8080 is reserved by GOPOD Caddy Ingress (HTTP). Use 8081+ or route via domain instead."
	case 8443:
		return false, "Host port 8443 is reserved by GOPOD Caddy Ingress (HTTPS). Use another port."
	case 8085:
		return false, "Host port 8085 is in use by GOPOD Core API."
	case 2019:
		return false, "Host port 2019 is in use by Caddy Admin API."
	}

	ip := hostIP
	if ip == "" {
		ip = "0.0.0.0"
	}

	proto := "tcp"
	if udp {
		proto = "udp"
	}

	addr := fmt.Sprintf("%s:%d", ip, port)
	if udp {
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			return false, fmt.Sprintf("Port %d/udp is already in use (%v)", port, err)
		}
		_ = conn.Close()
		return true, ""
	}

	ln, err := net.Listen(proto, addr)
	if err != nil {
		return false, fmt.Sprintf("Host port %d is already in use by another process or container", port)
	}
	_ = ln.Close()
	return true, ""
}

// ValidateKubernetesPodManifest inspects a Kubernetes Pod YAML manifest.
func ValidateKubernetesPodManifest(ctx context.Context, raw string) ManifestValidationResult {
	res := ManifestValidationResult{
		Type:      "kubernetes",
		Valid:     false,
		Errors:    []string{},
		Warnings:  []string{},
		Workloads: []ManifestWorkload{},
		Volumes:   []string{},
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		res.Errors = append(res.Errors, "Manifest content is empty")
		return res
	}

	var pod struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
		Spec struct {
			Volumes []struct {
				Name string `yaml:"name"`
			} `yaml:"volumes"`
			InitContainers []struct {
				Name      string `yaml:"name"`
				Image     string `yaml:"image"`
				Command   any    `yaml:"command"`
				Resources struct {
					Limits struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"initContainers"`
			Containers []struct {
				Name    string `yaml:"name"`
				Image   string `yaml:"image"`
				Command any    `yaml:"command"`
				Ports   []struct {
					ContainerPort int    `yaml:"containerPort"`
					HostPort      int    `yaml:"hostPort"`
					Protocol      string `yaml:"protocol"`
					HostIP        string `yaml:"hostIP"`
				} `yaml:"ports"`
				VolumeMounts []struct {
					Name      string `yaml:"name"`
					MountPath string `yaml:"mountPath"`
				} `yaml:"volumeMounts"`
				Resources struct {
					Limits struct {
						Memory string `yaml:"memory"`
						CPU    string `yaml:"cpu"`
					} `yaml:"limits"`
				} `yaml:"resources"`
			} `yaml:"containers"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal([]byte(trimmed), &pod); err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("YAML syntax error: %v", err))
		return res
	}

	if pod.Kind != "" && pod.Kind != "Pod" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("Resource kind '%s' declared. Podman native play kube works best with 'Pod'", pod.Kind))
	}

	for _, v := range pod.Spec.Volumes {
		if v.Name != "" {
			res.Volumes = append(res.Volumes, v.Name)
		}
	}

	// Init containers
	for _, ic := range pod.Spec.InitContainers {
		if strings.TrimSpace(ic.Name) == "" {
			res.Errors = append(res.Errors, "Init container declared without a name")
			continue
		}
		if strings.TrimSpace(ic.Image) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("Init container '%s' has no container image", ic.Name))
		}
		res.Workloads = append(res.Workloads, ManifestWorkload{
			Name:   ic.Name,
			Image:  ic.Image,
			Role:   "init",
			Memory: ic.Resources.Limits.Memory,
			CPU:    ic.Resources.Limits.CPU,
		})
	}

	// Main & Sidecar containers
	if len(pod.Spec.Containers) == 0 {
		res.Errors = append(res.Errors, "No containers declared in spec.containers")
		return res
	}

	for idx, c := range pod.Spec.Containers {
		cName := strings.TrimSpace(c.Name)
		if cName == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("Container at index %d has no name", idx))
			cName = fmt.Sprintf("container-%d", idx)
		}
		if strings.TrimSpace(c.Image) == "" {
			res.Errors = append(res.Errors, fmt.Sprintf("Container '%s' has no image specified", cName))
		}

		role := "main"
		if idx > 0 {
			role = "sidecar"
		}

		var portStrs []string
		for _, p := range c.Ports {
			if p.ContainerPort <= 0 {
				res.Errors = append(res.Errors, fmt.Sprintf("Container '%s' has invalid containerPort (%d)", cName, p.ContainerPort))
				continue
			}

			proto := strings.ToLower(strings.TrimSpace(p.Protocol))
			isUDP := proto == "udp"

			var portDesc string
			if p.HostPort > 0 {
				portDesc = fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort)
				if isUDP {
					portDesc += "/udp"
				}
				// Pre-flight host port collision test
				if ok, reason := checkPortAvailable(p.HostIP, p.HostPort, isUDP); !ok {
					res.Warnings = append(res.Warnings, fmt.Sprintf("Container '%s': %s", cName, reason))
				}
			} else {
				portDesc = fmt.Sprintf("%d", p.ContainerPort)
				if isUDP {
					portDesc += "/udp"
				}
			}
			portStrs = append(portStrs, portDesc)
		}

		var vmStrs []string
		for _, vm := range c.VolumeMounts {
			vmStrs = append(vmStrs, fmt.Sprintf("%s -> %s", vm.Name, vm.MountPath))
		}

		res.Workloads = append(res.Workloads, ManifestWorkload{
			Name:    cName,
			Image:   c.Image,
			Role:    role,
			Ports:   portStrs,
			Volumes: vmStrs,
			Memory:  c.Resources.Limits.Memory,
			CPU:     c.Resources.Limits.CPU,
		})
	}

	res.Valid = len(res.Errors) == 0
	return res
}

// ValidateComposeManifest inspects a Docker Compose YAML manifest.
func ValidateComposeManifest(ctx context.Context, raw string) ManifestValidationResult {
	res := ManifestValidationResult{
		Type:      "compose",
		Valid:     false,
		Errors:    []string{},
		Warnings:  []string{},
		Workloads: []ManifestWorkload{},
		Volumes:   []string{},
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		res.Errors = append(res.Errors, "Compose YAML content is empty")
		return res
	}

	var compose struct {
		Version  string                            `yaml:"version"`
		Services map[string]map[string]interface{} `yaml:"services"`
		Volumes  map[string]interface{}            `yaml:"volumes"`
		Networks map[string]interface{}            `yaml:"networks"`
	}

	if err := yaml.Unmarshal([]byte(trimmed), &compose); err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("YAML syntax error: %v", err))
		return res
	}

	if len(compose.Services) == 0 {
		res.Errors = append(res.Errors, "No services defined under 'services:' section")
		return res
	}

	for vName := range compose.Volumes {
		res.Volumes = append(res.Volumes, vName)
	}

	for sName, sDef := range compose.Services {
		var img string
		if imgRaw, ok := sDef["image"]; ok {
			img = fmt.Sprintf("%v", imgRaw)
		}
		if img == "" {
			if _, hasBuild := sDef["build"]; !hasBuild {
				res.Warnings = append(res.Warnings, fmt.Sprintf("Service '%s' has neither 'image' nor 'build' defined", sName))
			}
		}

		var portStrs []string
		if portsRaw, ok := sDef["ports"]; ok {
			if pList, ok := portsRaw.([]interface{}); ok {
				for _, pItem := range pList {
					pStr := fmt.Sprintf("%v", pItem)
					portStrs = append(portStrs, pStr)

					ip, hp, _, udp, ok := parsePortBinding(pStr)
					if ok && hp > 0 {
						if avail, reason := checkPortAvailable(ip, hp, udp); !avail {
							res.Warnings = append(res.Warnings, fmt.Sprintf("Service '%s': %s", sName, reason))
						}
					}
				}
			}
		}

		res.Workloads = append(res.Workloads, ManifestWorkload{
			Name:  sName,
			Image: img,
			Role:  "service",
			Ports: portStrs,
		})
	}

	res.Valid = len(res.Errors) == 0
	return res
}
