package adjudication_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/task"
)

func adjudicator(t *testing.T) task.Annotator {
	t.Helper()
	a, err := task.NewAnnotator("carol")
	require.NoError(t, err)
	return a
}

func goldByID(golds []adjudication.Gold) map[string]adjudication.Gold {
	m := map[string]adjudication.Gold{}
	for _, g := range golds {
		m[g.ItemID] = g
	}
	return m
}

func TestGoldHoldsAgreedAndAdjudicatedItemsInItemOrder(t *testing.T) {
	t.Parallel()

	d := pairsDocket(t)
	decisions := task.Replay(d.Scheme(), []task.Annotation{label("2", "distinct"), note("2", "renamed file")})

	golds := d.Gold(adjudicator(t), decisions)

	require.Len(t, golds, 2)
	assert.Equal(t, "1", golds[0].ItemID)
	assert.Equal(t, adjudication.SourceAgreed, golds[0].Source)
	assert.Equal(t, "same", golds[0].Final.Label)
	assert.Empty(t, golds[0].Adjudicator)
	assert.Equal(t, "2", golds[1].ItemID)
	assert.Equal(t, adjudication.SourceAdjudicated, golds[1].Source)
	assert.Equal(t, "distinct", golds[1].Final.Label)
	assert.Equal(t, "renamed file", golds[1].Final.Note)
	assert.Equal(t, "carol", golds[1].Adjudicator)
}

func TestGoldRecordsWhichAnnotationsWereCompared(t *testing.T) {
	t.Parallel()

	d := pairsDocket(t)
	decisions := task.Replay(d.Scheme(), []task.Annotation{label("2", "distinct")})

	g := goldByID(d.Gold(adjudicator(t), decisions))["2"]

	require.Len(t, g.Judgments, 2)
	assert.Equal(t, "alice", g.Judgments[0].Annotator)
	assert.Equal(t, "same", g.Judgments[0].State.Label)
	assert.Equal(t, 5, g.Judgments[0].Records, "alice's log held five records when compared")
	assert.Equal(t, "bob", g.Judgments[1].Annotator)
	assert.Equal(t, 7, g.Judgments[1].Records)
}

func TestGoldLeavesOutUnresolvedItems(t *testing.T) {
	t.Parallel()

	d := pairsDocket(t)
	decisions := task.Replay(d.Scheme(), []task.Annotation{hold("2"), label("5", "same"), undo("5")})

	got := goldByID(d.Gold(adjudicator(t), decisions))

	assert.Contains(t, got, "1")
	for _, id := range []string{"2", "3", "4", "5", "6"} {
		assert.NotContains(t, got, id, "item %s is held, single, undone, or unannotated", id)
	}
}

func TestGoldPrefersAgreementOverDecisionOnItemNoLongerDisputed(t *testing.T) {
	t.Parallel()

	d := pairsDocket(t)
	decisions := task.Replay(d.Scheme(), []task.Annotation{label("1", "distinct")})

	g := goldByID(d.Gold(adjudicator(t), decisions))["1"]

	assert.Equal(t, adjudication.SourceAgreed, g.Source)
	assert.Equal(t, "same", g.Final.Label)
}

func TestGoldOfChecklistCarriesAnswersAndDerivedVerdict(t *testing.T) {
	t.Parallel()

	ws := workspace(t, checklistSpec(), "1", "2")
	alice := annotator(ws, "alice",
		answer("1", "concrete", "yes"), answer("1", "scoped", "yes"),
		answer("2", "concrete", "yes"), answer("2", "scoped", "yes"))
	bob := annotator(ws, "bob",
		answer("1", "concrete", "yes"), answer("1", "scoped", "yes"),
		answer("2", "concrete", "yes"), answer("2", "scoped", "no"))
	d, err := adjudication.NewDocket(ws, []adjudication.Annotator{alice, bob})
	require.NoError(t, err)
	decisions := task.Replay(d.Scheme(), []task.Annotation{answer("2", "concrete", "no"), answer("2", "scoped", "no")})

	got := goldByID(d.Gold(adjudicator(t), decisions))

	assert.Equal(t, map[string]string{"concrete": "yes", "scoped": "yes"}, got["1"].Final.Answers)
	assert.Equal(t, task.VerdictPass, got["1"].Verdict)
	assert.Equal(t, map[string]string{"concrete": "no", "scoped": "no"}, got["2"].Final.Answers)
	assert.Equal(t, task.VerdictFail, got["2"].Verdict)
}

func TestGoldOmitsIncompleteChecklistDecision(t *testing.T) {
	t.Parallel()

	ws := workspace(t, checklistSpec(), "1")
	alice := annotator(ws, "alice", answer("1", "concrete", "yes"), answer("1", "scoped", "yes"))
	bob := annotator(ws, "bob", answer("1", "concrete", "no"), answer("1", "scoped", "yes"))
	d, err := adjudication.NewDocket(ws, []adjudication.Annotator{alice, bob})
	require.NoError(t, err)
	decisions := task.Replay(d.Scheme(), []task.Annotation{answer("1", "concrete", "no")})

	assert.Empty(t, d.Gold(adjudicator(t), decisions))
}
