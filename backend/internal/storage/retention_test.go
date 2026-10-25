package storage

import (
	"context"
	"testing"
)

// Regression test for the BE-1 retention defect: deletion ran from the head
// of a created_at-DESC list, so scheduled retention silently destroyed the
// NEWEST snapshots and kept the oldest. The contract is: keep the newest
// RetentionCount snapshots, delete the oldest excess.
func TestPruneSnapshotsByRetentionKeepsNewest(t *testing.T) {
	db, cleanup := setupTestStorageDB(t)
	defer cleanup()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo)
	ctx := context.Background()

	// Seed 4 snapshots for one volume with strictly increasing created_at
	// values (CURRENT_TIMESTAMP would tie within the same second, making the
	// DESC ordering non-deterministic for the test).
	rows := []struct{ id, createdAt string }{
		{"snap-oldest", "2026-01-01 00:00:00"},
		{"snap-old", "2026-01-02 00:00:00"},
		{"snap-new", "2026-01-03 00:00:00"},
		{"snap-newest", "2026-01-04 00:00:00"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO volume_snapshots (id, project_id, volume_name, filename, created_at) VALUES (?, 'proj-1', 'app_data', ?, ?)`,
			r.id, r.id+".tar.gz", r.createdAt); err != nil {
			t.Fatalf("seed snapshot %s: %v", r.id, err)
		}
	}

	snaps, err := svc.ListSnapshots(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	if len(snaps) != 4 {
		t.Fatalf("expected 4 snapshots, got %d", len(snaps))
	}
	// Lock the list contract the pruner relies on: newest first.
	if snaps[0].ID != "snap-newest" {
		t.Fatalf("ListSnapshots must be newest-first; got head %q", snaps[0].ID)
	}

	// Retention of 2 on a just-pruned 4: the two OLDEST must be gone and the
	// two newest (including the snapshot the schedule just created) must
	// survive.
	svc.pruneSnapshotsByRetention(ctx, snaps, 2)

	left, err := svc.ListSnapshots(ctx, "proj-1")
	if err != nil {
		t.Fatalf("re-ListSnapshots: %v", err)
	}
	if len(left) != 2 {
		t.Fatalf("expected 2 survivors, got %d", len(left))
	}
	kept := map[string]bool{}
	for _, snap := range left {
		kept[snap.ID] = true
	}
	if !kept["snap-new"] || !kept["snap-newest"] {
		t.Fatalf("retention kept %v; expected exactly the newest snapshots", kept)
	}
}
