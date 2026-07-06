// Package baseline implements ratchet mode (HLD §9): a committed file of
// accepted issues so runs fail only on new ones. Matching is fail-safe: an
// issue counts as new unless its fingerprint matches exactly.
package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/catdevman/hatchet/pkg/hatchet"
)

const schemaVersion = 1

type File struct {
	SchemaVersion int     `json:"schemaVersion"`
	AxeVersion    string  `json:"axeVersion"`
	Created       string  `json:"created"`
	Entries       []Entry `json:"entries"`
}

// Entry keeps the fingerprint components readable so humans can review
// baseline diffs; only Fingerprint is used for matching.
type Entry struct {
	Fingerprint string `json:"fingerprint"`
	Target      string `json:"target"`
	Code        string `json:"code"`
	Selector    string `json:"selector"`
}

// Fingerprint identifies an issue across runs by target, rule code,
// selector, and whitespace-normalized context.
func Fingerprint(target string, is hatchet.Issue) string {
	context := strings.Join(strings.Fields(is.Context), " ")
	sum := sha256.Sum256([]byte(target + "\x00" + is.Code + "\x00" + is.Selector + "\x00" + context))
	return hex.EncodeToString(sum[:])
}

// New builds a baseline from current results (failed targets contribute
// nothing). Output is deterministic: entries sorted by fingerprint.
func New(results []hatchet.Result, axeVersion string) *File {
	f := &File{
		SchemaVersion: schemaVersion,
		AxeVersion:    axeVersion,
		Created:       time.Now().UTC().Format(time.RFC3339),
		Entries:       []Entry{},
	}
	seen := map[string]bool{}
	for _, r := range results {
		for _, is := range r.Issues {
			fp := Fingerprint(r.Target, is)
			if seen[fp] {
				continue
			}
			seen[fp] = true
			f.Entries = append(f.Entries, Entry{
				Fingerprint: fp,
				Target:      r.Target,
				Code:        is.Code,
				Selector:    is.Selector,
			})
		}
	}
	sort.Slice(f.Entries, func(i, j int) bool { return f.Entries[i].Fingerprint < f.Entries[j].Fingerprint })
	return f
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing baseline %s: %w", path, err)
	}
	if f.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("baseline %s has schema version %d, this hatchet reads %d", path, f.SchemaVersion, schemaVersion)
	}
	return &f, nil
}

func (f *File) Write(path string) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Stats summarize an Apply.
type Stats struct {
	New       int // issues not in the baseline (kept in results)
	Baselined int // issues matched and filtered out
	Fixed     int // baseline entries that no longer occur
}

// Apply removes baselined issues from results (in place) and reports what
// matched. Failed targets are left untouched.
func (f *File) Apply(results []hatchet.Result) Stats {
	known := make(map[string]bool, len(f.Entries))
	for _, e := range f.Entries {
		known[e.Fingerprint] = true
	}

	var stats Stats
	matched := map[string]bool{}
	for i := range results {
		r := &results[i]
		kept := r.Issues[:0]
		for _, is := range r.Issues {
			fp := Fingerprint(r.Target, is)
			if known[fp] {
				stats.Baselined++
				matched[fp] = true
				continue
			}
			stats.New++
			kept = append(kept, is)
		}
		r.Issues = kept
	}
	stats.Fixed = len(f.Entries) - len(matched)
	return stats
}

// Prune drops entries whose issue no longer occurs (for --baseline-update).
// Call with the unfiltered results, i.e. before Apply strips them.
func (f *File) Prune(results []hatchet.Result) {
	present := map[string]bool{}
	for _, r := range results {
		for _, is := range r.Issues {
			present[Fingerprint(r.Target, is)] = true
		}
	}
	kept := f.Entries[:0]
	for _, e := range f.Entries {
		if present[e.Fingerprint] {
			kept = append(kept, e)
		}
	}
	f.Entries = kept
}
