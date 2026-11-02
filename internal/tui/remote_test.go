package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

type discard struct{}

func (discard) Append(task.Annotation) error { return nil }

func newTestSession(t *testing.T) *task.Session {
	t.Helper()
	tk, err := task.NewTask(task.Spec{
		Name: "pairs", Fields: []task.Field{{Name: "a", Title: "a"}},
		Labels: []task.Label{{Key: 's', Name: "same"}},
	})
	require.NoError(t, err)
	ws, err := task.NewWorkspace(tk, []task.Item{{ID: "1", Values: map[string]string{"a": "x"}}})
	require.NoError(t, err)
	s, err := task.NewSession(ws, nil, discard{})
	require.NoError(t, err)
	return s
}

// The note field's ctrl+v reads the clipboard of the machine running the
// program. On a served screen that is the server's clipboard, so it is off;
// the client's own terminal paste still arrives as a paste message.
func TestRemoteScreenNeverReadsTheHostClipboard(t *testing.T) {
	s := newTestSession(t)

	assert.True(t, New(s).input.KeyMap.Paste.Enabled(), "a local screen pastes from its own clipboard")
	assert.False(t, NewRemote(s).input.KeyMap.Paste.Enabled())
}
