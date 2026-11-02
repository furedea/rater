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

const fourItems = `{"id":"1","a":"x","b":"y"}
{"id":"2","a":"x","b":"y"}
{"id":"3","a":"x","b":"y"}
{"id":"4","a":"x","b":"y"}
`

func assignedIDs(t *testing.T, plan task.Assignment, name string) []string {
	t.Helper()
	view, err := plan.Workspace(mustAnnotator(t, name))
	require.NoError(t, err)
	ids := make([]string, 0, len(view.Items))
	for _, it := range view.Items {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestLoadAssignmentWithoutAFileShowsEveryoneEverything(t *testing.T) {
	dir := writeTaskDir(t, fourItems)
	ws, err := store.LoadWorkspace(dir)
	require.NoError(t, err)

	plan, err := store.LoadAssignment(dir, ws)

	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3", "4"}, assignedIDs(t, plan, "alice"))
}

func TestLoadAssignmentReadsSplitAndExplicitLists(t *testing.T) {
	dir := writeTaskDir(t, fourItems)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assignment.yaml"), []byte(`split:
  annotators: [alice, bob]
  copies: 1
items:
  dave: [4, "2"]
`), 0o600))
	ws, err := store.LoadWorkspace(dir)
	require.NoError(t, err)

	plan, err := store.LoadAssignment(dir, ws)

	require.NoError(t, err)
	assert.Equal(t, []string{"1", "3"}, assignedIDs(t, plan, "alice"))
	assert.Equal(t, []string{"2", "4"}, assignedIDs(t, plan, "dave"))
	assert.Equal(t, []string{"1", "2", "3", "4"}, assignedIDs(t, plan, "erin"))
}

func TestLoadAssignmentRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"unknown key", "split: {annotators: [alice], copies: 1}\nteams: {}\n", "teams"},
		{"unknown item", "items: {dave: [9]}\n", "unknown item"},
		{"malformed yaml", "items: [", "decode yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTaskDir(t, fourItems)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "assignment.yaml"), []byte(tt.yaml), 0o600))
			ws, err := store.LoadWorkspace(dir)
			require.NoError(t, err)

			_, err = store.LoadAssignment(dir, ws)

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
