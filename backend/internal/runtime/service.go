package runtime

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"gopod/internal/podman"
)

// Service encapsulates container engine interactions and telemetry business logic.
type Service struct {
	client      *podman.Client
	statsMu     sync.RWMutex
	cachedStats []podman.ContainerStat
	statsTime   time.Time

	// envSecretKeys reports env var keys GOPOD stores encrypted. Quadlet API
	// responses mask their values; the unit files on disk keep the real value
	// because systemd needs it.
	envSecretKeys func() map[string]bool
}

// NewService creates a new runtime domain service.
func NewService(client *podman.Client) *Service {
	return &Service{client: client}
}

// SetEnvSecretKeyProvider installs the callback that returns env var keys
// stored encrypted. Without it, Quadlet responses are returned verbatim.
func (s *Service) SetEnvSecretKeyProvider(fn func() map[string]bool) {
	s.envSecretKeys = fn
}

// secretEnvKeys returns the current secret key set (nil when unset).
func (s *Service) secretEnvKeys() map[string]bool {
	if s.envSecretKeys == nil {
		return nil
	}
	return s.envSecretKeys()
}

// SetupCmdEnv configures subprocess environment with active Podman socket.
func (s *Service) SetupCmdEnv(cmd *exec.Cmd) {
	if s.client != nil {
		s.client.SetupCmdEnv(cmd)
	}
}

// Client returns the underlying Podman client.
func (s *Service) Client() *podman.Client {
	return s.client
}

// SocketPath returns the active Podman socket path.
func (s *Service) SocketPath() string {
	if s.client == nil {
		return ""
	}
	return s.client.SocketPath()
}

// PingSocket reports whether the podman socket answers a libpod ping probe.
// Probe-only (no cache/cooldown side effects) so health checks leave the
// handshake machinery untouched.
func (s *Service) PingSocket() bool {
	if s.client == nil {
		return false
	}
	return s.client.Ping()
}

// GetSystemInfo retrieves host and engine metrics.
func (s *Service) GetSystemInfo(ctx context.Context) (*podman.SystemInfo, error) {
	return s.client.GetSystemInfo(ctx)
}

// GetContainerStats retrieves current CPU and memory consumption with 2s TTL cache to prevent CPU thrashing.
func (s *Service) GetContainerStats(ctx context.Context) ([]podman.ContainerStat, error) {
	s.statsMu.RLock()
	if time.Since(s.statsTime) < 2*time.Second && s.cachedStats != nil {
		cached := s.cachedStats
		s.statsMu.RUnlock()
		return cached, nil
	}
	s.statsMu.RUnlock()

	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	if time.Since(s.statsTime) < 2*time.Second && s.cachedStats != nil {
		return s.cachedStats, nil
	}

	stats, err := s.client.GetContainerStats(ctx)
	if err == nil {
		s.cachedStats = stats
		s.statsTime = time.Now()
	}
	return stats, err
}

// GetContainers returns all local containers with metrics.
func (s *Service) GetContainers(ctx context.Context) ([]podman.ContainerItem, error) {
	return s.client.GetContainers(ctx)
}

// StartContainer starts a container.
func (s *Service) StartContainer(ctx context.Context, id string) error {
	return s.client.StartContainer(ctx, id)
}

// StopContainer stops a container.
func (s *Service) StopContainer(ctx context.Context, id string) error {
	return s.client.StopContainer(ctx, id)
}

// RestartContainer restarts a container.
func (s *Service) RestartContainer(ctx context.Context, id string) error {
	return s.client.RestartContainer(ctx, id)
}

// DeleteContainer removes a container.
func (s *Service) DeleteContainer(ctx context.Context, id string, force bool) error {
	return s.client.DeleteContainer(ctx, id, force)
}

// GetContainerLogs returns logs for a container.
func (s *Service) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return s.client.GetContainerLogs(ctx, id, tail)
}

// PruneContainers deletes stopped containers and resets failed systemd units.
func (s *Service) PruneContainers(ctx context.Context) error {
	err := s.client.PruneContainers(ctx)
	if _, lookErr := exec.LookPath("systemctl"); lookErr == nil {
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed").Run()
	}
	return err
}

// GetPods lists Podman pods.
func (s *Service) GetPods(ctx context.Context) ([]podman.PodItem, error) {
	return s.client.GetPods(ctx)
}

// CreatePod creates a new Podman pod.
func (s *Service) CreatePod(ctx context.Context, name string, ports []string, network string, labels map[string]string) (string, error) {
	return s.client.CreatePod(ctx, podman.CreatePodOptions{
		Name:    name,
		Ports:   ports,
		Network: network,
		Labels:  labels,
	})
}

// StartPod starts a Podman pod.
func (s *Service) StartPod(ctx context.Context, idOrName string) error {
	return s.client.StartPod(ctx, idOrName)
}

// StopPod stops a Podman pod.
func (s *Service) StopPod(ctx context.Context, idOrName string) error {
	return s.client.StopPod(ctx, idOrName)
}

// RestartPod restarts a Podman pod.
func (s *Service) RestartPod(ctx context.Context, idOrName string) error {
	return s.client.RestartPod(ctx, idOrName)
}

// DeletePod deletes a Podman pod.
func (s *Service) DeletePod(ctx context.Context, idOrName string, force bool) error {
	return s.client.DeletePod(ctx, idOrName, force)
}

// GetImages lists local container images.
func (s *Service) GetImages(ctx context.Context) ([]podman.ImageItem, error) {
	return s.client.GetImages(ctx)
}

// PruneImages deletes unused images.
func (s *Service) PruneImages(ctx context.Context, all bool) error {
	return s.client.PruneImages(ctx, all)
}

// GetVolumes lists local volumes.
func (s *Service) GetVolumes(ctx context.Context) ([]podman.VolumeItem, error) {
	return s.client.GetVolumes(ctx)
}

// PruneVolumes deletes unused volumes.
func (s *Service) PruneVolumes(ctx context.Context) error {
	return s.client.PruneVolumes(ctx)
}

// GetNetworks lists container networks.
func (s *Service) GetNetworks(ctx context.Context) ([]podman.NetworkItem, error) {
	return s.client.GetNetworks(ctx)
}

// ErrManagedNetwork reports an attempt to remove the platform network through
// the Runtime page; live ingress routing depends on it.
var ErrManagedNetwork = errors.New("gopod-net is managed by gopod (ingress routing depends on it) and cannot be removed")

// RemoveImage deletes an image from local storage. Without force, Podman
// refuses images referenced by containers with its own reason.
func (s *Service) RemoveImage(ctx context.Context, id string, force bool) error {
	return s.client.RemoveImage(ctx, id, force)
}

// DeleteVolume removes a named volume. Without force, Podman refuses volumes
// attached to containers with its own reason.
func (s *Service) DeleteVolume(ctx context.Context, name string, force bool) error {
	return s.client.DeleteVolume(ctx, name, force)
}

// RemoveNetwork deletes a user-defined network. The platform-managed network
// is refused here so the Runtime page cannot break live routing. Attached
// containers make Podman refuse the removal with its own reason.
func (s *Service) RemoveNetwork(ctx context.Context, name string) error {
	if name == "gopod-net" {
		return ErrManagedNetwork
	}
	return s.client.RemoveNetwork(ctx, name)
}

// PruneSystem cleans up stopped containers, dangling images, and networks, and resets failed systemd units.
func (s *Service) PruneSystem(ctx context.Context) error {
	err := s.client.PruneSystem(ctx)
	if _, lookErr := exec.LookPath("systemctl"); lookErr == nil {
		_ = exec.CommandContext(ctx, "systemctl", "--user", "reset-failed").Run()
	}
	return err
}

// GetDiskUsage queries disk space metrics from Podman.
func (s *Service) GetDiskUsage(ctx context.Context) ([]podman.SystemDiskUsage, error) {
	return s.client.GetDiskUsage(ctx)
}
