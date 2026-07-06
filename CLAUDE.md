# Hatchet

Single-binary accessibility testing CLI in Go — a pa11y/pa11y-ci replacement
that drives headless Chrome and runs the embedded axe-core rule engine.

## Start here

1. **`TASKS.md`** — current status, milestone checklists, decisions log.
   Sessions may switch between models/providers; this file is the handoff.
   Update it as you complete work.
2. **`HLD.md`** — the reviewed design. Don't deviate structurally without
   recording why in TASKS.md's Decisions Log.

## Commands

- Build: `go build ./...` · Binary: `go build -o hatchet ./cmd/hatchet`
- Test: `go test ./...` — integration tests self-skip when Chrome is absent
- Local Chrome: `/usr/bin/google-chrome-stable` (may need `--no-sandbox` in
  containers)

## Architecture rules

- `cmd/hatchet` is a thin wrapper: flag parsing and wiring only; all
  capability lives in `pkg/hatchet` (keeps the library embedding story free).
- Public types (`Issue`, `Result`, `Options`, `Target`) live in `pkg/hatchet`.
  `internal/*` packages must not import `pkg/hatchet`, except
  `internal/reporter` and `internal/baseline` (which consume
  `[]hatchet.Result`). `internal/runner` returns raw axe structs;
  `pkg/hatchet` maps them to `Issue`s.
- axe-core is vendored and pinned in `third_party/axe/` (MPL-2.0 — keep the
  file unmodified and the LICENSE alongside). Version bumps are deliberate,
  never drive-by.
- Exit codes are contract: 0 clean, 2 issues ≥ `--level` over `--threshold`,
  1 operational error.
