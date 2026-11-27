package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/tree"
)

type listedBundleRow struct {
	Name             string   `json:"name"`
	Layer            string   `json:"layer"`
	LatestVersion    string   `json:"latest_version"`
	Description      string   `json:"description"`
	Skills           []string `json:"skills"`
	Installed        bool     `json:"installed"`
	InstalledVersion string   `json:"installed_version"`
}

func bundleListJSON(t *testing.T, dir string, args ...string) []listedBundleRow {
	t.Helper()
	var got []listedBundleRow
	out := mustRun(t, dir, append([]string{"list", "--json"}, args...)...)
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("list output is not JSON: %v\n%s", err, out)
	}
	return got
}

func publishArchive(t *testing.T, dir, name string, layer catalog.Layer, v semver.Version, desc, skill string) {
	t.Helper()
	c := archive.Assemble(
		catalog.Bundle{Name: name, Layer: layer, Version: v, Description: desc},
		map[string]tree.Tree{skill: {"SKILL.md": []byte(skillMD(skill))}}, nil, tree.Tree{"i.md": []byte("rules\n")})
	var b bytes.Buffer
	if err := archive.Write(&b, c, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, archive.FileName(name, v))
	os.WriteFile(f, b.Bytes(), 0o644)
	os.WriteFile(f+".sha256", []byte(tree.SHA(b.Bytes())+"  x\n"), 0o644)
}

func archiveCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	publishArchive(t, dir, "pdf-reports", catalog.Type, semver.Version{Minor: 1}, "Reports.", "pdf")
	publishArchive(t, dir, "finance", catalog.Domain, semver.Version{Minor: 1}, "Old finance.", "money")
	publishArchive(t, dir, "finance", catalog.Domain, semver.Version{Minor: 2}, "Finance.", "money")
	publishArchive(t, dir, "base", catalog.Base, semver.Version{Minor: 1}, "Base.", "workbench")
	return dir
}

func TestListOutsideAProjectShowsTheArchiveCatalogSortedByLayer(t *testing.T) {
	got := bundleListJSON(t, t.TempDir(), "--catalog", archiveCatalog(t))
	var names []string
	for _, b := range got {
		names = append(names, b.Name)
		if b.Installed || b.InstalledVersion != "" {
			t.Errorf("%s marked installed outside a project", b.Name)
		}
	}
	if len(got) != 3 || names[0] != "base" || names[1] != "finance" || names[2] != "pdf-reports" {
		t.Fatalf("names = %v", names)
	}
	f := got[1]
	if f.Layer != "domain" || f.LatestVersion != "0.2.0" || f.Description != "Finance." || len(f.Skills) != 1 || f.Skills[0] != "money" {
		t.Fatalf("finance = %+v", f)
	}
}

func TestListMarksInstalledBundlesWithTheirLockedVersion(t *testing.T) {
	cat := archiveCatalog(t)
	proj := t.TempDir()
	mustRun(t, proj, "init", "--bundles", "base,finance", "--catalog", cat, "--yes")
	publishArchive(t, cat, "finance", catalog.Domain, semver.Version{Minor: 3}, "Finance.", "money")
	got := bundleListJSON(t, proj)
	by := map[string]listedBundleRow{}
	for _, b := range got {
		by[b.Name] = b
	}
	if f := by["finance"]; !f.Installed || f.InstalledVersion != "0.2.0" || f.LatestVersion != "0.3.0" {
		t.Fatalf("finance = %+v", f)
	}
	if p := by["pdf-reports"]; p.Installed || p.InstalledVersion != "" {
		t.Fatalf("pdf-reports = %+v", p)
	}
}

func TestListReadsADevCatalogFromBundleYaml(t *testing.T) {
	e := newDevEnv(t)
	got := bundleListJSON(t, e.proj)
	if len(got) != 2 || got[0].Name != "base" || got[1].Name != "finance" {
		t.Fatalf("got %+v", got)
	}
	if !got[1].Installed || got[1].InstalledVersion != "0.1.0" || len(got[1].Skills) != 2 {
		t.Fatalf("finance = %+v", got[1])
	}
}

func TestListPrintsATable(t *testing.T) {
	out := mustRun(t, t.TempDir(), "list", "--catalog", archiveCatalog(t))
	for _, want := range []string{"NAME", "finance", "0.2.0", "Finance."} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
}
