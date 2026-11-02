package render

// Reader is the line an annotator is reading in a scrolled body.
//
// A hunk jump moves the reader to the hunk's own start even when the viewport
// cannot scroll that far (near the end of a body its offset stops at the
// largest one). Jumps then continue from the hunk, not from the clamped
// offset, so repeated jumps reach every hunk. A scroll replaces the line with
// the viewport's offset.
type Reader struct {
	hunks Hunks
	line  int
}

// NewReader starts reading at the top of a body with the given hunks.
func NewReader(h Hunks) Reader { return Reader{hunks: h} }

// Line returns the line being read; it may exceed the viewport's largest offset.
func (r Reader) Line() int { return r.line }

// ScrolledTo records that the viewport now starts at offset.
func (r Reader) ScrolledTo(offset int) Reader {
	r.line = offset
	return r
}

// NextHunk moves to the first hunk below the line being read.
func (r Reader) NextHunk() (Reader, bool) { return r.jump(r.hunks.Next) }

// PrevHunk moves to the last hunk above the line being read.
func (r Reader) PrevHunk() (Reader, bool) { return r.jump(r.hunks.Prev) }

// Hunk returns the 1-based number of the hunk being read, if the line is in one.
func (r Reader) Hunk() (int, bool) {
	n := 0
	for _, s := range r.hunks.starts {
		if s <= r.line {
			n++
		}
	}
	return n, n > 0
}

func (r Reader) jump(find func(offset int) (int, bool)) (Reader, bool) {
	line, ok := find(r.line)
	if !ok {
		return r, false
	}
	r.line = line
	return r, true
}
