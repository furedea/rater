package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
	"github.com/furedea/rater/internal/tui"
)

// adjudicationRun is one adjudicator's run over a fixed set of annotations.
type adjudicationRun struct {
	tm  *teatest.TestModel
	log *store.Log
}

func pairsWorkspace(t *testing.T) task.Workspace {
	t.Helper()
	tk, err := task.NewTask(task.Spec{
		Name:   "pairs",
		Fields: []task.Field{{Name: "a", Title: "Comment A"}},
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	})
	require.NoError(t, err)
	ws, err := task.NewWorkspace(tk, []task.Item{
		{ID: "1", Values: map[string]string{"a": "first text"}},
		{ID: "2", Values: map[string]string{"a": "second text"}},
		{ID: "3", Values: map[string]string{"a": "third text"}},
	})
	require.NoError(t, err)
	return ws
}

func labelRecord(id, name string) task.Annotation {
	return task.Annotation{ItemID: id, Action: task.ActionLabel, Label: name}
}

// pairsAnnotators agree on 2 and disagree on 1 and 3; bob explains item 1.
func pairsAnnotators(ws task.Workspace) []adjudication.Annotator {
	replay := func(name string, records ...task.Annotation) adjudication.Annotator {
		return adjudication.Annotator{Name: name, History: task.Replay(ws.Task.Scheme(), records), Records: len(records)}
	}
	return []adjudication.Annotator{
		replay("alice", labelRecord("1", "same"), labelRecord("2", "same"), labelRecord("3", "same")),
		replay("bob", labelRecord("1", "distinct"),
			task.Annotation{ItemID: "1", Action: task.ActionNote, Note: "renamed helper"},
			labelRecord("2", "same"), labelRecord("3", "distinct")),
	}
}

func pairsDocket(t *testing.T) adjudication.Docket {
	t.Helper()
	ws := pairsWorkspace(t)
	return newDocket(t, ws, pairsAnnotators(ws))
}

func newDocket(t *testing.T, ws task.Workspace, annotators []adjudication.Annotator) adjudication.Docket {
	t.Helper()
	d, err := adjudication.NewDocket(ws, annotators)
	require.NoError(t, err)
	return d
}

func startAdjudication(t *testing.T, dir string, docket adjudication.Docket) *adjudicationRun {
	t.Helper()
	who, err := task.NewAnnotator("carol")
	require.NoError(t, err)
	log, prior, err := store.OpenAdjudicationLog(dir, who)
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	session, err := docket.Open(prior, log)
	require.NoError(t, err)
	tm := teatest.NewTestModel(t, tui.NewAdjudication(session, docket),
		teatest.WithInitialTermSize(termWidth, 30),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	)
	return &adjudicationRun{tm: tm, log: log}
}

func (r *adjudicationRun) finish(t *testing.T) string {
	t.Helper()
	r.tm.Send(keyEsc)
	final, ok := r.tm.FinalModel(t, teatest.WithFinalTimeout(finalWait)).(tui.Model)
	require.True(t, ok)
	require.NoError(t, r.log.Close())
	return ansi.Strip(final.View().Content)
}

func TestAdjudicationShowsEachAnnotatorsLabelAndNote(t *testing.T) {
	r := startAdjudication(t, t.TempDir(), pairsDocket(t))

	screen := r.finish(t)

	for _, want := range []string{"1 / 2", "resolved 0 / 2", "first text", "alice: same", "bob: distinct", "renamed helper"} {
		assert.Contains(t, screen, want)
	}
	assert.NotContains(t, screen, "second text", "agreed item 2 is not shown")
}

func TestAdjudicationResolvesOneDisagreementAndResumes(t *testing.T) {
	docket := pairsDocket(t)
	dir := t.TempDir()
	first := startAdjudication(t, dir, docket)

	first.tm.Type("d")
	screen := first.finish(t)

	assert.Contains(t, screen, "resolved 1 / 2")
	assert.Contains(t, screen, "third text", "resolving moves to the next disagreement")

	second := startAdjudication(t, dir, docket)
	resumed := second.finish(t)

	assert.Contains(t, resumed, "2 / 2")
	assert.Contains(t, resumed, "resolved 1 / 2")
	assert.Contains(t, resumed, "third text")
}

func TestAdjudicationUndoReopensTheDecision(t *testing.T) {
	r := startAdjudication(t, t.TempDir(), pairsDocket(t))

	r.tm.Type("d")
	r.tm.Send(keyUndo)
	screen := r.finish(t)

	assert.Contains(t, screen, "resolved 0 / 2")
	assert.Contains(t, screen, "first text")
}

func TestAdjudicationOfChecklistShowsAnswersPerAnnotator(t *testing.T) {
	tk, err := task.NewTask(task.Spec{
		Name:   "criteria",
		Fields: []task.Field{{Name: "sentence", Title: "Guideline sentence"}},
		Checklist: &task.ChecklistSpec{
			Criteria: []task.Criterion{{Name: "specific", Title: "Names a construct"}, {Name: "scoped", Title: "States where"}},
			Answers:  []task.Label{{Key: 'y', Name: "yes"}, {Key: 'n', Name: "no"}},
			Rule:     task.AllPass{Pass: "yes", Fail: "no"},
		},
	})
	require.NoError(t, err)
	ws, err := task.NewWorkspace(tk, []task.Item{{ID: "g-1", Values: map[string]string{"sentence": "Wrap errors."}}})
	require.NoError(t, err)
	answer := func(criterion, value string) task.Annotation {
		return task.Annotation{ItemID: "g-1", Action: task.ActionAnswer, Criterion: criterion, Label: value}
	}
	annotators := []adjudication.Annotator{
		{Name: "alice", History: task.Replay(tk.Scheme(), []task.Annotation{answer("specific", "yes"), answer("scoped", "yes")})},
		{Name: "bob", History: task.Replay(tk.Scheme(), []task.Annotation{answer("specific", "yes"), answer("scoped", "no")})},
	}
	r := startAdjudication(t, t.TempDir(), newDocket(t, ws, annotators))

	r.tm.Type("y")
	screen := r.finish(t)

	assert.Regexp(t, `final\s+alice\s+bob`, screen)
	assert.Regexp(t, `Names a construct\s+yes\s+yes\s+yes`, screen)
	assert.Regexp(t, `States where\s+·\s+yes\s+no`, screen)
	assert.Regexp(t, `verdict\s+undecided\s+pass\s+fail`, screen)
}
