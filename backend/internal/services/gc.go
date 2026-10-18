package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopod/internal/activity"
	"gopod/internal/podman"
)

// The cleanup engine reclaims the artifacts gopod itself accumulates over time:
// dangling image layers from image builds, stopped containers, build cache, and
// quadlet unit files left behind by deleted services. It is deliberately
// conservative — named volumes are NEVER removed automatically (they hold user
// data), and unlabeled runtime objects are never touched because ownership is
// proven by the io.gopod.* labels.

// GCScope selects which artifact classes a cleanup run may touch.
type GCScope string

const (
	GCScopeImages     GCScope = "images"
	GCScopeContainers GCScope = "containers"
	GCScopeBuilds     GCScope = "builds"
	GCScopeUnits      GCScope = "units"
	GCScopeAll        GCScope = "all"
)

// ParseGCScope validates a requested scope, defaulting an empty value to "all".
func ParseGCScope(raw string) (GCScope, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "all":
		return GCScopeAll, nil
	case "images":
		return GCScopeImages, nil
	case "containers":
		return GCScopeContainers, nil
	case "builds":
		return GCScopeBuilds, nil
	case "units":
		return GCScopeUnits, nil
	default:
		return "", fmt.Errorf("invalid gc scope %q (expected images, containers, builds, units, or all)", raw)
	}
}

func (s GCScope) includes(part GCScope) bool {
	return s == GCScopeAll || s == part
}

// gcRuntime is the subset of Podman engine operations the cleanup engine needs.
// *podman.Client satisfies it; tests substitute fakes so the destructive paths
// can be exercised without touching real storage.
type gcRuntime interface {
	GetContainers(ctx context.Context) ([]podman.ContainerItem, error)
	GetVolumes(ctx context.Context) ([]podman.VolumeItem, error)
	GetDiskUsage(ctx context.Context) ([]podman.SystemDiskUsage, error)
	GetDanglingImages(ctx context.Context) (*podman.DanglingImageStat, error)
	PruneImages(ctx context.Context, all bool) error
	PruneContainers(ctx context.Context) error
	PruneBuilds(ctx context.Context) error
	DeleteVolume(ctx context.Context, name string, force bool) error
}

// GCPreview is a read-only inventory of what a cleanup run would reclaim.
type GCPreview struct {
	Busy                bool     `json:"busy"`
	DanglingImages      int      `json:"danglingImages"`
	DanglingImageBytes  int64    `json:"danglingImageBytes"`
	StoppedContainers   int      `json:"stoppedContainers"`
	WeightedReclaimable int64    `json:"reclaimableBytes"`
	StaleUnitFiles      []string `json:"staleUnitFiles"`
	OrphanVolumes       []string `json:"orphanVolumes"`
}

// GCResult reports what a cleanup run actually did.
type GCResult struct {
	Scope            string   `json:"scope"`
	Skipped          bool     `json:"skipped"`
	Reason           string   `json:"reason,omitempty"`
	DanglingImages   int      `json:"danglingImagesRemoved"`
	ReclaimedBytes   int64    `json:"reclaimedBytes"`
	UnitFilesRemoved []string `json:"unitFilesRemoved,omitempty"`
	VolumesRemoved   []string `json:"volumesRemoved,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// gcClient returns the runtime used by the cleanup engine, or nil when no
// podman client is configured.
func (s *WorkloadService) gcClient() gcRuntime {
	if s.gcRuntimeOverride != nil {
		return s.gcRuntimeOverride
	}
	if s.deployer == nil || s.deployer.podmanClient == nil {
		return nil
	}
	return s.deployer.podmanClient
}

// gcBusy reports whether a deployment is currently in flight. Cleanup must
// never race a deploy that is creating or replacing containers.
func (s *WorkloadService) gcBusy() bool {
	busy := false
	s.deployLocks.Range(func(_, _ any) bool {
		busy = true
		return false
	})
	if busy {
		return true
	}
	s.activeServiceDeps.Range(func(_, _ any) bool {
		busy = true
		return false
	})
	return busy
}

// knownServiceIDs returns the set of service IDs currently present in the DB.
func (s *WorkloadService) knownServiceIDs(ctx context.Context) (map[string]bool, error) {
	if s.repo == nil {
		return map[string]bool{}, nil
	}
	services, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(services))
	for _, svc := range services {
		known[svc.ID] = true
	}
	return known, nil
}

// gcQuadletDirs returns the directories that may hold gopod-owned quadlet unit
// files: the deployer's staging directory and the active systemd quadlet dir.
func (s *WorkloadService) gcQuadletDirs() []string {
	var dirs []string
	if s.deployer != nil && s.deployer.dataDir != "" {
		dirs = append(dirs, filepath.Join(s.deployer.dataDir, "quadlets"))
	}
	systemdDir := strings.TrimSpace(os.Getenv("QUADLET_DIR"))
	if systemdDir == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			systemdDir = filepath.Join(home, ".config", "containers", "systemd")
		}
	}
	if systemdDir != "" {
		dirs = append(dirs, systemdDir)
	}
	return dirs
}

// serviceLabelRe extracts the value of a gopod ownership label from unit text.
var serviceLabelRe = regexp.MustCompile(`io\.gopod\.service=([^\s"']+)`)

// findStaleUnitFiles returns quadlet unit files whose ownership label names a
// service whose DB row does not exist. Files without a gopod label are
// never considered: they were not written by gopod, so gopod leaves them alone.
func (s *WorkloadService) findStaleUnitFiles(known map[string]bool) []string {
	seen := make(map[string]bool)
	var stale []string
	for _, dir := range s.gcQuadletDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			switch ext {
			case ".container", ".pod", ".volume", ".network":
			default:
				continue
			}
			full := filepath.Join(dir, entry.Name())
			if seen[full] {
				continue
			}
			data, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			match := serviceLabelRe.FindStringSubmatch(string(data))
			if len(match) != 2 {
				continue
			}
			if known[match[1]] {
				continue
			}
			seen[full] = true
			stale = append(stale, full)
		}
	}
	return stale
}

// findOrphanVolumes returns gopod-labeled volumes whose owning service is gone.
func findOrphanVolumes(volumes []podman.VolumeItem, known map[string]bool) []string {
	var orphans []string
	for _, v := range volumes {
		if v.Labels == nil {
			continue
		}
		id := v.Labels["io.gopod.service"]
		if id != "" && !known[id] {
			orphans = append(orphans, v.Name)
		}
	}
	return orphans
}

// gcReclaimableBytes sums the reclaimable bytes Podman reports for the artifact
// classes the cleanup engine can prune (images and containers; never volumes).
func gcReclaimableBytes(df []podman.SystemDiskUsage) int64 {
	var total int64
	for _, entry := range df {
		t := strings.ToLower(entry.Type)
		if strings.Contains(t, "image") || strings.Contains(t, "container") {
			total += entry.RawReclaimable
		}
	}
	return total
}

// GCPreview returns a read-only inventory of reclaimable artifacts.
func (s *WorkloadService) GCPreview(ctx context.Context) (*GCPreview, error) {
	preview := &GCPreview{Busy: s.gcBusy()}

	known, err := s.knownServiceIDs(ctx)
	if err != nil {
		return nil, err
	}
	preview.StaleUnitFiles = s.findStaleUnitFiles(known)

	client := s.gcClient()
	if client == nil {
		return preview, nil
	}
	if dangling, err := client.GetDanglingImages(ctx); err == nil && dangling != nil {
		preview.DanglingImages = dangling.Count
		preview.DanglingImageBytes = dangling.Bytes
	}
	if containers, err := client.GetContainers(ctx); err == nil {
		for _, c := range containers {
			if !containerIsRunning(c) {
				preview.StoppedContainers++
			}
		}
	}
	if volumes, err := client.GetVolumes(ctx); err == nil {
		preview.OrphanVolumes = findOrphanVolumes(volumes, known)
	}
	if df, err := client.GetDiskUsage(ctx); err == nil {
		preview.WeightedReclaimable = gcReclaimableBytes(df)
	}
	return preview, nil
}

// containerIsRunning reports whether a container listing entry is currently
// running. Podman surfaces both a coarse Status string and a State field.
func containerIsRunning(c podman.ContainerItem) bool {
	if strings.TrimSpace(c.State) != "" {
		return strings.EqualFold(strings.TrimSpace(c.State), "running")
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Status)), "up")
}

// RunGC executes one cleanup pass for the requested scope. It is a no-op when a
// deployment is in flight: racing a deploy would delete the very layers it is
// creating. Named volumes are never removed unless explicitly requested AND
// provably orphaned.
func (s *WorkloadService) RunGC(ctx context.Context, scope GCScope, includeVolumes bool, volumeNames []string) (*GCResult, error) {
	result := &GCResult{Scope: string(scope)}

	if s.gcBusy() {
		result.Skipped = true
		result.Reason = "deployment in progress"
		emitEngineEventSafe(ctx, "info", "system", "", "", "engine", "gc.skipped_busy",
			"Cleanup skipped: a deployment is in progress", map[string]any{"scope": string(scope)})
		return result, nil
	}

	client := s.gcClient()
	if client == nil {
		result.Errors = append(result.Errors, "podman runtime unavailable")
		return result, nil
	}

	var before int64
	if df, err := client.GetDiskUsage(ctx); err == nil {
		before = gcReclaimableBytes(df)
	}

	if scope.includes(GCScopeImages) {
		if dangling, err := client.GetDanglingImages(ctx); err == nil && dangling != nil {
			result.DanglingImages = dangling.Count
		}
		if err := client.PruneImages(ctx, false); err != nil {
			result.Errors = append(result.Errors, "image prune: "+err.Error())
		}
	}
	if scope.includes(GCScopeBuilds) {
		if err := client.PruneBuilds(ctx); err != nil {
			result.Errors = append(result.Errors, "build prune: "+err.Error())
		}
	}
	if scope.includes(GCScopeContainers) {
		if err := client.PruneContainers(ctx); err != nil {
			result.Errors = append(result.Errors, "container prune: "+err.Error())
		}
	}
	if scope.includes(GCScopeUnits) {
		known, err := s.knownServiceIDs(ctx)
		if err != nil {
			result.Errors = append(result.Errors, "unit scan: "+err.Error())
		} else {
			stale := s.findStaleUnitFiles(known)
			for _, path := range stale {
				if err := os.Remove(path); err == nil {
					result.UnitFilesRemoved = append(result.UnitFilesRemoved, path)
				} else if !os.IsNotExist(err) {
					result.Errors = append(result.Errors, "unit remove "+path+": "+err.Error())
				}
			}
			if len(result.UnitFilesRemoved) > 0 && s.deployer != nil && s.deployer.hasUserManager() {
				_, _ = s.deployer.systemctlUser(ctx, "daemon-reload")
			}
		}
	}
	if includeVolumes && len(volumeNames) > 0 {
		known, err := s.knownServiceIDs(ctx)
		if err != nil {
			result.Errors = append(result.Errors, "volume scan: "+err.Error())
		} else if volumes, err := client.GetVolumes(ctx); err == nil {
			wanted := make(map[string]bool, len(volumeNames))
			for _, n := range volumeNames {
				wanted[strings.TrimSpace(n)] = true
			}
			for _, v := range volumes {
				if !wanted[v.Name] {
					continue
				}
				// Only provably orphaned, gopod-labeled volumes are removable.
				if v.Labels == nil || v.Labels["io.gopod.service"] == "" || known[v.Labels["io.gopod.service"]] {
					continue
				}
				if err := client.DeleteVolume(ctx, v.Name, true); err != nil {
					result.Errors = append(result.Errors, "volume remove "+v.Name+": "+err.Error())
				} else {
					result.VolumesRemoved = append(result.VolumesRemoved, v.Name)
				}
			}
		}
	}

	if df, err := client.GetDiskUsage(ctx); err == nil {
		after := gcReclaimableBytes(df)
		if before > after {
			result.ReclaimedBytes = before - after
		}
	}

	detail := fmt.Sprintf("Cleanup scope=%s reclaimed %d byte(s); dangling images=%d, unit files=%d, volumes=%d",
		scope, result.ReclaimedBytes, result.DanglingImages, len(result.UnitFilesRemoved), len(result.VolumesRemoved))
	emitEngineEventSafe(ctx, "info", "system", "", "", "engine", "gc.run", detail, map[string]any{
		"scope":          string(scope),
		"reclaimedBytes": result.ReclaimedBytes,
		"danglingImages": result.DanglingImages,
		"unitFiles":      len(result.UnitFilesRemoved),
		"volumes":        len(result.VolumesRemoved),
		"errors":         result.Errors,
	})
	activity.RecordActivity(ctx, "engine", "system.gc", "system", "", "", "", "success", detail, "")
	return result, nil
}

// gcSchedule reads the local nightly cleanup time (HH:MM). Defaults to 23:50.
func gcSchedule() (hour, minute int) {
	hour, minute = 23, 50
	raw := strings.TrimSpace(os.Getenv("GOPOD_GC_AT"))
	if raw == "" {
		return hour, minute
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return hour, minute
	}
	h, hErr := atoiInRange(parts[0], 0, 23)
	m, mErr := atoiInRange(parts[1], 0, 59)
	if hErr != nil || mErr != nil {
		return hour, minute
	}
	return h, m
}

func atoiInRange(raw string, lo, hi int) (int, error) {
	v := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &v); err != nil {
		return 0, err
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("%d out of range", v)
	}
	return v, nil
}

// gcSchedulerScope is the scope the nightly job runs. Volumes are never
// included. Defaults to "all".
func gcSchedulerScope() GCScope {
	raw := strings.TrimSpace(os.Getenv("GOPOD_GC_SCOPE"))
	if raw == "" {
		return GCScopeAll
	}
	scope, err := ParseGCScope(raw)
	if err != nil {
		return GCScopeAll
	}
	return scope
}

// StartGCScheduler launches the single nightly cleanup goroutine. It is a no-op
// when GOPOD_GC_DISABLE=1. The goroutine exits with ctx.
func (s *WorkloadService) StartGCScheduler(ctx context.Context) {
	if os.Getenv("GOPOD_GC_DISABLE") == "1" {
		return
	}
	hour, minute := gcSchedule()
	scope := gcSchedulerScope()
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if _, err := s.RunGC(runCtx, scope, false, nil); err != nil {
				emitEngineEventSafe(runCtx, "error", "system", "", "", "engine", "gc.error",
					"Nightly cleanup failed: "+err.Error(), nil)
			}
			cancel()
		}
	}()
}
