// Package config loads hatchet's JSON config file (HLD §12). Field names
// mirror pa11y/pa11y-ci so existing configs mostly port over: options can sit
// at the top level (pa11y style), under "defaults" (pa11y-ci style), or on
// individual "urls" entries. CLI flags win over defaults; per-URL entries win
// over everything for that URL. Pointer fields let callers tell "unset" from
// zero values.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings are the per-run options a config file can carry.
type Settings struct {
	Standard          string            `json:"standard"`
	Timeout           *int              `json:"timeout"` // milliseconds
	Wait              *int              `json:"wait"`    // milliseconds
	Threshold         *int              `json:"threshold"`
	Level             string            `json:"level"`
	Reporters         []string          `json:"reporters"`
	Ignore            []string          `json:"ignore"`
	IncludeNotices    *bool             `json:"includeNotices"`
	IncludeWarnings   *bool             `json:"includeWarnings"`
	RootElement       string            `json:"rootElement"`
	HideElements      string            `json:"hideElements"`
	Actions           []string          `json:"actions"`
	Renderer          string            `json:"renderer"`
	AxePath           string            `json:"axePath"`
	Viewport          *Viewport         `json:"viewport"`
	UserAgent         string            `json:"userAgent"`
	Headers           map[string]string `json:"headers"`
	Concurrency       *int              `json:"concurrency"`
	ChromePath        string            `json:"chromePath"`
	BrowserWSEndpoint string            `json:"browserWSEndpoint"`
	NoSandbox         *bool             `json:"noSandbox"`
}

type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// URLEntry is one entry of the "urls" array: either a bare URL string or an
// object with a "url" plus setting overrides.
type URLEntry struct {
	URL string `json:"url"`
	Settings
}

func (e *URLEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.URL = s
		return nil
	}
	type entry URLEntry // shed the method to avoid recursion
	var v entry
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.URL == "" {
		return fmt.Errorf("urls entry is missing \"url\"")
	}
	*e = URLEntry(v)
	return nil
}

type File struct {
	Settings
	Defaults *Settings  `json:"defaults"`
	URLs     []URLEntry `json:"urls"`
}

// EffectiveDefaults folds "defaults" over any top-level settings.
func (f *File) EffectiveDefaults() Settings {
	if f.Defaults == nil {
		return f.Settings
	}
	return Merge(f.Settings, *f.Defaults)
}

// Merge lays over's set fields on top of base.
func Merge(base, over Settings) Settings {
	out := base
	if over.Standard != "" {
		out.Standard = over.Standard
	}
	if over.Timeout != nil {
		out.Timeout = over.Timeout
	}
	if over.Wait != nil {
		out.Wait = over.Wait
	}
	if over.Threshold != nil {
		out.Threshold = over.Threshold
	}
	if over.Level != "" {
		out.Level = over.Level
	}
	if len(over.Reporters) > 0 {
		out.Reporters = over.Reporters
	}
	if len(over.Ignore) > 0 {
		out.Ignore = over.Ignore
	}
	if over.IncludeNotices != nil {
		out.IncludeNotices = over.IncludeNotices
	}
	if over.IncludeWarnings != nil {
		out.IncludeWarnings = over.IncludeWarnings
	}
	if over.RootElement != "" {
		out.RootElement = over.RootElement
	}
	if over.HideElements != "" {
		out.HideElements = over.HideElements
	}
	if len(over.Actions) > 0 {
		out.Actions = over.Actions
	}
	if over.Renderer != "" {
		out.Renderer = over.Renderer
	}
	if over.AxePath != "" {
		out.AxePath = over.AxePath
	}
	if over.Viewport != nil {
		out.Viewport = over.Viewport
	}
	if over.UserAgent != "" {
		out.UserAgent = over.UserAgent
	}
	if len(over.Headers) > 0 {
		out.Headers = over.Headers
	}
	if over.Concurrency != nil {
		out.Concurrency = over.Concurrency
	}
	if over.ChromePath != "" {
		out.ChromePath = over.ChromePath
	}
	if over.BrowserWSEndpoint != "" {
		out.BrowserWSEndpoint = over.BrowserWSEndpoint
	}
	if over.NoSandbox != nil {
		out.NoSandbox = over.NoSandbox
	}
	return out
}

// discoveryNames are checked in order in the working directory when no
// --config is given.
var discoveryNames = []string{".hatchetrc", "hatchet.json"}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &f, nil
}

// Discover returns the config in dir, an empty File when none exists, or an
// error when one exists but is invalid.
func Discover(dir string) (*File, error) {
	for _, name := range discoveryNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return &File{}, nil
}
