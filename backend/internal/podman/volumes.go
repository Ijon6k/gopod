package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getVolumeSize(mountPoint string) string {
	if mountPoint == "" {
		return "—"
	}
	var total int64
	_ = filepath.Walk(mountPoint, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if total == 0 {
		return "0 B"
	}
	return formatBytes(total)
}

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
			Name       string            `json:"Name"`
			Driver     string            `json:"Driver"`
			Mountpoint string            `json:"Mountpoint"`
			CreatedAt  string            `json:"CreatedAt"`
			Labels     map[string]string `json:"Labels"`
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
				Size:       getVolumeSize(item.Mountpoint),
				Labels:     item.Labels,
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
		mountPoint := getString(item, "Mountpoint")
		vols = append(vols, VolumeItem{
			Name:       getString(item, "Name"),
			Driver:     getString(item, "Driver"),
			MountPoint: mountPoint,
			Size:       getVolumeSize(mountPoint),
			Labels:     getLabels(item, "Labels"),
		})
	}
	return vols, nil
}

// CreateVolume ensures a named local volume exists, attaching the supplied
// ownership labels. Podman creates named volumes implicitly on first use, but
// an implicit volume carries no labels — which makes it impossible to prove
// ownership later. Creating the volume explicitly (and idempotently) keeps the
// label attached so teardown and garbage collection can match it. An already
// existing volume is treated as success: its original labels are left as-is.
func (c *Client) CreateVolume(ctx context.Context, name string, labels map[string]string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("volume name is required")
	}
	if c.httpClient != nil {
		payload, err := json.Marshal(struct {
			Name   string            `json:"Name"`
			Labels map[string]string `json:"Labels,omitempty"`
		}{Name: name, Labels: labels})
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/v5.0.0/libpod/volumes/create", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman create volume failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusConflict || strings.Contains(strings.ToLower(string(body)), "already exists") {
			return nil
		}
		return fmt.Errorf("podman create volume failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	args := []string{"volume", "create"}
	for k, v := range labels {
		if k == "" {
			continue
		}
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, name)
	if err := exec.CommandContext(ctx, "podman", args...).Run(); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return err
	}
	return nil
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
