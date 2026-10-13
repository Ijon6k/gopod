package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectGopodNetwork_NoExistingNetworks(t *testing.T) {
	input := `version: '3.8'
services:
  web:
    image: docker.io/library/nginx:alpine
    ports:
      - "80:80"
`

	output := injectGopodNetworkToCompose(input)

	// 1. Verify valid YAML
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\nOutput:\n%s", err, output)
	}

	// 2. Verify single networks key with default -> gopod-net
	networks, ok := doc["networks"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'networks' map in YAML, got: %+v", doc["networks"])
	}

	defaultNet, ok := networks["default"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'default' network map, got: %+v", networks["default"])
	}

	if defaultNet["name"] != "gopod-net" {
		t.Errorf("expected default.name == 'gopod-net', got %v", defaultNet["name"])
	}
	if defaultNet["external"] != true {
		t.Errorf("expected default.external == true, got %v", defaultNet["external"])
	}

	// 3. Verify exactly one root 'networks:' in raw string
	if countRootKey(output, "networks:") != 1 {
		t.Errorf("expected exactly 1 root 'networks:' key, got %d in:\n%s", countRootKey(output, "networks:"), output)
	}

	// 4. Test against podman-compose config CLI if available
	assertPodmanComposeConfigPasses(t, output)
}

func TestInjectGopodNetwork_UserCustomNetworks_SongMoodboardStack(t *testing.T) {
	// The user's exact song-moodboard stack that triggered the RuntimeError: missing networks: song-moodboard-net
	input := `version: '3.8'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    networks:
      - song-moodboard-net
    depends_on:
      - frontend
      - backend

  frontend:
    image: node:20-alpine
    networks:
      - song-moodboard-net

  backend:
    image: golang:1.22-alpine
    networks:
      - song-moodboard-net
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: songdb
    networks:
      - song-moodboard-net

  redis:
    image: redis:7-alpine
    networks:
      - song-moodboard-net

networks:
  song-moodboard-net:
    driver: bridge
`

	output := injectGopodNetworkToCompose(input)

	// 1. Verify valid YAML
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("output is not valid YAML: %v\nOutput:\n%s", err, output)
	}

	// 2. Verify networks mapping contains BOTH song-moodboard-net AND default
	networks, ok := doc["networks"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'networks' map, got %+v", doc["networks"])
	}

	// User custom network must NOT be lost!
	songNet, ok := networks["song-moodboard-net"].(map[string]any)
	if !ok {
		t.Fatalf("FATAL REGRESSION: user custom network 'song-moodboard-net' was wiped out! networks: %+v", networks)
	}
	if songNet["driver"] != "bridge" {
		t.Errorf("expected song-moodboard-net.driver == 'bridge', got %v", songNet["driver"])
	}

	// Default network must be present alongside song-moodboard-net
	defaultNet, ok := networks["default"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'default' network merged, got %+v", networks["default"])
	}
	if defaultNet["name"] != "gopod-net" || defaultNet["external"] != true {
		t.Errorf("unexpected default net config: %+v", defaultNet)
	}

	// 3. Root 'networks:' key must occur EXACTLY ONCE
	if countRootKey(output, "networks:") != 1 {
		t.Errorf("duplicate root 'networks:' detected! Count: %d\nOutput:\n%s", countRootKey(output, "networks:"), output)
	}

	// 4. Verify podman-compose config passes without RuntimeError
	assertPodmanComposeConfigPasses(t, output)
}

func TestInjectGopodNetwork_MultipleCustomNetworks(t *testing.T) {
	input := `version: '3.8'
services:
  api:
    image: api:v1
    networks:
      - frontend-net
      - backend-net
  db:
    image: postgres:16
    networks:
      - backend-net
  cache:
    image: redis:alpine
    networks:
      - cache-net

networks:
  frontend-net:
    driver: bridge
  backend-net:
    internal: true
  cache-net:
    driver: bridge
`

	output := injectGopodNetworkToCompose(input)

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	networks, ok := doc["networks"].(map[string]any)
	if !ok {
		t.Fatalf("expected networks map, got: %+v", doc["networks"])
	}

	// Ensure all 3 user networks are preserved
	for _, expectedNet := range []string{"frontend-net", "backend-net", "cache-net"} {
		if _, exists := networks[expectedNet]; !exists {
			t.Errorf("expected network %q to exist, but was missing", expectedNet)
		}
	}

	// Ensure default is added
	if _, exists := networks["default"]; !exists {
		t.Errorf("expected default network to be added")
	}

	// Ensure single root key
	if countRootKey(output, "networks:") != 1 {
		t.Errorf("expected exactly 1 root networks: key, got %d", countRootKey(output, "networks:"))
	}

	assertPodmanComposeConfigPasses(t, output)
}

func TestInjectGopodNetwork_AlreadyDeclaresDefaultNetwork(t *testing.T) {
	input := `version: '3.8'
services:
  web:
    image: nginx:alpine

networks:
  default:
    driver: overlay
    ipam:
      driver: default
`

	output := injectGopodNetworkToCompose(input)

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	networks := doc["networks"].(map[string]any)
	defaultNet := networks["default"].(map[string]any)

	// User defined their own default network driver, GOPOD MUST NOT overwrite it!
	if defaultNet["driver"] != "overlay" {
		t.Errorf("expected user-defined driver 'overlay' preserved, got: %v", defaultNet["driver"])
	}

	if countRootKey(output, "networks:") != 1 {
		t.Errorf("expected 1 root networks: key, got %d", countRootKey(output, "networks:"))
	}
}

func TestInjectGopodNetwork_AlreadyDeclaresGopodNet(t *testing.T) {
	input := `version: '3.8'
services:
  web:
    image: nginx:alpine
    networks:
      - gopod-net

networks:
  gopod-net:
    external: true
`

	output := injectGopodNetworkToCompose(input)

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	networks := doc["networks"].(map[string]any)

	// Should have gopod-net and not duplicate or force default
	if _, exists := networks["gopod-net"]; !exists {
		t.Errorf("expected gopod-net to be preserved")
	}

	if countRootKey(output, "networks:") != 1 {
		t.Errorf("expected 1 root networks: key, got %d", countRootKey(output, "networks:"))
	}

	assertPodmanComposeConfigPasses(t, output)
}

func TestInjectGopodNetwork_CustomNetworkWithNameGopodNet(t *testing.T) {
	input := `version: '3.8'
services:
  web:
    image: nginx:alpine

networks:
  ingress_bridge:
    name: gopod-net
    external: true
`

	output := injectGopodNetworkToCompose(input)

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	networks := doc["networks"].(map[string]any)
	if _, exists := networks["ingress_bridge"]; !exists {
		t.Errorf("expected ingress_bridge to exist")
	}
	// Should not inject redundant default since name: gopod-net is already bound
	if _, hasDefault := networks["default"]; hasDefault {
		t.Errorf("expected redundant default not to be injected when gopod-net name already exists")
	}
}

func TestInjectGopodNetwork_EmptyAndNullNetworksKey(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name: "empty networks key",
			input: `version: '3.8'
services:
  web:
    image: nginx:alpine
networks:
`,
		},
		{
			name: "null networks key",
			input: `version: '3.8'
services:
  web:
    image: nginx:alpine
networks: null
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := injectGopodNetworkToCompose(tc.input)

			var doc map[string]any
			if err := yaml.Unmarshal([]byte(output), &doc); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			networks, ok := doc["networks"].(map[string]any)
			if !ok {
				t.Fatalf("expected networks to become a valid map, got %+v", doc["networks"])
			}

			if _, ok := networks["default"]; !ok {
				t.Fatalf("expected default network in networks map: %+v", networks)
			}

			if countRootKey(output, "networks:") != 1 {
				t.Errorf("expected 1 root networks: key, got %d", countRootKey(output, "networks:"))
			}
		})
	}
}

func TestInjectGopodNetwork_PreservesCommentsAndFormatting(t *testing.T) {
	input := `# Top level stack documentation
version: '3.8'

# Services definition
services:
  # Web ingress reverse proxy
  web:
    image: nginx:alpine
    ports:
      - "80:80" # Standard HTTP port
`

	output := injectGopodNetworkToCompose(input)

	if !strings.Contains(output, "# Top level stack documentation") {
		t.Errorf("header comment was stripped: %s", output)
	}
	if !strings.Contains(output, "# Services definition") {
		t.Errorf("services comment was stripped: %s", output)
	}
	if !strings.Contains(output, "# Web ingress reverse proxy") {
		t.Errorf("web comment was stripped: %s", output)
	}

	assertPodmanComposeConfigPasses(t, output)
}

func TestInjectGopodNetwork_EdgeCasesAndMalformedYAML(t *testing.T) {
	edgeCases := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "whitespace only", input: "   \n\t   \n  "},
		{name: "invalid syntax", input: ":::malformed:yaml:::"},
		{name: "yaml scalar", input: "hello-world"},
		{name: "yaml list", input: "- item 1\n- item 2"},
	}

	for _, ec := range edgeCases {
		t.Run(ec.name, func(t *testing.T) {
			// Must never panic
			res := injectGopodNetworkToCompose(ec.input)
			if ec.input == "" && res != "" {
				t.Errorf("expected empty string for empty input, got %q", res)
			}
		})
	}
}

func TestInjectGopodNetwork_FullSongStackLiveConfig(t *testing.T) {
	fullSongYaml := `version: '3.8'

services:
  nginx:
    image: docker.io/library/nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - nginx-data:/etc/nginx/conf.d:ro
    networks:
      - song-moodboard-net
    depends_on:
      - frontend
      - backend

  frontend:
    image: docker.io/library/node:20-alpine
    environment:
      - NODE_ENV=production
    networks:
      - song-moodboard-net

  backend:
    image: docker.io/library/golang:1.22-alpine
    environment:
      - PORT=8080
      - DB_HOST=postgres
      - REDIS_HOST=redis
    networks:
      - song-moodboard-net
    depends_on:
      - postgres
      - redis

  postgres:
    image: docker.io/library/postgres:16-alpine
    environment:
      - POSTGRES_DB=songmoodboard
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=secret
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - song-moodboard-net

  redis:
    image: docker.io/library/redis:7-alpine
    volumes:
      - redis-data:/data
    networks:
      - song-moodboard-net

  seaweedfs:
    image: docker.io/chrislusf/seaweedfs:latest
    command: "server -dir=/data"
    volumes:
      - seaweedfs-data:/data
    networks:
      - song-moodboard-net

volumes:
  nginx-data:
  postgres-data:
  redis-data:
  seaweedfs-data:

networks:
  song-moodboard-net:
    driver: bridge
`

	processed := injectGopodNetworkToCompose(fullSongYaml)

	// Validate with podman-compose config
	assertPodmanComposeConfigPasses(t, processed)

	// Verify all 6 services and 4 volumes are preserved in YAML AST
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(processed), &doc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	services := doc["services"].(map[string]any)
	for _, expectedSvc := range []string{"nginx", "frontend", "backend", "postgres", "redis", "seaweedfs"} {
		if _, ok := services[expectedSvc]; !ok {
			t.Errorf("expected service %s in services", expectedSvc)
		}
	}

	volumes := doc["volumes"].(map[string]any)
	for _, expectedVol := range []string{"nginx-data", "postgres-data", "redis-data", "seaweedfs-data"} {
		if _, ok := volumes[expectedVol]; !ok {
			t.Errorf("expected volume %s in volumes", expectedVol)
		}
	}

	networks := doc["networks"].(map[string]any)
	if _, ok := networks["song-moodboard-net"]; !ok {
		t.Errorf("expected song-moodboard-net in networks")
	}
	if _, ok := networks["default"]; !ok {
		t.Errorf("expected default in networks")
	}
}

// Helpers

func countRootKey(content, key string) int {
	lines := strings.Split(content, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \r\t")
		if trimmed == key || strings.HasPrefix(trimmed, key+" ") {
			// Must be at root (zero indentation)
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				count++
			}
		}
	}
	return count
}

func assertPodmanComposeConfigPasses(t *testing.T, yamlContent string) {
	t.Helper()
	if _, err := exec.LookPath("podman-compose"); err != nil {
		t.Skip("podman-compose not installed on host, skipping CLI validation")
		return
	}

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write tmp compose file: %v", err)
	}

	cmd := exec.Command("podman-compose", "-f", composePath, "config")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("podman-compose config FAILED with error: %v\nOutput:\n%s\nYAML:\n%s", err, string(out), yamlContent)
	}
}
