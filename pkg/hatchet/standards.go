package hatchet

import (
	"fmt"
	"strings"
)

// standardTags maps a --standard value to the axe rule tags to run (HLD §11).
// Each level includes the tags of the levels below it.
func standardTags(standard string) ([]string, error) {
	a := []string{"wcag2a", "wcag21a"}
	aa := append(a, "wcag2aa", "wcag21aa")

	switch strings.ToUpper(standard) {
	case "", "WCAG2AA":
		return aa, nil
	case "WCAG2A":
		return a, nil
	case "WCAG22AA":
		return append(aa, "wcag22aa"), nil
	case "WCAG2AAA":
		return append(aa, "wcag2aaa"), nil
	default:
		return nil, fmt.Errorf("unknown standard %q (expected WCAG2A, WCAG2AA, WCAG22AA, or WCAG2AAA)", standard)
	}
}
