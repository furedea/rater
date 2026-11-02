package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
	"github.com/furedea/rater/internal/tui"
)

// screen shows an adjudication session until the adjudicator quits.
type screen func(*task.Session, adjudication.Docket) error

func showAdjudication(s *task.Session, d adjudication.Docket) error { return tui.RunAdjudication(s, d) }

func runAdjudicate(args []string, out io.Writer) error {
	return adjudicate(args, showAdjudication, out)
}

// adjudicate lets the adjudicator decide every disagreement on show, then
// derives gold.jsonl from the annotation logs and the adjudication log. The
// logs are the record; gold.jsonl is rewritten from them on every run, even
// when show fails, since each decision is already durable when it is made.
func adjudicate(args []string, show screen, out io.Writer) (err error) {
	opts, err := parseSessionArgs("adjudicate", args)
	if err != nil {
		return err
	}
	docket, err := loadDocket(opts.taskDir)
	if err != nil {
		return err
	}
	log, prior, err := store.OpenAdjudicationLog(opts.taskDir, opts.who)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	session, err := docket.Open(prior, log)
	if errors.Is(err, adjudication.ErrNoDisagreements) {
		if _, err := fmt.Fprintln(out, "no disagreements to adjudicate; writing agreed items to gold.jsonl"); err != nil {
			return err
		}
		return store.WriteGold(opts.taskDir, docket.Gold(opts.who, task.NewHistory(docket.Scheme())))
	}
	if err != nil {
		return fmt.Errorf("resume adjudication: %w", err)
	}
	showErr := show(session, docket)
	return errors.Join(showErr, store.WriteGold(opts.taskDir, docket.Gold(opts.who, session)))
}

func loadDocket(dir string) (adjudication.Docket, error) {
	ws, err := store.LoadWorkspace(dir)
	if err != nil {
		return adjudication.Docket{}, err
	}
	logs, err := store.ReadLogs(dir)
	if err != nil {
		return adjudication.Docket{}, err
	}
	annotators := make([]adjudication.Annotator, 0, len(logs))
	for _, l := range logs {
		h, err := task.ReplayLog(ws, l.Records)
		if err != nil {
			return adjudication.Docket{}, fmt.Errorf("log of %s: %w", l.Annotator, err)
		}
		annotators = append(annotators, adjudication.Annotator{Name: l.Annotator.String(), History: h, Records: len(l.Records)})
	}
	return adjudication.NewDocket(ws, annotators)
}
