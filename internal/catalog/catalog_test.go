package catalog_test

import (
	"strings"
	"testing"

	"github.com/cychiang/setup/internal/catalog"
)

const valid = `name: finance
layer: domain
version: 1.0.0
description: Finance domain.
suggests: []
skills:
  - local: money-precision
  - source: anthropic-kwp
    path: finance/skills/financial-statements
`

func TestParseBundle(t *testing.T) {
	b, err := catalog.ParseBundle([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if b.Layer != catalog.Domain || b.Version.String() != "1.0.0" || len(b.Skills) != 2 {
		t.Fatalf("%+v", b)
	}
	if r, ok := b.Skills[1].(catalog.Remote); !ok || r.SkillName() != "financial-statements" {
		t.Fatalf("remote ref = %+v", b.Skills[1])
	}
	out, err := b.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	again, err := catalog.ParseBundle(out)
	if err != nil || len(again.Skills) != 2 {
		t.Fatalf("round trip: %v %v", again, err)
	}
}

func TestParseBundleRejects(t *testing.T) {
	cases := map[string]string{
		"both local and source": strings.Replace(valid, "  - local: money-precision", "  - local: x\n    source: s\n    path: p", 1),
		"neither":               strings.Replace(valid, "  - local: money-precision", "  - {}", 1),
		"source without path":   strings.Replace(valid, "    path: finance/skills/financial-statements\n", "", 1),
		"bad layer":             strings.Replace(valid, "domain", "galaxy", 1),
		"bad version":           strings.Replace(valid, "1.0.0", "1.0", 1),
		"bad name":              strings.Replace(valid, "name: finance", "name: Finance", 1),
		"unknown key":           valid + "extra: 1\n",
		"duplicate skill":       valid + "  - local: money-precision\n",
	}
	for name, doc := range cases {
		if _, err := catalog.ParseBundle([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
