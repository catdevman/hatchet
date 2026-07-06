package reporter

import (
	"encoding/csv"
	"io"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

// CSV writes one row per issue. pa11y's columns plus a leading target column
// for multi-target runs. Failed targets contribute no rows.
type CSV struct{}

func (CSV) Report(w io.Writer, results []hatchet.Result) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"target", "type", "code", "message", "context", "selector"}); err != nil {
		return err
	}
	for _, r := range results {
		for _, is := range r.Issues {
			if err := cw.Write([]string{r.Target, is.Type, is.Code, is.Message, is.Context, is.Selector}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
