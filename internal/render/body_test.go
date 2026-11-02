package render_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

const width = 60

func newBody(t *testing.T, fields []task.Field, values map[string]string) (render.Body, []string) {
	t.Helper()
	body, err := render.NewBody(fields, task.Item{ID: "1", Values: values}, width)
	require.NoError(t, err)
	return body, strings.Split(ansi.Strip(body.Content()), "\n")
}

func TestBodyHunkOffsetsPointAtHunkHeaders(t *testing.T) {
	fields := []task.Field{
		{Name: "g", Title: "Guideline"},
		{Name: "d", Title: "Diff", Render: task.RenderDiff},
	}
	diff := "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n@@ -9 +9 @@\n-c\n+d"

	body, lines := newBody(t, fields, map[string]string{"g": "line one\nline two", "d": diff})

	first, ok := body.Hunks().Next(0)
	require.True(t, ok)
	assert.Equal(t, "@@ -1 +1 @@", lines[first])
	second, ok := body.Hunks().Next(first)
	require.True(t, ok)
	assert.Equal(t, "@@ -9 +9 @@", lines[second])
	_, ok = body.Hunks().Next(second)
	assert.False(t, ok)
}

func TestBodyColoursDiffLinesByKind(t *testing.T) {
	fields := []task.Field{{Name: "d", Title: "Diff", Render: task.RenderDiff}}

	body, _ := newBody(t, fields, map[string]string{"d": "@@ -1,2 +1,2 @@\n-old\n+new\n same"})

	styled := strings.Split(body.Content(), "\n")
	added, removed, context := styled[3], styled[2], styled[4]
	assert.NotEqual(t, ansi.Strip(added), added, "added line is styled")
	assert.NotEqual(t, ansi.Strip(removed), removed, "removed line is styled")
	assert.NotEqual(t, added[:strings.Index(added, "+")], removed[:strings.Index(removed, "-")], "added and removed differ in style")
	assert.Equal(t, " same", context)
}

func TestBodyWrapsLongDiffLinesAndKeepsOffsets(t *testing.T) {
	fields := []task.Field{{Name: "d", Title: "Diff", Render: task.RenderDiff}}
	long := "+" + strings.Repeat("x", width*2)

	body, lines := newBody(t, fields, map[string]string{"d": "@@ -1 +1,1 @@\n" + long + "\n@@ -5 +5 @@\n-y"})

	for _, l := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(l), width)
	}
	first, _ := body.Hunks().Next(0)
	second, ok := body.Hunks().Next(first)
	require.True(t, ok)
	assert.Equal(t, "@@ -5 +5 @@", lines[second])
}

func TestBodyHighlightsCode(t *testing.T) {
	src := "package main\n\nfunc main() {}"
	tests := []struct {
		name   string
		field  task.Field
		values map[string]string
	}{
		{"language from task", task.Field{Name: "c", Title: "Code", Render: task.RenderCode, Language: "go"}, map[string]string{"c": src}},
		{"language from item", task.Field{Name: "c", Title: "Code", Render: task.RenderCode, LanguageField: "lang"}, map[string]string{"c": src, "lang": "go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, lines := newBody(t, []task.Field{tt.field}, tt.values)

			assert.Equal(t, append([]string{"Code"}, strings.Split(src, "\n")...), lines)
			assert.Contains(t, body.Content(), "\x1b[", "code carries colour")
		})
	}
}

func TestBodyCodeWithUnknownItemLanguageFallsBack(t *testing.T) {
	field := task.Field{Name: "c", Title: "Code", Render: task.RenderCode, Language: "go", LanguageField: "lang"}

	_, lines := newBody(t, []task.Field{field}, map[string]string{"c": "x := 1", "lang": "nosuchlang"})

	assert.Equal(t, []string{"Code", "x := 1"}, lines)
}

func TestBodyRendersMarkdown(t *testing.T) {
	field := task.Field{Name: "m", Title: "Notes", Render: task.RenderMarkdown}

	_, lines := newBody(t, []task.Field{field}, map[string]string{"m": "# Heading\n\nSome **bold** text."})

	text := strings.Join(lines, "\n")
	assert.Contains(t, text, "Heading")
	assert.Contains(t, text, "Some bold text.")
	assert.NotContains(t, text, "**")
}

func TestBodyWrapsPlainText(t *testing.T) {
	field := task.Field{Name: "a", Title: "A"}
	words := strings.Repeat("word ", 30)

	_, lines := newBody(t, []task.Field{field}, map[string]string{"a": words})

	assert.Greater(t, len(lines), 2)
	for _, l := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(l), width)
	}
}
