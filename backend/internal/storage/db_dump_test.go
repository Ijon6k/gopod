package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
)

func TestDetectDatabaseEngine(t *testing.T) {
	cases := []struct {
		name     string
		image    string
		env      map[string]string
		expected string
	}{
		{
			name:     "postgres by image",
			image:    "docker.io/library/postgres:16-alpine",
			env:      map[string]string{},
			expected: "postgres",
		},
		{
			name:     "mariadb by image",
			image:    "docker.io/library/mariadb:11.4",
			env:      map[string]string{},
			expected: "mariadb",
		},
		{
			name:     "mysql by image",
			image:    "docker.io/library/mysql:8.0",
			env:      map[string]string{},
			expected: "mysql",
		},
		{
			name:     "redis by image",
			image:    "docker.io/library/redis:7-alpine",
			env:      map[string]string{},
			expected: "redis",
		},
		{
			name:     "valkey by image",
			image:    "docker.io/valkey/valkey:8",
			env:      map[string]string{},
			expected: "redis",
		},
		{
			name:     "mongo by image",
			image:    "docker.io/library/mongo:6",
			env:      map[string]string{},
			expected: "mongodb",
		},
		{
			name:     "postgres by env",
			image:    "custom-app:latest",
			env:      map[string]string{"POSTGRES_PASSWORD": "secret"},
			expected: "postgres",
		},
		{
			name:     "mysql by env",
			image:    "custom-app:latest",
			env:      map[string]string{"MYSQL_ROOT_PASSWORD": "secret"},
			expected: "mysql",
		},
		{
			name:     "custom fallback",
			image:    "custom-app:latest",
			env:      map[string]string{"APP_PORT": "3000"},
			expected: "custom",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectDatabaseEngine(tc.image, tc.env)
			if got != tc.expected {
				t.Fatalf("expected engine %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestRestoreSnapshotGuardsAndValidation(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "restore.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	svc := NewService(NewSQLiteRepository(database.DB))
	svc.SetSystemSource(database.DB, t.TempDir())
	ctx := context.Background()

	// Unsupported system archive restore stays rejected
	_ = svc.repo.CreateSnapshot(ctx, VolumeSnapshot{
		ID:         "sys-1",
		ProjectID:  "system",
		VolumeName: "system:gopod",
		Filename:   "sys.tar.gz",
	})
	if err := svc.RestoreSnapshot(ctx, "sys-1"); err != ErrSystemRestoreUnsupported {
		t.Fatalf("expected ErrSystemRestoreUnsupported, got %v", err)
	}

	// Database dump restore rejects missing running container / leading-dash credentials
	_ = svc.repo.CreateSnapshot(ctx, VolumeSnapshot{
		ID:         "db-snap-1",
		ProjectID:  "system",
		VolumeName: "db:postgres",
		Filename:   "db-postgres.sql.gz",
	})
	// Re-insert under non-system project to exercise DB validation branch
	_, _ = database.DB.Exec(`INSERT OR IGNORE INTO projects (id, name) VALUES ('proj-1', 'App')`)
	_ = svc.repo.CreateSnapshot(ctx, VolumeSnapshot{
		ID:         "db-snap-2",
		ProjectID:  "proj-1",
		ServiceID:  "",
		VolumeName: "db:postgres",
		Filename:   "db-postgres.sql.gz",
	})
	err = svc.RestoreSnapshotWithOptions(ctx, "db-snap-2", DatabaseRestoreRequest{
		Username: "-baduser",
	})
	if err == nil || (!strings.Contains(err.Error(), "snapshot file missing") && !strings.Contains(err.Error(), "serviceId")) {
		t.Fatalf("expected missing snapshot / validation error, got: %v", err)
	}
}
