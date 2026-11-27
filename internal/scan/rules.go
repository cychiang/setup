// Package scan checks skills for content that could steer an agent maliciously.
package scan

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/cychiang/setup/internal/catalog"
	"github.com/cychiang/setup/internal/tree"
)

type Severity int

const (
	Low Severity = iota
	Medium
	High
)

func (s Severity) String() string { return [...]string{"low", "medium", "high"}[s] }

// Hit is a rule's raw match; the scanner attaches rule and skill.
type Hit struct {
	Path string
	Line int
	Msg  string
}

// Rule inspects every file of one skill, keyed by path relative to the skill folder.
type Rule struct {
	ID       string
	Severity Severity
	Check    func(skill string, files tree.Tree) []Hit
}

var Rules = []Rule{
	{ID: "hidden-unicode", Severity: High, Check: hiddenUnicode},
	{ID: "frontmatter", Severity: High, Check: frontmatter},
	{ID: "curl-pipe-shell", Severity: High, Check: lineRule(curlPipe, "downloads and pipes a script into a shell")},
	{ID: "base64-blob", Severity: High, Check: lineRule(base64Blob, "long base64 blob")},
	{ID: "command-preamble", Severity: High, Check: lineRule(preamble, "line starts with a !` command preamble")},
}

var (
	curlPipe   = regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n]*\|\s*(sudo\s+)?(ba|z|da)?sh\b`)
	base64Blob = regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)
	preamble   = regexp.MustCompile("^\\s*!`")
)

func isText(b []byte) bool { return utf8.Valid(b) && !bytes.Contains(b, []byte{0}) }

func lineRule(re *regexp.Regexp, msg string) func(string, tree.Tree) []Hit {
	return func(_ string, files tree.Tree) []Hit {
		var hits []Hit
		for _, p := range files.Paths() {
			if !isText(files[p]) {
				continue
			}
			for i, line := range strings.Split(string(files[p]), "\n") {
				if re.MatchString(line) {
					hits = append(hits, Hit{p, i + 1, msg})
				}
			}
		}
		return hits
	}
}

func hiddenSuspect(r rune, offset int) bool {
	switch {
	case r == 0xFEFF:
		return offset != 0
	case r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF:
		return true
	case r >= 0xE0000 && r <= 0xE007F:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return unicode.Is(unicode.Cf, r)
}

func hiddenUnicode(_ string, files tree.Tree) []Hit {
	var hits []Hit
	for _, p := range files.Paths() {
		if !isText(files[p]) {
			continue
		}
		line := 1
		for off, r := range string(files[p]) {
			if r == '\n' {
				line++
			}
			if hiddenSuspect(r, off) {
				hits = append(hits, Hit{p, line, fmt.Sprintf("hidden character U+%04X", r)})
			}
		}
	}
	return hits
}

type skillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ParseFrontmatter extracts the YAML block between the leading --- fences.
func ParseFrontmatter(doc []byte) (skillMeta, error) {
	s := strings.ReplaceAll(string(doc), "\r\n", "\n")
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		return skillMeta{}, fmt.Errorf("no YAML frontmatter")
	}
	block, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return skillMeta{}, fmt.Errorf("unterminated YAML frontmatter")
	}
	var m skillMeta
	if err := yaml.Unmarshal([]byte(block), &m); err != nil {
		return skillMeta{}, fmt.Errorf("invalid YAML frontmatter: %v", err)
	}
	return m, nil
}

func frontmatter(skill string, files tree.Tree) []Hit {
	doc, ok := files["SKILL.md"]
	if !ok {
		return []Hit{{Path: "SKILL.md", Msg: "SKILL.md is missing"}}
	}
	hit := func(format string, a ...any) []Hit {
		return []Hit{{Path: "SKILL.md", Line: 1, Msg: fmt.Sprintf(format, a...)}}
	}
	m, err := ParseFrontmatter(doc)
	if err != nil {
		return hit("%v", err)
	}
	var hits []Hit
	if m.Name != skill {
		hits = append(hits, hit("name %q does not match folder %q", m.Name, skill)...)
	}
	if !catalog.ValidName(m.Name) {
		hits = append(hits, hit("name %q must be lowercase-hyphen and at most 64 characters", m.Name)...)
	}
	if d := strings.TrimSpace(m.Description); d == "" {
		hits = append(hits, hit("description is empty")...)
	} else if utf8.RuneCountInString(d) > 1024 {
		hits = append(hits, hit("description exceeds 1024 characters")...)
	}
	return hits
}
