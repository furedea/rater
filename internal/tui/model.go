// Package tui renders an annotation session with Bubble Tea. It only maps
// keys to session calls and session state to text; the rules live in task.
package tui

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/furedea/rater/internal/render"
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
	height  int
	status  string
	body    viewport.Model
	hunks   render.Hunks
	shown   shownBody
}

// shownBody identifies what the body viewport currently holds.
type shownBody struct {
	itemID string
	width  int
}

// New returns a model that drives session.
func New(session *task.Session) Model {
	in := textinput.New()
	in.Prompt = "note> "
	m := Model{session: session, input: in, width: defaultWidth, height: defaultHeight, body: viewport.New()}
	return m.refresh()
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
	next, cmd := m.handle(msg)
	return next.refresh(), cmd
}

func (m Model) handle(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		if m.mode == modeNote {
			return m.updateNote(msg)
		}
		return m.updateBrowse(msg)
	}
	return m.forwardToNote(msg)
}

func (m Model) updateBrowse(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	m.status = ""
	key := msg.String()
	if b, ok := m.session.Task().BindingFor(key); ok {
		m.apply(b)
		return m, nil
	}
	switch key {
	case task.KeyInterrupt, task.KeyQuit:
		return m, tea.Quit
	case task.KeyPrev:
		m.session.Prev()
	case task.KeyNext:
		m.session.Next()
	case task.KeyHold:
		m.status = errText(m.session.Hold())
	case task.KeyUndo:
		m.status = errText(m.session.Undo())
	case task.KeyNote:
		return m.startNote()
	default:
		m.status = m.choose(msg.Text)
	}
	return m, nil
}

// apply moves the body viewport. Keys are matched here rather than forwarded
// to the viewport, whose own key map would claim letters used as labels.
func (m *Model) apply(b task.Binding) {
	switch b {
	case task.BindingScrollUp:
		m.body.ScrollUp(1)
	case task.BindingScrollDown:
		m.body.ScrollDown(1)
	case task.BindingPageUp:
		m.body.PageUp()
	case task.BindingPageDown:
		m.body.PageDown()
	case task.BindingNextHunk:
		m.jump(m.hunks.Next)
	case task.BindingPrevHunk:
		m.jump(m.hunks.Prev)
	}
}

func (m *Model) jump(find func(offset int) (int, bool)) {
	if line, ok := find(m.body.YOffset()); ok {
		m.body.SetYOffset(line)
	}
}

func (m Model) choose(text string) string {
	runes := []rune(text)
	if len(runes) != 1 {
		return ""
	}
	return errText(m.session.Choose(runes[0]))
}

func (m Model) startNote() (Model, tea.Cmd) {
	m.mode = modeNote
	m.input.SetValue(m.session.CurrentState().Note)
	m.input.CursorEnd()
	return m, m.input.Focus()
}

func (m Model) updateNote(msg tea.KeyPressMsg) (Model, tea.Cmd) {
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
func (m Model) forwardToNote(msg tea.Msg) (Model, tea.Cmd) {
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

// refresh re-renders the body when the item or width changed, starting a new
// item from its top, then fits the viewport between the fixed sections.
func (m Model) refresh() Model {
	want := shownBody{itemID: m.session.Current().ID, width: m.contentWidth()}
	if want != m.shown {
		body, err := render.NewBody(m.session.Task().Fields(), m.session.Current(), want.width)
		m.body.SetContent(body.Content())
		m.hunks = body.Hunks()
		if want.itemID != m.shown.itemID {
			m.body.GotoTop()
		}
		if err != nil {
			m.status = err.Error()
		}
		m.shown = want
	}
	m.body.SetWidth(want.width)
	m.body.SetHeight(m.bodyHeight())
	// A taller body lowers the largest offset; re-clamp so it stays filled.
	m.body.SetYOffset(m.body.YOffset())
	return m
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
