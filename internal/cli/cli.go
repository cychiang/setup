// Package cli wires the setup commands.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cychiang/setup/internal/version"
)

// Env carries everything a command touches outside its arguments, so tests can drive it.
type Env struct {
	Dir    string
	Out    io.Writer
	Err    io.Writer
	Getenv func(string) string
}

func (e Env) getenv(k string) string {
	if e.Getenv != nil {
		return e.Getenv(k)
	}
	return os.Getenv(k)
}

// errExit is a failure that already printed its explanation.
var errExit = errors.New("")

type usageError string

func (u usageError) Error() string { return string(u) }

const usage = `usage: setup <command>

build mode:
  source add <name> <git-url>
  source sync [<name>]
  bundle new <name> --layer base|domain|type
  bundle add <bundle> <source>/<path> | --local <name>
  bundle remove <bundle> <skill>
  bundle check [<bundle>]
  bundle release <bundle> [--publish] [--out dist] [--allow-missing-scanners]

init mode:
  init --bundles a,b --catalog <dir|github:owner/repo> [--yes] [--json] [--force]
  add <bundle>...      remove <bundle>...
  outdated [--json]    update [<bundle>...] [--force]
  check [--json]

other:
  version              print version, commit and build date (also --version)
`

// Run executes one command line and returns the process exit code.
func Run(args []string, env Env) int {
	if env.Dir == "" {
		env.Dir = "."
	}
	err := dispatch(args, env)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		return 0
	case err == errExit:
		return 1
	}
	var u usageError
	if errors.As(err, &u) {
		fmt.Fprintf(env.Err, "setup: %v\n\n%s", err, usage)
		return 2
	}
	fmt.Fprintf(env.Err, "setup: %v\n", err)
	return 1
}

func dispatch(args []string, env Env) error {
	if len(args) == 0 {
		return usageError("no command given")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "source":
		return sourceCmd(rest, env)
	case "bundle":
		return bundleCmd(rest, env)
	case "init":
		return initCmd(rest, env)
	case "add":
		return addCmd(rest, env)
	case "remove":
		return removeCmd(rest, env)
	case "outdated":
		return outdatedCmd(rest, env)
	case "update":
		return updateCmd(rest, env)
	case "check":
		return checkCmd(rest, env)
	case "version", "--version", "-version":
		fmt.Fprintln(env.Out, version.String())
		return nil
	case "help", "-h", "--help":
		fmt.Fprint(env.Out, usage)
		return nil
	}
	return usageError("unknown command " + cmd)
}

func newFlags(name string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(env.Err)
	return fs
}

// parseFlags accepts flags before, between and after positional arguments.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
