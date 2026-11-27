package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cychiang/setup/internal/project"
	"github.com/cychiang/setup/internal/tree"
)

const importFile = "@AGENTS.md\n"

func installFinance(t *testing.T, e env) project.Result {
	t.Helper()
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance", "pdf-reports"}}
	res, err := project.Install(e.root, cfg, e.st, project.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func writeFile(t *testing.T, root, rel, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, rel), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSingleSource(t *testing.T, root string) {
	t.Helper()
	claude := read(t, root, "CLAUDE.md")
	agents := read(t, root, "AGENTS.md")
	for _, body := range []string{"base rules", "money rules", "pdf rules"} {
		if strings.Count(agents, body) != 1 {
			t.Errorf("AGENTS.md should hold %q once:\n%s", body, agents)
		}
		if strings.Contains(claude, body) {
			t.Errorf("CLAUDE.md must not repeat %q, Claude loads it through the import:\n%s", body, claude)
		}
	}
}

func TestClaudeMDAbsentGetsOnlyTheImport(t *testing.T) {
	e := standard(t)
	installFinance(t, e)
	if got := read(t, e.root, "CLAUDE.md"); got != importFile {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	assertSingleSource(t, e.root)
}

func TestClaudeMDWithUserImportIsLeftAlone(t *testing.T) {
	for _, line := range []string{"@AGENTS.md", "@./AGENTS.md", "  @AGENTS.md  "} {
		e := standard(t)
		user := "# Rules\n" + line + "\nMore text.\n"
		writeFile(t, e.root, "CLAUDE.md", user)
		res := installFinance(t, e)
		if got := read(t, e.root, "CLAUDE.md"); got != user {
			t.Errorf("%q: CLAUDE.md changed to %q", line, got)
		}
		for _, w := range res.Report.Written {
			if w == "CLAUDE.md" {
				t.Errorf("%q: CLAUDE.md reported as written", line)
			}
		}
		assertSingleSource(t, e.root)
	}
}

func TestClaudeMDImportInsideCodeFenceDoesNotCount(t *testing.T) {
	e := standard(t)
	user := "Example:\n\n```\n@AGENTS.md\n```\n"
	writeFile(t, e.root, "CLAUDE.md", user)
	installFinance(t, e)
	if got := read(t, e.root, "CLAUDE.md"); got != importFile+"\n"+user {
		t.Fatalf("CLAUDE.md = %q", got)
	}
}

func TestClaudeMDWithoutImportGetsItPrependedAndKeepsText(t *testing.T) {
	e := standard(t)
	user := "# My Claude rules\n\nBe kind.\n"
	writeFile(t, e.root, "CLAUDE.md", user)
	installFinance(t, e)
	if got := read(t, e.root, "CLAUDE.md"); got != importFile+"\n"+user {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	assertSingleSource(t, e.root)
}

func TestOwnedClaudeFileStaysTrackedAndUserTextIsNeverTracked(t *testing.T) {
	e := standard(t)
	installFinance(t, e)
	lock, _ := project.LoadLock(e.root)
	var tracked int
	for _, f := range lock.Files {
		if f.Path == "CLAUDE.md" {
			tracked++
			if f.Bundle != "base" || f.SHA256 != tree.SHA([]byte(importFile)) {
				t.Errorf("entry = %+v", f)
			}
		}
	}
	if tracked != 1 {
		t.Fatalf("CLAUDE.md tracked %d times", tracked)
	}

	e2 := standard(t)
	writeFile(t, e2.root, "CLAUDE.md", "mine\n")
	installFinance(t, e2)
	lock, _ = project.LoadLock(e2.root)
	for _, f := range lock.Files {
		if f.Path == "CLAUDE.md" {
			t.Fatalf("a user-owned CLAUDE.md must not be tracked: %+v", f)
		}
	}
}

func TestUserTakingOverAnOwnedFileEndsTrackingWithoutLosingText(t *testing.T) {
	e := standard(t)
	installFinance(t, e)
	writeFile(t, e.root, "CLAUDE.md", "@AGENTS.md\n\nMy extra rule.\n")
	installFinance(t, e)
	if got := read(t, e.root, "CLAUDE.md"); got != "@AGENTS.md\n\nMy extra rule.\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	lock, _ := project.LoadLock(e.root)
	for _, f := range lock.Files {
		if f.Path == "CLAUDE.md" {
			t.Fatalf("still tracked: %+v", f)
		}
	}
}

func TestRerunWritesNothingToEitherFile(t *testing.T) {
	e := standard(t)
	installFinance(t, e)
	res := installFinance(t, e)
	if len(res.Report.Written)+len(res.Report.Deleted)+len(res.Report.Conflicts) != 0 {
		t.Fatalf("rerun changed %+v", res.Report)
	}
}

// oldLayout rewrites a fresh install into what the previous version produced:
// every bundle block in CLAUDE.md, each tracked in setup.lock.
func oldLayout(t *testing.T, e env) {
	t.Helper()
	installFinance(t, e)
	writeFile(t, e.root, "CLAUDE.md", read(t, e.root, "AGENTS.md"))
	lock, err := project.LoadLock(e.root)
	if err != nil {
		t.Fatal(err)
	}
	var files []project.LockFile
	for _, f := range lock.Files {
		files = append(files, f)
		if f.Path == "AGENTS.md" {
			files = append(files, project.LockFile{Path: "CLAUDE.md", SHA256: f.SHA256, Bundle: f.Bundle})
		}
	}
	lock.Files = files
	data, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, e.root, project.LockName, string(data))
}

func TestMigrationFromTheDoubleWriteLayout(t *testing.T) {
	e := standard(t)
	oldLayout(t, e)
	if strings.Count(read(t, e.root, "CLAUDE.md"), "money rules") != 1 {
		t.Fatal("fixture should have the old layout")
	}
	res := installFinance(t, e)
	if got := read(t, e.root, "CLAUDE.md"); got != importFile {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	assertSingleSource(t, e.root)
	if len(res.Report.Conflicts) != 0 {
		t.Fatalf("conflicts %v", res.Report.Conflicts)
	}
	lock, _ := project.LoadLock(e.root)
	drift, err := project.Drift(e.root, lock)
	if err != nil || len(drift) != 0 {
		t.Fatalf("drift after migration = %v, %v", drift, err)
	}
	var claude []project.LockFile
	for _, f := range lock.Files {
		if f.Path == "CLAUDE.md" {
			claude = append(claude, f)
		}
	}
	if len(claude) != 1 || claude[0].Bundle != "base" || claude[0].SHA256 != tree.SHA([]byte(importFile)) {
		t.Fatalf("lock entries for CLAUDE.md = %+v", claude)
	}
	if again := installFinance(t, e); len(again.Report.Written)+len(again.Report.Deleted) != 0 {
		t.Fatalf("second run after migration changed %+v", again.Report)
	}
}

func TestMigrationKeepsTextOutsideTheOldBlocks(t *testing.T) {
	e := standard(t)
	oldLayout(t, e)
	writeFile(t, e.root, "CLAUDE.md", "My notes.\n\n"+read(t, e.root, "CLAUDE.md")+"\nTail.\n")
	installFinance(t, e)
	got := read(t, e.root, "CLAUDE.md")
	if !strings.HasPrefix(got, "@AGENTS.md\n\nMy notes.\n") || !strings.HasSuffix(got, "Tail.\n") {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	assertSingleSource(t, e.root)
	lock, _ := project.LoadLock(e.root)
	for _, f := range lock.Files {
		if f.Path == "CLAUDE.md" {
			t.Fatalf("user text file must stay untracked: %+v", f)
		}
	}
	if again := installFinance(t, e); len(again.Report.Written) != 0 {
		t.Fatalf("rerun wrote %v", again.Report.Written)
	}
}

func TestMigrationTreatsAHandEditedOldBlockAsConflict(t *testing.T) {
	e := standard(t)
	oldLayout(t, e)
	edited := strings.Replace(read(t, e.root, "CLAUDE.md"), "money rules", "money rules EDITED", 1)
	writeFile(t, e.root, "CLAUDE.md", edited)
	cfg := project.Config{Catalog: e.catalog, Bundles: []string{"finance", "pdf-reports"}}
	res, err := project.Install(e.root, cfg, e.st, project.Options{})
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want conflict, got %v", err)
	}
	if len(res.Report.Conflicts) != 1 || res.Report.Conflicts[0] != "CLAUDE.md" {
		t.Fatalf("conflicts = %v", res.Report.Conflicts)
	}
	if read(t, e.root, "CLAUDE.md") != edited {
		t.Fatal("nothing may change while a conflict stands")
	}
	if _, err := project.Install(e.root, cfg, e.st, project.Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, e.root, "CLAUDE.md"); got != importFile {
		t.Fatalf("forced CLAUDE.md = %q", got)
	}
}

func TestCheckFollowsTheNewDesiredState(t *testing.T) {
	e := standard(t)
	installFinance(t, e)
	lock, _ := project.LoadLock(e.root)
	if drift, _ := project.Drift(e.root, lock); len(drift) != 0 {
		t.Fatalf("fresh install drifts: %v", drift)
	}
	writeFile(t, e.root, "CLAUDE.md", "something else\n")
	drift, _ := project.Drift(e.root, lock)
	if len(drift) != 1 || drift[0] != "CLAUDE.md: modified" {
		t.Fatalf("drift = %v", drift)
	}
}
