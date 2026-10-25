package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// systemdUserBusPath resolves the host systemd user-manager bus socket.
// Inside a containerized engine the host bus is bind-mounted (see
// compose.dev.yml GOPOD_USER_BUS); on a host-native install the standard
// XDG runtime bus is used.
func systemdUserBusPath() string {
	if p := strings.TrimSpace(os.Getenv("GOPOD_USER_BUS")); p != "" {
		return p
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); xdg != "" {
		return filepath.Join(xdg, "bus")
	}
	return filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "bus")
}

var userBusMu sync.Mutex

// busProbe caches dialability briefly: a stalled bus must never serialize
// deployments (each systemctlUser call would otherwise pay full timeouts).
var busProbeMu sync.Mutex
var busProbeOK bool
var busProbeAt time.Time

const busProbeTTL = 30 * time.Second
const busDialTimeout = 3 * time.Second

// dialUserBus opens a fresh connection to the systemd user manager bus.
// The dial itself is timeout-guarded: D-Bus auth handshakes can stall
// (observed hanging Dial with zero output), and a stuck dial must never
// block a deployment.
func dialUserBus() (*dbus.Conn, error) {
	return dialUserBusTimeout(busDialTimeout)
}

func dialUserBusTimeout(d time.Duration) (*dbus.Conn, error) {
	path := systemdUserBusPath()
	type res struct {
		c   *dbus.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		// Connect (not Dial): performs Auth + Hello. A bare Dial leaves
		// the connection unauthenticated and every call hangs forever.
		conn, err := dbus.Connect(fmt.Sprintf("unix:path=%s", path))
		ch <- res{conn, err}
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-timer.C:
		// dialUserBusTimeout deliberately returns before the dial completes.
		// If the handshake finishes after we gave up, the caller has already
		// returned: without this reaper the fresh connection leaks an fd plus
		// the godbus receive goroutine on every bus stall over the deadline.
		go func() {
			if r := <-ch; r.c != nil {
				_ = r.c.Close()
			}
		}()
		return nil, fmt.Errorf("user bus dial timeout (%s)", path)
	}
}

// busReachable reports whether the user bus answers, with a short TTL cache.
func busReachable() bool {
	busProbeMu.Lock()
	if time.Since(busProbeAt) < busProbeTTL {
		ok := busProbeOK
		busProbeMu.Unlock()
		return ok
	}
	busProbeMu.Unlock()
	conn, err := dialUserBusTimeout(2 * time.Second)
	ok := err == nil
	if ok {
		_ = conn.Close()
	}
	busProbeMu.Lock()
	busProbeOK = ok
	busProbeAt = time.Now()
	busProbeMu.Unlock()
	return ok
}

const systemdManagerIFace = "org.freedesktop.systemd1.Manager"
const systemdManagerPath = "/org/freedesktop/systemd1"

func busManagerCall(ctx context.Context, method string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := dialUserBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	obj := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath(systemdManagerPath))
	call := obj.CallWithContext(ctx, systemdManagerIFace+"."+method, 0, args...)
	return call.Err
}

// busUnitState returns (loaded, activeState) for a unit via D-Bus.
func busUnitState(ctx context.Context, unit string) (loaded bool, st SystemdUnitState) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := dialUserBus()
	if err != nil {
		return false, st
	}
	defer conn.Close()
	unitPath := busGetUnitPath(ctx, conn, unit)
	if unitPath == "" {
		return false, st
	}
	unitObj := conn.Object("org.freedesktop.systemd1", unitPath)
	getProp := func(iface, prop string) string {
		v, err := unitObj.GetProperty(iface + "." + prop)
		if err != nil {
			return ""
		}
		if s, ok := v.Value().(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v.Value())
	}
	st.ID = unit
	st.LoadState = getProp("org.freedesktop.systemd1.Unit", "LoadState")
	st.ActiveState = getProp("org.freedesktop.systemd1.Unit", "ActiveState")
	st.SubState = getProp("org.freedesktop.systemd1.Unit", "SubState")
	st.Result = getProp("org.freedesktop.systemd1.Unit", "Result")
	if v, err := unitObj.GetProperty("org.freedesktop.systemd1.Service.NRestarts"); err == nil {
		switch n := v.Value().(type) {
		case uint32:
			st.NRestarts = int(n)
		case uint64:
			st.NRestarts = int(n)
		}
	}
	return st.LoadState == "loaded", st
}

func busGetUnitPath(ctx context.Context, conn *dbus.Conn, unit string) dbus.ObjectPath {
	obj := conn.Object("org.freedesktop.systemd1", dbus.ObjectPath(systemdManagerPath))
	var unitPath dbus.ObjectPath
	if err := obj.CallWithContext(ctx, systemdManagerIFace+".GetUnit", 0, unit).Store(&unitPath); err != nil {
		return ""
	}
	return unitPath
}

// querySystemdUnitsViaBus mirrors querySystemdUnits without the binary.
func querySystemdUnitsViaBus(ctx context.Context, unitNames ...string) map[string]SystemdUnitState {
	out := make(map[string]SystemdUnitState, len(unitNames))
	userBusMu.Lock()
	defer userBusMu.Unlock()
	for _, u := range unitNames {
		loaded, st := busUnitState(ctx, u)
		if loaded {
			out[u] = st
		}
	}
	return out
}

// hasUserManager reports whether host user-manager control is possible:
// native systemctl binary, or a reachable D-Bus user bus (containerized).
func (d *Deployer) hasUserManager() bool {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return true
	}
	return busReachable()
}

// systemctlUser runs a `systemctl --user ...` operation, translating to
// D-Bus manager calls when the binary is unavailable (containerized engine
// with the host user bus mounted). Returns combined output for commands
// that produce it (is-active).
func (d *Deployer) systemctlUser(ctx context.Context, args ...string) (string, error) {
	if _, err := exec.LookPath("systemctl"); err == nil {
		full := append([]string{"--user"}, args...)
		cmd := exec.CommandContext(ctx, "systemctl", full...)
		d.setupCmdEnv(cmd)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if !busReachable() {
		return "", fmt.Errorf("no systemd user manager reachable (no binary, bus down)")
	}
	if len(args) == 0 {
		return "", fmt.Errorf("systemctl: no arguments and no binary")
	}
	userBusMu.Lock()
	defer userBusMu.Unlock()
	switch args[0] {
	case "daemon-reload", "daemon-reexec":
		return "", busManagerCall(ctx, "Reload")
	case "reset-failed":
		if len(args) >= 2 {
			for _, u := range args[1:] {
				if err := busManagerCall(ctx, "ResetFailedUnit", u); err != nil {
					return "", err
				}
			}
			return "", nil
		}
		return "", busManagerCall(ctx, "ResetFailed")
	case "start":
		if len(args) < 2 {
			return "", fmt.Errorf("systemctl start: missing unit")
		}
		for _, u := range args[1:] {
			if err := busManagerCall(ctx, "StartUnit", u, "replace"); err != nil {
				return "", err
			}
		}
		return "", nil
	case "stop":
		if len(args) < 2 {
			return "", fmt.Errorf("systemctl stop: missing unit")
		}
		for _, u := range args[1:] {
			if err := busManagerCall(ctx, "StopUnit", u, "replace"); err != nil {
				return "", err
			}
		}
		return "", nil
	case "restart":
		if len(args) < 2 {
			return "", fmt.Errorf("systemctl restart: missing unit")
		}
		for _, u := range args[1:] {
			if err := busManagerCall(ctx, "RestartUnit", u, "replace"); err != nil {
				return "", err
			}
		}
		return "", nil
	case "kill":
		// systemctl kill --kill-who=all --signal=SIGKILL <unit>
		who, sig := "all", int32(9)
		var units []string
		for _, a := range args[1:] {
			if v, ok := strings.CutPrefix(a, "--kill-who="); ok {
				who = v
			} else if v, ok := strings.CutPrefix(a, "--signal="); ok {
				sig = parseSignal(v)
			} else if !strings.HasPrefix(a, "--") {
				units = append(units, a)
			}
		}
		for _, u := range units {
			if err := busManagerCall(ctx, "KillUnit", u, who, sig); err != nil {
				return "", err
			}
		}
		return "", nil
	case "is-active":
		if len(args) < 2 {
			return "", fmt.Errorf("systemctl is-active: missing unit")
		}
		loaded, st := busUnitState(ctx, args[1])
		if !loaded {
			return "inactive\n", fmt.Errorf("not loaded")
		}
		return st.ActiveState + "\n", nil
	case "disable":
		// No persistent enablement is used by GOPOD (WantedBy only takes
		// effect via daemon-reload); treat as no-op over D-Bus.
		return "", nil
	default:
		return "", fmt.Errorf("systemctl --user %s: unsupported without binary", args[0])
	}
}

func parseSignal(name string) int32 {
	n := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(name, "SIG")))
	switch n {
	case "HUP", "1":
		return 1
	case "INT", "2":
		return 2
	case "QUIT", "3":
		return 3
	case "KILL", "9":
		return 9
	case "TERM", "15":
		return 15
	}
	if v, err := strconv.Atoi(n); err == nil {
		return int32(v)
	}
	return 9
}

// isUnitActive reports whether the unit is active, via binary or D-Bus.
func (d *Deployer) isUnitActive(ctx context.Context, unit string) bool {
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", unit)
		d.setupCmdEnv(cmd)
		return cmd.Run() == nil
	}
	loaded, st := busUnitState(ctx, unit)
	return loaded && st.ActiveState == "active"
}
