package reporter

import (
	"encoding/json"
	"io"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

// JSON writes the machine-readable report. The schema is versioned so
// scripts consuming it survive upgrades (HLD §8).
type JSON struct {
	HatchetVersion string
	AxeVersion     string
}

const schemaVersion = 1

type jsonDoc struct {
	SchemaVersion  int          `json:"schemaVersion"`
	HatchetVersion string       `json:"hatchetVersion"`
	AxeVersion     string       `json:"axeVersion"`
	Results        []jsonResult `json:"results"`
	Total          counts       `json:"total"`
}

type jsonResult struct {
	Target string          `json:"target"`
	Issues []hatchet.Issue `json:"issues"`
	Error  *string         `json:"error,omitempty"`
}

func (j JSON) Report(w io.Writer, results []hatchet.Result) error {
	doc := jsonDoc{
		SchemaVersion:  schemaVersion,
		HatchetVersion: j.HatchetVersion,
		AxeVersion:     j.AxeVersion,
		Results:        make([]jsonResult, 0, len(results)),
		Total:          tally(results),
	}
	for _, r := range results {
		jr := jsonResult{Target: r.Target, Issues: r.Issues}
		if jr.Issues == nil {
			jr.Issues = []hatchet.Issue{}
		}
		if r.Err != nil {
			msg := r.Err.Error()
			jr.Error = &msg
		}
		doc.Results = append(doc.Results, jr)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
