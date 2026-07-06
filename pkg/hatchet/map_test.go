package hatchet

import (
	"testing"

	"github.com/catdevman/hatchet/internal/runner"
)

func TestIssuesFromAxe(t *testing.T) {
	res := &runner.Results{
		Violations: []runner.Rule{{
			ID:      "image-alt",
			Impact:  "critical",
			Help:    "Images must have alternative text",
			HelpURL: "https://dequeuniversity.com/rules/axe/4.10/image-alt",
			Tags:    []string{"wcag2a"},
			Nodes: []runner.Node{
				{HTML: `<img src="a.png">`, Target: runner.TargetList{"img:nth-child(1)"}},
				{HTML: `<img src="b.png">`, Target: runner.TargetList{"img:nth-child(2)"}, Impact: "serious"},
			},
		}},
		Incomplete: []runner.Rule{{
			ID:      "color-contrast",
			Help:    "Elements must meet minimum color contrast ratio thresholds",
			HelpURL: "https://example.com/color-contrast",
			Nodes:   []runner.Node{{HTML: `<p>hi</p>`, Target: runner.TargetList{"p"}}},
		}},
	}

	issues := issuesFromAxe(res)
	if len(issues) != 3 {
		t.Fatalf("got %d issues, want 3", len(issues))
	}

	first := issues[0]
	if first.Code != "image-alt" || first.Type != TypeError || first.TypeCode != 1 {
		t.Errorf("violation mapped wrong: %+v", first)
	}
	if first.Message != "Images must have alternative text (https://dequeuniversity.com/rules/axe/4.10/image-alt)" {
		t.Errorf("message = %q", first.Message)
	}
	if first.Selector != "img:nth-child(1)" || first.Runner != "axe" {
		t.Errorf("selector/runner mapped wrong: %+v", first)
	}
	if first.RunnerExtras["impact"] != "critical" {
		t.Errorf("node without impact should inherit rule impact, got %v", first.RunnerExtras["impact"])
	}
	if issues[1].RunnerExtras["impact"] != "serious" {
		t.Errorf("node impact should win over rule impact, got %v", issues[1].RunnerExtras["impact"])
	}

	warn := issues[2]
	if warn.Code != "color-contrast" || warn.Type != TypeWarning || warn.TypeCode != 2 {
		t.Errorf("incomplete mapped wrong: %+v", warn)
	}
}
