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

type annotateOptions struct {
	taskDir   string
	annotator task.Annotator
}

func parseAnnotateArgs(args []string) (annotateOptions, error) {
	fs := flag.NewFlagSet("annotate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	taskDir := fs.String("task", "", "task directory containing task.yaml and items.jsonl")
	as := fs.String("as", "", "annotator name")
	if err := fs.Parse(args); err != nil {
		return annotateOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	if *taskDir == "" {
		return annotateOptions{}, errors.New("--task is required")
	}
	who, err := task.NewAnnotator(*as)
	if err != nil {
		return annotateOptions{}, fmt.Errorf("--as: %w", err)
	}
	return annotateOptions{taskDir: *taskDir, annotator: who}, nil
}

func runAnnotate(args []string) (err error) {
	opts, err := parseAnnotateArgs(args)
	if err != nil {
		return err
	}
	ws, err := store.LoadWorkspace(opts.taskDir)
	if err != nil {
		return err
	}
	log, prior, err := store.OpenLog(opts.taskDir, opts.annotator)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	session, err := task.NewSession(ws, prior, log)
	if err != nil {
		return fmt.Errorf("resume session: %w", err)
	}
	return tui.Run(session)
}
