package agreement_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/task"
)

func workspace(t *testing.T, spec task.Spec, ids ...string) task.Workspace {
	t.Helper()
	tk, err := task.NewTask(spec)
	require.NoError(t, err)
	items := make([]task.Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, task.Item{ID: id, Values: map[string]string{"text": id}})
	}
	ws, err := task.NewWorkspace(tk, items)
	require.NoError(t, err)
	return ws
}

func singleChoiceSpec() task.Spec {
	return task.Spec{
		Name:   "pairs",
		Fields: []task.Field{{Name: "text"}},
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	}
}

func checklistSpec() task.Spec {
	return task.Spec{
		Name:   "criteria",
		Fields: []task.Field{{Name: "text"}},
		Checklist: &task.ChecklistSpec{
			Criteria: []task.Criterion{{Name: "concrete"}, {Name: "scoped"}},
			Answers:  []task.Label{{Key: 'y', Name: "yes"}, {Key: 'n', Name: "no"}},
			Rule:     task.AllPass{Pass: "yes", Fail: "no"},
		},
	}
}

func annotator(ws task.Workspace, name string, records ...task.Annotation) agreement.Annotator {
	return agreement.Annotator{Name: name, History: task.Replay(ws.Task.Scheme(), records)}
}

func label(id, name string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionLabel, Label: name}
}

func answer(id, criterion, value string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionAnswer, Criterion: criterion, Label: value}
}

func hold(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionHold} }

func undo(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionUndo} }

func newReport(t *testing.T, ws task.Workspace, annotators []agreement.Annotator) agreement.Report {
	t.Helper()
	r, err := agreement.NewReport(ws, annotators)
	require.NoError(t, err)
	return r
}

func TestReportNeedsTwoAnnotators(t *testing.T) {
	t.Parallel()

	ws := workspace(t, singleChoiceSpec(), "1")

	_, err := agreement.NewReport(ws, []agreement.Annotator{annotator(ws, "alice", label("1", "same"))})

	assert.ErrorIs(t, err, agreement.ErrTooFewAnnotators)
}

func TestReportComparesLatestEffectiveLabels(t *testing.T) {
	t.Parallel()

	ws := workspace(t, singleChoiceSpec(), "1", "2", "3", "4")
	alice := annotator(ws, "alice", label("1", "same"), label("2", "same"), label("3", "distinct"), hold("4"))
	bob := annotator(ws, "bob",
		label("1", "same"), label("2", "distinct"),
		label("3", "distinct"), label("3", "same"), undo("3"),
		label("4", "same"),
	)

	r := newReport(t, ws, []agreement.Annotator{alice, bob})

	assert.Equal(t, "pairs", r.Task)
	assert.Equal(t, 4, r.Items)
	assert.Equal(t, []agreement.Coverage{
		{Name: "alice", Labeled: 3, Held: 1},
		{Name: "bob", Labeled: 4},
	}, r.Annotators)
	assert.Empty(t, r.Criteria, "a single-choice task has no criteria")
	assert.Equal(t, "label", r.Overall.Name)
	require.NotNil(t, r.Overall.Pair)
	pair := r.Overall.Pair
	assert.Equal(t, 1, pair.Count("same", "distinct"), "item 2")
	assert.Equal(t, 1, pair.Count("distinct", "distinct"), "item 3 after bob's undo")
	assert.Equal(t, 1, pair.Count("same", "same"), "item 1; held item 4 is left out")
	k := pair.Kappa()
	assert.Equal(t, 3, k.Units)
	// p_o = 2/3, p_e = (2*1 + 1*2)/9 = 4/9, kappa = (2/9)/(5/9).
	assert.InDelta(t, 0.4, requireValue(t, k.Kappa), exact)
	assert.Equal(t, 3, r.Overall.Alpha.Units)
}

func TestReportWithThreeAnnotatorsHasNoPair(t *testing.T) {
	t.Parallel()

	ws := workspace(t, singleChoiceSpec(), "1", "2")
	alice := annotator(ws, "alice", label("1", "same"), label("2", "same"))
	bob := annotator(ws, "bob", label("1", "same"), label("2", "distinct"))
	carol := annotator(ws, "carol", label("1", "same"))

	r := newReport(t, ws, []agreement.Annotator{alice, bob, carol})

	assert.Nil(t, r.Overall.Pair, "kappa and the confusion matrix are for two annotators")
	assert.Equal(t, 2, r.Overall.Alpha.Units)
	assert.Equal(t, 5, r.Overall.Alpha.Values)
	assert.Equal(t, agreement.Coverage{Name: "carol", Labeled: 1, Pending: 1}, r.Annotators[2])
}

func TestReportMeasuresEachCriterionAndTheDerivedVerdict(t *testing.T) {
	t.Parallel()

	ws := workspace(t, checklistSpec(), "1", "2", "3")
	alice := annotator(ws, "alice",
		answer("1", "concrete", "yes"), answer("1", "scoped", "yes"),
		answer("2", "concrete", "no"), answer("2", "scoped", "yes"),
		answer("3", "concrete", "yes"), answer("3", "scoped", "no"),
	)
	bob := annotator(ws, "bob",
		answer("1", "concrete", "yes"), answer("1", "scoped", "no"),
		answer("2", "concrete", "no"), answer("2", "scoped", "yes"), hold("2"),
		answer("3", "concrete", "yes"),
	)

	r := newReport(t, ws, []agreement.Annotator{alice, bob})

	assert.Equal(t, []agreement.Coverage{
		{Name: "alice", Labeled: 3},
		{Name: "bob", Labeled: 1, Held: 1, Pending: 1},
	}, r.Annotators)
	require.Len(t, r.Criteria, 2)
	assert.Equal(t, "concrete", r.Criteria[0].Name)
	assert.Equal(t, "scoped", r.Criteria[1].Name)
	require.NotNil(t, r.Criteria[1].Pair)
	assert.Equal(t, 1, r.Criteria[1].Pair.Count("yes", "no"), "only item 1 is complete for both")
	assert.Equal(t, 1, r.Criteria[1].Pair.Kappa().Units)

	assert.Equal(t, "verdict", r.Overall.Name)
	require.NotNil(t, r.Overall.Pair)
	assert.Equal(t, 1, r.Overall.Pair.Count("pass", "fail"), "verdicts are recomputed with all_pass")
	assert.Equal(t, []string{"fail", "pass"}, r.Overall.Pair.Categories())
}
