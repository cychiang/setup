// Package archive defines the release archive: the only interface between build and init mode.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/tree"
)

const FormatVersion = 1

// FileEntry paths are relative to the skill folder for skills and to the
// instructions folder for instructions.
type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Source records where a third-party skill came from.
type Source struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Commit string `json:"commit"`
}

type ManifestSkill struct {
	Name   string      `json:"name"`
	Source *Source     `json:"source,omitempty"`
	Files  []FileEntry `json:"files"`
}

type Manifest struct {
	FormatVersion int             `json:"format_version"`
	Name          string          `json:"name"`
	Layer         catalog.Layer   `json:"layer"`
	Version       semver.Version  `json:"version"`
	Suggests      []string        `json:"suggests"`
	Skills        []ManifestSkill `json:"skills"`
	Instructions  []FileEntry     `json:"instructions"`
}

// Content is a fully resolved bundle: the manifest plus every file it describes.
type Content struct {
	Manifest     Manifest
	Skills       map[string]tree.Tree // skill name -> files relative to the skill folder
	Instructions tree.Tree            // file name -> bytes
}

func entries(t tree.Tree) []FileEntry {
	out := []FileEntry{}
	for _, p := range t.Paths() {
		out = append(out, FileEntry{Path: p, SHA256: tree.SHA(t[p])})
	}
	return out
}

func Assemble(b catalog.Bundle, skills map[string]tree.Tree, sources map[string]Source, instructions tree.Tree) Content {
	m := Manifest{
		FormatVersion: FormatVersion,
		Name:          b.Name,
		Layer:         b.Layer,
		Version:       b.Version,
		Suggests:      append([]string{}, b.Suggests...),
		Skills:        []ManifestSkill{},
		Instructions:  entries(instructions),
	}
	names := make([]string, 0, len(skills))
	for n := range skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ms := ManifestSkill{Name: n, Files: entries(skills[n])}
		if src, ok := sources[n]; ok {
			ms.Source = &src
		}
		m.Skills = append(m.Skills, ms)
	}
	return Content{Manifest: m, Skills: skills, Instructions: instructions}
}

func (m Manifest) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (c Content) files() (tree.Tree, error) {
	mj, err := c.Manifest.JSON()
	if err != nil {
		return nil, err
	}
	all := tree.Tree{"manifest.json": mj}
	for n, files := range c.Skills {
		for p, b := range files {
			all[path.Join("skills", n, p)] = b
		}
	}
	for p, b := range c.Instructions {
		all[path.Join("instructions", p)] = b
	}
	return all, nil
}

// Write produces a byte-reproducible tar.gz: sorted entries, fixed times and
// owners, fixed modes, and a gzip header with no name or mtime.
func Write(w io.Writer, c Content, mtime time.Time) error {
	files, err := c.files()
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for p := range files {
		names[p] = false
		for d := path.Dir(p); d != "."; d = path.Dir(d) {
			names[d+"/"] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	for _, n := range sorted {
		hdr := &tar.Header{Name: n, ModTime: mtime, Format: tar.FormatUSTAR}
		if names[n] {
			hdr.Typeflag, hdr.Mode = tar.TypeDir, 0o755
		} else {
			hdr.Typeflag, hdr.Mode, hdr.Size = tar.TypeReg, 0o644, int64(len(files[n]))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !names[n] {
			if _, err := tw.Write(files[n]); err != nil {
				return err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// Read parses an archive and verifies every file against the manifest.
func Read(data []byte) (Content, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return Content{}, fmt.Errorf("archive: %w", err)
	}
	files := tree.Tree{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Content{}, fmt.Errorf("archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg || !tree.ValidPath(hdr.Name) {
			return Content{}, fmt.Errorf("archive: unsafe entry %q", hdr.Name)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return Content{}, fmt.Errorf("archive: %w", err)
		}
		files[hdr.Name] = b
	}
	mj, ok := files["manifest.json"]
	if !ok {
		return Content{}, fmt.Errorf("archive: manifest.json missing")
	}
	delete(files, "manifest.json")
	var m Manifest
	if err := json.Unmarshal(mj, &m); err != nil {
		return Content{}, fmt.Errorf("archive: manifest.json: %w", err)
	}
	if m.FormatVersion != FormatVersion {
		return Content{}, fmt.Errorf("archive: unsupported format_version %d (want %d)", m.FormatVersion, FormatVersion)
	}
	c := Content{Manifest: m, Skills: map[string]tree.Tree{}, Instructions: tree.Tree{}}
	claimed := map[string]bool{}
	verify := func(full string, e FileEntry) ([]byte, error) {
		if !tree.ValidPath(e.Path) {
			return nil, fmt.Errorf("archive: unsafe manifest path %q", e.Path)
		}
		b, ok := files[full]
		if !ok {
			return nil, fmt.Errorf("archive: %s listed in manifest but missing", full)
		}
		if tree.SHA(b) != e.SHA256 {
			return nil, fmt.Errorf("archive: %s does not match manifest hash", full)
		}
		claimed[full] = true
		return b, nil
	}
	for _, s := range m.Skills {
		if !catalog.ValidName(s.Name) {
			return Content{}, fmt.Errorf("archive: invalid skill name %q", s.Name)
		}
		t := tree.Tree{}
		for _, e := range s.Files {
			b, err := verify(path.Join("skills", s.Name, e.Path), e)
			if err != nil {
				return Content{}, err
			}
			t[e.Path] = b
		}
		c.Skills[s.Name] = t
	}
	for _, e := range m.Instructions {
		b, err := verify(path.Join("instructions", e.Path), e)
		if err != nil {
			return Content{}, err
		}
		c.Instructions[e.Path] = b
	}
	for p := range files {
		if !claimed[p] {
			return Content{}, fmt.Errorf("archive: %s is not listed in manifest", p)
		}
	}
	if strings.TrimSpace(m.Name) == "" {
		return Content{}, fmt.Errorf("archive: manifest has no name")
	}
	return c, nil
}

func FileName(name string, v semver.Version) string { return fmt.Sprintf("%s-%s.tar.gz", name, v) }
