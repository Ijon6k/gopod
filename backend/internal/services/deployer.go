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

	"gopod/internal/credentials"
	"gopod/internal/podman"
)

// SSHKeyProvider provides private SSH keys for Git authentication.
type SSHKeyProvider interface {
	GetPrivateKey(ctx context.Context, id string) (string, error)
}

// RegistryProvider provides credentials for private container registries.
type RegistryProvider interface {
	GetRegistry(ctx context.Context, id string) (*credentials.ContainerRegistry, error)
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
	podmanClient     *podman.Client
	sshKeyProvider   SSHKeyProvider
	registryProvider RegistryProvider
	logManager       *LogManager
	dataDir          string
}

// NewDeployer creates a new container deployer.
func NewDeployer(client *podman.Client, keyProvider SSHKeyProvider, logMgr *LogManager, dataDir string) *Deployer {
	if dataDir == "" {
		dataDir = "./data"
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	_ = os.MkdirAll(dataDir, 0755)
	return &Deployer{
		podmanClient:   client,
		sshKeyProvider: keyProvider,
		logManager:     logMgr,
		dataDir:        dataDir,
	}
}

// SetRegistryProvider configures private registry credential resolution.
func (d *Deployer) SetRegistryProvider(rp RegistryProvider) {
	d.registryProvider = rp
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
		// Prevent internal control-plane environment variables from polluting workload subprocesses
		if strings.HasPrefix(e, "PORT=") ||
			strings.HasPrefix(e, "CADDY_ADMIN_URL=") ||
			strings.HasPrefix(e, "GOPOD_DB_PATH=") {
			continue
		}
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

	// Guarantee rootless systemd user session environment (XDG_RUNTIME_DIR and DBUS)
	uid := os.Getuid()
	userRunDir := fmt.Sprintf("/run/user/%d", uid)
	hasXDG := false
	for _, e := range newEnv {
		if strings.HasPrefix(e, "XDG_RUNTIME_DIR=") {
			hasXDG = true
			break
		}
	}
	if !hasXDG {
		if _, err := os.Stat(userRunDir); err == nil {
			newEnv = append(newEnv, fmt.Sprintf("XDG_RUNTIME_DIR=%s", userRunDir))
		}
	}

	hasDBUS := false
	for _, e := range newEnv {
		if strings.HasPrefix(e, "DBUS_SESSION_BUS_ADDRESS=") {
			hasDBUS = true
			break
		}
	}
	if !hasDBUS {
		busPath := filepath.Join(userRunDir, "bus")
		if _, err := os.Stat(busPath); err == nil {
			newEnv = append(newEnv, fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=%s", busPath))
		}
	}

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

	defer func() {
		if d.logManager != nil {
			d.logManager.CloseLog(depID)
		}
	}()

	logLine := func(line string) {
		if d.logManager != nil {
			d.logManager.LogLine(depID, line)
		}
		log.Printf("[%s] %s", s.Name, line)
	}

	logLine(fmt.Sprintf("🚀 Starting deployment for service '%s' (type: %s, trigger: %s)", s.Name, s.Type, trig))

	// ── 1. POD WORKLOAD (Must precede k8s/compose to prevent interception) ──
	if s.Type == "pod" || s.SourceType == "pod" || (s.RuntimeTarget == "pod" && s.Image == "" && s.GitRepo == "") {
		return d.deployPod(ctx, s, depID, logLine)
	}

	// ── 2. KUBERNETES MANIFEST WORKLOAD ──
	if (s.Type == "kubernetes" || s.Type == "k8s" || strings.TrimSpace(s.K8sYaml) != "") && s.Type != "pod" && s.SourceType != "pod" && s.Type != "compose" && s.Type != "quadlet" {
		return d.deployKube(ctx, s, depID, trig, logLine)
	}

	// ── 3. COMPOSE WORKLOAD ──
	if (s.Type == "compose" || strings.TrimSpace(s.ComposeYaml) != "") && s.Type != "pod" && s.SourceType != "pod" && s.Type != "quadlet" {
		return d.deployCompose(ctx, s, depID, trig, logLine)
	}

	// ── 4. QUADLET WORKLOAD ──
	if s.Type == "quadlet" || s.RuntimeTarget == "quadlet" || strings.TrimSpace(s.QuadletConfig) != "" {
		return d.deployQuadlet(ctx, s, depID, trig, logLine)
	}

	// ── 5. STATIC DROP WORKLOAD ──
	if s.SourceType == "drop" || s.Type == "static" {
		return d.deployDrop(ctx, s, depID, trig, logLine)
	}

	// ── 6. GIT WORKLOAD ──
	isGit := s.GitRepo != "" || s.Source == "git" ||
		strings.HasPrefix(s.Source, "http://") || strings.HasPrefix(s.Source, "https://") ||
		strings.HasPrefix(s.Source, "git@") || strings.HasSuffix(s.Source, ".git")

	if isGit {
		return d.deployGit(ctx, s, depID, logLine)
	}

	// ── 7. DIRECT IMAGE WORKLOAD ──
	return d.deployImage(ctx, s, depID, trig, logLine)
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
		if err := scanner.Err(); err != nil && err != io.EOF {
			logLine("[stream error] " + err.Error())
		}
		done <- true
	}
	go stream(stdout, false)
	go stream(stderr, true)
	<-done
	<-done
	return errLines
}
