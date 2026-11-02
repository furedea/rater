// Package tui renders an annotation session with Bubble Tea. It only maps
// keys to session calls and session state to text; the rules live in task.
package tui

import (
	"errors"
	"fmt"

	"charm.land/bubbles/v2/textinput"
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
//
// The body is one pane, or two side by side when the task asks for it and the
// terminal is wide enough. Each pane scrolls on its own; the scroll and hunk
// keys act on the focused pane and switch_pane moves the focus. Panes are not
// scrolled together because the two fields usually differ in length and kind
// (a short comment beside a long diff), so line-locked scrolling would pair
// unrelated lines and stop at the shorter field.
type Model struct {
	session *task.Session
	mode    mode
	input   textinput.Model
	width   int
	height  int
	status  string
	panes   []pane
	focus   int
	shown   shownBody
}

// shownBody identifies what the panes currently hold.
type shownBody struct {
	itemID  string
	width   int
	columns bool
}

// New returns a model that drives session.
func New(session *task.Session) Model {
	in := textinput.New()
	in.Prompt = "note> "
	m := Model{session: session, input: in, width: defaultWidth, height: defaultHeight}
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

// apply performs a view binding. Keys are matched here rather than forwarded
// to the viewport, whose own key map would claim letters used as labels.
func (m *Model) apply(b task.Binding) {
	p := &m.panes[m.focus]
	switch b {
	case task.BindingScrollUp:
		p.lineUp()
	case task.BindingScrollDown:
		p.lineDown()
	case task.BindingPageUp:
		p.pageUp()
	case task.BindingPageDown:
		p.pageDown()
	case task.BindingNextHunk:
		p.nextHunk()
	case task.BindingPrevHunk:
		p.prevHunk()
	case task.BindingPrevCriterion:
		m.session.FocusPrev()
	case task.BindingNextCriterion:
		m.session.FocusNext()
	case task.BindingSwitchPane:
		m.focus = (m.focus + 1) % len(m.panes)
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

// refresh re-renders the panes when the item, width, or arrangement changed,
// starting a new item or arrangement from the top, then fits the panes between
// the fixed sections.
func (m Model) refresh() Model {
	cols, columns := m.columns()
	want := shownBody{itemID: m.session.Current().ID, width: m.contentWidth(), columns: columns}
	if want != m.shown {
		rearranged := m.panes == nil || want.columns != m.shown.columns
		if rearranged {
			m.panes, m.focus = m.newPanes(columns), 0
		}
		fresh := rearranged || want.itemID != m.shown.itemID
		widths := []int{want.width}
		if columns {
			widths = []int{cols.Left, cols.Right}
		}
		var errs []error
		for i := range m.panes {
			errs = append(errs, m.panes[i].fill(m.session.Current(), widths[i], fresh))
		}
		if err := errors.Join(errs...); err != nil {
			m.status = err.Error()
		}
		m.shown = want
	}
	height := m.bodyHeight()
	for i := range m.panes {
		m.panes[i].resize(height)
	}
	return m
}

// columns splits the content width for a side-by-side task; it reports false
// for a stacked task or a terminal too narrow for two columns.
func (m Model) columns() (render.Columns, bool) {
	if m.session.Task().Layout() != task.LayoutSideBySide {
		return render.Columns{}, false
	}
	return render.SplitColumns(m.contentWidth())
}

func (m Model) newPanes(columns bool) []pane {
	fields := m.session.Task().Fields()
	if !columns {
		return []pane{newPane(fields)}
	}
	return []pane{newPane(fields[:1]), newPane(fields[1:])}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
