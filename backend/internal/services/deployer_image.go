package services

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gopod/internal/podman"
)

// Redeploy ordering strategies. start-first mirrors dokploy's deploy
// semantics: the replacement container starts while the previous one keeps
// serving, and the previous container is only removed once the replacement is
// confirmed up — a failed run therefore never takes the healthy service down.
// When that is impossible (pod membership, or the replacement needs a host
// port the predecessor still holds) we keep the classic stop-then-create
// ordering.
const (
	redeployStartFirst     = "start-first"
	redeployStopThenCreate = "stop-then-create"
)

type containerRedeployPlan struct {
	mode          string
	predecessorID string // live previous container (short id) when start-first
	runName       string // name under which the new container runs
}

// planContainerRedeploy inspects the runtime and selects the redeploy
// strategy plus the run name for a plain container service (image/git deploys).
// Fresh deploys (no live predecessor) keep the canonical name and the classic
// preflight; unknown or unlabeled runtimes always fall back to the previous
// behavior, which keeps old data safe.
func (d *Deployer) planContainerRedeploy(ctx context.Context, s Service, desiredName string, logLine func(string)) containerRedeployPlan {
	plan := containerRedeployPlan{mode: redeployStopThenCreate, runName: desiredName}
	if d.podmanClient == nil || desiredName == "" {
		return plan
	}
	if s.InPod || s.RuntimeTarget == "pod" {
		// Pod members are replaced within their pod; start-first cannot
		// express two slots for the same member.
		return plan
	}

	containers, err := d.podmanClient.GetContainers(ctx)
	if err != nil {
		return plan
	}
	return selectRedeployPlan(s, containers, desiredName, logLine)
}

// selectRedeployPlan maps the live container list to a redeploy strategy.
// Purity note: no I/O here — the same mapping is unit-testable offline.
func selectRedeployPlan(s Service, containers []podman.ContainerItem, desiredName string, logLine func(string)) containerRedeployPlan {
	plan := containerRedeployPlan{mode: redeployStopThenCreate, runName: desiredName}

	var predecessor *podman.ContainerItem
	for i := range containers {
		c := containers[i]
		if c.Labels == nil || c.Labels["io.gopod.service"] != s.ID {
			continue
		}
		state := strings.ToLower(c.State)
		if state != "running" && state != "paused" {
			continue // only a LIVE container is a start-first predecessor
		}
		predecessor = &containers[i]
		break
	}
	if predecessor == nil {
		return plan
	}

	if s.HostPort > 0 && containerBindsHostPort(*predecessor, s.HostPort) {
		if logLine != nil {
			logLine(fmt.Sprintf("🔁 Redeploy strategy: stop-then-create (host port %d is currently bound by the previous container).", s.HostPort))
		}
		return plan
	}

	if logLine != nil {
		logLine("🟦 Redeploy strategy: start-first — replacement starts while the previous container keeps serving; it is removed only after the replacement is confirmed up.")
	}
	plan.mode = redeployStartFirst
	plan.predecessorID = predecessor.ID
	plan.runName = GenerateContainerName(s.ProjectID, s.Name)
	return plan
}

// containerBindsHostPort reports whether a listed container currently holds
// the given host-side port binding (Ports format: "hostPort:containerPort/proto").
func containerBindsHostPort(c podman.ContainerItem, hostPort int) bool {
	for _, entry := range strings.Split(c.Ports, ", ") {
		hostPart, _, ok := strings.Cut(entry, ":")
		if ok && hostPart == strconv.Itoa(hostPort) {
			return true
		}
	}
	return false
}

// cleanPreflightForStartFirst is the start-first counterpart of
// cleanPreflightConflicts: candidate systemd units are still stopped (they can
// restart under systemd supervision) and ghost containers from aborted
// previous deploys are removed, but the live predecessor container survives
// until the replacement is confirmed up.
func (d *Deployer) cleanPreflightForStartFirst(ctx context.Context, s Service, predecessorID string, logLine func(string)) {
	candUnits, _, _ := d.findServiceUnitFilesAndCandidates(s)
	if d.hasUserManager() {
		for _, u := range candUnits {
			_, _ = d.systemctlUser(ctx, "stop", u)
			_, _ = d.systemctlUser(ctx, "kill", "--kill-who=all", "--signal=SIGKILL", u)
			_, _ = d.systemctlUser(ctx, "reset-failed", u)
		}
	}
	if d.podmanClient != nil {
		if containers, err := d.podmanClient.GetContainers(ctx); err == nil {
			for _, c := range containers {
				if c.Labels == nil || c.Labels["io.gopod.service"] != s.ID || c.ID == predecessorID {
					continue
				}
				// Ghost from an interrupted previous deploy — safe to remove now.
				_ = d.podmanClient.StopContainer(ctx, c.ID)
				_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
			}
		}
	}
	if logLine != nil {
		logLine("🧹 Start-first preflight: reconciled unit ghosts; live predecessor kept serving.")
	}
}

// retirePredecessor stops and removes the previous container now that the
// replacement is confirmed up, then reclaims the canonical container name so
// name-based consumers (default ingress target, exec/terminal lookups, shell
// scripts) keep resolving as before. Returns the final container name.
func (d *Deployer) retirePredecessor(ctx context.Context, predecessorID, canonicalName, replacementName string, logLine func(string)) string {
	finalName := replacementName
	logLine(fmt.Sprintf("🔄 Start-first: replacement is up — retiring previous container '%s'...", predecessorID))
	_ = d.podmanClient.StopContainer(ctx, predecessorID)
	_ = d.podmanClient.DeleteContainer(ctx, predecessorID, true)
	if canonicalName == "" || replacementName == "" || replacementName == canonicalName {
		return finalName
	}
	cmdRename := exec.CommandContext(ctx, "podman", "rename", replacementName, canonicalName)
	d.setupCmdEnv(cmdRename)
	if _, err := cmdRename.CombinedOutput(); err != nil {
		logLine(fmt.Sprintf("⚠️ Could not reclaim canonical container name '%s' (replacement keeps '%s'): %v", canonicalName, replacementName, err))
	} else {
		finalName = canonicalName
		logLine(fmt.Sprintf("✏️ Reclaimed canonical container name '%s' for the replacement container.", canonicalName))
	}
	return finalName
}

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

	// Pick the redeploy ordering (start-first when safe, stop-then-create
	// otherwise) before reconciling preflight state.
	plan := d.planContainerRedeploy(ctx, s, containerName, logLine)
	if plan.mode == redeployStartFirst {
		d.cleanPreflightForStartFirst(ctx, s, plan.predecessorID, logLine)
	} else {
		d.cleanPreflightConflicts(ctx, s, logLine)
	}

	if err := d.runContainer(ctx, s, plan.runName, imageToRun, logLine); err != nil {
		return nil, err
	}

	if plan.mode == redeployStartFirst {
		containerName = d.retirePredecessor(ctx, plan.predecessorID, containerName, plan.runName, logLine)
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
		var createdPodHere bool
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
				// The pod was freshly created by us only when CreatePod returned
				// nil. ("already exists" means someone else owns it — never roll
				// back a pod we did not create.)
				createdPodHere = err == nil
				emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.autocreate",
					fmt.Sprintf("Pod '%s' auto-created with ports %v for service '%s'", targetPod, podPorts, s.Name), map[string]any{"pod": targetPod, "ports": podPorts, "service": s.Name})
			} else {
				logLine(fmt.Sprintf("🔗 Target pod '%s' exists; joining it.", targetPod))
			}
		}

		// Attach ownership labels to explicitly named volumes before the
		// container claims them. Podman would otherwise create the volume
		// implicitly and leave it unlabeled, which makes ownership impossible to
		// prove at teardown. Best-effort: a labeling failure never blocks deploy.
		for _, spec := range vols {
			src := volumeSourceOf(spec)
			if !isNamedVolumeSource(src) {
				continue
			}
			if err := d.podmanClient.CreateVolume(ctx, src, map[string]string{
				"io.gopod.service": s.ID,
				"io.gopod.project": s.ProjectID,
			}); err != nil {
				logLine(fmt.Sprintf("⚠️ Could not label named volume '%s': %v", src, err))
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
			// A pod that was auto-created just for this failed run would keep
			// holding its host ports forever. Roll it back so the next deploy
			// is not blocked with 'address already in use'.
			if createdPodHere {
				if delErr := d.podmanClient.DeletePod(ctx, targetPod, true); delErr != nil {
					logLine(fmt.Sprintf("⚠️ Failed to roll back auto-created pod '%s': %v", targetPod, delErr))
				} else {
					logLine(fmt.Sprintf("🧹 Rolled back auto-created pod '%s' after failed container start (port release).", targetPod))
					emitEngineEventSafe(ctx, "info", "service", s.ID, s.ProjectID, "engine", "pod.rollback",
						fmt.Sprintf("Rolled back auto-created pod '%s' after failed container start for '%s'", targetPod, s.Name),
						map[string]any{"pod": targetPod, "service": s.Name, "container": containerName})
				}
			}
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

// volumeSourceOf extracts the source segment of a "<source>:<target>[:opts]"
// volume specification.
func volumeSourceOf(spec string) string {
	parts := strings.SplitN(spec, ":", 2)
	return strings.TrimSpace(parts[0])
}

// isNamedVolumeSource reports whether a volume source names a managed volume
// rather than a host path. Host binds (absolute, relative, ~-prefixed, or any
// source containing a path separator) are left untouched — they are not
// volumes gopod created.
func isNamedVolumeSource(src string) bool {
	if src == "" {
		return false
	}
	if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "~") {
		return false
	}
	return !strings.Contains(src, "/")
}
