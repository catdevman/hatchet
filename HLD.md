# Hatchet — High-Level Design

A single-binary accessibility testing CLI written in Go that replaces both
[pa11y](https://github.com/pa11y/pa11y) and
[pa11y-ci](https://github.com/pa11y/pa11y-ci) for CI use. It orchestrates page
loading and injects [axe-core](https://github.com/dequelabs/axe-core) as the
rule engine, reporting WCAG 2.x issues at levels A / AA / AAA.

The value proposition is operational, not linguistic: a team should go from
nothing to a failing accessibility check in CI in seconds, with no Node.js, no
`npm install`, and no preinstalled browser. Every design decision below is
judged against that bar.

## 1. Goals

- Replace the `pa11y <url>` *and* `pa11y-ci` workflows: single-URL and
  multi-URL/sitemap runs, same issue semantics (errors / warnings / notices),
  same exit-code contract.
- Rule engine: axe-core, embedded in the binary (`go:embed`), version-pinned.
- Standards: WCAG 2.1 and 2.2, selectable level A / AA / AAA via `--standard`.
- Zero-setup browser: auto-download of a pinned `chrome-headless-shell`, plus
  system-Chrome discovery and remote CDP connection (§5).
- CI-native reporting: SARIF (GitHub code scanning / PR annotations) and JUnit
  XML alongside pa11y's cli/json/csv (§8).
- Baseline ("ratchet") mode: accept existing issues, fail only on new ones, so
  legacy sites can adopt incrementally (§9).
- Concurrency: one browser process, a pool of tabs checking many URLs in
  parallel — the structural speed win over sequential pa11y-ci (§7).
- Library-friendly core: the CLI is a thin wrapper over a public Go package so
  the engine can be embedded (e.g. in `go test` against `httptest.Server`).
  Supported by construction, but not a v1 headline; API stability is not
  promised before v1.0 (§4).

## 2. Non-Goals (v1)

- Section 508 ruleset.
- HTML_CodeSniffer runner support (axe-core only; pa11y issue codes are mapped,
  not reproduced — see Open Question 1).
- Reimplementing accessibility rules natively in Go.
- Crawling beyond sitemaps (no link-following spider).
- Reporting UI beyond `--screen-capture` and the reporters in §8.

## 3. Background: how pa11y works

pa11y is a thin orchestrator: launch headless Chrome (Puppeteer) → navigate →
optionally run scripted *actions* → inject a JS test runner → collect issues →
filter by standard/ignore rules → hand to a *reporter* → exit non-zero if
error-level issues exceed the threshold. pa11y-ci wraps this in a loop over a
URL list or sitemap.

Hatchet keeps this exact shape and swaps Node/Puppeteer for Go/chromedp, with
the loop made concurrent.

## 4. Architecture

```
┌─────────┐   ┌────────┐   ┌───────────────────┐   ┌──────────┐   ┌──────────┐
│ CLI     │──▶│ Config │──▶│ Browser pool       │──▶│ Runner   │──▶│ Reporter │
│ (flags) │   │ merge  │   │ (tabs × N targets) │   │ (axe-core│   │ cli/json │
└─────────┘   └────────┘   │ + Actions          │   │  inject) │   │ csv/sarif│
                           └───────────────────┘   └──────────┘   │ junit    │
                                                        │          └──────────┘
                                                  Issue model ──▶ filter (ignore,
                                                  root/hide, level) ──▶ baseline
                                                  diff ──▶ threshold → exit code
```

### Package layout

```
cmd/hatchet/          main; flag parsing, thin wiring only
pkg/hatchet/          public API: Run(ctx, targets, opts) — everything the CLI can do
internal/config/      config file + flag merging, defaults, pa11y-ci-style URL lists
internal/browser/     acquisition (download/discover/connect), lifecycle, tab pool
internal/renderer/    Renderer interface; chrome/ and static/ implementations
internal/actions/     pa11y action DSL parser + executor (Chrome renderer only)
internal/runner/      axe-core injection, result → Issue mapping, standards → axe tags
internal/issue/       Issue type, filtering, baseline fingerprinting + diff
internal/reporter/    Reporter interface; cli, json, csv, sarif, junit
third_party/axe/      vendored axe.min.js (MPL-2.0, unmodified) + version manifest
```

The CLI holds no logic beyond flag parsing: every capability is reachable
through `pkg/hatchet`, which is what keeps the `go test` embedding story free.

### Core types (illustrative, not final)

```go
type Issue struct {
    Code     string // axe rule id, e.g. "color-contrast"
    Type     string // "error" | "warning" | "notice"
    Message  string
    Context  string // outerHTML snippet, truncated like pa11y
    Selector string // CSS selector to the node
    RunnerExtras map[string]any // axe impact, help URL, tags
}

// One entry per target URL; multi-URL runs aggregate these.
type Result struct {
    Target string
    Issues []Issue
    Err    error // per-URL failure doesn't abort the run
}

type Renderer interface {
    Load(ctx context.Context, target Target, opts LoadOptions) (Page, error)
}

type Reporter interface {
    Report(w io.Writer, results []Result) error
}
```

## 5. Browser acquisition

Chrome availability is the make-or-break operational question, so hatchet
supports three paths, tried in this order unless overridden:

1. **Explicit path / discovery** — `--chrome-path`, else well-known system
   locations for Chrome/Chromium per OS.
2. **Managed download** — `hatchet browser install` fetches a pinned
   `chrome-headless-shell` build (the stripped headless-only binary,
   substantially smaller than full Chrome) from Google's
   Chrome-for-Testing distribution into `~/.cache/hatchet/`, verified by
   checksum. Runs also offer this interactively / via
   `--browser-install` when nothing is found. The pinned version lives next to
   the axe version manifest and is bumped deliberately.
3. **Remote CDP** — `--browser-ws-endpoint ws://...` connects to an
   already-running browser (browserless, a Chrome sidecar container, etc.) and
   launches nothing locally. In this mode hatchet is a pure static binary with
   zero local browser footprint — the preferred setup for hosted CI.

An official Docker image with the headless shell baked in is a later
distribution item (§12), not a v1 blocker, because paths 2 and 3 already cover
CI.

## 6. Renderer

### 6.1 Chrome renderer (default)

- [chromedp](https://github.com/chromedp/chromedp) over the DevTools protocol,
  against whichever browser §5 produced.
- Supports everything pa11y does: JS-rendered pages, `--wait`, actions,
  basic-auth/headers/cookies, viewport size, `--user-agent`, `--screen-capture`,
  `--timeout` (whole-run deadline via context).

### 6.2 Static renderer

Purpose: deterministic checking of server-rendered HTML without executing page
JS — immune to flaky client-side rendering. axe-core still needs a live DOM,
so static mode means: **navigate with script execution disabled while the
document parses (the page's own scripts are skipped for good), then re-enable
execution so axe can run via CDP evaluation.** One rule engine, two loading
semantics; the served markup is checked exactly as delivered. With managed
download / remote CDP (§5), the browser requirement is no longer an
operational burden, so this stays simple.

Actions are unavailable in static mode (no page JS); combining them is a CLI
error.

## 7. Multi-URL runs (pa11y-ci replacement)

- Targets come from: repeated positional args, `--sitemap <url>` (with
  pa11y-ci's `--sitemap-find/--sitemap-replace/--sitemap-exclude`), or the
  `urls` array in the config file.
- Config mirrors pa11y-ci's shape: top-level `defaults` plus per-URL entries
  that may override any option (timeout, actions, ignore, threshold, …).
- Execution: **one browser process, a tab pool of `--concurrency` workers**
  (default 4). Tabs are isolated browser contexts, so cookies/auth per URL
  don't bleed. Per-URL failures are recorded in that URL's `Result`, not fatal.
- Aggregation: reporters receive all `Result`s; exit code reflects the
  aggregate (any URL over threshold → exit 2; any operational error → exit 1,
  matching pa11y-ci).
- Single-URL is just the N=1 case — there is no separate code path.

## 8. Reporters

| Reporter | Purpose |
|----------|---------|
| `cli` (default) | Human-readable, TTY-colored, per-URL sections + summary |
| `json` | Machine-readable; **schema carries a version field** so scripts survive upgrades |
| `csv` | pa11y parity |
| `sarif` | GitHub code scanning ingestion → issues surface as PR annotations natively |
| `junit` | Test-report ingestion in GitLab, Jenkins, CircleCI, etc. |

`--reporter` is repeatable with per-reporter output paths
(e.g. `--reporter cli --reporter sarif=report.sarif`), so a CI run can get the
human summary and the machine artifact in one pass.

## 9. Baseline ("ratchet") mode

The #1 adoption blocker for accessibility CI on existing sites: the first run
reports hundreds of issues and the check gets deleted. Baseline mode fixes the
incentive:

- `hatchet --baseline-write baseline.json <targets>` records current issues.
- Subsequent runs with `--baseline baseline.json` fail **only on issues not in
  the baseline**; fixed issues are reported as such and can be pruned with
  `--baseline-update`.
- Issue identity across runs uses a fingerprint of *(target URL, rule code,
  selector, normalized context hash)*. Selectors drift when the DOM changes;
  matching falls back through fingerprint components and treats ambiguity as
  "new" (fail-safe). This is the hardest design problem in the feature — see
  Open Question 4.
- Optional `--baseline-ratchet`: additionally fail if the total baselined count
  did not decrease, for teams that want forced burn-down.

Baseline files are JSON, deterministic (sorted), and meant to be committed.

## 10. CLI surface

```
hatchet [options] <url | path | -> [<url>...]
hatchet browser install
```

| Flag | Notes |
|------|-------|
| `--standard, -s` | see §11; default `WCAG2AA` |
| `--reporter, -r` | repeatable; `cli` (default), `json`, `csv`, `sarif`, `junit` (§8) |
| `--ignore, -E` | repeatable; rule code or issue type |
| `--include-notices` / `--include-warnings` | off by default, like pa11y |
| `--root-element` / `--hide-elements` | CSS selector scoping |
| `--threshold, -T` | max issues before failing exit code; default 0 |
| `--level` | minimum type that affects exit code (`error` default) |
| `--baseline` / `--baseline-write` / `--baseline-update` / `--baseline-ratchet` | §9 |
| `--sitemap` / `--sitemap-find` / `--sitemap-replace` / `--sitemap-exclude` | §7 |
| `--concurrency` | tab-pool size for multi-URL runs; default 4 |
| `--timeout, -t` | ms, per URL; `--wait, -w` ms after load |
| `--config, -c` | JSON config file (see §12) |
| `--actions, -a` | repeatable action strings (Chrome mode only) |
| `--renderer` | `chrome` (default) \| `static` |
| `--chrome-path` / `--browser-install` / `--browser-ws-endpoint` | §5 |
| `--axe-path` | override the embedded axe-core with a local build/locale |
| `--viewport`, `--user-agent`, `--add-cookie`, `--add-header`, `--basic-auth` | environment knobs |
| `--screen-capture` | PNG path (Chrome mode, single URL) |
| `--debug, -d` | structured logs to stderr |

**Exit codes** (pa11y contract): `0` clean, `2` issues at/above `--level`
exceeded threshold (post-baseline-filtering), `1` operational error (bad URL,
no browser, timeout).

## 11. Runner (axe-core)

- `axe.min.js` embedded via `go:embed`; version pinned in
  `third_party/axe/VERSION` and printed by `hatchet --version`;
  `--axe-path` overrides for custom versions/locales.
- Injected after load/actions; run with `axe.run(document, {runOnly: {type:
  "tag", values: [...]}})` where tags derive from `--standard`:

| `--standard`      | axe tags                                      |
|-------------------|-----------------------------------------------|
| `WCAG2A`          | `wcag2a`, `wcag21a`                           |
| `WCAG2AA` (default)| A tags + `wcag2aa`, `wcag21aa`               |
| `WCAG22AA`        | AA tags + `wcag22aa`                          |
| `WCAG2AAA`        | AA tags + `wcag2aaa`                          |

- Result mapping (same as pa11y's axe runner): axe *violations* → `error`,
  *incomplete* → `warning`, plus optional notices; node target → `Selector`,
  html → `Context`.
- `--runner` flag reserved for future runners but only `axe` in v1.

## 12. Config file & distribution

**Config:** JSON, merged under CLI flags (flags win). Field names mirror
pa11y/pa11y-ci (`defaults`, `urls`, `ignore`, `hideElements`, `timeout`,
`actions`, …) so existing `.pa11yrc` / `.pa11yci` files mostly port over.
Discovery: `--config` path, else `.hatchetrc` / `hatchet.json` in CWD. No JS
config files.

**Distribution (v1 floor):**
- goreleaser: signed-off, reproducible cross-platform binaries
  (linux/macos/windows, amd64/arm64) on tagged releases.
- Homebrew tap (`brew install catdevman/tools/hatchet`).

**Later (explicitly deferred, tracked as post-v1 items):**
- Official Docker image with `chrome-headless-shell` baked in.
- Published GitHub Action (`uses: catdevman/hatchet@v1`).
- Supply-chain hardening: cosign-signed artifacts, SBOM, provenance.

## 13. Actions

Same string DSL as pa11y, parsed in Go, executed via chromedp:

```
click element <selector>
set field <selector> to <value>
check/uncheck field <selector>
wait for element <selector> to be visible|hidden|added|removed
wait for url|path|fragment to be|to not be <value>
navigate to <url>
screen capture <path>
```

Unknown action strings fail fast before the page is loaded.

## 14. Testing strategy

- Unit: action parser, config merge, issue filtering, baseline fingerprinting
  and diff (table-driven with DOM-drift cases), standards→tag mapping,
  reporters (golden files, including SARIF/JUnit schema validation).
- Integration: `httptest` server serving fixture pages with known violations
  (missing alt, bad contrast, missing labels); assert issue codes/counts
  through the real browser, including concurrent multi-URL runs. Tagged so CI
  can skip when no browser is available.
- Embedding smoke test: a `go test` that uses `pkg/hatchet` directly against an
  `httptest.Server` — doubles as the documented library example.
- Parity check (manual/CI-optional): run pa11y-ci (axe runner) and hatchet
  against the same fixtures and diff issue sets.

## 15. Milestones

1. **Skeleton** — `pkg/hatchet` core with CLI wrapper, config, browser
   discovery + `--browser-ws-endpoint`, Chrome renderer, axe injection,
   cli + json reporters, exit codes. Usable in CI (against an existing
   browser) at this point.
2. **Parity** — ignore/threshold/level, root/hide elements, notices/warnings,
   cookies/headers/auth, viewport/UA, csv reporter, actions, screen capture.
3. **CI-grade** — multi-URL + sitemap + tab-pool concurrency, SARIF + JUnit
   reporters, baseline mode, `hatchet browser install` (managed
   headless-shell download).
4. **Ship** — goreleaser + Homebrew tap, static renderer, docs, parity test
   suite vs pa11y-ci, pinned axe/browser update workflow.

## 16. Open questions / risks

1. **Issue-code compatibility**: pa11y's default runner (HTML_CodeSniffer)
   emits codes like `WCAG2AA.Principle1.Guideline1_1...`; axe emits
   `color-contrast`. Existing ignore-lists written against HTMLCS codes won't
   port. Accept axe codes as canonical, or ship a best-effort HTMLCS→axe alias
   table?
2. **axe-core updates**: rule changes across axe versions alter results —
   and silently invalidate committed baselines. Pinning + explicit version
   bumps is the plan; baseline files should record the axe version they were
   written with and warn on mismatch.
3. **WCAG 2.2 coverage in axe**: some 2.2 criteria have limited/no automated
   axe rules; `--standard WCAG22AA` can only report what axe implements. State
   this in docs to set expectations.
4. **Baseline fingerprint stability**: selector/context drift on dynamic pages
   will cause baselined issues to resurface as "new". The fail-safe bias (§9)
   is correct but may frustrate users on churn-heavy pages; may need a
   `--baseline-match loose|strict` knob. Validate against real sites before
   freezing the file format.
5. **headless-shell download source**: Chrome-for-Testing URLs and layout are
   Google-controlled and have changed before; the installer needs a manifest we
   control (mirroring version → URL → checksum) rather than hardcoding.
