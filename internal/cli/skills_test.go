package cli_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

type listed struct {
	Ref, Source, Path, Name, Description string
	UsedBy                               []string `json:"used_by"`
	Valid                                bool
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// skillsFixture builds a catalog with a pinned upstream source holding nested
// skills, an unpinned second source, and one bundle with a local skill.
func skillsFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("SETUP_HOME", t.TempDir())
	repo := t.TempDir()
	write(t, repo, "finance/skills/comps/SKILL.md", "---\nname: comps\ndescription: Comparable company analysis | multiples.\n---\nbody\n")
	write(t, repo, "finance/skills/comps/ref.md", "ref\n")
	write(t, repo, "legal/skills/nda/SKILL.md", skillMD("nda"))
	write(t, repo, "misc/broken/SKILL.md", "no frontmatter here\n")
	write(t, repo, "README.md", "not a skill\n")
	commitAll(t, repo)

	work := t.TempDir()
	mustRun(t, work, "source", "add", "fixture", repo)
	mustRun(t, work, "source", "sync")
	mustRun(t, work, "source", "add", "later", t.TempDir())
	mustRun(t, work, "bundle", "new", "base", "--layer", "base")
	write(t, work, "bundles/base/skills/workbench/SKILL.md", skillMD("workbench"))
	mustRun(t, work, "bundle", "add", "base", "--local", "workbench")
	mustRun(t, work, "bundle", "new", "finance", "--layer", "domain")
	mustRun(t, work, "bundle", "add", "finance", "fixture/finance/skills/comps")
	return work
}

func listJSON(t *testing.T, work string, args ...string) []listed {
	t.Helper()
	out := mustRun(t, work, append([]string{"skills", "--json"}, args...)...)
	var got []listed
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	return got
}

func TestSkillsListsUpstreamAndLocal(t *testing.T) {
	work := skillsFixture(t)
	got := listJSON(t, work)
	want := []listed{
		{Ref: "local:base/workbench", Path: "workbench", Name: "workbench", UsedBy: []string{"base"}, Valid: true},
		{Ref: "fixture/finance/skills/comps", Source: "fixture", Path: "finance/skills/comps", Name: "comps", UsedBy: []string{"finance"}, Valid: true},
		{Ref: "fixture/legal/skills/nda", Source: "fixture", Path: "legal/skills/nda", Name: "nda", UsedBy: []string{}, Valid: true},
		{Ref: "fixture/misc/broken", Source: "fixture", Path: "misc/broken", Name: "broken", UsedBy: []string{}, Valid: false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d skills: %+v", len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Ref != w.Ref || g.Source != w.Source || g.Path != w.Path || g.Name != w.Name || g.Valid != w.Valid || strings.Join(g.UsedBy, ",") != strings.Join(w.UsedBy, ",") {
			t.Errorf("skill %d = %+v, want %+v", i, g, w)
		}
	}
	if got[1].Description != "Comparable company analysis | multiples." {
		t.Errorf("description = %q", got[1].Description)
	}
	if !strings.Contains(mustRun(t, work, "skills", "--json", "--query", "nomatch"), "[]") {
		t.Error("empty result must be an empty array")
	}
}

func TestSkillsQueryAndSourceFilters(t *testing.T) {
	work := skillsFixture(t)
	if got := listJSON(t, work, "--query", "MULTIPLES"); len(got) != 1 || got[0].Name != "comps" {
		t.Fatalf("description match, case-insensitive: %+v", got)
	}
	if got := listJSON(t, work, "--query", "misc/"); len(got) != 1 || got[0].Name != "broken" {
		t.Fatalf("path match: %+v", got)
	}
	got := listJSON(t, work, "--source", "fixture")
	if len(got) != 3 || got[0].Source != "fixture" {
		t.Fatalf("--source must hide local skills: %+v", got)
	}
	if code, _, _ := run(t, work, "skills", "--source", "nosuch"); code == 0 {
		t.Fatal("unknown source must fail")
	}
}

func TestSkillsUnpinnedSourceWarns(t *testing.T) {
	work := skillsFixture(t)
	code, _, errs := run(t, work, "skills")
	if code != 0 || !strings.Contains(errs, "later") || !strings.Contains(errs, "setup source sync later") {
		t.Fatalf("want warning naming source and fix, code=%d stderr=%q", code, errs)
	}
}

func TestSkillsHumanAndMarkdownOutput(t *testing.T) {
	work := skillsFixture(t)
	table := mustRun(t, work, "skills")
	for _, want := range []string{"REF", "NAME", "USED BY", "DESCRIPTION", "fixture/finance/skills/comps", "broken (invalid)"} {
		if !strings.Contains(table, want) {
			t.Errorf("table lacks %q:\n%s", want, table)
		}
	}
	md := mustRun(t, work, "skills", "--markdown", "--query", "comps")
	if !strings.Contains(md, "| Ref | Name | Used by | Description |") || !strings.Contains(md, `Comparable company analysis \| multiples.`) {
		t.Errorf("markdown must escape pipes:\n%s", md)
	}
}

func TestSkillsRefRoundTripsIntoBundleAdd(t *testing.T) {
	work := skillsFixture(t)
	mustRun(t, work, "bundle", "new", "reports", "--layer", "type")
	write(t, work, "bundles/reports/skills/x/SKILL.md", skillMD("x"))
	for _, s := range listJSON(t, work, "--source", "fixture") {
		if s.Name != "nda" {
			continue
		}
		mustRun(t, work, "bundle", "add", "reports", s.Ref)
	}
	if code, out, errs := run(t, work, "bundle", "check"); code != 0 {
		t.Fatalf("bundle check after adding a listed ref: %s%s", out, errs)
	}
	for _, s := range listJSON(t, work, "--source", "fixture") {
		if s.Name == "nda" && strings.Join(s.UsedBy, ",") != "reports" {
			t.Errorf("UsedBy after add = %v", s.UsedBy)
		}
	}
}

func TestBundleList(t *testing.T) {
	work := skillsFixture(t)
	mustRun(t, work, "bundle", "new", "aaa", "--layer", "type")
	out := mustRun(t, work, "bundle", "list", "--json")
	var got []struct {
		Name, Layer, Version string
		Skills               []string
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, b := range got {
		order = append(order, b.Name)
	}
	if strings.Join(order, ",") != "base,finance,aaa" {
		t.Fatalf("order by layer then name: %v", order)
	}
	if got[1].Version != "0.1.0" || got[1].Layer != "domain" || got[1].Skills[0] != "fixture/finance/skills/comps" || got[0].Skills[0] != "local:base/workbench" {
		t.Fatalf("%+v", got)
	}
	if human := mustRun(t, work, "bundle", "list"); !strings.Contains(human, "NAME") || !strings.Contains(human, "finance") {
		t.Fatalf("table: %s", human)
	}
}
