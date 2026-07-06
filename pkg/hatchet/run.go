package hatchet

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/catdevman/hatchet/internal/actions"
	"github.com/catdevman/hatchet/internal/browser"
	"github.com/catdevman/hatchet/internal/runner"
)

const (
	defaultTimeout        = 30 * time.Second
	defaultViewportWidth  = 1280 // pa11y defaults
	defaultViewportHeight = 1024
)

// Target is one URL to check. Options, when set, replace the run-level
// Options for this target (the caller merges); browser-level settings
// (ChromePath, BrowserWSEndpoint, NoSandbox, Concurrency, Logf) always come
// from the run-level Options.
type Target struct {
	URL     string
	Options *Options
}

// Run checks each URL with the same options. See RunTargets.
func Run(ctx context.Context, urls []string, opts Options) ([]Result, error) {
	targets := make([]Target, len(urls))
	for i, u := range urls {
		targets[i] = Target{URL: u}
	}
	return RunTargets(ctx, targets, opts)
}

// prepared holds everything derived from a target's options before any page
// work, so option errors fail the whole run up front.
type prepared struct {
	runnerOpts runner.Options
	acts       []chromedp.Action
	opts       Options
}

// RunTargets checks each target and returns one Result per target, in order.
// Per-target failures land in Result.Err; the returned error is reserved for
// run-level problems (bad options, unparseable actions, no browser).
func RunTargets(ctx context.Context, targets []Target, opts Options) ([]Result, error) {
	preps := make([]prepared, len(targets))
	axeSources := map[string]string{} // path → source, loaded once
	for i, t := range targets {
		o := opts
		if t.Options != nil {
			o = *t.Options
		}
		tags, err := standardTags(o.Standard)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", t.URL, err)
		}
		acts, err := actions.Parse(o.Actions)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", t.URL, err)
		}
		if o.Renderer != "" && o.Renderer != "chrome" && o.Renderer != "static" {
			return nil, fmt.Errorf("target %s: unknown renderer %q (expected chrome or static)", t.URL, o.Renderer)
		}
		if o.Renderer == "static" && len(o.Actions) > 0 {
			return nil, fmt.Errorf("target %s: actions need page JavaScript and cannot run with the static renderer", t.URL)
		}
		var axeSource string
		if o.AxePath != "" {
			src, ok := axeSources[o.AxePath]
			if !ok {
				data, err := os.ReadFile(o.AxePath)
				if err != nil {
					return nil, fmt.Errorf("loading axe from --axe-path: %w", err)
				}
				src = string(data)
				axeSources[o.AxePath] = src
			}
			axeSource = src
		}
		preps[i] = prepared{
			runnerOpts: runner.Options{
				Tags:         tags,
				RootElement:  o.RootElement,
				HideElements: splitSelectors(o.HideElements),
				Source:       axeSource,
			},
			acts: acts,
			opts: o,
		}
	}

	b, err := browser.New(ctx, browser.Options{
		ChromePath: opts.ChromePath,
		WSEndpoint: opts.BrowserWSEndpoint,
		NoSandbox:  opts.NoSandbox,
		Logf:       opts.Logf,
	})
	if err != nil {
		return nil, err
	}
	defer b.Close()

	sem := make(chan struct{}, max(opts.Concurrency, 1))
	var wg sync.WaitGroup
	results := make([]Result, len(targets))
	for i, t := range targets {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = checkTarget(b, t.URL, preps[i])
		})
	}
	wg.Wait()
	return results, nil
}

func checkTarget(b *browser.Browser, target string, p prepared) Result {
	runnerOpts, acts, opts := p.runnerOpts, p.acts, p.opts
	res := Result{Target: target}

	url, err := resolveTarget(target)
	if err != nil {
		res.Err = err
		return res
	}

	tabCtx, closeTab := b.NewTab()
	defer closeTab()

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	tabCtx, cancel := context.WithTimeout(tabCtx, timeout)
	defer cancel()

	load := environmentActions(url, opts)
	if opts.Renderer == "static" {
		// Scripts are disabled while the document parses, so the page's own
		// scripts are skipped for good (re-enabling later does not revisit
		// them). axe needs a live event loop, so execution is switched back
		// on after load, once no page code is left to run.
		load = append(load, emulation.SetScriptExecutionDisabled(true))
	}
	load = append(load, chromedp.Navigate(url))
	if opts.Wait > 0 {
		load = append(load, chromedp.Sleep(opts.Wait))
	}
	load = append(load, acts...)
	if opts.Renderer == "static" {
		load = append(load, emulation.SetScriptExecutionDisabled(false))
	}
	if err := chromedp.Run(tabCtx, load...); err != nil {
		res.Err = fmt.Errorf("loading %s: %w", url, err)
		return res
	}

	axeRes, err := runner.Run(tabCtx, runnerOpts)
	if err != nil {
		res.Err = err
		return res
	}
	res.Issues = filterIssues(issuesFromAxe(axeRes), opts)

	if opts.ScreenCapture != "" {
		var buf []byte
		if err := chromedp.Run(tabCtx, chromedp.FullScreenshot(&buf, 100)); err != nil {
			res.Err = fmt.Errorf("capturing screen: %w", err)
			return res
		}
		if err := os.WriteFile(opts.ScreenCapture, buf, 0o644); err != nil {
			res.Err = err
		}
	}
	return res
}

// environmentActions configure the tab (viewport, UA, headers, cookies)
// before navigation.
func environmentActions(url string, opts Options) []chromedp.Action {
	w, h := opts.ViewportWidth, opts.ViewportHeight
	if w <= 0 {
		w = defaultViewportWidth
	}
	if h <= 0 {
		h = defaultViewportHeight
	}
	acts := []chromedp.Action{chromedp.EmulateViewport(int64(w), int64(h))}

	if opts.UserAgent != "" {
		acts = append(acts, emulation.SetUserAgentOverride(opts.UserAgent))
	}

	headers := make(map[string]any, len(opts.Headers)+1)
	for k, v := range opts.Headers {
		headers[k] = v
	}
	if opts.BasicAuth != "" {
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(opts.BasicAuth))
	}
	if len(headers) > 0 {
		acts = append(acts, network.Enable(), network.SetExtraHTTPHeaders(network.Headers(headers)))
	}

	for _, c := range opts.Cookies {
		acts = append(acts, chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetCookie(c.Name, c.Value).WithURL(url).Do(ctx)
		}))
	}
	return acts
}

// splitSelectors breaks a comma-separated CSS selector list into individual
// selectors for axe's exclude context.
func splitSelectors(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var schemeRe = regexp.MustCompile(`^(https?|file)://`)

// resolveTarget applies pa11y's target rules: URLs with a scheme pass
// through, existing local paths become file:// URLs, anything else gets
// http:// prepended.
func resolveTarget(target string) (string, error) {
	if schemeRe.MatchString(target) {
		return target, nil
	}
	if abs, err := filepath.Abs(target); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			return "file://" + abs, nil
		}
	}
	return "http://" + target, nil
}
