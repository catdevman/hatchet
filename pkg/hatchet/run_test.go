package hatchet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTarget(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		in   string
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"http://example.com/x", "http://example.com/x"},
		{"file:///tmp/x.html", "file:///tmp/x.html"},
		{page, "file://" + page},
		{"example.com", "http://example.com"},
		{"localhost:8080/path", "http://localhost:8080/path"},
	}
	for _, tt := range tests {
		got, err := resolveTarget(tt.in)
		if err != nil {
			t.Errorf("resolveTarget(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("resolveTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
