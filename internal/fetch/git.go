package fetch

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var commitRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// Git implements Fetcher with the git CLI and a bare-clone cache.
type Git struct {
	// CacheDir holds one bare clone per source plus extracted checkouts.
	CacheDir string
	// URLs lets Materialize fetch a pinned commit missing from the cache.
	URLs map[string]string
}

// DefaultCacheDir honors SETUP_HOME and falls back to ~/.setup/sources.
func DefaultCacheDir(getenv func(string) string) (string, error) {
	if h := getenv("SETUP_HOME"); h != "" {
		return filepath.Join(h, "sources"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".setup", "sources"), nil
}

func (g *Git) bare(name string) string { return filepath.Join(g.CacheDir, name, "repo.git") }

func git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g *Git) Sync(name, url string) (string, error) {
	repo := g.bare(name)
	if _, err := os.Stat(repo); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(repo), 0o755); err != nil {
			return "", err
		}
		if _, err := git("clone", "--bare", "--quiet", url, repo); err != nil {
			return "", err
		}
	} else {
		if _, err := git("--git-dir", repo, "remote", "set-url", "origin", url); err != nil {
			return "", err
		}
		if _, err := git("--git-dir", repo, "fetch", "--quiet", "--force", "origin", "+refs/heads/*:refs/heads/*"); err != nil {
			return "", err
		}
	}
	out, err := git("--git-dir", repo, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *Git) Materialize(name, commit string) (string, error) {
	if !commitRe.MatchString(commit) {
		return "", fmt.Errorf("source %s: invalid pinned commit %q", name, commit)
	}
	dir := filepath.Join(g.CacheDir, name, "checkouts", commit)
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}
	repo := g.bare(name)
	if _, err := git("--git-dir", repo, "cat-file", "-e", commit+"^{commit}"); err != nil {
		url, ok := g.URLs[name]
		if !ok {
			return "", fmt.Errorf("source %s: commit %s not cached; run `setup source sync %s`", name, commit, name)
		}
		if _, err := g.Sync(name, url); err != nil {
			return "", err
		}
		if _, err := git("--git-dir", repo, "cat-file", "-e", commit+"^{commit}"); err != nil {
			return "", fmt.Errorf("source %s: pinned commit %s not found upstream", name, commit)
		}
	}
	out, err := git("--git-dir", repo, "archive", "--format=tar", commit)
	if err != nil {
		return "", err
	}
	tmp := dir + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := extract(out, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func extract(data []byte, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(hdr.Name))
		if rel, err := filepath.Rel(dir, target); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unsafe path %q in source archive", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			b, err := io.ReadAll(tr)
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, b, 0o644); err != nil {
				return err
			}
		}
	}
}
