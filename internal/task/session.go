package task

import (
	"errors"
	"fmt"
)

// Recorder persists annotation records. It is satisfied by the store's log.
type Recorder interface {
	Append(Annotation) error
}

// Sentinel errors that callers branch on.
var (
	ErrUnknownLabel    = errors.New("unknown label")
	ErrNothingToUndo   = errors.New("nothing to undo")
	ErrUnknownItem     = errors.New("record refers to unknown item")
	ErrRecordMismatch  = errors.New("record does not fit the task")
	errCursorOutOfBand = errors.New("cursor out of range")
)

// Session is one annotator working through a workspace. For a checklist task
// it also tracks which criterion of the current item the next answer goes to.
type Session struct {
	ws      Workspace
	history *History
	rec     Recorder
	cursor  int
	focus   int
}

// NewSession resumes from prior records: the cursor starts at the first
// pending item, then the first held one, else the last item.
func NewSession(ws Workspace, prior []Annotation, rec Recorder) (*Session, error) {
	if err := checkRecords(ws, prior); err != nil {
		return nil, err
	}
	s := &Session{ws: ws, history: Replay(ws.Task.Scheme(), prior), rec: rec}
	s.moveTo(s.resumeIndex())
	return s, nil
}

// Task returns the task being annotated.
func (s *Session) Task() Task { return s.ws.Task }

// Current returns the item under the cursor.
func (s *Session) Current() Item { return s.ws.Items[s.cursor] }

// CurrentState returns the state of the item under the cursor.
func (s *Session) CurrentState() State { return s.history.State(s.Current().ID) }

// Position returns the 1-based cursor position and the item count.
func (s *Session) Position() (n, total int) { return s.cursor + 1, len(s.ws.Items) }

// Focus returns the index of the criterion the next answer goes to; it is 0
// for a single-choice task.
func (s *Session) Focus() int { return s.focus }

// Labeled counts items that currently carry a label or a complete checklist.
func (s *Session) Labeled() int {
	count := 0
	for _, it := range s.ws.Items {
		if s.history.State(it.ID).Status == StatusLabeled {
			count++
		}
	}
	return count
}

// Choose records what key means under the task's scheme: a label, which moves
// on, or an answer to the focused criterion, which moves on once the checklist
// is complete.
func (s *Session) Choose(key rune) error {
	switch scheme := s.ws.Task.Scheme().(type) {
	case SingleChoice:
		return s.chooseLabel(scheme, key)
	case Checklist:
		return s.answer(scheme, key)
	}
	return fmt.Errorf("%w: key %q", ErrUnknownLabel, key)
}

// FocusNext moves the focus to the following criterion; it stays on the last.
func (s *Session) FocusNext() {
	if s.focus < s.criteriaCount()-1 {
		s.focus++
	}
}

// FocusPrev moves the focus to the preceding criterion; it stays on the first.
func (s *Session) FocusPrev() {
	if s.focus > 0 {
		s.focus--
	}
}

// Hold defers the current item, then moves on.
func (s *Session) Hold() error {
	if err := s.record(Annotation{Action: ActionHold}); err != nil {
		return err
	}
	s.Next()
	return nil
}

// SetNote replaces the current item's note.
func (s *Session) SetNote(text string) error {
	return s.record(Annotation{Action: ActionNote, Note: text})
}

// Undo reverts the most recent effective record and moves the cursor to its item.
func (s *Session) Undo() error {
	id, ok := s.history.LastItemID()
	if !ok {
		return ErrNothingToUndo
	}
	idx, err := s.indexOf(id)
	if err != nil {
		return err
	}
	if err := s.append(Annotation{ItemID: id, Action: ActionUndo}); err != nil {
		return err
	}
	s.moveTo(idx)
	return nil
}

// Next moves to the following item; it stays on the last one.
func (s *Session) Next() { s.moveTo(min(s.cursor+1, len(s.ws.Items)-1)) }

// Prev moves to the preceding item; it stays on the first one.
func (s *Session) Prev() { s.moveTo(max(s.cursor-1, 0)) }

func (s *Session) chooseLabel(scheme SingleChoice, key rune) error {
	l, ok := scheme.LabelByKey(key)
	if !ok {
		return fmt.Errorf("%w: key %q", ErrUnknownLabel, key)
	}
	if err := s.record(Annotation{Action: ActionLabel, Label: l.Name}); err != nil {
		return err
	}
	s.Next()
	return nil
}

func (s *Session) answer(scheme Checklist, key rune) error {
	a, ok := scheme.AnswerByKey(key)
	if !ok {
		return fmt.Errorf("%w: key %q", ErrUnknownLabel, key)
	}
	criterion := scheme.criteria[s.focus].Name
	if err := s.record(Annotation{Action: ActionAnswer, Criterion: criterion, Label: a.Name}); err != nil {
		return err
	}
	answers := s.CurrentState().Answers
	if scheme.Complete(answers) {
		s.Next()
		return nil
	}
	s.focus = scheme.firstUnanswered(answers, s.focus+1)
	return nil
}

// moveTo puts the cursor on item idx and the focus on its first unanswered criterion.
func (s *Session) moveTo(idx int) {
	s.cursor = idx
	s.focus = 0
	if c, ok := s.ws.Task.Scheme().(Checklist); ok {
		s.focus = max(c.firstUnanswered(s.CurrentState().Answers, 0), 0)
	}
}

func (s *Session) criteriaCount() int {
	switch scheme := s.ws.Task.Scheme().(type) {
	case Checklist:
		return len(scheme.criteria)
	case SingleChoice:
	}
	return 0
}

func (s *Session) record(a Annotation) error {
	a.ItemID = s.Current().ID
	return s.append(a)
}

func (s *Session) append(a Annotation) error {
	if err := s.rec.Append(a); err != nil {
		return fmt.Errorf("record %s for %q: %w", a.Action, a.ItemID, err)
	}
	s.history.Apply(a)
	return nil
}

func (s *Session) indexOf(id string) (int, error) {
	for i, it := range s.ws.Items {
		if it.ID == id {
			return i, nil
		}
	}
	return 0, errCursorOutOfBand
}

func (s *Session) resumeIndex() int {
	for _, want := range []Status{StatusPending, StatusHeld} {
		for i, it := range s.ws.Items {
			if s.history.State(it.ID).Status == want {
				return i
			}
		}
	}
	return len(s.ws.Items) - 1
}

// checkRecords rejects prior records about unknown items or that the task's
// scheme cannot have produced, such as a label in a checklist task.
func checkRecords(ws Workspace, prior []Annotation) error {
	known := map[string]bool{}
	for _, it := range ws.Items {
		known[it.ID] = true
	}
	for i, a := range prior {
		if !known[a.ItemID] {
			return fmt.Errorf("%w: %q", ErrUnknownItem, a.ItemID)
		}
		if !fits(ws.Task.Scheme(), a) {
			return fmt.Errorf("%w: record %d (%s %q) under a %s task", ErrRecordMismatch, i+1, a.Action, a.ItemID, schemeName(ws.Task.Scheme()))
		}
	}
	return nil
}

func fits(scheme Scheme, a Annotation) bool {
	switch a.Action {
	case ActionLabel:
		sc, ok := scheme.(SingleChoice)
		return ok && sc.hasLabel(a.Label)
	case ActionAnswer:
		c, ok := scheme.(Checklist)
		return ok && c.hasCriterion(a.Criterion) && hasLabel(c.answers, a.Label)
	case ActionHold, ActionNote, ActionUndo:
	}
	return true
}

func schemeName(scheme Scheme) string {
	switch scheme.(type) {
	case SingleChoice:
		return "single-choice"
	case Checklist:
		return "checklist"
	}
	return "unknown"
}
