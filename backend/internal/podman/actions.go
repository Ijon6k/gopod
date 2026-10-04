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
	"time"
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
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman delete failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		if err != nil {
			return "", err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("podman logs request failed: %w", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
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
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman action %s failed: %w", action, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman %s failed (HTTP %d): %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// CLI fallback
	return exec.CommandContext(ctx, "podman", action, id).Run()
}

// ── Pods ──

func (c *Client) GetPods(ctx context.Context) ([]PodItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/pods/json", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("podman pods request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman pods returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			ID         string `json:"Id"`
			Name       string `json:"Name"`
			Status     string `json:"Status"`
			Created    string `json:"Created"`
			Containers []struct {
				ID    string      `json:"Id"`
				Names interface{} `json:"Names"`
			} `json:"Containers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		var result []PodItem
		for _, p := range raw {
			var cNames []string
			for _, cont := range p.Containers {
				switch v := cont.Names.(type) {
				case string:
					if v != "" {
						cNames = append(cNames, strings.TrimPrefix(v, "/"))
					}
				case []interface{}:
					for _, item := range v {
						if str, ok := item.(string); ok && str != "" {
							cNames = append(cNames, strings.TrimPrefix(str, "/"))
						}
					}
				case []string:
					for _, str := range v {
						if str != "" {
							cNames = append(cNames, strings.TrimPrefix(str, "/"))
						}
					}
				}
				if len(cNames) == 0 && cont.ID != "" {
					idShort := cont.ID
					if len(idShort) > 12 {
						idShort = idShort[:12]
					}
					cNames = append(cNames, idShort)
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
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/images/json", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("podman images request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman images returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			ID       string   `json:"Id"`
			RepoTags []string `json:"RepoTags"`
			Names    []string `json:"Names"`
			Size     int64    `json:"Size"`
			Created  int64    `json:"Created"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		var images []ImageItem
		for _, item := range raw {
			idShort := item.ID
			if len(idShort) > 12 {
				idShort = idShort[:12]
			}

			repo := "<none>"
			tag := "<none>"
			fullName := ""
			if len(item.RepoTags) > 0 && item.RepoTags[0] != "" {
				fullName = item.RepoTags[0]
			} else if len(item.Names) > 0 && item.Names[0] != "" {
				fullName = item.Names[0]
			}

			if fullName != "" {
				parts := strings.Split(fullName, ":")
				repo = parts[0]
				if len(parts) > 1 {
					tag = parts[1]
				}
			}

			createdStr := "Recent"
			if item.Created > 0 {
				createdStr = time.Unix(item.Created, 0).Format("2006-01-02 15:04")
			}
			images = append(images, ImageItem{
				ID:         idShort,
				Repository: repo,
				Tag:        tag,
				Size:       formatBytes(item.Size),
				Created:    createdStr,
			})
		}
		return images, nil
	}

	// CLI fallback
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
	if c.httpClient != nil {
		url := "http://d/v5.0.0/libpod/images/prune"
		if all {
			url += "?all=true"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman image prune failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman image prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	args := []string{"image", "prune", "-f"}
	if all {
		args = append(args, "-a")
	}
	return exec.CommandContext(ctx, "podman", args...).Run()
}

// ── Volumes & Storage ──

func (c *Client) GetVolumes(ctx context.Context) ([]VolumeItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/volumes/json", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("podman volumes request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman volumes returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
			CreatedAt  string `json:"CreatedAt"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		var vols []VolumeItem
		for _, item := range raw {
			vols = append(vols, VolumeItem{
				Name:       item.Name,
				Driver:     item.Driver,
				MountPoint: item.Mountpoint,
				Size:       "Active",
			})
		}
		return vols, nil
	}

	// CLI fallback
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
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v5.0.0/libpod/volumes/prune", nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman volume prune failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman volume prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return exec.CommandContext(ctx, "podman", "volume", "prune", "-f").Run()
}

// ── Networks ──

func (c *Client) GetNetworks(ctx context.Context) ([]NetworkItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v5.0.0/libpod/networks/json", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("podman networks request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman networks returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var raw []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Driver  string `json:"driver"`
			Subnets []struct {
				Subnet  string `json:"subnet"`
				Gateway string `json:"gateway"`
			} `json:"subnets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}

		var nets []NetworkItem
		for _, item := range raw {
			idShort := item.ID
			if len(idShort) > 12 {
				idShort = idShort[:12]
			}
			subnet := "—"
			gateway := "—"
			if len(item.Subnets) > 0 {
				subnet = item.Subnets[0].Subnet
				gateway = item.Subnets[0].Gateway
			}
			nets = append(nets, NetworkItem{
				ID:      idShort,
				Name:    item.Name,
				Driver:  item.Driver,
				Subnet:  subnet,
				Gateway: gateway,
			})
		}
		return nets, nil
	}

	// CLI fallback
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
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v5.0.0/libpod/system/prune", nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman system prune failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman system prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return exec.CommandContext(ctx, "podman", "system", "prune", "-f").Run()
}
