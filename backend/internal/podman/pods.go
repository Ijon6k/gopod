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

// GetPods returns list of pods.
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

// DeletePod removes a pod, optionally forcing removal.
func (c *Client) DeletePod(ctx context.Context, idOrName string, force bool) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/v5.0.0/libpod/pods/%s?force=%t", idOrName, force)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("podman delete pod failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman delete pod failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	args := []string{"pod", "rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, idOrName)
	return exec.CommandContext(ctx, "podman", args...).Run()
}
