package render

import (
	"errors"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/task"
)

const tabWidth = 4

var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	addedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	removedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	hunkHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	fileHeaderStyle = lipgloss.NewStyle().Bold(true)
)

// Body is the rendered text of one item's fields, wrapped to a width, and
// the lines at which its diff hunks start.
type Body struct {
	content string
	hunks   Hunks
}

// Content returns the rendered lines joined by newlines.
func (b Body) Content() string { return b.content }

// Hunks returns the line offsets of the diff hunks in Content.
func (b Body) Hunks() Hunks { return b.hunks }

// NewBody renders each field of item under its title. A field that fails to
// render is shown as plain text and its error is returned with the body.
func NewBody(fields []task.Field, item task.Item, width int) (Body, error) {
	w := bodyWriter{width: max(width, 1)}
	var errs []error
	for i, f := range fields {
		if i > 0 {
			w.addBlock("")
		}
		w.addBlock(titleStyle.Render(f.Title))
		errs = append(errs, w.addField(f, item))
	}
	return Body{content: strings.Join(w.lines, "\n"), hunks: NewHunks(w.hunks)}, errors.Join(errs...)
}

// bodyWriter collects wrapped lines and remembers where hunks start.
type bodyWriter struct {
	width int
	lines []string
	hunks []int
}

func (w *bodyWriter) addField(f task.Field, item task.Item) error {
	value := item.Values[f.Name]
	switch f.Render {
	case task.RenderText:
		w.addBlock(value)
	case task.RenderDiff:
		w.addDiff(value)
	case task.RenderCode:
		return w.addRendered(value, func(b *strings.Builder) error { return writeCode(b, value, languageOf(f, item)) })
	case task.RenderMarkdown:
		return w.addRendered(value, func(b *strings.Builder) error { return writeMarkdown(b, value, w.width) })
	}
	return nil
}

// addRendered adds what write produces, or the plain value if write fails.
func (w *bodyWriter) addRendered(value string, write func(*strings.Builder) error) error {
	var b strings.Builder
	if err := write(&b); err != nil {
		w.addBlock(value)
		return err
	}
	w.addBlock(b.String())
	return nil
}

func (w *bodyWriter) addDiff(text string) {
	for _, l := range ParseDiff(text) {
		if l.Kind == LineHunkHeader {
			w.hunks = append(w.hunks, len(w.lines))
		}
		w.addBlock(styleDiffLine(l))
	}
}

func styleDiffLine(l DiffLine) string {
	switch l.Kind {
	case LineAdded:
		return addedStyle.Render(l.Text)
	case LineRemoved:
		return removedStyle.Render(l.Text)
	case LineHunkHeader:
		return hunkHeaderStyle.Render(l.Text)
	case LineFileHeader:
		return fileHeaderStyle.Render(l.Text)
	case LineContext:
	}
	return l.Text
}

// addBlock wraps s to the width and appends its lines. lipgloss.Wrap reopens
// the active style on each wrapped row, so a row scrolled to on its own keeps
// its colour.
func (w *bodyWriter) addBlock(s string) {
	s = strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
	w.lines = append(w.lines, strings.Split(lipgloss.Wrap(s, w.width, ""), "\n")...)
}
