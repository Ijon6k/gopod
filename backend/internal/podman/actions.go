package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// PodItem represents a Podman pod.
type PodItem struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Containers []string `json:"containers"`
	Network    string   `json:"network"`
	Created    string   `json:"created"`
}

// ImageItem represents a container image in local storage.
type ImageItem struct {
	ID        string `json:"id"`
	Repository string `json:"name"`
	Tag       string `json:"tag"`
	Size      string `json:"size"`
	Created   string `json:"createdAt"`
}

// VolumeItem represents a local named volume.
type VolumeItem struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	MountPoint string `json:"mount"`
	Size       string `json:"size"`
}

// NetworkItem represents a container network.
type NetworkItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// RunContainerOptions specifies parameters for launching a new container.
type RunContainerOptions struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Ports   []string          `json:"ports,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Network string            `json:"network,omitempty"`
}

// RunContainer starts a new container with the specified options.
func (c *Client) RunContainer(ctx context.Context, opts RunContainerOptions) (string, error) {
	args := []string{"run", "-d", "--name", opts.Name}
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	args = append(args, opts.Image)

	cmd := exec.CommandContext(ctx, "podman", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman run failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ── Container Lifecycle Actions ──

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "start")
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "stop")
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.postContainerAction(ctx, id, "restart")
}

func (c *Client) DeleteContainer(ctx context.Context, id string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s?force=%t", id, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
					return nil
				}
			}
		}
	}

	// CLI fallback
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)
	return exec.CommandContext(ctx, "podman", args...).Run()
}

func (c *Client) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}

	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s/logs?stdout=true&stderr=true&tail=%d", id, tail)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				return string(body), nil
			}
		}
	}

	// CLI fallback
	cmd := exec.CommandContext(ctx, "podman", "logs", "--tail", fmt.Sprintf("%d", tail), id)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (c *Client) postContainerAction(ctx context.Context, id, action string) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/containers/%s/%s", id, action)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
					return nil
				}
			}
		}
	}

	// CLI fallback
	return exec.CommandContext(ctx, "podman", action, id).Run()
}

// ── Pods ──

func (c *Client) GetPods(ctx context.Context) ([]PodItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/pods/json", nil)
		if err == nil {
			resp, err := c.httpClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var raw []struct {
					ID         string `json:"Id"`
					Name       string `json:"Name"`
					Status     string `json:"Status"`
					Created    string `json:"Created"`
					Containers []struct {
						Names []string `json:"Names"`
					} `json:"Containers"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
					var result []PodItem
					for _, p := range raw {
						var cNames []string
						for _, cont := range p.Containers {
							if len(cont.Names) > 0 {
								cNames = append(cNames, cont.Names[0])
							}
						}
						idShort := p.ID
						if len(idShort) > 12 {
							idShort = idShort[:12]
						}
						result = append(result, PodItem{
							ID:         idShort,
							Name:       p.Name,
							Status:     p.Status,
							Containers: cNames,
							Created:    p.Created,
						})
					}
					return result, nil
				}
			}
		}
	}

	// CLI fallback
	cmd := exec.CommandContext(ctx, "podman", "pod", "ps", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	var pods []PodItem
	for _, item := range raw {
		id := getString(item, "Id")
		if len(id) > 12 {
			id = id[:12]
		}
		pods = append(pods, PodItem{
			ID:      id,
			Name:    getString(item, "Name"),
			Status:  getString(item, "Status"),
			Created: getString(item, "Created"),
		})
	}
	return pods, nil
}

// ── Images ──

func (c *Client) GetImages(ctx context.Context) ([]ImageItem, error) {
	cmd := exec.CommandContext(ctx, "podman", "images", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	var images []ImageItem
	for _, item := range raw {
		id := getString(item, "Id")
		if len(id) > 12 {
			id = id[:12]
		}
		repo := getString(item, "Repository")
		tag := getString(item, "Tag")
		size := getString(item, "Size")
		created := getString(item, "Created")

		images = append(images, ImageItem{
			ID:         id,
			Repository: repo,
			Tag:        tag,
			Size:       size,
			Created:    created,
		})
	}
	return images, nil
}

func (c *Client) PruneImages(ctx context.Context, all bool) error {
	args := []string{"image", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}
	return exec.CommandContext(ctx, "podman", args...).Run()
}

// ── Volumes & Storage ──

func (c *Client) GetVolumes(ctx context.Context) ([]VolumeItem, error) {
	cmd := exec.CommandContext(ctx, "podman", "volume", "ls", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	var vols []VolumeItem
	for _, item := range raw {
		vols = append(vols, VolumeItem{
			Name:       getString(item, "Name"),
			Driver:     getString(item, "Driver"),
			MountPoint: getString(item, "Mountpoint"),
			Size:       "Calculated",
		})
	}
	return vols, nil
}

func (c *Client) PruneVolumes(ctx context.Context) error {
	return exec.CommandContext(ctx, "podman", "volume", "prune", "-f").Run()
}

// ── Networks ──

func (c *Client) GetNetworks(ctx context.Context) ([]NetworkItem, error) {
	cmd := exec.CommandContext(ctx, "podman", "network", "ls", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}

	var nets []NetworkItem
	for _, item := range raw {
		nets = append(nets, NetworkItem{
			ID:     getString(item, "network_id"),
			Name:   getString(item, "name"),
			Driver: getString(item, "driver"),
		})
	}
	return nets, nil
}

// PruneSystem cleans up stopped containers, unused networks, and dangling images.
func (c *Client) PruneSystem(ctx context.Context) error {
	return exec.CommandContext(ctx, "podman", "system", "prune", "-f").Run()
}
