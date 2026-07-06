package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

func sampleResults() []hatchet.Result {
	return []hatchet.Result{
		{
			Target: "https://example.com",
			Issues: []hatchet.Issue{
				{
					Code: "image-alt", Type: hatchet.TypeError, TypeCode: 1,
					Message:  "Images must have alternative text (https://example.com/help)",
					Context:  "<img\n   src=\"a.png\">",
					Selector: "img", Runner: "axe",
				},
				{
					Code: "color-contrast", Type: hatchet.TypeWarning, TypeCode: 2,
					Message: "Contrast", Context: "<p>x</p>", Selector: "p", Runner: "axe",
				},
			},
		},
		{Target: "https://broken.example.com", Err: errors.New("loading failed")},
	}
}

func TestCLIReporter(t *testing.T) {
	var buf bytes.Buffer
	if err := (CLI{}).Report(&buf, sampleResults()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"Results for https://example.com:",
		"Error: Images must have alternative text",
		"image-alt",
		`<img src="a.png">`, // whitespace condensed
		"Warning: Contrast",
		"Results for https://broken.example.com:",
		"loading failed",
		"1 errors, 1 warnings, 0 notices",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cli output missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("cli output has ANSI codes with Color disabled")
	}
}

func TestCLIReporterNoIssues(t *testing.T) {
	var buf bytes.Buffer
	results := []hatchet.Result{{Target: "https://clean.example.com"}}
	if err := (CLI{}).Report(&buf, results); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No issues found.") {
		t.Errorf("expected clean message, got:\n%s", buf.String())
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	j := JSON{HatchetVersion: "test", AxeVersion: "4.10.3"}
	if err := j.Report(&buf, sampleResults()); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		SchemaVersion  int    `json:"schemaVersion"`
		HatchetVersion string `json:"hatchetVersion"`
		AxeVersion     string `json:"axeVersion"`
		Results        []struct {
			Target string          `json:"target"`
			Issues []hatchet.Issue `json:"issues"`
			Error  *string         `json:"error"`
		} `json:"results"`
		Total struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
			Notices  int `json:"notices"`
		} `json:"total"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if doc.SchemaVersion != 1 || doc.HatchetVersion != "test" || doc.AxeVersion != "4.10.3" {
		t.Errorf("header wrong: %+v", doc)
	}
	if len(doc.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(doc.Results))
	}
	if doc.Results[0].Issues[0].Code != "image-alt" {
		t.Errorf("first issue = %+v", doc.Results[0].Issues[0])
	}
	if doc.Results[1].Error == nil || *doc.Results[1].Error != "loading failed" {
		t.Errorf("per-target error not rendered: %+v", doc.Results[1])
	}
	if doc.Results[1].Issues == nil {
		t.Error("issues should be [] not null for failed targets")
	}
	if doc.Total.Errors != 1 || doc.Total.Warnings != 1 {
		t.Errorf("totals wrong: %+v", doc.Total)
	}
}
