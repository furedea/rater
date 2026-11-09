package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgreeArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           []string
		wantFormat     reportFormat
		wantAnnotators []string
		wantErr        string
	}{
		{"defaults", []string{"--task", "dir"}, formatText, nil, ""},
		{"json", []string{"--task", "dir", "--format", "json"}, formatJSON, nil, ""},
		{"pair", []string{"--task", "dir", "--annotators", "alice,bob"}, formatText, []string{"alice", "bob"}, ""},
		{"missing task", []string{"--format", "json"}, 0, nil, "--task is required"},
		{"unknown format", []string{"--task", "dir", "--format", "csv"}, 0, nil, "--format"},
		{"one annotator", []string{"--task", "dir", "--annotators", "alice"}, 0, nil, "at least two"},
		{"repeated annotator", []string{"--task", "dir", "--annotators", "alice,alice"}, 0, nil, "twice"},
		{"bad annotator", []string{"--task", "dir", "--annotators", "alice,../x"}, 0, nil, "--annotators"},
		{"unknown flag", []string{"--task", "dir", "--as", "alice"}, 0, nil, "parse flags"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseAgreeArgs(tt.args)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "dir", opts.taskDir)
			assert.Equal(t, tt.wantFormat, opts.format)
			names := make([]string, 0, len(opts.annotators))
			for _, a := range opts.annotators {
				names = append(names, a.String())
			}
			assert.Equal(t, tt.wantAnnotators, nilIfEmpty(names))
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

const pairsTask = `name: pairs
fields: [{name: text}]
labels:
  - {key: s, name: same}
  - {key: d, name: distinct}
`

const criteriaTask = `name: criteria
fields: [{name: text}]
checklist:
  criteria: [{name: concrete}, {name: scoped}]
  answers: [{key: y, name: "yes"}, {key: "n", name: "no"}]
  verdict: {rule: all_pass, pass: "yes", fail: "no"}
`

const fourItems = `{"id":"1","text":"a"}
{"id":"2","text":"b"}
{"id":"3","text":"c"}
{"id":"4","text":"d"}
`

func taskDir(t *testing.T, taskYAML string, logs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task.yaml"), []byte(taskYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "items.jsonl"), []byte(fourItems), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "annotations"), 0o750))
	for name, records := range logs {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "annotations", name+".jsonl"), []byte(records), 0o600))
	}
	return dir
}

// record writes one log line; fields are "action item [label [criterion]]".
func record(spec string) string {
	f := strings.Fields(spec)
	doc := map[string]string{"action": f[0], "item": f[1], "annotator": "x", "at": "2026-01-01T00:00:00Z"}
	if len(f) > 2 {
		doc["label"] = f[2]
	}
	if len(f) > 3 {
		doc["criterion"] = f[3]
	}
	line, _ := json.Marshal(doc) //nolint:errchkjson // a map of strings always encodes
	return string(line) + "\n"
}

func logOf(specs ...string) string {
	var b strings.Builder
	for _, s := range specs {
		b.WriteString(record(s))
	}
	return b.String()
}

func pairsLogs() map[string]string {
	return map[string]string{
		"alice": logOf("label 1 same", "label 2 same", "label 3 distinct", "hold 4"),
		"bob": logOf("label 1 same", "label 2 distinct",
			"label 3 distinct", "label 3 same", "undo 3", "label 4 same"),
		"carol": logOf("label 1 same", "label 2 same", "label 3 same"),
	}
}

func agreeJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, runAgree(append(args, "--format", "json"), &out))
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc), out.String())
	return doc
}

func TestAgreeReportsAPairWithKappaAndConfusion(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())

	doc := agreeJSON(t, "--task", dir, "--annotators", "alice,bob")

	assert.Equal(t, "pairs", doc["task"])
	assert.Equal(t, []any{
		map[string]any{"name": "alice", "labeled": 3.0, "held": 1.0, "pending": 0.0},
		map[string]any{"name": "bob", "labeled": 4.0, "held": 0.0, "pending": 0.0},
	}, doc["annotators"])
	overall := doc["overall"].(map[string]any)
	assert.Equal(t, "label", overall["name"])
	kappa := overall["kappa"].(map[string]any)
	assert.InDelta(t, 0.4, kappa["value"], 1e-9, "items 1-3: bob's undo restores distinct on 3; alice held 4")
	assert.InDelta(t, 2.0/3, kappa["observed"], 1e-9)
	assert.InDelta(t, 4.0/9, kappa["expected"], 1e-9)
	assert.InDelta(t, 3.0, kappa["items"], 0)
	assert.Equal(t, map[string]any{
		"rows": "alice", "columns": "bob",
		"categories": []any{"distinct", "same"},
		"counts":     []any{[]any{1.0, 0.0}, []any{1.0, 1.0}},
	}, overall["confusion"])
	assert.NotContains(t, doc, "criteria")
}

func TestAgreeReadsOnlyTheSelectedLogs(t *testing.T) {
	t.Parallel()

	logs := pairsLogs()
	logs["carol"] = "{corrupt}\n"
	dir := taskDir(t, pairsTask, logs)

	doc := agreeJSON(t, "--task", dir, "--annotators", "alice,bob")

	assert.Len(t, doc["annotators"], 2)
}

func TestAgreeAcrossAllAnnotatorsReportsAlphaOnly(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())

	doc := agreeJSON(t, "--task", dir)

	assert.Len(t, doc["annotators"], 3)
	overall := doc["overall"].(map[string]any)
	assert.NotContains(t, overall, "kappa")
	assert.NotContains(t, overall, "confusion")
	alpha := overall["alpha"].(map[string]any)
	assert.InDelta(t, 3.0, alpha["items"], 0, "item 4 has only bob's label")
	assert.Contains(t, alpha, "value")
}

func TestAgreeReportsEachCriterionAndTheVerdict(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, criteriaTask, map[string]string{
		"alice": logOf("answer 1 yes concrete", "answer 1 yes scoped", "answer 2 yes concrete", "answer 2 yes scoped"),
		"bob": logOf("answer 1 yes concrete", "answer 1 no scoped",
			"answer 2 yes concrete", "answer 2 yes scoped", "answer 2 no scoped", "undo 2"),
	})

	doc := agreeJSON(t, "--task", dir)

	criteria := doc["criteria"].([]any)
	require.Len(t, criteria, 2)
	concrete := criteria[0].(map[string]any)
	assert.Equal(t, "concrete", concrete["name"])
	assert.Equal(t, map[string]any{"value": nil, "undefined": "single category", "observed": 1.0, "expected": 1.0, "items": 2.0}, concrete["kappa"])
	overall := doc["overall"].(map[string]any)
	assert.Equal(t, "verdict", overall["name"])
	assert.Equal(t, []any{"fail", "pass"}, overall["confusion"].(map[string]any)["categories"])
}

func TestAgreeTextReport(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, pairsLogs())
	var out bytes.Buffer

	require.NoError(t, runAgree([]string{"--task", dir, "--annotators", "alice,bob"}, &out))

	text := out.String()
	assert.Contains(t, text, "pairs")
	assert.Regexp(t, `alice\s+3\s+1\s+0`, text)
	assert.Regexp(t, `label\s+3\s+0\.667\s+0\.444\s+0\.400\s+`, text)
	assert.Regexp(t, `distinct\s+1\s+0\n`, text)
}

func TestAgreeTextReportExplainsUndefinedValues(t *testing.T) {
	t.Parallel()

	dir := taskDir(t, pairsTask, map[string]string{
		"alice": logOf("label 1 same"),
		"bob":   logOf("label 2 same"),
	})
	var out bytes.Buffer

	require.NoError(t, runAgree([]string{"--task", dir}, &out))

	assert.Contains(t, out.String(), "no overlapping items")
}

func TestAgreeRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		logs    map[string]string
		args    []string
		wantErr string
	}{
		{"one annotator", map[string]string{"alice": logOf("label 1 same")}, nil, "at least two"},
		{"unknown annotator", pairsLogs(), []string{"--annotators", "alice,dave"}, "dave"},
		{"record unfit for the task", map[string]string{"alice": logOf("label 1 maybe"), "bob": ""}, nil, "alice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := taskDir(t, pairsTask, tt.logs)

			err := runAgree(append([]string{"--task", dir}, tt.args...), &bytes.Buffer{})

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
