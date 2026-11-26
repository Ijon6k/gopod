package podman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

// HostPortBinding maps a container port to a host port binding.
type HostPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// InspectMount captures a container mount configuration.
type InspectMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

// ContainerFullInspect contains runtime configuration needed for self-inspection
// and zero-downtime replacement with identical mounts, env, ports, and networks.
type ContainerFullInspect struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Image       string `json:"Image"`
	ImageName   string `json:"ImageName"`
	ImageDigest string `json:"ImageDigest"`
	Pod         string `json:"Pod"`
	Config      struct {
		Hostname   string              `json:"Hostname"`
		User       string              `json:"User"`
		Env        []string            `json:"Env"`
		Cmd        []string            `json:"Cmd"`
		Entrypoint []string            `json:"Entrypoint"`
		WorkingDir string              `json:"WorkingDir"`
		Labels     map[string]string   `json:"Labels"`
		StopSignal string              `json:"StopSignal"`
	} `json:"Config"`
	HostConfig struct {
		Binds         []string                     `json:"Binds"`
		NetworkMode   string                       `json:"NetworkMode"`
		PortBindings  map[string][]HostPortBinding `json:"PortBindings"`
		SecurityOpt   []string                     `json:"SecurityOpt"`
		Privileged    bool                         `json:"Privileged"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	Mounts          []InspectMount `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]struct {
			Aliases []string `json:"Aliases"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// InspectContainerFull inspects container configuration over the Podman socket or CLI.
func (c *Client) InspectContainerFull(ctx context.Context, idOrName string) (*ContainerFullInspect, error) {
	if c == nil {
		return nil, errors.New("podman client not initialized")
	}
	clean := strings.TrimSpace(idOrName)
	if clean == "" || strings.HasPrefix(clean, "-") {
		return nil, errors.New("invalid container identifier")
	}

	parse := func(body []byte) (*ContainerFullInspect, error) {
		var single ContainerFullInspect
		if err := json.Unmarshal(body, &single); err == nil && (single.ID != "" || single.Name != "") {
			single.Name = strings.TrimPrefix(single.Name, "/")
			return &single, nil
		}
		var arr []ContainerFullInspect
		if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
			arr[0].Name = strings.TrimPrefix(arr[0].Name, "/")
			return &arr[0], nil
		}
		return nil, errors.New("unable to decode container inspect payload")
	}

	if c.httpClient != nil {
		reqURL := fmt.Sprintf("http://d/%s/libpod/containers/%s/json", c.libpodVersion(), url.PathEscape(clean))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, err
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("container %s: %w", clean, ErrNotFound)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return nil, fmt.Errorf("podman inspect HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			return nil, err
		}
		return parse(body)
	}

	cmd := exec.CommandContext(ctx, "podman", "inspect", "--format", "json", "--", clean)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("podman inspect failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parse(out)
}

// InspectImageRepoDigests returns local RepoDigests and primary Digest for an image ref.
func (c *Client) InspectImageRepoDigests(ctx context.Context, imageRef string) ([]string, string, error) {
	if c == nil || strings.TrimSpace(imageRef) == "" {
		return nil, "", errors.New("image reference required")
	}
	clean := strings.TrimSpace(imageRef)
	if strings.HasPrefix(clean, "-") {
		return nil, "", errors.New("invalid image reference")
	}

	type imgInspect struct {
		Digest      string   `json:"Digest"`
		RepoDigests []string `json:"RepoDigests"`
	}
	parse := func(body []byte) ([]string, string, error) {
		var single imgInspect
		if err := json.Unmarshal(body, &single); err == nil && (single.Digest != "" || len(single.RepoDigests) > 0) {
			return single.RepoDigests, single.Digest, nil
		}
		var arr []imgInspect
		if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
			return arr[0].RepoDigests, arr[0].Digest, nil
		}
		return nil, "", nil
	}

	if c.httpClient != nil {
		reqURL := fmt.Sprintf("http://d/%s/libpod/images/%s/json", c.libpodVersion(), url.PathEscape(clean))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, "", err
		}
		resp, cancel, err := c.do(ctx, readTimeout, req)
		if err != nil {
			return nil, "", err
		}
		defer cancel()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, "", nil
		}
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("image inspect HTTP %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return parse(body)
	}

	cmd := exec.CommandContext(ctx, "podman", "image", "inspect", "--format", "json", "--", clean)
	c.SetupCmdEnv(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", err
	}
	return parse(out)
}

// PullImage pulls a remote OCI image via the Podman socket or CLI.
func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	clean := strings.TrimSpace(imageRef)
	if clean == "" || strings.HasPrefix(clean, "-") || strings.ContainsAny(clean, " \t\r\n") {
		return fmt.Errorf("invalid image reference %q", imageRef)
	}
	if c != nil && c.httpClient != nil {
		reqURL := fmt.Sprintf("http://d/%s/libpod/images/pull?reference=%s", c.libpodVersion(), url.QueryEscape(clean))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
		if err != nil {
			return err
		}
		resp, cancel, err := c.do(ctx, pruneTimeout, req)
		if err != nil {
			return fmt.Errorf("podman pull request failed: %w", err)
		}
		defer cancel()
		defer resp.Body.Close()

		// Podman streams ndjson status lines ending with {"error": "..."} on pull failure even with HTTP 200.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("podman pull HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var ev struct {
				Error string `json:"error"`
			}
			if json.Unmarshal([]byte(line), &ev) == nil && strings.TrimSpace(ev.Error) != "" {
				return fmt.Errorf("podman pull failed: %s", ev.Error)
			}
		}
		return nil
	}

	cmd := exec.CommandContext(ctx, "podman", "pull", "--", clean)
	if c != nil {
		c.SetupCmdEnv(cmd)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman pull failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

