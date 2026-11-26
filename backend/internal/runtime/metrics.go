package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	metricsSampleInterval = 30 * time.Second
	metricsRetention      = 7 * 24 * time.Hour
)

// MetricSample represents a single telemetry observation (or downsampled bucket)
// for either the host node or an individual service.
type MetricSample struct {
	Timestamp      string  `json:"timestamp"`
	Label          string  `json:"label"`
	Scope          string  `json:"scope"`
	TargetID       string  `json:"targetId"`
	CPUPercent     float64 `json:"cpuPercent"`
	MemUsedBytes   int64   `json:"memUsedBytes"`
	MemLimitBytes  int64   `json:"memLimitBytes"`
	StorageUsedGB  float64 `json:"storageUsedGB"`
	StorageTotalGB float64 `json:"storageTotalGB"`
	NetRxBps       float64 `json:"netRxBps"`
	NetTxBps       float64 `json:"netTxBps"`
}

// MetricsStore persists periodic telemetry snapshots in SQLite and provides
// downsampled range queries for dashboard & per-service charts.
type MetricsStore struct {
	db *sql.DB
}

// NewMetricsStore wraps an initialized *sql.DB handle.
func NewMetricsStore(db *sql.DB) *MetricsStore {
	return &MetricsStore{db: db}
}

// RecordSamples inserts a batch of metric samples in a single SQLite transaction.
func (m *MetricsStore) RecordSamples(ctx context.Context, samples []MetricSample) error {
	if m == nil || m.db == nil || len(samples) == 0 {
		return nil
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metric_samples (
			ts, scope, target_id, cpu_pct, mem_used_bytes, mem_limit_bytes,
			storage_used_gb, storage_total_gb, net_rx_bps, net_tx_bps
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	nowStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	for _, s := range samples {
		ts := nowStr
		if s.Timestamp != "" {
			if parsed, perr := time.Parse(time.RFC3339, s.Timestamp); perr == nil {
				ts = parsed.UTC().Format("2006-01-02 15:04:05")
			} else {
				ts = s.Timestamp
			}
		}
		if _, err := stmt.ExecContext(
			ctx, ts, s.Scope, s.TargetID, s.CPUPercent,
			s.MemUsedBytes, s.MemLimitBytes, s.StorageUsedGB, s.StorageTotalGB,
			s.NetRxBps, s.NetTxBps,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// PruneOlderThan deletes metric samples older than age.
func (m *MetricsStore) PruneOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	if m == nil || m.db == nil {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-age).Format("2006-01-02 15:04:05")
	res, err := m.db.ExecContext(ctx, `DELETE FROM metric_samples WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func resolveHistoryWindow(rangeStr string) (lookback time.Duration, bucketSec int64) {
	switch strings.ToLower(strings.TrimSpace(rangeStr)) {
	case "6h":
		return 6 * time.Hour, 120
	case "24h":
		return 24 * time.Hour, 600
	case "7d":
		return 7 * 24 * time.Hour, 3600
	default:
		return 1 * time.Hour, 0
	}
}

// QueryHistory returns time-ordered telemetry points for scope+targetID over
// rangeStr ("1h", "6h", "24h", "7d"), downsampling longer windows cleanly.
func (m *MetricsStore) QueryHistory(ctx context.Context, scope, targetID, rangeStr string) ([]MetricSample, error) {
	if m == nil || m.db == nil {
		return []MetricSample{}, nil
	}
	lookback, bucketSec := resolveHistoryWindow(rangeStr)
	cutoff := time.Now().UTC().Add(-lookback).Format("2006-01-02 15:04:05")

	var rows *sql.Rows
	var err error
	if bucketSec <= 0 {
		rows, err = m.db.QueryContext(ctx, `
			SELECT ts, cpu_pct, mem_used_bytes, mem_limit_bytes, storage_used_gb, storage_total_gb, net_rx_bps, net_tx_bps
			FROM metric_samples
			WHERE scope = ? AND target_id = ? AND ts >= ?
			ORDER BY ts ASC
			LIMIT 360
		`, scope, targetID, cutoff)
	} else {
		rows, err = m.db.QueryContext(ctx, fmt.Sprintf(`
			SELECT
				datetime((CAST(strftime('%%s', ts) AS INTEGER) / %d) * %d, 'unixepoch') AS bucket_ts,
				AVG(cpu_pct),
				CAST(AVG(mem_used_bytes) AS INTEGER),
				CAST(MAX(mem_limit_bytes) AS INTEGER),
				AVG(storage_used_gb),
				MAX(storage_total_gb),
				AVG(net_rx_bps),
				AVG(net_tx_bps)
			FROM metric_samples
			WHERE scope = ? AND target_id = ? AND ts >= ?
			GROUP BY bucket_ts
			ORDER BY bucket_ts ASC
			LIMIT 360
		`, bucketSec, bucketSec), scope, targetID, cutoff)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricSample, 0)
	for rows.Next() {
		var tsRaw string
		var item MetricSample
		if err := rows.Scan(
			&tsRaw,
			&item.CPUPercent,
			&item.MemUsedBytes,
			&item.MemLimitBytes,
			&item.StorageUsedGB,
			&item.StorageTotalGB,
			&item.NetRxBps,
			&item.NetTxBps,
		); err != nil {
			return nil, err
		}
		item.Scope = scope
		item.TargetID = targetID
		if t, perr := time.Parse("2006-01-02 15:04:05", tsRaw); perr == nil {
			item.Timestamp = t.UTC().Format(time.RFC3339)
			item.Label = t.Local().Format("15:04")
			if rangeStr == "7d" {
				item.Label = t.Local().Format("01/02 15:04")
			}
		} else {
			item.Timestamp = tsRaw
			item.Label = tsRaw
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type netCounterSample struct {
	rx int64
	tx int64
	at time.Time
}

type metricsSampler struct {
	mu      sync.Mutex
	prevNet map[string]netCounterSample
}

func newMetricsSampler() *metricsSampler {
	return &metricsSampler{
		prevNet: make(map[string]netCounterSample),
	}
}

func (ms *metricsSampler) computeRate(key string, rxTotal, txTotal int64, now time.Time) (rxBps, txBps float64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	prev, ok := ms.prevNet[key]
	ms.prevNet[key] = netCounterSample{rx: rxTotal, tx: txTotal, at: now}
	if !ok {
		return 0, 0
	}
	dt := now.Sub(prev.at).Seconds()
	if dt <= 0.1 {
		return 0, 0
	}
	if rxTotal >= prev.rx {
		rxBps = float64(rxTotal-prev.rx) / dt
	}
	if txTotal >= prev.tx {
		txBps = float64(txTotal-prev.tx) / dt
	}
	return rxBps, txBps
}

// StartMetricsSampler runs a periodic 30s background collector storing host and
// per-service telemetry samples into SQLite with 7-day automatic retention.
func (s *Service) StartMetricsSampler(ctx context.Context) {
	if s.metrics == nil || s.client == nil {
		return
	}
	sampler := newMetricsSampler()
	_, _ = s.metrics.PruneOlderThan(ctx, metricsRetention)

	// Record an initial baseline sample right away (rates start at 0 until next tick).
	s.captureMetricTick(ctx, sampler)

	ticker := time.NewTicker(metricsSampleInterval)
	defer ticker.Stop()

	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticks++
			s.captureMetricTick(ctx, sampler)
			if ticks%60 == 0 {
				if _, err := s.metrics.PruneOlderThan(ctx, metricsRetention); err != nil {
					log.Printf("[metrics] prune error: %v", err)
				}
			}
		}
	}
}

func (s *Service) captureMetricTick(ctx context.Context, sampler *metricsSampler) {
	tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	var batch []MetricSample

	sysInfo, sysErr := s.GetSystemInfo(tickCtx)
	stats, statsErr := s.GetContainerStats(tickCtx)
	containers, _ := s.GetContainers(tickCtx)

	containerToService := make(map[string]string, len(containers)*2)
	for _, c := range containers {
		svcID := strings.TrimSpace(c.Labels["io.gopod.service"])
		if svcID == "" {
			continue
		}
		if c.ID != "" {
			containerToService[c.ID] = svcID
			if len(c.ID) > 12 {
				containerToService[c.ID[:12]] = svcID
			}
		}
		for _, n := range c.Names {
			clean := strings.TrimPrefix(n, "/")
			if clean != "" {
				containerToService[clean] = svcID
			}
		}
	}

	type svcAgg struct {
		cpu      float64
		memUsed  int64
		memLimit int64
		netRx    int64
		netTx    int64
	}
	svcMap := make(map[string]*svcAgg)
	var hostNetRxTotal, hostNetTxTotal int64

	if statsErr == nil {
		for _, st := range stats {
			hostNetRxTotal += st.NetRx
			hostNetTxTotal += st.NetTx

			targetSvc := containerToService[st.ID]
			if targetSvc == "" && len(st.ID) > 12 {
				targetSvc = containerToService[st.ID[:12]]
			}
			if targetSvc == "" {
				targetSvc = containerToService[strings.TrimPrefix(st.Name, "/")]
			}
			if targetSvc == "" {
				continue
			}
			agg, ok := svcMap[targetSvc]
			if !ok {
				agg = &svcAgg{}
				svcMap[targetSvc] = agg
			}
			agg.cpu += st.CPUPercent
			agg.memUsed += st.MemUsage
			agg.memLimit += st.MemLimit
			agg.netRx += st.NetRx
			agg.netTx += st.NetTx
		}
	}

	hostRxBps, hostTxBps := sampler.computeRate("host", hostNetRxTotal, hostNetTxTotal, now)
	if sysErr == nil && sysInfo != nil {
		batch = append(batch, MetricSample{
			Scope:          "host",
			TargetID:       "host",
			CPUPercent:     sysInfo.CPUUsage,
			MemUsedBytes:   sysInfo.MemoryUsed,
			MemLimitBytes:  sysInfo.MemoryTotal,
			StorageUsedGB:  sysInfo.StorageUsedGB,
			StorageTotalGB: sysInfo.StorageTotalGB,
			NetRxBps:       hostRxBps,
			NetTxBps:       hostTxBps,
		})
	}

	for svcID, agg := range svcMap {
		rxBps, txBps := sampler.computeRate("svc:"+svcID, agg.netRx, agg.netTx, now)
		sample := MetricSample{
			Scope:         "service",
			TargetID:      svcID,
			CPUPercent:    agg.cpu,
			MemUsedBytes:  agg.memUsed,
			MemLimitBytes: agg.memLimit,
			NetRxBps:      rxBps,
			NetTxBps:      txBps,
		}
		if sysInfo != nil {
			sample.StorageUsedGB = sysInfo.StorageUsedGB
			sample.StorageTotalGB = sysInfo.StorageTotalGB
		}
		batch = append(batch, sample)
	}

	if err := s.metrics.RecordSamples(tickCtx, batch); err != nil {
		log.Printf("[metrics] failed recording telemetry samples: %v", err)
	}
}

