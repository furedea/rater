package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/render"
)

func TestSplitColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		want   render.Columns
		wantOK bool
	}{
		{"even remainder", 103, render.Columns{Left: 50, Right: 50}, true},
		{"odd remainder gives the left column the extra cell", 100, render.Columns{Left: 49, Right: 48}, true},
		{"narrowest that fits", render.MinSideBySideWidth, render.Columns{Left: render.MinColumnWidth, Right: render.MinColumnWidth}, true},
		{"one cell too narrow", render.MinSideBySideWidth - 1, render.Columns{}, false},
		{"zero width", 0, render.Columns{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := render.SplitColumns(tt.width)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
			if ok {
				assert.Equal(t, tt.width, got.Left+render.ColumnGap+got.Right, "columns and gap fill the width")
			}
		})
	}
}
