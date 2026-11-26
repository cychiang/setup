package scan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/cychiang/setup/tool/internal/tree"
)

type Finding struct {
	RuleID   string   `json:"rule"`
	Severity Severity `json:"-"`
	Skill    string   `json:"skill"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Msg      string   `json:"msg"`
}

func (f Finding) String() string {
	return fmt.Sprintf("%s [%s] %s/%s:%d %s", f.Severity, f.RuleID, f.Skill, f.Path, f.Line, f.Msg)
}

// Allow suppresses one rule for one skill, only while the skill's content hash matches.
type Allow struct {
	Skill  string `yaml:"skill"`
	SHA256 string `yaml:"sha256"`
	Rule   string `yaml:"rule"`
	Reason string `yaml:"reason"`
}

func LoadAllow(path string) ([]Allow, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var allow []Allow
	if err := yaml.Unmarshal(data, &allow); err != nil {
		return nil, fmt.Errorf("security-allow.yaml: %w", err)
	}
	for i, a := range allow {
		if a.Skill == "" || a.SHA256 == "" || a.Rule == "" || a.Reason == "" {
			return nil, fmt.Errorf("security-allow.yaml: entry %d needs skill, sha256, rule and reason", i)
		}
	}
	return allow, nil
}

// Skill runs every built-in rule and drops findings covered by a current suppression.
func Skill(name string, files tree.Tree, allow []Allow) []Finding {
	hash := files.Hash()
	var out []Finding
	for _, r := range Rules {
		for _, h := range r.Check(name, files) {
			if suppressed(allow, name, hash, r.ID) {
				continue
			}
			out = append(out, Finding{RuleID: r.ID, Severity: r.Severity, Skill: name, Path: h.Path, Line: h.Line, Msg: h.Msg})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func suppressed(allow []Allow, skill, hash, rule string) bool {
	for _, a := range allow {
		if a.Skill == skill && a.SHA256 == hash && a.Rule == rule {
			return true
		}
	}
	return false
}

// Blocking reports whether any finding is high severity.
func Blocking(fs []Finding) bool {
	for _, f := range fs {
		if f.Severity >= High {
			return true
		}
	}
	return false
}
