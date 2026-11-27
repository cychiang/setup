package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/cychiang/setup/internal/available"
	"github.com/cychiang/setup/internal/catalog"
)

const descWidth = 100

func skillsCmd(args []string, env Env) error {
	fs := newFlags("skills", env)
	catFlag := fs.String("catalog", "", "catalog root (default ./bundles)")
	source := fs.String("source", "", "only list skills of this source")
	query := fs.String("query", "", "case-insensitive filter over name, path and description")
	asJSON := fs.Bool("json", false, "machine-readable output")
	asMD := fs.Bool("markdown", false, "Markdown table with full descriptions")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 {
		return usageError("skills takes flags only")
	}
	if *asJSON && *asMD {
		return usageError("skills: use either --json or --markdown")
	}
	c, err := loadCatalog(env, *catFlag, false)
	if err != nil {
		return err
	}
	f, err := gitFetcher(env, c)
	if err != nil {
		return err
	}
	roots, warnings, err := available.Roots(c, f, *source)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(env.Err, "warning:", w)
	}
	list, err := available.List(c, roots, *source)
	if err != nil {
		return err
	}
	list = available.Filter(list, *query)
	switch {
	case *asJSON:
		if list == nil {
			list = []available.Skill{}
		}
		return printJSON(env.Out, list)
	case *asMD:
		writeSkillsMarkdown(env.Out, list)
	default:
		writeSkillsTable(env.Out, list)
	}
	return nil
}

func writeSkillsTable(w io.Writer, list []available.Skill) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "REF\tNAME\tUSED BY\tDESCRIPTION")
	for _, s := range list {
		name := s.Name
		if !s.Valid {
			name += " (invalid)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Ref, name, dash(strings.Join(s.UsedBy, ",")), truncate(s.Description, descWidth))
	}
	tw.Flush()
}

func writeSkillsMarkdown(w io.Writer, list []available.Skill) {
	fmt.Fprintln(w, "| Ref | Name | Used by | Description |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, s := range list {
		name := s.Name
		if !s.Valid {
			name += " (invalid)"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", mdCell("`"+s.Ref+"`"), mdCell(name), mdCell(dash(strings.Join(s.UsedBy, ", "))), mdCell(s.Description))
	}
}

func mdCell(s string) string {
	return strings.ReplaceAll(oneLine(s), "|", `\|`)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	s = oneLine(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}

type bundleInfo struct {
	Name        string   `json:"name"`
	Layer       string   `json:"layer"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Skills      []string `json:"skills"`
}

func bundleList(env Env, catFlag string, asJSON bool) error {
	c, err := loadCatalog(env, catFlag, false)
	if err != nil {
		return err
	}
	infos := []bundleInfo{}
	for _, b := range c.Bundles {
		info := bundleInfo{Name: b.Name, Layer: b.Layer.String(), Version: b.Version.String(), Description: oneLine(b.Description), Skills: []string{}}
		for _, s := range b.Skills {
			info.Skills = append(info.Skills, catalog.Ref(b.Name, s))
		}
		infos = append(infos, info)
	}
	if asJSON {
		return printJSON(env.Out, infos)
	}
	tw := tabwriter.NewWriter(env.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLAYER\tVERSION\tDESCRIPTION\tSKILLS")
	for _, b := range infos {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", b.Name, b.Layer, b.Version, truncate(b.Description, descWidth), dash(strings.Join(b.Skills, ", ")))
	}
	return tw.Flush()
}
