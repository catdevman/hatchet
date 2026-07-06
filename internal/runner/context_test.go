package runner

import "testing"

func TestAxeContext(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{"default", Options{}, "document"},
		{"root only", Options{RootElement: "main"}, `{"include":[["main"]]}`},
		{"hide only", Options{HideElements: []string{"#ad", ".banner"}},
			`{"exclude":[["#ad"],[".banner"]]}`},
		{"both", Options{RootElement: "main", HideElements: []string{"#ad"}},
			`{"exclude":[["#ad"]],"include":[["main"]]}`},
	}
	for _, tt := range tests {
		got, err := axeContext(tt.opts)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: axeContext = %s, want %s", tt.name, got, tt.want)
		}
	}
}
