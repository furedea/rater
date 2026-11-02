package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	// maxWidth caps a stacked body; side-by-side gets room for two such columns.
	maxWidth           = 100
	maxSideBySideWidth = 160
	minBodyHeight      = 3
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	mutedStyle  = lipgloss.NewStyle().Faint(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

const controlHelp = "tab: hold · ←/→: prev/next · ctrl+u: undo · ctrl+n: note · esc: quit"

// View implements tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string { return m.compose(m.bodyView()) }

// compose lays the fixed sections around the scrollable body.
func (m Model) compose(body string) string {
	sections := []string{
		m.headerLine(), m.metaLine(), m.layoutNotice(), body, m.scrollLine(),
		m.answerSection(), m.stateLine(), m.noteLine(), m.statusLine(), m.helpLines(),
	}
	frame := lipgloss.NewStyle().Width(m.contentWidth())
	return frame.Render(strings.Join(nonEmpty(sections), "\n\n"))
}

func (m Model) contentWidth() int {
	limit := maxWidth
	if m.session.Task().Layout() == task.LayoutSideBySide {
		limit = maxSideBySideWidth
	}
	return max(min(m.width, limit), 1)
}

// bodyHeight is what the terminal leaves for the body after the fixed sections.
func (m Model) bodyHeight() int {
	chrome := lipgloss.Height(m.compose("x")) - 1
	return max(m.height-chrome, minBodyHeight)
}

// bodyView draws one pane, or two columns separated by a ruled gap.
func (m Model) bodyView() string {
	if len(m.panes) == 1 {
		return m.panes[0].view.View()
	}
	cols, _ := m.columns()
	height := m.panes[0].view.Height()
	gap := strings.TrimSuffix(strings.Repeat(" │ \n", height), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, m.panes[0].render(cols.Left), gap, m.panes[1].render(cols.Right))
}

func (m Model) headerLine() string {
	n, total := m.session.Position()
	progress := fmt.Sprintf("%d / %d", n, total)
	done := fmt.Sprintf("labeled %d", m.session.Labeled())
	if m.adjudicating() {
		done = fmt.Sprintf("resolved %d / %d", m.session.Labeled(), total)
	}
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

// layoutNotice explains a side-by-side task shown stacked on a narrow terminal.
func (m Model) layoutNotice() string {
	if m.session.Task().Layout() != task.LayoutSideBySide || len(m.panes) != 1 {
		return ""
	}
	return statusStyle.Render(fmt.Sprintf(
		"terminal too narrow for side-by-side (needs %d columns); showing stacked", render.MinSideBySideWidth))
}

// scrollLine shows each pane's position; with two panes, ▶ marks the focused one.
func (m Model) scrollLine() string {
	if len(m.panes) == 1 {
		return mutedStyle.Render(m.panes[0].position())
	}
	cols, _ := m.columns()
	parts := make([]string, len(m.panes))
	widths := []int{cols.Left, cols.Right}
	for i, p := range m.panes {
		marker := "  "
		if i == m.focus {
			marker = "▶ "
		}
		parts[i] = lipgloss.NewStyle().Width(widths[i]).Render(marker + p.position())
	}
	return mutedStyle.Render(parts[0] + strings.Repeat(" ", render.ColumnGap) + parts[1])
}

func (m Model) helpLines() string {
	tk := m.session.Task()
	noun := "label"
	if _, ok := tk.Scheme().(task.Checklist); ok {
		noun = "answer"
	}
	if m.adjudicating() {
		noun = "final " + noun
	}
	first := "key: " + noun + " · " + controlHelp
	second := []string{
		fmt.Sprintf("%s/%s: scroll", tk.KeyFor(task.BindingScrollUp), tk.KeyFor(task.BindingScrollDown)),
		fmt.Sprintf("%s/%s: page", tk.KeyFor(task.BindingPageUp), tk.KeyFor(task.BindingPageDown)),
	}
	if m.hasHunks() {
		second = append(second, fmt.Sprintf("%s/%s: hunk", tk.KeyFor(task.BindingPrevHunk), tk.KeyFor(task.BindingNextHunk)))
	}
	if _, ok := tk.Scheme().(task.Checklist); ok {
		second = append(second, fmt.Sprintf("%s/%s: criterion", tk.KeyFor(task.BindingPrevCriterion), tk.KeyFor(task.BindingNextCriterion)))
	}
	if len(m.panes) > 1 {
		second = append(second, tk.KeyFor(task.BindingSwitchPane)+": switch pane")
	}
	return mutedStyle.Render(first + "\n" + strings.Join(second, " · "))
}

func (m Model) hasHunks() bool {
	for _, p := range m.panes {
		if p.hunks.Len() > 0 {
			return true
		}
	}
	return false
}

// answerSection shows the keys of the task's scheme: the labels, or the
// answers above the criteria and the verdict they lead to.
func (m Model) answerSection() string {
	switch s := m.session.Task().Scheme().(type) {
	case task.SingleChoice:
		if m.adjudicating() {
			return legend(s.Labels()) + "\n\n" + m.judgmentLines()
		}
		return legend(s.Labels())
	case task.Checklist:
		if m.adjudicating() {
			return m.adjudicationChecklist(s)
		}
		return m.checklistView(s)
	}
	return ""
}

func (m Model) checklistView(c task.Checklist) string {
	answers := m.session.CurrentState().Answers
	criteria := c.Criteria()
	titleWidth := 0
	for _, cr := range criteria {
		titleWidth = max(titleWidth, lipgloss.Width(cr.Title))
	}
	lines := []string{legend(c.Answers())}
	for i, cr := range criteria {
		marker := "  "
		if i == m.session.Focus() {
			marker = "▶ "
		}
		answer, ok := answers[cr.Name]
		if !ok {
			answer = mutedStyle.Render("·")
		}
		lines = append(lines, fmt.Sprintf("%s%-*s  %s", marker, titleWidth, cr.Title, answer))
	}
	lines = append(lines, "verdict: "+c.Verdict(answers).String())
	return strings.Join(lines, "\n")
}

func legend(labels []task.Label) string {
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		parts = append(parts, fmt.Sprintf("[%c] %s", l.Key, l.Name))
	}
	return strings.Join(parts, "   ")
}

func (m Model) stateLine() string {
	return "state: " + describe(m.session.Task().Scheme(), m.session.CurrentState())
}

func describe(scheme task.Scheme, s task.State) string {
	switch s.Status {
	case task.StatusLabeled:
		if _, ok := scheme.(task.Checklist); ok {
			return "complete"
		}
		return "labeled as " + s.Label
	case task.StatusHeld:
		return "held"
	case task.StatusPending:
		if c, ok := scheme.(task.Checklist); ok {
			return fmt.Sprintf("%d of %d answered", len(s.Answers), len(c.Criteria()))
		}
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
