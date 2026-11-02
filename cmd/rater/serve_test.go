package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseServeArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    serveOptions
		wantErr string
	}{
		{
			"host key defaults into the task dir",
			[]string{"--task", "dir", "--listen", ":2222", "--keys", "keys.txt"},
			serveOptions{taskDir: "dir", listen: ":2222", keysPath: "keys.txt", hostKeyPath: filepath.Join("dir", "host_ed25519")},
			"",
		},
		{
			"explicit host key",
			[]string{"--task", "dir", "--listen", ":2222", "--keys", "keys.txt", "--host-key", "/srv/host"},
			serveOptions{taskDir: "dir", listen: ":2222", keysPath: "keys.txt", hostKeyPath: "/srv/host"},
			"",
		},
		{"missing task", []string{"--listen", ":2222", "--keys", "k"}, serveOptions{}, "--task is required"},
		{"missing listen", []string{"--task", "dir", "--keys", "k"}, serveOptions{}, "--listen is required"},
		{"missing keys", []string{"--task", "dir", "--listen", ":2222"}, serveOptions{}, "--keys is required"},
		{"unknown flag", []string{"--task", "dir", "--as", "alice"}, serveOptions{}, "parse flags"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServeArgs(tt.args)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServeHelpExplainsKeysAndAssignment(t *testing.T) {
	var out bytes.Buffer

	err := serve(context.Background(), []string{"--help"}, &out)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "usage: rater serve")
	assert.Contains(t, out.String(), "assignment.yaml")
	assert.Contains(t, out.String(), "adjudicate is not served")
}

func TestServeRefusesAnInvalidKeysFileBeforeListening(t *testing.T) {
	dir := taskDir(t, pairsTask, nil)
	keys := filepath.Join(t.TempDir(), "keys.txt")
	require.NoError(t, os.WriteFile(keys, []byte("alice\n"), 0o600))

	err := serve(context.Background(), []string{"--task", dir, "--listen", "127.0.0.1:0", "--keys", keys}, &bytes.Buffer{})

	assert.ErrorContains(t, err, "invalid keys file")
	assert.NoFileExists(t, filepath.Join(dir, "host_ed25519"))
}
