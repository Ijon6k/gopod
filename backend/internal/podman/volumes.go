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

// volumeSizes reports per-volume disk usage from the podman system df
// dataset. Walking volume mountpoints from this process is unreliable once
// GoPod itself runs inside a container: the host storage root is not mounted
// there, so every walk would quietly measure zero bytes. Podman owns the
// storage tree, so its numbers are authoritative.
func (c *Client) volumeSizes(ctx context.Context) map[string]int64 {
	sizes := make(map[string]int64)

	type dfVolume struct {
		VolumeName string `json:"VolumeName"`
		Size       int64  `json:"Size"`
	}
	var df struct {
		Volumes []dfVolume `json:"Volumes"`
	}

	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/system/df", nil)
		if err != nil {
			return sizes
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return sizes
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return sizes
		}
		if err := json.NewDecoder(resp.Body).Decode(&df); err != nil {
			return sizes
		}
	} else {
		cmd := exec.CommandContext(ctx, "podman", "system", "df", "--format", "json")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return sizes
		}
		if err := json.Unmarshal(stdout.Bytes(), &df); err != nil {
			return sizes
		}
	}

	for _, v := range df.Volumes {
		if v.VolumeName != "" {
			sizes[v.VolumeName] = v.Size
		}
	}
	return sizes
}

// volumeSizeLabel prefers podman's reported usage and falls back to a local
// filesystem walk for volumes the df dataset does not cover.
func volumeSizeLabel(sizes map[string]int64, name, mountPoint string) string {
	if used, ok := sizes[name]; ok {
		return formatBytes(used)
	}
	return getVolumeSize(mountPoint)
}

// GetVolumes returns list of local named volumes.
func (c *Client) GetVolumes(ctx context.Context) ([]VolumeItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/volumes/json", nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman volumes request failed: %w", err)
		}
		defer cancel()
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
		sizes := c.volumeSizes(ctx)
		for _, item := range raw {
			vols = append(vols, VolumeItem{
				Name:       item.Name,
				Driver:     item.Driver,
				MountPoint: item.Mountpoint,
				Size:       volumeSizeLabel(sizes, item.Name, item.Mountpoint),
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
	sizes := c.volumeSizes(ctx)
	for _, item := range raw {
		mountPoint := getString(item, "Mountpoint")
		vols = append(vols, VolumeItem{
			Name:       getString(item, "Name"),
			Driver:     getString(item, "Driver"),
			MountPoint: mountPoint,
			Size:       volumeSizeLabel(sizes, getString(item, "Name"), mountPoint),
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://d/"+c.libpodVersion()+"/libpod/volumes/create", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman create volume failed: %w", err)
		}
		defer cancel()
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
	args = append(args, "--", name)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	if err := cmd.Run(); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

// PruneVolumes removes unused volumes gopod owns. Unused volumes frequently
// belong to other tooling on the host, so the ownership-label filter applies
// to both the libpod API request and the CLI fallback — matching the "named
// volumes are never removed unless provably orphaned" GC policy of gopod.
func (c *Client) PruneVolumes(ctx context.Context) error {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, pruneFiltersURL("http://d/"+c.libpodVersion()+"/libpod/volumes/prune"), nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, pruneTimeout, req)
		if err != nil {
			return fmt.Errorf("podman volume prune failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman volume prune failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// CLI fallback targets the configured gopod socket, not the ambient one.
	cmd := exec.CommandContext(ctx, "podman", "volume", "prune", "-f", "--filter", "label="+OwnershipLabel)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}

// DeleteVolume removes a volume, optionally forcing removal.
func (c *Client) DeleteVolume(ctx context.Context, name string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/volumes/%s?force=%t", c.libpodVersion(), name, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman delete volume failed: %w", err)
		}
		defer cancel()
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
	// `--` guarantees the volume name is parsed as an identifier; a crafted
	// volume name starting with a dash would otherwise become a CLI flag
	// (e.g. `podman volume rm -f --all` wipes every volume on the host).
	args = append(args, "--", name)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}
