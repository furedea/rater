package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/render"
)

func TestParseDiff(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []render.LineKind
	}{
		{
			"file headers, hunk, and body",
			"diff --git a/x.go b/x.go\nindex 1..2 100644\n--- a/x.go\n+++ b/x.go\n@@ -1,3 +1,3 @@\n ctx\n-old\n+new\n ctx",
			[]render.LineKind{
				render.LineFileHeader, render.LineFileHeader, render.LineFileHeader, render.LineFileHeader,
				render.LineHunkHeader, render.LineContext, render.LineRemoved, render.LineAdded, render.LineContext,
			},
		},
		{
			"removed line that looks like a file header stays in the hunk",
			"@@ -1,2 +1,1 @@\n--- comment\n-x\n+y",
			[]render.LineKind{render.LineHunkHeader, render.LineRemoved, render.LineRemoved, render.LineAdded},
		},
		{
			"header after a completed hunk starts the next file",
			"@@ -1 +1 @@\n-a\n+b\n--- a/y.go\n+++ b/y.go\n@@ -5,0 +5,1 @@\n+c",
			[]render.LineKind{
				render.LineHunkHeader, render.LineRemoved, render.LineAdded,
				render.LineFileHeader, render.LineFileHeader, render.LineHunkHeader, render.LineAdded,
			},
		},
		{
			"truncated hunk ends at the next non-body line",
			"@@ -10,3 +10,3 @@\n-a\n+b\ndiff --git a/z b/z\n@@ -1 +1 @@\n x",
			[]render.LineKind{
				render.LineHunkHeader, render.LineRemoved, render.LineAdded,
				render.LineFileHeader, render.LineHunkHeader, render.LineContext,
			},
		},
		{
			"no-newline marker and blank context line",
			"@@ -1,2 +1,2 @@\n\n-a\n\\ No newline at end of file\n+b",
			[]render.LineKind{render.LineHunkHeader, render.LineContext, render.LineRemoved, render.LineContext, render.LineAdded},
		},
		{"plain text outside any hunk", "just text", []render.LineKind{render.LineFileHeader}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := render.ParseDiff(tt.text)
			kinds := make([]render.LineKind, 0, len(lines))
			for _, l := range lines {
				kinds = append(kinds, l.Kind)
			}
			assert.Equal(t, tt.want, kinds)
		})
	}
}

func TestHunksNavigation(t *testing.T) {
	hunks := render.NewHunks([]int{3, 10, 25})
	tests := []struct {
		name     string
		offset   int
		wantNext int
		nextOK   bool
		wantPrev int
		prevOK   bool
	}{
		{"before the first hunk", 0, 3, true, 0, false},
		{"on a hunk", 10, 25, true, 3, true},
		{"between hunks", 12, 25, true, 10, true},
		{"on the last hunk", 25, 0, false, 10, true},
		{"after the last hunk", 30, 0, false, 25, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, ok := hunks.Next(tt.offset)
			assert.Equal(t, tt.nextOK, ok)
			assert.Equal(t, tt.wantNext, next)
			prev, ok := hunks.Prev(tt.offset)
			assert.Equal(t, tt.prevOK, ok)
			assert.Equal(t, tt.wantPrev, prev)
		})
	}
}

func TestHunksEmpty(t *testing.T) {
	var hunks render.Hunks

	_, ok := hunks.Next(0)
	assert.False(t, ok)
	_, ok = hunks.Prev(5)
	assert.False(t, ok)
}
