package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/furedea/rater/internal/task"
)

// AnnotatorLog is the records one annotator's log held when it was read.
type AnnotatorLog struct {
	Annotator task.Annotator
	Records   []task.Annotation
}

const logExt = ".jsonl"

// ErrNoLog means an annotator has no log in the task directory.
var ErrNoLog = errors.New("annotator has no log")

// ReadLogs reads every <dir>/annotations/<annotator>.jsonl, sorted by
// annotator name. It takes no lock and never writes, so annotators may keep
// working: a torn final line is skipped here and left for its writer to drop.
// Without an annotations directory there are no logs.
func ReadLogs(dir string) ([]AnnotatorLog, error) {
	return inRoot(dir, readLogsIn)
}

// ReadLog reads one annotator's log the way ReadLogs does.
func ReadLog(dir string, who task.Annotator) (AnnotatorLog, error) {
	l, err := inRoot(dir, func(fsys fs.FS) (AnnotatorLog, error) {
		return readLog(fsys, who.String()+logExt)
	})
	if errors.Is(err, fs.ErrNotExist) {
		return AnnotatorLog{}, fmt.Errorf("%w: %s", ErrNoLog, who)
	}
	return l, err
}

// inRoot runs read on the file system confined to dir.
func inRoot[T any](dir string, read func(fs.FS) (T, error)) (T, error) {
	var zero T
	root, err := os.OpenRoot(dir)
	if err != nil {
		return zero, fmt.Errorf("open dir %s: %w", dir, err)
	}
	v, readErr := read(root.FS())
	closeErr := root.Close()
	if readErr != nil {
		return zero, readErr
	}
	return v, closeErr
}

func readLogsIn(fsys fs.FS) ([]AnnotatorLog, error) {
	entries, err := fs.ReadDir(fsys, annotationsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", annotationsDir, err)
	}
	var logs []AnnotatorLog
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), logExt) {
			continue
		}
		l, err := readLog(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func readLog(fsys fs.FS, name string) (AnnotatorLog, error) {
	who, err := task.NewAnnotator(strings.TrimSuffix(name, logExt))
	if err != nil {
		return AnnotatorLog{}, fmt.Errorf("log %s: %w", name, err)
	}
	p := path.Join(annotationsDir, name)
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return AnnotatorLog{}, fmt.Errorf("read log %s: %w", p, err)
	}
	records, err := parseRecords(completeLines(data))
	if err != nil {
		return AnnotatorLog{}, fmt.Errorf("parse log %s: %w", p, err)
	}
	return AnnotatorLog{Annotator: who, Records: records}, nil
}
