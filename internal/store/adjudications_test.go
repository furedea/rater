package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
)

func TestAdjudicationLogKeepsDecisionsApartFromAnnotations(t *testing.T) {
	dir := t.TempDir()
	carol := mustAnnotator(t, "carol")
	l, prior, err := store.OpenAdjudicationLog(dir, carol)
	require.NoError(t, err)
	assert.Empty(t, prior)

	require.NoError(t, l.Append(task.Annotation{ItemID: "2", Action: task.ActionLabel, Label: "same"}))
	require.NoError(t, l.Close())

	data, err := os.ReadFile(filepath.Join(dir, "adjudications", "carol.jsonl")) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	assert.Contains(t, string(data), `"item":"2","annotator":"carol","action":"label","label":"same"`)
	logs, err := store.ReadLogs(dir)
	require.NoError(t, err)
	assert.Empty(t, logs, "an adjudicator is not an annotator")
}

func TestAdjudicationLogResumesPriorDecisions(t *testing.T) {
	dir := t.TempDir()
	carol := mustAnnotator(t, "carol")
	first, _, err := store.OpenAdjudicationLog(dir, carol)
	require.NoError(t, err)
	require.NoError(t, first.Append(task.Annotation{ItemID: "2", Action: task.ActionLabel, Label: "same", Basis: "b1"}))
	require.NoError(t, first.Append(task.Annotation{ItemID: "2", Action: task.ActionUndo, Basis: "b1"}))
	require.NoError(t, first.Close())

	second, prior, err := store.OpenAdjudicationLog(dir, carol)
	require.NoError(t, err)
	require.NoError(t, second.Close())

	require.Len(t, prior, 2)
	assert.Equal(t, task.ActionLabel, prior[0].Action)
	assert.Equal(t, "b1", prior[0].Basis, "the basis of a decision survives a resume")
	assert.Equal(t, task.ActionUndo, prior[1].Action)
}

func TestAdjudicationLogIsExclusivePerAdjudicator(t *testing.T) {
	dir := t.TempDir()
	carol := mustAnnotator(t, "carol")
	first, _, err := store.OpenAdjudicationLog(dir, carol)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Close() })

	_, _, err = store.OpenAdjudicationLog(dir, carol)
	require.ErrorIs(t, err, store.ErrLogInUse)

	annotating, _, err := store.OpenLog(dir, carol)
	require.NoError(t, err, "carol's own annotation log is a different file")
	require.NoError(t, annotating.Close())
}

func TestAdjudicationLogRefusesASecondAdjudicatorBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	carol, _, err := store.OpenAdjudicationLog(dir, mustAnnotator(t, "carol"))
	require.NoError(t, err)
	require.NoError(t, carol.Close())

	_, _, err = store.OpenAdjudicationLog(dir, mustAnnotator(t, "dave"))

	require.ErrorIs(t, err, store.ErrOtherAdjudicator)
	assert.ErrorContains(t, err, "carol")
	_, statErr := os.Stat(filepath.Join(dir, "adjudications", "dave.jsonl"))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "a refused adjudicator gets no log")
}

func TestAdjudicationLogLetsTheSameAdjudicatorResume(t *testing.T) {
	dir := t.TempDir()
	carol := mustAnnotator(t, "carol")
	first, _, err := store.OpenAdjudicationLog(dir, carol)
	require.NoError(t, err)
	require.NoError(t, first.Append(task.Annotation{ItemID: "2", Action: task.ActionLabel, Label: "same"}))
	require.NoError(t, first.Close())

	second, prior, err := store.OpenAdjudicationLog(dir, carol)

	require.NoError(t, err)
	require.NoError(t, second.Close())
	assert.Len(t, prior, 1)
}

func TestAdjudicationLogStartsTheFirstAdjudicator(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"missing adjudications dir", func(*testing.T, string) {}},
		{"empty adjudications dir", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(filepath.Join(dir, "adjudications"), 0o750))
		}},
		{"only a handed-over log", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(filepath.Join(dir, "adjudications"), 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "adjudications", "carol.jsonl.handed-over"), nil, 0o600))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			l, prior, err := store.OpenAdjudicationLog(dir, mustAnnotator(t, "dave"))

			require.NoError(t, err)
			require.NoError(t, l.Close())
			assert.Empty(t, prior)
		})
	}
}
