package server_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/furedea/rater/internal/server"
)

func TestLoadHostKeyCreatesAnOwnerOnlyKeyOnFirstStart(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "host_ed25519")

	_, err := server.LoadHostKey(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoadHostKeyReusesThePersistedKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "host_ed25519")
	first, err := server.LoadHostKey(path)
	require.NoError(t, err)

	again, err := server.LoadHostKey(path)
	require.NoError(t, err)

	assert.Equal(t, first.PublicKey().Marshal(), again.PublicKey().Marshal())
}

func TestLoadHostKeyRefusesAKeyOthersCanRead(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "host_ed25519")
	_, err := server.LoadHostKey(path)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(path, 0o644)) //nolint:gosec // the test exposes the key on purpose

	_, err = server.LoadHostKey(path)

	assert.ErrorIs(t, err, server.ErrHostKeyExposed)
}
