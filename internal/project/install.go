package project

import (
	"fmt"
	"strings"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/semver"
	"github.com/cychiang/setup/internal/store"
)

type Options struct {
	Force bool
	// Upgrade names bundles to move to their latest version; UpgradeAll covers every bundle.
	Upgrade    []string
	UpgradeAll bool
	// Warn receives non-blocking dev build warnings, such as external scanner notes.
	Warn func(string)
}

type Result struct {
	Bundles []LockBundle
	Report  Report
	Lock    Lock
}

func (o Options) upgrades(name string) bool {
	if o.UpgradeAll {
		return true
	}
	for _, n := range o.Upgrade {
		if n == name {
			return true
		}
	}
	return false
}

// Install resolves the configured bundles from the source, plans, applies, and writes setup.lock.
// Released bundles already in the lock stay at their locked version unless named in opt.
// A Dev source always builds from the working catalog, and dev lock entries are never pins.
func Install(root string, cfg Config, src store.Source, opt Options) (Result, error) {
	lock, err := LoadLock(root)
	if err != nil {
		return Result{}, err
	}
	var contents []archive.Content
	var locked []LockBundle
	switch src := src.(type) {
	case *store.Dev:
		contents, locked, err = fromDev(src, WithBase(cfg.Bundles), opt.Warn)
	case store.Store:
		contents, locked, err = fromArchives(src, WithBase(cfg.Bundles), lock, opt)
	default:
		err = fmt.Errorf("unsupported bundle source %T", src)
	}
	if err != nil {
		return Result{}, err
	}
	plan, err := MakePlan(contents, cfg.Disable)
	if err != nil {
		return Result{}, err
	}
	rep, files, err := Apply(root, plan, lock, opt.Force)
	if err != nil {
		return Result{Report: rep}, err
	}
	newLock := Lock{FormatVersion: LockFormatVersion, Bundles: locked, Files: files}
	if err := saveLock(root, newLock); err != nil {
		return Result{}, err
	}
	return Result{Bundles: locked, Report: rep, Lock: newLock}, nil
}

func fromDev(dev *store.Dev, names []string, warn func(string)) ([]archive.Content, []LockBundle, error) {
	if warn == nil {
		warn = func(string) {}
	}
	built, err := dev.Build(names, warn)
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]archive.Content{}
	for _, c := range built {
		byName[c.Manifest.Name] = c
	}
	var contents []archive.Content
	var locked []LockBundle
	for _, name := range names {
		c, ok := byName[name]
		if !ok {
			return nil, nil, fmt.Errorf("bundle %q not found in dev catalog %s", name, dev.Dir)
		}
		contents = append(contents, c)
		locked = append(locked, LockBundle{Name: name, Version: c.Manifest.Version, Source: SourceDev})
	}
	return contents, locked, nil
}

func fromArchives(st store.Store, names []string, lock Lock, opt Options) ([]archive.Content, []LockBundle, error) {
	var contents []archive.Content
	var locked []LockBundle
	for _, name := range names {
		prev, pinned := lock.Bundle(name)
		pinned = pinned && !prev.Dev()
		var v semver.Version
		if pinned && !opt.upgrades(name) {
			v = prev.Version
		} else {
			var err error
			if v, err = store.Latest(st, name); err != nil {
				return nil, nil, err
			}
		}
		data, sum, err := st.Fetch(name, v)
		if err != nil {
			return nil, nil, fmt.Errorf("bundle %s %s: %w", name, v, err)
		}
		if pinned && prev.Version == v && prev.ArchiveSHA256 != sum {
			return nil, nil, fmt.Errorf("bundle %s %s: archive checksum differs from setup.lock", name, v)
		}
		c, err := archive.Read(data)
		if err != nil {
			return nil, nil, fmt.Errorf("bundle %s %s: %w", name, v, err)
		}
		if c.Manifest.Name != name || c.Manifest.Version != v {
			return nil, nil, fmt.Errorf("bundle %s %s: archive manifest says %s %s", name, v, c.Manifest.Name, c.Manifest.Version)
		}
		contents = append(contents, c)
		locked = append(locked, LockBundle{Name: name, Version: v, ArchiveSHA256: sum})
	}
	return contents, locked, nil
}

type Update struct {
	Name    string         `json:"name"`
	Current semver.Version `json:"current"`
	Latest  semver.Version `json:"latest"`
}

// Outdated lists released bundles with a newer version. Dev bundles never count as updates.
func Outdated(lock Lock, st store.Store) ([]Update, error) {
	var out []Update
	for _, b := range lock.Bundles {
		if b.Dev() {
			continue
		}
		latest, err := store.Latest(st, b.Name)
		if err != nil {
			return nil, err
		}
		if latest.Compare(b.Version) > 0 {
			out = append(out, Update{b.Name, b.Version, latest})
		}
	}
	return out, nil
}

// SkillDirs lists installed skill directories (both agent roots) touched by the given file paths.
func SkillDirs(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		for _, root := range SkillRoots {
			rest, ok := strings.CutPrefix(p, root+"/")
			if !ok {
				continue
			}
			name, _, _ := strings.Cut(rest, "/")
			d := root + "/" + name
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out
}
