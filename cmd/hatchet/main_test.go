package main

import (
	"errors"
	"testing"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

func TestLevelTypeCode(t *testing.T) {
	tests := []struct {
		level   string
		want    int
		wantErr bool
	}{
		{"error", 1, false},
		{"warning", 2, false},
		{"notice", 3, false},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		got, err := levelTypeCode(tt.level)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("levelTypeCode(%q) = %d, %v; want %d, wantErr %v",
				tt.level, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestExitCode(t *testing.T) {
	errIssue := hatchet.Issue{Type: hatchet.TypeError, TypeCode: 1}
	warnIssue := hatchet.Issue{Type: hatchet.TypeWarning, TypeCode: 2}

	tests := []struct {
		name       string
		results    []hatchet.Result
		levelCode  int
		thresholds []int
		want       int
	}{
		{"clean", []hatchet.Result{{Target: "a"}}, 1, []int{0}, exitClean},
		{"errors fail", []hatchet.Result{{Issues: []hatchet.Issue{errIssue}}}, 1, []int{0}, exitIssues},
		{"warnings ignored at error level", []hatchet.Result{{Issues: []hatchet.Issue{warnIssue}}}, 1, []int{0}, exitClean},
		{"warnings counted at warning level", []hatchet.Result{{Issues: []hatchet.Issue{warnIssue}}}, 2, []int{0}, exitIssues},
		{"threshold tolerates", []hatchet.Result{{Issues: []hatchet.Issue{errIssue, errIssue}}}, 1, []int{2}, exitClean},
		{"threshold exceeded", []hatchet.Result{{Issues: []hatchet.Issue{errIssue, errIssue, errIssue}}}, 1, []int{2}, exitIssues},
		{"target error is operational", []hatchet.Result{{Err: errors.New("boom")}}, 1, []int{0}, exitOperational},
		{"operational beats issues", []hatchet.Result{{Err: errors.New("boom")}, {Issues: []hatchet.Issue{errIssue}}}, 1, []int{0, 0}, exitOperational},
		{"per-target thresholds", []hatchet.Result{
			{Issues: []hatchet.Issue{errIssue, errIssue}}, // tolerated by its threshold of 2
			{Issues: []hatchet.Issue{errIssue}},           // fails its threshold of 0
		}, 1, []int{2, 0}, exitIssues},
	}
	for _, tt := range tests {
		if got := exitCode(tt.results, tt.levelCode, tt.thresholds); got != tt.want {
			t.Errorf("%s: exitCode = %d, want %d", tt.name, got, tt.want)
		}
	}
}
