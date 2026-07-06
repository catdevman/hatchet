package hatchet

import (
	"reflect"
	"testing"
)

func TestStandardTags(t *testing.T) {
	aa := []string{"wcag2a", "wcag21a", "wcag2aa", "wcag21aa"}
	tests := []struct {
		standard string
		want     []string
		wantErr  bool
	}{
		{standard: "", want: aa},
		{standard: "WCAG2AA", want: aa},
		{standard: "wcag2aa", want: aa},
		{standard: "WCAG2A", want: []string{"wcag2a", "wcag21a"}},
		{standard: "WCAG22AA", want: append(append([]string{}, aa...), "wcag22aa")},
		{standard: "WCAG2AAA", want: append(append([]string{}, aa...), "wcag2aaa")},
		{standard: "Section508", wantErr: true},
	}
	for _, tt := range tests {
		got, err := standardTags(tt.standard)
		if (err != nil) != tt.wantErr {
			t.Errorf("standardTags(%q) error = %v, wantErr %v", tt.standard, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
			t.Errorf("standardTags(%q) = %v, want %v", tt.standard, got, tt.want)
		}
	}
}
