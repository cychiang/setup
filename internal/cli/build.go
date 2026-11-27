package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cychiang/setup/internal/build"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/fetch"
	"github.com/cychiang/setup/internal/semver"
)

func catalogDir(env Env, flagVal string) string {
	if flagVal == "" {
		flagVal = "bundles"
	}
	if filepath.IsAbs(flagVal) {
		return flagVal
	}
	return filepath.Join(env.Dir, flagVal)
}

func gitFetcher(env Env, cat *catalog.Catalog) (*fetch.Git, error) {
	dir, err := fetch.DefaultCacheDir(env.getenv)
	if err != nil {
		return nil, err
	}
	return &fetch.Git{CacheDir: dir, URLs: cat.Sources}, nil
}

func loadCatalog(env Env, flagVal string, create bool) (*catalog.Catalog, error) {
	dir := catalogDir(env, flagVal)
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return catalog.Load(dir)
}

func sourceCmd(args []string, env Env) error {
	if len(args) == 0 {
		return usageError("source: want add or sync")
	}
	fs := newFlags("source "+args[0], env)
	cat := fs.String("catalog", "", "catalog root (default ./bundles)")
	pos, err := parseFlags(fs, args[1:])
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if len(pos) != 2 {
			return usageError("source add <name> <git-url>")
		}
		c, err := loadCatalog(env, *cat, true)
		if err != nil {
			return err
		}
		if !catalog.ValidName(pos[0]) {
			return fmt.Errorf("invalid source name %q", pos[0])
		}
		if _, ok := c.Sources[pos[0]]; ok {
			return fmt.Errorf("source %q is already registered", pos[0])
		}
		c.Sources[pos[0]] = pos[1]
		if err := c.SaveSources(); err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "registered %s; run `setup source sync %s` to pin it\n", pos[0], pos[0])
		return nil
	case "sync":
		c, err := loadCatalog(env, *cat, false)
		if err != nil {
			return err
		}
		names := pos
		if len(names) == 0 {
			for n := range c.Sources {
				names = append(names, n)
			}
			sort.Strings(names)
		}
		f, err := gitFetcher(env, c)
		if err != nil {
			return err
		}
		for _, n := range names {
			url, ok := c.Sources[n]
			if !ok {
				return fmt.Errorf("unknown source %q", n)
			}
			commit, err := f.Sync(n, url)
			if err != nil {
				return err
			}
			if old := c.Pins[n]; old != commit {
				fmt.Fprintf(env.Out, "%s: %s -> %s\n", n, short(old), short(commit))
			} else {
				fmt.Fprintf(env.Out, "%s: up to date at %s\n", n, short(commit))
			}
			c.Pins[n] = commit
		}
		return c.SavePins()
	}
	return usageError("source: unknown subcommand " + args[0])
}

func short(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	if c == "" {
		return "(none)"
	}
	return c
}

func bundleCmd(args []string, env Env) error {
	if len(args) == 0 {
		return usageError("bundle: want new, add, remove, check or release")
	}
	sub := args[0]
	fs := newFlags("bundle "+sub, env)
	catFlag := fs.String("catalog", "", "catalog root (default ./bundles)")
	layer := fs.String("layer", "", "bundle layer (new)")
	local := fs.String("local", "", "local skill name (add)")
	publish := fs.Bool("publish", false, "create the GitHub release (release)")
	out := fs.String("out", "dist", "output directory (release)")
	allowMissing := fs.Bool("allow-missing-scanners", false, "release even if external scanners are missing")
	asJSON := fs.Bool("json", false, "machine-readable output (check)")
	pos, err := parseFlags(fs, args[1:])
	if err != nil {
		return err
	}
	switch sub {
	case "new":
		if len(pos) != 1 || *layer == "" {
			return usageError("bundle new <name> --layer base|domain|type")
		}
		l, err := catalog.ParseLayer(*layer)
		if err != nil {
			return err
		}
		c, err := loadCatalog(env, *catFlag, true)
		if err != nil {
			return err
		}
		if !catalog.ValidName(pos[0]) {
			return fmt.Errorf("invalid bundle name %q", pos[0])
		}
		if _, ok := c.Bundle(pos[0]); ok {
			return fmt.Errorf("bundle %q already exists", pos[0])
		}
		for _, d := range []string{"skills", "instructions"} {
			if err := os.MkdirAll(filepath.Join(c.BundleDir(pos[0]), d), 0o755); err != nil {
				return err
			}
		}
		return c.SaveBundle(catalog.Bundle{Name: pos[0], Layer: l, Version: semver.Version{Minor: 1}})
	case "add":
		return bundleAdd(env, *catFlag, *local, pos)
	case "remove":
		if len(pos) != 2 {
			return usageError("bundle remove <bundle> <skill>")
		}
		c, err := loadCatalog(env, *catFlag, false)
		if err != nil {
			return err
		}
		b, ok := c.Bundle(pos[0])
		if !ok {
			return fmt.Errorf("unknown bundle %q", pos[0])
		}
		var kept []catalog.SkillRef
		for _, s := range b.Skills {
			if s.SkillName() != pos[1] {
				kept = append(kept, s)
			}
		}
		if len(kept) == len(b.Skills) {
			return fmt.Errorf("bundle %s has no skill %q", pos[0], pos[1])
		}
		b.Skills = kept
		return c.SaveBundle(b)
	case "check":
		return bundleCheck(env, *catFlag, pos, *asJSON)
	case "release":
		if len(pos) != 1 {
			return usageError("bundle release <bundle> [--publish]")
		}
		return bundleRelease(env, *catFlag, pos[0], *out, *allowMissing, *publish)
	}
	return usageError("bundle: unknown subcommand " + sub)
}

func bundleAdd(env Env, catFlag, local string, pos []string) error {
	want := 2
	if local != "" {
		want = 1
	}
	if len(pos) != want {
		return usageError("bundle add <bundle> <source>/<path> | bundle add <bundle> --local <name>")
	}
	c, err := loadCatalog(env, catFlag, false)
	if err != nil {
		return err
	}
	b, ok := c.Bundle(pos[0])
	if !ok {
		return fmt.Errorf("unknown bundle %q", pos[0])
	}
	var ref catalog.SkillRef
	if local != "" {
		if _, err := os.Stat(filepath.Join(c.BundleDir(b.Name), "skills", local)); err != nil {
			return fmt.Errorf("no local skill folder bundles/%s/skills/%s", b.Name, local)
		}
		ref = catalog.Local{Name: local}
	} else {
		src, path, found := strings.Cut(pos[1], "/")
		if !found || path == "" {
			return usageError("expected <source>/<path>, got " + pos[1])
		}
		if _, ok := c.Sources[src]; !ok {
			return fmt.Errorf("source %q is not registered; run `setup source add`", src)
		}
		ref = catalog.Remote{Source: src, Path: path}
	}
	for _, s := range b.Skills {
		if s.SkillName() == ref.SkillName() {
			return fmt.Errorf("bundle %s already has skill %q", b.Name, ref.SkillName())
		}
	}
	b.Skills = append(b.Skills, ref)
	return c.SaveBundle(b)
}

func bundleCheck(env Env, catFlag string, names []string, asJSON bool) error {
	c, err := loadCatalog(env, catFlag, false)
	if err != nil {
		return err
	}
	f, err := gitFetcher(env, c)
	if err != nil {
		return err
	}
	rep, err := (&build.Builder{Catalog: c, Fetcher: f}).Check(names)
	if err != nil {
		return err
	}
	if asJSON {
		if err := printJSON(env.Out, map[string]any{"findings": rep.Findings, "warnings": rep.Warnings, "ok": !rep.Blocking()}); err != nil {
			return err
		}
	} else {
		for _, fd := range rep.Findings {
			fmt.Fprintln(env.Out, fd)
		}
		for _, w := range rep.Warnings {
			fmt.Fprintln(env.Err, "warning:", w)
		}
	}
	if rep.Blocking() {
		return errExit
	}
	return nil
}

func bundleRelease(env Env, catFlag, name, out string, allowMissing, publish bool) error {
	c, err := loadCatalog(env, catFlag, false)
	if err != nil {
		return err
	}
	f, err := gitFetcher(env, c)
	if err != nil {
		return err
	}
	mtime := time.Unix(0, 0)
	if s := env.getenv("SOURCE_DATE_EPOCH"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("SOURCE_DATE_EPOCH: %w", err)
		}
		mtime = time.Unix(n, 0)
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(env.Dir, out)
	}
	rel, rep, err := (&build.Builder{Catalog: c, Fetcher: f}).Release(name, build.ReleaseOptions{OutDir: out, AllowMissingScanners: allowMissing, Mtime: mtime})
	for _, fd := range rep.Findings {
		fmt.Fprintln(env.Err, fd)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "%s\nsha256 %s\n", rel.Archive, rel.SHA256)
	if publish {
		bd, _ := c.Bundle(name)
		return build.Publish(bd, rel)
	}
	return nil
}
