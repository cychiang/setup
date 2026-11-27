package cli

// DefaultCatalog is used when no flag, setup.yaml entry or SETUP_CATALOG names a catalog.
const DefaultCatalog = "github:cychiang/setup"

// catalogSpec picks the released catalog: --catalog, then setup.yaml, then SETUP_CATALOG, then DefaultCatalog.
// The dev link in setup.local.yaml is resolved before this and takes precedence over all of them.
func catalogSpec(flag, configured string, getenv func(string) string) string {
	for _, s := range []string{flag, configured, getenv("SETUP_CATALOG")} {
		if s != "" {
			return s
		}
	}
	return DefaultCatalog
}
