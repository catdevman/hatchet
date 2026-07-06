package actions

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	valid := []string{
		"click element #submit",
		"click element .nav > li:first-child a",
		"set field #name to Jane Doe",
		"check field #terms",
		"uncheck field input[name=optin]",
		"wait for element #modal to be visible",
		"wait for element #spinner to be hidden",
		"wait for element .result to be added",
		"wait for element .toast to be removed",
		"wait for url to be https://example.com/done",
		"wait for path to be /done",
		"wait for path to not be /login",
		"wait for fragment to be #section",
		"navigate to https://example.com/step2",
		"screen capture out.png",
		"  click element #padded  ", // whitespace is trimmed
	}
	acts, err := Parse(valid)
	if err != nil {
		t.Fatalf("Parse(valid) error: %v", err)
	}
	if len(acts) != len(valid) {
		t.Fatalf("got %d actions, want %d", len(acts), len(valid))
	}
	for i, a := range acts {
		if a == nil {
			t.Errorf("action %d (%q) is nil", i, valid[i])
		}
	}
}

func TestParseUnknown(t *testing.T) {
	unknown := []string{
		"click #submit",
		"wait for element #x to be purple",
		"wait for hostname to be example.com",
		"do a barrel roll",
		"",
	}
	for _, s := range unknown {
		if _, err := Parse([]string{s}); err == nil {
			t.Errorf("Parse(%q) should fail", s)
		} else if !strings.Contains(err.Error(), "unknown action") {
			t.Errorf("Parse(%q) error = %v, want 'unknown action'", s, err)
		}
	}
}
