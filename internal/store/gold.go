package store

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/furedea/rater/internal/adjudication"
)

const goldFile = "gold.jsonl"

// goldDoc is one line of gold.jsonl. Exactly one of Label and Answers is
// set; Verdict comes with Answers.
type goldDoc struct {
	ID          string            `json:"id"`
	Label       string            `json:"label,omitempty"`
	Answers     map[string]string `json:"answers,omitempty"`
	Verdict     string            `json:"verdict,omitempty"`
	Source      string            `json:"source"`
	Adjudicator string            `json:"adjudicator,omitempty"`
	Note        string            `json:"note,omitempty"`
	Annotations []judgmentDoc     `json:"annotations"`
}

// judgmentDoc is one compared annotation: the annotator's state and how
// many records of their append-only log it was replayed from.
type judgmentDoc struct {
	Annotator string            `json:"annotator"`
	Status    string            `json:"status"`
	Label     string            `json:"label,omitempty"`
	Answers   map[string]string `json:"answers,omitempty"`
	Note      string            `json:"note,omitempty"`
	Records   int               `json:"records"`
}

// WriteGold replaces <dir>/gold.jsonl with one line per gold answer, in the
// order given. The file is derived from the logs, so it is rewritten whole:
// the lines go to a temporary file that is fsynced and then renamed over the
// old one, and a crash leaves either the old or the new file, never a mix.
// encoding/json writes map keys sorted, so equal input gives equal bytes.
func WriteGold(dir string, golds []adjudication.Gold) error {
	data, err := encodeGold(golds)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open dir %s: %w", dir, err)
	}
	writeErr := replaceFile(root, goldFile, data)
	closeErr := root.Close()
	return errors.Join(writeErr, closeErr)
}

func encodeGold(golds []adjudication.Gold) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, g := range golds {
		if err := enc.Encode(toGoldDoc(g)); err != nil {
			return nil, fmt.Errorf("encode gold %q: %w", g.ItemID, err)
		}
	}
	return buf.Bytes(), nil
}

func toGoldDoc(g adjudication.Gold) goldDoc {
	doc := goldDoc{
		ID: g.ItemID, Label: g.Final.Label, Answers: g.Final.Answers,
		Source: g.Source.String(), Adjudicator: g.Adjudicator, Note: g.Final.Note,
		Annotations: make([]judgmentDoc, 0, len(g.Judgments)),
	}
	if len(g.Final.Answers) > 0 {
		doc.Verdict = g.Verdict.String()
	}
	for _, j := range g.Judgments {
		doc.Annotations = append(doc.Annotations, judgmentDoc{
			Annotator: j.Annotator, Status: j.State.Status.String(),
			Label: j.State.Label, Answers: j.State.Answers, Note: j.State.Note, Records: j.Records,
		})
	}
	return doc
}

// replaceFile atomically replaces name in root with data.
func replaceFile(root *os.Root, name string, data []byte) error {
	tmp := "." + name + "." + rand.Text() + ".tmp"
	if err := writeSynced(root, tmp, data); err != nil {
		return errors.Join(err, removeIfExists(root, tmp))
	}
	if err := root.Rename(tmp, name); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", name, err), removeIfExists(root, tmp))
	}
	return syncDir(root, ".")
}

func writeSynced(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func removeIfExists(root *os.Root, name string) error {
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", name, err)
	}
	return nil
}
