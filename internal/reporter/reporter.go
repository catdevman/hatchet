// Package reporter formats run results for humans and machines (HLD §8).
package reporter

import (
	"io"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

type Reporter interface {
	Report(w io.Writer, results []hatchet.Result) error
}

// counts tallies issues by type across results.
type counts struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Notices  int `json:"notices"`
}

func tally(results []hatchet.Result) counts {
	var c counts
	for _, r := range results {
		for _, is := range r.Issues {
			switch is.Type {
			case hatchet.TypeError:
				c.Errors++
			case hatchet.TypeWarning:
				c.Warnings++
			case hatchet.TypeNotice:
				c.Notices++
			}
		}
	}
	return c
}
