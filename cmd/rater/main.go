// Command rater is a terminal annotation tool for research manual checks.
package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// versionFile is the single source of the release version: the Nix package reads it, and the
// release workflow refuses a tag that does not match it.
//
//go:embed version.txt
var versionFile string

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rater:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New(`usage:
  rater annotate --task <dir> --as <annotator>
  rater agree --task <dir> [--format text|json] [--annotators a,b]
  rater adjudicate --task <dir> --as <adjudicator>
  rater serve --task <dir> --listen <addr> --keys <file> [--host-key <path>]
  rater --version`)

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "annotate":
		return runAnnotate(args[1:])
	case "agree":
		return runAgree(args[1:], stdout)
	case "adjudicate":
		return runAdjudicate(args[1:], stdout)
	case "serve":
		return runServe(args[1:], os.Stderr)
	case "--version":
		_, err := fmt.Fprintln(stdout, "rater", strings.TrimSpace(versionFile))
		return err
	}
	return fmt.Errorf("unknown subcommand %q: %w", args[0], errUsage)
}
