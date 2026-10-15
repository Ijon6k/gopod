package services

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractPodPorts(t *testing.T) {
	t.Run("primary port only", func(t *testing.T) {
		s := Service{
			Port:     80,
			HostPort: 8080,
		}
		got := extractPodPorts(s)
		expected := []string{"8080:80"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("host port defaults to container port if zero", func(t *testing.T) {
		s := Service{
			Port:     3000,
			HostPort: 0,
		}
		got := extractPodPorts(s)
		expected := []string{"3000:3000"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("multiple ports from compose yaml", func(t *testing.T) {
		s := Service{
			Port:     80,
			HostPort: 8090,
			ComposeYaml: `version: "3.8"
services:
  cluster-demo:
    ports:
      - "8090:80"
      - "6379:6379"
      - "9000:9000/udp"
`,
		}
		got := extractPodPorts(s)
		expected := []string{"8090:80", "6379:6379", "9000:9000/udp"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("deduplicates redundant port declarations", func(t *testing.T) {
		s := Service{
			Port:     80,
			HostPort: 8090,
			ComposeYaml: `version: "3.8"
services:
  cluster-demo:
    ports:
      - "8090:80"
      - "8090:80"
`,
		}
		got := extractPodPorts(s)
		expected := []string{"8090:80"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("no ports configured", func(t *testing.T) {
		s := Service{
			Port:        0,
			HostPort:    0,
			ComposeYaml: "",
		}
		got := extractPodPorts(s)
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})

	t.Run("multiple ports from kubernetes pod manifest with udp and hostPort defaults", func(t *testing.T) {
		s := Service{
			K8sYaml: `apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
spec:
  containers:
    - name: web
      image: nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 8091
          protocol: TCP
        - containerPort: 53
          hostPort: 5353
          protocol: UDP
    - name: cache
      image: redis:alpine
      ports:
        - containerPort: 6379
`,
		}
		got := extractPodPorts(s)
		expected := []string{"8091:80", "5353:53/udp", "6379:6379"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("prepareK8sPodManifest injects podName and tracking labels", func(t *testing.T) {
		raw := `apiVersion: v1
kind: Pod
metadata:
  name: demo-gui-pod
spec:
  containers:
    - name: web-gui
      image: nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 8088
`
		if !hasK8sWorkloadContainers(raw) {
			t.Fatal("expected hasK8sWorkloadContainers to be true")
		}
		prepared, count, err := prepareK8sPodManifest(raw, "pod_my-svc-123", "proj-a", "my-svc-123", []string{"8088:80"})
		if err != nil {
			t.Fatalf("prepareK8sPodManifest failed: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 container, got %d", count)
		}
		if !strings.Contains(prepared, "pod_my-svc-123") {
			t.Errorf("expected pod name in prepared manifest, got: %s", prepared)
		}
		if !strings.Contains(prepared, "io.gopod.project: proj-a") {
			t.Errorf("expected project label, got: %s", prepared)
		}
		if !strings.Contains(prepared, "io.gopod.service: my-svc-123") {
			t.Errorf("expected service label, got: %s", prepared)
		}
	})
}
