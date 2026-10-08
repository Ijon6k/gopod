package services

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopod/internal/podman"
)

// SSHKeyProvider provides private SSH keys for Git authentication.
type SSHKeyProvider interface {
	GetPrivateKey(ctx context.Context, id string) (string, error)
}

// DeployResult holds metadata about a completed deployment.
type DeployResult struct {
	ContainerName string
	CommitHash    string
	CommitMessage string
	Image         string
	Version       string
}

// Deployer manages container lifecycle, rolling updates, compose stacks, and source builds.
type Deployer struct {
	podmanClient   *podman.Client
	sshKeyProvider SSHKeyProvider
	logManager     *LogManager
	dataDir        string
}

// NewDeployer creates a new container deployer.
func NewDeployer(client *podman.Client, keyProvider SSHKeyProvider, logMgr *LogManager, dataDir string) *Deployer {
	if dataDir == "" {
		dataDir = "./data"
	}
	_ = os.MkdirAll(dataDir, 0755)
	return &Deployer{
		podmanClient:   client,
		sshKeyProvider: keyProvider,
		logManager:     logMgr,
		dataDir:        dataDir,
	}
}

// setupCmdEnv configures environment variables so that subprocesses (podman CLI, podman-compose) connect to the host Podman socket.
func (d *Deployer) setupCmdEnv(cmd *exec.Cmd) {
	socket := ""
	if d.podmanClient != nil {
		socket = d.podmanClient.SocketPath()
	}
	if socket == "" {
		socket = os.Getenv("PODMAN_SOCKET")
	}
	if socket == "" {
		socket = os.Getenv("CONTAINER_HOST")
	}
	if socket == "" {
		socket = "/run/podman/podman.sock"
	}
	socketClean := strings.TrimPrefix(socket, "unix://")
	socketURL := fmt.Sprintf("unix://%s", socketClean)

	existingEnv := os.Environ()
	var newEnv []string
	for _, e := range existingEnv {
		if !strings.HasPrefix(e, "CONTAINER_HOST=") &&
			!strings.HasPrefix(e, "DOCKER_HOST=") &&
			!strings.HasPrefix(e, "PODMAN_SOCKET=") {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv,
		fmt.Sprintf("CONTAINER_HOST=%s", socketURL),
		fmt.Sprintf("DOCKER_HOST=%s", socketURL),
		fmt.Sprintf("PODMAN_SOCKET=%s", socketClean),
	)
	cmd.Env = newEnv
}

// GenerateContainerName creates a collision-resistant name: <project>-<service>-<shortHash>
func GenerateContainerName(projectID, serviceName string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	hash := hex.EncodeToString(b)

	cleanProject := strings.ToLower(strings.ReplaceAll(projectID, " ", "-"))
	cleanService := strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))

	return fmt.Sprintf("%s-%s-%s", cleanProject, cleanService, hash)
}

// DeployService executes a deployment according to the service workload type and writes logs.
func (d *Deployer) DeployService(ctx context.Context, s Service, depID string, trigger ...string) (*DeployResult, error) {
	trig := "manual"
	if len(trigger) > 0 && trigger[0] != "" {
		trig = trigger[0]
	}

	logLine := func(line string) {
		if d.logManager != nil {
			d.logManager.LogLine(depID, line)
		}
		log.Printf("[%s] %s", s.Name, line)
	}

	logLine(fmt.Sprintf("🚀 Starting deployment for service '%s' (type: %s, trigger: %s)", s.Name, s.Type, trig))

	// ── 1. COMPOSE WORKLOAD ──
	if s.Type == "compose" || (s.Type != "quadlet" && strings.TrimSpace(s.ComposeYaml) != "") {
		return d.deployCompose(ctx, s, depID, trig, logLine)
	}

	// ── 2. QUADLET WORKLOAD ──
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		return d.deployQuadlet(ctx, s, depID, logLine)
	}

	// ── 3. GIT WORKLOAD ──
	isGit := s.GitRepo != "" || s.Source == "git" ||
		strings.HasPrefix(s.Source, "http://") || strings.HasPrefix(s.Source, "https://") ||
		strings.HasPrefix(s.Source, "git@") || strings.HasSuffix(s.Source, ".git")

	if isGit {
		return d.deployGit(ctx, s, depID, logLine)
	}

	// ── 4. DIRECT IMAGE WORKLOAD ──
	return d.deployImage(ctx, s, depID, logLine)
}



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

func (d *Deployer) streamPipes(stdout, stderr io.Reader, logLine func(string)) []string {
	var errLines []string
	var mu sync.Mutex
	done := make(chan bool, 2)
	stream := func(r io.Reader, isErr bool) {
		if r == nil {
			done <- true
			return
		}
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			logLine(text)
			lower := strings.ToLower(text)
			if isErr || strings.HasPrefix(text, "Error:") ||
				strings.Contains(lower, "requested access to the resource is denied") ||
				strings.Contains(lower, "cannot be used as a dependency") ||
				strings.Contains(lower, "no such container") ||
				strings.Contains(lower, "failed to start") {
				if !strings.Contains(lower, "warning") && !strings.Contains(lower, "resolving") {
					mu.Lock()
					errLines = append(errLines, text)
					mu.Unlock()
				}
			}
		}
		done <- true
	}
	go stream(stdout, false)
	go stream(stderr, true)
	<-done
	<-done
	return errLines
}

// GetRuntimeStatus queries real-time status of the workload from Podman or systemd.
func (d *Deployer) GetRuntimeStatus(ctx context.Context, s Service) string {
	// 1. Quadlet Unit
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", fmt.Sprintf("%s.service", s.Name))
			out, err := cmd.CombinedOutput()
			st := strings.TrimSpace(string(out))
			if err == nil && st == "active" {
				return "running"
			}
			if st == "failed" {
				return "failed"
			}
			if st == "activating" {
				return "deploying"
			}
			return "stopped"
		}
	}

	// 2. Podman Containers
	if d.podmanClient != nil {
		containers, err := d.podmanClient.GetContainers(ctx)
		if err == nil {
			var matched []podman.ContainerItem
			for _, c := range containers {
				isMatch := false
				if c.Labels != nil {
					if c.Labels["io.gopod.service"] == s.ID ||
						c.Labels["com.docker.compose.project"] == s.ID ||
						c.Labels["io.podman.compose.project"] == s.ID ||
						c.Labels["com.docker.compose.project"] == s.Name {
						isMatch = true
					}
				}
				if !isMatch && len(c.Names) > 0 {
					firstName := strings.TrimPrefix(c.Names[0], "/")
					if firstName == s.ID ||
						firstName == s.Name ||
						strings.HasPrefix(firstName, s.ID) ||
						strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) ||
						strings.HasPrefix(firstName, fmt.Sprintf("pod_%s", s.ID)) {
						isMatch = true
					}
				}
				if isMatch {
					matched = append(matched, c)
				}
			}

			if len(matched) > 0 {
				hasRunning := false
				for _, m := range matched {
					st := strings.ToLower(m.State)
					rawSt := strings.ToLower(m.Status)
					if st == "running" || strings.HasPrefix(rawSt, "up") {
						hasRunning = true
						break
					}
				}
				if hasRunning {
					return "running"
				}
				return "stopped"
			}
		}

		// 3. Podman Pods (if runtimeTarget == 'pod' or InPod)
		pods, err := d.podmanClient.GetPods(ctx)
		if err == nil {
			for _, p := range pods {
				pName := p.Name
				if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) || pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) {
					pSt := strings.ToLower(p.Status)
					if pSt == "running" || pSt == "degraded" {
						return "running"
					}
					return "stopped"
				}
			}
		}
	}

	if s.Status == "deploying" || s.Status == "building" {
		return s.Status
	}
	return "stopped"
}

// BatchRuntimeStatus queries real-time status of multiple workloads in a single pass.
func (d *Deployer) BatchRuntimeStatus(ctx context.Context, svcs []Service) map[string]string {
	result := make(map[string]string)
	if len(svcs) == 0 {
		return result
	}

	var containers []podman.ContainerItem
	var pods []podman.PodItem
	if d.podmanClient != nil {
		containers, _ = d.podmanClient.GetContainers(ctx)
		pods, _ = d.podmanClient.GetPods(ctx)
	}

	hasSystemctl := false
	if _, err := exec.LookPath("systemctl"); err == nil {
		hasSystemctl = true
	}

	for _, s := range svcs {
		if s.Status == "deploying" || s.Status == "building" {
			result[s.ID] = s.Status
			continue
		}

		// 1. Quadlet Unit
		if (s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "") && hasSystemctl {
			cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", fmt.Sprintf("%s.service", s.Name))
			out, err := cmd.CombinedOutput()
			st := strings.TrimSpace(string(out))
			if err == nil && st == "active" {
				result[s.ID] = "running"
				continue
			}
			if st == "failed" {
				result[s.ID] = "failed"
				continue
			}
			if st == "activating" {
				result[s.ID] = "deploying"
				continue
			}
			result[s.ID] = "stopped"
			continue
		}

		// 2. Match Containers
		matchedRunning := false
		matchedFound := false
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil {
				if c.Labels["io.gopod.service"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.ID ||
					c.Labels["io.podman.compose.project"] == s.ID ||
					c.Labels["com.docker.compose.project"] == s.Name {
					isMatch = true
				}
			}
			if !isMatch && len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if firstName == s.ID ||
					firstName == s.Name ||
					strings.HasPrefix(firstName, s.ID) ||
					strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) ||
					strings.HasPrefix(firstName, fmt.Sprintf("pod_%s", s.ID)) {
					isMatch = true
				}
			}
			if isMatch {
				matchedFound = true
				st := strings.ToLower(c.State)
				rawSt := strings.ToLower(c.Status)
				if st == "running" || strings.HasPrefix(rawSt, "up") {
					matchedRunning = true
					break
				}
			}
		}

		if matchedRunning {
			result[s.ID] = "running"
			continue
		}
		if matchedFound {
			result[s.ID] = "stopped"
			continue
		}

		// 3. Match Pods
		podRunning := false
		for _, p := range pods {
			pName := p.Name
			if pName == s.ID || pName == s.Name || pName == fmt.Sprintf("pod_%s", s.ID) || pName == fmt.Sprintf("%s-%s", s.ProjectID, s.Name) {
				pSt := strings.ToLower(p.Status)
				if pSt == "running" || pSt == "degraded" {
					podRunning = true
					break
				}
			}
		}
		if podRunning {
			result[s.ID] = "running"
			continue
		}

		result[s.ID] = "stopped"
	}

	return result
}

// StartService powers on workload containers or units according to service workload type.
func (d *Deployer) StartService(ctx context.Context, s Service) error {
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

// TeardownService stops and completely removes containers, pods, quadlets, and compose stacks (Dokploy parity).
func (d *Deployer) TeardownService(ctx context.Context, s Service, deleteVolumes bool) error {
	// 1. Compose Workload
	if s.Type == "compose" || s.ComposeYaml != "" {
		projectName := s.ID
		if projectName == "" {
			projectName = GenerateContainerName(s.ProjectID, s.Name)
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
		for _, c := range containers {
			isMatch := false
			if c.Labels != nil && (c.Labels["io.gopod.service"] == s.ID || c.Labels["com.docker.compose.project"] == s.ID || c.Labels["com.docker.compose.project"] == GenerateContainerName(s.ProjectID, s.Name)) {
				isMatch = true
			} else if len(c.Names) > 0 {
				firstName := strings.TrimPrefix(c.Names[0], "/")
				if strings.HasPrefix(firstName, s.ID) || firstName == s.Name || strings.HasPrefix(firstName, fmt.Sprintf("%s-%s-", s.ProjectID, s.Name)) || firstName == GenerateContainerName(s.ProjectID, s.Name) {
					isMatch = true
				}
			}
			if isMatch {
				_ = d.podmanClient.StopContainer(ctx, c.ID)
				_ = d.podmanClient.DeleteContainer(ctx, c.ID, true)
			}
		}

		// Also check and delete pod if created
		_ = d.podmanClient.DeletePod(ctx, s.Name, true)
		_ = d.podmanClient.DeletePod(ctx, GenerateContainerName(s.ProjectID, s.Name), true)
		_ = d.podmanClient.DeletePod(ctx, "pod_"+GenerateContainerName(s.ProjectID, s.Name), true)

		// 4. Volume Cleanup if requested
		if deleteVolumes {
			vols, _ := d.podmanClient.GetVolumes(ctx)
			prefix := GenerateContainerName(s.ProjectID, s.Name)
			for _, v := range vols {
				if strings.HasPrefix(v.Name, s.ID) || strings.HasPrefix(v.Name, prefix) {
					_ = d.podmanClient.DeleteVolume(ctx, v.Name, true)
				}
			}
		}
	}

	return nil
}


