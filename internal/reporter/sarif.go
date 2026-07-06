package reporter

import (
	"encoding/json"
	"io"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

// SARIF writes SARIF 2.1.0 for ingestion by GitHub code scanning and similar
// tools. One run, rules deduped by axe code, one result per issue.
type SARIF struct {
	HatchetVersion string
}

type sarifDoc struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string        `json:"id"`
	ShortDescription sarifText     `json:"shortDescription"`
	HelpURI          string        `json:"helpUri,omitempty"`
	Properties       map[string]any `json:"properties,omitempty"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	RuleIndex int             `json:"ruleIndex"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical  `json:"physicalLocation"`
	LogicalLocations []sarifLogical `json:"logicalLocations,omitempty"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifLogical struct {
	FullyQualifiedName string `json:"fullyQualifiedName"`
}

func sarifLevel(issueType string) string {
	switch issueType {
	case hatchet.TypeError:
		return "error"
	case hatchet.TypeWarning:
		return "warning"
	default:
		return "note"
	}
}

func (s SARIF) Report(w io.Writer, results []hatchet.Result) error {
	run := sarifRun{
		Tool: sarifTool{Driver: sarifDriver{
			Name:           "hatchet",
			Version:        s.HatchetVersion,
			InformationURI: "https://github.com/catdevman/hatchet",
			Rules:          []sarifRule{},
		}},
		Results: []sarifResult{},
	}

	ruleIndex := map[string]int{}
	for _, r := range results {
		for _, is := range r.Issues {
			idx, seen := ruleIndex[is.Code]
			if !seen {
				idx = len(run.Tool.Driver.Rules)
				ruleIndex[is.Code] = idx
				rule := sarifRule{ID: is.Code, ShortDescription: sarifText{Text: is.Message}}
				if help, ok := is.RunnerExtras["helpUrl"].(string); ok {
					rule.HelpURI = help
				}
				if desc, ok := is.RunnerExtras["description"].(string); ok {
					rule.ShortDescription = sarifText{Text: desc}
				}
				run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, rule)
			}
			run.Results = append(run.Results, sarifResult{
				RuleID:    is.Code,
				RuleIndex: idx,
				Level:     sarifLevel(is.Type),
				Message:   sarifText{Text: is.Message + "\n\nSelector: " + is.Selector},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysical{ArtifactLocation: sarifArtifact{URI: r.Target}},
					LogicalLocations: []sarifLogical{{FullyQualifiedName: is.Selector}},
				}},
			})
		}
	}

	doc := sarifDoc{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{run},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
