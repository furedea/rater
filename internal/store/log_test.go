package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func mustAnnotator(t *testing.T, name string) task.Annotator {
	t.Helper()
	a, err := task.NewAnnotator(name)
	require.NoError(t, err)
	return a
}

func logPath(dir string) string { return filepath.Join(dir, "annotations", "alice.jsonl") }

func openLog(t *testing.T, dir string) (*Log, []task.Annotation) {
	t.Helper()
	l, prior, err := OpenLog(dir, mustAnnotator(t, "alice"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return l, prior
}

func TestLogAppendWritesFullStampedLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l, prior := openLog(t, dir)
	assert.Empty(t, prior)
	l.now = func() time.Time { return time.Date(2026, 11, 2, 3, 4, 5, 0, time.UTC) }

	require.NoError(t, l.Append(task.Annotation{ItemID: "1", Action: task.ActionLabel, Label: "same", Note: "n"}))
	require.NoError(t, l.Append(task.Annotation{ItemID: "1", Action: task.ActionUndo}))

	got, err := os.ReadFile(logPath(dir))
	require.NoError(t, err)
	want := `{"item":"1","annotator":"alice","action":"label","label":"same","note":"n","at":"2026-11-02T03:04:05Z"}` + "\n" +
		`{"item":"1","annotator":"alice","action":"undo","at":"2026-11-02T03:04:05Z"}` + "\n"
	assert.Equal(t, want, string(got))
}

func TestLogResumeReturnsPriorRecordsAndAppends(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, _ := openLog(t, dir)
	require.NoError(t, first.Append(task.Annotation{ItemID: "1", Action: task.ActionLabel, Label: "same"}))
	require.NoError(t, first.Append(task.Annotation{ItemID: "2", Action: task.ActionHold}))
	require.NoError(t, first.Close())

	second, prior := openLog(t, dir)
	require.NoError(t, second.Append(task.Annotation{ItemID: "2", Action: task.ActionUndo}))

	require.Len(t, prior, 2)
	assert.Equal(t, task.ActionLabel, prior[0].Action)
	assert.Equal(t, "same", prior[0].Label)
	assert.Equal(t, "alice", prior[0].Annotator)
	assert.Equal(t, task.ActionHold, prior[1].Action)

	again, err := os.ReadFile(logPath(dir))
	require.NoError(t, err)
	assert.Equal(t, 3, countLines(again))
}

func TestLogResumeThroughSessionHonoursUndo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, _ := openLog(t, dir)
	for _, a := range []task.Annotation{
		{ItemID: "1", Action: task.ActionLabel, Label: "same"},
		{ItemID: "2", Action: task.ActionLabel, Label: "same"},
		{ItemID: "2", Action: task.ActionUndo},
	} {
		require.NoError(t, first.Append(a))
	}
	require.NoError(t, first.Close())

	_, prior := openLog(t, dir)
	h := task.Replay(task.SingleChoice{}, prior)

	assert.Equal(t, task.StatusLabeled, h.State("1").Status)
	assert.Equal(t, task.StatusPending, h.State("2").Status)
}

func TestLogWritesAnswerRecordsWithTheirCriterion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l, _ := openLog(t, dir)
	l.now = func() time.Time { return time.Date(2026, 11, 2, 3, 4, 5, 0, time.UTC) }

	require.NoError(t, l.Append(task.Annotation{ItemID: "g-1", Action: task.ActionAnswer, Criterion: "specific", Label: "yes"}))

	got, err := os.ReadFile(logPath(dir))
	require.NoError(t, err)
	want := `{"item":"g-1","annotator":"alice","action":"answer","criterion":"specific","label":"yes","at":"2026-11-02T03:04:05Z"}` + "\n"
	assert.Equal(t, want, string(got))
}

// m1Log is a log as written by the first release, before answer records existed.
const m1Log = `{"item":"1","annotator":"alice","action":"label","label":"same","at":"2026-10-01T00:00:00Z"}
{"item":"2","annotator":"alice","action":"hold","at":"2026-10-01T00:00:01Z"}
{"item":"2","annotator":"alice","action":"note","note":"unclear","at":"2026-10-01T00:00:02Z"}
{"item":"3","annotator":"alice","action":"label","label":"distinct","at":"2026-10-01T00:00:03Z"}
{"item":"3","annotator":"alice","action":"undo","at":"2026-10-01T00:00:04Z"}
`

func TestOpenLogReadsFirstReleaseLogs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath(dir)), 0o750))
	require.NoError(t, os.WriteFile(logPath(dir), []byte(m1Log), 0o600))

	_, prior := openLog(t, dir)

	require.Len(t, prior, 5)
	assert.Equal(t, task.Annotation{
		ItemID: "1", Annotator: "alice", Action: task.ActionLabel, Label: "same",
		At: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}, prior[0])
	h := task.Replay(task.SingleChoice{}, prior)
	assert.Equal(t, task.State{Status: task.StatusLabeled, Label: "same"}, h.State("1"))
	assert.Equal(t, task.State{Status: task.StatusHeld, Note: "unclear"}, h.State("2"))
	assert.Equal(t, task.State{}, h.State("3"))
}

func checklistWorkspace(t *testing.T) task.Workspace {
	t.Helper()
	tk, err := ParseTask([]byte(`name: criteria
fields: [{name: sentence}]
checklist:
  criteria: [{name: specific}, {name: checkable}]
  answers: [{key: y, name: "yes"}, {key: "n", name: "no"}]
  verdict: {rule: all_pass, pass: "yes", fail: "no"}
`))
	require.NoError(t, err)
	ws, err := task.NewWorkspace(tk, []task.Item{
		{ID: "1", Values: map[string]string{"sentence": "a"}},
		{ID: "2", Values: map[string]string{"sentence": "b"}},
	})
	require.NoError(t, err)
	return ws
}

func TestLogResumesChecklistAnswersThroughSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, _ := openLog(t, dir)
	s, err := task.NewSession(checklistWorkspace(t), nil, first)
	require.NoError(t, err)
	require.NoError(t, s.Choose('y'))
	require.NoError(t, s.Choose('n'))
	require.NoError(t, s.Choose('y'))
	require.NoError(t, s.Undo())
	require.NoError(t, first.Close())

	second, prior := openLog(t, dir)
	resumed, err := task.NewSession(checklistWorkspace(t), prior, second)
	require.NoError(t, err)

	assert.Equal(t, "2", resumed.Current().ID, "the undone answer leaves item 2 pending")
	assert.Equal(t, 0, resumed.Focus())
	assert.Equal(t, 1, resumed.Labeled())
	resumed.Prev()
	assert.Equal(t, map[string]string{"specific": "yes", "checkable": "no"}, resumed.CurrentState().Answers)
}

func TestLogRecoversFromTornFinalLine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	full := `{"item":"1","annotator":"alice","action":"hold","at":"2026-11-02T00:00:00Z"}` + "\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath(dir)), 0o750))
	require.NoError(t, os.WriteFile(logPath(dir), []byte(full+`{"item":"2","annot`), 0o600))

	l, prior := openLog(t, dir)
	require.NoError(t, l.Append(task.Annotation{ItemID: "2", Action: task.ActionHold}))

	assert.Len(t, prior, 1)
	data, err := os.ReadFile(logPath(dir))
	require.NoError(t, err)
	assert.Equal(t, 2, countLines(data))
	require.NoError(t, l.Close())
	_, reread := openLog(t, dir)
	assert.Len(t, reread, 2)
}

func TestOpenLogRejectsCorruptCompleteLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"not json", "garbage\n"},
		{"unknown action", `{"item":"1","action":"explode"}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Dir(logPath(dir)), 0o750))
			require.NoError(t, os.WriteFile(logPath(dir), []byte(tt.content), 0o600))

			_, _, err := OpenLog(dir, mustAnnotator(t, "alice"))

			assert.ErrorContains(t, err, "line 1")
		})
	}
}

// halfWriter writes half of the first line it is given, then fails once.
type halfWriter struct {
	logFile
	failed bool
}

func (h *halfWriter) Write(p []byte) (int, error) {
	if h.failed {
		return h.logFile.Write(p)
	}
	h.failed = true
	n, err := h.logFile.Write(p[:len(p)/2])
	if err != nil {
		return n, err
	}
	return n, errors.New("no space left on device")
}

func TestLogAppendRollsBackPartialWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l, _ := openLog(t, dir)
	require.NoError(t, l.Append(task.Annotation{ItemID: "1", Action: task.ActionHold}))
	l.file = &halfWriter{logFile: l.file}

	err := l.Append(task.Annotation{ItemID: "2", Action: task.ActionHold})
	require.ErrorContains(t, err, "no space left")
	require.NoError(t, l.Append(task.Annotation{ItemID: "3", Action: task.ActionHold}))
	require.NoError(t, l.Close())

	_, prior := openLog(t, dir)
	require.Len(t, prior, 2)
	assert.Equal(t, "1", prior[0].ItemID)
	assert.Equal(t, "3", prior[1].ItemID)
}

// stuckTruncate partially writes, then fails both the write and the rollback.
type stuckTruncate struct{ halfWriter }

func (s *stuckTruncate) Truncate(int64) error { return errors.New("input/output error") }

func TestLogStopsAppendingWhenRollbackFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	l, _ := openLog(t, dir)
	l.file = &stuckTruncate{halfWriter{logFile: l.file}}

	require.Error(t, l.Append(task.Annotation{ItemID: "1", Action: task.ActionHold}))
	err := l.Append(task.Annotation{ItemID: "2", Action: task.ActionHold})

	require.ErrorContains(t, err, "unusable")
	data, readErr := os.ReadFile(logPath(dir))
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), `"item":"2"`)
}

func TestOpenLogIsExclusive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first, _ := openLog(t, dir)

	_, _, err := OpenLog(dir, mustAnnotator(t, "alice"))
	require.ErrorIs(t, err, ErrLogInUse)

	require.NoError(t, first.Close())
	again, _, err := OpenLog(dir, mustAnnotator(t, "alice"))
	require.NoError(t, err)
	require.NoError(t, again.Close())
}

func TestOpenLogAllowsDifferentAnnotators(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	alice, _ := openLog(t, dir)

	bob, _, err := OpenLog(dir, mustAnnotator(t, "bob"))

	require.NoError(t, err)
	require.NoError(t, bob.Close())
	require.NoError(t, alice.Close())
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
