// Package actions parses pa11y's action DSL (HLD §13) into chromedp actions
// that run after page load and before axe.
package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	clickRe   = regexp.MustCompile(`^click element (.+)$`)
	setRe     = regexp.MustCompile(`^set field (.+?) to (.+)$`)
	checkRe   = regexp.MustCompile(`^(check|uncheck) field (.+)$`)
	waitElRe  = regexp.MustCompile(`^wait for element (.+?) to be (visible|hidden|added|removed)$`)
	waitURLRe = regexp.MustCompile(`^wait for (url|path|fragment) to (be|not be) (.+)$`)
	navRe     = regexp.MustCompile(`^navigate to (.+)$`)
	shotRe    = regexp.MustCompile(`^screen capture (.+)$`)
)

// Parse turns pa11y action strings into chromedp actions. Unknown actions
// fail here, before any page is loaded.
func Parse(strs []string) ([]chromedp.Action, error) {
	acts := make([]chromedp.Action, 0, len(strs))
	for _, s := range strs {
		a, err := parseOne(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	return acts, nil
}

func parseOne(s string) (chromedp.Action, error) {
	switch {
	case clickRe.MatchString(s):
		sel := clickRe.FindStringSubmatch(s)[1]
		return chromedp.Click(sel, chromedp.ByQuery), nil

	case setRe.MatchString(s):
		m := setRe.FindStringSubmatch(s)
		return setField(m[1], m[2]), nil

	case checkRe.MatchString(s):
		m := checkRe.FindStringSubmatch(s)
		return checkField(m[2], m[1] == "check"), nil

	case waitElRe.MatchString(s):
		m := waitElRe.FindStringSubmatch(s)
		sel := m[1]
		switch m[2] {
		case "visible":
			return chromedp.WaitVisible(sel, chromedp.ByQuery), nil
		case "hidden":
			return chromedp.WaitNotVisible(sel, chromedp.ByQuery), nil
		case "added":
			return chromedp.WaitReady(sel, chromedp.ByQuery), nil
		default: // removed
			return chromedp.WaitNotPresent(sel, chromedp.ByQuery), nil
		}

	case waitURLRe.MatchString(s):
		m := waitURLRe.FindStringSubmatch(s)
		return waitForLocation(m[1], m[3], m[2] == "not be"), nil

	case navRe.MatchString(s):
		return chromedp.Navigate(navRe.FindStringSubmatch(s)[1]), nil

	case shotRe.MatchString(s):
		return screenCapture(shotRe.FindStringSubmatch(s)[1]), nil

	default:
		return nil, fmt.Errorf("unknown action %q", s)
	}
}

// jsStr renders a Go string as a JS string literal.
func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// setField assigns a form field's value and fires the input/change events
// frameworks listen for, like pa11y does.
func setField(sel, value string) chromedp.Action {
	return evalOnElement(sel, fmt.Sprintf(
		`el.value = %s;
		 el.dispatchEvent(new Event("input", {bubbles: true}));
		 el.dispatchEvent(new Event("change", {bubbles: true}));`, jsStr(value)))
}

func checkField(sel string, checked bool) chromedp.Action {
	return evalOnElement(sel, fmt.Sprintf(
		`el.checked = %t;
		 el.dispatchEvent(new Event("change", {bubbles: true}));`, checked))
}

// evalOnElement runs body with `el` bound to the selector's first match,
// failing when nothing matches.
func evalOnElement(sel, body string) chromedp.Action {
	expr := fmt.Sprintf(
		`(() => {
			const el = document.querySelector(%s);
			if (!el) return false;
			%s
			return true;
		})()`, jsStr(sel), body)
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var ok bool
		if err := chromedp.Evaluate(expr, &ok).Do(ctx); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("no element matches selector %q", sel)
		}
		return nil
	})
}

// waitForLocation polls until the page URL (or its path/fragment) matches —
// or stops matching — the wanted value.
func waitForLocation(kind, want string, negate bool) chromedp.Action {
	var get string
	switch kind {
	case "path":
		get = "location.pathname"
	case "fragment":
		get = "location.hash"
	default:
		get = "location.href"
	}
	expr := fmt.Sprintf(`%s === %s`, get, jsStr(want))
	if negate {
		expr = "!(" + expr + ")"
	}
	return chromedp.ActionFunc(func(ctx context.Context) error {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			var ok bool
			if err := chromedp.Evaluate(expr, &ok).Do(ctx); err != nil {
				return err
			}
			if ok {
				return nil
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("waiting for %s: %w", kind, ctx.Err())
			case <-ticker.C:
			}
		}
	})
}

func screenCapture(path string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var buf []byte
		if err := chromedp.FullScreenshot(&buf, 100).Do(ctx); err != nil {
			return err
		}
		return os.WriteFile(path, buf, 0o644)
	})
}
