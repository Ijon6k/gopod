package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StartService powers on workload containers or units according to service workload type.
func (d *Deployer) StartService(ctx context.Context, s Service) error {
	// 0. Kubernetes Manifest
	if s.Type == "kubernetes" || s.Type == "k8s" || strings.TrimSpace(s.K8sYaml) != "" {
		manifestName := s.ID
		if manifestName == "" {
			manifestName = s.Name
		}
		kubeFile := filepath.Join(d.dataDir, "kube", s.ProjectID, manifestName, "manifest.yaml")
		if _, err := os.Stat(kubeFile); err == nil {
			cmd := exec.CommandContext(ctx, "podman", "play", "kube", "--network", "gopod-net", "--replace", kubeFile)
			d.setupCmdEnv(cmd)
			_ = cmd.Run()
			return nil
		}
		_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
		return err
	}

	// 1. Compose Stack
	if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = s.Name
		}

		composeFile := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName, "docker-compose.yml")
		if _, err := os.Stat(composeFile); err != nil {
			legacyFile := filepath.Join(d.dataDir, "compose", s.ProjectID, s.Name, "docker-compose.yml")
			if _, err2 := os.Stat(legacyFile); err2 == nil {
				composeFile = legacyFile
			} else {
				_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
				return err
			}
		}

		runner := "podman-compose"
		usePod := s.InPod || s.RuntimeTarget == "pod"
		podVal := "false"
		if usePod {
			podVal = "true"
		}
		args := []string{"--in-pod", podVal, "-p", projectName, "-f", composeFile, "up", "-d"}
		if _, err := exec.LookPath("podman-compose"); err != nil {
			runner = "podman"
			args = []string{"compose", "-p", projectName, "-f", composeFile, "up", "-d"}
		}

		cmd := exec.CommandContext(ctx, runner, args...)
		d.setupCmdEnv(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			outStr := string(out)
			if !strings.Contains(outStr, "pod already exists") && !strings.Contains(outStr, "already in use") {
				return fmt.Errorf("compose start error: %s (%w)", strings.TrimSpace(outStr), err)
			}
		}
		return nil
	}

	// 2. Quadlet Service
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "start", fmt.Sprintf("%s.service", s.Name))
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl start error: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			return nil
		}
	}

	// 3. Container / Git / Direct Image
	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		found := false
		var startErr error
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
				isMatch = true
			} else if len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) || firstName == s.Name || strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) {
					isMatch = true
				}
			}
			if isMatch {
				found = true
				if err := d.podmanClient.StartContainer(ctx, c.ID); err != nil {
					startErr = err
				}
			}
		}
		if found {
			return startErr
		}
	}

	// Fallback to fresh deployment if no container exists
	_, err := d.DeployService(ctx, s, fmt.Sprintf("start-%d", time.Now().Unix()))
	return err
}

// StopService gracefully halts containers or units for the service workload.
func (d *Deployer) StopService(ctx context.Context, s Service) error {
	// 0. Kubernetes Manifest
	if s.Type == "kubernetes" || s.Type == "k8s" || strings.TrimSpace(s.K8sYaml) != "" {
		manifestName := s.ID
		if manifestName == "" {
			manifestName = s.Name
		}
		kubeFile := filepath.Join(d.dataDir, "kube", s.ProjectID, manifestName, "manifest.yaml")
		if _, err := os.Stat(kubeFile); err == nil {
			cmd := exec.CommandContext(ctx, "podman", "play", "kube", "--down", kubeFile)
			d.setupCmdEnv(cmd)
			_ = cmd.Run()
		}
		return nil
	}

	// 1. Compose Stack
	if s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = s.Name
		}

		composeFile := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName, "docker-compose.yml")
		if _, err := os.Stat(composeFile); err != nil {
			legacyFile := filepath.Join(d.dataDir, "compose", s.ProjectID, s.Name, "docker-compose.yml")
			if _, err2 := os.Stat(legacyFile); err2 == nil {
				composeFile = legacyFile
			}
		}

		if _, err := os.Stat(composeFile); err == nil {
			runner := "podman-compose"
			args := []string{"-p", projectName, "-f", composeFile, "stop"}
			if _, err := exec.LookPath("podman-compose"); err != nil {
				runner = "podman"
				args = []string{"compose", "-p", projectName, "-f", composeFile, "stop"}
			}
			cmd := exec.CommandContext(ctx, runner, args...)
			d.setupCmdEnv(cmd)
			_ = cmd.Run()
		}

		if d.podmanClient != nil {
			containers, _ := d.podmanClient.GetContainers(ctx)
			for _, c := range containers {
				if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID || c.Labels["io.podman.compose.project"] == s.ID) {
					_ = d.podmanClient.StopContainer(ctx, c.ID)
				}
			}
		}
		return nil
	}

	// 2. Quadlet Service
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "stop", fmt.Sprintf("%s.service", s.Name))
			_ = cmd.Run()
			return nil
		}
	}

	// 3. Container / Git / Direct Image
	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
				isMatch = true
			} else if len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) || firstName == s.Name || strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) {
					isMatch = true
				}
			}
			if isMatch {
				_ = d.podmanClient.StopContainer(ctx, c.ID)
			}
		}
	}

	return nil
}

// RestartService restarts running containers or units for the service workload.
func (d *Deployer) RestartService(ctx context.Context, s Service) error {
	_ = d.StopService(ctx, s)
	return d.StartService(ctx, s)
}

// TeardownService stops and completely removes containers, pods, quadlets, compose stacks, and kube workloads.
func (d *Deployer) TeardownService(ctx context.Context, s Service, deleteVolumes bool) error {
	// 0. Kubernetes Manifest
	if s.Type == "kubernetes" || s.Type == "k8s" || strings.TrimSpace(s.K8sYaml) != "" {
		manifestName := s.ID
		if manifestName == "" {
			manifestName = s.Name
		}
		kubeDir := filepath.Join(d.dataDir, "kube", s.ProjectID, manifestName)
		kubeFile := filepath.Join(kubeDir, "manifest.yaml")
		if _, err := os.Stat(kubeFile); err == nil {
			downArgs := []string{"play", "kube", "--down"}
			if deleteVolumes {
				downArgs = append(downArgs, "--force")
			}
			downArgs = append(downArgs, kubeFile)
			cmd := exec.CommandContext(ctx, "podman", downArgs...)
			d.setupCmdEnv(cmd)
			_ = cmd.Run()
		}
		_ = os.RemoveAll(kubeDir)
	}

	// 1. Compose Workload
	if s.Type == "compose" || s.ComposeYaml != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = s.Name
		}
		composeDir := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName)
		composeFile := filepath.Join(composeDir, "docker-compose.yml")

		if _, err := os.Stat(composeFile); err == nil {
			var downArgs []string
			if _, err := exec.LookPath("podman-compose"); err == nil {
				downArgs = []string{"-p", projectName, "-f", composeFile, "down"}
				if deleteVolumes {
					downArgs = append(downArgs, "-v")
				}
				cmd := exec.CommandContext(ctx, "podman-compose", downArgs...)
				d.setupCmdEnv(cmd)
				_ = cmd.Run()
			} else if _, err := exec.LookPath("podman"); err == nil {
				downArgs = []string{"compose", "-p", projectName, "-f", composeFile, "down"}
				if deleteVolumes {
					downArgs = append(downArgs, "-v")
				}
				cmd := exec.CommandContext(ctx, "podman", downArgs...)
				d.setupCmdEnv(cmd)
				_ = cmd.Run()
			}
		}

		// Also remove pod if created by podman-compose
		if d.podmanClient != nil {
			_ = d.podmanClient.DeletePod(ctx, "pod_"+projectName, true)
		}

		// Clean up disk workspace
		_ = os.RemoveAll(composeDir)
	}

	// 2. Quadlet Service
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		quadletDir := filepath.Join(d.dataDir, "quadlets")
		_ = os.Remove(filepath.Join(quadletDir, fmt.Sprintf("%s.container", s.Name)))

		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			systemdQuadletDir := filepath.Join(homeDir, ".config", "containers", "systemd")
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.container", s.Name)))
			_ = os.Remove(filepath.Join(systemdQuadletDir, fmt.Sprintf("%s.pod", s.Name)))
		}

		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.CommandContext(ctx, "systemctl", "--user", "stop", fmt.Sprintf("%s.service", s.Name)).Run()
			_ = exec.CommandContext(ctx, "systemctl", "--user", "disable", fmt.Sprintf("%s.service", s.Name)).Run()
			_ = exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
		}
	}

	// 3. Container / Pod / Direct Image Workload cleanup
	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		cleanPrefix := fmt.Sprintf("%s-%s", s.ProjectID, s.Name)
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
				isMatch = true
			} else if len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) || firstName == s.Name || strings.HasPrefix(firstName, cleanPrefix) {
					isMatch = true
				}
			}
			if isMatch {
				_ = d.podmanClient.StopContainer(ctx, c.ID)
				_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
			}
		}

		// Deterministic pod deletion
		_ = d.podmanClient.DeletePod(ctx, s.ID, true)
		_ = d.podmanClient.DeletePod(ctx, s.Name, true)
		_ = d.podmanClient.DeletePod(ctx, "pod_"+s.ID, true)
		_ = d.podmanClient.DeletePod(ctx, "pod_"+s.Name, true)
		_ = d.podmanClient.DeletePod(ctx, cleanPrefix, true)

		// 4. Volume Cleanup if requested (deterministic prefix matching)
		if deleteVolumes {
			vols, _ := d.podmanClient.GetVolumes(ctx)
			for _, v := range vols {
				if strings.HasPrefix(v.Name, s.ID) || strings.HasPrefix(v.Name, cleanPrefix) || strings.HasPrefix(v.Name, s.Name) {
					_ = d.podmanClient.DeleteVolume(ctx, v.Name, true)
				}
			}
		}
	}

	return nil
}
