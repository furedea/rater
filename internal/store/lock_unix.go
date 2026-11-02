//go:build unix

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive advisory lock that lasts until f is closed.
func lockFile(f *os.File) error {
	conn, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw file: %w", err)
	}
	var lockErr error
	if err := conn.Control(func(fd uintptr) {
		lockErr = syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
	}); err != nil {
		return fmt.Errorf("control raw file: %w", err)
	}
	if errors.Is(lockErr, syscall.EWOULDBLOCK) {
		return ErrLogInUse
	}
	if lockErr != nil {
		return fmt.Errorf("flock: %w", lockErr)
	}
	return nil
}
