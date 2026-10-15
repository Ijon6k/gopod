package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"gopod/internal/podman"
)

// deployImage pulls container image and starts container.
func (d *Deployer) deployImage(ctx context.Context, s Service, depID, trigger string, logLine func(string)) (*DeployResult, error) {
	imageToRun := s.Image
	if imageToRun == "" {
		imageToRun = "docker.io/library/nginx:alpine"
	}

	// Smart Image Cache Check: avoid redundant remote pulls when image exists locally
	hasImage := false
	if d.podmanClient != nil {
		chkCmd := exec.CommandContext(ctx, "podman", "image", "exists", imageToRun)
		d.setupCmdEnv(chkCmd)
		if err := chkCmd.Run(); err == nil {
			hasImage = true
		}
	}

	shouldPull := !hasImage || trigger == "pull" || trigger == "force-pull" || trigger == "update"
	if shouldPull {
		logLine(fmt.Sprintf("📦 Pulling container image '%s'...", imageToRun))
		var pullArgs []string
		if s.RegistryID != "" && d.registryProvider != nil {
			if reg, err := d.registryProvider.GetRegistry(ctx, s.RegistryID); err == nil && reg != nil && reg.Username != "" {
				logLine(fmt.Sprintf("🔑 Authenticating with registry '%s' (%s)...", reg.Name, reg.Username))
				pullArgs = append(pullArgs, "--creds", fmt.Sprintf("%s:%s", reg.Username, reg.Token))
			}
		}
		pullArgs = append(pullArgs, "pull", imageToRun)
		pullCmd := exec.CommandContext(ctx, "podman", pullArgs...)
		d.setupCmdEnv(pullCmd)
		pStdout, _ := pullCmd.StdoutPipe()
		pStderr, _ := pullCmd.StderrPipe()
		if err := pullCmd.Start(); err == nil {
			d.streamPipes(pStdout, pStderr, logLine)
			_ = pullCmd.Wait()
		}
	} else {
		logLine(fmt.Sprintf("⚡ Container image '%s' is cached locally. Reusing existing image.", imageToRun))
	}

	containerName := s.ID
	if containerName == "" {
		containerName = GenerateContainerName(s.ProjectID, s.Name)
	}
	d.cleanPreflightConflicts(ctx, s, logLine)

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
		ports = append(ports, servicePortBinding(s.HostIP, s.HostPort, s.Port))
	} else if s.HostPort > 0 {
		ports = append(ports, servicePortBinding(s.HostIP, s.HostPort, s.HostPort))
	}

	envMap := make(map[string]string)
	for _, env := range s.EnvVars {
		if env.Key != "" && !strings.HasPrefix(env.Key, "#") {
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
		if s.Advanced.Runtime.UserNamespace != "" && s.Advanced.Runtime.UserNamespace != "default" && s.Advanced.Runtime.UserNamespace != "none" {
			userNS = s.Advanced.Runtime.UserNamespace
		}
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
		var targetPod string
		if s.InPod || s.RuntimeTarget == "pod" {
			rawPod := strings.TrimSpace(s.PodID)
			if rawPod == "" {
				rawPod = s.Name
			}
			// Member PodIDs are already canonical runtime names (pod_...);
			// only slugify ad-hoc values.
			if strings.HasPrefix(strings.ToLower(rawPod), "pod_") {
				targetPod = "pod_" + strings.ToLower(SanitizeQuadletSlug(strings.TrimPrefix(rawPod, "pod_")))
			} else {
				targetPod = PodNameFor(rawPod)
			}
			logLine(fmt.Sprintf("🔗 Resolved target pod '%s' for service '%s' (inPod=%v runtimeTarget=%s rawPodID=%q).", targetPod, s.Name, s.InPod, s.RuntimeTarget, s.PodID))
		}

		if targetPod != "" {
			if _, err := d.podmanClient.InspectPod(ctx, targetPod); err != nil {
				logLine(fmt.Sprintf("📦 Auto-creating pod '%s' on network gopod-net...", targetPod))
				var podPorts []string
				if s.HostPort > 0 && s.Port > 0 {
					podPorts = append(podPorts, servicePortBinding(s.HostIP, s.HostPort, s.Port))
				} else if s.HostPort > 0 {
					podPorts = append(podPorts, servicePortBinding(s.HostIP, s.HostPort, s.HostPort))
				}
				logLine(fmt.Sprintf("📦 Pod auto-create: name='%s' network=gopod-net ports=%v (service '%s').", targetPod, podPorts, s.Name))
				_, err := d.podmanClient.CreatePod(ctx, podman.CreatePodOptions{
					Name:    targetPod,
					Ports:   podPorts,
					Network: "gopod-net",
					Labels: map[string]string{
						"io.gopod.project": s.ProjectID,
					},
				})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					logLine(fmt.Sprintf("❌ Failed to create pod '%s': %v", targetPod, err))
					emitEngineEventSafe(ctx, "error", "service", s.ID, s.ProjectID, "engine", "pod.autocreate_failed",
						fmt.Sprintf("Pod auto-create '%s' failed: %v", targetPod, err), map[string]any{"pod": targetPod, "service": s.Name})
					return fmt.Errorf("failed to create pod '%s': %w", targetPod, err)
				}
				emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.autocreate",
					fmt.Sprintf("Pod '%s' auto-created with ports %v for service '%s'", targetPod, podPorts, s.Name), map[string]any{"pod": targetPod, "ports": podPorts, "service": s.Name})
			} else {
				logLine(fmt.Sprintf("🔗 Target pod '%s' exists; joining it.", targetPod))
			}
		}

		var cmdArgs []string
		if strings.TrimSpace(s.Command) != "" {
			cmdArgs = strings.Fields(s.Command)
		}

		runOpts := podman.RunContainerOptions{
			Name:          containerName,
			Image:         imageToRun,
			Command:       cmdArgs,
			Pod:           targetPod,
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
		}
		logLine(fmt.Sprintf("🧾 Equivalent: %s", sanitizedPodmanRunArgs(runOpts)))
		_, err := d.podmanClient.RunContainer(ctx, runOpts)
		if err != nil {
			logLine(fmt.Sprintf("❌ Container run failed: %v", err))
			d.enrichRunFailure(ctx, containerName, err, logLine)
			emitEngineEventSafe(ctx, "error", "service", s.ID, s.ProjectID, "engine", "container.run_failed",
				fmt.Sprintf("Container '%s' run failed: %v", containerName, err), map[string]any{"container": containerName, "image": imageToRun, "pod": targetPod})
			return err
		}
		emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "container.run",
			fmt.Sprintf("Container '%s' started (image %s, pod %q)", containerName, imageToRun, targetPod), map[string]any{"container": containerName, "image": imageToRun, "pod": targetPod})
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
