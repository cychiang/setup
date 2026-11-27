package catalog

import "fmt"

// Layer orders bundles: base first, then domains, then types.
type Layer int

const (
	Base Layer = iota
	Domain
	Type
)

var layerNames = [...]string{"base", "domain", "type"}

func (l Layer) String() string { return layerNames[l] }

func ParseLayer(s string) (Layer, error) {
	for i, n := range layerNames {
		if n == s {
			return Layer(i), nil
		}
	}
	return 0, fmt.Errorf("unknown layer %q (want base, domain or type)", s)
}

func (l Layer) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

func (l *Layer) UnmarshalText(b []byte) error {
	p, err := ParseLayer(string(b))
	if err != nil {
		return err
	}
	*l = p
	return nil
}
