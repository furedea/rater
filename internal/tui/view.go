package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/task"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	maxWidth      = 100
	minBodyHeight = 3
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	mutedStyle  = lipgloss.NewStyle().Faint(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

const helpText = "key: label · tab: hold · ←/→: prev/next · ctrl+u: undo · ctrl+n: note · esc: quit"

// View implements tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string { return m.compose(m.body.View()) }

// compose lays the fixed sections around the scrollable body.
func (m Model) compose(body string) string {
	sections := []string{
		m.headerLine(), m.metaLine(), body, m.scrollLine(),
		m.labelLegend(), m.stateLine(), m.noteLine(), m.statusLine(), m.helpLines(),
	}
	frame := lipgloss.NewStyle().Width(m.contentWidth())
	return frame.Render(strings.Join(nonEmpty(sections), "\n\n"))
}

func (m Model) contentWidth() int { return max(min(m.width, maxWidth), 1) }

// bodyHeight is what the terminal leaves for the body after the fixed sections.
func (m Model) bodyHeight() int {
	chrome := lipgloss.Height(m.compose("x")) - 1
	return max(m.height-chrome, minBodyHeight)
}

func (m Model) headerLine() string {
	n, total := m.session.Position()
	progress := fmt.Sprintf("%d / %d", n, total)
	done := fmt.Sprintf("labeled %d", m.session.Labeled())
	return titleStyle.Render(m.session.Task().Name()) + "  " + progress + "  " + mutedStyle.Render(done)
}

func (m Model) metaLine() string {
	item := m.session.Current()
	parts := []string{"id: " + item.ID}
	for _, name := range m.session.Task().Header() {
		parts = append(parts, name+": "+item.Values[name])
	}
	return mutedStyle.Render(strings.Join(parts, " · "))
}

func (m Model) scrollLine() string {
	total := m.body.TotalLineCount()
	first := min(m.body.YOffset()+1, total)
	last := min(m.body.YOffset()+m.body.Height(), total)
	return mutedStyle.Render(fmt.Sprintf("lines %d–%d of %d", first, last, total))
}

func (m Model) helpLines() string {
	tk := m.session.Task()
	scroll := fmt.Sprintf("%s/%s: scroll · %s/%s: page",
		tk.KeyFor(task.BindingScrollUp), tk.KeyFor(task.BindingScrollDown),
		tk.KeyFor(task.BindingPageUp), tk.KeyFor(task.BindingPageDown))
	if m.hunks.Len() > 0 {
		scroll += fmt.Sprintf(" · %s/%s: hunk", tk.KeyFor(task.BindingPrevHunk), tk.KeyFor(task.BindingNextHunk))
	}
	return mutedStyle.Render(helpText + "\n" + scroll)
}

func (m Model) labelLegend() string {
	var parts []string
	for _, l := range m.session.Task().Labels() {
		parts = append(parts, fmt.Sprintf("[%c] %s", l.Key, l.Name))
	}
	return strings.Join(parts, "   ")
}

func (m Model) stateLine() string {
	return "state: " + describe(m.session.CurrentState())
}

func describe(s task.State) string {
	switch s.Status {
	case task.StatusLabeled:
		return "labeled as " + s.Label
	case task.StatusHeld:
		return "held"
	case task.StatusPending:
		return "pending"
	}
	return s.Status.String()
}

func (m Model) noteLine() string {
	if m.mode == modeNote {
		return m.input.View()
	}
	if note := m.session.CurrentState().Note; note != "" {
		return "note: " + note
	}
	return ""
}

func (m Model) statusLine() string {
	if m.status == "" {
		return ""
	}
	return statusStyle.Render(m.status)
}

func nonEmpty(parts []string) []string {
	out := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
