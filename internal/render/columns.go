package render

// Side-by-side geometry: two columns separated by a gap of ColumnGap cells.
const (
	ColumnGap          = 3
	MinColumnWidth     = 30
	MinSideBySideWidth = 2*MinColumnWidth + ColumnGap
)

// Columns is how a width splits into two side-by-side columns.
type Columns struct {
	Left  int
	Right int
}

// SplitColumns divides width into two columns and the gap between them. The
// left column takes the odd cell. It reports false when either column would be
// narrower than MinColumnWidth.
func SplitColumns(width int) (Columns, bool) {
	if width < MinSideBySideWidth {
		return Columns{}, false
	}
	usable := width - ColumnGap
	left := (usable + 1) / 2
	return Columns{Left: left, Right: usable - left}, true
}
