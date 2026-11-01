// Package tui renders an annotation session with Bubble Tea. It only maps
// keys to session calls and session state to text; the rules live in task.
package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/furedea/rater/internal/task"
)

type mode int

const (
	modeBrowse mode = iota
	modeNote
)

// Model is the Bubble Tea model for one annotation session.
type Model struct {
	session *task.Session
	mode    mode
	input   textinput.Model
	width   int
	status  string
}

// New returns a model that drives session.
func New(session *task.Session) Model {
	in := textinput.New()
	in.Prompt = "note> "
	return Model{session: session, input: in, width: defaultWidth}
}

// Run shows the session until the annotator quits. Every action is already
// appended to the log when it happens, so quitting needs no extra save.
func Run(session *task.Session) error {
	if _, err := tea.NewProgram(New(session)).Run(); err != nil {
		return fmt.Errorf("run annotate screen: %w", err)
	}
	return nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyPressMsg:
		if m.mode == modeNote {
			return m.updateNote(msg)
		}
		return m.updateBrowse(msg)
	}
	return m.forwardToNote(msg)
}

func (m Model) updateBrowse(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	switch key := msg.String(); key {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "left":
		m.session.Prev()
	case "right":
		m.session.Next()
	case "tab":
		m.status = errText(m.session.Hold())
	case "ctrl+u":
		m.status = errText(m.session.Undo())
	case "ctrl+n":
		return m.startNote()
	default:
		m.status = m.choose(msg.Text)
	}
	return m, nil
}

func (m Model) choose(text string) string {
	runes := []rune(text)
	if len(runes) != 1 {
		return ""
	}
	return errText(m.session.Choose(runes[0]))
}

func (m Model) startNote() (tea.Model, tea.Cmd) {
	m.mode = modeNote
	m.input.SetValue(m.session.CurrentState().Note)
	m.input.CursorEnd()
	return m, m.input.Focus()
}

func (m Model) updateNote(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.status = errText(m.session.SetNote(m.input.Value()))
		return m.endNote(), nil
	case "esc":
		return m.endNote(), nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// forwardToNote hands non-key messages (terminal paste, the input's own clipboard
// and blink results) to the note field while it is being edited.
func (m Model) forwardToNote(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode != modeNote {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) endNote() Model {
	m.mode = modeBrowse
	m.input.Blur()
	return m
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
