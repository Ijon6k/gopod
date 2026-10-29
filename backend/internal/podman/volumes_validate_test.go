package podman

import "testing"

func TestValidateRunVolumeSpec(t *testing.T) {
	valid := []string{
		"mydata:/data",
		"mydata:/data:ro",
		"/srv/app/uploads:/app/uploads:Z",
		"/srv/db:/app/db",
		"./data:/app/data",
		"../shared:/app/shared:ro",
		"/var/lib/other/pgdata:/var/lib/postgresql/data:Z,rslave",
	}
	for _, in := range valid {
		if err := ValidateRunVolumeSpec(in); err != nil {
			t.Errorf("ValidateRunVolumeSpec(%q) unexpectedly rejected: %v", in, err)
		}
	}

	invalid := []string{
		// Reserved virtual filesystems.
		"/proc:/host/proc",
		"/proc/kcore:/x",
		"/sys:/host/sys",
		"/sys/kernel:/x:ro",
		"/dev:/tmp/dev",
		"/dev/input:/x",
		"/boot:/host/boot",
		"/run/host:/x",
		"/var/run:/x",
		// Container runtime sockets (any override path, any suffix).
		"/run/user/1000/podman/podman.sock:/run/podman/podman.sock",
		"/tmp/XDG/podman/podman.sock:/run/podman/podman.sock",
		"/var/run/docker.sock:/var/run/docker.sock",
		"/tmp/storage/docker.sock:/c/docker.sock",
		"/run/containerd/containerd.sock:/x",
		"/some/dir/containerd-containerd.sock:/x",
		"/tmp/probe.podman.sock:/x",
		// Root bind.
		"/:/host",
		// Malformed.
		"",
		"   ",
		"/only-source",
		// Newline injection.
		"/tmp/nul\n/var/run:/x",
		// /etc and /root protection.
		"/etc:/app/etc",
		"/etc/shadow:/app/shadow:ro",
		"/root:/app/root",
		"~/.ssh:/app/ssh",
		// Colon injection in source.
		"/srv/app:evil:/app/data",
	}
	for _, in := range invalid {
		if err := ValidateRunVolumeSpec(in); err == nil {
			t.Errorf("ValidateRunVolumeSpec(%q) unexpectedly accepted", in)
		}
	}
}

func TestValidateVolumeSource(t *testing.T) {
	// Prefix bypass attempts must be caught (e.g. /procX is a different path
	// and stays allowed; /runX too).
	if err := ValidateVolumeSource("/procinfo/data"); err != nil {
		t.Errorf("adjacent-name path /procinfo unexpectedly rejected: %v", err)
	}
	if err := ValidateVolumeSource("/rundeck/jobs"); err != nil {
		t.Errorf("adjacent-name path /rundeck unexpectedly rejected: %v", err)
	}
	// Exception allowed case (a regular user directory beside /run).
	if err := ValidateVolumeSource("/var/www/html"); err != nil {
		t.Errorf("ordinary path /var/www/html unexpectedly rejected: %v", err)
	}
}
