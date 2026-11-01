package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/furedea/rater/internal/task"
)

const (
	dirPerm  = 0o750
	filePerm = 0o640
)

// Log is one annotator's append-only JSONL file.
//
// Crash safety: each record is written with a single write of a full,
// newline-terminated line and then fsynced. A crash can therefore leave at
// most one torn final line without its newline; OpenLog drops that tail.
type Log struct {
	file      *os.File
	annotator task.Annotator
	now       func() time.Time
}

type recordDoc struct {
	Item      string    `json:"item"`
	Annotator string    `json:"annotator"`
	Action    string    `json:"action"`
	Label     string    `json:"label,omitempty"`
	Note      string    `json:"note,omitempty"`
	At        time.Time `json:"at"`
}

// OpenLog opens <dir>/annotations/<annotator>.jsonl for appending and returns its prior records.
func OpenLog(dir string, who task.Annotator) (*Log, []task.Annotation, error) {
	path := filepath.Join(dir, "annotations", who.String()+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return nil, nil, fmt.Errorf("create annotations dir: %w", err)
	}
	prior, err := recoverRecords(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, filePerm) //nolint:gosec // path is built from a validated Annotator
	if err != nil {
		return nil, nil, fmt.Errorf("open log %s: %w", path, err)
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return nil, nil, errors.Join(err, f.Close())
	}
	return &Log{file: f, annotator: who, now: time.Now}, prior, nil
}

// Append stamps the record with the annotator and the current time, then durably appends it.
func (l *Log) Append(a task.Annotation) error {
	a.Annotator = l.annotator.String()
	a.At = l.now().UTC()
	line, err := json.Marshal(toDoc(a))
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	if _, err := l.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write record: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("sync log: %w", err)
	}
	return nil
}

// Close releases the file.
func (l *Log) Close() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close log: %w", err)
	}
	return nil
}

func recoverRecords(path string) ([]task.Annotation, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from a validated Annotator
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read log %s: %w", path, err)
	}
	complete := completeLines(data)
	if len(complete) != len(data) {
		if err := os.Truncate(path, int64(len(complete))); err != nil {
			return nil, fmt.Errorf("drop torn tail of %s: %w", path, err)
		}
	}
	return parseRecords(complete)
}

func completeLines(data []byte) []byte {
	end := bytes.LastIndexByte(data, '\n')
	return data[:end+1]
}

func parseRecords(data []byte) ([]task.Annotation, error) {
	var records []task.Annotation
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, maxLineBytes)
	for line := 1; sc.Scan(); line++ {
		a, err := parseRecord(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		records = append(records, a)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}
	return records, nil
}

func parseRecord(line []byte) (task.Annotation, error) {
	var d recordDoc
	if err := json.Unmarshal(line, &d); err != nil {
		return task.Annotation{}, fmt.Errorf("decode record: %w", err)
	}
	action, err := task.ParseAction(d.Action)
	if err != nil {
		return task.Annotation{}, err
	}
	return task.Annotation{
		ItemID: d.Item, Annotator: d.Annotator, Action: action,
		Label: d.Label, Note: d.Note, At: d.At,
	}, nil
}

func toDoc(a task.Annotation) recordDoc {
	return recordDoc{
		Item: a.ItemID, Annotator: a.Annotator, Action: a.Action.String(),
		Label: a.Label, Note: a.Note, At: a.At,
	}
}

func syncDir(dir string) error {
	d, err := os.Open(dir) //nolint:gosec // directory under the task dir
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return fmt.Errorf("sync dir %s: %w", dir, syncErr)
	}
	return closeErr
}
