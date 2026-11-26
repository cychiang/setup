// Package fetch resolves registered third-party sources to pinned directory trees.
package fetch

// Fetcher is the single seam between build mode and third-party sources.
type Fetcher interface {
	// Sync fetches the latest state of the source and returns the commit to pin.
	Sync(name, url string) (commit string, err error)
	// Materialize returns a directory holding the source tree at commit.
	Materialize(name, commit string) (dir string, err error)
}
