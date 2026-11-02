package tui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
	"github.com/furedea/rater/internal/tui"
)

var (
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyTab   = tea.KeyPressMsg{Code: tea.KeyTab}
	keyLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyUndo  = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	keyNote  = tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyPgDn  = tea.KeyPressMsg{Code: tea.KeyPgDown}
)

const finalWait = 3 * time.Second

type harness struct {
	dir string
	tm  *teatest.TestModel
	log *store.Log
}

func start(t *testing.T) *harness {
	t.Helper()
	fields := []task.Field{{Name: "a", Title: "Comment A"}}
	items := []task.Item{
		{ID: "1", Values: map[string]string{"a": "first text", "repo": "r1"}},
		{ID: "2", Values: map[string]string{"a": "second text", "repo": "r2"}},
		{ID: "3", Values: map[string]string{"a": "third text", "repo": "r3"}},
	}
	return startWith(t, fields, items)
}

// termHeight is small enough that a long body or a multi-hunk diff overflows it.
const termHeight = 20

// startWith runs a session over items on a 100×termHeight terminal.
// The colour profile is pinned so the program's output does not depend on the
// terminal running the tests.
func startWith(t *testing.T, fields []task.Field, items []task.Item) *harness {
	t.Helper()
	dir := t.TempDir()
	tk, err := task.NewTask(task.Spec{
		Name:   "pairs",
		Header: []string{"repo"},
		Fields: fields,
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	})
	require.NoError(t, err)
	ws, err := task.NewWorkspace(tk, items)
	require.NoError(t, err)
	who, err := task.NewAnnotator("alice")
	require.NoError(t, err)
	log, prior, err := store.OpenLog(dir, who)
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	session, err := task.NewSession(ws, prior, log)
	require.NoError(t, err)
	tm := teatest.NewTestModel(t, tui.New(session),
		teatest.WithInitialTermSize(100, termHeight),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	)
	return &harness{dir: dir, tm: tm, log: log}
}

// finish quits and returns the last rendered screen.
func (h *harness) finish(t *testing.T) string {
	t.Helper()
	h.tm.Send(keyEsc)
	final, ok := h.tm.FinalModel(t, teatest.WithFinalTimeout(finalWait)).(tui.Model)
	require.True(t, ok)
	require.NoError(t, h.log.Close())
	return ansi.Strip(final.View().Content)
}

func (h *harness) replay(t *testing.T) *task.History {
	t.Helper()
	who, err := task.NewAnnotator("alice")
	require.NoError(t, err)
	log, prior, err := store.OpenLog(h.dir, who)
	require.NoError(t, err)
	require.NoError(t, log.Close())
	return task.Replay(prior)
}

func TestLabelAdvancesAndPersists(t *testing.T) {
	h := start(t)

	h.tm.Type("s")
	h.tm.Type("d")
	screen := h.finish(t)

	assert.Contains(t, screen, "3 / 3")
	assert.Contains(t, screen, "labeled 2")
	history := h.replay(t)
	assert.Equal(t, "same", history.State("1").Label)
	assert.Equal(t, "distinct", history.State("2").Label)
}

func TestUndoRevertsLastLabel(t *testing.T) {
	h := start(t)

	h.tm.Type("s")
	h.tm.Send(keyUndo)
	screen := h.finish(t)

	assert.Contains(t, screen, "1 / 3")
	assert.Contains(t, screen, "state: pending")
	assert.Equal(t, task.StatusPending, h.replay(t).State("1").Status)
}

func TestHoldAndNavigation(t *testing.T) {
	h := start(t)

	h.tm.Send(keyTab)
	h.tm.Send(keyLeft)
	screen := h.finish(t)

	assert.Contains(t, screen, "1 / 3")
	assert.Contains(t, screen, "state: held")
	assert.Equal(t, task.StatusHeld, h.replay(t).State("1").Status)
}

func TestNoteEntry(t *testing.T) {
	h := start(t)

	h.tm.Send(keyNote)
	h.tm.Type("odd pair")
	h.tm.Send(keyEnter)
	screen := h.finish(t)

	assert.Contains(t, screen, "note: odd pair")
	assert.Equal(t, "odd pair", h.replay(t).State("1").Note)
}

func TestNoteAcceptsPaste(t *testing.T) {
	h := start(t)

	h.tm.Send(keyNote)
	h.tm.Send(tea.PasteMsg{Content: "pasted text"})
	h.tm.Send(keyEnter)
	h.finish(t)

	assert.Equal(t, "pasted text", h.replay(t).State("1").Note)
}

func TestNoteEscapeCancels(t *testing.T) {
	h := start(t)

	h.tm.Send(keyNote)
	h.tm.Type("draft")
	h.tm.Send(keyEsc)
	screen := h.finish(t)

	assert.NotContains(t, screen, "draft")
	assert.Empty(t, h.replay(t).State("1").Note)
}

func TestScreenShowsFieldsMetadataAndLabels(t *testing.T) {
	h := start(t)

	screen := h.finish(t)

	for _, want := range []string{"Comment A", "first text", "repo: r1", "[s] same", "[d] distinct", "1 / 3"} {
		assert.Contains(t, screen, want)
	}
}

func numberedLines(n int) string {
	lines := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		lines = append(lines, fmt.Sprintf("line %02d", i))
	}
	return strings.Join(lines, "\n")
}

func startLongBody(t *testing.T) *harness {
	t.Helper()
	items := []task.Item{
		{ID: "1", Values: map[string]string{"a": numberedLines(60), "repo": "r1"}},
		{ID: "2", Values: map[string]string{"a": "short", "repo": "r2"}},
	}
	return startWith(t, []task.Field{{Name: "a", Title: "Comment A"}}, items)
}

func TestLongBodyStartsAtTop(t *testing.T) {
	h := startLongBody(t)

	screen := h.finish(t)

	assert.Contains(t, screen, "line 01")
	assert.NotContains(t, screen, "line 60")
	assert.Contains(t, screen, "[s] same", "labels stay visible below a long body")
}

func TestScrollKeysMoveTheBody(t *testing.T) {
	t.Run("line down", func(t *testing.T) {
		h := startLongBody(t)

		h.tm.Send(keyDown)
		h.tm.Send(keyDown)
		screen := h.finish(t)

		assert.NotContains(t, screen, "line 01")
		assert.Contains(t, screen, "line 02")
	})
	t.Run("page down to the end", func(t *testing.T) {
		h := startLongBody(t)

		for range 15 {
			h.tm.Send(keyPgDn)
		}
		screen := h.finish(t)

		assert.Contains(t, screen, "line 60")
		assert.NotContains(t, screen, "line 30")
	})
}

func TestGrowingTheTerminalKeepsTheBodyFilled(t *testing.T) {
	h := startLongBody(t)

	for range 15 {
		h.tm.Send(keyPgDn)
	}
	h.tm.Send(tea.WindowSizeMsg{Width: 100, Height: 60})
	screen := h.finish(t)

	assert.Contains(t, screen, "line 60")
	assert.Contains(t, screen, "line 30", "the taller body shows earlier lines instead of blank rows")
}

func TestScrollingDoesNotLabel(t *testing.T) {
	h := startLongBody(t)

	h.tm.Send(keyPgDn)
	h.tm.Type("s")
	screen := h.finish(t)

	assert.Contains(t, screen, "2 / 2")
	assert.Equal(t, "same", h.replay(t).State("1").Label)
}

func TestNextItemShowsItsBodyFromTheTop(t *testing.T) {
	h := startLongBody(t)

	h.tm.Send(keyPgDn)
	h.tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	h.tm.Send(keyLeft)
	screen := h.finish(t)

	assert.Contains(t, screen, "line 01")
}

// threeHunkDiff has hunks far enough apart that only one header fits on screen.
func threeHunkDiff() string {
	var b strings.Builder
	b.WriteString("--- a/x.go\n+++ b/x.go")
	for _, start := range []int{1, 20, 40} {
		fmt.Fprintf(&b, "\n@@ -%d,10 +%d,10 @@", start, start)
		for i := range 10 {
			fmt.Fprintf(&b, "\n ctx %d-%d", start, i)
		}
	}
	return b.String()
}

func startDiff(t *testing.T) *harness {
	t.Helper()
	fields := []task.Field{{Name: "d", Title: "Diff", Render: task.RenderDiff}}
	items := []task.Item{{ID: "1", Values: map[string]string{"d": threeHunkDiff(), "repo": "r1"}}}
	return startWith(t, fields, items)
}

func TestHunkKeysJumpBetweenHunks(t *testing.T) {
	t.Run("forward twice reaches the second hunk", func(t *testing.T) {
		h := startDiff(t)

		h.tm.Type("]]")
		screen := h.finish(t)

		assert.Contains(t, screen, "@@ -20,10 +20,10 @@")
		assert.NotContains(t, screen, "@@ -1,10")
		assert.NotContains(t, screen, "@@ -40,10")
	})
	t.Run("back returns to the first hunk", func(t *testing.T) {
		h := startDiff(t)

		h.tm.Type("]]]")
		h.tm.Type("[[")
		screen := h.finish(t)

		assert.Contains(t, screen, "@@ -1,10 +1,10 @@")
		assert.NotContains(t, screen, "@@ -20,10")
	})
}
