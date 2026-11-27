package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cychiang/setup/internal/cli"
)

type devEnv struct {
	work, cat, proj string
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// newDevEnv builds a source catalog with a base bundle and a finance bundle that
// pins a third-party skill from a local git repo, then links a fresh project to it.
func newDevEnv(t *testing.T) devEnv {
	t.Helper()
	t.Setenv("SETUP_HOME", t.TempDir())
	repo := gitRepo(t)
	work := t.TempDir()
	mustRun(t, work, "source", "add", "fixture", repo)
	mustRun(t, work, "source", "sync")
	mustRun(t, work, "bundle", "new", "base", "--layer", "base")
	write(t, work, "bundles/base/skills/workbench/SKILL.md", skillMD("workbench"))
	write(t, work, "bundles/base/instructions/conventions.md", "Base conventions.\n")
	mustRun(t, work, "bundle", "add", "base", "--local", "workbench")
	mustRun(t, work, "bundle", "new", "finance", "--layer", "domain")
	write(t, work, "bundles/finance/skills/money/SKILL.md", skillMD("money"))
	mustRun(t, work, "bundle", "add", "finance", "--local", "money")
	mustRun(t, work, "bundle", "add", "finance", "fixture/skills/comps")

	e := devEnv{work: work, cat: filepath.Join(work, "bundles"), proj: t.TempDir()}
	mustRun(t, e.proj, "init", "--bundles", "finance", "--catalog", e.cat, "--yes")
	return e
}

func TestDevInitInstallsAndKeepsPathsLocal(t *testing.T) {
	e := newDevEnv(t)
	for _, root := range []string{".claude/skills", ".agents/skills"} {
		for _, s := range []string{"workbench", "money", "comps"} {
			if !exists(e.proj, root+"/"+s+"/SKILL.md") {
				t.Fatalf("%s/%s not installed", root, s)
			}
		}
	}
	if !strings.Contains(readFile(t, e.proj, "setup.local.yaml"), e.cat) {
		t.Fatal("setup.local.yaml must hold the dev catalog")
	}
	mustRun(t, e.proj, "update")
	if n := strings.Count(readFile(t, e.proj, ".gitignore"), "setup.local.yaml"); n != 1 {
		t.Fatalf(".gitignore lists setup.local.yaml %d times", n)
	}
	for _, f := range []string{"setup.yaml", "setup.lock"} {
		if got := readFile(t, e.proj, f); strings.Contains(got, e.work) {
			t.Fatalf("%s leaks the local path:\n%s", f, got)
		}
	}
	if !strings.Contains(readFile(t, e.proj, "setup.lock"), `"source": "dev"`) {
		t.Fatal("lock must mark dev bundles")
	}
}

func TestDevUpdatePicksUpCatalogEditsAndIsIdempotent(t *testing.T) {
	e := newDevEnv(t)
	write(t, e.work, "bundles/finance/skills/money/SKILL.md", skillMD("money")+"\nedited in the catalog\n")
	mustRun(t, e.proj, "update")
	if !strings.Contains(readFile(t, e.proj, ".claude/skills/money/SKILL.md"), "edited in the catalog") {
		t.Fatal("update did not rebuild from the dev catalog")
	}
	if out := mustRun(t, e.proj, "update"); !strings.Contains(out, "nothing to change") {
		t.Fatalf("rerun must write nothing:\n%s", out)
	}
	mustRun(t, e.proj, "add", "base")
}

func TestDevUpdateKeepsHandEditProtection(t *testing.T) {
	e := newDevEnv(t)
	p := filepath.Join(e.proj, ".claude/skills/money/SKILL.md")
	os.WriteFile(p, []byte("hand edited"), 0o644)
	write(t, e.work, "bundles/finance/skills/money/SKILL.md", skillMD("money")+"\nnew\n")
	if code, _, errs := run(t, e.proj, "update"); code == 0 || !strings.Contains(errs, "hand-edited") {
		t.Fatalf("want conflict, got %d %s", code, errs)
	}
	if readFile(t, e.proj, ".claude/skills/money/SKILL.md") != "hand edited" {
		t.Fatal("conflicting file was modified")
	}
}

func TestDevCheckFailsWhileLinkedAndWithoutLocalFile(t *testing.T) {
	e := newDevEnv(t)
	code, out, _ := run(t, e.proj, "check")
	if code != 1 || !strings.Contains(out, "base, finance") || !strings.Contains(out, "setup unlink") {
		t.Fatalf("check while linked: %d %s", code, out)
	}
	if err := os.Remove(filepath.Join(e.proj, "setup.local.yaml")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := run(t, e.proj, "check"); code != 1 || !strings.Contains(out, "setup unlink") {
		t.Fatalf("check without setup.local.yaml: %d %s", code, out)
	}
	if code, out, _ := run(t, e.proj, "check", "--json"); code != 1 || !strings.Contains(out, `"dev_linked"`) {
		t.Fatalf("check --json: %d %s", code, out)
	}
}

func TestDevOutdatedReportsDevLinked(t *testing.T) {
	e := newDevEnv(t)
	out := mustRun(t, e.proj, "outdated")
	if !strings.Contains(out, "finance dev-linked") || strings.Contains(out, "->") {
		t.Fatalf("outdated:\n%s", out)
	}
}

func TestDevSecurityGateRefusesAndWritesNothing(t *testing.T) {
	e := newDevEnv(t)
	skill := ".claude/skills/money/SKILL.md"
	before, lockBefore := readFile(t, e.proj, skill), readFile(t, e.proj, "setup.lock")
	write(t, e.work, "bundles/finance/skills/money/SKILL.md", skillMD("money")+"\nsneaky\u200b text\n")
	code, _, errs := run(t, e.proj, "update")
	if code == 0 || !strings.Contains(errs, "hidden") {
		t.Fatalf("want refusal, got %d %s", code, errs)
	}
	if readFile(t, e.proj, skill) != before || readFile(t, e.proj, "setup.lock") != lockBefore {
		t.Fatal("a refused dev update must leave the project unchanged")
	}
}

func TestUnlinkRestoresReleasedInstall(t *testing.T) {
	e := newDevEnv(t)
	dist := filepath.Join(e.work, "dist")
	for _, b := range []string{"base", "finance"} {
		mustRun(t, e.work, "bundle", "release", b, "--out", dist, "--allow-missing-scanners")
	}
	var out, errb bytes.Buffer
	env := cli.Env{Dir: e.proj, Out: &out, Err: &errb, Getenv: func(k string) string {
		if k == "SETUP_CATALOG" {
			return dist
		}
		return os.Getenv(k)
	}}
	if code := cli.Run([]string{"unlink"}, env); code != 0 {
		t.Fatalf("unlink via SETUP_CATALOG: exit %d\n%s", code, errb.String())
	}
	if exists(e.proj, "setup.local.yaml") {
		t.Fatal("unlink must remove setup.local.yaml")
	}
	if lock := readFile(t, e.proj, "setup.lock"); strings.Contains(lock, `"dev"`) || strings.Contains(lock, `"archive_sha256": ""`) {
		t.Fatalf("lock still has dev markers:\n%s", lock)
	}
	if out := mustRun(t, e.proj, "check"); !strings.Contains(out, "ok") {
		t.Fatalf("check after unlink: %s", out)
	}
	if !exists(e.proj, ".claude/skills/money/SKILL.md") {
		t.Fatal("skills must stay installed after unlink")
	}
}

func TestLinkAnInitializedProject(t *testing.T) {
	e := newDevEnv(t)
	dist := filepath.Join(e.work, "dist")
	for _, b := range []string{"base", "finance"} {
		mustRun(t, e.work, "bundle", "release", b, "--out", dist, "--allow-missing-scanners")
	}
	mustRun(t, e.proj, "unlink", "--catalog", dist)
	if code, _, _ := run(t, e.proj, "link", dist); code == 0 {
		t.Fatal("link must reject an archive directory")
	}
	mustRun(t, e.proj, "link", e.cat)
	if !exists(e.proj, "setup.local.yaml") {
		t.Fatal("link must write setup.local.yaml")
	}
	if code, _, _ := run(t, e.proj, "check"); code != 1 {
		t.Fatal("check must fail after link")
	}
}

func TestMediumLinkFindingsAreShownButNeverBlock(t *testing.T) {
	e := newDevEnv(t)
	write(t, e.work, "bundles/finance/skills/money/SKILL.md", skillMD("money")+"\nSee [shared](../../SHARED.md).\n")

	code, out, errs := run(t, e.work, "bundle", "check")
	if code != 0 || !strings.Contains(out, "medium [skill-link] money/SKILL.md") {
		t.Fatalf("bundle check: exit %d\nstdout: %s\nstderr: %s", code, out, errs)
	}
	code, _, errs = run(t, e.proj, "update")
	if code != 0 || !strings.Contains(errs, "medium [skill-link] money/SKILL.md") {
		t.Fatalf("dev update: exit %d\nstderr: %s", code, errs)
	}
}
