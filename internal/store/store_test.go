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

const validTaskYAML = `name: pairs
header: [repo]
fields:
  - name: a
    title: Comment A
  - name: b
labels:
  - {key: s, name: same}
  - {key: d, name: distinct}
`

func TestParseTask(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"valid", validTaskYAML, ""},
		{"explicit stacked text", "name: x\nlayout: stacked\nfields: [{name: a, render: text}]\nlabels: [{key: y, name: yes}]\n", ""},
		{"unknown top-level key", validTaskYAML + "colour: red\n", "colour"},
		{"unsupported layout", "name: x\nlayout: side-by-side\nfields: [{name: a}]\nlabels: [{key: y, name: yes}]\n", "unsupported layout"},
		{"unsupported render", "name: x\nfields: [{name: a, render: diff}]\nlabels: [{key: y, name: yes}]\n", "unsupported render"},
		{"multi-char key", "name: x\nfields: [{name: a}]\nlabels: [{key: yy, name: yes}]\n", "one character"},
		{"empty key", "name: x\nfields: [{name: a}]\nlabels: [{name: yes}]\n", "one character"},
		{"duplicate keys", "name: x\nfields: [{name: a}]\nlabels: [{key: y, name: yes}, {key: y, name: no}]\n", "duplicate label key"},
		{"missing fields", "name: x\nlabels: [{key: y, name: yes}]\n", "no fields"},
		{"malformed yaml", "name: [", "decode yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.ParseTask([]byte(tt.yaml))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestParseTaskDefaults(t *testing.T) {
	tk, err := store.ParseTask([]byte(validTaskYAML))
	require.NoError(t, err)

	assert.Equal(t, task.LayoutStacked, tk.Layout())
	assert.Equal(t, []task.Field{{Name: "a", Title: "Comment A"}, {Name: "b", Title: "b"}}, tk.Fields())
	assert.Equal(t, []string{"repo"}, tk.Header())
}

func TestParseItems(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantIDs []string
		wantErr string
	}{
		{"two items", "{\"id\":\"1\",\"a\":\"x\"}\n{\"id\":\"2\",\"a\":\"y\"}\n", []string{"1", "2"}, ""},
		{"blank lines skipped", "\n{\"id\":\"1\"}\n\n", []string{"1"}, ""},
		{"missing newline at end", "{\"id\":\"1\"}", []string{"1"}, ""},
		{"bad json", "{\"id\":\"1\"}\nnope\n", nil, "line 2"},
		{"numeric id", "{\"id\":1}\n", nil, "id must be a string"},
		{"missing id", "{\"a\":\"x\"}\n", nil, "id must be a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := store.ParseItems([]byte(tt.input))
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			ids := make([]string, 0, len(items))
			for _, it := range items {
				ids = append(ids, it.ID)
			}
			assert.Equal(t, tt.wantIDs, ids)
		})
	}
}

func TestParseItemsDisplaysNonStringValues(t *testing.T) {
	items, err := store.ParseItems([]byte(`{"id":"1","pr":42,"tags":["a","b"],"text":"hi"}` + "\n"))
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"id": "1", "pr": "42", "tags": `["a","b"]`, "text": "hi"}, items[0].Values)
}

func writeTaskDir(t *testing.T, items string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task.yaml"), []byte(validTaskYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "items.jsonl"), []byte(items), 0o600))
	return dir
}

func TestLoadWorkspace(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := writeTaskDir(t, `{"id":"1","a":"x","b":"y"}`+"\n")

		ws, err := store.LoadWorkspace(dir)

		require.NoError(t, err)
		assert.Len(t, ws.Items, 1)
	})
	t.Run("item lacks configured field", func(t *testing.T) {
		dir := writeTaskDir(t, `{"id":"1","a":"x"}`+"\n")

		_, err := store.LoadWorkspace(dir)

		assert.ErrorIs(t, err, task.ErrInvalidItems)
	})
	t.Run("missing task file", func(t *testing.T) {
		_, err := store.LoadWorkspace(t.TempDir())

		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
