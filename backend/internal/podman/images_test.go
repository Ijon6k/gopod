package podman

import (
	"testing"
)

func TestSplitImageNameTag(t *testing.T) {
	tests := []struct {
		input    string
		wantRepo string
		wantTag  string
	}{
		{
			input:    "localhost:5000/my-app:v1.2",
			wantRepo: "localhost:5000/my-app",
			wantTag:  "v1.2",
		},
		{
			input:    "registry.example.com:8443/org/repo:latest",
			wantRepo: "registry.example.com:8443/org/repo",
			wantTag:  "latest",
		},
		{
			input:    "redis:alpine",
			wantRepo: "redis",
			wantTag:  "alpine",
		},
		{
			input:    "ubuntu",
			wantRepo: "ubuntu",
			wantTag:  "latest",
		},
		{
			input:    "localhost:5000/my-app",
			wantRepo: "localhost:5000/my-app",
			wantTag:  "latest",
		},
		{
			input:    "",
			wantRepo: "<none>",
			wantTag:  "<none>",
		},
	}

	for _, tt := range tests {
		gotRepo, gotTag := SplitImageNameTag(tt.input)
		if gotRepo != tt.wantRepo || gotTag != tt.wantTag {
			t.Errorf("SplitImageNameTag(%q) = (%q, %q); want (%q, %q)", tt.input, gotRepo, gotTag, tt.wantRepo, tt.wantTag)
		}
	}
}
