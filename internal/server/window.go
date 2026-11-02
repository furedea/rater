package server

import "charm.land/ssh"

// The largest screen a client gets. The program allocates a cell buffer of
// the client's terminal size, so an unbounded size would let one client
// exhaust the server's memory; these are far beyond any real terminal.
const (
	maxColumns = 1000
	maxRows    = 500
)

// screenSize is the client's window capped at maxColumns by maxRows.
func screenSize(w ssh.Window) (width, height int) {
	return min(w.Width, maxColumns), min(w.Height, maxRows)
}
