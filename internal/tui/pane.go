package tui

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

	"github.com/furedea/rater/internal/render"
	"github.com/furedea/rater/internal/task"
)

// pane is one scrollable column of the body: the fields it shows, their
// rendered lines in a viewport, and the reading line that hunk jumps follow.
type pane struct {
	fields []task.Field
	view   viewport.Model
	hunks  render.Hunks
	reader render.Reader
}

func newPane(fields []task.Field) pane {
	return pane{fields: fields, view: viewport.New()}
}

// fill renders item at width. A new item starts from its top; a re-render of
// the same item at another width keeps the viewport offset.
func (p *pane) fill(item task.Item, width int, fresh bool) error {
	body, err := render.NewBody(p.fields, item, width)
	p.view.SetWidth(width)
	p.view.SetContent(body.Content())
	if fresh {
		p.view.GotoTop()
	}
	p.hunks = body.Hunks()
	p.reader = render.NewReader(p.hunks).ScrolledTo(p.view.YOffset())
	return err
}

// resize fits the viewport to height. A taller body lowers the largest offset;
// re-clamping keeps the viewport filled. The reading line is left alone, so a
// jump target below the largest offset survives.
func (p *pane) resize(height int) {
	p.view.SetHeight(height)
	p.view.SetYOffset(p.view.YOffset())
}

func (p *pane) lineUp()   { p.view.ScrollUp(1); p.syncReader() }
func (p *pane) lineDown() { p.view.ScrollDown(1); p.syncReader() }
func (p *pane) pageUp()   { p.view.PageUp(); p.syncReader() }
func (p *pane) pageDown() { p.view.PageDown(); p.syncReader() }

func (p *pane) nextHunk() { p.jump(p.reader.NextHunk) }
func (p *pane) prevHunk() { p.jump(p.reader.PrevHunk) }

func (p *pane) jump(move func() (render.Reader, bool)) {
	if r, ok := move(); ok {
		p.reader = r
		p.view.SetYOffset(r.Line())
	}
}

func (p *pane) syncReader() { p.reader = p.reader.ScrolledTo(p.view.YOffset()) }

// render draws the viewport as a block of exactly width × height cells.
func (p pane) render(width int) string {
	return lipgloss.NewStyle().Width(width).Height(p.view.Height()).MaxHeight(p.view.Height()).Render(p.view.View())
}

// position describes the visible lines and, in a diff, the hunk being read.
func (p pane) position() string {
	total := p.view.TotalLineCount()
	first := min(p.view.YOffset()+1, total)
	last := min(p.view.YOffset()+p.view.Height(), total)
	text := fmt.Sprintf("lines %d–%d of %d", first, last, total)
	if n, ok := p.reader.Hunk(); ok {
		text += fmt.Sprintf(" · hunk %d of %d", n, p.hunks.Len())
	}
	return text
}
