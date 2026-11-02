package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSessionArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"valid", []string{"--task", "dir", "--as", "alice"}, ""},
		{"missing task", []string{"--as", "alice"}, "--task is required"},
		{"missing annotator", []string{"--task", "dir"}, "--as"},
		{"bad annotator", []string{"--task", "dir", "--as", "../x"}, "--as"},
		{"unknown flag", []string{"--task", "dir", "--as", "a", "--nope"}, "parse flags"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseSessionArgs("annotate", tt.args)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "dir", opts.taskDir)
			assert.Equal(t, "alice", opts.who.String())
		})
	}
}

func TestRunRejectsBadSubcommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"unknown", []string{"explode"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, run(tt.args, io.Discard), errUsage)
		})
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, run([]string{"--version"}, &out))

	assert.Regexp(t, `^rater \d+\.\d+\.\d+\n$`, out.String())
}
