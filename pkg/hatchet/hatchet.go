// Package hatchet checks web pages for accessibility issues by running the
// embedded axe-core rule engine in a headless Chrome browser.
//
// The hatchet CLI is a thin wrapper around this package; everything it can do
// is reachable from here, including from tests against httptest servers. The
// API is not stable before v1.0.
package hatchet

import (
	"time"

	"github.com/catdevman/hatchet/third_party/axe"
)

// Issue types, ordered by severity. TypeCode follows pa11y's convention.
const (
	TypeError   = "error"   // TypeCode 1
	TypeWarning = "warning" // TypeCode 2
	TypeNotice  = "notice"  // TypeCode 3
)

// Issue is one accessibility finding on a page.
type Issue struct {
	Code         string         `json:"code"`     // axe rule id, e.g. "color-contrast"
	Type         string         `json:"type"`     // "error" | "warning" | "notice"
	TypeCode     int            `json:"typeCode"` // 1 | 2 | 3
	Message      string         `json:"message"`
	Context      string         `json:"context"`  // outerHTML snippet of the node
	Selector     string         `json:"selector"` // CSS selector to the node
	Runner       string         `json:"runner"`   // always "axe" for now
	RunnerExtras map[string]any `json:"runnerExtras,omitempty"`
}

// Result holds the outcome for one target. A per-target failure is recorded
// in Err and does not abort a multi-target run.
type Result struct {
	Target string
	Issues []Issue
	Err    error
}

// Cookie is set on the target's URL before navigation.
type Cookie struct {
	Name  string
	Value string
}

// Options configure a Run. The zero value checks against WCAG2AA with a 30s
// per-target timeout using a discovered system browser, reporting errors only
// (pa11y behavior: warnings and notices are opt-in).
type Options struct {
	// Standard is WCAG2A, WCAG2AA (default), WCAG22AA, or WCAG2AAA.
	Standard string
	// Timeout bounds each target's whole check (load + actions + axe).
	// Default 30s.
	Timeout time.Duration
	// Wait is extra settling time after page load before running axe.
	Wait time.Duration

	// IncludeWarnings keeps warning-level issues (axe "incomplete" checks).
	IncludeWarnings bool
	// IncludeNotices keeps notice-level issues. The axe runner produces no
	// notices, so this is accepted for pa11y compatibility but has no effect.
	IncludeNotices bool
	// Ignore drops issues by rule code or issue type, case-insensitively.
	Ignore []string

	// Renderer is "chrome" (default: pages run their JavaScript) or
	// "static": the page's own scripts never execute, so the served markup
	// is checked as-is — deterministic, immune to client-side rendering.
	// Actions are unavailable in static mode.
	Renderer string
	// AxePath loads an alternate axe-core build (e.g. a locale build)
	// instead of the embedded one.
	AxePath string

	// RootElement restricts checking to the subtree of this CSS selector.
	RootElement string
	// HideElements excludes elements matching this comma-separated CSS
	// selector list from checking.
	HideElements string

	// Actions are pa11y action strings run after load, before axe.
	Actions []string

	// ViewportWidth/Height set the emulated viewport. Default 1280x1024
	// (pa11y's default).
	ViewportWidth  int
	ViewportHeight int
	// UserAgent overrides the browser's user agent.
	UserAgent string
	// Headers are extra HTTP headers sent with every request.
	Headers map[string]string
	// Cookies are set on the target URL before navigation.
	Cookies []Cookie
	// BasicAuth is "user:pass", sent as an Authorization header.
	BasicAuth string

	// ScreenCapture writes a full-page PNG of each target after checking.
	// With multiple targets, later captures overwrite earlier ones.
	ScreenCapture string

	// Concurrency is how many targets are checked in parallel, each in its
	// own tab of the shared browser. Values below 1 mean sequential.
	Concurrency int

	// ChromePath is an explicit browser binary; empty means discover one.
	ChromePath string
	// BrowserWSEndpoint connects to a running browser over CDP instead of
	// launching one.
	BrowserWSEndpoint string
	// NoSandbox disables Chrome's sandbox (needed in most containers).
	NoSandbox bool
	// Logf receives debug output; nil means silent.
	Logf func(format string, args ...any)
}

// AxeVersion reports the embedded axe-core version.
func AxeVersion() string {
	return axe.Version()
}
