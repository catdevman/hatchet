# hatchet

Accessibility testing for CI in one binary. Hatchet is a Go replacement for
[pa11y](https://github.com/pa11y/pa11y) and
[pa11y-ci](https://github.com/pa11y/pa11y-ci): it loads pages in headless
Chrome, runs the embedded [axe-core](https://github.com/dequelabs/axe-core)
rule engine, and fails your build on WCAG issues — no Node.js, no
`npm install`, no Puppeteer download.

```console
$ hatchet https://example.com
Results for https://example.com:

 • Error: Images must have alternative text (https://dequeuniversity.com/rules/axe/4.10/image-alt)
   ├── image-alt
   ├── img
   └── <img src="hero.png">

1 errors, 0 warnings, 0 notices
$ echo $?
2
```

## Install

```sh
brew install catdevman/tap/hatchet     # Homebrew
go install github.com/catdevman/hatchet/cmd/hatchet@latest
```

Hatchet uses any system Chrome/Chromium. On machines without one:

```sh
hatchet browser install    # pinned chrome-headless-shell into ~/.cache/hatchet
```

Or point it at a browser running elsewhere (e.g. a CI sidecar container):

```sh
hatchet --browser-ws-endpoint ws://chrome:9222 https://example.com
```

In containers, Chrome usually needs `--no-sandbox`.

## Usage

```
hatchet [options] <url | path | -> [<url>...]
hatchet browser install
```

Exit codes: `0` clean · `2` issues at/above `--level` exceeded `--threshold` ·
`1` operational error.

| Flag | Description |
|------|-------------|
| `-s, --standard` | `WCAG2A`, `WCAG2AA` (default), `WCAG22AA`, `WCAG2AAA` |
| `-r, --reporter` | `cli`, `json`, `csv`, `sarif`, `junit`; repeatable; `name=path` writes to a file |
| `-E, --ignore` | ignore a rule code (`color-contrast`) or type (`warning`); repeatable |
| `--include-warnings`, `--include-notices` | report more than errors (pa11y default is errors only) |
| `--root-element`, `--hide-elements` | scope checking with CSS selectors |
| `-a, --actions` | pa11y action strings run before checking; repeatable |
| `-T, --threshold` | issues a target may have before failing |
| `--level` | minimum type affecting the exit code (default `error`) |
| `-t, --timeout`, `-w, --wait` | per-target timeout / post-load settle, ms |
| `--viewport`, `--user-agent`, `--add-cookie`, `--add-header`, `--basic-auth` | page environment |
| `--concurrency` | parallel targets (default 4) |
| `--sitemap`, `--sitemap-find/-replace/-exclude` | load targets from an XML sitemap |
| `--baseline`, `--baseline-write`, `--baseline-update`, `--baseline-ratchet` | see Baselines |
| `--renderer` | `chrome` (default) or `static` (page JS never runs) |
| `--screen-capture` | full-page PNG (single target) |
| `--chrome-path`, `--browser-ws-endpoint`, `--no-sandbox` | browser control |
| `--axe-path` | alternate axe-core build (e.g. locale builds) |
| `-c, --config` | config file (default: `.hatchetrc` / `hatchet.json` in CWD) |

### Actions

Drive the page before checking (Chrome renderer only):

```sh
hatchet \
  -a "set field #email to user@example.com" \
  -a "click element #login" \
  -a "wait for path to be /dashboard" \
  https://example.com/signin
```

Supported: `click element <sel>` · `set field <sel> to <value>` ·
`check/uncheck field <sel>` · `wait for element <sel> to be
visible|hidden|added|removed` · `wait for url|path|fragment to (not) be <val>`
· `navigate to <url>` · `screen capture <path>`.

### Multi-URL runs (pa11y-ci replacement)

`hatchet.json`:

```json
{
  "defaults": {"standard": "WCAG2AA", "timeout": 30000},
  "urls": [
    "https://example.com/",
    {"url": "https://example.com/checkout", "threshold": 3,
     "actions": ["click element #accept-cookies"]}
  ]
}
```

Then just `hatchet` (or `hatchet -c hatchet.json`). Per-URL settings override
defaults and flags for that URL. Targets run concurrently in tabs of one
browser process. Or check a whole site:

```sh
hatchet --sitemap https://example.com/sitemap.xml --sitemap-exclude '\.pdf$'
```

### Baselines: adopt on a site with existing issues

Accept today's issues, fail only on new ones:

```sh
hatchet --baseline-write a11y-baseline.json https://example.com   # commit the file
hatchet --baseline a11y-baseline.json https://example.com          # CI: new issues fail
hatchet --baseline a11y-baseline.json --baseline-update ...        # prune fixed issues
hatchet --baseline a11y-baseline.json --baseline-ratchet ...       # also fail when nothing improved
```

Baselines record the axe-core version; hatchet warns when it differs.

### CI recipes

GitHub Actions with PR annotations via code scanning:

```yaml
- run: hatchet -r cli -r sarif=a11y.sarif --no-sandbox https://staging.example.com
- uses: github/codeql-action/upload-sarif@v3
  if: always()
  with: {sarif_file: a11y.sarif}
```

GitLab test reports:

```yaml
a11y:
  script: hatchet -r junit=a11y.xml --no-sandbox https://staging.example.com
  artifacts:
    reports: {junit: a11y.xml}
```

### Static renderer

`--renderer static` checks the served markup exactly as delivered: the page's
own JavaScript never executes, making results deterministic for
server-rendered sites (and immune to client-side rendering flake). Actions
are unavailable in this mode.

### Use as a Go library

```go
import "github.com/catdevman/hatchet/pkg/hatchet"

results, err := hatchet.Run(ctx, []string{server.URL}, hatchet.Options{})
```

Works against `httptest.Server` in ordinary Go tests — accessibility
assertions in your test suite with no separate CI step. The API is not stable
before v1.0.

## Migrating from pa11y

- Flags and config keys keep pa11y's names where they exist
  (`--standard`, `--ignore`, `--root-element`, actions, `.pa11yci`-style
  `defaults`/`urls`, …).
- **Issue codes are axe rule ids** (`image-alt`, `color-contrast`), not
  HTML_CodeSniffer codes (`WCAG2AA.Principle1...`) — hatchet's engine is axe.
  Ignore lists written for HTMLCS need translating.
- Warnings map from axe "incomplete" checks; axe produces no notices, so
  `--include-notices` exists for compatibility but adds nothing.
- pa11y's JSON output was a bare issue array; hatchet's is a versioned,
  multi-target document (`schemaVersion` field).

## Development

See `CLAUDE.md` for architecture rules and `TASKS.md` for project status.
axe-core (MPL-2.0) is vendored unmodified in `third_party/axe/`; pins are
bumped with `scripts/update-pins.sh`.
