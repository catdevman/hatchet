package reporter

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func TestSARIFReporter(t *testing.T) {
	var buf bytes.Buffer
	if err := (SARIF{HatchetVersion: "test"}).Report(&buf, sampleResults()); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				RuleIndex int    `json:"ruleIndex"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		t.Fatalf("header wrong: version=%s runs=%d", doc.Version, len(doc.Runs))
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Name != "hatchet" {
		t.Errorf("driver name = %q", run.Tool.Driver.Name)
	}
	if len(run.Results) != 2 || len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("got %d results / %d rules, want 2 / 2", len(run.Results), len(run.Tool.Driver.Rules))
	}
	first := run.Results[0]
	if first.RuleID != "image-alt" || first.Level != "error" {
		t.Errorf("first result = %+v", first)
	}
	if run.Tool.Driver.Rules[first.RuleIndex].ID != "image-alt" {
		t.Error("ruleIndex does not point at the matching rule")
	}
	if uri := first.Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "https://example.com" {
		t.Errorf("artifact uri = %q", uri)
	}
	if run.Results[1].Level != "warning" {
		t.Errorf("warning level = %q", run.Results[1].Level)
	}
}

func TestJUnitReporter(t *testing.T) {
	var buf bytes.Buffer
	if err := (JUnit{}).Report(&buf, sampleResults()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	var doc struct {
		XMLName  xml.Name `xml:"testsuites"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Errors   int      `xml:"errors,attr"`
		Suites   []struct {
			Name  string `xml:"name,attr"`
			Cases []struct {
				Name    string `xml:"name,attr"`
				Failure *struct {
					Message string `xml:"message,attr"`
				} `xml:"failure"`
				Error *struct {
					Message string `xml:"message,attr"`
				} `xml:"error"`
			} `xml:"testcase"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, out)
	}

	if !strings.HasPrefix(out, xml.Header) {
		t.Error("missing XML header")
	}
	if doc.Tests != 3 || doc.Failures != 2 || doc.Errors != 1 {
		t.Errorf("totals: tests=%d failures=%d errors=%d", doc.Tests, doc.Failures, doc.Errors)
	}
	if len(doc.Suites) != 2 {
		t.Fatalf("got %d suites, want 2", len(doc.Suites))
	}
	if doc.Suites[0].Cases[0].Failure == nil {
		t.Error("issue case has no <failure>")
	}
	if doc.Suites[1].Cases[0].Error == nil {
		t.Error("failed target case has no <error>")
	}
}

func TestJUnitCleanTarget(t *testing.T) {
	var buf bytes.Buffer
	results := sampleResults()[:1]
	results[0].Issues = nil
	if err := (JUnit{}).Report(&buf, results); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `failures="0"`) {
		t.Errorf("clean target should have zero failures:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "accessibility check") {
		t.Errorf("clean target should have a passing case:\n%s", buf.String())
	}
}
