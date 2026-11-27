// Package tree models a set of files as slash-separated relative path -> bytes.
package tree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Tree map[string][]byte

func SHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (t Tree) Paths() []string {
	out := make([]string, 0, len(t))
	for p := range t {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Hash is a stable content hash over paths and file hashes.
func (t Tree) Hash() string {
	var sb strings.Builder
	for _, p := range t.Paths() {
		fmt.Fprintf(&sb, "%s\x00%s\n", p, SHA(t[p]))
	}
	return SHA([]byte(sb.String()))
}

// ValidPath reports whether p is a clean relative slash path with no traversal.
func ValidPath(p string) bool {
	if p == "" || strings.Contains(p, "\\") || strings.Contains(p, "\x00") {
		return false
	}
	return path.Clean(p) == p && !path.IsAbs(p) && p != ".." && !strings.HasPrefix(p, "../")
}

// ReadDir loads every regular file under dir. Symlinks are rejected so a
// third-party skill cannot smuggle in files from outside its directory.
func ReadDir(dir string) (Tree, error) {
	t := Tree{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file (symlinks are not allowed)", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		t[filepath.ToSlash(rel)] = b
		return nil
	})
	return t, err
}
