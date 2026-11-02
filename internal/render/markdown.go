package render

import (
	"fmt"
	"io"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
)

// writeMarkdown renders src with Glamour's dark style, wrapped to width.
// The style is fixed rather than detected so rendering never queries the terminal.
func writeMarkdown(w io.Writer, src string, width int) error {
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(styles.DarkStyle), glamour.WithWordWrap(width))
	if err != nil {
		return fmt.Errorf("create markdown renderer: %w", err)
	}
	out, err := r.Render(src)
	if err != nil {
		return fmt.Errorf("render markdown: %w", err)
	}
	if _, err := io.WriteString(w, strings.Trim(out, "\n")); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}
	return nil
}
