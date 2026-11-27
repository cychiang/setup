// Package version reports the build identity of the setup binary.
package version

import (
	"fmt"
	"runtime/debug"
)

// Set at link time with -ldflags "-X github.com/cychiang/setup/internal/version.Version=...".
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// String formats the identity as one line.
func String() string {
	commit, date := Commit, Date
	if commit == "" || date == "" {
		bc, bd := buildInfo()
		if commit == "" {
			commit = bc
		}
		if date == "" {
			date = bd
		}
	}
	if commit == "" {
		commit = "unknown"
	}
	if date == "" {
		date = "unknown"
	}
	return fmt.Sprintf("setup %s (commit %s, built %s)", Version, commit, date)
}

func buildInfo() (commit, date string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			date = s.Value
		}
	}
	return commit, date
}
