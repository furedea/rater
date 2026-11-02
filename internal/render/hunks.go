package render

import "slices"

// Hunks holds the body line offsets at which diff hunks start, ascending.
type Hunks struct {
	starts []int
}

// NewHunks sorts starts into a Hunks.
func NewHunks(starts []int) Hunks {
	sorted := slices.Clone(starts)
	slices.Sort(sorted)
	return Hunks{starts: sorted}
}

// Len returns the number of hunks.
func (h Hunks) Len() int { return len(h.starts) }

// Next returns the first hunk start below offset.
func (h Hunks) Next(offset int) (int, bool) {
	for _, s := range h.starts {
		if s > offset {
			return s, true
		}
	}
	return 0, false
}

// Prev returns the last hunk start above offset.
func (h Hunks) Prev(offset int) (int, bool) {
	for _, s := range slices.Backward(h.starts) {
		if s < offset {
			return s, true
		}
	}
	return 0, false
}
