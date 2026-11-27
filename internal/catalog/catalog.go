package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Catalog is the bundles folder plus its sources.yaml and sources.lock.
type Catalog struct {
	Dir     string
	Sources map[string]string // name -> git URL
	Pins    map[string]string // name -> commit
	Bundles []Bundle          // sorted by layer, then name
}

func (c *Catalog) Bundle(name string) (Bundle, bool) {
	for _, b := range c.Bundles {
		if b.Name == name {
			return b, true
		}
	}
	return Bundle{}, false
}

func (c *Catalog) BundleDir(name string) string { return filepath.Join(c.Dir, name) }

func Load(dir string) (*Catalog, error) {
	c := &Catalog{Dir: dir}
	var err error
	if c.Sources, err = readMap(filepath.Join(dir, "sources.yaml")); err != nil {
		return nil, err
	}
	if c.Pins, err = readMap(filepath.Join(dir, "sources.lock")); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), "bundle.yaml"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		b, err := ParseBundle(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if b.Name != e.Name() {
			return nil, fmt.Errorf("%s: bundle.yaml name %q does not match folder", e.Name(), b.Name)
		}
		c.Bundles = append(c.Bundles, b)
	}
	sort.Slice(c.Bundles, func(i, j int) bool {
		a, b := c.Bundles[i], c.Bundles[j]
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		return a.Name < b.Name
	})
	return c, nil
}

func (c *Catalog) SaveBundle(b Bundle) error {
	data, err := b.Marshal()
	if err != nil {
		return err
	}
	dir := c.BundleDir(b.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bundle.yaml"), data, 0o644)
}

func (c *Catalog) SaveSources() error {
	return writeMap(filepath.Join(c.Dir, "sources.yaml"), c.Sources)
}
func (c *Catalog) SavePins() error { return writeMap(filepath.Join(c.Dir, "sources.lock"), c.Pins) }

func readMap(p string) (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(p), err)
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

func writeMap(p string, m map[string]string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
