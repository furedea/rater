package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	file      logFile
	size      int64 // length of the durable, newline-terminated prefix
	broken    error // set when a rollback failed; the tail may be torn, so appends stop
	annotator task.Annotator
	now       func() time.Time
}

// logFile is the part of *os.File that Log needs; tests substitute a failing one.
type logFile interface {
	io.Writer
	Sync() error
	Truncate(size int64) error
	Close() error
}

// recordDoc is one JSONL line. Fields added after the first release are
// omitempty, so older lines decode unchanged with those fields empty.
type recordDoc struct {
	Item      string    `json:"item"`
	Annotator string    `json:"annotator"`
	Action    string    `json:"action"`
	Criterion string    `json:"criterion,omitempty"`
	Label     string    `json:"label,omitempty"`
	Note      string    `json:"note,omitempty"`
	At        time.Time `json:"at"`
}

// ErrLogInUse means another process already has this annotator's log open.
var ErrLogInUse = errors.New("annotation log is in use by another process")

// Log directories under a task directory. Adjudications live apart from
// annotations so that an adjudicator is never compared as an annotator.
const (
	annotationsDir   = "annotations"
	adjudicationsDir = "adjudications"
)

// OpenLog opens <dir>/annotations/<annotator>.jsonl for appending and returns its prior records.
// It holds an exclusive lock until Close, so two processes cannot append to one log.
func OpenLog(dir string, who task.Annotator) (*Log, []task.Annotation, error) {
	return openLogAt(dir, annotationsDir, who)
}

// OpenAdjudicationLog opens <dir>/adjudications/<adjudicator>.jsonl the way
// OpenLog opens an annotation log; its records are the adjudicator's decisions.
func OpenAdjudicationLog(dir string, who task.Annotator) (*Log, []task.Annotation, error) {
	return openLogAt(dir, adjudicationsDir, who)
}

func openLogAt(dir, logDir string, who task.Annotator) (*Log, []task.Annotation, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("open dir %s: %w", dir, err)
	}
	l, prior, err := openLogIn(root, logDir, who)
	if closeErr := root.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close dir %s: %w", dir, closeErr))
	}
	if err != nil {
		return nil, nil, errors.Join(err, closeIfOpen(l))
	}
	return l, prior, nil
}

func openLogIn(root *os.Root, logDir string, who task.Annotator) (*Log, []task.Annotation, error) {
	if err := root.MkdirAll(logDir, dirPerm); err != nil {
		return nil, nil, fmt.Errorf("create %s dir: %w", logDir, err)
	}
	name := filepath.Join(logDir, who.String()+".jsonl")
	f, err := root.OpenFile(name, os.O_RDWR|os.O_APPEND|os.O_CREATE, filePerm)
	if err != nil {
		return nil, nil, fmt.Errorf("open log %s: %w", name, err)
	}
	l := &Log{file: f, annotator: who, now: time.Now}
	if err := lockFile(f); err != nil {
		return l, nil, fmt.Errorf("lock log %s: %w", name, err)
	}
	prior, size, err := recoverRecords(f)
	if err != nil {
		return l, nil, fmt.Errorf("recover log %s: %w", name, err)
	}
	l.size = size
	if err := syncDir(root, logDir); err != nil {
		return l, nil, err
	}
	return l, prior, nil
}

func closeIfOpen(l *Log) error {
	if l == nil {
		return nil
	}
	return l.Close()
}

// Append stamps the record with the annotator and the current time, then durably appends it.
func (l *Log) Append(a task.Annotation) error {
	if l.broken != nil {
		return fmt.Errorf("log unusable after failed rollback: %w", l.broken)
	}
	a.Annotator = l.annotator.String()
	a.At = l.now().UTC()
	line, err := json.Marshal(toDoc(a))
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	line = append(line, '\n')
	if _, err := l.file.Write(line); err != nil {
		return l.rollback(fmt.Errorf("write record: %w", err))
	}
	if err := l.file.Sync(); err != nil {
		return l.rollback(fmt.Errorf("sync log: %w", err))
	}
	l.size += int64(len(line))
	return nil
}

// rollback cuts a failed or partial write back to the last good size, so a retry
// cannot glue a new record onto a torn line.
func (l *Log) rollback(cause error) error {
	if err := l.file.Truncate(l.size); err != nil {
		l.broken = err
		return errors.Join(cause, fmt.Errorf("roll back partial write: %w", err))
	}
	return cause
}

// Close releases the file.
func (l *Log) Close() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close log: %w", err)
	}
	return nil
}

// recoverRecords reads the log, drops a torn final line, and returns the records
// with the length of the durable prefix.
func recoverRecords(f *os.File) ([]task.Annotation, int64, error) {
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}
	complete := completeLines(data)
	if len(complete) != len(data) {
		if err := f.Truncate(int64(len(complete))); err != nil {
			return nil, 0, fmt.Errorf("drop torn tail: %w", err)
		}
	}
	records, err := parseRecords(complete)
	return records, int64(len(complete)), err
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
		Criterion: d.Criterion, Label: d.Label, Note: d.Note, At: d.At,
	}, nil
}

func toDoc(a task.Annotation) recordDoc {
	return recordDoc{
		Item: a.ItemID, Annotator: a.Annotator, Action: a.Action.String(),
		Criterion: a.Criterion, Label: a.Label, Note: a.Note, At: a.At,
	}
}

func syncDir(root *os.Root, logDir string) error {
	d, err := root.Open(logDir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", logDir, err)
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return fmt.Errorf("sync dir %s: %w", logDir, syncErr)
	}
	return closeErr
}
