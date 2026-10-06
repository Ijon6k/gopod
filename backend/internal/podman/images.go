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

// GetImages returns container images in local storage.
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

// PruneImages cleans up unused container images.
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
