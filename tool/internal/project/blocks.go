package project

import (
	"fmt"
	"strings"
)

// The marker format is a versioned contract.
func beginMarker(bundle string) string { return "<!-- setup:begin " + bundle + " -->" }
func endMarker(bundle string) string   { return "<!-- setup:end " + bundle + " -->" }

func renderBlock(b Block) string {
	return beginMarker(b.Bundle) + "\n" + b.Body + "\n" + endMarker(b.Bundle) + "\n"
}

// seg is plain text (bundle == "") or one managed block including its marker lines.
type seg struct {
	bundle string
	text   string
}

func (s seg) body() string {
	lines := strings.Split(strings.TrimSuffix(s.text, "\n"), "\n")
	return strings.Join(lines[1:len(lines)-1], "\n")
}

func parseSegs(doc string) ([]seg, error) {
	var segs []seg
	var text strings.Builder
	var cur *seg
	flush := func() {
		if text.Len() > 0 {
			segs = append(segs, seg{text: text.String()})
			text.Reset()
		}
	}
	for _, line := range strings.SplitAfter(doc, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case cur != nil:
			cur.text += line
			if trim == endMarker(cur.bundle) {
				segs = append(segs, *cur)
				cur = nil
			}
		case strings.HasPrefix(trim, "<!-- setup:begin ") && strings.HasSuffix(trim, " -->"):
			flush()
			name := strings.TrimSuffix(strings.TrimPrefix(trim, "<!-- setup:begin "), " -->")
			cur = &seg{bundle: name, text: line}
		default:
			text.WriteString(line)
		}
	}
	if cur != nil {
		return nil, fmt.Errorf("unterminated managed block for bundle %s", cur.bundle)
	}
	flush()
	return segs, nil
}

func blockBodies(segs []seg) map[string]string {
	m := map[string]string{}
	for _, s := range segs {
		if s.bundle != "" {
			m[s.bundle] = s.body()
		}
	}
	return m
}

// editBlocks replaces or inserts the blocks in set and removes those in remove.
// order lists the desired bundles in layer order and decides where new blocks go.
// Text outside the markers is never touched.
func editBlocks(doc string, set []Block, remove map[string]bool, order []string) (string, error) {
	segs, err := parseSegs(doc)
	if err != nil {
		return "", err
	}
	rank := map[string]int{}
	for i, b := range order {
		rank[b] = i
	}
	have := map[string]bool{}
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		if s.bundle == "" {
			continue
		}
		have[s.bundle] = true
		for _, b := range set {
			if b.Bundle == s.bundle {
				segs[i].text = renderBlock(b)
			}
		}
		if remove[s.bundle] {
			end := i + 1
			switch {
			case end < len(segs) && segs[end].text == "\n":
				segs = append(segs[:i], segs[end+1:]...)
			case i > 0 && segs[i-1].text == "\n":
				segs = append(segs[:i-1], segs[end:]...)
				i--
			default:
				segs = append(segs[:i], segs[end:]...)
			}
			i--
		}
	}
	for _, b := range set {
		if have[b.Bundle] {
			continue
		}
		segs = insertBlock(segs, b, rank)
	}
	var sb strings.Builder
	for _, s := range segs {
		sb.WriteString(s.text)
	}
	return sb.String(), nil
}

func insertBlock(segs []seg, b Block, rank map[string]int) []seg {
	blk := seg{bundle: b.Bundle, text: renderBlock(b)}
	lastBlock := -1
	for i, s := range segs {
		if s.bundle == "" {
			continue
		}
		if r, ok := rank[s.bundle]; ok && r > rank[b.Bundle] {
			out := append([]seg(nil), segs[:i]...)
			return append(append(out, blk, seg{text: "\n"}), segs[i:]...)
		}
		lastBlock = i
	}
	if lastBlock >= 0 {
		out := append([]seg(nil), segs[:lastBlock+1]...)
		return append(append(out, seg{text: "\n"}, blk), segs[lastBlock+1:]...)
	}
	if n := len(segs); n > 0 {
		last := &segs[n-1]
		if !strings.HasSuffix(last.text, "\n") {
			last.text += "\n"
		}
		return append(segs, seg{text: "\n"}, blk)
	}
	return append(segs, blk)
}
