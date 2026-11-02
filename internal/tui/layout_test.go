package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

var keySwitchPane = tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}

func startSideBySide(t *testing.T, a, b string) *harness {
	t.Helper()
	spec := task.Spec{
		Name:   "pairs",
		Layout: task.LayoutSideBySide,
		Fields: []task.Field{{Name: "a", Title: "Comment A"}, {Name: "b", Title: "Comment B"}},
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	}
	items := []task.Item{{ID: "1", Values: map[string]string{"a": a, "b": b}}}
	return startSpec(t, spec, items)
}

// lineWith returns the first screen line containing every part, or "".
func lineWith(screen string, parts ...string) string {
	for line := range strings.SplitSeq(screen, "\n") {
		found := true
		for _, p := range parts {
			found = found && strings.Contains(line, p)
		}
		if found {
			return line
		}
	}
	return ""
}

func TestSideBySideShowsFieldsInColumns(t *testing.T) {
	h := startSideBySide(t, "null check missing", "guard user before use")

	screen := h.finish(t)

	assert.NotEmpty(t, lineWith(screen, "Comment A", "Comment B"), "titles share a row:\n%s", screen)
	row := lineWith(screen, "null check missing", "guard user before use")
	assert.NotEmpty(t, row, "texts share a row:\n%s", screen)
	cols, ok := render.SplitColumns(termWidth)
	require.True(t, ok)
	column := ansi.StringWidth(row[:strings.Index(row, "guard")])
	assert.Equal(t, cols.Left+render.ColumnGap, column, "the right column starts after the left column and the gap")
}

func TestSideBySideFallsBackToStackedWhenNarrow(t *testing.T) {
	h := startSideBySide(t, "null check missing", "guard user before use")

	h.tm.Send(tea.WindowSizeMsg{Width: 50, Height: 40})
	screen := h.finish(t)

	assert.Contains(t, screen, "too narrow for side-by-side")
	assert.Contains(t, screen, "Comment A")
	assert.Contains(t, screen, "Comment B")
	assert.Empty(t, lineWith(screen, "Comment A", "Comment B"), "titles are stacked")
}

func TestSideBySideScrollsTheFocusedPane(t *testing.T) {
	t.Run("the left pane scrolls first", func(t *testing.T) {
		h := startSideBySide(t, numberedLines(60), "short")

		h.tm.Send(keyPgDn)
		screen := h.finish(t)

		assert.NotContains(t, screen, "line 01")
		assert.Contains(t, screen, "short", "the right pane stays put")
	})
	t.Run("switching panes moves scrolling to the right", func(t *testing.T) {
		h := startSideBySide(t, numberedLines(60), numberedLines(60))

		h.tm.Send(keySwitchPane)
		h.tm.Send(keyPgDn)
		screen := h.finish(t)

		row := lineWith(screen, "line 01")
		assert.NotEmpty(t, row, "the left pane is still at its top")
		assert.Equal(t, 1, strings.Count(row, "line 01"), "only the left pane shows line 01")
	})
}

// tailHunksDiff puts two short hunks after two long ones, so the last two
// hunk headers lie beyond the viewport's largest offset.
func tailHunksDiff() string {
	var b strings.Builder
	b.WriteString("--- a/x.go\n+++ b/x.go")
	for _, start := range []int{1, 20} {
		fmt.Fprintf(&b, "\n@@ -%d,10 +%d,10 @@", start, start)
		for i := range 10 {
			fmt.Fprintf(&b, "\n ctx %d-%d", start, i)
		}
	}
	for _, start := range []int{40, 50} {
		fmt.Fprintf(&b, "\n@@ -%d,1 +%d,1 @@\n ctx %d", start, start, start)
	}
	return b.String()
}

func TestHunkKeysReachHunksNearTheEnd(t *testing.T) {
	fields := []task.Field{{Name: "d", Title: "Diff", Render: task.RenderDiff}}
	items := []task.Item{{ID: "1", Values: map[string]string{"d": tailHunksDiff(), "repo": "r1"}}}
	t.Run("forward reaches the last hunk", func(t *testing.T) {
		h := startWith(t, fields, items)

		h.tm.Type("]]]]")
		screen := h.finish(t)

		assert.Contains(t, screen, "hunk 4 of 4")
		assert.Contains(t, screen, "@@ -50,1 +50,1 @@")
	})
	t.Run("back from the last hunk returns to the one before", func(t *testing.T) {
		h := startWith(t, fields, items)

		h.tm.Type("]]]][")
		screen := h.finish(t)

		assert.Contains(t, screen, "hunk 3 of 4")
	})
}
