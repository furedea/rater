package task

import (
	"fmt"
	"maps"
	"time"
)

// Action is what one annotation record does to an item's state.
type Action int

// Actions. Undo is itself a record; the log is never rewritten.
const (
	ActionLabel Action = iota
	ActionHold
	ActionNote
	ActionUndo
	ActionAnswer
)

func (a Action) String() string {
	switch a {
	case ActionLabel:
		return "label"
	case ActionHold:
		return "hold"
	case ActionNote:
		return "note"
	case ActionUndo:
		return "undo"
	case ActionAnswer:
		return "answer"
	}
	return fmt.Sprintf("Action(%d)", int(a))
}

// ParseAction is the inverse of Action.String.
func ParseAction(s string) (Action, error) {
	for _, a := range []Action{ActionLabel, ActionHold, ActionNote, ActionUndo, ActionAnswer} {
		if a.String() == s {
			return a, nil
		}
	}
	return 0, fmt.Errorf("unknown action %q", s)
}

// Annotation is one record of the append-only log. A label record carries
// Label; an answer record carries Criterion and the answer's name in Label.
type Annotation struct {
	ItemID    string
	Annotator string
	Action    Action
	Criterion string
	Label     string
	Note      string
	At        time.Time
}

// Status is where an item stands for one annotator.
type Status int

// Statuses. A checklist item is labeled once every criterion has an answer.
const (
	StatusPending Status = iota
	StatusLabeled
	StatusHeld
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusLabeled:
		return "labeled"
	case StatusHeld:
		return "held"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// State is the latest effective state of one item. Answers maps criterion
// names to answer names; it is nil until a checklist answer is given and is
// never modified in place.
type State struct {
	Status  Status
	Label   string
	Answers map[string]string
	Note    string
}

// History derives per-item state from annotation records, in log order.
type History struct {
	scheme Scheme
	stacks map[string][]State
	trail  []string
}

// NewHistory returns an empty history for items answered under scheme.
func NewHistory(scheme Scheme) *History {
	return &History{scheme: scheme, stacks: map[string][]State{}}
}

// Replay builds a history from records, as read back from a log.
func Replay(scheme Scheme, records []Annotation) *History {
	h := NewHistory(scheme)
	for _, r := range records {
		h.Apply(r)
	}
	return h
}

// Apply folds one record in. An undo with nothing to undo is ignored.
func (h *History) Apply(a Annotation) {
	if a.Action == ActionUndo {
		h.pop(a.ItemID)
		return
	}
	next := h.State(a.ItemID)
	switch a.Action {
	case ActionLabel:
		next.Status, next.Label = StatusLabeled, a.Label
	case ActionHold:
		next.Status, next.Label = StatusHeld, ""
	case ActionNote:
		next.Note = a.Note
	case ActionAnswer:
		next.Answers = withAnswer(next.Answers, a.Criterion, a.Label)
		next.Status = h.answeredStatus(next.Answers)
	case ActionUndo:
	}
	h.stacks[a.ItemID] = append(h.stacks[a.ItemID], next)
	h.trail = append(h.trail, a.ItemID)
}

// State returns the latest effective state of an item.
func (h *History) State(itemID string) State {
	stack := h.stacks[itemID]
	if len(stack) == 0 {
		return State{}
	}
	return stack[len(stack)-1]
}

// LastItemID returns the item touched by the most recent effective record.
func (h *History) LastItemID() (string, bool) {
	if len(h.trail) == 0 {
		return "", false
	}
	return h.trail[len(h.trail)-1], true
}

// answeredStatus is labeled when the answers complete the checklist, else pending.
func (h *History) answeredStatus(answers map[string]string) Status {
	switch s := h.scheme.(type) {
	case Checklist:
		if s.Complete(answers) {
			return StatusLabeled
		}
	case SingleChoice:
	}
	return StatusPending
}

func withAnswer(answers map[string]string, criterion, answer string) map[string]string {
	next := make(map[string]string, len(answers)+1)
	maps.Copy(next, answers)
	next[criterion] = answer
	return next
}

func (h *History) pop(itemID string) {
	stack := h.stacks[itemID]
	if len(stack) == 0 {
		return
	}
	h.stacks[itemID] = stack[:len(stack)-1]
	for i := len(h.trail) - 1; i >= 0; i-- {
		if h.trail[i] == itemID {
			h.trail = append(h.trail[:i], h.trail[i+1:]...)
			return
		}
	}
}
