package adjudication

import (
	"fmt"

	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/task"
)

// Source says how a gold answer was reached.
type Source int

// Sources.
const (
	// SourceAgreed: every completed annotation gave this answer.
	SourceAgreed Source = iota
	// SourceAdjudicated: annotators disagreed and the adjudicator decided.
	SourceAdjudicated
)

func (s Source) String() string {
	switch s {
	case SourceAgreed:
		return "agreed"
	case SourceAdjudicated:
		return "adjudicated"
	}
	return fmt.Sprintf("Source(%d)", int(s))
}

// Gold is the final answer of one item.
type Gold struct {
	ItemID string
	Source Source
	// Final holds the label or the checklist answers, and the adjudicator's
	// note; an agreed item has no note.
	Final task.State
	// Verdict is derived from a checklist's answers; it is VerdictUndecided
	// for a single-choice task.
	Verdict task.Verdict
	// Adjudicator is empty for an agreed item.
	Adjudicator string
	Judgments   []Judgment
}

// Decisions is what Gold reads of an adjudicator's work: the latest state of
// each item. Both a replayed history and a live session provide it.
type Decisions interface {
	State(itemID string) task.State
}

var (
	_ Decisions = (*task.History)(nil)
	_ Decisions = (*task.Session)(nil)
)

// Gold returns the final answers in workspace order: the agreed answer of
// every agreed item and the adjudicator's complete decision on every
// disputed one. Items that are undecided, or that fewer than two annotators
// completed, are left out.
func (d Docket) Gold(adjudicator task.Annotator, decisions Decisions) []Gold {
	var golds []Gold
	for _, it := range d.ws.Items {
		g, ok := d.gold(it.ID, adjudicator, decisions)
		if !ok {
			continue
		}
		g.Verdict = d.verdict(g.Final)
		g.Judgments = d.Judgments(it.ID)
		golds = append(golds, g)
	}
	return golds
}

func (d Docket) gold(itemID string, adjudicator task.Annotator, decisions Decisions) (Gold, bool) {
	switch d.outcomes[itemID] {
	case agreement.OutcomeAgreed:
		return Gold{ItemID: itemID, Source: SourceAgreed, Final: d.agreed(itemID)}, true
	case agreement.OutcomeDisagreed:
		decided := decisions.State(itemID)
		if decided.Status != task.StatusLabeled {
			return Gold{}, false
		}
		return Gold{ItemID: itemID, Source: SourceAdjudicated, Final: decided, Adjudicator: adjudicator.String()}, true
	case agreement.OutcomeUncompared:
	}
	return Gold{}, false
}

// agreed returns the shared answer of an agreed item, without anyone's note.
func (d Docket) agreed(itemID string) task.State {
	for _, s := range d.states(itemID) {
		if s.Status == task.StatusLabeled {
			return task.State{Status: task.StatusLabeled, Label: s.Label, Answers: s.Answers}
		}
	}
	return task.State{}
}

func (d Docket) verdict(final task.State) task.Verdict {
	if c, ok := d.Scheme().(task.Checklist); ok {
		return c.Verdict(final.Answers)
	}
	return task.VerdictUndecided
}
