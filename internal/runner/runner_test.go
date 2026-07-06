package runner

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTargetListUnmarshal(t *testing.T) {
	tests := []struct {
		in   string
		want TargetList
	}{
		{`["img"]`, TargetList{"img"}},
		{`["iframe", "img"]`, TargetList{"iframe", "img"}},
		// Shadow-DOM targets nest arrays; hatchet flattens them.
		{`[["#host", "img"]]`, TargetList{"#host", "img"}},
		{`[]`, TargetList{}},
	}
	for _, tt := range tests {
		var got TargetList
		if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
			t.Errorf("unmarshal %s: %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("unmarshal %s = %v, want %v", tt.in, got, tt.want)
		}
	}
}
