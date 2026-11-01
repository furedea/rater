package tui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
)

const finalWait = 3 * time.Second

type harness struct {
	dir string
	tm  *teatest.TestModel
	log *store.Log
}

func start(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	tk, err := task.NewTask(task.Spec{
		Name:   "pairs",
		Header: []string{"repo"},
		Fields: []task.Field{{Name: "a", Title: "Comment A"}},
		Labels: []task.Label{{Key: 's', Name: "same"}, {Key: 'd', Name: "distinct"}},
	})
	require.NoError(t, err)
	items := []task.Item{
		{ID: "1", Values: map[string]string{"a": "first text", "repo": "r1"}},
		{ID: "2", Values: map[string]string{"a": "second text", "repo": "r2"}},
		{ID: "3", Values: map[string]string{"a": "third text", "repo": "r3"}},
	}
	ws, err := task.NewWorkspace(tk, items)
	require.NoError(t, err)
	who, err := task.NewAnnotator("alice")
	require.NoError(t, err)
	log, prior, err := store.OpenLog(dir, who)
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	session, err := task.NewSession(ws, prior, log)
	require.NoError(t, err)
	tm := teatest.NewTestModel(t, tui.New(session), teatest.WithInitialTermSize(100, 40))
	return &harness{dir: dir, tm: tm, log: log}
}

// finish quits and returns the last rendered screen.
func (h *harness) finish(t *testing.T) string {
	t.Helper()
	h.tm.Send(keyEsc)
	final, ok := h.tm.FinalModel(t, teatest.WithFinalTimeout(finalWait)).(tui.Model)
	require.True(t, ok)
	return final.View().Content
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
