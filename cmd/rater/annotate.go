package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
	"github.com/furedea/rater/internal/tui"
)

// sessionOptions names the task directory and the person working on it.
type sessionOptions struct {
	taskDir string
	who     task.Annotator
}

// parseSessionArgs parses the --task and --as flags of an interactive command.
func parseSessionArgs(command string, args []string) (sessionOptions, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	taskDir := fs.String("task", "", "task directory containing task.yaml and items.jsonl")
	as := fs.String("as", "", "your name, which names your log")
	if err := fs.Parse(args); err != nil {
		return sessionOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	if *taskDir == "" {
		return sessionOptions{}, errors.New("--task is required")
	}
	who, err := task.NewAnnotator(*as)
	if err != nil {
		return sessionOptions{}, fmt.Errorf("--as: %w", err)
	}
	return sessionOptions{taskDir: *taskDir, who: who}, nil
}

func runAnnotate(args []string) (err error) {
	opts, err := parseSessionArgs("annotate", args)
	if err != nil {
		return err
	}
	ws, err := store.LoadWorkspace(opts.taskDir)
	if err != nil {
		return err
	}
	plan, err := store.LoadAssignment(opts.taskDir, ws)
	if err != nil {
		return err
	}
	log, prior, err := store.OpenLog(opts.taskDir, opts.who)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	session, err := plan.StartSession(opts.who, prior, log)
	if err != nil {
		return fmt.Errorf("resume session: %w", err)
	}
	return tui.Run(session)
}
