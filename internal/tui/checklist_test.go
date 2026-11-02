package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/task"
)

var keyShiftDown = tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}

func startChecklist(t *testing.T) *harness {
	t.Helper()
	spec := task.Spec{
		Name:   "criteria",
		Fields: []task.Field{{Name: "sentence", Title: "Guideline sentence"}},
		Checklist: &task.ChecklistSpec{
			Criteria: []task.Criterion{
				{Name: "specific", Title: "Names a concrete construct"},
				{Name: "checkable", Title: "Can be checked in a diff"},
				{Name: "scoped", Title: "States where it applies"},
			},
			Answers: []task.Label{{Key: 'y', Name: "yes"}, {Key: 'n', Name: "no"}, {Key: '?', Name: "unsure"}},
			Rule:    task.AllPass{Pass: "yes", Fail: "no"},
		},
	}
	items := []task.Item{
		{ID: "g-1", Values: map[string]string{"sentence": "Wrap errors with context."}},
		{ID: "g-2", Values: map[string]string{"sentence": "Write good code."}},
	}
	return startSpec(t, spec, items)
}

func TestChecklistShowsCriteriaAnswersAndVerdict(t *testing.T) {
	h := startChecklist(t)

	h.tm.Type("y")
	h.tm.Type("n")
	screen := h.finish(t)

	for _, want := range []string{
		"Names a concrete construct", "Can be checked in a diff", "States where it applies",
		"[y] yes", "[n] no", "[?] unsure",
		"verdict: fail", "2 of 3 answered",
	} {
		assert.Contains(t, screen, want)
	}
	assert.Contains(t, screen, "1 / 2", "an incomplete checklist stays on its item")
	assert.Equal(t, map[string]string{"specific": "yes", "checkable": "no"}, h.replay(t).State("g-1").Answers)
}

func TestChecklistCompletesAndAdvances(t *testing.T) {
	h := startChecklist(t)

	h.tm.Type("yyy")
	screen := h.finish(t)

	assert.Contains(t, screen, "2 / 2")
	assert.Contains(t, screen, "labeled 1")
	assert.Contains(t, screen, "verdict: undecided", "the next item starts unanswered")
	assert.Equal(t, task.StatusLabeled, h.replay(t).State("g-1").Status)
}

func TestChecklistFocusKeyChoosesTheCriterion(t *testing.T) {
	h := startChecklist(t)

	h.tm.Send(keyShiftDown)
	h.tm.Type("n")
	h.finish(t)

	assert.Equal(t, map[string]string{"checkable": "no"}, h.replay(t).State("g-1").Answers)
}
