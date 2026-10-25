package storage

import (
	"testing"
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
