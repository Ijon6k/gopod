package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"gopod/internal/storage"
	"gopod/pkg/crypto"
)

func execCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func main() {
	fmt.Println("==================================================")
	fmt.Println("🚀 RUNNING GOPOD LIVE BACKUP & RESTORE E2E TESTS")
	fmt.Println("==================================================")

	ctx := context.Background()

	// 1. Setup isolated test DB
	tmpDir, err := os.MkdirTemp("", "gopod-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "e2e.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_ = crypto.InitMasterKey(tmpDir)

	schema := `
	CREATE TABLE volume_snapshots (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		filename TEXT NOT NULL,
		size TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0,
		status TEXT DEFAULT 'completed',
		compression TEXT DEFAULT 'gzip',
		remote_provider TEXT DEFAULT '',
		remote_destination_id TEXT DEFAULT '',
		remote_key TEXT DEFAULT '',
		checksum TEXT DEFAULT '',
		remote_status TEXT DEFAULT '',
		remote_error TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE volume_schedules (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		service_id TEXT DEFAULT '',
		volume_name TEXT NOT NULL,
		cron TEXT NOT NULL,
		label TEXT DEFAULT '',
		retention_count INTEGER DEFAULT 7,
		enabled BOOLEAN DEFAULT 1,
		last_run DATETIME,
		destination_id TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE backup_destinations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		region TEXT DEFAULT 'us-east-1',
		access_key TEXT DEFAULT '',
		secret_enc TEXT DEFAULT '',
		bucket TEXT NOT NULL,
		prefix TEXT DEFAULT '',
		path_style INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		panic(err)
	}

	repo := storage.NewSQLiteRepository(db)
	svc := storage.NewService(repo)

	// Clean any previous test containers/volumes
	_, _ = execCmd("podman", "rm", "-f", "test_e2e_app", "test_e2e_pg")
	_, _ = execCmd("podman", "volume", "rm", "-f", "test_e2e_app_vol", "test_e2e_pg_vol")

	// ----------------------------------------------------
	// SCENARIO 1: Simple App Volume Backup & Restore
	// ----------------------------------------------------
	fmt.Println("\n▶ [Scenario 1] Simple App Volume Backup & Restore...")

	// 1a. Create volume & populate data
	_, _ = execCmd("podman", "volume", "create", "test_e2e_app_vol")
	out, err := execCmd("podman", "run", "--rm", "-v", "test_e2e_app_vol:/data", "docker.io/library/alpine:latest",
		"sh", "-c", "echo 'INITIAL_APP_DATA_12345' > /data/app.state && cat /data/app.state")
	if err != nil {
		fmt.Printf("❌ Failed to populate volume: %v (%s)\n", err, out)
		os.Exit(1)
	}
	fmt.Printf("  • Seeded data in volume: %s\n", out)

	// 1b. Take snapshot
	snap, err := svc.CreateSnapshot(ctx, storage.VolumeSnapshot{
		ProjectID:  "proj-e2e",
		VolumeName: "test_e2e_app_vol",
	})
	if err != nil {
		fmt.Printf("❌ CreateSnapshot failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Snapshot created: %s (%s, %s)\n", snap.ID, snap.Filename, snap.Size)

	// 1c. Mutate/corrupt data
	_, err = execCmd("podman", "run", "--rm", "-v", "test_e2e_app_vol:/data", "docker.io/library/alpine:latest",
		"sh", "-c", "echo 'CORRUPTED_OVERWRITTEN_DATA' > /data/app.state")
	if err != nil {
		fmt.Printf("❌ Failed to corrupt data: %v\n", err)
		os.Exit(1)
	}
	readCorrupted, _ := execCmd("podman", "run", "--rm", "-v", "test_e2e_app_vol:/data", "docker.io/library/alpine:latest", "cat", "/data/app.state")
	fmt.Printf("  • Data mutated to: %s\n", readCorrupted)

	// 1d. Restore snapshot
	err = svc.RestoreSnapshot(ctx, snap.ID)
	if err != nil {
		fmt.Printf("❌ RestoreSnapshot failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  • RestoreSnapshot completed successfully.")

	// 1e. Verify restored data
	restoredData, err := execCmd("podman", "run", "--rm", "-v", "test_e2e_app_vol:/data", "docker.io/library/alpine:latest", "cat", "/data/app.state")
	if err != nil || restoredData != "INITIAL_APP_DATA_12345" {
		fmt.Printf("❌ Verification failed! Got '%s', expected 'INITIAL_APP_DATA_12345'\n", restoredData)
		os.Exit(1)
	}
	fmt.Printf("  ✅ Verified restored data: %s (MATCH!)\n", restoredData)

	// ----------------------------------------------------
	// SCENARIO 2: Database (Postgres) Live Dump & Volume Snapshot
	// ----------------------------------------------------
	fmt.Println("\n▶ [Scenario 2] Database (PostgreSQL) Live Dump & Snapshot...")

	// 2a. Run postgres container with mounted volume
	_, _ = execCmd("podman", "volume", "create", "test_e2e_pg_vol")
	pgRunOut, err := execCmd("podman", "run", "-d",
		"--name", "test_e2e_pg",
		"--label", "io.gopod.service=svc-pg-test",
		"-v", "test_e2e_pg_vol:/var/lib/postgresql/data",
		"-e", "POSTGRES_PASSWORD=e2epassword",
		"-e", "POSTGRES_DB=e2edb",
		"docker.io/library/postgres:16-alpine",
	)
	if err != nil {
		fmt.Printf("❌ Failed to start postgres: %v (%s)\n", err, pgRunOut)
		os.Exit(1)
	}
	fmt.Println("  • PostgreSQL container started. Waiting for ready...")

	// Wait for postgres ready
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		checkOut, checkErr := execCmd("podman", "exec", "test_e2e_pg", "pg_isready", "-U", "postgres", "-d", "e2edb")
		if checkErr == nil && strings.Contains(checkOut, "accepting connections") {
			ready = true
			break
		}
	}
	if !ready {
		fmt.Println("❌ PostgreSQL did not become ready in 30 seconds.")
		os.Exit(1)
	}
	fmt.Println("  • PostgreSQL is ready accepting connections.")

	// 2b. Insert critical records
	seedSQL := "CREATE TABLE customers (id SERIAL PRIMARY KEY, name TEXT, balance NUMERIC); INSERT INTO customers (name, balance) VALUES ('Alice', 100.50), ('Bob', 250.75);"
	_, err = execCmd("podman", "exec", "-i", "test_e2e_pg", "psql", "-U", "postgres", "-d", "e2edb", "-c", seedSQL)
	if err != nil {
		fmt.Printf("❌ Failed to seed database: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  • Inserted test table 'customers' with 2 rows (Alice, Bob).")

	// 2c. Run native Database Dump (PR 3 engine)
	dumpSnap, err := svc.CreateDatabaseDump(ctx, storage.DatabaseDumpRequest{
		ProjectID: "proj-e2e",
		ServiceID: "svc-pg-test",
		Engine:    "postgres",
		Database:  "e2edb",
		Username:  "postgres",
		Password:  "e2epassword",
	})
	if err != nil {
		fmt.Printf("❌ CreateDatabaseDump failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Database Dump archive generated: %s (%s, %s)\n", dumpSnap.ID, dumpSnap.Filename, dumpSnap.Size)

	// 2d. Run volume snapshot on the database volume
	pgVolSnap, err := svc.CreateSnapshot(ctx, storage.VolumeSnapshot{
		ProjectID:  "proj-e2e",
		ServiceID:  "svc-pg-test",
		VolumeName: "test_e2e_pg_vol",
	})
	if err != nil {
		fmt.Printf("❌ Volume Snapshot on pg failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Database volume snapshot created: %s (%s, %s)\n", pgVolSnap.ID, pgVolSnap.Filename, pgVolSnap.Size)

	// 2e. Destroy/corrupt data inside PostgreSQL
	_, err = execCmd("podman", "exec", "-i", "test_e2e_pg", "psql", "-U", "postgres", "-d", "e2edb", "-c", "DROP TABLE customers;")
	if err != nil {
		fmt.Printf("❌ Failed to drop table: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  • Table 'customers' DROPPED (simulating accidental data loss).")

	// 2f. Restore from native logical dump
	dumpFilePath := filepath.Join("./data/backups", dumpSnap.Filename)
	restoreDumpCmd := exec.Command("sh", "-c", fmt.Sprintf("gzip -dc %s | podman exec -i -e PGPASSWORD=e2epassword test_e2e_pg psql -U postgres -d e2edb", dumpFilePath))
	restoreOut, err := restoreDumpCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Restoring from dump archive failed: %v (%s)\n", err, string(restoreOut))
		os.Exit(1)
	}
	fmt.Println("  • Database restored from logical dump archive successfully.")

	// 2g. Verify table and rows are fully recovered
	verifyOut, err := execCmd("podman", "exec", "-i", "test_e2e_pg", "psql", "-U", "postgres", "-d", "e2edb", "-t", "-c", "SELECT count(*) FROM customers;")
	if err != nil || strings.TrimSpace(verifyOut) != "2" {
		fmt.Printf("❌ Row count mismatch! Got '%s', expected '2'\n", verifyOut)
		os.Exit(1)
	}
	fmt.Printf("  ✅ Verified table recovered: %s rows present in 'customers' table! (MATCH!)\n", strings.TrimSpace(verifyOut))

	// ----------------------------------------------------
	// SCENARIO 3: Multi-container Compose / Service Stack
	// ----------------------------------------------------
	fmt.Println("\n▶ [Scenario 3] Compose / Multi-Container Stack Backup & Restore...")

	// 3a. Start compose-like stack: Redis with named volume
	_, _ = execCmd("podman", "rm", "-f", "test_e2e_compose_redis")
	_, _ = execCmd("podman", "volume", "rm", "-f", "test_e2e_compose_redis_vol")
	_, _ = execCmd("podman", "volume", "create", "test_e2e_compose_redis_vol")

	redisRunOut, err := execCmd("podman", "run", "-d",
		"--name", "test_e2e_compose_redis",
		"--label", "io.gopod.service=svc-compose-redis",
		"--label", "io.gopod.project=proj-compose",
		"-v", "test_e2e_compose_redis_vol:/data",
		"docker.io/library/redis:7-alpine",
		"redis-server", "--appendonly", "yes",
	)
	if err != nil {
		fmt.Printf("❌ Failed to start redis stack: %v (%s)\n", err, redisRunOut)
		os.Exit(1)
	}
	time.Sleep(2 * time.Second)

	// 3b. Write state across keys
	_, err = execCmd("podman", "exec", "test_e2e_compose_redis", "redis-cli", "SET", "gopod:cluster:status", "HEALTHY_OPTIMAL")
	if err != nil {
		fmt.Printf("❌ Failed to set redis key: %v\n", err)
		os.Exit(1)
	}
	_, _ = execCmd("podman", "exec", "test_e2e_compose_redis", "redis-cli", "BGSAVE")
	time.Sleep(1 * time.Second)
	fmt.Println("  • Seeded Redis key 'gopod:cluster:status' = 'HEALTHY_OPTIMAL'")

	// 3c. Backup volume of the compose stack
	redisSnap, err := svc.CreateSnapshot(ctx, storage.VolumeSnapshot{
		ProjectID:  "proj-compose",
		ServiceID:  "svc-compose-redis",
		VolumeName: "test_e2e_compose_redis_vol",
	})
	if err != nil {
		fmt.Printf("❌ Failed to snapshot redis volume: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Compose Redis volume snapshot created: %s (%s, %s)\n", redisSnap.ID, redisSnap.Filename, redisSnap.Size)

	// 3d. Stop and corrupt redis key
	_, _ = execCmd("podman", "exec", "test_e2e_compose_redis", "redis-cli", "SET", "gopod:cluster:status", "COMPROMISED")
	corruptVal, _ := execCmd("podman", "exec", "test_e2e_compose_redis", "redis-cli", "GET", "gopod:cluster:status")
	fmt.Printf("  • Mutated key value to: %s\n", corruptVal)

	// Stop container before volume restore
	_, _ = execCmd("podman", "stop", "test_e2e_compose_redis")

	// 3e. Restore volume snapshot
	err = svc.RestoreSnapshot(ctx, redisSnap.ID)
	if err != nil {
		fmt.Printf("❌ Failed to restore redis volume snapshot: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  • Volume snapshot restored over test_e2e_compose_redis_vol.")

	// 3f. Restart container and verify key value
	_, _ = execCmd("podman", "start", "test_e2e_compose_redis")
	time.Sleep(2 * time.Second)

	recoveredVal, err := execCmd("podman", "exec", "test_e2e_compose_redis", "redis-cli", "GET", "gopod:cluster:status")
	if err != nil || recoveredVal != "HEALTHY_OPTIMAL" {
		fmt.Printf("❌ Redis recovered value mismatch! Got '%s', expected 'HEALTHY_OPTIMAL'\n", recoveredVal)
		os.Exit(1)
	}
	fmt.Printf("  ✅ Verified compose stack restored: 'gopod:cluster:status' = %s (MATCH!)\n", recoveredVal)

	// ----------------------------------------------------
	// SCENARIO 4: GoPod Instance Disaster Recovery (Self-Backup)
	// ----------------------------------------------------
	fmt.Println("\n▶ [Scenario 4] GoPod Instance Self-Backup & Key Verification...")

	svc.SetSystemSource(db, tmpDir)

	// 4a. Trigger instance backup
	sysSnap, err := svc.CreateSystemBackup(ctx, "")
	if err != nil {
		fmt.Printf("❌ CreateSystemBackup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Instance disaster recovery archive generated: %s (%s, %s)\n", sysSnap.ID, sysSnap.Filename, sysSnap.Size)

	// 4b. Verify Key Status
	keyStatus := svc.GetKeyStatus()
	if !keyStatus.KeyExists || keyStatus.Fingerprint == "" {
		fmt.Printf("❌ Key status verification failed: %+v\n", keyStatus)
		os.Exit(1)
	}
	fmt.Printf("  • Master Encryption Key Verified (Algorithm: %s, Fingerprint: sha256:%s)\n", keyStatus.Algorithm, keyStatus.Fingerprint)

	// 4c. Verify archive file on disk
	sysArchivePath, _, err := svc.GetSnapshotFilePath(ctx, sysSnap.ID)
	if err != nil {
		fmt.Printf("❌ Archive file not found via service: %v\n", err)
		os.Exit(1)
	}
	fi, err := os.Stat(sysArchivePath)
	if err != nil || fi.Size() == 0 {
		fmt.Printf("❌ Archive file check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Archive verified on disk (%d bytes).\n", fi.Size())

	// 4d. Test System Backup Schedule Policy
	sched, err := svc.SaveSystemBackupSchedule(ctx, "0 3 * * *", 14, "", true)
	if err != nil {
		fmt.Printf("❌ SaveSystemBackupSchedule failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  • Saved system backup schedule: %s (Retention: %d copies, Enabled: %v)\n", sched.Cron, sched.RetentionCount, sched.Enabled)

	fmt.Println("  ✅ Verified GoPod self-backup engine & master key integrity! (MATCH!)")

	// Cleanup test resources
	fmt.Println("\n🧹 Cleaning up test containers & volumes...")
	_, _ = execCmd("podman", "rm", "-f", "test_e2e_app", "test_e2e_pg", "test_e2e_compose_redis")
	_, _ = execCmd("podman", "volume", "rm", "-f", "test_e2e_app_vol", "test_e2e_pg_vol", "test_e2e_compose_redis_vol")
	_ = os.Remove(filepath.Join("./data/backups", snap.Filename))
	_ = os.Remove(filepath.Join("./data/backups", dumpSnap.Filename))
	_ = os.Remove(filepath.Join("./data/backups", pgVolSnap.Filename))
	_ = os.Remove(filepath.Join("./data/backups", redisSnap.Filename))
	_ = os.Remove(sysArchivePath)

	fmt.Println("\n==================================================")
	fmt.Println("🎉 ALL E2E SCENARIOS PASSED WITH LIVE CONTAINERS!")
	fmt.Println("==================================================")
}
