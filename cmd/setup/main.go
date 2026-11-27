package main

import (
	"os"

	"github.com/cychiang/setup/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Env{Dir: ".", Out: os.Stdout, Err: os.Stderr}))
}
