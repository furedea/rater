package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func newChecklistWorkspace(t *testing.T, ids ...string) task.Workspace {
	t.Helper()
	tk, err := task.NewTask(checklistSpec())
	require.NoError(t, err)
	items := make([]task.Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, task.Item{ID: id, Values: map[string]string{"sentence": id}})
	}
	ws, err := task.NewWorkspace(tk, items)
	require.NoError(t, err)
	return ws
}

func newChecklistSession(t *testing.T, prior []task.Annotation, ids ...string) (*task.Session, *fakeRecorder) {
	t.Helper()
	rec := &fakeRecorder{}
	s, err := task.NewSession(newChecklistWorkspace(t, ids...), prior, rec)
	require.NoError(t, err)
	return s, rec
}

func answer(id, criterion, value string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionAnswer, Criterion: criterion, Label: value}
}

func TestChecklistAnswerRecordsTheFocusedCriterion(t *testing.T) {
	t.Parallel()

	s, rec := newChecklistSession(t, nil, "1", "2")

	require.NoError(t, s.Choose('n'))

	assert.Equal(t, []task.Annotation{answer("1", "specific", "no")}, rec.records)
	assert.Equal(t, "1", s.Current().ID, "an incomplete checklist stays on its item")
	assert.Equal(t, 1, s.Focus(), "focus moves to the next unanswered criterion")
	assert.Equal(t, map[string]string{"specific": "no"}, s.CurrentState().Answers)
	assert.Equal(t, task.StatusPending, s.CurrentState().Status)
}

func TestChecklistCompletingAllCriteriaAdvances(t *testing.T) {
	t.Parallel()

	s, _ := newChecklistSession(t, nil, "1", "2")

	for _, key := range "yyy" {
		require.NoError(t, s.Choose(key))
	}

	assert.Equal(t, "2", s.Current().ID)
	assert.Equal(t, 0, s.Focus())
	assert.Equal(t, 1, s.Labeled())
	s.Prev()
	assert.Equal(t, task.StatusLabeled, s.CurrentState().Status)
}

func TestChecklistUnsureAnswerCountsAsAnswered(t *testing.T) {
	t.Parallel()

	s, rec := newChecklistSession(t, nil, "1", "2")

	for _, key := range "yy?" {
		require.NoError(t, s.Choose(key))
	}

	assert.Equal(t, "2", s.Current().ID, "an unsure answer completes the checklist")
	resumed, err := task.NewSession(newChecklistWorkspace(t, "1", "2"), rec.records, &fakeRecorder{})
	require.NoError(t, err)
	assert.Equal(t, "2", resumed.Current().ID, "resume skips the completed item")
	resumed.Prev()
	assert.Equal(t, task.StatusLabeled, resumed.CurrentState().Status)
	cl := checklistOf(t, resumed.Task())
	assert.Equal(t, task.VerdictUndecided, cl.Verdict(resumed.CurrentState().Answers))
}

func TestChecklistFocusSkipsAnsweredCriteria(t *testing.T) {
	t.Parallel()

	s, _ := newChecklistSession(t, nil, "1")
	s.FocusNext()
	require.NoError(t, s.Choose('y'))
	require.Equal(t, 2, s.Focus())

	require.NoError(t, s.Choose('y'))

	assert.Equal(t, 0, s.Focus(), "focus wraps to the first unanswered criterion")
}

func TestChecklistFocusStaysInRange(t *testing.T) {
	t.Parallel()

	s, _ := newChecklistSession(t, nil, "1")

	s.FocusPrev()
	assert.Equal(t, 0, s.Focus())
	for range 5 {
		s.FocusNext()
	}
	assert.Equal(t, 2, s.Focus())
}

func TestChecklistReanswerReplacesTheAnswer(t *testing.T) {
	t.Parallel()

	s, rec := newChecklistSession(t, nil, "1", "2")
	for _, key := range "yyy" {
		require.NoError(t, s.Choose(key))
	}
	s.Prev()
	s.FocusNext()

	require.NoError(t, s.Choose('n'))

	assert.Equal(t, answer("1", "checkable", "no"), rec.records[len(rec.records)-1])
	s.Prev()
	assert.Equal(t, "no", s.CurrentState().Answers["checkable"])
}

func TestChecklistUndoRevertsOneAnswer(t *testing.T) {
	t.Parallel()

	s, rec := newChecklistSession(t, nil, "1", "2")
	for _, key := range "yyy" {
		require.NoError(t, s.Choose(key))
	}

	require.NoError(t, s.Undo())

	assert.Equal(t, "1", s.Current().ID)
	assert.Equal(t, map[string]string{"specific": "yes", "checkable": "yes"}, s.CurrentState().Answers)
	assert.Equal(t, task.StatusPending, s.CurrentState().Status)
	assert.Equal(t, 2, s.Focus(), "focus returns to the reverted criterion")
	assert.Equal(t, task.ActionUndo, rec.records[len(rec.records)-1].Action)
}

func TestChecklistHoldKeepsAnswers(t *testing.T) {
	t.Parallel()

	s, _ := newChecklistSession(t, nil, "1", "2")
	require.NoError(t, s.Choose('y'))

	require.NoError(t, s.Hold())
	s.Prev()

	assert.Equal(t, task.StatusHeld, s.CurrentState().Status)
	assert.Equal(t, map[string]string{"specific": "yes"}, s.CurrentState().Answers)
	assert.Equal(t, 1, s.Focus())
}

func TestChecklistResume(t *testing.T) {
	t.Parallel()

	complete := []task.Annotation{answer("1", "specific", "yes"), answer("1", "checkable", "no"), answer("1", "scoped", "yes")}
	tests := []struct {
		name        string
		prior       []task.Annotation
		wantCurrent string
		wantFocus   int
		wantLabeled int
	}{
		{"complete item is skipped", complete, "2", 0, 1},
		{"partial item is resumed at its first unanswered criterion", []task.Annotation{answer("1", "specific", "yes")}, "1", 1, 0},
		{"undone answer is unanswered again", append(append([]task.Annotation(nil), complete...), task.Annotation{ItemID: "1", Action: task.ActionUndo}), "1", 2, 0},
		{"held partial item comes after pending ones", []task.Annotation{answer("1", "specific", "yes"), {ItemID: "1", Action: task.ActionHold}}, "2", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := newChecklistSession(t, tt.prior, "1", "2")

			assert.Equal(t, tt.wantCurrent, s.Current().ID)
			assert.Equal(t, tt.wantFocus, s.Focus())
			assert.Equal(t, tt.wantLabeled, s.Labeled())
		})
	}
}

func TestNewSessionRejectsRecordsThatDoNotFitTheScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		prior []task.Annotation
	}{
		{"label record in a checklist task", []task.Annotation{{ItemID: "1", Action: task.ActionLabel, Label: "yes"}}},
		{"unknown criterion", []task.Annotation{answer("1", "ghost", "yes")}},
		{"unknown answer", []task.Annotation{answer("1", "specific", "maybe")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tk, err := task.NewTask(checklistSpec())
			require.NoError(t, err)
			ws, err := task.NewWorkspace(tk, []task.Item{{ID: "1", Values: map[string]string{"sentence": "s"}}})
			require.NoError(t, err)

			_, err = task.NewSession(ws, tt.prior, &fakeRecorder{})

			assert.ErrorIs(t, err, task.ErrRecordMismatch)
		})
	}
}

func TestNewSessionRejectsAnswersInASingleChoiceTask(t *testing.T) {
	t.Parallel()

	prior := []task.Annotation{answer("1", "specific", "yes")}

	_, err := task.NewSession(newWorkspace(t, "1"), prior, &fakeRecorder{})

	assert.ErrorIs(t, err, task.ErrRecordMismatch)
}

func TestNewSessionRejectsUnknownLabelInASingleChoiceTask(t *testing.T) {
	t.Parallel()

	prior := []task.Annotation{{ItemID: "1", Action: task.ActionLabel, Label: "other"}}

	_, err := task.NewSession(newWorkspace(t, "1"), prior, &fakeRecorder{})

	assert.ErrorIs(t, err, task.ErrRecordMismatch)
}
