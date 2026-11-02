package agreement

import (
	"errors"
	"fmt"

	"github.com/furedea/rater/internal/task"
)

// Annotator is one annotator's history of a task, already replayed from the log.
type Annotator struct {
	Name    string
	History *task.History
}

// Report is the agreement of several annotators on one task.
type Report struct {
	Task       string
	Items      int
	Annotators []Coverage
	// Criteria holds one measure per checklist criterion, in task order; it
	// is empty for a single-choice task.
	Criteria []Measure
	// Overall compares the label, or a checklist's derived verdict.
	Overall Measure
}

// Coverage counts where an annotator's items stand. Only labeled items, and
// for a checklist only complete ones, take part in agreement.
type Coverage struct {
	Name    string
	Labeled int
	Held    int
	Pending int
}

// Measure is the agreement on one value of an item.
type Measure struct {
	Name  string
	Alpha AlphaResult
	// Pair is the confusion matrix, from which kappa follows; it is set only
	// when exactly two annotators are compared.
	Pair *Confusion
}

// ErrTooFewAnnotators means agreement was asked of fewer than two annotators.
var ErrTooFewAnnotators = errors.New("agreement needs at least two annotators")

const pair = 2

// NewReport compares the annotators' latest effective states on every item.
func NewReport(ws task.Workspace, annotators []Annotator) (Report, error) {
	if len(annotators) < pair {
		return Report{}, fmt.Errorf("%w, got %d", ErrTooFewAnnotators, len(annotators))
	}
	r := Report{Task: ws.Task.Name(), Items: len(ws.Items)}
	for _, a := range annotators {
		r.Annotators = append(r.Annotators, coverage(ws, a))
	}
	compared := comparison{items: ws.Items, annotators: annotators}
	var err error
	switch scheme := ws.Task.Scheme().(type) {
	case task.SingleChoice:
		r.Overall, err = compared.measure("label", func(s task.State) string { return s.Label })
	case task.Checklist:
		r.Criteria, err = compared.criteria(scheme)
		if err != nil {
			return Report{}, err
		}
		r.Overall, err = compared.measure("verdict", func(s task.State) string { return scheme.Verdict(s.Answers).String() })
	}
	if err != nil {
		return Report{}, err
	}
	return r, nil
}

func coverage(ws task.Workspace, a Annotator) Coverage {
	c := Coverage{Name: a.Name}
	for _, it := range ws.Items {
		switch a.History.State(it.ID).Status {
		case task.StatusLabeled:
			c.Labeled++
		case task.StatusHeld:
			c.Held++
		case task.StatusPending:
			c.Pending++
		}
	}
	return c
}

// comparison is the annotators' states on the items, one unit per item.
type comparison struct {
	items      []task.Item
	annotators []Annotator
}

func (c comparison) criteria(scheme task.Checklist) ([]Measure, error) {
	var measures []Measure
	for _, cr := range scheme.Criteria() {
		m, err := c.measure(cr.Name, func(s task.State) string { return s.Answers[cr.Name] })
		if err != nil {
			return nil, err
		}
		measures = append(measures, m)
	}
	return measures, nil
}

// measure compares the value that valueOf reads from each labeled state.
func (c comparison) measure(name string, valueOf func(task.State) string) (Measure, error) {
	units := make([][]Value, 0, len(c.items))
	for _, it := range c.items {
		unit := make([]Value, 0, len(c.annotators))
		for _, a := range c.annotators {
			unit = append(unit, labeledValue(a.History.State(it.ID), valueOf))
		}
		units = append(units, unit)
	}
	d, err := NewData(len(c.annotators), units)
	if err != nil {
		return Measure{}, fmt.Errorf("measure %s: %w", name, err)
	}
	m := Measure{Name: name, Alpha: Alpha(d)}
	if len(c.annotators) != pair {
		return m, nil
	}
	confusion, err := NewConfusion(d)
	if err != nil {
		return Measure{}, fmt.Errorf("measure %s: %w", name, err)
	}
	m.Pair = &confusion
	return m, nil
}

func labeledValue(s task.State, valueOf func(task.State) string) Value {
	if s.Status != task.StatusLabeled {
		return Missing
	}
	return Category(valueOf(s))
}
