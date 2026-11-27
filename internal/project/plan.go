package project

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/cychiang/setup/internal/archive"
	"github.com/cychiang/setup/internal/catalog"
)

// Instruction files that carry managed blocks.
var BlockFiles = []string{"AGENTS.md", "CLAUDE.md"}

// Skill roots, one per agent family. Both get real copies.
var SkillRoots = []string{".claude/skills", ".agents/skills"}

type PlanFile struct {
	Data   []byte
	Bundle string
}

type Block struct {
	Bundle string
	Body   string
}

// Plan is the desired tree: whole files by path and managed blocks in layer order.
type Plan struct {
	Files  map[string]PlanFile
	Blocks []Block
}

func isBlockFile(p string) bool {
	for _, f := range BlockFiles {
		if f == p {
			return true
		}
	}
	return false
}

// MakePlan merges bundles by the fixed merge rules. It is pure.
func MakePlan(contents []archive.Content, disable []string) (Plan, error) {
	cs := append([]archive.Content(nil), contents...)
	sort.Slice(cs, func(i, j int) bool {
		a, b := cs[i].Manifest, cs[j].Manifest
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		return a.Name < b.Name
	})
	hasBase := false
	byName := map[string]archive.Content{}
	for _, c := range cs {
		n := c.Manifest.Name
		if _, dup := byName[n]; dup {
			return Plan{}, fmt.Errorf("bundle %s selected twice", n)
		}
		byName[n] = c
		hasBase = hasBase || c.Manifest.Layer == catalog.Base
	}
	if !hasBase {
		return Plan{}, fmt.Errorf("the base bundle must always be included")
	}

	off := map[string]bool{}
	for _, d := range disable {
		parts := strings.SplitN(d, "/", 3)
		c, ok := byName[parts[0]]
		if len(parts) != 3 || !ok {
			return Plan{}, fmt.Errorf("unknown disable entry %q", d)
		}
		switch parts[1] {
		case "skills":
			_, ok = c.Skills[parts[2]]
		case "instructions":
			_, ok = c.Instructions[parts[2]]
		default:
			ok = false
		}
		if !ok {
			return Plan{}, fmt.Errorf("unknown disable entry %q", d)
		}
		off[d] = true
	}

	p := Plan{Files: map[string]PlanFile{}}
	owner := map[string]string{}
	for _, c := range cs {
		name := c.Manifest.Name
		for skill, files := range c.Skills {
			if prev, ok := owner[skill]; ok {
				return Plan{}, fmt.Errorf("skill %q is provided by both bundle %s and bundle %s", skill, prev, name)
			}
			owner[skill] = name
			if off[name+"/skills/"+skill] {
				continue
			}
			for rel, data := range files {
				for _, root := range SkillRoots {
					p.Files[path.Join(root, skill, rel)] = PlanFile{Data: data, Bundle: name}
				}
			}
		}
		var parts []string
		for _, f := range c.Instructions.Paths() {
			if !off[name+"/instructions/"+f] {
				parts = append(parts, strings.TrimSpace(string(c.Instructions[f])))
			}
		}
		if len(parts) > 0 {
			p.Blocks = append(p.Blocks, Block{Bundle: name, Body: strings.Join(parts, "\n\n")})
		}
	}
	return p, nil
}
