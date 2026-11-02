package render

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LineKind classifies one line of a unified diff.
type LineKind int

// Line kinds. Lines outside any hunk (diff, index, ---, +++) are file headers.
const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
	LineHunkHeader
	LineFileHeader
)

func (k LineKind) String() string {
	switch k {
	case LineContext:
		return "context"
	case LineAdded:
		return "added"
	case LineRemoved:
		return "removed"
	case LineHunkHeader:
		return "hunk-header"
	case LineFileHeader:
		return "file-header"
	}
	return fmt.Sprintf("LineKind(%d)", int(k))
}

// DiffLine is one classified line of a unified diff.
type DiffLine struct {
	Kind LineKind
	Text string
}

// ParseDiff classifies every line of a unified diff. A hunk lasts for the
// line counts its header declares, or, when the counts are missing or larger
// than the text, until the first line that cannot belong to a hunk body.
func ParseDiff(text string) []DiffLine {
	var h hunk
	raw := strings.Split(text, "\n")
	lines := make([]DiffLine, 0, len(raw))
	for _, s := range raw {
		lines = append(lines, DiffLine{Kind: h.classify(s), Text: s})
	}
	return lines
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// hunk tracks the body lines still expected by the current hunk.
type hunk struct {
	open    bool
	counted bool
	oldLeft int
	newLeft int
}

func (h *hunk) classify(s string) LineKind {
	if strings.HasPrefix(s, "@@") {
		*h = startHunk(s)
		return LineHunkHeader
	}
	if h.accepts(s) {
		return h.consume(s)
	}
	h.open = false
	if s == "" {
		return LineContext
	}
	return LineFileHeader
}

func startHunk(header string) hunk {
	m := hunkHeader.FindStringSubmatch(header)
	if m == nil {
		return hunk{open: true}
	}
	return hunk{open: true, counted: true, oldLeft: lineCount(m[1]), newLeft: lineCount(m[2])}
}

// lineCount reads a hunk range length, which defaults to 1 when omitted.
func lineCount(s string) int {
	if s == "" {
		return 1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func (h *hunk) accepts(s string) bool {
	if !h.open || (h.counted && h.oldLeft <= 0 && h.newLeft <= 0) {
		return false
	}
	return s == "" || strings.ContainsRune(" +-\\", rune(s[0]))
}

func (h *hunk) consume(s string) LineKind {
	switch {
	case strings.HasPrefix(s, "+"):
		h.newLeft--
		return LineAdded
	case strings.HasPrefix(s, "-"):
		h.oldLeft--
		return LineRemoved
	case strings.HasPrefix(s, "\\"):
		return LineContext
	default:
		h.oldLeft--
		h.newLeft--
		return LineContext
	}
}
