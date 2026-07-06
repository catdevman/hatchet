package reporter

import (
	"fmt"
	"io"
	"strings"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

// CLI writes a human-readable report, one section per target.
type CLI struct {
	// Color enables ANSI colors; the caller decides based on TTY/NO_COLOR.
	Color bool
}

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
	ansiDim    = "\x1b[2m"
	ansiBold   = "\x1b[1m"
)

func (c CLI) paint(color, s string) string {
	if !c.Color {
		return s
	}
	return color + s + ansiReset
}

func (c CLI) Report(w io.Writer, results []hatchet.Result) error {
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\n", c.paint(ansiBold, "Results for "+r.Target+":"))

		if r.Err != nil {
			fmt.Fprintf(w, "\n %s %v\n", c.paint(ansiRed, "✖ Failed:"), r.Err)
			continue
		}
		if len(r.Issues) == 0 {
			fmt.Fprintf(w, "\n No issues found.\n")
			continue
		}

		for _, is := range r.Issues {
			label := is.Type
			switch is.Type {
			case hatchet.TypeError:
				label = c.paint(ansiRed, "Error")
			case hatchet.TypeWarning:
				label = c.paint(ansiYellow, "Warning")
			case hatchet.TypeNotice:
				label = c.paint(ansiCyan, "Notice")
			}
			fmt.Fprintf(w, "\n • %s: %s\n", label, is.Message)
			fmt.Fprintf(w, "   %s\n", c.paint(ansiDim, "├── "+is.Code))
			fmt.Fprintf(w, "   %s\n", c.paint(ansiDim, "├── "+is.Selector))
			fmt.Fprintf(w, "   %s\n", c.paint(ansiDim, "└── "+condense(is.Context)))
		}
	}

	t := tally(results)
	fmt.Fprintf(w, "\n%s\n", c.paint(ansiBold, fmt.Sprintf(
		"%d errors, %d warnings, %d notices", t.Errors, t.Warnings, t.Notices)))
	return nil
}

// condense collapses whitespace runs and truncates long HTML context so tree
// lines stay single-line.
func condense(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 200
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
