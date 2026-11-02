package task_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

type fakeRecorder struct {
	records []task.Annotation
	err     error
}

func (f *fakeRecorder) Append(a task.Annotation) error {
	if f.err != nil {
		return f.err
	}
	f.records = append(f.records, a)
	return nil
}

func newWorkspace(t *testing.T, ids ...string) task.Workspace {
	t.Helper()
	tk, err := task.NewTask(validSpec())
	require.NoError(t, err)
	items := make([]task.Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, task.Item{ID: id, Values: map[string]string{"a": id, "b": id}})
	}
	ws, err := task.NewWorkspace(tk, items)
	require.NoError(t, err)
	return ws
}

func newSession(t *testing.T, prior []task.Annotation, ids ...string) (*task.Session, *fakeRecorder) {
	t.Helper()
	rec := &fakeRecorder{}
	s, err := task.NewSession(newWorkspace(t, ids...), prior, rec)
	require.NoError(t, err)
	return s, rec
}

func TestSessionChooseRecordsAndAdvances(t *testing.T) {
	s, rec := newSession(t, nil, "1", "2")

	require.NoError(t, s.Choose('s'))

	require.Len(t, rec.records, 1)
	assert.Equal(t, task.Annotation{ItemID: "1", Action: task.ActionLabel, Label: "same"}, rec.records[0])
	assert.Equal(t, "2", s.Current().ID)
	assert.Equal(t, 1, s.Labeled())
}

func TestSessionChooseUnknownKey(t *testing.T) {
	s, rec := newSession(t, nil, "1")

	err := s.Choose('x')

	require.ErrorIs(t, err, task.ErrUnknownLabel)
	assert.Empty(t, rec.records)
}

func TestSessionRecorderFailureLeavesStateUntouched(t *testing.T) {
	s, rec := newSession(t, nil, "1", "2")
	rec.err = errors.New("disk full")

	err := s.Choose('s')

	require.ErrorContains(t, err, "disk full")
	assert.Equal(t, "1", s.Current().ID)
	assert.Equal(t, task.StatusPending, s.CurrentState().Status)
}

func TestSessionHoldAndNote(t *testing.T) {
	s, _ := newSession(t, nil, "1", "2")

	require.NoError(t, s.SetNote("odd"))
	require.NoError(t, s.Hold())
	s.Prev()

	assert.Equal(t, task.State{Status: task.StatusHeld, Note: "odd"}, s.CurrentState())
}

func TestSessionNoteSurvivesRelabel(t *testing.T) {
	s, _ := newSession(t, nil, "1")

	require.NoError(t, s.SetNote("why"))
	require.NoError(t, s.Choose('d'))

	assert.Equal(t, task.State{Status: task.StatusLabeled, Label: "distinct", Note: "why"}, s.CurrentState())
}

func TestSessionUndo(t *testing.T) {
	tests := []struct {
		name      string
		act       func(*task.Session) error
		wantItem  string
		wantState task.State
	}{
		{
			name:      "undo label returns to its item",
			act:       func(s *task.Session) error { return s.Choose('s') },
			wantItem:  "1",
			wantState: task.State{},
		},
		{
			name:      "undo hold returns to its item",
			act:       func(s *task.Session) error { return s.Hold() },
			wantItem:  "1",
			wantState: task.State{},
		},
		{
			name:      "undo note keeps label",
			act:       func(s *task.Session) error { return s.SetNote("x") },
			wantItem:  "1",
			wantState: task.State{Status: task.StatusLabeled, Label: "same"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, rec := newSession(t, nil, "1", "2")
			if tt.wantState.Status == task.StatusLabeled {
				require.NoError(t, s.Choose('s'))
				s.Prev()
			}
			require.NoError(t, tt.act(s))

			require.NoError(t, s.Undo())

			assert.Equal(t, tt.wantItem, s.Current().ID)
			assert.Equal(t, tt.wantState, s.CurrentState())
			assert.Equal(t, task.ActionUndo, rec.records[len(rec.records)-1].Action)
		})
	}
}

func TestSessionUndoIsGlobalAndRepeatable(t *testing.T) {
	s, _ := newSession(t, nil, "1", "2", "3")
	require.NoError(t, s.Choose('s'))
	require.NoError(t, s.Choose('d'))

	require.NoError(t, s.Undo())
	assert.Equal(t, "2", s.Current().ID)
	require.NoError(t, s.Undo())
	assert.Equal(t, "1", s.Current().ID)

	assert.ErrorIs(t, s.Undo(), task.ErrNothingToUndo)
	assert.Equal(t, 0, s.Labeled())
}

func TestSessionNavigationStaysInRange(t *testing.T) {
	s, _ := newSession(t, nil, "1", "2")

	s.Prev()
	n, total := s.Position()
	assert.Equal(t, [2]int{1, 2}, [2]int{n, total})

	s.Next()
	s.Next()
	n, total = s.Position()
	assert.Equal(t, [2]int{2, 2}, [2]int{n, total})
}

func TestSessionResume(t *testing.T) {
	label := func(id string) task.Annotation {
		return task.Annotation{ItemID: id, Action: task.ActionLabel, Label: "same"}
	}
	hold := func(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionHold} }
	undo := func(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionUndo} }
	tests := []struct {
		name        string
		prior       []task.Annotation
		wantCurrent string
		wantLabeled int
	}{
		{"fresh log starts at first item", nil, "1", 0},
		{"skips labeled items", []task.Annotation{label("1")}, "2", 1},
		{"undone label is pending again", []task.Annotation{label("1"), undo("1")}, "1", 0},
		{"pending wins over held", []task.Annotation{hold("1"), label("2")}, "3", 1},
		{"held revisited when nothing pending", []task.Annotation{hold("1"), label("2"), label("3")}, "1", 2},
		{"all done lands on last", []task.Annotation{label("1"), label("2"), label("3")}, "3", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newSession(t, tt.prior, "1", "2", "3")

			assert.Equal(t, tt.wantCurrent, s.Current().ID)
			assert.Equal(t, tt.wantLabeled, s.Labeled())
		})
	}
}

func TestSessionResumeThenUndoUsesReplayedTrail(t *testing.T) {
	prior := []task.Annotation{
		{ItemID: "1", Action: task.ActionLabel, Label: "same"},
		{ItemID: "2", Action: task.ActionLabel, Label: "same"},
	}
	s, _ := newSession(t, prior, "1", "2", "3")

	require.NoError(t, s.Undo())

	assert.Equal(t, "2", s.Current().ID)
}

func TestNewSessionRejectsUnknownItemInLog(t *testing.T) {
	prior := []task.Annotation{{ItemID: "ghost", Action: task.ActionHold}}

	_, err := task.NewSession(newWorkspace(t, "1"), prior, &fakeRecorder{})

	assert.ErrorIs(t, err, task.ErrUnknownItem)
}
