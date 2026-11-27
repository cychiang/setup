package archive_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/golden"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/tree"
)

func fixture() archive.Content {
	b := catalog.Bundle{Name: "finance", Layer: catalog.Domain, Version: semver.Version{Major: 1, Minor: 2, Patch: 3}, Suggests: []string{"pdf-reports"}}
	skills := map[string]tree.Tree{
		"money-precision": {"SKILL.md": []byte("---\nname: money-precision\ndescription: d\n---\nUse decimals.\n"), "ref/notes.md": []byte("notes\n")},
		"comps":           {"SKILL.md": []byte("---\nname: comps\ndescription: d\n---\n")},
	}
	src := map[string]archive.Source{"comps": {Name: "kwp", URL: "https://example.com/kwp", Commit: strings.Repeat("a", 40)}}
	return archive.Assemble(b, skills, src, tree.Tree{"rules.md": []byte("Be precise.\n")})
}

func write(t *testing.T, c archive.Content, mtime time.Time) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := archive.Write(&buf, c, mtime); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestManifestGolden(t *testing.T) {
	mj, err := fixture().Manifest.JSON()
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "manifest.json", mj)
}

func TestArchiveIsReproducible(t *testing.T) {
	a, b := write(t, fixture(), time.Unix(0, 0)), write(t, fixture(), time.Unix(0, 0))
	if tree.SHA(a) != tree.SHA(b) {
		t.Fatal("same bundle built twice produced different archives")
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	c, err := archive.Read(write(t, fixture(), time.Unix(0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(c.Skills["money-precision"]["ref/notes.md"]); got != "notes\n" {
		t.Errorf("skill file = %q", got)
	}
	if c.Manifest.Skills[0].Name != "comps" || c.Manifest.Skills[0].Source == nil {
		t.Errorf("manifest skills not sorted or source lost: %+v", c.Manifest.Skills)
	}
	if string(c.Instructions["rules.md"]) != "Be precise.\n" {
		t.Errorf("instructions lost")
	}
}

func TestReadRejectsTamperedContent(t *testing.T) {
	c := fixture()
	data := write(t, c, time.Unix(0, 0))
	c.Skills["money-precision"]["SKILL.md"] = []byte("tampered")
	if _, err := archive.Read(write(t, c, time.Unix(0, 0))); err == nil {
		t.Fatal("expected hash mismatch error")
	}
	if _, err := archive.Read(data); err != nil {
		t.Fatalf("untampered archive should read: %v", err)
	}
}
