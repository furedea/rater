// Package adjudication selects the items annotators disagree on, runs an
// adjudicator's decisions over them as an ordinary annotation session, and
// derives the gold answers. It does no I/O.
package adjudication

import (
	"errors"
	"fmt"

	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/task"
)

// Annotator is one annotator's history of a task, replayed from a log that
// held Records records when it was read.
type Annotator struct {
	Name    string
	History *task.History
	Records int
}

// Judgment is one annotator's state of an item, with the length of the log it
// came from, so the compared annotations can be found again in an
// append-only log.
type Judgment struct {
	Annotator string
	State     task.State
	Records   int
}

// Docket is the comparison of every item of a workspace across annotators.
type Docket struct {
	ws         task.Workspace
	annotators []Annotator
	outcomes   map[string]agreement.Outcome
}

// ErrNoDisagreements means no item is left for an adjudicator to decide.
var ErrNoDisagreements = errors.New("annotators disagree on no item")

const minAnnotators = 2

// NewDocket compares the annotators' latest effective states of each item.
func NewDocket(ws task.Workspace, annotators []Annotator) (Docket, error) {
	if len(annotators) < minAnnotators {
		return Docket{}, fmt.Errorf("%w, got %d", agreement.ErrTooFewAnnotators, len(annotators))
	}
	d := Docket{ws: ws, annotators: append([]Annotator(nil), annotators...), outcomes: map[string]agreement.Outcome{}}
	for _, it := range ws.Items {
		d.outcomes[it.ID] = agreement.Compare(ws.Task.Scheme(), d.states(it.ID))
	}
	return d, nil
}

// Scheme returns how the task's items are answered.
func (d Docket) Scheme() task.Scheme { return d.ws.Task.Scheme() }

// Disputed returns the items annotators disagree on, in workspace order.
func (d Docket) Disputed() []task.Item {
	var items []task.Item
	for _, it := range d.ws.Items {
		if d.outcomes[it.ID] == agreement.OutcomeDisagreed {
			items = append(items, it)
		}
	}
	return items
}

// Judgments returns every annotator's state of an item, in annotator order.
func (d Docket) Judgments(itemID string) []Judgment {
	judgments := make([]Judgment, 0, len(d.annotators))
	for _, a := range d.annotators {
		judgments = append(judgments, Judgment{Annotator: a.Name, State: a.History.State(itemID), Records: a.Records})
	}
	return judgments
}

// Open starts an adjudicator's session over the disputed items, resuming
// from the adjudicator's prior decisions. Decisions are annotation records
// of the task's scheme, so labels, answers, holds, notes, and undo work as
// in annotate.
//
// Each new decision is stamped with the basis of its item: a digest of every
// annotator's current state of it. Prior decisions must fit the whole
// workspace, but only those on items still disputed and made against the
// current basis are replayed; the rest stay in the log. So a decision whose
// compared annotations have since changed is shown again as undecided, and
// gold never pairs a decision with annotations the adjudicator did not see.
func (d Docket) Open(prior []task.Annotation, rec task.Recorder) (*task.Session, error) {
	if _, err := task.ReplayLog(d.ws, prior); err != nil {
		return nil, err
	}
	disputed := d.Disputed()
	if len(disputed) == 0 {
		return nil, ErrNoDisagreements
	}
	ws, err := task.NewWorkspace(d.ws.Task, disputed)
	if err != nil {
		return nil, err
	}
	return task.NewSession(ws, d.currentDecisions(prior), basisRecorder{rec: rec, docket: d})
}

func (d Docket) currentDecisions(prior []task.Annotation) []task.Annotation {
	var kept []task.Annotation
	for _, a := range prior {
		if d.outcomes[a.ItemID] == agreement.OutcomeDisagreed && a.Basis == d.basis(a.ItemID) {
			kept = append(kept, a)
		}
	}
	return kept
}

func (d Docket) states(itemID string) []task.State {
	states := make([]task.State, 0, len(d.annotators))
	for _, a := range d.annotators {
		states = append(states, a.History.State(itemID))
	}
	return states
}
