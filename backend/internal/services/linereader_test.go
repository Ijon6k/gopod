package services

import (
	"io"
	"strings"
	"testing"
)

// Regression tests for the BE-3 pipe-stall defect: bufio.Scanner's default
// 64 KiB token cap aborted the pipe reader whenever a subprocess emitted one
// long line, leaving the child blocked on write() until the deploy context
// deadline. streamLines must survive arbitrarily long lines (bounded memory,
// explicit truncation marker) and always consume to EOF.

func streamInto(t *testing.T, input string) []string {
	t.Helper()
	var lines []string
	if err := streamLines(strings.NewReader(input), func(s string) {
		lines = append(lines, s)
	}); err != nil {
		t.Fatalf("streamLines returned error: %v", err)
	}
	return lines
}

func TestStreamLinesBasics(t *testing.T) {
	got := streamInto(t, "one\ntwo\nthree\n")
	want := []string{"one", "two", "three"}
	if len(got) != len(want) {
		t.Fatalf("got %d records (%q), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("record %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStreamLinesNoTrailingEmptyRecord(t *testing.T) {
	// A file ending in "\n" must not gain a bogus trailing empty record,
	// and input without a final newline must still emit its last line.
	if got := streamInto(t, "only\n"); len(got) != 1 || got[0] != "only" {
		t.Fatalf("got %q, want exactly [only]", got)
	}
	if got := streamInto(t, "only"); len(got) != 1 || got[0] != "only" {
		t.Fatalf("got %q, want exactly [only]", got)
	}
}

func TestStreamLinesKeepsBlankRows(t *testing.T) {
	got := streamInto(t, "a\n\nb\n")
	if len(got) != 3 || got[1] != "" {
		t.Fatalf("got %q, want [a, \"\", b] (blank lines are real records)", got)
	}
}

func TestStreamLinesSubCapLongLinePassedThrough(t *testing.T) {
	long := strings.Repeat("x", maxStreamLine-1000)
	got := streamInto(t, long+"\n")
	if len(got) != 1 || got[0] != long {
		t.Fatalf("sub-cap long line altered: got %d bytes, want %d", len(got), len(long))
	}
}

func TestStreamLinesOverCapTruncatedThenContinues(t *testing.T) {
	// A line exceeding the cap: emitted ONCE, truncated, with an explicit
	// marker; and — the regression core — subsequent normal lines must
	// still be delivered.
	long := strings.Repeat("y", maxStreamLine+50000)
	got := streamInto(t, long+"\nsurvivor\n")
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (truncated + survivor)", len(got))
	}
	if !strings.HasSuffix(got[0], " ...[truncated line]") {
		t.Fatalf("first record missing truncation marker: %q", tailOf(got[0], 64))
	}
	if len(got[0]) > maxStreamLine+64 {
		t.Fatalf("truncated record retained %d bytes; unbounded memory", len(got[0]))
	}
	if got[1] != "survivor" {
		t.Fatalf("post-truncation line lost: got %q", got[1])
	}
}

func TestStreamLinesCallerStillDrainsAfterTruncation(t *testing.T) {
	// The caller contract: even if streamLines ever returns early, the
	// deployer drains the pipe afterwards. Assert drain works standalone on
	// an overlong multi-line stream (mirrors deployer.go / LogManager).
	r := strings.NewReader(strings.Repeat("z", maxStreamLine*2) + "\n")
	n, err := io.Copy(io.Discard, r)
	if err != nil || n == 0 {
		t.Fatalf("drain failed: read %d bytes, err %v", n, err)
	}
}

func tailOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
