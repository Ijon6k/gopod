package services

import (
	"testing"

	"gopod/internal/podman"
)

func TestValidateVolumeMount(t *testing.T) {
	valid := [][2]string{
		{"mydata", "/app/data"},
		{"/srv/uploads", "/app/uploads"},
		{"./local", "/app/local"},
	}
	for _, pair := range valid {
		if err := ValidateVolumeMount(pair[0], pair[1]); err != nil {
			t.Errorf("ValidateVolumeMount(%q, %q) unexpectedly rejected: %v", pair[0], pair[1], err)
		}
	}

	invalid := [][2]string{
		{"", "/app/x"},
		{"/srv/x", ""},
		{"/proc", "/host/proc"},
		{"/run/user/1000/podman/podman.sock", "/run/podman/podman.sock"},
		{"/", "/host"},
		{"/srv/x\n", "/app/x"},
		{"/etc", "/app/etc"},
		{"/root", "/app/root"},
		{"~/.ssh", "/app/ssh"},
		{"/srv/data:extra", "/app/data"},
		{"/srv/data", "/app/data:extra"},
		{"/srv/data", "relative/target"},
	}
	for _, pair := range invalid {
		if err := ValidateVolumeMount(pair[0], pair[1]); err == nil {
			t.Errorf("ValidateVolumeMount(%q, %q) unexpectedly accepted", pair[0], pair[1])
		}
	}
}

func TestVolumePolicyDelegation(t *testing.T) {
	// Keep the services wrapper and the engine choke point in lockstep: a
	// source rejected by one must be rejected by the other too.
	if err := ValidateVolumeMount("/run/monitor.sock", "/tmp/monitor.sock"); err == nil {
		t.Fatal("services wrapper accepted a runtime socket mount")
	}
	if err := podman.ValidateRunVolumeSpec("/run/monitor.sock:/tmp/monitor.sock"); err == nil {
		t.Fatal("engine choke point accepted a runtime socket mount")
	}
}
