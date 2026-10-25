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
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/"+c.libpodVersion()+"/libpod/networks/json", nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, fmt.Errorf("podman networks request failed: %w", err)
		}
		defer cancel()
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

// EnsureNetwork verifies network exists, creates bridge network if absent.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	if name == "" {
		name = "gopod-net"
	}
	nets, err := c.GetNetworks(ctx)
	if err == nil {
		for _, n := range nets {
			if n.Name == name {
				return nil
			}
		}
	}

	cmd := exec.CommandContext(ctx, "podman", "network", "create", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create podman network %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveNetwork removes a user-defined network. Podman refuses networks with
// attached containers, so the default call surface gets the engine's own
// in-use refusal with its real reason.
func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	if c.httpClient != nil {
		url := fmt.Sprintf("http://d/%s/libpod/networks/%s", c.libpodVersion(), name)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, writeTimeout, req)
		if err != nil {
			return fmt.Errorf("podman remove network failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()
		// 404 is expected when the snapshot the page holds is already stale
		// (the network was removed elsewhere); removal stays idempotent.
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("podman remove network failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// `--` before the name: a client-supplied name like `--all` must be
	// treated as a network name (and then fail), never as a podman flag.
	cmd := exec.CommandContext(ctx, "podman", "network", "rm", "--", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "no such network") {
			return nil
		}
		return fmt.Errorf("failed to remove podman network %s: %s (%w)", name, strings.TrimSpace(string(out)), err)
	}
	return nil
}
