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
	lockErr := flock(f, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(lockErr, syscall.EWOULDBLOCK) {
		return ErrLogInUse
	}
	return lockErr
}

// waitLock takes an exclusive advisory lock that lasts until f is closed,
// waiting for another holder to release it.
func waitLock(f *os.File) error {
	return flock(f, syscall.LOCK_EX)
}

func flock(f *os.File, how int) error {
	conn, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf("get raw file: %w", err)
	}
	var lockErr error
	if err := conn.Control(func(fd uintptr) {
		lockErr = syscall.Flock(int(fd), how)
	}); err != nil {
		return fmt.Errorf("control raw file: %w", err)
	}
	if lockErr != nil {
		return fmt.Errorf("flock: %w", lockErr)
	}
	return nil
}
