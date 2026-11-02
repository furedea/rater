package adjudication_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/agreement"
	"github.com/furedea/rater/internal/task"
)

type memoryRecorder struct{ records []task.Annotation }

func (m *memoryRecorder) Append(a task.Annotation) error {
	m.records = append(m.records, a)
	return nil
}

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

func annotator(ws task.Workspace, name string, records ...task.Annotation) adjudication.Annotator {
	return adjudication.Annotator{Name: name, History: task.Replay(ws.Task.Scheme(), records), Records: len(records)}
}

func label(id, name string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionLabel, Label: name}
}

func answer(id, criterion, value string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionAnswer, Criterion: criterion, Label: value}
}

func note(id, text string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionNote, Note: text}
}

func hold(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionHold} }

func undo(id string) task.Annotation { return task.Annotation{ItemID: id, Action: task.ActionUndo} }

func ids(items []task.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

// pairsDocket: 1 agreed, 2 disagreed, 3 held by bob, 4 labeled by alice only,
// 5 disagreed after bob's undo, 6 untouched.
func pairsDocket(t *testing.T) adjudication.Docket {
	t.Helper()
	ws := workspace(t, singleChoiceSpec(), "1", "2", "3", "4", "5", "6")
	alice := annotator(ws, "alice",
		label("1", "same"), label("2", "same"), label("3", "same"), label("4", "same"), label("5", "same"))
	bob := annotator(ws, "bob",
		label("1", "same"), label("2", "distinct"), note("2", "different files"), hold("3"),
		label("5", "same"), undo("5"), label("5", "distinct"))
	d, err := adjudication.NewDocket(ws, []adjudication.Annotator{alice, bob})
	require.NoError(t, err)
	return d
}

func TestDocketListsOnlyDisagreementsInItemOrder(t *testing.T) {
	d := pairsDocket(t)

	assert.Equal(t, []string{"2", "5"}, ids(d.Disputed()))
}

func TestDocketComparesChecklistsPerCriterion(t *testing.T) {
	ws := workspace(t, checklistSpec(), "1", "2", "3")
	alice := annotator(ws, "alice",
		answer("1", "concrete", "yes"), answer("1", "scoped", "yes"),
		answer("2", "concrete", "no"), answer("2", "scoped", "yes"),
		answer("3", "concrete", "no"), answer("3", "scoped", "yes"))
	bob := annotator(ws, "bob",
		answer("1", "concrete", "yes"), answer("1", "scoped", "yes"),
		answer("2", "concrete", "no"), answer("2", "scoped", "no"),
		answer("3", "concrete", "yes"))

	d, err := adjudication.NewDocket(ws, []adjudication.Annotator{alice, bob})
	require.NoError(t, err)

	assert.Equal(t, []string{"2"}, ids(d.Disputed()), "2 fails either way but differs on scoped; 3 is incomplete")
}

func TestDocketNeedsTwoAnnotators(t *testing.T) {
	ws := workspace(t, singleChoiceSpec(), "1")

	_, err := adjudication.NewDocket(ws, []adjudication.Annotator{annotator(ws, "alice", label("1", "same"))})

	assert.ErrorIs(t, err, agreement.ErrTooFewAnnotators)
}

func TestDocketShowsEachAnnotatorsStateOfAnItem(t *testing.T) {
	d := pairsDocket(t)

	got := d.Judgments("2")

	require.Len(t, got, 2)
	assert.Equal(t, "alice", got[0].Annotator)
	assert.Equal(t, "same", got[0].State.Label)
	assert.Equal(t, "bob", got[1].Annotator)
	assert.Equal(t, "distinct", got[1].State.Label)
	assert.Equal(t, "different files", got[1].State.Note)
}

func TestOpenWalksDisputedItemsOnly(t *testing.T) {
	d := pairsDocket(t)
	rec := &memoryRecorder{}

	s, err := d.Open(nil, rec)
	require.NoError(t, err)

	assert.Equal(t, "2", s.Current().ID)
	n, total := s.Position()
	assert.Equal(t, 1, n)
	assert.Equal(t, 2, total)
	require.NoError(t, s.Choose('d'))
	assert.Equal(t, "5", s.Current().ID)
	assert.Equal(t, 1, s.Labeled(), "one disagreement resolved")
	assert.Equal(t, []task.Annotation{label("2", "distinct")}, rec.records)
}

func TestOpenResumesAtFirstUndecidedDisagreementAndUndoes(t *testing.T) {
	d := pairsDocket(t)
	prior := []task.Annotation{label("2", "same"), note("2", "same intent")}

	s, err := d.Open(prior, &memoryRecorder{})
	require.NoError(t, err)

	assert.Equal(t, "5", s.Current().ID)
	assert.Equal(t, 1, s.Labeled())
	require.NoError(t, s.Undo())
	require.NoError(t, s.Undo())
	assert.Equal(t, "2", s.Current().ID)
	assert.Equal(t, 0, s.Labeled())
}

func TestOpenKeepsDecisionsOnItemsNoLongerDisputed(t *testing.T) {
	d := pairsDocket(t)

	s, err := d.Open([]task.Annotation{label("1", "distinct")}, &memoryRecorder{})
	require.NoError(t, err)

	assert.Equal(t, 0, s.Labeled(), "a decision on agreed item 1 does not count as resolved")
}

func TestOpenRejectsDecisionOnUnknownItem(t *testing.T) {
	d := pairsDocket(t)

	_, err := d.Open([]task.Annotation{label("9", "same")}, &memoryRecorder{})

	assert.ErrorIs(t, err, task.ErrUnknownItem)
}

func TestOpenWithoutDisagreements(t *testing.T) {
	ws := workspace(t, singleChoiceSpec(), "1")
	d, err := adjudication.NewDocket(ws, []adjudication.Annotator{
		annotator(ws, "alice", label("1", "same")), annotator(ws, "bob", label("1", "same")),
	})
	require.NoError(t, err)

	_, err = d.Open(nil, &memoryRecorder{})

	assert.ErrorIs(t, err, adjudication.ErrNoDisagreements)
}
