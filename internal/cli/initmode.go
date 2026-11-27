package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/cychiang/setup/internal/fetch"
	"github.com/cychiang/setup/internal/project"
	"github.com/cychiang/setup/internal/store"
)

type initFlags struct {
	catalog string
	bundles string
	asJSON  bool
	force   bool
}

func newInitFlags(name string, env Env) (*flag.FlagSet, *initFlags) {
	fs := newFlags(name, env)
	f := &initFlags{}
	fs.StringVar(&f.catalog, "catalog", "", "directory of release archives, github:owner/repo, or a source catalog to link in dev mode")
	fs.StringVar(&f.bundles, "bundles", "", "comma-separated bundle names")
	fs.BoolVar(&f.asJSON, "json", false, "machine-readable output")
	fs.BoolVar(&f.force, "force", false, "overwrite hand-edited generated files")
	fs.Bool("yes", false, "accepted for compatibility; commands never prompt")
	return fs, f
}

type installResult struct {
	Bundles      []project.LockBundle `json:"bundles"`
	Written      []string             `json:"written"`
	Deleted      []string             `json:"deleted"`
	Skills       []string             `json:"skills"`
	Instructions []string             `json:"instructions"`
	Note         string               `json:"activation_note"`
}

func activation(rep project.Report) (skills, instr []string, note string) {
	skills = project.SkillDirs(rep.Written)
	for _, p := range rep.Written {
		if slices.Contains(project.BlockFiles, p) {
			instr = append(instr, p)
		}
	}
	if len(skills) == 0 && len(instr) == 0 {
		return skills, instr, ""
	}
	var sb strings.Builder
	sb.WriteString("Activation note: new skills and instructions were written. A running agent does not\npick them up on its own, so read these now:\n")
	for _, s := range skills {
		if strings.HasPrefix(s, ".claude/") {
			sb.WriteString("  " + s + "/SKILL.md\n")
		}
	}
	for _, s := range skills {
		if strings.HasPrefix(s, ".agents/") {
			sb.WriteString("  " + s + "/SKILL.md\n")
		}
	}
	for _, p := range instr {
		sb.WriteString("  " + p + "\n")
	}
	return skills, instr, sb.String()
}

func report(env Env, res project.Result, asJSON bool) error {
	skills, instr, note := activation(res.Report)
	if asJSON {
		return printJSON(env.Out, installResult{
			Bundles: res.Bundles, Written: nonNil(res.Report.Written), Deleted: nonNil(res.Report.Deleted),
			Skills: nonNil(skills), Instructions: nonNil(instr), Note: note,
		})
	}
	for _, b := range res.Bundles {
		fmt.Fprintf(env.Out, "%s %s\n", b.Name, b.Version)
	}
	if len(res.Report.Written)+len(res.Report.Deleted) == 0 {
		fmt.Fprintln(env.Out, "nothing to change")
		return nil
	}
	fmt.Fprintf(env.Out, "wrote %d file(s), deleted %d\n", len(res.Report.Written), len(res.Report.Deleted))
	fmt.Fprint(env.Out, note)
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// resolved is a project's effective bundle source, decided once per command.
// link is the dev catalog to record in setup.local.yaml after a successful install.
type resolved struct {
	src     store.Source
	catalog string
	link    string
}

// resolveSource applies the precedence setup.local.yaml > --catalog > setup.yaml.
func resolveSource(env Env, cfg project.Config, flagCatalog string) (resolved, error) {
	local, err := project.LoadLocal(env.Dir)
	if err != nil {
		return resolved{}, err
	}
	if local.DevCatalog != "" {
		if !store.IsSourceCatalog(local.DevCatalog) {
			return resolved{}, fmt.Errorf("dev catalog %s is not a source catalog; fix it or run `setup unlink`", local.DevCatalog)
		}
		if flagCatalog != "" {
			fmt.Fprintf(env.Err, "note: dev link active (%s); ignoring --catalog\n", project.LocalFile)
		}
		cache, err := fetch.DefaultCacheDir(env.getenv)
		if err != nil {
			return resolved{}, err
		}
		return resolved{src: &store.Dev{Dir: local.DevCatalog, CacheDir: cache}, catalog: cfg.Catalog}, nil
	}
	return resolveSpec(env, cfg, flagCatalog)
}

// resolveSpec ignores setup.local.yaml: a source catalog spec enters dev mode, anything else is released.
func resolveSpec(env Env, cfg project.Config, spec string) (resolved, error) {
	spec = catalogSpec(spec, cfg.Catalog, env.getenv)
	cache, err := fetch.DefaultCacheDir(env.getenv)
	if err != nil {
		return resolved{}, err
	}
	dev, ok, err := store.DevAt(spec, env.Dir, cache)
	if err != nil {
		return resolved{}, err
	}
	if ok {
		return resolved{src: dev, catalog: cfg.Catalog, link: dev.Dir}, nil
	}
	st, err := store.Open(spec, env.Dir, env.getenv)
	if err != nil {
		return resolved{}, err
	}
	return resolved{src: st, catalog: spec}, nil
}

func install(env Env, cfg project.Config, r resolved, f *initFlags, opt project.Options) error {
	cfg.Catalog = r.catalog
	opt.Force = f.force
	opt.Warn = func(w string) { fmt.Fprintln(env.Err, "warning:", w) }
	res, err := project.Install(env.Dir, cfg, r.src, opt)
	if errors.Is(err, project.ErrConflicts) {
		for _, c := range res.Report.Conflicts {
			fmt.Fprintln(env.Err, "conflict (hand-edited):", c)
		}
	}
	if err != nil {
		return err
	}
	if err := project.SaveConfig(env.Dir, cfg); err != nil {
		return err
	}
	if r.link != "" {
		if err := project.LinkDev(env.Dir, r.link); err != nil {
			return err
		}
		fmt.Fprintf(env.Err, "dev link active: bundles come from %s; `setup check` fails until `setup unlink`\n", r.link)
	}
	return report(env, res, f.asJSON)
}

func installFrom(env Env, cfg project.Config, flagCatalog string, f *initFlags, opt project.Options) error {
	r, err := resolveSource(env, cfg, flagCatalog)
	if err != nil {
		return err
	}
	return install(env, cfg, r, f, opt)
}

func loadConfigOrNew(env Env) (project.Config, error) {
	cfg, err := project.LoadConfig(env.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return project.Config{}, nil
	}
	return cfg, err
}

func initCmd(args []string, env Env) error {
	fs, f := newInitFlags("init", env)
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfigOrNew(env)
	if err != nil {
		return err
	}
	if f.bundles != "" || len(cfg.Bundles) == 0 {
		cfg.Bundles = project.WithBase(splitList(f.bundles))
	}
	return installFrom(env, cfg, f.catalog, f, project.Options{})
}

func requireConfig(env Env) (project.Config, error) {
	cfg, err := project.LoadConfig(env.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("no %s here; run `setup init` first", project.ConfigFile)
	}
	return cfg, err
}

func addCmd(args []string, env Env) error {
	fs, f := newInitFlags("add", env)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usageError("add <bundle>...")
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	cfg.Bundles = project.WithBase(append(cfg.Bundles, pos...))
	return installFrom(env, cfg, f.catalog, f, project.Options{})
}

func removeCmd(args []string, env Env) error {
	fs, f := newInitFlags("remove", env)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usageError("remove <bundle>...")
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	for _, n := range pos {
		if n == "base" {
			return fmt.Errorf("base is always included and cannot be removed")
		}
		if !slices.Contains(cfg.Bundles, n) {
			return fmt.Errorf("bundle %q is not in %s", n, project.ConfigFile)
		}
		cfg.Bundles = slices.DeleteFunc(cfg.Bundles, func(b string) bool { return b == n })
		cfg.Disable = slices.DeleteFunc(cfg.Disable, func(d string) bool { return strings.HasPrefix(d, n+"/") })
	}
	return installFrom(env, cfg, f.catalog, f, project.Options{})
}

func updateCmd(args []string, env Env) error {
	fs, f := newInitFlags("update", env)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	return installFrom(env, cfg, f.catalog, f, project.Options{Upgrade: pos, UpgradeAll: len(pos) == 0})
}

func outdatedCmd(args []string, env Env) error {
	fs, f := newInitFlags("outdated", env)
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	lock, err := project.LoadLock(env.Dir)
	if err != nil {
		return err
	}
	var ups []project.Update
	if released := len(lock.Bundles) > len(lock.DevBundles()); released {
		r, err := resolveSource(env, cfg, f.catalog)
		if err != nil {
			return err
		}
		if st, ok := r.src.(store.Store); ok {
			if ups, err = project.Outdated(lock, st); err != nil {
				return err
			}
		}
	}
	dev := lock.DevBundles()
	if f.asJSON {
		if ups == nil {
			ups = []project.Update{}
		}
		if err := printJSON(env.Out, ups); err != nil {
			return err
		}
		for _, n := range dev {
			fmt.Fprintf(env.Err, "%s dev-linked\n", n)
		}
	} else {
		for _, u := range ups {
			fmt.Fprintf(env.Out, "%s %s -> %s\n", u.Name, u.Current, u.Latest)
		}
		for _, n := range dev {
			fmt.Fprintf(env.Out, "%s dev-linked\n", n)
		}
		if len(ups) == 0 && len(dev) == 0 {
			fmt.Fprintln(env.Out, "all bundles are up to date")
		}
	}
	if len(ups) > 0 {
		return errExit
	}
	return nil
}

func linkCmd(args []string, env Env) error {
	fs, f := newInitFlags("link", env)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageError("link <source-catalog-dir>")
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	r, err := resolveSpec(env, cfg, pos[0])
	if err != nil {
		return err
	}
	if r.link == "" {
		return fmt.Errorf("%s is not a source catalog (no sources.yaml or <bundle>/bundle.yaml)", pos[0])
	}
	return install(env, cfg, r, f, project.Options{UpgradeAll: true})
}

func unlinkCmd(args []string, env Env) error {
	fs, f := newInitFlags("unlink", env)
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	cfg, err := requireConfig(env)
	if err != nil {
		return err
	}
	spec := catalogSpec(f.catalog, cfg.Catalog, env.getenv)
	r, err := resolveSpec(env, cfg, spec)
	if err != nil {
		return err
	}
	if r.link != "" {
		return fmt.Errorf("%s is a source catalog; unlink needs a released catalog (archives or github:owner/repo)", spec)
	}
	if err := install(env, cfg, r, f, project.Options{}); err != nil {
		return err
	}
	return project.Unlink(env.Dir)
}

func checkCmd(args []string, env Env) error {
	fs, f := newInitFlags("check", env)
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	lock, err := project.LoadLock(env.Dir)
	if err != nil {
		return err
	}
	if len(lock.Bundles) == 0 {
		return fmt.Errorf("no %s here; run `setup init` first", project.LockName)
	}
	drift, err := project.Drift(env.Dir, lock)
	if err != nil {
		return err
	}
	dev := lock.DevBundles()
	if f.asJSON {
		if drift == nil {
			drift = []string{}
		}
		if err := printJSON(env.Out, map[string]any{"ok": len(drift) == 0 && len(dev) == 0, "drift": drift, "dev_linked": nonNil(dev)}); err != nil {
			return err
		}
	} else {
		for _, d := range drift {
			fmt.Fprintln(env.Out, "drift:", d)
		}
		if len(dev) > 0 {
			fmt.Fprintf(env.Out, "dev link active: %s built from a local catalog; run `setup unlink` before you commit or share\n", strings.Join(dev, ", "))
		}
		if len(drift) == 0 && len(dev) == 0 {
			fmt.Fprintln(env.Out, "ok")
		}
	}
	if len(drift) > 0 || len(dev) > 0 {
		return errExit
	}
	return nil
}
