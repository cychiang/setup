package project_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/golden"
	"github.com/cychiang/setup/internal/project"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/store"
	"github.com/cychiang/setup/internal/tree"
)

func content(name string, layer catalog.Layer, skills []string, instr map[string]string) archive.Content {
	sk := map[string]tree.Tree{}
	for _, s := range skills {
		sk[s] = tree.Tree{"SKILL.md": []byte("---\nname: " + s + "\ndescription: d\n---\n")}
	}
	in := tree.Tree{}
	for k, v := range instr {
		in[k] = []byte(v)
	}
	return archive.Assemble(catalog.Bundle{Name: name, Layer: layer, Version: semver.Version{Major: 1}}, sk, nil, in)
}

func TestMergeRules(t *testing.T) {
	base := content("base", catalog.Base, []string{"workbench"}, map[string]string{"a.md": "base rules\n"})
	fin := content("finance", catalog.Domain, []string{"money"}, map[string]string{"x.md": "money rules\n", "y.md": "more\n"})
	pdf := content("pdf-reports", catalog.Type, []string{"pdf"}, map[string]string{"p.md": "pdf rules\n"})
	clash := content("charts", catalog.Type, []string{"money"}, nil)

	tests := []struct {
		name     string
		in       []archive.Content
		disable  []string
		wantErr  string
		wantFile []string
		noFile   []string
		blocks   []string
	}{
		{name: "layer order regardless of input order", in: []archive.Content{pdf, fin, base},
			wantFile: []string{".claude/skills/pdf/SKILL.md", ".agents/skills/money/SKILL.md"}, blocks: []string{"base", "finance", "pdf-reports"}},
		{name: "name collision names both bundles", in: []archive.Content{base, fin, clash}, wantErr: "both bundle finance and bundle charts"},
		{name: "base is mandatory", in: []archive.Content{fin}, wantErr: "base"},
		{name: "disable skill", in: []archive.Content{base, fin}, disable: []string{"finance/skills/money"},
			noFile: []string{".claude/skills/money/SKILL.md", ".agents/skills/money/SKILL.md"}, blocks: []string{"base", "finance"}},
		{name: "disable instruction file drops only that part", in: []archive.Content{base, fin}, disable: []string{"finance/instructions/y.md"}, blocks: []string{"base", "finance"}},
		{name: "disabling all instructions drops the block", in: []archive.Content{base, pdf}, disable: []string{"pdf-reports/instructions/p.md"}, blocks: []string{"base"}},
		{name: "unknown disable skill", in: []archive.Content{base}, disable: []string{"base/skills/nope"}, wantErr: "unknown disable entry"},
		{name: "unknown disable bundle", in: []archive.Content{base}, disable: []string{"ghost/skills/x"}, wantErr: "unknown disable entry"},
		{name: "malformed disable", in: []archive.Content{base}, disable: []string{"base"}, wantErr: "unknown disable entry"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := project.MakePlan(tc.in, tc.disable)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range tc.wantFile {
				if _, ok := plan.Files[f]; !ok {
					t.Errorf("missing %s", f)
				}
			}
			for _, f := range tc.noFile {
				if _, ok := plan.Files[f]; ok {
					t.Errorf("%s should be disabled", f)
				}
			}
			var got []string
			for _, b := range plan.Blocks {
				got = append(got, b.Bundle)
			}
			if strings.Join(got, ",") != strings.Join(tc.blocks, ",") {
				t.Errorf("blocks = %v, want %v", got, tc.blocks)
			}
		})
	}
}

// setup builds a directory catalog of archives and a project dir.
type env struct {
	catalog, root string
	st            store.Store
}

func newEnv(t *testing.T, contents ...archive.Content) env {
	t.Helper()
	e := env{catalog: t.TempDir(), root: t.TempDir()}
	e.publish(t, contents...)
	e.st = &store.Dir{Path: e.catalog}
	return e
}

func (e env) publish(t *testing.T, contents ...archive.Content) {
	t.Helper()
	for _, c := range contents {
		var b bytes.Buffer
		if err := archive.Write(&b, c, time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(e.catalog, archive.FileName(c.Manifest.Name, c.Manifest.Version))
		os.WriteFile(f, b.Bytes(), 0o644)
		os.WriteFile(f+".sha256", []byte(tree.SHA(b.Bytes())+"  x\n"), 0o644)
	}
}

func standard(t *testing.T) env {
	return newEnv(t,
		content("base", catalog.Base, []string{"workbench"}, map[string]string{"a.md": "base rules\n"}),
		content("finance", catalog.Domain, []string{"money"}, map[string]string{"x.md": "money rules\n"}),
		content("pdf-reports", catalog.Type, []string{"pdf"}, map[string]string{"p.md": "pdf rules\n"}),
	)
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallGoldens(t *testing.T) {
	e := standard(t)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"pdf-reports", "finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "AGENTS.md", []byte(read(t, e.root, "AGENTS.md")))
	if read(t, e.root, "AGENTS.md") != read(t, e.root, "CLAUDE.md") {
		t.Error("AGENTS.md and CLAUDE.md should carry the same blocks")
	}
	lock := read(t, e.root, "setup.lock")
	// archive hashes depend on the gzip implementation; pin the structure, not those bytes.
	lock = redactArchiveHashes(lock)
	golden.Assert(t, "setup.lock", []byte(lock))
	for _, p := range []string{".claude/skills/money/SKILL.md", ".agents/skills/money/SKILL.md", ".claude/skills/workbench/SKILL.md"} {
		if read(t, e.root, p) == "" {
			t.Errorf("%s empty", p)
		}
	}
}

func redactArchiveHashes(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.Contains(l, `"archive_sha256"`) {
			lines[i] = `      "archive_sha256": "<redacted>"`
		}
	}
	return strings.Join(lines, "\n")
}

func TestSecondInstallWritesNothing(t *testing.T) {
	e := standard(t)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	watched := []string{"AGENTS.md", "CLAUDE.md", "setup.lock", ".claude/skills/money/SKILL.md"}
	before := map[string]time.Time{}
	for _, w := range watched {
		fi, _ := os.Stat(filepath.Join(e.root, w))
		before[w] = fi.ModTime()
	}
	time.Sleep(20 * time.Millisecond)
	res, err := project.Install(e.root, cfg, e.st, project.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Report.Written)+len(res.Report.Deleted) != 0 {
		t.Fatalf("second run changed %+v", res.Report)
	}
	for _, w := range watched {
		fi, _ := os.Stat(filepath.Join(e.root, w))
		if !fi.ModTime().Equal(before[w]) {
			t.Errorf("%s was rewritten", w)
		}
	}
}

func TestTextOutsideMarkersIsPreserved(t *testing.T) {
	e := standard(t)
	user := "# My project\n\nHand-written rules.\n"
	os.WriteFile(filepath.Join(e.root, "AGENTS.md"), []byte(user), 0o644)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	got := read(t, e.root, "AGENTS.md")
	if !strings.HasPrefix(got, user) || !strings.Contains(got, "<!-- setup:begin finance -->") {
		t.Fatalf("user text lost or block missing:\n%s", got)
	}
	// user edits around the markers and adds a bundle in the middle of the order
	edited := "Intro line.\n" + got + "\nTrailing notes.\n"
	os.WriteFile(filepath.Join(e.root, "AGENTS.md"), []byte(edited), 0o644)
	cfg.Bundles = []string{"finance", "pdf-reports"}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	got = read(t, e.root, "AGENTS.md")
	for _, want := range []string{"Intro line.\n# My project", "Hand-written rules.", "Trailing notes.\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q:\n%s", want, got)
		}
	}
	bi, fi, pi := strings.Index(got, "setup:begin base"), strings.Index(got, "setup:begin finance"), strings.Index(got, "setup:begin pdf-reports")
	if !(bi < fi && fi < pi) {
		t.Errorf("blocks out of layer order:\n%s", got)
	}
	// removing a bundle removes only its block
	cfg.Bundles = []string{"pdf-reports"}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	got = read(t, e.root, "AGENTS.md")
	if strings.Contains(got, "finance") || !strings.Contains(got, "Hand-written rules.") || !strings.Contains(got, "Trailing notes.") {
		t.Errorf("unexpected result:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(e.root, ".claude/skills/money")); !os.IsNotExist(err) {
		t.Error("removed bundle's skill should be deleted")
	}
}

func TestHandEditedFileIsConflictUnlessForced(t *testing.T) {
	e := standard(t)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(e.root, ".claude/skills/money/SKILL.md")
	os.WriteFile(skillPath, []byte("hand edited"), 0o644)

	lock, _ := project.LoadLock(e.root)
	drift, err := project.Drift(e.root, lock)
	if err != nil || len(drift) != 1 || !strings.Contains(drift[0], ".claude/skills/money/SKILL.md: modified") {
		t.Fatalf("drift = %v, err = %v", drift, err)
	}

	e.publish(t, withVersion(content("finance", catalog.Domain, []string{"money"}, map[string]string{"x.md": "v2\n"}), 2))
	_, err = project.Install(e.root, cfg, e.st, project.Options{UpgradeAll: true})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want conflict error, got %v", err)
	}
	if read(t, e.root, ".claude/skills/money/SKILL.md") != "hand edited" {
		t.Fatal("conflicting file was modified")
	}
	if strings.Contains(read(t, e.root, "AGENTS.md"), "v2") {
		t.Fatal("nothing may be written when a conflict aborts the run")
	}
	res, err := project.Install(e.root, cfg, e.st, project.Options{UpgradeAll: true, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, e.root, "AGENTS.md"), "v2") || read(t, e.root, ".claude/skills/money/SKILL.md") == "hand edited" {
		t.Fatalf("force should overwrite: %+v", res.Report)
	}
}

func TestHandEditedBlockIsDrift(t *testing.T) {
	e := standard(t)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(e.root, "AGENTS.md")
	os.WriteFile(p, []byte(strings.Replace(read(t, e.root, "AGENTS.md"), "money rules", "money rules EDITED", 1)), 0o644)
	lock, _ := project.LoadLock(e.root)
	drift, _ := project.Drift(e.root, lock)
	if len(drift) != 1 || !strings.Contains(drift[0], "block finance") {
		t.Fatalf("drift = %v", drift)
	}
}

func TestOutdatedAndPinning(t *testing.T) {
	e := standard(t)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err != nil {
		t.Fatal(err)
	}
	e.publish(t, withVersion(content("finance", catalog.Domain, []string{"money"}, map[string]string{"x.md": "v2\n"}), 2))
	lock, _ := project.LoadLock(e.root)
	ups, err := project.Outdated(lock, e.st)
	if err != nil || len(ups) != 1 || ups[0].Name != "finance" || ups[0].Latest.Major != 2 {
		t.Fatalf("ups = %+v err = %v", ups, err)
	}
	res, err := project.Install(e.root, cfg, e.st, project.Options{})
	if err != nil || len(res.Report.Written) != 0 {
		t.Fatalf("plain re-run must stay pinned: %+v %v", res.Report, err)
	}
}

func TestChecksumMismatchIsRejected(t *testing.T) {
	e := standard(t)
	f := filepath.Join(e.catalog, "finance-1.0.0.tar.gz.sha256")
	os.WriteFile(f, []byte(strings.Repeat("0", 64)+"  x\n"), 0o644)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance"}}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
}

func withVersion(c archive.Content, major int) archive.Content {
	c.Manifest.Version = semver.Version{Major: major}
	return c
}
