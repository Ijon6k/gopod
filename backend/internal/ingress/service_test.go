package ingress

import (
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
