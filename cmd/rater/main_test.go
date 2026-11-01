package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAnnotateArgs(t *testing.T) {
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
			opts, err := parseAnnotateArgs(tt.args)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "dir", opts.taskDir)
			assert.Equal(t, "alice", opts.annotator.String())
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
			assert.ErrorIs(t, run(tt.args), errUsage)
		})
	}
}
