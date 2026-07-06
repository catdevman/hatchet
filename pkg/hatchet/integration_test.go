package hatchet

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/catdevman/hatchet/internal/browser"
)

const brokenPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Hatchet fixture</title></head>
<body>
  <main>
    <h1>Fixture</h1>
    <img src="cat.png">
    <input type="text">
  </main>
</body>
</html>`

const cleanPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Clean fixture</title></head>
<body>
  <main><h1>Clean</h1><p>Nothing wrong here.</p></main>
</body>
</html>`

func integrationOpts(t *testing.T) Options {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	if _, err := browser.Discover(); err != nil {
		t.Skipf("no browser available: %v", err)
	}
	// Sandbox needs privileges tests often lack (containers, root).
	return Options{NoSandbox: true, Timeout: 60 * time.Second}
}

func serve(t *testing.T, html string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRunFindsKnownViolations(t *testing.T) {
	opts := integrationOpts(t)
	srv := serve(t, brokenPage)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("target failed: %v", results[0].Err)
	}

	found := map[string]bool{}
	for _, is := range results[0].Issues {
		found[is.Code] = true
		if is.Runner != "axe" {
			t.Errorf("issue runner = %q, want axe", is.Runner)
		}
	}
	for _, code := range []string{"image-alt", "label"} {
		if !found[code] {
			t.Errorf("expected %s violation, found codes: %v", code, found)
		}
	}
}

// scopedPage has one violation inside <main> (unlabeled input) and one
// outside it (image without alt).
const scopedPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Scoped fixture</title></head>
<body>
  <img src="outside.png">
  <main>
    <h1>Scoped</h1>
    <input type="text">
  </main>
</body>
</html>`

// actionPage adds a broken image only after the button is clicked.
const actionPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Action fixture</title></head>
<body>
  <main>
    <h1>Actions</h1>
    <button id="reveal">Reveal</button>
    <div id="slot"></div>
    <script>
      document.getElementById("reveal").addEventListener("click", () => {
        document.getElementById("slot").innerHTML = '<img id="added" src="late.png">';
      });
    </script>
  </main>
</body>
</html>`

func codesOf(t *testing.T, results []Result) map[string]bool {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("target failed: %v", results[0].Err)
	}
	found := map[string]bool{}
	for _, is := range results[0].Issues {
		found[is.Code] = true
	}
	return found
}

func TestRunRootElement(t *testing.T) {
	opts := integrationOpts(t)
	opts.RootElement = "main"
	srv := serve(t, scopedPage)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	found := codesOf(t, results)
	if !found["label"] {
		t.Errorf("expected label violation inside root, found: %v", found)
	}
	if found["image-alt"] {
		t.Errorf("image outside --root-element should not be checked, found: %v", found)
	}
}

func TestRunHideElements(t *testing.T) {
	opts := integrationOpts(t)
	opts.HideElements = "img"
	srv := serve(t, brokenPage)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	found := codesOf(t, results)
	if found["image-alt"] {
		t.Errorf("hidden img should not be checked, found: %v", found)
	}
	if !found["label"] {
		t.Errorf("label violation should survive hide-elements, found: %v", found)
	}
}

func TestRunIgnore(t *testing.T) {
	opts := integrationOpts(t)
	opts.Ignore = []string{"image-alt"}
	srv := serve(t, brokenPage)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	found := codesOf(t, results)
	if found["image-alt"] {
		t.Errorf("ignored code still reported: %v", found)
	}
	if !found["label"] {
		t.Errorf("non-ignored violation missing: %v", found)
	}
}

func TestRunActions(t *testing.T) {
	opts := integrationOpts(t)
	srv := serve(t, actionPage)

	// Without actions the page is clean.
	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if found := codesOf(t, results); found["image-alt"] {
		t.Fatalf("broken image should not exist before the click: %v", found)
	}

	// Clicking reveals the broken image; axe must see the mutated DOM.
	opts.Actions = []string{
		"click element #reveal",
		"wait for element #added to be added",
	}
	results, err = Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if found := codesOf(t, results); !found["image-alt"] {
		t.Errorf("expected image-alt after click action, found: %v", found)
	}
}

// jsInjectPage is statically clean except for an unlabeled input; its inline
// script adds a broken image as soon as it runs.
const jsInjectPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>JS inject fixture</title></head>
<body>
  <main>
    <h1>Static</h1>
    <input type="text">
    <div id="slot"></div>
    <script>
      document.getElementById("slot").innerHTML = '<img src="dynamic.png">';
    </script>
  </main>
</body>
</html>`

func TestStaticRenderer(t *testing.T) {
	opts := integrationOpts(t)
	srv := serve(t, jsInjectPage)

	// Chrome mode executes the script: both violations.
	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	found := codesOf(t, results)
	if !found["image-alt"] || !found["label"] {
		t.Fatalf("chrome mode should see both violations, found: %v", found)
	}

	// Static mode: the script never runs, so only the served markup counts —
	// and axe itself must still work through CDP evaluation.
	opts.Renderer = "static"
	results, err = Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	found = codesOf(t, results)
	if found["image-alt"] {
		t.Errorf("static mode executed page JS, found: %v", found)
	}
	if !found["label"] {
		t.Errorf("static mode lost the static violation (axe may not have run), found: %v", found)
	}
}

func TestStaticRendererRejectsActions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	opts := Options{Renderer: "static", Actions: []string{"click element #x"}}
	if _, err := Run(context.Background(), []string{"https://example.com"}, opts); err == nil {
		t.Fatal("static renderer with actions must fail fast")
	}
}

func TestRunTargetsConcurrent(t *testing.T) {
	opts := integrationOpts(t)
	opts.Concurrency = 4

	// Alternate broken/clean across enough URLs to exercise the pool, with
	// per-target option overrides for the odd ones.
	mux := http.NewServeMux()
	mux.HandleFunc("/broken/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(brokenPage))
	})
	mux.HandleFunc("/clean/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(cleanPage))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ignoreAll := opts
	ignoreAll.Ignore = []string{"image-alt", "label"}

	var targets []Target
	for i := range 8 {
		if i%2 == 0 {
			targets = append(targets, Target{URL: fmt.Sprintf("%s/broken/%d", srv.URL, i)})
		} else {
			// Broken page but with per-target ignore overrides: must be clean.
			targets = append(targets, Target{URL: fmt.Sprintf("%s/broken/%d", srv.URL, i), Options: &ignoreAll})
		}
	}

	results, err := RunTargets(context.Background(), targets, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 8 {
		t.Fatalf("got %d results, want 8", len(results))
	}
	for i, r := range results {
		if r.Target != targets[i].URL {
			t.Fatalf("result %d out of order: %s != %s", i, r.Target, targets[i].URL)
		}
		if r.Err != nil {
			t.Fatalf("target %s failed: %v", r.Target, r.Err)
		}
		if i%2 == 0 && len(r.Issues) == 0 {
			t.Errorf("broken target %s reported no issues", r.Target)
		}
		if i%2 == 1 && len(r.Issues) != 0 {
			t.Errorf("per-target ignore override not applied to %s: %v", r.Target, r.Issues)
		}
	}
}

func TestRunSendsEnvironment(t *testing.T) {
	opts := integrationOpts(t)
	opts.UserAgent = "hatchet-test-agent"
	opts.Headers = map[string]string{"X-Hatchet": "yes"}
	opts.Cookies = []Cookie{{Name: "session", Value: "abc123"}}
	opts.BasicAuth = "user:pass"

	var mu sync.Mutex
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if got == nil { // first request is the page itself
			got = r.Header.Clone()
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(cleanPage))
	}))
	t.Cleanup(srv.Close)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Err != nil {
		t.Fatalf("target failed: %v", results[0].Err)
	}

	mu.Lock()
	defer mu.Unlock()
	if ua := got.Get("User-Agent"); ua != "hatchet-test-agent" {
		t.Errorf("User-Agent = %q", ua)
	}
	if h := got.Get("X-Hatchet"); h != "yes" {
		t.Errorf("X-Hatchet = %q", h)
	}
	if c := got.Get("Cookie"); !strings.Contains(c, "session=abc123") {
		t.Errorf("Cookie = %q", c)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if a := got.Get("Authorization"); a != want {
		t.Errorf("Authorization = %q, want %q", a, want)
	}
}

func TestRunBadActionFailsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	opts := Options{Actions: []string{"do a barrel roll"}}
	// Must fail during parsing — no browser needed, even on machines without one.
	if _, err := Run(context.Background(), []string{"https://example.com"}, opts); err == nil {
		t.Fatal("expected parse error for unknown action")
	}
}

func TestRunCleanPage(t *testing.T) {
	opts := integrationOpts(t)
	srv := serve(t, cleanPage)

	results, err := Run(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Err != nil {
		t.Fatalf("target failed: %v", results[0].Err)
	}
	for _, is := range results[0].Issues {
		if is.Type == TypeError {
			t.Errorf("clean page has error: %+v", is)
		}
	}
}
