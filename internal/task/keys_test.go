package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/task"
)

func TestParseKeyNormalizesSynonyms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain letter", "s", "s"},
		{"upper letter stays as typed", "S", "S"},
		{"shift letter is the upper letter", "shift+s", "S"},
		{"shift upper letter is the upper letter", "shift+S", "S"},
		{"named key", "down", "down"},
		{"named key in upper case", "Down", "down"},
		{"escape alias", "escape", "esc"},
		{"return alias", "return", "enter"},
		{"page down alias", "pagedown", "pgdown"},
		{"page up alias", "pageup", "pgup"},
		{"space character", " ", "space"},
		{"modifier order", "shift+ctrl+up", "ctrl+shift+up"},
		{"modifier case", "CTRL+j", "ctrl+j"},
		{"control alias", "control+j", "ctrl+j"},
		{"ctrl with upper letter adds shift", "ctrl+J", "ctrl+shift+j"},
		{"shift with other modifier keeps lower letter", "ctrl+shift+j", "ctrl+shift+j"},
		{"plus key", "+", "+"},
		{"modified plus key", "ctrl++", "ctrl++"},
		{"shift digit is left as written", "shift+1", "shift+1"},
		{"bracket", "]", "]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := task.ParseKey(tt.in)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestParseKeyRejectsUnusableNotation(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "  ", " down", "ctrl+", "hyperdrive+a", "ctrl+ctrl+a"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			_, err := task.ParseKey(in)

			assert.Error(t, err)
		})
	}
}

func TestTaskMatchesKeySynonyms(t *testing.T) {
	t.Parallel()

	spec := validSpec()
	spec.Keys = map[task.Binding]string{task.BindingScrollDown: "shift+j"}
	tk, err := task.NewTask(spec)
	require.NoError(t, err)

	b, ok := tk.BindingFor("J")

	assert.True(t, ok)
	assert.Equal(t, task.BindingScrollDown, b)
	assert.Equal(t, "J", tk.KeyFor(task.BindingScrollDown))
}

func TestNewTaskDetectsCollisionsBetweenSynonyms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		keys    map[task.Binding]string
		labels  []task.Label
		wantErr string
	}{
		{"shift binding against upper label", map[task.Binding]string{task.BindingScrollDown: "shift+s"}, []task.Label{{Key: 'S', Name: "same"}}, `collides with label "same"`},
		{"alias against control key", map[task.Binding]string{task.BindingScrollDown: "escape"}, nil, "reserved"},
		{"two spellings of one key", map[task.Binding]string{task.BindingScrollDown: "ctrl+shift+j", task.BindingPageDown: "shift+ctrl+j"}, nil, "bound twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := validSpec()
			spec.Keys = tt.keys
			if tt.labels != nil {
				spec.Labels = tt.labels
			}

			_, err := task.NewTask(spec)

			require.ErrorIs(t, err, task.ErrInvalidTask)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
