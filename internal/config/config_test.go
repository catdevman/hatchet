package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hatchet.json")
	body := `{
		"standard": "WCAG22AA",
		"timeout": 5000,
		"threshold": 3,
		"reporters": ["json"],
		"noSandbox": true
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Standard != "WCAG22AA" {
		t.Errorf("Standard = %q", f.Standard)
	}
	if f.Timeout == nil || *f.Timeout != 5000 {
		t.Errorf("Timeout = %v", f.Timeout)
	}
	if f.Wait != nil {
		t.Errorf("unset Wait should be nil, got %v", *f.Wait)
	}
	if f.Threshold == nil || *f.Threshold != 3 {
		t.Errorf("Threshold = %v", f.Threshold)
	}
	if len(f.Reporters) != 1 || f.Reporters[0] != "json" {
		t.Errorf("Reporters = %v", f.Reporters)
	}
	if f.NoSandbox == nil || !*f.NoSandbox {
		t.Errorf("NoSandbox = %v", f.NoSandbox)
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadPa11yCIShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hatchet.json")
	body := `{
		"defaults": {"standard": "WCAG22AA", "timeout": 9000},
		"urls": [
			"https://example.com/",
			{"url": "https://example.com/slow", "timeout": 60000, "threshold": 5,
			 "actions": ["click element #go"]}
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	def := f.EffectiveDefaults()
	if def.Standard != "WCAG22AA" || def.Timeout == nil || *def.Timeout != 9000 {
		t.Errorf("defaults = %+v", def)
	}
	if len(f.URLs) != 2 {
		t.Fatalf("got %d urls, want 2", len(f.URLs))
	}
	if f.URLs[0].URL != "https://example.com/" || f.URLs[0].Timeout != nil {
		t.Errorf("bare url entry = %+v", f.URLs[0])
	}
	e := f.URLs[1]
	if e.URL != "https://example.com/slow" || *e.Timeout != 60000 || *e.Threshold != 5 || len(e.Actions) != 1 {
		t.Errorf("override entry = %+v", e)
	}
}

func TestMerge(t *testing.T) {
	nine, five := 9000, 5
	base := Settings{Standard: "WCAG2AA", Timeout: &nine, Level: "error"}
	over := Settings{Standard: "WCAG22AA", Threshold: &five}

	m := Merge(base, over)
	if m.Standard != "WCAG22AA" {
		t.Errorf("over should win: %q", m.Standard)
	}
	if m.Timeout == nil || *m.Timeout != 9000 {
		t.Errorf("unset over field should keep base: %v", m.Timeout)
	}
	if m.Threshold == nil || *m.Threshold != 5 {
		t.Errorf("over-only field lost: %v", m.Threshold)
	}
	if m.Level != "error" {
		t.Errorf("base-only field lost: %q", m.Level)
	}
}

func TestURLEntryInvalid(t *testing.T) {
	var f File
	if err := json.Unmarshal([]byte(`{"urls":[{"timeout": 5}]}`), &f); err == nil {
		t.Error("urls object without url should fail")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	f, err := Discover(dir)
	if err != nil || f == nil {
		t.Fatalf("empty dir should give empty config, got %v, %v", f, err)
	}
	if f.Standard != "" {
		t.Errorf("empty config has Standard %q", f.Standard)
	}

	if err := os.WriteFile(filepath.Join(dir, ".hatchetrc"), []byte(`{"standard":"WCAG2A"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err = Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Standard != "WCAG2A" {
		t.Errorf("discovered Standard = %q, want WCAG2A", f.Standard)
	}
}
