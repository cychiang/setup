package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/cychiang/setup/internal/tree"
)

// Report says what an apply changed. A re-run with nothing to do leaves every list empty.
type Report struct {
	Written   []string
	Deleted   []string
	Conflicts []string
}

var ErrConflicts = errors.New("hand-edited files would be overwritten; rerun with --force to replace them")

// unit is one thing the lock tracks: a whole file, or one managed block of AGENTS.md/CLAUDE.md.
type unit struct {
	path, bundle string
	want         string // desired content; ignored when !present
	wantHash     string
	present      bool // desired
}

func unitKey(path, bundle string) string {
	if isBlockFile(path) {
		return path + "#" + bundle
	}
	return path
}

// disk reads the current content of a unit. ok is false when it is absent.
func disk(root string, u unit) (content string, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(u.path)))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !isBlockFile(u.path) {
		return string(data), true, nil
	}
	segs, err := parseSegs(string(data))
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", u.path, err)
	}
	body, ok := blockBodies(segs)[u.bundle]
	return body, ok, nil
}

type action int

const (
	keep action = iota
	write
	remove
)

type step struct {
	unit
	do action
}

// Apply reconciles the plan against disk and the previous lock. Nothing is
// written when conflicts exist and force is false.
func Apply(root string, plan Plan, old Lock, force bool) (Report, []LockFile, error) {
	var units []unit
	for p, f := range plan.Files {
		units = append(units, unit{path: p, bundle: f.Bundle, want: string(f.Data), wantHash: tree.SHA(f.Data), present: true})
	}
	for _, b := range plan.Blocks {
		for _, f := range BlockFiles {
			units = append(units, unit{path: f, bundle: b.Bundle, want: b.Body, wantHash: tree.SHA([]byte(b.Body)), present: true})
		}
	}
	oldIdx := map[string]LockFile{}
	for _, f := range old.Files {
		oldIdx[unitKey(f.Path, f.Bundle)] = f
	}
	desired := map[string]bool{}
	for _, u := range units {
		desired[unitKey(u.path, u.bundle)] = true
	}
	for _, f := range old.Files {
		if !desired[unitKey(f.Path, f.Bundle)] {
			units = append(units, unit{path: f.Path, bundle: f.Bundle})
		}
	}
	sort.Slice(units, func(i, j int) bool {
		return unitKey(units[i].path, units[i].bundle) < unitKey(units[j].path, units[j].bundle)
	})

	var rep Report
	var steps []step
	var newLock []LockFile
	for _, u := range units {
		cur, onDisk, err := disk(root, u)
		if err != nil {
			return rep, nil, err
		}
		curHash := tree.SHA([]byte(cur))
		prev, locked := oldIdx[unitKey(u.path, u.bundle)]
		clean := locked && onDisk && prev.SHA256 == curHash
		if u.present {
			newLock = append(newLock, LockFile{Path: u.path, SHA256: u.wantHash, Bundle: u.bundle})
			switch {
			case !onDisk:
				steps = append(steps, step{u, write})
			case curHash == u.wantHash:
			case clean:
				steps = append(steps, step{u, write})
			default:
				rep.Conflicts = append(rep.Conflicts, u.path)
				steps = append(steps, step{u, write})
			}
			continue
		}
		switch {
		case !onDisk:
		case clean:
			steps = append(steps, step{u, remove})
		default:
			rep.Conflicts = append(rep.Conflicts, u.path)
			steps = append(steps, step{u, remove})
		}
	}
	if len(rep.Conflicts) > 0 && !force {
		return rep, nil, ErrConflicts
	}
	rep.Conflicts = nil

	order := make([]string, len(plan.Blocks))
	for i, b := range plan.Blocks {
		order[i] = b.Bundle
	}
	type blockEdit struct {
		set    []Block
		remove map[string]bool
	}
	edits := map[string]*blockEdit{}
	for _, s := range steps {
		if isBlockFile(s.path) {
			e := edits[s.path]
			if e == nil {
				e = &blockEdit{remove: map[string]bool{}}
				edits[s.path] = e
			}
			if s.do == write {
				e.set = append(e.set, Block{Bundle: s.bundle, Body: s.want})
			} else {
				e.remove[s.bundle] = true
			}
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(s.path))
		if s.do == write {
			if err := writeAtomic(full, []byte(s.want)); err != nil {
				return rep, nil, err
			}
			rep.Written = append(rep.Written, s.path)
		} else {
			if err := os.Remove(full); err != nil {
				return rep, nil, err
			}
			pruneEmpty(root, filepath.Dir(full))
			rep.Deleted = append(rep.Deleted, s.path)
		}
	}
	for _, f := range BlockFiles {
		e := edits[f]
		if e == nil {
			continue
		}
		full := filepath.Join(root, f)
		data, err := os.ReadFile(full)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return rep, nil, err
		}
		out, err := editBlocks(string(data), e.set, e.remove, order)
		if err != nil {
			return rep, nil, fmt.Errorf("%s: %w", f, err)
		}
		switch {
		case out == string(data):
		case out == "":
			if err := os.Remove(full); err != nil {
				return rep, nil, err
			}
			rep.Deleted = append(rep.Deleted, f)
		default:
			if err := writeAtomic(full, []byte(out)); err != nil {
				return rep, nil, err
			}
			rep.Written = append(rep.Written, f)
		}
	}
	sort.Strings(rep.Written)
	sort.Strings(rep.Deleted)
	return rep, newLock, nil
}

func pruneEmpty(root, dir string) {
	for dir != root && len(dir) > len(root) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// Drift lists generated content that no longer matches the lock.
func Drift(root string, lock Lock) ([]string, error) {
	var out []string
	for _, f := range lock.Files {
		cur, ok, err := disk(root, unit{path: f.Path, bundle: f.Bundle})
		if err != nil {
			return nil, err
		}
		label := f.Path
		if isBlockFile(f.Path) {
			label = fmt.Sprintf("%s (block %s)", f.Path, f.Bundle)
		}
		switch {
		case !ok:
			out = append(out, label+": missing")
		case tree.SHA([]byte(cur)) != f.SHA256:
			out = append(out, label+": modified")
		}
	}
	return out, nil
}
