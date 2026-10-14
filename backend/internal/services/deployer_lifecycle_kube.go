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

func (d *Deployer) isKubeWorkload(s Service) bool {
	return s.Type == "kubernetes" || s.Type == "k8s" || (s.Type != "compose" && s.Type != "quadlet" && s.Type != "pod" && s.Type != "image" && strings.TrimSpace(s.K8sYaml) != "")
}

func (d *Deployer) startKubeService(ctx context.Context, s Service) error {
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

func (d *Deployer) stopKubeService(ctx context.Context, s Service) error {
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

func (d *Deployer) teardownKubeService(ctx context.Context, s Service, deleteVolumes bool) {
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
