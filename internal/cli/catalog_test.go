package cli

import "testing"

func TestCatalogSpecPrecedence(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "SETUP_CATALOG" {
				return v
			}
			return ""
		}
	}
	cases := []struct {
		name, flag, configured, envVal, want string
	}{
		{"flag wins", "/flag", "/cfg", "/env", "/flag"},
		{"setup.yaml next", "", "/cfg", "/env", "/cfg"},
		{"SETUP_CATALOG next", "", "", "/env", "/env"},
		{"built-in default last", "", "", "", "github:cychiang/setup"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := catalogSpec(c.flag, c.configured, env(c.envVal)); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
