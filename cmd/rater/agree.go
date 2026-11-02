package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
)

// reportFormat is how the agreement report is written.
type reportFormat int

const (
	formatText reportFormat = iota
	formatJSON
)

func parseReportFormat(s string) (reportFormat, error) {
	switch s {
	case "text":
		return formatText, nil
	case "json":
		return formatJSON, nil
	}
	return 0, fmt.Errorf("unknown format %q (want text or json)", s)
}

type agreeOptions struct {
	taskDir string
	format  reportFormat
	// annotators selects whose logs to compare; empty means every log.
	annotators []task.Annotator
}

const minAnnotators = 2

func parseAgreeArgs(args []string) (agreeOptions, error) {
	fs := flag.NewFlagSet("agree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	taskDir := fs.String("task", "", "task directory containing task.yaml, items.jsonl, and annotations/")
	format := fs.String("format", "text", "output format: text or json")
	names := fs.String("annotators", "", "comma-separated annotators to compare (default: all)")
	if err := fs.Parse(args); err != nil {
		return agreeOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	if *taskDir == "" {
		return agreeOptions{}, errors.New("--task is required")
	}
	f, err := parseReportFormat(*format)
	if err != nil {
		return agreeOptions{}, fmt.Errorf("--format: %w", err)
	}
	selected, err := parseAnnotatorList(*names)
	if err != nil {
		return agreeOptions{}, fmt.Errorf("--annotators: %w", err)
	}
	return agreeOptions{taskDir: *taskDir, format: f, annotators: selected}, nil
}

// parseAnnotatorList parses "a,b,..." into at least two distinct annotators; "" selects none.
func parseAnnotatorList(list string) ([]task.Annotator, error) {
	if list == "" {
		return nil, nil
	}
	var selected []task.Annotator
	seen := map[string]bool{}
	for name := range strings.SplitSeq(list, ",") {
		a, err := task.NewAnnotator(strings.TrimSpace(name))
		if err != nil {
			return nil, err
		}
		if seen[a.String()] {
			return nil, fmt.Errorf("annotator %q is listed twice", a)
		}
		seen[a.String()] = true
		selected = append(selected, a)
	}
	if len(selected) < minAnnotators {
		return nil, fmt.Errorf("need at least two annotators, got %d", len(selected))
	}
	return selected, nil
}

func runAgree(args []string, out io.Writer) error {
	opts, err := parseAgreeArgs(args)
	if err != nil {
		return err
	}
	ws, err := store.LoadWorkspace(opts.taskDir)
	if err != nil {
		return err
	}
	logs, err := readLogs(opts.taskDir, opts.annotators)
	if err != nil {
		return err
	}
	annotators, err := replayLogs(ws, logs)
	if err != nil {
		return err
	}
	report, err := agreement.NewReport(ws, annotators)
	if err != nil {
		return err
	}
	return writeReport(out, opts.format, report)
}

// readLogs reads only the selected annotators' logs, in the order given, or
// every log when none is selected.
func readLogs(dir string, selected []task.Annotator) ([]store.AnnotatorLog, error) {
	if len(selected) == 0 {
		return store.ReadLogs(dir)
	}
	logs := make([]store.AnnotatorLog, 0, len(selected))
	for _, a := range selected {
		l, err := store.ReadLog(dir, a)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func replayLogs(ws task.Workspace, logs []store.AnnotatorLog) ([]agreement.Annotator, error) {
	annotators := make([]agreement.Annotator, 0, len(logs))
	for _, l := range logs {
		h, err := task.ReplayLog(ws, l.Records)
		if err != nil {
			return nil, fmt.Errorf("log of %s: %w", l.Annotator, err)
		}
		annotators = append(annotators, agreement.Annotator{Name: l.Annotator.String(), History: h})
	}
	return annotators, nil
}
