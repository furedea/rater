//go:build unix

package store_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/store"
)

// A first start checks for other adjudicators and creates its log while
// holding an exclusive lock on adjudications/, so two adjudicators starting
// on an empty task at once cannot both pass the check.
func TestAdjudicationLogChecksOtherAdjudicatorsUnderTheDirectoryLock(t *testing.T) {
	dir := t.TempDir()
	adjudications := filepath.Join(dir, "adjudications")
	require.NoError(t, os.Mkdir(adjudications, 0o750))
	held, err := os.Open(adjudications) //nolint:gosec // dir is a t.TempDir
	require.NoError(t, err)
	require.NoError(t, syscall.Flock(int(held.Fd()), syscall.LOCK_EX))

	done := make(chan error, 1)
	go func() {
		l, _, err := store.OpenAdjudicationLog(dir, mustAnnotator(t, "dave"))
		if err == nil {
			err = l.Close()
		}
		done <- err
	}()
	time.Sleep(100 * time.Millisecond) // let dave reach the check while carol holds the lock
	require.NoError(t, os.WriteFile(filepath.Join(adjudications, "carol.jsonl"), nil, 0o600))
	require.NoError(t, held.Close())

	select {
	case err := <-done:
		assert.ErrorIs(t, err, store.ErrOtherAdjudicator)
	case <-time.After(5 * time.Second):
		t.Fatal("dave's start did not finish after the lock was released")
	}
}
