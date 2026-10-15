package ingress

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTrafficFilteringAndMetrics(t *testing.T) {
	svc := &Service{}

	sampleLogs := []AccessLogItem{
		{
			ID:         "1",
			Timestamp:  time.Now().Format(time.RFC3339),
			ClientIP:   "192.168.1.1",
			Method:     "GET",
			Host:       "api.example.com",
			URI:        "/health",
			Status:     200,
			DurationMs: 12.5,
			ProjectID:  "proj-1",
			ServiceID:  "svc-1",
		},
		{
			ID:         "2",
			Timestamp:  time.Now().Format(time.RFC3339),
			ClientIP:   "192.168.1.2",
			Method:     "POST",
			Host:       "api.example.com",
			URI:        "/users",
			Status:     201,
			DurationMs: 25.0,
			ProjectID:  "proj-1",
			ServiceID:  "svc-1",
		},
		{
			ID:         "3",
			Timestamp:  time.Now().Format(time.RFC3339),
			ClientIP:   "192.168.1.3",
			Method:     "GET",
			Host:       "web.example.com",
			URI:        "/missing",
			Status:     404,
			DurationMs: 5.0,
			ProjectID:  "proj-2",
			ServiceID:  "svc-2",
		},
		{
			ID:         "4",
			Timestamp:  time.Now().Format(time.RFC3339),
			ClientIP:   "192.168.1.4",
			Method:     "POST",
			Host:       "api.example.com",
			URI:        "/crash",
			Status:     500,
			DurationMs: 50.0,
			ProjectID:  "proj-1",
			ServiceID:  "svc-1",
		},
	}

	// Filter by ProjectID
	filtered := svc.filterLogs(sampleLogs, TrafficFilter{ProjectID: "proj-1"})
	if len(filtered) != 3 {
		t.Fatalf("expected 3 items for proj-1, got %d", len(filtered))
	}

	// Filter by StatusGroup 2xx
	filtered2xx := svc.filterLogs(sampleLogs, TrafficFilter{StatusGroup: "2xx"})
	if len(filtered2xx) != 2 {
		t.Fatalf("expected 2 items for 2xx, got %d", len(filtered2xx))
	}

	// Filter by search
	filteredSearch := svc.filterLogs(sampleLogs, TrafficFilter{Search: "missing"})
	if len(filteredSearch) != 1 || filteredSearch[0].Status != 404 {
		t.Fatalf("expected 1 item for search 'missing', got %d", len(filteredSearch))
	}
}

func TestGenerateCaddyfile_PodUpstreamResolution(t *testing.T) {
	r := NewReconciler("http://127.0.0.1:2019", "")
	r.SetServiceResolver(func(ctx context.Context, idOrName string) (*ServiceMeta, error) {
		switch idOrName {
		case "cluster-demo", "self-hosted-cluster-demo-9af20c":
			return &ServiceMeta{
				ID:   "self-hosted-cluster-demo-9af20c",
				Name: "cluster-demo",
				Type: "pod",
			}, nil
		case "web-app", "self-hosted-web-app-d7b2bf":
			return &ServiceMeta{
				ID:    "self-hosted-web-app-d7b2bf",
				Name:  "web-app",
				Type:  "container",
				InPod: true,
				PodID: "pod_cluster-demo",
			}, nil
		case "standalone-app":
			return &ServiceMeta{
				ID:   "standalone-app",
				Name: "standalone-app",
				Type: "container",
			}, nil
		}
		return nil, nil
	})

	domains := []Domain{
		{
			ID:            "d1",
			Hostname:      "pod-direct.localhost",
			ServiceID:     "self-hosted-cluster-demo-9af20c",
			ContainerPort: 80,
		},
		{
			ID:              "d2",
			Hostname:        "pod-member.localhost",
			ServiceID:       "self-hosted-cluster-demo-9af20c",
			TargetContainer: "self-hosted-web-app-d7b2bf",
			ContainerPort:   8080,
		},
		{
			ID:            "d3",
			Hostname:      "member-direct.localhost",
			ServiceID:     "self-hosted-web-app-d7b2bf",
			ContainerPort: 80,
		},
		{
			ID:            "d4",
			Hostname:      "standalone.localhost",
			ServiceID:     "standalone-app",
			ContainerPort: 3000,
		},
	}

	caddyfile := r.GenerateCaddyfile(domains)

	// 1. Direct pod service should dial pod_cluster-demo:80
	if !strings.Contains(caddyfile, "reverse_proxy pod_cluster-demo:80") {
		t.Errorf("expected caddyfile to contain reverse_proxy pod_cluster-demo:80, got:\n%s", caddyfile)
	}

	// 2. Member container inside pod service should dial pod_cluster-demo:8080
	if !strings.Contains(caddyfile, "reverse_proxy pod_cluster-demo:8080") {
		t.Errorf("expected caddyfile to contain reverse_proxy pod_cluster-demo:8080, got:\n%s", caddyfile)
	}

	// 3. Member service direct domain route should dial pod_cluster-demo:80
	if !strings.Contains(caddyfile, "reverse_proxy pod_cluster-demo:80") {
		t.Errorf("expected caddyfile to contain reverse_proxy pod_cluster-demo:80, got:\n%s", caddyfile)
	}

	// 4. Standalone service should dial standalone-app:3000
	if !strings.Contains(caddyfile, "reverse_proxy standalone-app:3000") {
		t.Errorf("expected caddyfile to contain reverse_proxy standalone-app:3000, got:\n%s", caddyfile)
	}
}
