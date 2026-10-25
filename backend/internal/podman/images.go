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

// SplitImageNameTag separates image repository from tag, correctly handling registry port colons.
func SplitImageNameTag(fullName string) (string, string) {
	if fullName == "" {
		return "<none>", "<none>"
	}
	lastColon := strings.LastIndex(fullName, ":")
	lastSlash := strings.LastIndex(fullName, "/")
	if lastColon > -1 && lastColon > lastSlash {
		return fullName[:lastColon], fullName[lastColon+1:]
	}
	return fullName, "latest"
}

// GetImages returns container images in local storage.
func (c *Client) GetImages(ctx context.Context) ([]ImageItem, error) {
	if c.httpClient != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/images/json", nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman images request failed: %w", err)
		}
		defer cancel()
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
				lastColon := strings.LastIndex(fullName, ":")
				lastSlash := strings.LastIndex(fullName, "/")
				if lastColon > -1 && lastColon > lastSlash {
					repo = fullName[:lastColon]
					tag = fullName[lastColon+1:]
				} else {
					repo = fullName
					tag = "latest"
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

// DanglingImageStat summarises untagged (dangling) images: the intermediate
// build layers that accumulate during image builds and are unreachable by any
// reference.
type DanglingImageStat struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// GetDanglingImages counts untagged images and their total byte size.
func (c *Client) GetDanglingImages(ctx context.Context) (*DanglingImageStat, error) {
	if c.httpClient != nil {
		url := fmt.Sprintf(`http://d/%s/libpod/images/json?filters={"dangling":{"true":true}}`, c.libpodVersion())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman dangling images request failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("podman dangling images returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var raw []struct {
			ID   string `json:"Id"`
			Size int64  `json:"Size"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}
		stat := &DanglingImageStat{Count: len(raw)}
		for _, item := range raw {
			if item.Size > 0 {
				stat.Bytes += item.Size
			}
		}
		return stat, nil
	}

	cmd := exec.CommandContext(ctx, "podman", "images", "--filter", "dangling=true", "--format", "json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, err
	}
	stat := &DanglingImageStat{Count: len(raw)}
	for _, item := range raw {
		if size, ok := item["Size"].(float64); ok && size > 0 {
			stat.Bytes += int64(size)
		} else if size, ok := item["Size"].(int64); ok && size > 0 {
			stat.Bytes += size
		}
	}
	return stat, nil
}

// PruneImages cleans up unused container images.
func (c *Client) PruneImages(ctx context.Context, all bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/images/prune", c.libpodVersion())
		if all {
			url += "?all=true"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, pruneTimeout, req)
		if err != nil {
			return fmt.Errorf("podman image prune failed: %w", err)
		}
		defer cancel()
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
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}

// RemoveImage removes an image from local storage. Podman refuses images
// referenced by containers unless force is set, so the default call surface
// gets the engine's own in-use refusal with its real reason.
func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/images/%s?force=%t", c.libpodVersion(), id, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman remove image failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		// 404 is expected when the snapshot the page holds is already stale
		// (the image was removed elsewhere); removal stays idempotent.
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman remove image failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, id)
	cmd := exec.CommandContext(ctx, "podman", args...)
	c.SetupCmdEnv(cmd)
	return cmd.Run()
}
