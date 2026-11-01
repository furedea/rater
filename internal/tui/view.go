package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/task"
)

const (
	defaultWidth = 80
	maxWidth     = 100
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	mutedStyle  = lipgloss.NewStyle().Faint(true)
	fieldStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

const helpText = "key: label · tab: hold · ←/→: prev/next · ctrl+u: undo · ctrl+n: note · esc: quit"

// View implements tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	width := min(m.width, maxWidth)
	body := lipgloss.NewStyle().Width(width)
	sections := []string{m.headerLine(), m.metaLine()}
	sections = append(sections, m.fieldBlocks()...)
	sections = append(sections, m.labelLegend(), m.stateLine(), m.noteLine(), m.statusLine(), mutedStyle.Render(helpText))
	return body.Render(strings.Join(nonEmpty(sections), "\n\n"))
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

func (m Model) fieldBlocks() []string {
	item := m.session.Current()
	var blocks []string
	for _, f := range m.session.Task().Fields() {
		blocks = append(blocks, fieldStyle.Render(f.Title)+"\n"+item.Values[f.Name])
	}
	return blocks
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
