// Package project plans and applies bundle installs into a project tree.
package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/cychiang/setup/tool/internal/semver"
)

const (
	ConfigFile = "setup.yaml"
	LockName   = "setup.lock"

	LockFormatVersion = 1
)

// Config is setup.yaml.
type Config struct {
	Catalog string   `yaml:"catalog"`
	Bundles []string `yaml:"bundles"`
	Disable []string `yaml:"disable,omitempty"`
}

func LoadConfig(root string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	return c, nil
}

func SaveConfig(root string, c Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(root, ConfigFile), data)
}

// WithBase returns bundles with base first and no duplicates, the rest sorted.
func WithBase(bundles []string) []string {
	seen := map[string]bool{"base": true}
	rest := []string{}
	for _, b := range bundles {
		if !seen[b] {
			seen[b] = true
			rest = append(rest, b)
		}
	}
	sort.Strings(rest)
	return append([]string{"base"}, rest...)
}

type LockBundle struct {
	Name          string         `json:"name"`
	Version       semver.Version `json:"version"`
	ArchiveSHA256 string         `json:"archive_sha256"`
}

// LockFile covers a whole file, or for AGENTS.md and CLAUDE.md the managed block of Bundle.
type LockFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bundle string `json:"bundle"`
}

// Lock is setup.lock.
type Lock struct {
	FormatVersion int          `json:"format_version"`
	Bundles       []LockBundle `json:"bundles"`
	Files         []LockFile   `json:"files"`
}

func (l Lock) JSON() ([]byte, error) {
	if l.Bundles == nil {
		l.Bundles = []LockBundle{}
	}
	if l.Files == nil {
		l.Files = []LockFile{}
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (l Lock) Bundle(name string) (LockBundle, bool) {
	for _, b := range l.Bundles {
		if b.Name == name {
			return b, true
		}
	}
	return LockBundle{}, false
}

// LoadLock returns an empty lock when the file does not exist.
func LoadLock(root string) (Lock, error) {
	data, err := os.ReadFile(filepath.Join(root, LockName))
	if errors.Is(err, fs.ErrNotExist) {
		return Lock{FormatVersion: LockFormatVersion}, nil
	}
	if err != nil {
		return Lock{}, err
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return Lock{}, fmt.Errorf("%s: %w", LockName, err)
	}
	if l.FormatVersion != LockFormatVersion {
		return Lock{}, fmt.Errorf("%s: unsupported format_version %d (want %d)", LockName, l.FormatVersion, LockFormatVersion)
	}
	return l, nil
}

func saveLock(root string, l Lock) error {
	data, err := l.JSON()
	if err != nil {
		return err
	}
	p := filepath.Join(root, LockName)
	if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, data) {
		return nil
	}
	return writeAtomic(p, data)
}

func writeAtomic(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".setup-tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
