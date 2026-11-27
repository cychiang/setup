package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

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
	fs.StringVar(&f.catalog, "catalog", "", "directory of release archives or github:owner/repo")
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

func install(env Env, cfg project.Config, flagCatalog string, f *initFlags, opt project.Options) error {
	if flagCatalog != "" {
		cfg.Catalog = flagCatalog
	}
	st, err := store.Open(cfg.Catalog, env.Dir, env.getenv)
	if err != nil {
		return err
	}
	opt.Force = f.force
	res, err := project.Install(env.Dir, cfg, st, opt)
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
	return report(env, res, f.asJSON)
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
	return install(env, cfg, f.catalog, f, project.Options{})
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
	return install(env, cfg, f.catalog, f, project.Options{})
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
	return install(env, cfg, f.catalog, f, project.Options{})
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
	return install(env, cfg, f.catalog, f, project.Options{Upgrade: pos, UpgradeAll: len(pos) == 0})
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
	if f.catalog != "" {
		cfg.Catalog = f.catalog
	}
	st, err := store.Open(cfg.Catalog, env.Dir, env.getenv)
	if err != nil {
		return err
	}
	lock, err := project.LoadLock(env.Dir)
	if err != nil {
		return err
	}
	ups, err := project.Outdated(lock, st)
	if err != nil {
		return err
	}
	if f.asJSON {
		if ups == nil {
			ups = []project.Update{}
		}
		if err := printJSON(env.Out, ups); err != nil {
			return err
		}
	} else if len(ups) == 0 {
		fmt.Fprintln(env.Out, "all bundles are up to date")
	}
	for _, u := range ups {
		if !f.asJSON {
			fmt.Fprintf(env.Out, "%s %s -> %s\n", u.Name, u.Current, u.Latest)
		}
	}
	if len(ups) > 0 {
		return errExit
	}
	return nil
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
	if f.asJSON {
		if drift == nil {
			drift = []string{}
		}
		if err := printJSON(env.Out, map[string]any{"ok": len(drift) == 0, "drift": drift}); err != nil {
			return err
		}
	} else {
		for _, d := range drift {
			fmt.Fprintln(env.Out, "drift:", d)
		}
		if len(drift) == 0 {
			fmt.Fprintln(env.Out, "ok")
		}
	}
	if len(drift) > 0 {
		return errExit
	}
	return nil
}
