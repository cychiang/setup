// Package catalog models the bundle catalog: bundles, registered sources and their pins.
package catalog

import (
	"bytes"
	"fmt"
	"path"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/cychiang/setup/internal/semver"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func ValidName(s string) bool { return len(s) <= 64 && nameRe.MatchString(s) }

// SkillRef is exactly one of Local or Remote.
type SkillRef interface {
	SkillName() string
	isSkillRef()
}

// Local names a skill stored in the bundle's skills/ folder.
type Local struct{ Name string }

// Remote names a skill folder inside a registered, pinned source.
type Remote struct{ Source, Path string }

func (l Local) SkillName() string  { return l.Name }
func (r Remote) SkillName() string { return path.Base(r.Path) }
func (Local) isSkillRef()          {}
func (Remote) isSkillRef()         {}

type Bundle struct {
	Name        string
	Layer       Layer
	Version     semver.Version
	Description string
	Suggests    []string
	Skills      []SkillRef
}

type rawSkill struct {
	Local  string `yaml:"local,omitempty"`
	Source string `yaml:"source,omitempty"`
	Path   string `yaml:"path,omitempty"`
}

type rawBundle struct {
	Name        string         `yaml:"name"`
	Layer       Layer          `yaml:"layer"`
	Version     semver.Version `yaml:"version"`
	Description string         `yaml:"description"`
	Suggests    []string       `yaml:"suggests"`
	Skills      []rawSkill     `yaml:"skills"`
}

func ParseBundle(data []byte) (Bundle, error) {
	var raw rawBundle
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return Bundle{}, fmt.Errorf("bundle.yaml: %w", err)
	}
	if !ValidName(raw.Name) {
		return Bundle{}, fmt.Errorf("bundle.yaml: invalid bundle name %q", raw.Name)
	}
	b := Bundle{Name: raw.Name, Layer: raw.Layer, Version: raw.Version, Description: raw.Description, Suggests: raw.Suggests}
	seen := map[string]bool{}
	for i, s := range raw.Skills {
		var ref SkillRef
		switch {
		case s.Local != "" && (s.Source != "" || s.Path != ""):
			return Bundle{}, fmt.Errorf("bundle %s: skills[%d]: use either local or source+path, not both", b.Name, i)
		case s.Local != "":
			ref = Local{s.Local}
		case s.Source != "" && s.Path != "":
			ref = Remote{s.Source, path.Clean(s.Path)}
		default:
			return Bundle{}, fmt.Errorf("bundle %s: skills[%d]: need local, or both source and path", b.Name, i)
		}
		if seen[ref.SkillName()] {
			return Bundle{}, fmt.Errorf("bundle %s: duplicate skill %q", b.Name, ref.SkillName())
		}
		seen[ref.SkillName()] = true
		b.Skills = append(b.Skills, ref)
	}
	return b, nil
}

func (b Bundle) Marshal() ([]byte, error) {
	raw := rawBundle{Name: b.Name, Layer: b.Layer, Version: b.Version, Description: b.Description, Suggests: b.Suggests}
	if raw.Suggests == nil {
		raw.Suggests = []string{}
	}
	raw.Skills = []rawSkill{}
	for _, s := range b.Skills {
		switch s := s.(type) {
		case Local:
			raw.Skills = append(raw.Skills, rawSkill{Local: s.Name})
		case Remote:
			raw.Skills = append(raw.Skills, rawSkill{Source: s.Source, Path: s.Path})
		}
	}
	return yaml.Marshal(raw)
}

// Ref formats a skill reference as the user types it: "<source>/<path>" for
// upstream skills and "local:<bundle>/<name>" for a bundle's own skills.
func Ref(bundle string, s SkillRef) string {
	switch s := s.(type) {
	case Local:
		return "local:" + bundle + "/" + s.Name
	case Remote:
		return s.Source + "/" + s.Path
	}
	return ""
}
