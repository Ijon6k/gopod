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

// GetNetworks returns list of container networks.
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
