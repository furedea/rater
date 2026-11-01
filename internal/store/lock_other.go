//go:build !unix

package store

import "os"

// lockFile is a no-op where flock is unavailable.
func lockFile(*os.File) error { return nil }
