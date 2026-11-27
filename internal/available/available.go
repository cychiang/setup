// Package available lists every skill a catalog can pick from: skills inside
// registered upstream sources and the catalog's own local skills.
package available

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/fetch"
)

// Skill is one pickable skill. Ref is accepted verbatim by `setup bundle add`
// for upstream skills.
type Skill struct {
	Ref         string   `json:"ref"`
	Source      string   `json:"source"`
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	UsedBy      []string `json:"used_by"`
	Valid       bool     `json:"valid"`
}

// Roots materializes each registered, pinned source (only `only` when set).
// A registered source without a pin yields a warning rather than an error.
func Roots(cat *catalog.Catalog, f fetch.Fetcher, only string) (roots map[string]string, warnings []string, err error) {
	if only != "" {
		if _, ok := cat.Sources[only]; !ok {
			return nil, nil, fmt.Errorf("unknown source %q", only)
		}
	}
	roots = map[string]string{}
	names := make([]string, 0, len(cat.Sources))
	for n := range cat.Sources {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if only != "" && n != only {
			continue
		}
		commit, ok := cat.Pins[n]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("source %s is registered but not pinned; run `setup source sync %s`", n, n))
			continue
		}
		dir, err := f.Materialize(n, commit)
		if err != nil {
			return nil, nil, err
		}
		roots[n] = dir
	}
	return roots, warnings, nil
}

// List walks the materialized source trees and the catalog's local skill
// folders. Local skills are skipped when onlySource is set.
func List(cat *catalog.Catalog, roots map[string]string, onlySource string) ([]Skill, error) {
	used := usage(cat)
	var out []Skill
	for src, root := range roots {
		found, err := walk(root)
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", src, err)
		}
		for _, rel := range found {
			ref := src + "/" + rel
			out = append(out, parse(Skill{Ref: ref, Source: src, Path: rel}, filepath.Join(root, filepath.FromSlash(rel)), used[ref]))
		}
	}
	if onlySource == "" {
		for _, b := range cat.Bundles {
			dir := filepath.Join(cat.BundleDir(b.Name), "skills")
			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err != nil {
					continue
				}
				ref := catalog.Ref(b.Name, catalog.Local{Name: e.Name()})
				out = append(out, parse(Skill{Ref: ref, Path: e.Name()}, filepath.Join(dir, e.Name()), used[ref]))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// Filter keeps skills whose name, path or description contains q, ignoring case.
func Filter(skills []Skill, q string) []Skill {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return skills
	}
	var out []Skill
	for _, s := range skills {
		if strings.Contains(strings.ToLower(s.Name+"\n"+s.Path+"\n"+s.Description), q) {
			out = append(out, s)
		}
	}
	return out
}

func usage(cat *catalog.Catalog) map[string][]string {
	used := map[string][]string{}
	for _, b := range cat.Bundles {
		for _, s := range b.Skills {
			ref := catalog.Ref(b.Name, s)
			used[ref] = append(used[ref], b.Name)
		}
	}
	return used
}

// walk returns the slash paths of directories holding a SKILL.md. A skill
// folder is self-contained, so the walk does not descend into it.
func walk(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		if p == root {
			return nil
		}
		if st, err := os.Stat(filepath.Join(p, "SKILL.md")); err == nil && st.Mode().IsRegular() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			found = append(found, path.Clean(filepath.ToSlash(rel)))
			return filepath.SkipDir
		}
		return nil
	})
	return found, err
}

func parse(s Skill, dir string, usedBy []string) Skill {
	s.UsedBy = usedBy
	if s.UsedBy == nil {
		s.UsedBy = []string{}
	}
	s.Name = path.Base(s.Path)
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return s
	}
	fm, ok := frontmatter(data)
	if !ok {
		return s
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return s
	}
	if meta.Name != "" {
		s.Name = meta.Name
	}
	s.Description = strings.TrimSpace(meta.Description)
	s.Valid = meta.Name != "" && s.Description != ""
	return s
}

func frontmatter(data []byte) ([]byte, bool) {
	data = bytes.ReplaceAll(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		return nil, false
	}
	fm, _, ok := bytes.Cut(rest, []byte("\n---"))
	return fm, ok
}
