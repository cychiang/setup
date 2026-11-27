// Package golden compares test output with files under testdata/golden. Pass -update to rewrite them.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func Assert(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update): %v", p, err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from golden:\n--- want\n%s\n--- got\n%s", p, want, got)
	}
}
