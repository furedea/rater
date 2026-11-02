package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/furedea/rater/internal/agreement"
)

// writeText writes the report as aligned plain-text tables.
func writeText(w io.Writer, r agreement.Report) error {
	doc := newReportDoc(r)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	out := &rowWriter{w: tw}
	out.row(fmt.Sprintf("task %s · %d items", doc.Task, doc.Items))
	out.row()
	out.row("annotator", "labeled", "held", "pending")
	for _, c := range doc.Annotators {
		out.row(c.Name, strconv.Itoa(c.Labeled), strconv.Itoa(c.Held), strconv.Itoa(c.Pending))
	}
	out.row()
	out.row("measure", "items", "observed", "expected", "kappa", "alpha")
	measures := slices.Concat(doc.Criteria, []measureDoc{doc.Overall})
	for _, m := range measures {
		out.measure(m)
	}
	if c := doc.Overall.Confusion; c != nil {
		out.confusion(doc.Overall.Name, c)
	}
	out.notes(measures)
	if out.err != nil {
		return fmt.Errorf("write text report: %w", out.err)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write text report: %w", err)
	}
	return nil
}

// rowWriter writes tab-separated rows and keeps the first error, so a report
// is written line by line without checking every call.
type rowWriter struct {
	w   io.Writer
	err error
}

func (o *rowWriter) row(cells ...string) {
	if o.err != nil {
		return
	}
	_, o.err = io.WriteString(o.w, strings.Join(cells, "\t")+"\n")
}

func (o *rowWriter) measure(m measureDoc) {
	observed, expected, kappa := "-", "-", "-"
	if k := m.Kappa; k != nil {
		observed, expected, kappa = optionalNumber(k.Observed), optionalNumber(k.Expected), optionalNumber(k.Value)
	}
	o.row(m.Name, strconv.Itoa(m.Alpha.Items), observed, expected, kappa, optionalNumber(m.Alpha.Value))
}

func (o *rowWriter) confusion(name string, c *confusionDoc) {
	o.row()
	o.row(fmt.Sprintf("%s confusion (rows: %s, columns: %s)", name, c.Rows, c.Columns))
	o.row(slices.Concat([]string{""}, c.Categories)...)
	for i, category := range c.Categories {
		cells := []string{category}
		for _, n := range c.Counts[i] {
			cells = append(cells, strconv.Itoa(n))
		}
		o.row(cells...)
	}
}

// notes explains every n/a in the measure table.
func (o *rowWriter) notes(measures []measureDoc) {
	o.row()
	o.row("held and incomplete items are left out of agreement.")
	for _, m := range measures {
		if m.Kappa != nil && m.Kappa.Undefined != "" {
			o.row(fmt.Sprintf("%s kappa is n/a: %s", m.Name, m.Kappa.Undefined))
		}
		if m.Alpha.Undefined != "" {
			o.row(fmt.Sprintf("%s alpha is n/a: %s", m.Name, m.Alpha.Undefined))
		}
	}
}

func optionalNumber(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*v, 'f', 3, 64)
}
