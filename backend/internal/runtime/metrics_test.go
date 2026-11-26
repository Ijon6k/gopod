package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gopod/internal/db"
)

func TestMetricsStoreRecordQueryAndPrune(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "metrics.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	store := NewMetricsStore(database.DB)
	ctx := context.Background()

	now := time.Now().UTC()
	oldTs := now.Add(-8 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	recent1 := now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05")
	recent2 := now.Add(-1 * time.Minute).Format("2006-01-02 15:04:05")

	err = store.RecordSamples(ctx, []MetricSample{
		{
			Timestamp: oldTs, Scope: "host", TargetID: "host",
			CPUPercent: 99, MemUsedBytes: 1000, MemLimitBytes: 2000,
			StorageUsedGB: 10, StorageTotalGB: 100,
		},
		{
			Timestamp: recent1, Scope: "host", TargetID: "host",
			CPUPercent: 10, MemUsedBytes: 1000, MemLimitBytes: 4000,
			StorageUsedGB: 25, StorageTotalGB: 100, NetRxBps: 500, NetTxBps: 250,
		},
		{
			Timestamp: recent2, Scope: "host", TargetID: "host",
			CPUPercent: 20, MemUsedBytes: 2000, MemLimitBytes: 4000,
			StorageUsedGB: 26, StorageTotalGB: 100, NetRxBps: 1500, NetTxBps: 750,
		},
		{
			Timestamp: recent2, Scope: "service", TargetID: "svc-1",
			CPUPercent: 12.5, MemUsedBytes: 128 * 1024 * 1024, MemLimitBytes: 512 * 1024 * 1024,
			NetRxBps: 400, NetTxBps: 100,
		},
	})
	if err != nil {
		t.Fatalf("RecordSamples failed: %v", err)
	}

	pruned, err := store.PruneOlderThan(ctx, metricsRetention)
	if err != nil {
		t.Fatalf("PruneOlderThan failed: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1 old row", pruned)
	}

	host1h, err := store.QueryHistory(ctx, "host", "host", "1h")
	if err != nil {
		t.Fatalf("QueryHistory 1h failed: %v", err)
	}
	if len(host1h) != 2 {
		t.Fatalf("host 1h len = %d, want 2", len(host1h))
	}
	if host1h[0].CPUPercent != 10 || host1h[1].CPUPercent != 20 {
		t.Fatalf("unexpected cpu values: %+v", host1h)
	}

	// Downsampled range query (6h buckets by 120s)
	host6h, err := store.QueryHistory(ctx, "host", "host", "6h")
	if err != nil {
		t.Fatalf("QueryHistory 6h failed: %v", err)
	}
	if len(host6h) == 0 {
		t.Fatalf("host 6h expected non-empty downsampled series")
	}

	svc1h, err := store.QueryHistory(ctx, "service", "svc-1", "1h")
	if err != nil {
		t.Fatalf("QueryHistory service 1h failed: %v", err)
	}
	if len(svc1h) != 1 || svc1h[0].CPUPercent != 12.5 {
		t.Fatalf("unexpected service history: %+v", svc1h)
	}
}

func TestMetricsSamplerRateResetGuard(t *testing.T) {
	sampler := newMetricsSampler()
	t0 := time.Now().Add(-30 * time.Second)
	t1 := time.Now()
	t2 := t1.Add(30 * time.Second)

	rx, tx := sampler.computeRate("host", 1000, 500, t0)
	if rx != 0 || tx != 0 {
		t.Fatalf("first baseline sample must return 0 rate, got rx=%v tx=%v", rx, tx)
	}
	rx, tx = sampler.computeRate("host", 4000, 2000, t1)
	if rx <= 0 || tx <= 0 {
		t.Fatalf("second sample expected positive rate, got rx=%v tx=%v", rx, tx)
	}
	// Simulated container counter reset below previous totals -> clamped to 0, never negative
	rx, tx = sampler.computeRate("host", 200, 100, t2)
	if rx != 0 || tx != 0 {
		t.Fatalf("counter reset must clamp to 0, got rx=%v tx=%v", rx, tx)
	}
}

