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

// writeLog writes content to file, creating its directory, and returns the path.
func writeLog(t *testing.T, file, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o750))
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
	return file
}

func annotationsFile(dir, name string) string { return filepath.Join(dir, "annotations", name) }

const (
	bobLine   = `{"item":"1","annotator":"bob","action":"label","label":"same","at":"2026-01-01T00:00:00Z"}` + "\n"
	aliceLine = `{"item":"1","annotator":"alice","action":"hold","at":"2026-01-01T00:00:00Z"}` + "\n"
)

func TestReadLogsReturnsEveryAnnotatorSortedByName(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, annotationsFile(dir, "bob.jsonl"), bobLine)
	writeLog(t, annotationsFile(dir, "alice.jsonl"), aliceLine)
	writeLog(t, annotationsFile(dir, "notes.txt"), "not a log")

	logs, err := store.ReadLogs(dir)

	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "alice", logs[0].Annotator.String())
	assert.Equal(t, []task.Annotation{{ItemID: "1", Annotator: "alice", Action: task.ActionHold, At: logs[0].Records[0].At}}, logs[0].Records)
	assert.Equal(t, "bob", logs[1].Annotator.String())
	assert.Equal(t, "same", logs[1].Records[0].Label)
}

func TestReadLogReadsOneAnnotator(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, annotationsFile(dir, "bob.jsonl"), bobLine)
	writeLog(t, annotationsFile(dir, "carol.jsonl"), "{corrupt}\n")

	l, err := store.ReadLog(dir, mustAnnotator(t, "bob"))

	require.NoError(t, err)
	assert.Equal(t, "bob", l.Annotator.String())
	assert.Len(t, l.Records, 1)
}

func TestReadLogOfAnAnnotatorWithoutALogFails(t *testing.T) {
	_, err := store.ReadLog(t.TempDir(), mustAnnotator(t, "dave"))

	assert.ErrorIs(t, err, store.ErrNoLog)
}

func mustAnnotator(t *testing.T, name string) task.Annotator {
	t.Helper()
	a, err := task.NewAnnotator(name)
	require.NoError(t, err)
	return a
}

func TestReadLogsWithoutAnnotationsIsEmpty(t *testing.T) {
	logs, err := store.ReadLogs(t.TempDir())

	require.NoError(t, err)
	assert.Empty(t, logs)
}

// An annotator may be writing while agreement is read: a torn final line is
// skipped, and the file is left alone for its writer to recover.
func TestReadLogsSkipsTornTailWithoutTouchingTheFile(t *testing.T) {
	dir := t.TempDir()
	content := bobLine + `{"item":"2","annot`
	path := writeLog(t, annotationsFile(dir, "bob.jsonl"), content)

	logs, err := store.ReadLogs(dir)

	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Len(t, logs[0].Records, 1)
	data, err := os.ReadFile(path) //nolint:gosec // path is under t.TempDir
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestReadLogsRejectsBadLogs(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		wantErr string
	}{
		{"file name is not an annotator", "Bad Name.jsonl", bobLine, "annotator"},
		{"corrupt complete line", "bob.jsonl", "{nope}\n", "bob.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeLog(t, annotationsFile(dir, tt.file), tt.content)

			_, err := store.ReadLogs(dir)

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
