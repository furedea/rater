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
	errCursorOutOfBand = errors.New("cursor out of range")
)

// Session is one annotator working through a workspace.
type Session struct {
	ws      Workspace
	history *History
	rec     Recorder
	cursor  int
}

// NewSession resumes from prior records: the cursor starts at the first
// pending item, then the first held one, else the last item.
func NewSession(ws Workspace, prior []Annotation, rec Recorder) (*Session, error) {
	known := map[string]bool{}
	for _, it := range ws.Items {
		known[it.ID] = true
	}
	for _, a := range prior {
		if !known[a.ItemID] {
			return nil, fmt.Errorf("%w: %q", ErrUnknownItem, a.ItemID)
		}
	}
	s := &Session{ws: ws, history: Replay(prior), rec: rec}
	s.cursor = s.resumeIndex()
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

// Labeled counts items that currently carry a label.
func (s *Session) Labeled() int {
	count := 0
	for _, it := range s.ws.Items {
		if s.history.State(it.ID).Status == StatusLabeled {
			count++
		}
	}
	return count
}

// Choose labels the current item with the label bound to key, then moves on.
func (s *Session) Choose(key rune) error {
	l, ok := s.ws.Task.LabelByKey(key)
	if !ok {
		return fmt.Errorf("%w: key %q", ErrUnknownLabel, key)
	}
	if err := s.record(Annotation{Action: ActionLabel, Label: l.Name}); err != nil {
		return err
	}
	s.Next()
	return nil
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
	s.cursor = idx
	return nil
}

// Next moves to the following item; it stays on the last one.
func (s *Session) Next() {
	if s.cursor < len(s.ws.Items)-1 {
		s.cursor++
	}
}

// Prev moves to the preceding item; it stays on the first one.
func (s *Session) Prev() {
	if s.cursor > 0 {
		s.cursor--
	}
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
