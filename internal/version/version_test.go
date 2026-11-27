package version

import (
	"strings"
	"testing"
)

func TestStringUsesStampedValues(t *testing.T) {
	v, c, d := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = v, c, d })
	Version, Commit, Date = "v1.2.3", "abc123", "2026-01-02T03:04:05Z"
	want := "setup v1.2.3 (commit abc123, built 2026-01-02T03:04:05Z)"
	if got := String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStringDefaultsToDev(t *testing.T) {
	v, c, d := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = v, c, d })
	Version, Commit, Date = "dev", "", ""
	got := String()
	if !strings.HasPrefix(got, "setup dev (commit ") || !strings.Contains(got, "built ") {
		t.Fatalf("unexpected default: %q", got)
	}
}
