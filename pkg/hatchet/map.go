package hatchet

import (
	"fmt"
	"strings"

	"github.com/catdevman/hatchet/internal/runner"
)

// issuesFromAxe maps raw axe output to Issues the way pa11y's axe runner
// does: violations become errors, incomplete checks become warnings. axe has
// no notice-level output.
func issuesFromAxe(res *runner.Results) []Issue {
	var issues []Issue
	issues = append(issues, ruleIssues(res.Violations, TypeError, 1)...)
	issues = append(issues, ruleIssues(res.Incomplete, TypeWarning, 2)...)
	return issues
}

func ruleIssues(rules []runner.Rule, typ string, typeCode int) []Issue {
	var issues []Issue
	for _, rule := range rules {
		for _, node := range rule.Nodes {
			impact := node.Impact
			if impact == "" {
				impact = rule.Impact
			}
			issues = append(issues, Issue{
				Code:     rule.ID,
				Type:     typ,
				TypeCode: typeCode,
				Message:  fmt.Sprintf("%s (%s)", rule.Help, rule.HelpURL),
				Context:  node.HTML,
				Selector: strings.Join(node.Target, " "),
				Runner:   "axe",
				RunnerExtras: map[string]any{
					"description": rule.Description,
					"impact":      impact,
					"helpUrl":     rule.HelpURL,
					"tags":        rule.Tags,
				},
			})
		}
	}
	return issues
}
