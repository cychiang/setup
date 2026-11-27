package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/project"
	"github.com/cychiang/setup/internal/store"
)

// listedBundle is one catalog bundle as `setup list` reports it.
type listedBundle struct {
	Name             string        `json:"name"`
	Layer            catalog.Layer `json:"layer"`
	LatestVersion    string        `json:"latest_version"`
	Description      string        `json:"description"`
	Skills           []string      `json:"skills"`
	Installed        bool          `json:"installed"`
	InstalledVersion string        `json:"installed_version,omitempty"`
}

func infoFromManifest(m archive.Manifest) listedBundle {
	info := listedBundle{Name: m.Name, Layer: m.Layer, LatestVersion: m.Version.String(), Description: m.Description, Skills: []string{}}
	for _, s := range m.Skills {
		info.Skills = append(info.Skills, s.Name)
	}
	return info
}

// catalogBundles reads the bundles a source offers. Released catalogs report the
// latest release's manifest; a dev catalog reports its working bundle.yaml files.
func catalogBundles(src store.Source) ([]listedBundle, error) {
	var out []listedBundle
	switch src := src.(type) {
	case *store.Dev:
		cat, err := catalog.Load(src.Dir)
		if err != nil {
			return nil, fmt.Errorf("dev catalog %s: %w", src.Dir, err)
		}
		for _, b := range cat.Bundles {
			info := listedBundle{Name: b.Name, Layer: b.Layer, LatestVersion: b.Version.String(), Description: b.Description, Skills: []string{}}
			for _, s := range b.Skills {
				info.Skills = append(info.Skills, s.SkillName())
			}
			out = append(out, info)
		}
	case store.Store:
		names, err := src.Bundles()
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			v, err := store.Latest(src, n)
			if err != nil {
				return nil, err
			}
			data, _, err := src.Fetch(n, v)
			if err != nil {
				return nil, fmt.Errorf("bundle %s %s: %w", n, v, err)
			}
			c, err := archive.Read(data)
			if err != nil {
				return nil, fmt.Errorf("bundle %s %s: %w", n, v, err)
			}
			out = append(out, infoFromManifest(c.Manifest))
		}
	default:
		return nil, fmt.Errorf("unsupported bundle source %T", src)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Layer != out[j].Layer {
			return out[i].Layer < out[j].Layer
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func listCmd(args []string, env Env) error {
	fs, f := newInitFlags("list", env)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 {
		return usageError("list takes flags only")
	}
	cfg, err := loadConfigOrNew(env)
	if err != nil {
		return err
	}
	r, err := resolveSource(env, cfg, f.catalog)
	if err != nil {
		return err
	}
	infos, err := catalogBundles(r.src)
	if err != nil {
		return err
	}
	lock, err := project.LoadLock(env.Dir)
	if err != nil {
		return err
	}
	for i := range infos {
		for _, b := range cfg.Bundles {
			if b == infos[i].Name {
				infos[i].Installed = true
			}
		}
		if b, ok := lock.Bundle(infos[i].Name); ok && infos[i].Installed {
			infos[i].InstalledVersion = b.Version.String()
		}
	}
	if infos == nil {
		infos = []listedBundle{}
	}
	if f.asJSON {
		return printJSON(env.Out, infos)
	}
	tw := tabwriter.NewWriter(env.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLAYER\tLATEST\tINSTALLED\tSKILLS\tDESCRIPTION")
	for _, b := range infos {
		installed := "-"
		if b.Installed {
			installed = "yes"
			if b.InstalledVersion != "" {
				installed = b.InstalledVersion
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", b.Name, b.Layer, b.LatestVersion, installed, strings.Join(b.Skills, ","), b.Description)
	}
	return tw.Flush()
}
