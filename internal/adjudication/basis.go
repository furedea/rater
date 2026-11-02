package adjudication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"github.com/furedea/rater/internal/task"
)

// basis digests every annotator's current state of an item: names, statuses,
// labels or answers, and notes, which is what the adjudication screen shows.
// Log lengths are left out, so records about other items do not change it.
func (d Docket) basis(itemID string) string {
	var text []byte
	for _, j := range d.Judgments(itemID) {
		s := j.State
		text = fmt.Appendf(text, "%q %d %q %q", j.Annotator, s.Status, s.Label, s.Note)
		for _, criterion := range slices.Sorted(maps.Keys(s.Answers)) {
			text = fmt.Appendf(text, " %q=%q", criterion, s.Answers[criterion])
		}
		text = append(text, '\n')
	}
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])
}

// basisRecorder stamps each decision with the basis of its item before
// handing it to the log.
type basisRecorder struct {
	rec    task.Recorder
	docket Docket
}

func (r basisRecorder) Append(a task.Annotation) error {
	a.Basis = r.docket.basis(a.ItemID)
	return r.rec.Append(a)
}
