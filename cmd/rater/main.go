// Command rater is a terminal annotation tool for research manual checks.
package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rater:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New(`usage:
  rater annotate --task <dir> --as <annotator>
  rater agree --task <dir> [--format text|json] [--annotators a,b]
  rater adjudicate --task <dir> --as <adjudicator>
  rater serve --task <dir> --listen <addr> --keys <file> [--host-key <path>]`)

func run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "annotate":
		return runAnnotate(args[1:])
	case "agree":
		return runAgree(args[1:], os.Stdout)
	case "adjudicate":
		return runAdjudicate(args[1:], os.Stdout)
	case "serve":
		return runServe(args[1:], os.Stderr)
	}
	return fmt.Errorf("unknown subcommand %q: %w", args[0], errUsage)
}
