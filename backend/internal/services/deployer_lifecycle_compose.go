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

func (d *Deployer) isComposeWorkload(s Service) bool {
	return s.Type == "compose" || (s.Type != "quadlet" && strings.TrimSpace(s.ComposeYaml) != "")
}

func (d *Deployer) startComposeService(ctx context.Context, s Service) error {
	projectName := s.ID
	if projectName == "" {
		projectName = s.Name
	}

	composeFile := filepath.Join(d.dataDir, "compose", s.ProjectID, projectName, "docker-compose.yml")
	if abs, err := filepath.Abs(composeFile); err == nil {
		composeFile = abs
	}
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
	cmd.Dir = filepath.Dir(composeFile)
	d.setupCmdEnv(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := string(out)
		if !strings.Contains(outStr, "pod already exists") && !strings.Contains(outStr, "already in use") {
			return fmt.Errorf("compose start error: %s (%w)", strings.TrimSpace(outStr), err)
		}
	}
	return nil
}

func (d *Deployer) stopComposeService(ctx context.Context, s Service) error {
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
		cmd.Dir = filepath.Dir(composeFile)
		d.setupCmdEnv(cmd)
		_ = cmd.Run()
	}

	if d.podmanClient != nil {
		containers, _ := d.podmanClient.GetContainers(ctx)
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil {
				if c.Labels["io.gopod.service"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.ID ||
					c.Labels["io.podman.compose.project"] == s.ID ||
					c.Labels["com.docker.compose.project"] == projectName ||
					c.Labels["io.podman.compose.project"] == projectName ||
					c.Labels["com.docker.compose.project"] == s.Name ||
					c.Labels["io.podman.compose.project"] == s.Name {
					isMatch = true
				}
			}
			if !isMatch && len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, projectName+"_") ||
					strings.HasPrefix(firstName, s.ID+"_") ||
					strings.HasPrefix(firstName, s.Name+"_") ||
					strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) {
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

func (d *Deployer) teardownComposeService(ctx context.Context, s Service, deleteVolumes bool) {
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

	if d.podmanClient != nil {
		_ = d.podmanClient.DeletePod(ctx, "pod_"+projectName, true)
	}
	_ = os.RemoveAll(composeDir)
}
