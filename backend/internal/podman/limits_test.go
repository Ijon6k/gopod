package podman

import "testing"

func TestNormalizeMemorySwap(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		isErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"0", "", false},
		{"default", "", false},
		{"unlimited", "-1", false},
		{"-1", "-1", false},
		{"512MB", "512MB", false},
		{"512mb", "512mb", false},
		{"1gb", "1gb", false},
		{"256k", "256k", false},
		{"262144000", "262144000", false},
		{"1024b", "1024b", false},
		{"MB", "", true},
		{"512 KB", "512KB", false}, // interior space is canonicalized away
		{"garbage!", "", true},
		{"12x", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeMemorySwap(tc.in)
		if tc.isErr && err == nil {
			t.Errorf("NormalizeMemorySwap(%q) expected an error, got %q", tc.in, got)
			continue
		}
		if !tc.isErr && err != nil {
			t.Errorf("NormalizeMemorySwap(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if !tc.isErr && got != tc.want {
			t.Errorf("NormalizeMemorySwap(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
