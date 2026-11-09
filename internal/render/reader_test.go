package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/render"
)

// step is one annotator action on a reader: a hunk jump or a scroll that left
// the viewport at offset.
type step struct {
	op     string
	offset int
}

func play(r render.Reader, steps []step) render.Reader {
	for _, s := range steps {
		switch s.op {
		case "]":
			r, _ = r.NextHunk()
		case "[":
			r, _ = r.PrevHunk()
		case "scroll":
			r = r.ScrolledTo(s.offset)
		}
	}
	return r
}

func TestReaderReachesEveryHunk(t *testing.T) {
	t.Parallel()

	// The viewport can scroll no further than line 25, so the last hunk's
	// start (40) is never a viewport offset.
	hunks := render.NewHunks([]int{2, 20, 40})
	tests := []struct {
		name     string
		steps    []step
		wantLine int
		wantHunk int
	}{
		{"fresh reader is before every hunk", nil, 0, 0},
		{"one jump reaches the first hunk", []step{{op: "]"}}, 2, 1},
		{"jumps past the largest offset reach the last hunk", []step{{op: "]"}, {op: "]"}, {op: "]"}}, 40, 3},
		{"a jump beyond the last hunk stays on it", []step{{op: "]"}, {op: "]"}, {op: "]"}, {op: "]"}}, 40, 3},
		{"back from the last hunk returns to the one before", []step{{op: "]"}, {op: "]"}, {op: "]"}, {op: "["}}, 20, 2},
		{"scrolling replaces the jump position", []step{{op: "]"}, {op: "]"}, {op: "]"}, {op: "scroll", offset: 24}}, 24, 2},
		{"a jump after scrolling starts from the scrolled line", []step{{op: "scroll", offset: 21}, {op: "]"}}, 40, 3},
		{"back before the first hunk stays put", []step{{op: "]"}, {op: "["}}, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := play(render.NewReader(hunks), tt.steps)

			assert.Equal(t, tt.wantLine, r.Line())
			hunk, _ := r.Hunk()
			assert.Equal(t, tt.wantHunk, hunk)
		})
	}
}

func TestReaderWithoutHunks(t *testing.T) {
	t.Parallel()

	r := render.NewReader(render.Hunks{})

	_, moved := r.NextHunk()
	_, inHunk := r.Hunk()

	assert.False(t, moved)
	assert.False(t, inHunk)
}
