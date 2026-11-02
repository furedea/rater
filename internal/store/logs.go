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

// ReadLogs reads every <dir>/annotations/<annotator>.jsonl, sorted by
// annotator name. It takes no lock and never writes, so annotators may keep
// working: a torn final line is skipped here and left for its writer to drop.
// Without an annotations directory there are no logs.
func ReadLogs(dir string) ([]AnnotatorLog, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open dir %s: %w", dir, err)
	}
	logs, readErr := readLogsIn(root.FS())
	closeErr := root.Close()
	if readErr != nil {
		return nil, readErr
	}
	return logs, closeErr
}

func readLogsIn(fsys fs.FS) ([]AnnotatorLog, error) {
	entries, err := fs.ReadDir(fsys, logDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", logDir, err)
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
	p := path.Join(logDir, name)
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
