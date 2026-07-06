package hatchet

import "strings"

// filterIssues applies pa11y's post-run filtering: warnings and notices are
// dropped unless opted in, and ignored codes/types are removed.
func filterIssues(issues []Issue, opts Options) []Issue {
	ignore := make(map[string]bool, len(opts.Ignore))
	for _, ig := range opts.Ignore {
		ignore[strings.ToLower(ig)] = true
	}

	out := make([]Issue, 0, len(issues))
	for _, is := range issues {
		if is.Type == TypeWarning && !opts.IncludeWarnings {
			continue
		}
		if is.Type == TypeNotice && !opts.IncludeNotices {
			continue
		}
		if ignore[strings.ToLower(is.Type)] || ignore[strings.ToLower(is.Code)] {
			continue
		}
		out = append(out, is)
	}
	return out
}
