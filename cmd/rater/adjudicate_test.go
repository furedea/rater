package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/task"
)

// choosing returns a screen that types keys into the session and records
// where each run started.
func choosing(started *[]string, keys ...rune) screen {
	return func(s *task.Session, _ adjudication.Docket) error {
		*started = append(*started, s.Current().ID)
		for _, k := range keys {
			if err := s.Choose(k); err != nil {
				return err
			}
		}
		return nil
	}
}

func readGoldLines(t *testing.T, dir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "gold.jsonl")) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	var lines []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var line map[string]any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
		lines = append(lines, line)
	}
	require.NoError(t, sc.Err())
	return lines
}

func adjudicateArgs(dir string) []string { return []string{"--task", dir, "--as", "dave"} }

func TestAdjudicateWritesGoldOnQuit(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	var started []string

	require.NoError(t, adjudicate(adjudicateArgs(dir), choosing(&started, 's'), &bytes.Buffer{}))

	assert.Equal(t, []string{"2"}, started, "item 1 is agreed, so the first disagreement is 2")
	gold := readGoldLines(t, dir)
	require.Len(t, gold, 2)
	assert.Equal(t, "1", gold[0]["id"])
	assert.Equal(t, "agreed", gold[0]["source"])
	assert.Equal(t, "2", gold[1]["id"])
	assert.Equal(t, "same", gold[1]["label"])
	assert.Equal(t, "adjudicated", gold[1]["source"])
	assert.Equal(t, "dave", gold[1]["adjudicator"])
	annotations, ok := gold[1]["annotations"].([]any)
	require.True(t, ok)
	assert.Len(t, annotations, 3, "alice, bob, and carol were compared")
}

func TestAdjudicateResumesFromTheAdjudicationLog(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	var started []string
	require.NoError(t, adjudicate(adjudicateArgs(dir), choosing(&started, 's'), &bytes.Buffer{}))

	require.NoError(t, adjudicate(adjudicateArgs(dir), choosing(&started, 'd'), &bytes.Buffer{}))

	assert.Equal(t, []string{"2", "3"}, started)
	gold := readGoldLines(t, dir)
	require.Len(t, gold, 3)
	assert.Equal(t, "distinct", gold[2]["label"])
	_, err := os.Stat(filepath.Join(dir, "adjudications", "dave.jsonl"))
	assert.NoError(t, err)
}

func TestAdjudicateReopensADecisionWhoseAnnotationsChanged(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	var started []string
	require.NoError(t, adjudicate(adjudicateArgs(dir), choosing(&started, 's'), &bytes.Buffer{}))
	carolLog := filepath.Join(dir, "annotations", "carol.jsonl")
	f, err := os.OpenFile(carolLog, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	_, err = f.WriteString(record("label 2 distinct"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	require.NoError(t, adjudicate(adjudicateArgs(dir), choosing(&started), &bytes.Buffer{}))

	assert.Equal(t, []string{"2", "2"}, started, "carol changed item 2 after it was decided")
	gold := readGoldLines(t, dir)
	require.Len(t, gold, 1, "only agreed item 1 remains in gold")
	assert.Equal(t, "1", gold[0]["id"])
}

func TestAdjudicateWritesGoldEvenWhenTheScreenFails(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	broken := func(s *task.Session, _ adjudication.Docket) error {
		require.NoError(t, s.Choose('s'))
		return errors.New("terminal lost")
	}

	err := adjudicate(adjudicateArgs(dir), broken, &bytes.Buffer{})

	require.ErrorContains(t, err, "terminal lost")
	assert.Len(t, readGoldLines(t, dir), 2, "the decision made before the failure is in gold")
}

func TestAdjudicateWithoutDisagreementsWritesAgreedGold(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, map[string]string{
		"alice": logOf("label 1 same", "label 2 distinct"),
		"bob":   logOf("label 1 same", "label 2 distinct"),
	})
	var out bytes.Buffer
	unused := func(*task.Session, adjudication.Docket) error {
		t.Fatal("no screen without disagreements")
		return nil
	}

	require.NoError(t, adjudicate(adjudicateArgs(dir), unused, &out))

	assert.Contains(t, out.String(), "no disagreements")
	assert.Len(t, readGoldLines(t, dir), 2)
}

func TestAdjudicateRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		logs    map[string]string
		args    func(dir string) []string
		wantErr string
	}{
		{"one annotator", map[string]string{"alice": logOf("label 1 same")}, adjudicateArgs, "at least two"},
		{"no adjudicator", pairsLogs(), func(dir string) []string { return []string{"--task", dir} }, "--as"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := taskDir(t, pairsTask, tt.logs)

			err := adjudicate(tt.args(dir), choosing(new([]string)), &bytes.Buffer{})

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestAdjudicateRefusesASecondAdjudicatorWithoutTouchingGold(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	var started []string
	require.NoError(t, adjudicate([]string{"--task", dir, "--as", "carol"}, choosing(&started, 's'), &bytes.Buffer{}))
	goldPath := filepath.Join(dir, "gold.jsonl")
	before, err := os.ReadFile(goldPath) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)

	err = adjudicate(adjudicateArgs(dir), choosing(&started), &bytes.Buffer{})

	require.ErrorContains(t, err, "carol")
	assert.Equal(t, []string{"2"}, started, "dave never sees the screen")
	after, err := os.ReadFile(goldPath) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	assert.Equal(t, before, after, "carol's decisions stay in gold")
	_, err = os.Stat(filepath.Join(dir, "adjudications", "dave.jsonl"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestAdjudicateHelpStatesTheOneAdjudicatorRule(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	require.NoError(t, adjudicate([]string{"--help"}, choosing(new([]string)), &out))

	assert.Contains(t, out.String(), "one adjudicator")
	assert.Contains(t, out.String(), "adjudications/")
}
