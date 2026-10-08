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

// GetVolumes returns list of local named volumes.
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

// PruneVolumes removes unused local volumes.
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

// DeleteVolume removes a volume, optionally forcing removal.
func (c *Client) DeleteVolume(ctx context.Context, name string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/volumes/%s?force=%t", name, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman delete volume failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete volume failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	args := []string{"volume", "rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)
	return exec.CommandContext(ctx, "podman", args...).Run()
}
