package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/furedea/rater/internal/task"
)

func TestNewAnnotator(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "alice", false},
		{"with dash and digit", "rater-2", false},
		{"with underscore", "rater_b", false},
		{"empty", "", true},
		{"uppercase", "Alice", true},
		{"path traversal", "../x", true},
		{"slash", "a/b", true},
		{"leading dash", "-a", true},
		{"space", "a b", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := task.NewAnnotator(tt.input)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.input, a.String())
			}
		})
	}
}
