package runtime

import (
	"context"

	"gopod/internal/podman"
)

// Service encapsulates container engine interactions and telemetry business logic.
type Service struct {
	client *podman.Client
}

// NewService creates a new runtime domain service.
func NewService(client *podman.Client) *Service {
	return &Service{client: client}
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

// GetSystemInfo retrieves host and engine metrics.
func (s *Service) GetSystemInfo(ctx context.Context) (*podman.SystemInfo, error) {
	return s.client.GetSystemInfo(ctx)
}

// GetContainerStats retrieves current CPU and memory consumption.
func (s *Service) GetContainerStats(ctx context.Context) ([]podman.ContainerStat, error) {
	return s.client.GetContainerStats(ctx)
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

// GetPods lists Podman pods.
func (s *Service) GetPods(ctx context.Context) ([]podman.PodItem, error) {
	return s.client.GetPods(ctx)
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

// PruneSystem cleans up stopped containers, dangling images, and networks.
func (s *Service) PruneSystem(ctx context.Context) error {
	return s.client.PruneSystem(ctx)
}
