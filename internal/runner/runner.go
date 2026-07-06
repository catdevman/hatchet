// Package runner injects axe-core into a loaded page and returns its raw
// results. Mapping to hatchet's Issue model happens in pkg/hatchet, keeping
// this package free of public-API imports.
package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/catdevman/hatchet/third_party/axe"
)

// Options control one axe execution.
type Options struct {
	// Tags restrict which rules run (axe runOnly tag filter).
	Tags []string
	// RootElement restricts checking to this CSS selector's subtree.
	RootElement string
	// HideElements excludes these CSS selectors from checking.
	HideElements []string
	// Source overrides the embedded axe-core build.
	Source string
}

// Results is the subset of axe.run() output hatchet consumes.
type Results struct {
	TestEngine struct {
		Version string `json:"version"`
	} `json:"testEngine"`
	Violations []Rule `json:"violations"`
	Incomplete []Rule `json:"incomplete"`
}

type Rule struct {
	ID          string   `json:"id"`
	Impact      string   `json:"impact"`
	Description string   `json:"description"`
	Help        string   `json:"help"`
	HelpURL     string   `json:"helpUrl"`
	Tags        []string `json:"tags"`
	Nodes       []Node   `json:"nodes"`
}

type Node struct {
	HTML           string     `json:"html"`
	Impact         string     `json:"impact"`
	Target         TargetList `json:"target"`
	FailureSummary string     `json:"failureSummary"`
}

// TargetList is axe's node target: a list of CSS selectors where each entry
// is a string, or a nested list when the node is inside shadow DOM.
type TargetList []string

func (t *TargetList) UnmarshalJSON(data []byte) error {
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = flattenTargets(raw)
	return nil
}

func flattenTargets(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		switch s := v.(type) {
		case string:
			out = append(out, s)
		case []any:
			out = append(out, flattenTargets(s)...)
		}
	}
	return out
}

// axeContext builds the first argument to axe.run: "document", or an
// include/exclude object when scoping options are set.
func axeContext(opts Options) (string, error) {
	if opts.RootElement == "" && len(opts.HideElements) == 0 {
		return "document", nil
	}
	ctx := map[string]any{}
	if opts.RootElement != "" {
		ctx["include"] = [][]string{{opts.RootElement}}
	}
	if len(opts.HideElements) > 0 {
		exclude := make([][]string, 0, len(opts.HideElements))
		for _, sel := range opts.HideElements {
			exclude = append(exclude, []string{sel})
		}
		ctx["exclude"] = exclude
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Run injects the embedded axe-core into the page behind tabCtx and executes
// it with the given options.
func Run(tabCtx context.Context, opts Options) (*Results, error) {
	source := opts.Source
	if source == "" {
		source = axe.Source
	}
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(source, nil)); err != nil {
		return nil, fmt.Errorf("injecting axe-core: %w", err)
	}

	tagsJSON, err := json.Marshal(opts.Tags)
	if err != nil {
		return nil, err
	}
	axeCtx, err := axeContext(opts)
	if err != nil {
		return nil, err
	}
	// Stringify in the page: CDP's deep object serialization chokes on large
	// result trees, a JSON string round-trips cleanly.
	expr := fmt.Sprintf(
		`axe.run(%s, {runOnly: {type: "tag", values: %s}}).then(r => JSON.stringify(r))`,
		axeCtx, tagsJSON,
	)

	var raw string
	err = chromedp.Run(tabCtx, chromedp.Evaluate(expr, &raw,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}))
	if err != nil {
		return nil, fmt.Errorf("running axe-core: %w", err)
	}

	var res Results
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, fmt.Errorf("parsing axe-core results: %w", err)
	}
	return &res, nil
}
