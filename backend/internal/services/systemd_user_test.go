package services

import (
	"context"
	"testing"
	"time"
)

func TestParseSignal(t *testing.T) {
	if parseSignal("SIGKILL") != 9 || parseSignal("KILL") != 9 || parseSignal("9") != 9 {
		t.Errorf("SIGKILL should map to 9")
	}
	if parseSignal("SIGTERM") != 15 {
		t.Errorf("SIGTERM should map to 15")
	}
}

func TestBusUnitStateMissingUnit(t *testing.T) {
	// Must never claim a bogus unit is loaded, with or without a bus.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loaded, _ := busUnitState(ctx, "gopod-definitely-missing-12345.service")
	if loaded {
		t.Errorf("missing unit reported as loaded")
	}
}

func TestQuadletSupervisionDescribe(t *testing.T) {
	d := NewDeployer(nil, nil, nil, t.TempDir())
	unit, file, active := d.DescribeQuadletSupervision(Service{
		ID:        "proj-qweb-abc123",
		Name:      "qweb",
		ProjectID: "proj",
		Type:      "quadlet",
		Image:     "docker.io/traefik/whoami:v1.10.1",
		Port:      80,
		HostPort:  8089,
	})
	if unit == "" {
		t.Errorf("expected a primary unit name")
	}
	if file == "" {
		t.Errorf("expected a unit file path")
	}
	if active {
		t.Errorf("fresh unit must not report active")
	}
}
