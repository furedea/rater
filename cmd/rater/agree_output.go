package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/furedea/rater/internal/agreement"
)

func writeReport(w io.Writer, f reportFormat, r agreement.Report) error {
	switch f {
	case formatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(newReportDoc(r)); err != nil {
			return fmt.Errorf("write json report: %w", err)
		}
		return nil
	case formatText:
		return writeText(w, r)
	}
	return fmt.Errorf("unknown format %d", f)
}

// reportDoc is the JSON form of a report. An undefined coefficient has a
// null value and names why in "undefined".
type reportDoc struct {
	Task       string        `json:"task"`
	Items      int           `json:"items"`
	Annotators []coverageDoc `json:"annotators"`
	Criteria   []measureDoc  `json:"criteria,omitempty"`
	Overall    measureDoc    `json:"overall"`
}

type coverageDoc struct {
	Name    string `json:"name"`
	Labeled int    `json:"labeled"`
	Held    int    `json:"held"`
	Pending int    `json:"pending"`
}

type measureDoc struct {
	Name      string        `json:"name"`
	Alpha     alphaDoc      `json:"alpha"`
	Kappa     *kappaDoc     `json:"kappa,omitempty"`
	Confusion *confusionDoc `json:"confusion,omitempty"`
}

type coefficientDoc struct {
	Value     *float64 `json:"value"`
	Undefined string   `json:"undefined,omitempty"`
}

type alphaDoc struct {
	coefficientDoc
	Items  int `json:"items"`
	Values int `json:"values"`
}

type kappaDoc struct {
	coefficientDoc
	Observed *float64 `json:"observed,omitempty"`
	Expected *float64 `json:"expected,omitempty"`
	Items    int      `json:"items"`
}

type confusionDoc struct {
	Rows       string   `json:"rows"`
	Columns    string   `json:"columns"`
	Categories []string `json:"categories"`
	Counts     [][]int  `json:"counts"`
}

func newReportDoc(r agreement.Report) reportDoc {
	doc := reportDoc{Task: r.Task, Items: r.Items, Overall: newMeasureDoc(r, r.Overall)}
	for _, c := range r.Annotators {
		doc.Annotators = append(doc.Annotators, coverageDoc(c))
	}
	for _, m := range r.Criteria {
		doc.Criteria = append(doc.Criteria, newMeasureDoc(r, m))
	}
	return doc
}

func newMeasureDoc(r agreement.Report, m agreement.Measure) measureDoc {
	doc := measureDoc{
		Name:  m.Name,
		Alpha: alphaDoc{coefficientDoc: newCoefficientDoc(m.Alpha.Alpha), Items: m.Alpha.Units, Values: m.Alpha.Values},
	}
	if m.Pair == nil {
		return doc
	}
	k := m.Pair.Kappa()
	doc.Kappa = &kappaDoc{coefficientDoc: newCoefficientDoc(k.Kappa), Items: k.Units}
	if k.Units > 0 {
		doc.Kappa.Observed, doc.Kappa.Expected = &k.Observed, &k.Expected
	}
	doc.Confusion = newConfusionDoc(r, *m.Pair)
	return doc
}

func newCoefficientDoc(c agreement.Coefficient) coefficientDoc {
	v, ok := c.Value()
	if !ok {
		return coefficientDoc{Undefined: c.Reason().String()}
	}
	return coefficientDoc{Value: &v}
}

func newConfusionDoc(r agreement.Report, c agreement.Confusion) *confusionDoc {
	categories := c.Categories()
	counts := make([][]int, len(categories))
	for i, row := range categories {
		counts[i] = make([]int, len(categories))
		for j, col := range categories {
			counts[i][j] = c.Count(row, col)
		}
	}
	return &confusionDoc{
		Rows: r.Annotators[0].Name, Columns: r.Annotators[1].Name,
		Categories: categories, Counts: counts,
	}
}
