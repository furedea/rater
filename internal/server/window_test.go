package server

import (
	"testing"

	"charm.land/ssh"
	"github.com/stretchr/testify/assert"
)

// A client chooses its terminal size, and the screen allocates a buffer of
// that size, so the size is capped before it reaches the program.
func TestScreenSizeCapsClientWindows(t *testing.T) {
	tests := []struct {
		name          string
		window        ssh.Window
		width, height int
	}{
		{"ordinary terminal", ssh.Window{Width: 120, Height: 40}, 120, 40},
		{"at the cap", ssh.Window{Width: maxColumns, Height: maxRows}, maxColumns, maxRows},
		{"huge request", ssh.Window{Width: 1_000_000, Height: 1_000_000}, maxColumns, maxRows},
		{"one side too big", ssh.Window{Width: 80, Height: 1 << 30}, 80, maxRows},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			width, height := screenSize(tt.window)
			assert.Equal(t, []int{tt.width, tt.height}, []int{width, height})
		})
	}
}
