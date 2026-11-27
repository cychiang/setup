package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	LocalFile  = "setup.local.yaml"
	gitignore  = ".gitignore"
	devSection = "dev_catalog"
)

// Local is setup.local.yaml: per-machine state that is never committed.
type Local struct {
	DevCatalog string `yaml:"dev_catalog,omitempty"`
}

// LoadLocal returns an empty Local when the file does not exist.
func LoadLocal(root string) (Local, error) {
	data, err := os.ReadFile(filepath.Join(root, LocalFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Local{}, nil
	}
	if err != nil {
		return Local{}, err
	}
	var l Local
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&l); err != nil && !errors.Is(err, io.EOF) {
		return Local{}, fmt.Errorf("%s: %w", LocalFile, err)
	}
	if l.DevCatalog != "" && !filepath.IsAbs(l.DevCatalog) {
		return Local{}, fmt.Errorf("%s: %s must be an absolute path", LocalFile, devSection)
	}
	return l, nil
}

// LinkDev records the dev catalog and keeps setup.local.yaml out of git.
func LinkDev(root, catalogDir string) error {
	data, err := yaml.Marshal(Local{DevCatalog: catalogDir})
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(root, LocalFile), data); err != nil {
		return err
	}
	return ignoreLocal(root)
}

// Unlink removes setup.local.yaml. It is not an error when the file is already gone.
func Unlink(root string) error {
	err := os.Remove(filepath.Join(root, LocalFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func ignoreLocal(root string) error {
	p := filepath.Join(root, gitignore)
	data, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == LocalFile {
			return nil
		}
	}
	out := string(data)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return writeAtomic(p, []byte(out+LocalFile+"\n"))
}
