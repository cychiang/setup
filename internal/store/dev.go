package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/build"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/fetch"
)

// Dev is a source catalog (a checkout's bundles folder) built in memory on every
// call. It is only ever read, and its bundles are never released archives.
type Dev struct {
	Dir      string // absolute path of the source catalog
	CacheDir string // where pinned third-party sources are materialized
}

func (*Dev) source() {}

// Build gates the named bundles with the same rules as a release and returns them
// as the contents the planner consumes. Any blocking finding refuses all of them.
// External scanners only warn, through warn.
func (d *Dev) Build(names []string, warn func(string)) ([]archive.Content, error) {
	cat, err := catalog.Load(d.Dir)
	if err != nil {
		return nil, fmt.Errorf("dev catalog %s: %w", d.Dir, err)
	}
	b := &build.Builder{Catalog: cat, Fetcher: &fetch.Git{CacheDir: d.CacheDir, URLs: cat.Sources}}
	rep, contents, err := b.Gate(names, build.Advisory)
	if err != nil {
		return nil, err
	}
	for _, w := range rep.Warnings {
		warn(w)
	}
	if rep.Blocking() {
		var sb strings.Builder
		for _, f := range rep.Findings {
			fmt.Fprintf(&sb, "\n  %s", f)
		}
		return nil, fmt.Errorf("dev build refused by the security gate:%s", sb.String())
	}
	for _, f := range rep.Findings {
		warn(f.String())
	}
	return contents, nil
}

// IsSourceCatalog reports whether dir is a source catalog (sources.yaml, or a
// bundle folder with bundle.yaml) rather than a directory of release archives.
func IsSourceCatalog(dir string) bool {
	if archives, _ := filepath.Glob(filepath.Join(dir, "*.tar.gz")); len(archives) > 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "sources.yaml")); err == nil {
		return true
	}
	bundles, _ := filepath.Glob(filepath.Join(dir, "*", "bundle.yaml"))
	return len(bundles) > 0
}

// DevAt returns a Dev for a local directory spec resolved against baseDir, or
// ok=false when the spec is not a source catalog.
func DevAt(spec, baseDir, cacheDir string) (dev *Dev, ok bool, err error) {
	if strings.HasPrefix(spec, "github:") || spec == "" {
		return nil, false, nil
	}
	if !filepath.IsAbs(spec) {
		spec = filepath.Join(baseDir, spec)
	}
	abs, err := filepath.Abs(spec)
	if err != nil {
		return nil, false, err
	}
	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !IsSourceCatalog(abs) {
		return nil, false, nil
	}
	return &Dev{Dir: abs, CacheDir: cacheDir}, true, nil
}
