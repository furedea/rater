package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
)

func labeledAs(l, note string) task.State {
	return task.State{Status: task.StatusLabeled, Label: l, Note: note}
}

func pairsGold() []adjudication.Gold {
	return []adjudication.Gold{
		{
			ItemID: "1", Source: adjudication.SourceAgreed, Final: labeledAs("same", ""),
			Judgments: []adjudication.Judgment{
				{Annotator: "alice", State: labeledAs("same", ""), Records: 5},
				{Annotator: "bob", State: labeledAs("same", "obvious"), Records: 7},
			},
		},
		{
			ItemID: "2", Source: adjudication.SourceAdjudicated, Final: labeledAs("distinct", "renamed file"),
			Adjudicator: "carol",
			Judgments: []adjudication.Judgment{
				{Annotator: "alice", State: labeledAs("same", ""), Records: 5},
				{Annotator: "bob", State: labeledAs("distinct", ""), Records: 7},
				{Annotator: "dave", State: task.State{Status: task.StatusHeld}, Records: 1},
			},
		},
	}
}

func readGold(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "gold.jsonl")) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	return string(data)
}

func TestWriteGoldWritesOneLinePerItem(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, store.WriteGold(dir, pairsGold()))

	want := `{"id":"1","label":"same","source":"agreed",` +
		`"annotations":[{"annotator":"alice","status":"labeled","label":"same","records":5},` +
		`{"annotator":"bob","status":"labeled","label":"same","note":"obvious","records":7}]}` + "\n" +
		`{"id":"2","label":"distinct","source":"adjudicated","adjudicator":"carol","note":"renamed file",` +
		`"annotations":[{"annotator":"alice","status":"labeled","label":"same","records":5},` +
		`{"annotator":"bob","status":"labeled","label":"distinct","records":7},` +
		`{"annotator":"dave","status":"held","records":1}]}` + "\n"
	assert.Equal(t, want, readGold(t, dir))
}

func TestWriteGoldOfChecklistHasSortedAnswersAndVerdict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	answers := map[string]string{"scoped": "no", "concrete": "yes", "actionable": "yes"}
	golds := []adjudication.Gold{{
		ItemID: "g-1", Source: adjudication.SourceAdjudicated, Adjudicator: "carol",
		Final:   task.State{Status: task.StatusLabeled, Answers: answers},
		Verdict: task.VerdictFail,
		Judgments: []adjudication.Judgment{
			{Annotator: "alice", State: task.State{Status: task.StatusPending, Answers: map[string]string{"scoped": "no"}}, Records: 1},
		},
	}}

	require.NoError(t, store.WriteGold(dir, golds))

	want := `{"id":"g-1","answers":{"actionable":"yes","concrete":"yes","scoped":"no"},"verdict":"fail",` +
		`"source":"adjudicated","adjudicator":"carol",` +
		`"annotations":[{"annotator":"alice","status":"pending","answers":{"scoped":"no"},"records":1}]}` + "\n"
	assert.Equal(t, want, readGold(t, dir))
}

func TestWriteGoldReplacesTheWholeFileAndLeavesNoTemporaryFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, store.WriteGold(dir, pairsGold()))

	require.NoError(t, store.WriteGold(dir, pairsGold()[:1]))

	assert.Equal(t, 1, countLines([]byte(readGold(t, dir))))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "gold.jsonl", entries[0].Name())
}

func TestWriteGoldWithNothingResolvedIsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	require.NoError(t, store.WriteGold(dir, nil))

	assert.Empty(t, readGold(t, dir))
}

func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}
