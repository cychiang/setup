// Package build assembles bundles from a catalog, gates them through the scanner, and releases archives.
package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cychiang/setup/tool/internal/archive"
	"github.com/cychiang/setup/tool/internal/catalog"
	"github.com/cychiang/setup/tool/internal/fetch"
	"github.com/cychiang/setup/tool/internal/scan"
	"github.com/cychiang/setup/tool/internal/tree"
)

type Builder struct {
	Catalog *catalog.Catalog
	Fetcher fetch.Fetcher
}

// Resolve loads every skill and instruction of a bundle, fetching pinned remote skills.
func (b *Builder) Resolve(bundle catalog.Bundle) (archive.Content, error) {
	skills := map[string]tree.Tree{}
	sources := map[string]archive.Source{}
	for _, ref := range bundle.Skills {
		var dir string
		var license []byte
		switch ref := ref.(type) {
		case catalog.Local:
			dir = filepath.Join(b.Catalog.BundleDir(bundle.Name), "skills", ref.Name)
		case catalog.Remote:
			if _, ok := b.Catalog.Sources[ref.Source]; !ok {
				return archive.Content{}, fmt.Errorf("bundle %s: source %q is not registered in sources.yaml", bundle.Name, ref.Source)
			}
			commit, ok := b.Catalog.Pins[ref.Source]
			if !ok {
				return archive.Content{}, fmt.Errorf("bundle %s: source %q has no pin; run `setup source sync %s`", bundle.Name, ref.Source, ref.Source)
			}
			root, err := b.Fetcher.Materialize(ref.Source, commit)
			if err != nil {
				return archive.Content{}, err
			}
			if !tree.ValidPath(ref.Path) {
				return archive.Content{}, fmt.Errorf("bundle %s: unsafe source path %q", bundle.Name, ref.Path)
			}
			dir = filepath.Join(root, filepath.FromSlash(ref.Path))
			license = findLicense(root)
			sources[ref.SkillName()] = archive.Source{Name: ref.Source, URL: b.Catalog.Sources[ref.Source], Commit: commit}
		}
		files, err := tree.ReadDir(dir)
		if err != nil {
			return archive.Content{}, fmt.Errorf("bundle %s: skill %s: %w", bundle.Name, ref.SkillName(), err)
		}
		if len(files) == 0 {
			return archive.Content{}, fmt.Errorf("bundle %s: skill %s: no files in %s", bundle.Name, ref.SkillName(), dir)
		}
		if license != nil {
			files["LICENSE.upstream"] = license
		}
		skills[ref.SkillName()] = files
	}
	instr, err := readInstructions(filepath.Join(b.Catalog.BundleDir(bundle.Name), "instructions"))
	if err != nil {
		return archive.Content{}, err
	}
	return archive.Assemble(bundle, skills, sources, instr), nil
}

// findLicense returns the source repo's top-level license so Apache-style attribution travels with the skill.
func findLicense(root string) []byte {
	for _, n := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "LICENCE"} {
		if data, err := os.ReadFile(filepath.Join(root, n)); err == nil {
			return data
		}
	}
	return nil
}

func readInstructions(dir string) (tree.Tree, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return tree.Tree{}, nil
	}
	if err != nil {
		return nil, err
	}
	t := tree.Tree{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		t[e.Name()] = data
	}
	return t, nil
}

type Report struct {
	Findings []scan.Finding
	Warnings []string
	// MissingScanners names optional external scanners that were not on PATH.
	MissingScanners []string
}

func (r Report) Blocking() bool { return scan.Blocking(r.Findings) }

// CheckNames enforces catalog-wide skill-name uniqueness.
func (b *Builder) CheckNames() error {
	owner := map[string]string{}
	for _, bd := range b.Catalog.Bundles {
		for _, s := range bd.Skills {
			if prev, ok := owner[s.SkillName()]; ok {
				return fmt.Errorf("skill %q is in both bundle %s and bundle %s", s.SkillName(), prev, bd.Name)
			}
			owner[s.SkillName()] = bd.Name
		}
	}
	return nil
}

// Check validates names and pins, then scans the named bundles (all when empty).
func (b *Builder) Check(names []string) (Report, error) {
	var rep Report
	if err := b.CheckNames(); err != nil {
		return rep, err
	}
	allow, err := scan.LoadAllow(filepath.Join(b.Catalog.Dir, "security-allow.yaml"))
	if err != nil {
		return rep, err
	}
	targets := b.Catalog.Bundles
	if len(names) > 0 {
		targets = nil
		for _, n := range names {
			bd, ok := b.Catalog.Bundle(n)
			if !ok {
				return rep, fmt.Errorf("unknown bundle %q", n)
			}
			targets = append(targets, bd)
		}
	}
	stage, err := os.MkdirTemp("", "setup-scan-")
	if err != nil {
		return rep, err
	}
	defer os.RemoveAll(stage)
	for _, bd := range targets {
		c, err := b.Resolve(bd)
		if err != nil {
			return rep, err
		}
		for _, name := range sortedKeys(c.Skills) {
			rep.Findings = append(rep.Findings, scan.Skill(name, c.Skills[name], allow)...)
			if err := stageSkill(stage, name, c.Skills[name]); err != nil {
				return rep, err
			}
		}
		for _, s := range bd.Suggests {
			if _, ok := b.Catalog.Bundle(s); !ok {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("bundle %s suggests unknown bundle %q", bd.Name, s))
			}
		}
	}
	ext, missing, err := scan.RunExternal(stage)
	if err != nil {
		return rep, err
	}
	rep.Findings = append(rep.Findings, ext...)
	rep.MissingScanners = missing
	for _, m := range missing {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("external scanner %s not found on PATH", m))
	}
	return rep, nil
}

func stageSkill(stage, name string, files tree.Tree) error {
	for p, data := range files {
		dst := filepath.Join(stage, name, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type ReleaseOptions struct {
	OutDir               string
	AllowMissingScanners bool
	Mtime                time.Time
}

type Released struct {
	Archive string
	SHA256  string
	SumFile string
}

// Release gates the bundle through Check and writes <bundle>-<version>.tar.gz and .sha256.
func (b *Builder) Release(name string, opt ReleaseOptions) (Released, Report, error) {
	bd, ok := b.Catalog.Bundle(name)
	if !ok {
		return Released{}, Report{}, fmt.Errorf("unknown bundle %q", name)
	}
	rep, err := b.Check([]string{name})
	if err != nil {
		return Released{}, rep, err
	}
	if rep.Blocking() {
		return Released{}, rep, fmt.Errorf("release blocked by %d high-severity finding(s)", countHigh(rep.Findings))
	}
	if len(rep.MissingScanners) > 0 && !opt.AllowMissingScanners {
		return Released{}, rep, fmt.Errorf("external scanner(s) missing: %s (install them or pass --allow-missing-scanners)", strings.Join(rep.MissingScanners, ", "))
	}
	c, err := b.Resolve(bd)
	if err != nil {
		return Released{}, rep, err
	}
	var buf bytes.Buffer
	if err := archive.Write(&buf, c, opt.Mtime); err != nil {
		return Released{}, rep, err
	}
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return Released{}, rep, err
	}
	file := archive.FileName(bd.Name, bd.Version)
	out := Released{Archive: filepath.Join(opt.OutDir, file), SHA256: tree.SHA(buf.Bytes())}
	out.SumFile = out.Archive + ".sha256"
	if err := os.WriteFile(out.Archive, buf.Bytes(), 0o644); err != nil {
		return Released{}, rep, err
	}
	if err := os.WriteFile(out.SumFile, []byte(out.SHA256+"  "+file+"\n"), 0o644); err != nil {
		return Released{}, rep, err
	}
	return out, rep, nil
}

func countHigh(fs []scan.Finding) int {
	n := 0
	for _, f := range fs {
		if f.Severity >= scan.High {
			n++
		}
	}
	return n
}

// Publish creates the GitHub release <bundle>/v<version> with the gh CLI.
func Publish(bd catalog.Bundle, r Released) error {
	tag := fmt.Sprintf("%s/v%s", bd.Name, bd.Version)
	cmd := exec.Command("gh", "release", "create", tag, r.Archive, r.SumFile, "--title", tag, "--notes", bd.Description)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh release create %s: %w: %s", tag, err, strings.TrimSpace(string(out)))
	}
	return nil
}
