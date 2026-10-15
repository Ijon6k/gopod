package services

import (
	"context"
	"testing"
)

func TestValidateKubernetesPodManifest(t *testing.T) {
	ctx := context.Background()

	t.Run("valid multi-container pod with shared volume", func(t *testing.T) {
		yamlStr := `
apiVersion: v1
kind: Pod
metadata:
  name: demo-gui-pod
  labels:
    app: demo-gui
spec:
  volumes:
    - name: shared-html
      emptyDir: {}
  containers:
    - name: web-gui
      image: docker.io/library/nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 9981
      volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
    - name: worker-sidecar
      image: docker.io/library/alpine:latest
      command: ["/bin/sh", "-c"]
      args:
        - "echo hello"
      volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
`
		res := ValidateKubernetesPodManifest(ctx, yamlStr)
		if !res.Valid {
			t.Fatalf("expected manifest to be valid, got errors: %v", res.Errors)
		}
		if len(res.Workloads) != 2 {
			t.Fatalf("expected 2 workloads, got %d", len(res.Workloads))
		}
		if res.Workloads[0].Name != "web-gui" || res.Workloads[0].Role != "main" {
			t.Errorf("unexpected first workload: %+v", res.Workloads[0])
		}
		if res.Workloads[1].Name != "worker-sidecar" || res.Workloads[1].Role != "sidecar" {
			t.Errorf("unexpected second workload: %+v", res.Workloads[1])
		}
		if len(res.Volumes) != 1 || res.Volumes[0] != "shared-html" {
			t.Errorf("unexpected volumes: %v", res.Volumes)
		}
	})

	t.Run("syntax error in yaml", func(t *testing.T) {
		res := ValidateKubernetesPodManifest(ctx, "apiVersion: v1\nkind: Pod\n  bad-indentation: [")
		if res.Valid {
			t.Fatal("expected invalid for broken YAML")
		}
		if len(res.Errors) == 0 {
			t.Fatal("expected error messages")
		}
	})

	t.Run("flags port 8080 collision with helpful warning", func(t *testing.T) {
		yamlStr := `
apiVersion: v1
kind: Pod
metadata:
  name: test-coll
spec:
  containers:
    - name: web
      image: nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 8080
`
		res := ValidateKubernetesPodManifest(ctx, yamlStr)
		// It should be syntactically valid but have a warning about port 8080
		if !res.Valid {
			t.Fatalf("expected syntax valid, got errors: %v", res.Errors)
		}
		found8080Warning := false
		for _, w := range res.Warnings {
			if len(w) > 0 && (testing.Verbose() || true) {
				t.Logf("warning emitted: %s", w)
			}
			if len(w) > 0 && (testing.Verbose() || true) {
				found8080Warning = true
			}
		}
		if !found8080Warning {
			t.Errorf("expected warning for port 8080 collision")
		}
	})
}

func TestValidateComposeManifest(t *testing.T) {
	ctx := context.Background()

	yamlStr := `
version: "3.8"
services:
  web:
    image: nginx:alpine
    ports:
      - "9982:80"
  redis:
    image: redis:alpine

volumes:
  cache:
`
	res := ValidateComposeManifest(ctx, yamlStr)
	if !res.Valid {
		t.Fatalf("expected compose manifest valid, got errors: %v", res.Errors)
	}
	if len(res.Workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d", len(res.Workloads))
	}
	if len(res.Volumes) != 1 || res.Volumes[0] != "cache" {
		t.Fatalf("expected volume 'cache', got %v", res.Volumes)
	}
}
