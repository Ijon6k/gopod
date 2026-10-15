package services

import "testing"

func TestPodNameFor(t *testing.T) {
	cases := map[string]string{
		"my-app":        "pod_my-app",
		"pod_my-app":    "pod_my-app",
		"My App!":       "pod_my-app",
		"pod_e2e-sync":  "pod_e2e-sync",
		"E2E_Cluster.D": "pod_e2e_cluster-d",
	}
	for in, want := range cases {
		if got := PodNameFor(in); got != want {
			t.Errorf("PodNameFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPodNameForService(t *testing.T) {
	a := PodNameForService(Service{ID: "self-hosted-weblab-02c149", Name: "weblab", ProjectID: "self-hosted"})
	b := PodNameForService(Service{ID: "other-weblab-9f8e7d", Name: "weblab", ProjectID: "other"})
	if a == b {
		t.Errorf("same pod name for different projects: %q", a)
	}
	if a != "pod_self-hosted-weblab-02c149" {
		t.Errorf("unexpected canonical name: %q", a)
	}
	// Deterministic across deploys (stable DNS + Quadlet unit).
	if again := PodNameForService(Service{ID: "self-hosted-weblab-02c149", Name: "weblab"}); again != a {
		t.Errorf("unstable canonical name: %q vs %q", a, again)
	}
	// Fallback without ID.
	if got := PodNameForService(Service{Name: "WebLab"}); got != "pod_weblab" {
		t.Errorf("fallback = %q, want pod_weblab", got)
	}
}

func TestNormalizePortBinding(t *testing.T) {
	cases := map[string]string{
		"8091:80":                  "0.0.0.0:8091:80",
		"8091:80/tcp":              "0.0.0.0:8091:80",
		"8091:80/UDP":              "0.0.0.0:8091:80/udp",
		"0.0.0.0:8091:80":          "0.0.0.0:8091:80",
		"127.0.0.1:6380:6379":      "127.0.0.1:6380:6379",
		"127.0.0.1:6380:6379/tcp":  "127.0.0.1:6380:6379",
		"6380:6379/udp":            "0.0.0.0:6380:6379/udp",
		"127.0.0.1:18402:5230/udp": "127.0.0.1:18402:5230/udp",
		"18401:80":                 "0.0.0.0:18401:80",
		"18401:80/udp":             "0.0.0.0:18401:80/udp",
		"bad":                      "",
		"0:80":                     "",
	}
	for in, want := range cases {
		if got := normalizePortBinding(in); got != want {
			t.Errorf("normalizePortBinding(%q) = %q, want %q", in, got, want)
		}
	}
}
