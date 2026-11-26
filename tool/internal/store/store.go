// Package store reads released bundle archives from a directory or a GitHub repository.
package store

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cychiang/setup/tool/internal/semver"
)

// Store lists and downloads released archives. Fetch verifies the archive against its published checksum.
type Store interface {
	Versions(bundle string) ([]semver.Version, error)
	Fetch(bundle string, v semver.Version) (data []byte, sha256 string, err error)
}

// Open resolves a catalog spec: `github:owner/repo` or a directory of archives.
func Open(spec, baseDir string, getenv func(string) string) (Store, error) {
	if repo, ok := strings.CutPrefix(spec, "github:"); ok {
		if strings.Count(repo, "/") != 1 {
			return nil, fmt.Errorf("catalog %q: want github:owner/repo", spec)
		}
		token := getenv("GITHUB_TOKEN")
		if token == "" {
			token = getenv("GH_TOKEN")
		}
		return &GitHub{Repo: repo, Token: token, API: "https://api.github.com"}, nil
	}
	if spec == "" {
		return nil, fmt.Errorf("no catalog configured; pass --catalog <dir|github:owner/repo>")
	}
	if !filepath.IsAbs(spec) {
		spec = filepath.Join(baseDir, spec)
	}
	return &Dir{Path: spec}, nil
}

// Latest returns the highest version of a bundle.
func Latest(s Store, bundle string) (semver.Version, error) {
	vs, err := s.Versions(bundle)
	if err != nil {
		return semver.Version{}, err
	}
	if len(vs) == 0 {
		return semver.Version{}, fmt.Errorf("bundle %q not found in catalog", bundle)
	}
	best := vs[0]
	for _, v := range vs[1:] {
		if v.Compare(best) > 0 {
			best = v
		}
	}
	return best, nil
}

func parseChecksum(data []byte, want string) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", fmt.Errorf("malformed checksum file for %s", want)
	}
	return fields[0], nil
}
