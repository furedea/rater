package store_test

import (
	"os"
	"path/filepath"
	"strings"
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
		{"side-by-side", "name: x\nlayout: side-by-side\nfields: [{name: a}, {name: b, render: diff}]\nlabels: [{key: y, name: yes}]\n", ""},
		{"side-by-side with one field", "name: x\nlayout: side-by-side\nfields: [{name: a}]\nlabels: [{key: y, name: yes}]\n", "exactly two fields"},
		{"unsupported layout", "name: x\nlayout: grid\nfields: [{name: a}]\nlabels: [{key: y, name: yes}]\n", "unsupported layout"},
		{"unsupported render", "name: x\nfields: [{name: a, render: html}]\nlabels: [{key: y, name: yes}]\n", "unsupported render"},
		{"language on text field", "name: x\nfields: [{name: a, language: go}]\nlabels: [{key: y, name: yes}]\n", "language"},
		{"unknown code language", "name: x\nfields: [{name: a, render: code, language: nosuchlang}]\nlabels: [{key: y, name: yes}]\n", "unknown language"},
		{"unknown key binding", "name: x\nfields: [{name: a}]\nlabels: [{key: y, name: yes}]\nkeys: {jump: j}\n", "jump"},
		{"binding collides with label", "name: x\nfields: [{name: a}]\nlabels: [{key: j, name: yes}]\nkeys: {scroll_down: j}\n", "collides"},
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

func TestParseTaskRenders(t *testing.T) {
	const doc = `name: x
fields:
  - {name: d, render: diff}
  - {name: c, render: code, language: go, language_field: lang}
  - {name: m, render: markdown}
labels: [{key: y, name: yes}]
keys: {next_hunk: n, prev_hunk: p}
`
	tk, err := store.ParseTask([]byte(doc))
	require.NoError(t, err)

	assert.Equal(t, []task.Field{
		{Name: "d", Title: "d", Render: task.RenderDiff},
		{Name: "c", Title: "c", Render: task.RenderCode, Language: "go", LanguageField: "lang"},
		{Name: "m", Title: "m", Render: task.RenderMarkdown},
	}, tk.Fields())
	assert.Equal(t, "n", tk.KeyFor(task.BindingNextHunk))
	assert.Equal(t, "p", tk.KeyFor(task.BindingPrevHunk))
	assert.Equal(t, "down", tk.KeyFor(task.BindingScrollDown))
}

const checklistTaskYAML = `name: criteria
fields: [{name: sentence}]
checklist:
  criteria:
    - {name: specific, title: Names a concrete construct}
    - {name: checkable}
  answers:
    - {key: y, name: "yes"}
    - {key: "n", name: "no"}
    - {key: "?", name: unsure}
  verdict: {rule: all_pass, pass: "yes", fail: "no"}
`

func TestParseTaskChecklist(t *testing.T) {
	tk, err := store.ParseTask([]byte(checklistTaskYAML))
	require.NoError(t, err)

	cl, ok := tk.Scheme().(task.Checklist)
	require.True(t, ok)
	assert.Equal(t, []task.Criterion{
		{Name: "specific", Title: "Names a concrete construct"},
		{Name: "checkable", Title: "checkable"},
	}, cl.Criteria())
	assert.Equal(t, []task.Label{{Key: 'y', Name: "yes"}, {Key: 'n', Name: "no"}, {Key: '?', Name: "unsure"}}, cl.Answers())
	assert.Equal(t, task.VerdictPass, cl.Verdict(map[string]string{"specific": "yes", "checkable": "yes"}))
	assert.Equal(t, task.VerdictFail, cl.Verdict(map[string]string{"checkable": "no"}))
}

func TestParseTaskChecklistErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"unknown rule", strings.Replace(checklistTaskYAML, "all_pass", "majority", 1), "majority"},
		{"missing rule", strings.Replace(checklistTaskYAML, "rule: all_pass, ", "", 1), "verdict rule"},
		{"labels and checklist", checklistTaskYAML + "labels: [{key: s, name: same}]\n", "not both"},
		{"multi-char answer key", strings.Replace(checklistTaskYAML, "{key: y,", "{key: yy,", 1), "one character"},
		{"unknown checklist key", strings.Replace(checklistTaskYAML, "  answers:", "  colour: red\n  answers:", 1), "colour"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.ParseTask([]byte(tt.yaml))

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestParseTaskNormalizesLabelKeys(t *testing.T) {
	tk, err := store.ParseTask([]byte("name: x\nfields: [{name: a}]\nlabels: [{key: shift+s, name: same}]\n"))
	require.NoError(t, err)

	sc, ok := tk.Scheme().(task.SingleChoice)
	require.True(t, ok)
	l, ok := sc.LabelByKey('S')

	assert.True(t, ok)
	assert.Equal(t, "same", l.Name)
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

func TestShippedExamplesLoad(t *testing.T) {
	for _, name := range []string{"review-comment-pairs", "guideline-criteria", "diff-relevance"} {
		t.Run(name, func(t *testing.T) {
			ws, err := store.LoadWorkspace(filepath.Join("..", "..", "examples", name))

			require.NoError(t, err)
			assert.Equal(t, name, ws.Task.Name())
			assert.NotEmpty(t, ws.Items)
		})
	}
}

func TestShippedExamplesUseTheirLayoutAndScheme(t *testing.T) {
	pairs, err := store.LoadTask(filepath.Join("..", "..", "examples", "review-comment-pairs"))
	require.NoError(t, err)
	criteria, err := store.LoadTask(filepath.Join("..", "..", "examples", "guideline-criteria"))
	require.NoError(t, err)

	assert.Equal(t, task.LayoutSideBySide, pairs.Layout())
	cl, ok := criteria.Scheme().(task.Checklist)
	require.True(t, ok)
	assert.Len(t, cl.Criteria(), 5)
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
