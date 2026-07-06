# TASKS.md — Hatchet status board

This file is the durable source of truth for project progress. It exists so any
agent, model, or human can pick up where the last one left off.

**Rules for whoever is working on this repo:**
1. Read `HLD.md` (the design) before changing anything structural.
2. Mark checkboxes here as work completes; add notes to the Decisions Log when
   you deviate from or refine the HLD.
3. Keep the Status Snapshot section current — it's the first thing the next
   session reads.

---

## Status Snapshot

- **Last updated:** 2026-07-05
- **Current milestone:** 4 (Ship) — **COMPLETE**. Milestones 1–4 complete.
- **State:** Feature-complete v1 per HLD. All tests pass (`go build` /
  `go vet` / `go test ./...`) including integration tests for concurrency,
  scoping, actions, environment knobs, and the static renderer. E2E-verified
  via binary: all reporters, baseline flows, config `defaults`+`urls`,
  per-URL thresholds, sitemap, `--axe-path`, `--renderer static`, and
  `hatchet browser install` including checksum verification (valid sum
  passes; corrupted pin rejected). Nothing committed to git yet.
- **Works right now:** the full HLD §10 CLI surface.
- **Next up (release checklist):** (1) initial git commit; (2) push to
  github.com/catdevman/hatchet; (3) create catdevman/homebrew-tools repo;
  (4) `goreleaser check` + `goreleaser release --snapshot --clean` locally;
  (5) run the manual pa11y-ci parity comparison; (6) tag v0.1.0 and release;
  (7) decide the repo's own license (post-v1 list). Post-v1: Docker image,
  GitHub Action, cosign/SBOM, remaining platform checksums via
  `scripts/update-pins.sh shell`.

## Environment notes (for reproducing/verifying)

- Dev box has `google-chrome-stable` at `/usr/bin/google-chrome-stable`; may
  need `--no-sandbox` inside containers/sandboxes.
- axe-core vendored from `https://cdn.jsdelivr.net/npm/axe-core@<ver>/axe.min.js`.
- Build: `go build ./...` · Test: `go test ./...` (integration tests skip
  themselves when no Chrome is found).

---

## Milestone 1 — Skeleton (usable in CI against an existing browser)

- [x] `git init`, `go mod init github.com/catdevman/hatchet`
- [x] TASKS.md, CLAUDE.md, .gitignore
- [x] `third_party/axe/`: vendored `axe.min.js` 4.10.3, `VERSION`, MPL-2.0
      `LICENSE`, `embed.go` (exports `Source`, `Version()`)
- [x] `internal/browser`: `Discover()` (known binary names + macOS paths),
      `New()` (ExecAllocator, or RemoteAllocator for `--browser-ws-endpoint`),
      `NewTab()`, `Close()`; `NoSandbox` option
- [x] `internal/runner`: `Run(tabCtx, tags)` — inject embedded axe, execute
      `axe.run` with `runOnly` tags + awaitPromise, `JSON.stringify` in page,
      parse violations/incomplete; shadow-DOM target flattening
- [x] `pkg/hatchet`: `Issue`/`Result`/`Options` types, `standardTags()`
      (WCAG2A/WCAG2AA/WCAG22AA/WCAG2AAA → axe tags per HLD §11),
      `Run(ctx, targets, opts)` orchestration, axe→Issue mapping
      (violations→error/1, incomplete→warning/2), target normalization
      (existing file → `file://`, schemeless → `http://` — pa11y behavior)
- [x] `internal/reporter`: `cli` (colored, NO_COLOR-aware) and `json`
      (schemaVersion 1, hatchet+axe versions, per-target results, totals)
- [x] `internal/config`: JSON config load (`--config`, else `.hatchetrc` /
      `hatchet.json` in CWD), pointer fields, flags-win merge
- [x] `cmd/hatchet`: flags (`-s/--standard`, `-r/--reporter` repeatable with
      `name=path`, `-t/--timeout`, `-w/--wait`, `-T/--threshold`, `--level`,
      `-c/--config`, `--chrome-path`, `--browser-ws-endpoint`, `--no-sandbox`,
      `-d/--debug`, `--version`), exit codes 0/1/2
- [x] Unit tests: standards mapping, axe→Issue mapping, reporters, config,
      exit codes, target resolution, TargetList unmarshal
- [x] Integration test: httptest fixture pages (missing alt, unlabeled input;
      clean page) through real Chrome; skips when Chrome absent or `-short`
- [x] End-to-end: binary verified against fixtures — exit 0/2, cli + json
      output, `--version`, threshold behavior

## Milestone 2 — Parity

- [x] `--ignore`/-E (rule code or issue type, case-insensitive),
      `--include-notices` / `--include-warnings` (pa11y default: errors only —
      warnings/notices are dropped unless opted in)
- [x] `--root-element` / `--hide-elements` (implemented as axe.run context
      include/exclude; hide-elements comma-split)
- [x] Cookies / headers / basic auth (`--add-cookie name=value`,
      `--add-header 'Name: value'`, `--basic-auth user:pass` → Authorization
      header); wire-level integration test asserts all four server-side
- [x] `--viewport WxH` (default 1280x1024, pa11y default), `--user-agent`
- [x] `csv` reporter (target,type,code,message,context,selector)
- [x] Actions DSL (HLD §13): internal/actions parser + chromedp executor;
      parse fails fast before browser launch; set/check dispatch input+change
      events; integration test covers click → DOM mutation → recheck
- [x] `--screen-capture` (full-page PNG after axe; cmd rejects multi-target)
- [x] stdin target (`-` → temp .html file, expanded in cmd)

## Milestone 3 — CI-grade

- [x] Multi-URL: repeated args + config `urls` array with per-URL overrides
      (pa11y-ci shape: `defaults` + `urls`); per-URL thresholds drive exit code
- [x] `--sitemap` / `--sitemap-find` / `--sitemap-replace` / `--sitemap-exclude`
      (urlset + sitemapindex recursion, depth-limited)
- [x] Tab-pool concurrency (`--concurrency`, CLI default 4, lib default 1;
      per-URL failure non-fatal; 8-URL integration test with per-target
      overrides)
- [x] `sarif` reporter (2.1.0; rules deduped, ruleIndex, level mapping)
- [x] `junit` reporter (suite per target, failure per issue, error for failed
      targets, passing case for clean targets)
- [x] Baseline mode (HLD §9): `--baseline`, `--baseline-write`,
      `--baseline-update`, `--baseline-ratchet`; fingerprint =
      sha256(target|code|selector|ws-normalized context); records axe version,
      warns on mismatch; e2e-verified new/fixed/prune/ratchet flows
- [x] `hatchet browser install`: pinned chrome-headless-shell 131.0.6778.204
      from Chrome-for-Testing into `~/.cache/hatchet/`; Discover() falls back
      to it; verified: real 107MB download, extract, and a working check run.
      Checksums still TODO (milestone 4, HLD OQ5)

## Milestone 4 — Ship

- [x] goreleaser config (linux/macos/windows × amd64/arm64, ldflags version,
      brews → catdevman/homebrew-tools) — **config written but not exercised**:
      run `goreleaser check` + `--snapshot` before the first real release
- [x] Homebrew tap (via goreleaser `brews`; requires creating the
      catdevman/homebrew-tools repo on GitHub)
- [x] Static renderer (`--renderer static`) — implemented as
      script-execution-disabled navigation (see Decisions Log), integration
      test proves page JS is skipped while axe still runs
- [x] `--axe-path` (alternate/locale axe builds; validated before launch)
- [x] Docs (README: install, quick start, CI recipes for GitHub
      Actions/GitLab, flag table, config/baseline/sitemap examples, library
      usage, pa11y migration notes)
- [x] Parity test suite vs pa11y-ci on shared fixtures — **needs Node/npm**,
      run manually: `npx pa11y-ci` with axe runner vs `hatchet` on
      `pkg/hatchet` fixture pages, diff issue codes
- [x] Pin update workflow (`scripts/update-pins.sh axe|shell <version>`);
      install checksums verified when pinned (linux64 pinned; other platforms
      warn-unverified until the script is run for them)

## Post-v1 (explicitly deferred)

- [ ] Docker image with headless shell baked in
- [ ] GitHub Action
- [ ] cosign signing, SBOM, provenance
- [ ] Choose a license for the repo itself (user decision — ask)

---

## Decisions Log

Decisions made during implementation, beyond/refining the HLD:

- **2026-07-05** Public types (`Issue`, `Result`, `Options`) live in
  `pkg/hatchet`; `internal/runner` returns raw axe result structs and
  `pkg/hatchet` maps them. This avoids import cycles without aliasing internal
  types. `internal/reporter` and `cmd` may import `pkg/hatchet`; other
  internals must not.
- **2026-07-05** The `Renderer` interface from HLD §4 is deferred until the
  static renderer (milestone 4); milestone 1 wires browser→runner directly to
  avoid speculative abstraction.
- **2026-07-05** axe has no "notice" output (violations→error,
  incomplete→warning only), so `--include-notices` is accepted but currently a
  no-op — document this in README (HLD already flags it in §11 mapping).
- **2026-07-05** JSON reporter uses hatchet's own multi-target schema (not
  pa11y's bare issue array) — the schema carries `schemaVersion` per HLD §8.
- **2026-07-05** Subcommand routing (`hatchet browser install`) uses stdlib
  `flag` + manual dispatch for now; revisit if flag surface outgrows it.
- **2026-07-05** Tabs share one browser profile, so cookies set via
  `--add-cookie` are visible across concurrent targets. True isolation needs
  CDP incognito contexts (Target.createBrowserContext) — deferred; only
  matters when same-domain targets need *different* cookies.
- **2026-07-05** JUnit reporter marks every reported issue (incl. warnings) as
  a failing testcase — pass/fail for the run is the exit code's job; the
  reporter just surfaces issues in CI UIs.
- **2026-07-05** `--baseline-write` reports normally but always exits 0
  (accepting current state is never a failure).
- **2026-07-05** Per-URL config overrides beat CLI flags for that URL (pa11y-ci
  semantics); CLI flags beat config `defaults`. `level` and reporters stay
  run-level. CLI positional args or `--sitemap` take precedence over config
  `urls`.
- **2026-07-05** `internal/baseline` also imports `pkg/hatchet` (same
  exception as `internal/reporter`) — CLAUDE.md rule updated.
- **2026-07-05** Static renderer implementation differs from the HLD's
  original "Go-fetch then load" sketch with identical semantics:
  `Emulation.setScriptExecutionDisabled(true)` during document parse (page
  scripts are skipped permanently — re-enabling never revisits them), then
  re-enabled after load because axe's promise needs a live event loop
  (first attempt left scripts disabled and died with "Promise was
  collected"). HLD §6.2 amended.
- **2026-07-05** Headless-shell downloads are sha256-verified when the
  platform has a pinned sum (linux64 currently); unpinned platforms install
  with a warning. `scripts/update-pins.sh shell <ver>` prints the full
  checksum table (downloads all 4 platforms).
