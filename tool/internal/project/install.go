package project

import (
	"fmt"
	"strings"

	"github.com/cychiang/setup/tool/internal/archive"
	"github.com/cychiang/setup/tool/internal/semver"
	"github.com/cychiang/setup/tool/internal/store"
)

type Options struct {
	Force bool
	// Upgrade names bundles to move to their latest version; UpgradeAll covers every bundle.
	Upgrade    []string
	UpgradeAll bool
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

// Install resolves the configured bundles from the store, plans, applies, and writes setup.lock.
// Bundles already in the lock stay at their locked version unless named in opt.
func Install(root string, cfg Config, st store.Store, opt Options) (Result, error) {
	lock, err := LoadLock(root)
	if err != nil {
		return Result{}, err
	}
	var contents []archive.Content
	var locked []LockBundle
	for _, name := range WithBase(cfg.Bundles) {
		prev, pinned := lock.Bundle(name)
		var v semver.Version
		if pinned && !opt.upgrades(name) {
			v = prev.Version
		} else if v, err = store.Latest(st, name); err != nil {
			return Result{}, err
		}
		data, sum, err := st.Fetch(name, v)
		if err != nil {
			return Result{}, fmt.Errorf("bundle %s %s: %w", name, v, err)
		}
		if pinned && prev.Version == v && prev.ArchiveSHA256 != sum {
			return Result{}, fmt.Errorf("bundle %s %s: archive checksum differs from setup.lock", name, v)
		}
		c, err := archive.Read(data)
		if err != nil {
			return Result{}, fmt.Errorf("bundle %s %s: %w", name, v, err)
		}
		if c.Manifest.Name != name || c.Manifest.Version != v {
			return Result{}, fmt.Errorf("bundle %s %s: archive manifest says %s %s", name, v, c.Manifest.Name, c.Manifest.Version)
		}
		contents = append(contents, c)
		locked = append(locked, LockBundle{Name: name, Version: v, ArchiveSHA256: sum})
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

type Update struct {
	Name    string         `json:"name"`
	Current semver.Version `json:"current"`
	Latest  semver.Version `json:"latest"`
}

func Outdated(lock Lock, st store.Store) ([]Update, error) {
	var out []Update
	for _, b := range lock.Bundles {
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
