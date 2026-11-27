package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/tree"
)

// Dir is a directory holding <bundle>-<version>.tar.gz and .sha256 files.
type Dir struct{ Path string }

func (d *Dir) source() {}

func (d *Dir) Name() string { return d.Path }

func (d *Dir) Bundles() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(d.Path, "*.tar.gz"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, m := range matches {
		base := strings.TrimSuffix(filepath.Base(m), ".tar.gz")
		for i, c := range base {
			if c != '-' {
				continue
			}
			if _, err := semver.Parse(base[i+1:]); err == nil {
				names = append(names, base[:i])
				break
			}
		}
	}
	return uniqueSorted(names), nil
}

func (d *Dir) Versions(bundle string) ([]semver.Version, error) {
	matches, err := filepath.Glob(filepath.Join(d.Path, bundle+"-*.tar.gz"))
	if err != nil {
		return nil, err
	}
	var out []semver.Version
	for _, m := range matches {
		ver := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), bundle+"-"), ".tar.gz")
		// A bundle name may itself contain "-<digits>", so skip files that are another bundle's.
		if v, err := semver.Parse(ver); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

func (d *Dir) Fetch(bundle string, v semver.Version) ([]byte, string, error) {
	file := filepath.Join(d.Path, archive.FileName(bundle, v))
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	sum, err := os.ReadFile(file + ".sha256")
	if err != nil {
		return nil, "", fmt.Errorf("%s: checksum file: %w", filepath.Base(file), err)
	}
	want, err := parseChecksum(sum, filepath.Base(file))
	if err != nil {
		return nil, "", err
	}
	got := tree.SHA(data)
	if got != want {
		return nil, "", fmt.Errorf("%s: checksum mismatch (want %s, got %s)", filepath.Base(file), want, got)
	}
	return data, got, nil
}
