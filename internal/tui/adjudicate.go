package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/adjudication"
	"github.com/furedea/rater/internal/task"
)

// Judgments supplies what each annotator gave an item. The adjudication
// screen reads nothing else of the comparison, so a docket or a test fake
// can provide it.
type Judgments interface {
	Judgments(itemID string) []adjudication.Judgment
}

// NewAdjudication returns a model in which session holds an adjudicator's
// decisions over the disputed items and judgments shows the annotators'
// answers beside each item. Keys work as in annotate.
func NewAdjudication(session *task.Session, judgments Judgments) Model {
	m := New(session)
	m.judgments = judgments
	return m.refresh()
}

// RunAdjudication shows the adjudication screen until the adjudicator quits.
// Like annotate, every decision is in the log when it is made.
func RunAdjudication(session *task.Session, judgments Judgments) error {
	return run(NewAdjudication(session, judgments), "adjudicate")
}

func (m Model) adjudicating() bool { return m.judgments != nil }

func (m Model) currentJudgments() []adjudication.Judgment {
	return m.judgments.Judgments(m.session.Current().ID)
}

// judgmentLines lists each annotator's label and note under a single-choice legend.
func (m Model) judgmentLines() string {
	var lines []string
	for _, j := range m.currentJudgments() {
		line := j.Annotator + ": " + answerOf(j.State, j.State.Label)
		if j.State.Note != "" {
			line += " · note: " + j.State.Note
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// adjudicationChecklist is a table with one row per criterion and a verdict
// row, and one column for the final answers followed by one per annotator.
func (m Model) adjudicationChecklist(c task.Checklist) string {
	judgments := m.currentJudgments()
	final := m.session.CurrentState().Answers
	header := []string{"", "", "final"}
	for _, j := range judgments {
		header = append(header, j.Annotator)
	}
	rows := [][]string{header}
	for i, cr := range c.Criteria() {
		row := []string{focusMarker(i == m.session.Focus()), cr.Title, cell(final, cr.Name)}
		for _, j := range judgments {
			row = append(row, cell(j.State.Answers, cr.Name))
		}
		rows = append(rows, row)
	}
	verdicts := []string{"", "verdict", c.Verdict(final).String()}
	for _, j := range judgments {
		verdicts = append(verdicts, answerOf(j.State, c.Verdict(j.State.Answers).String()))
	}
	rows = append(rows, verdicts)
	return strings.Join(append([]string{legend(c.Answers()), table(rows)}, m.judgmentNotes()...), "\n")
}

func (m Model) judgmentNotes() []string {
	var notes []string
	for _, j := range m.currentJudgments() {
		if j.State.Note != "" {
			notes = append(notes, fmt.Sprintf("%s note: %s", j.Annotator, j.State.Note))
		}
	}
	return notes
}

// answerOf is an annotator's answer when they completed the item, else their status.
func answerOf(s task.State, answer string) string {
	if s.Status != task.StatusLabeled {
		return mutedStyle.Render(s.Status.String())
	}
	return answer
}

func cell(answers map[string]string, criterion string) string {
	if a, ok := answers[criterion]; ok {
		return a
	}
	return mutedStyle.Render("·")
}

func focusMarker(focused bool) string {
	if focused {
		return "▶"
	}
	return " "
}

// table left-aligns cells in columns two spaces apart.
func table(rows [][]string) string {
	var widths []int
	for _, row := range rows {
		for i, c := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i, c := range row {
			b.WriteString(c + strings.Repeat(" ", widths[i]-lipgloss.Width(c)+2))
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return strings.Join(lines, "\n")
}
