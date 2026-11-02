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

var errUsage = errors.New("usage: rater annotate --task <dir> --as <annotator>")

func run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}
	switch args[0] {
	case "annotate":
		return runAnnotate(args[1:])
	}
	return fmt.Errorf("unknown subcommand %q: %w", args[0], errUsage)
}
