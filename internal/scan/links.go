package scan

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/cychiang/setup/internal/tree"
)

var (
	inlineLink  = regexp.MustCompile(`!?\[[^\]]*\]\(\s*<?([^)>\s]+)>?(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)`)
	refLink     = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*<?([^>\s]+)>?`)
	codeSpan    = regexp.MustCompile("`[^`\n]*`")
	urlScheme   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	fenceMarker = regexp.MustCompile("^\\s*(```|~~~)")
)

// skillLinks flags Markdown links and images whose relative target resolves outside
// the skill folder or to a file the skill does not contain. Once a skill is copied
// into a project, such a link points at nothing.
func skillLinks(_ string, files tree.Tree) []Hit {
	var hits []Hit
	for _, p := range files.Paths() {
		if !strings.HasSuffix(strings.ToLower(p), ".md") || !isText(files[p]) {
			continue
		}
		inFence := false
		for i, line := range strings.Split(string(files[p]), "\n") {
			if fenceMarker.MatchString(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			for _, target := range linkTargets(codeSpan.ReplaceAllString(line, "")) {
				if msg := brokenLink(files, p, target); msg != "" {
					hits = append(hits, Hit{p, i + 1, msg})
				}
			}
		}
	}
	return hits
}

func linkTargets(line string) []string {
	var out []string
	for _, m := range inlineLink.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	if m := refLink.FindStringSubmatch(line); m != nil {
		out = append(out, m[1])
	}
	return out
}

func brokenLink(files tree.Tree, from, target string) string {
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if target == "" || strings.HasPrefix(target, "/") || urlScheme.MatchString(target) {
		return ""
	}
	if dec, err := url.PathUnescape(target); err == nil {
		target = dec
	}
	resolved := path.Join(path.Dir(from), target)
	switch {
	case resolved == ".." || strings.HasPrefix(resolved, "../"):
		return "link " + target + " leaves the skill folder, so it breaks once the skill is installed"
	case !has(files, resolved):
		return "link " + target + " points at a file the skill does not contain"
	}
	return ""
}

func has(files tree.Tree, p string) bool {
	if p == "." {
		return true
	}
	if _, ok := files[p]; ok {
		return true
	}
	for f := range files {
		if strings.HasPrefix(f, p+"/") {
			return true
		}
	}
	return false
}
