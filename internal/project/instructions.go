package project

import (
	"strings"

	"github.com/cychiang/setup/internal/tree"
)

const (
	AgentsFile = "AGENTS.md"
	ClaudeFile = "CLAUDE.md"

	claudeImport = "@AGENTS.md"
)

// BlockFiles are the instruction files that carry managed blocks.
var BlockFiles = []string{AgentsFile}

// ownedClaude is the whole CLAUDE.md that setup creates and tracks.
const ownedClaude = claudeImport + "\n"

// claudeMD is the desired state of CLAUDE.md. Claude Code loads CLAUDE.md in
// preference to AGENTS.md and expands @path imports, so AGENTS.md stays the single
// source and CLAUDE.md only points at it. Owned means setup created the whole file
// and tracks it in setup.lock; otherwise the file is the user's and stays untracked.
type claudeMD struct {
	Text  string
	Owned bool
}

// desiredClaude decides CLAUDE.md from its text after legacy blocks were removed.
// wasOwned says setup.lock tracked the file as a whole before.
func desiredClaude(doc string, wasOwned bool) claudeMD {
	switch {
	case strings.TrimSpace(doc) == "":
		return claudeMD{Text: ownedClaude, Owned: true}
	case !importsAgents(doc):
		return claudeMD{Text: claudeImport + "\n\n" + doc}
	default:
		return claudeMD{Text: doc, Owned: wasOwned && doc == ownedClaude}
	}
}

// importsAgents reports whether a line is exactly an AGENTS.md import outside a code fence.
func importsAgents(doc string) bool {
	fence := ""
	for _, line := range strings.Split(doc, "\n") {
		trim := strings.TrimSpace(line)
		if marker := fenceMarker(trim); marker != "" {
			switch {
			case fence == "":
				fence = marker
			case strings.HasPrefix(trim, fence):
				fence = ""
			}
			continue
		}
		if fence == "" && (trim == "@AGENTS.md" || trim == "@./AGENTS.md") {
			return true
		}
	}
	return false
}

func fenceMarker(trim string) string {
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trim, m) {
			return m
		}
	}
	return ""
}

// claudeLock is what setup.lock said about CLAUDE.md before this run.
type claudeLock struct {
	// Legacy are bundle blocks written by the version that copied every block into CLAUDE.md.
	Legacy []LockFile
	// Owned is true when the lock tracked the whole file.
	Owned bool
}

// splitClaudeLock separates the CLAUDE.md entries from the rest of the lock files.
// A whole-file entry is told from a legacy block by its hash, which is that of ownedClaude.
func splitClaudeLock(files []LockFile) (claudeLock, []LockFile) {
	var c claudeLock
	var rest []LockFile
	for _, f := range files {
		switch {
		case f.Path != ClaudeFile:
			rest = append(rest, f)
		case f.Bundle == "base" && f.SHA256 == tree.SHA([]byte(ownedClaude)):
			c.Owned = true
		default:
			c.Legacy = append(c.Legacy, f)
		}
	}
	return c, rest
}

// stripLegacy removes the legacy bundle blocks from doc. conflict is true when
// one of them was hand-edited since the lock recorded it.
func stripLegacy(doc string, legacy []LockFile) (rest string, conflict bool, err error) {
	if len(legacy) == 0 {
		return doc, false, nil
	}
	segs, err := parseSegs(doc)
	if err != nil {
		return "", false, err
	}
	bodies := blockBodies(segs)
	remove := map[string]bool{}
	for _, f := range legacy {
		body, ok := bodies[f.Bundle]
		if !ok {
			continue
		}
		remove[f.Bundle] = true
		conflict = conflict || tree.SHA([]byte(body)) != f.SHA256
	}
	rest, err = editBlocks(doc, nil, remove, nil)
	return rest, conflict, err
}
