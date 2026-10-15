package services

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Contract: canonical backend port string is `[IP:]host:container[/proto]`
// (IP in {0.0.0.0, 127.0.0.1}, absent = 0.0.0.0); drift key is
// `{ip}:{host}:{container}{/udp if udp}` with empty IP -> 0.0.0.0.

func TestCanonicalPortBinding(t *testing.T) {
	cases := map[string]string{
		"18401:80":                 "18401:80",
		"18401:80/udp":             "18401:80/udp",
		"18401:80/tcp":             "18401:80",
		"0.0.0.0:18401:80":         "18401:80",
		"127.0.0.1:18402:5230":     "127.0.0.1:18402:5230",
		"127.0.0.1:18402:5230/udp": "127.0.0.1:18402:5230/udp",
		"127.0.0.1:18402:5230/TCP": "127.0.0.1:18402:5230",
		"bad":                      "",
		"0:80":                     "",
		"80":                       "",
	}
	for in, want := range cases {
		if got := canonicalPortBinding(in); got != want {
			t.Errorf("canonicalPortBinding(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractPodPortsPreservesHostIP(t *testing.T) {
	t.Run("service-level HostIP loopback", func(t *testing.T) {
		s := Service{Port: 5230, HostPort: 18402, HostIP: "127.0.0.1"}
		got := extractPodPorts(s)
		want := []string{"127.0.0.1:18402:5230"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})

	t.Run("service-level empty HostIP means all interfaces", func(t *testing.T) {
		s := Service{Port: 80, HostPort: 18401}
		got := extractPodPorts(s)
		want := []string{"18401:80"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})

	t.Run("service-level explicit 0.0.0.0 omitted in canonical form", func(t *testing.T) {
		s := Service{Port: 80, HostPort: 18401, HostIP: "0.0.0.0"}
		got := extractPodPorts(s)
		want := []string{"18401:80"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
		if k := normalizePortBinding(got[0]); k != "0.0.0.0:18401:80" {
			t.Errorf("normalized key = %q, want 0.0.0.0:18401:80", k)
		}
	})

	t.Run("compose IP bindings plus service-level HostIP", func(t *testing.T) {
		s := Service{
			Port:     80,
			HostPort: 18401,
			HostIP:   "127.0.0.1",
			ComposeYaml: `services:
  web:
    ports:
      - "18401:80"
      - "127.0.0.1:18402:5230/udp"
      - "0.0.0.0:9000:9000/udp"
`,
		}
		got := extractPodPorts(s)
		want := []string{
			"127.0.0.1:18401:80",
			"18401:80",
			"127.0.0.1:18402:5230/udp",
			"9000:9000/udp",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})
}

func TestPortBindingIPDrift(t *testing.T) {
	// 0.0.0.0 vs 127.0.0.1 on the same ports MUST be different bindings.
	a := portBindingSet([]string{"18401:80"})
	b := portBindingSet([]string{"127.0.0.1:18401:80"})
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("expected singleton sets, got %v and %v", a, b)
	}
	for k := range a {
		if b[k] {
			t.Errorf("drift missed: %q present in both 0.0.0.0 and 127.0.0.1 sets", k)
		}
	}
	if !a["0.0.0.0:18401:80"] {
		t.Errorf("expected key 0.0.0.0:18401:80 in %v", a)
	}
	if !b["127.0.0.1:18401:80"] {
		t.Errorf("expected key 127.0.0.1:18401:80 in %v", b)
	}

	// Same IP + ports + proto from different spellings converge.
	c := portBindingSet([]string{"127.0.0.1:18402:5230/udp", "127.0.0.1:18402:5230/UDP"})
	if len(c) != 1 || !c["127.0.0.1:18402:5230/udp"] {
		t.Errorf("udp spellings should converge, got %v", c)
	}

	// udp vs tcp on the same IP/ports differ.
	d := portBindingSet([]string{"127.0.0.1:18402:5230", "127.0.0.1:18402:5230/udp"})
	if len(d) != 2 {
		t.Errorf("udp and tcp bindings should differ, got %v", d)
	}
}

func TestBuildPodQuadletBundlePreservesIP(t *testing.T) {
	s := Service{ID: "proj-web-abc123", Name: "web", ProjectID: "proj"}
	bundle := buildPodQuadletBundle(s, "pod_proj-web-abc123", []string{
		"18401:80",
		"127.0.0.1:18402:5230",
		"127.0.0.1:5353:53/udp",
		"9000:9000/udp",
	})
	for _, want := range []string{
		"PublishPort=18401:80",
		"PublishPort=127.0.0.1:18402:5230",
		"PublishPort=127.0.0.1:5353:53/udp",
		"PublishPort=9000:9000/udp",
	} {
		if !strings.Contains(bundle, want+"\n") {
			t.Errorf("bundle missing %q:\n%s", want, bundle)
		}
	}
	if strings.Contains(bundle, "PublishPort=0.0.0.0:") {
		t.Errorf("default 0.0.0.0 must be omitted in PublishPort, got:\n%s", bundle)
	}
}

func TestQuadletPublishPortsPreservesIP(t *testing.T) {
	content := `[Container]
Image=nginx:alpine
PublishPort=18401:80
PublishPort=127.0.0.1:18402:5230
PublishPort=127.0.0.1:5353:53/udp
`
	got := quadletPublishPorts(content)
	want := []string{"18401:80", "127.0.0.1:18402:5230", "127.0.0.1:5353:53/udp"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestServicePortBindingHelper(t *testing.T) {
	cases := []struct {
		ip, want string
		hp, cp   int
	}{
		{"", "18401:80", 18401, 80},
		{"0.0.0.0", "18401:80", 18401, 80},
		{"127.0.0.1", "127.0.0.1:18402:5230", 18402, 5230},
	}
	for _, c := range cases {
		if got := servicePortBinding(c.ip, c.hp, c.cp); got != c.want {
			t.Errorf("servicePortBinding(%q, %d, %d) = %q, want %q", c.ip, c.hp, c.cp, got, c.want)
		}
	}
}

func TestServiceHostIPJSONField(t *testing.T) {
	s := Service{ID: "x", HostIP: "127.0.0.1"}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["hostIp"] != "127.0.0.1" {
		t.Errorf("expected hostIp JSON field, got %s", string(raw))
	}
	var back Service
	if err := json.Unmarshal([]byte(`{"id":"x","hostIp":"127.0.0.1"}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.HostIP != "127.0.0.1" {
		t.Errorf("hostIp did not round-trip, got %q", back.HostIP)
	}
}
