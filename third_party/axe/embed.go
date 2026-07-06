// Package axe vendors the axe-core accessibility rule engine.
//
// axe.min.js is the unmodified upstream build pinned in VERSION and is
// licensed under MPL-2.0 (see LICENSE in this directory). Version bumps are
// deliberate: update axe.min.js, VERSION, and LICENSE together.
package axe

import (
	_ "embed"
	"strings"
)

//go:embed axe.min.js
var Source string

//go:embed VERSION
var version string

// Version returns the pinned axe-core version.
func Version() string {
	return strings.TrimSpace(version)
}
