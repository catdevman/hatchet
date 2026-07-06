package hatchet

import "testing"

func TestFilterIssues(t *testing.T) {
	issues := []Issue{
		{Code: "image-alt", Type: TypeError, TypeCode: 1},
		{Code: "label", Type: TypeError, TypeCode: 1},
		{Code: "color-contrast", Type: TypeWarning, TypeCode: 2},
		{Code: "some-notice", Type: TypeNotice, TypeCode: 3},
	}

	codes := func(is []Issue) []string {
		var out []string
		for _, i := range is {
			out = append(out, i.Code)
		}
		return out
	}

	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{"default drops warnings and notices", Options{}, []string{"image-alt", "label"}},
		{"include warnings", Options{IncludeWarnings: true}, []string{"image-alt", "label", "color-contrast"}},
		{"include everything", Options{IncludeWarnings: true, IncludeNotices: true},
			[]string{"image-alt", "label", "color-contrast", "some-notice"}},
		{"ignore by code", Options{Ignore: []string{"image-alt"}}, []string{"label"}},
		{"ignore is case-insensitive", Options{Ignore: []string{"IMAGE-ALT"}}, []string{"label"}},
		{"ignore by type", Options{IncludeWarnings: true, Ignore: []string{"error"}}, []string{"color-contrast"}},
	}
	for _, tt := range tests {
		got := codes(filterIssues(issues, tt.opts))
		if len(got) != len(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}
