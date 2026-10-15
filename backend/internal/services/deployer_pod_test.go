package services

import (
	"reflect"
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

	t.Run("host IP prefix binding in compose yaml", func(t *testing.T) {
		s := Service{
			ComposeYaml: `version: "3.8"
services:
  web:
    ports:
      - "0.0.0.0:8088:80"
      - "127.0.0.1:3001:3000"
`,
		}
		got := extractPodPorts(s)
		// 0.0.0.0 is the default bind and omitted in canonical form;
		// 127.0.0.1 loopback bindings are preserved.
		expected := []string{"8088:80", "127.0.0.1:3001:3000"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})
}
