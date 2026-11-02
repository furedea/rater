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
	continuation := strings.Split(body.Content(), "\n")[3]
	assert.NotEqual(t, ansi.Strip(continuation), continuation, "a wrapped line keeps its colour on every row")
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
			assert.Equal(t, goKeywordColour, keywordColour(t, body), "package is highlighted as a Go keyword")
		})
	}
}

// goKeywordColour is monokai's KeywordNamespace colour (#f92672), which the Go
// lexer gives "package", as a 24-bit SGR sequence.
const goKeywordColour = "\x1b[38;2;249;38;114m"

// keywordColour returns the SGR sequence that opens the first code line,
// where the sources under test start with the keyword "package".
func keywordColour(t *testing.T, body render.Body) string {
	t.Helper()
	line := strings.Split(body.Content(), "\n")[1]
	end := strings.Index(line, "package")
	require.Positive(t, end, "code line starts with a style sequence")
	return line[:end]
}

func TestBodyCodeLanguagePrecedence(t *testing.T) {
	src := "package main"
	field := task.Field{Name: "c", Title: "Code", Render: task.RenderCode, Language: "go", LanguageField: "lang"}
	tests := []struct {
		name     string
		itemLang string
		isGo     bool
	}{
		{"item language wins over the task's", "python", false},
		{"unknown item language falls back to the task's", "nosuchlang", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, lines := newBody(t, []task.Field{field}, map[string]string{"c": src, "lang": tt.itemLang})

			assert.Equal(t, []string{"Code", src}, lines)
			assert.Equal(t, tt.isGo, strings.HasPrefix(strings.Split(body.Content(), "\n")[1], goKeywordColour))
		})
	}
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
