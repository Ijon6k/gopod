package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"gopod/internal/podman"
)

// deployImage pulls container image and starts container.
func (d *Deployer) deployImage(ctx context.Context, s Service, depID string, logLine func(string)) (*DeployResult, error) {
	imageToRun := s.Image
	if imageToRun == "" {
		imageToRun = "docker.io/library/nginx:alpine"
	}

	logLine(fmt.Sprintf("📦 Pulling container image '%s'...", imageToRun))
	pullCmd := exec.CommandContext(ctx, "podman", "pull", imageToRun)
	d.setupCmdEnv(pullCmd)
	pStdout, _ := pullCmd.StdoutPipe()
	pStderr, _ := pullCmd.StderrPipe()
	if err := pullCmd.Start(); err == nil {
		d.streamPipes(pStdout, pStderr, logLine)
		_ = pullCmd.Wait()
	}

	containerName := s.ID
	if containerName == "" {
		containerName = GenerateContainerName(s.ProjectID, s.Name)
	}
	d.stopPreviousContainer(ctx, s, logLine)

	if err := d.runContainer(ctx, s, containerName, imageToRun, logLine); err != nil {
		return nil, err
	}

	tag := "latest"
	if parts := strings.Split(imageToRun, ":"); len(parts) > 1 {
		tag = parts[len(parts)-1]
	}

	return &DeployResult{
		ContainerName: containerName,
		CommitHash:    "",
		CommitMessage: fmt.Sprintf("Image deploy: %s", imageToRun),
		Image:         imageToRun,
		Version:       tag,
	}, nil
}

func (d *Deployer) runContainer(ctx context.Context, s Service, containerName, imageToRun string, logLine func(string)) error {
	ports := []string{}
	if s.HostPort > 0 && s.Port > 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", s.HostPort, s.Port))
	} else if s.HostPort > 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", s.HostPort, s.HostPort))
	}

	envMap := make(map[string]string)
	for _, env := range s.EnvVars {
		if env.Key != "" {
			envMap[env.Key] = env.Value
		}
	}

	labels := map[string]string{
		"io.gopod.project": s.ProjectID,
		"io.gopod.service": s.ID,
		"io.gopod.name":    s.Name,
	}

	networkToUse := "gopod-net"
	if d.podmanClient != nil {
		_ = d.podmanClient.EnsureNetwork(ctx, networkToUse)
	}

	var vols []string
	var capAdd, capDrop, devices, secOpts []string
	userNS := ""
	privileged := false
	pidsLimit := 0

	if s.Advanced != nil {
		userNS = s.Advanced.Runtime.UserNamespace
		privileged = s.Advanced.Security.Privileged
		capAdd = s.Advanced.Security.CapAdd
		capDrop = s.Advanced.Security.CapDrop
		devices = s.Advanced.Runtime.Devices
		pidsLimit = s.Advanced.Resources.PIDsLimit
		rawLabel := strings.TrimSpace(s.Advanced.Security.SELinuxLabel)
		if rawLabel != "" && rawLabel != "container_file_t" {
			if rawLabel == "disable" || rawLabel == "disabled" || rawLabel == "label=disable" {
				secOpts = append(secOpts, "label=disable")
			} else if strings.HasPrefix(rawLabel, "label=") {
				secOpts = append(secOpts, rawLabel)
			} else if strings.HasPrefix(rawLabel, "type:") || strings.HasPrefix(rawLabel, "level:") || strings.HasPrefix(rawLabel, "filetype:") {
				secOpts = append(secOpts, fmt.Sprintf("label=%s", rawLabel))
			} else {
				secOpts = append(secOpts, fmt.Sprintf("label=type:%s", rawLabel))
			}
		}
		if s.Advanced.Security.AppArmorProfile != "" {
			prof := strings.TrimSpace(s.Advanced.Security.AppArmorProfile)
			if prof == "unconfined" || prof == "disable" || prof == "disabled" {
				secOpts = append(secOpts, "apparmor=unconfined")
			} else {
				secOpts = append(secOpts, fmt.Sprintf("apparmor=%s", prof))
			}
		}
		if s.Advanced.Security.NoNewPrivileges {
			secOpts = append(secOpts, "no-new-privileges")
		}
		for _, v := range s.Advanced.Storage.Volumes {
			if strings.TrimSpace(v.Source) != "" && strings.TrimSpace(v.Target) != "" {
				if v.Options != "" {
					vols = append(vols, fmt.Sprintf("%s:%s:%s", v.Source, v.Target, v.Options))
				} else {
					vols = append(vols, fmt.Sprintf("%s:%s", v.Source, v.Target))
				}
			}
		}
	}

	logLine(fmt.Sprintf("🚀 Starting container '%s' (image: %s)...", containerName, imageToRun))
	if d.podmanClient != nil {
		_, err := d.podmanClient.RunContainer(ctx, podman.RunContainerOptions{
			Name:          containerName,
			Image:         imageToRun,
			Ports:         ports,
			Env:           envMap,
			Network:       networkToUse,
			Labels:        labels,
			RestartPolicy: s.RestartPolicy,
			CPULimit:      s.CPULimit,
			MemoryLimit:   s.MemoryLimit,
			Volumes:       vols,
			UserNS:        userNS,
			Privileged:    privileged,
			CapAdd:        capAdd,
			CapDrop:       capDrop,
			Devices:       devices,
			PidsLimit:     pidsLimit,
			SecurityOpt:   secOpts,
		})
		if err != nil {
			logLine(fmt.Sprintf("❌ Container run failed: %v", err))
			return err
		}
	}
	logLine(fmt.Sprintf("✅ Container '%s' is up and healthy.", containerName))
	return nil
}

func (d *Deployer) stopPreviousContainer(ctx context.Context, s Service, logLine func(string)) {
	if d.podmanClient == nil {
		return
	}
	existing, _ := d.podmanClient.GetContainers(ctx)
	for _, c := range existing {
		isSameService := false
		if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID) {
			isSameService = true
		} else if len(c.Names) > 0 && (strings.HasPrefix(c.Names[0], s.ID) || strings.HasPrefix(c.Names[0], fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) || c.Names[0] == s.Name) {
			isSameService = true
		}
		if isSameService {
			logLine(fmt.Sprintf("🔄 Stopping previous container '%s'...", c.ID))
			_ = d.podmanClient.StopContainer(ctx, c.ID)
			_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
		}
	}
}
