package task

import (
	"fmt"
	"regexp"
)

var annotatorPattern = regexp.MustCompile(`^[a-z0-9]+([_-][a-z0-9]+)*$`)

// Annotator names one person; it doubles as a log file name, so it is restricted.
type Annotator struct{ name string }

// NewAnnotator validates name once.
func NewAnnotator(name string) (Annotator, error) {
	if !annotatorPattern.MatchString(name) {
		return Annotator{}, fmt.Errorf("annotator %q must match %s", name, annotatorPattern)
	}
	return Annotator{name: name}, nil
}

// String returns the annotator name.
func (a Annotator) String() string { return a.name }
