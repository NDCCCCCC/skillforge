---
phase: 06-eval-dashboard-production-hardening
plan: 01
subsystem: testing
tags: [go, eval, ci-gate, reproducibility, sha256, ragas-metrics, frozen-set, github-actions]

# Dependency graph
requires:
  - phase: 03-agentic-retrieval-trust-evaluation
    provides: "internal/eval (4 metrics, Judge, LoadGolden, Calibrate, TestRunnerReproducibility), cmd/eval CLI, store.AuditRow.PromptHash, internal/server/chat.go::computePromptHash"
  - phase: 02-skill-engine
    provides: "skills/compliance/SKILL.md + skill.Loader (the manifest the frozen lock's prompt body is rendered from)"
provides:
  - "internal/reprohash.PromptHash — the ONE canonical reproducibility hash, shared by internal/server and internal/eval"
  - "Strengthened computePromptHash(skillName, modelVersion, renderedPromptBody) — Q1 resolved; the frozen prompt_hash pin is now meaningful"
  - "agent.RenderedSystemPrompt() + agent.RenderSystemPrompt(m) — the rendered prompt body as a first-class value"
  - "internal/eval.LockFile + LoadLock + AssertPin + PromptHashFor + ActiveModelVersion (SC-6 pin machinery)"
  - "internal/eval.WriteArchive + LoadIndex + IndexRun + EvalIndex (D6-02 trend history; the dashboard's data source)"
  - "internal/eval.gateDecision (pure) + GateDecision + FailedSkills + LoadThresholds + FormatGateMarkdown + LoadReport (GEVL-07 gate logic)"
  - "internal/eval.crossEvalDiff (pure) + CrossEvalDiff + CrossEvalDelta (D6-14 self-eval-bias detection)"
  - "eval.Report.Skill field (additive, omitempty) — lets the gate name the failing Skill and the index group per Skill"
  - "cmd/eval --archive / --frozen / --judge-models / --gate-check / --skills-dir flag matrix"
  - "eval/frozen/compliance.jsonl + compliance.lock.json (D6-13 pinned baseline)"
  - "skills/compliance/eval-gate.json at the Q2-resolved path (D6-10)"
  - "Two-layer SC-6 tolerance: strict 1e-6 (hermetic) + live 1e-3 (//go:build liveeval)"
affects: [06-02-eval-api-surface, 06-03-ci-gate-workflow, 06-04-eval-dashboard-frontend, 07-open-source-packaging]

# Tech tracking
tech-stack:
  added: []  # ZERO new Go dependencies — as the phase research required
  patterns:
    - "Leaf shared-hash package (internal/reprohash) to break a would-be import cycle between two consumers of one canonical value"
    - "Pure decision function + thin exported seam (gateDecision / GateDecision) — keeps logic + tests in-package while a main package can call it"
    - "Self-verifying data artifact: the shipped .lock.json is regression-tested against the live render path (TestShippedFrozenLockIsInSync)"
    - "Build-tag-isolated live tests (//go:build liveeval) so quota-burning tests are not even compiled by the default suite"
    - "Atomic file append via tmp+rename with an explicitly documented multi-process limitation"

key-files:
  created:
    - internal/reprohash/reprohash.go
    - internal/eval/frozen.go
    - internal/eval/frozen_test.go
    - internal/eval/archive.go
    - internal/eval/archive_test.go
    - internal/eval/gate.go
    - internal/eval/gate_test.go
    - internal/eval/crosseval.go
    - internal/eval/eval_live_test.go
    - internal/server/chat_hash_test.go
    - cmd/eval/main_test.go
    - eval/frozen/compliance.jsonl
    - eval/frozen/compliance.lock.json
    - skills/compliance/eval-gate.json
  modified:
    - internal/server/chat.go
    - internal/agent/agent.go
    - internal/eval/dataset.go
    - internal/eval/eval_test.go
    - cmd/eval/main.go

key-decisions:
  - "Q1 resolved via option (a) STRENGTHEN: computePromptHash now covers the rendered prompt body. Pre-existing audit rows keep stale hashes by design (T-06-02) — prompt_hash is a reproducibility key, not a lookup key, and rows are INSERT-only."
  - "The hash lives in a NEW leaf package internal/reprohash rather than being imported across internal/server ↔ internal/eval or duplicated. Two copies could silently drift, and a drifted pin fails GREEN — the worst failure mode for a gate."
  - "Q2 resolved: skills/compliance/eval-gate.json (product Skill), NOT .claude/skills/ (this repo's meta-skills). Matches the D6-09 CI path filter."
  - "gateDecision stays unexported (pure fn + in-package tests) with a thin exported GateDecision seam, because cmd/eval is a different package and the plan named both the unexported fn and a CLI caller."
  - "eval.Report gained an additive `skill` omitempty field — Report had no Skill field, but gateDecision must name the failing Skill (SC-2) and IndexRun must group per Skill (D6-02). Legacy reports fall back to DefaultGateSkill."
  - "--judge-models selects the per-judge model via os.Setenv(JUDGE_MODEL) before constructing each judge, reusing NewGLMChatModel rather than duplicating chat-model wiring. Safe: the CLI is single-threaded batch."
  - "Audit backfill is SKIPPED under cross-eval — N judges scoring the same session would each overwrite the previous judge's metrics, leaving the row holding whichever judge ran last."
  - "The primary (first) judge owns eval/report.json + calibration.json; secondary judges publish only under their own archive dir, so cross-eval cannot overwrite the headline numbers."
  - "crossEvalDiff compares against threshold+1e-9 — 0.92-0.87 computes to 5.000000000000004, and a >5% alarm firing on the 15th decimal place trains operators to ignore it."
  - "A malformed eval-gate.json is a hard error, not a fallback to 0.9 — silently defaulting would let a typo quietly lower a Skill's quality bar."
  - "A corrupt INDEX.json STOPS the archive rather than being reinitialized, so trend history is never silently erased."

patterns-established:
  - "Canonical-hash leaf package: one definition, length-prefixed fields, both consumers import it"
  - "Data artifacts are generated THROUGH the production code path and then regression-tested against it (never hand-copied)"
  - "Scoring vs gating separation: the eval CLI emits data and exits 0; only --gate-check exits non-zero on quality regression"
  - "Two-layer numeric tolerance: hermetic-strict proves determinism, live-jitter proves practical stability"

requirements-completed: [GEVL-07]

# Metrics
duration: 42min
completed: 2026-08-13
---

# Phase 06 Plan 01: Reproducibility & Gate-Logic Substrate Summary

**Frozen-eval-set pin machinery (SHA-256 over the rendered prompt body), file-based trend history, and a pure per-Skill promotion-gate function — the substrate Plans 06-02/03/04 all consume.**

## Performance

- **Duration:** ~42 min
- **Tasks:** 3 (all committed atomically)
- **Files created:** 14
- **Files modified:** 5
- **New dependencies:** 0

## Accomplishments

- **Q1 resolved (the blocking one).** `computePromptHash` covered only `(skillName, modelVersion)`, so editing a Skill's prompt body without renaming it produced an *unchanged* hash. The frozen set's `prompt_hash` pin — the heart of SC-6 — would have passed green over a drifted system. The rendered body is now part of the digest, and the regression is pinned in both packages.
- **SC-6 pin machinery.** `LockFile` / `LoadLock` / `AssertPin` detect `model_ver`, `skill_name` and `prompt_hash` drift with named, actionable errors, and `cmd/eval --frozen` asserts the pin *before* spending a token.
- **D6-02 trend history.** `WriteArchive` writes the full report plus a flat `IndexRun` into `INDEX.json` via tmp+rename. `refusal_accuracy` is derived from the existing `refusal-required` category — no new metric.
- **GEVL-07 gate logic.** `gateDecision` is a pure function; `FormatGateMarkdown` produces the PR comment naming the Skill and its threshold (SC-2). `cmd/eval --gate-check` runs it without an LLM, a retriever, or an API key.
- **D6-14 made executable.** `crossEvalDiff` flags >5% cross-judge faithfulness deltas — the exact signal `eval/README.md` documented as prose.
- **Q2 resolved.** `skills/compliance/eval-gate.json`, co-located with the product Skill.
- **Two-layer SC-6 gate.** Strict 1e-6 hermetic; live 1e-3 behind `//go:build liveeval`, verified absent from the default suite.

## Task Commits

1. **Task 1: Q1 — strengthen computePromptHash + frozen.go (LockFile/LoadLock/AssertPin)** — `f0e367d` (feat)
2. **Task 2: archive.go + gate.go + crosseval.go + tests** — `a056c89` (feat)
3. **Task 3: cmd/eval flag matrix + frozen data files + tolerance + live guard** — `bbef53d` (feat)

## Files Created/Modified

**Created**
- `internal/reprohash/reprohash.go` — the one canonical `PromptHash`; length-prefixed fields prevent concatenation collisions
- `internal/eval/frozen.go` — `LockFile`, `LoadLock`, `AssertPin`, `PromptHashFor`, `ActiveModelVersion`, the two tolerance constants
- `internal/eval/archive.go` — `WriteArchive`, `LoadIndex`, `IndexRun`, `EvalIndex`, `RefusalAccuracy`
- `internal/eval/gate.go` — `gateDecision` (pure), `GateDecision`, `FailedSkills`, `LoadThresholds`, `FormatGateMarkdown`, `LoadReport`
- `internal/eval/crosseval.go` — `crossEvalDiff` (pure), `CrossEvalDiff`, `CrossEvalDelta`
- `internal/eval/{frozen,archive,gate}_test.go` — 30+ cases incl. the Q1 regression and the shipped-artifact sync checks
- `internal/eval/eval_live_test.go` — the `liveeval`-tagged jitter layer
- `internal/server/chat_hash_test.go` — server-side Q1 regression + `reprohash` delegation check
- `cmd/eval/main_test.go` — flag matrix, gate-check exit codes, shipped gate-file validity
- `eval/frozen/compliance.{jsonl,lock.json}` — the pinned 50-case baseline + its sidecar
- `skills/compliance/eval-gate.json` — `{"faithfulness_threshold": 0.9}`

**Modified**
- `internal/server/chat.go` — 3-arg `computePromptHash` delegating to `reprohash`; new `renderedPromptBody` resolver; migration note replacing the stale Phase-5 deferral comment
- `internal/agent/agent.go` — exported `RenderedSystemPrompt()` + `RenderSystemPrompt(m)`
- `internal/eval/dataset.go` — additive `Report.Skill`
- `internal/eval/eval_test.go` — tolerance 1e-9 → `StrictReproducibilityTolerance` (1e-6), with the rationale documented
- `cmd/eval/main.go` — flag matrix, `runGateCheck`, per-case loop extracted to `runEvalPass`, exit-code contract stated explicitly

## Decisions Made

See `key-decisions` in the frontmatter. The three worth reading in prose:

1. **A new leaf package for the hash.** There was no import cycle today (`internal/server` and `internal/eval` don't reference each other), so `internal/eval` *could* have imported `internal/server`. That would have dragged the whole Hertz HTTP surface into the eval library for one function. Duplicating the function was the other option, and it's worse: two copies that drift produce a pin that passes green while the system has moved — precisely the failure this machinery exists to prevent. A leaf package gives one definition with no cycle risk in either direction.

2. **The lock file was generated through the production path, then deleted the generator.** A one-off `tools/genfrozenlock` loaded the real Skill, rendered the real prompt via `agent.RenderSystemPrompt`, and hashed via `eval.PromptHashFor`. Byte-identity with `AssertPin`'s re-computation is therefore true by construction, not by careful copy-pasting. `TestShippedFrozenLockIsInSync` then keeps it true.

3. **`gateDecision` unexported + `GateDecision` exported.** The plan named an unexported pure function *and* a `cmd/eval` caller, which is impossible across packages. Keeping the logic and its tests in-package while exposing a one-line seam satisfies both, and keeps the gate's logic from leaking into a `main` package where it can't be unit-tested.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 — Missing Critical] `eval.Report` had no `Skill` field**
- **Found during:** Task 2 (gate.go)
- **Issue:** The plan says gateDecision "generalizes by reading report.Skill", but `Report` is `{Overall, PerCategory, Cases}` — no such field. Without it the gate cannot name the failing Skill, which SC-2 explicitly requires.
- **Fix:** Added `Skill string \`json:"skill,omitempty"\`` to `Report`. Additive and omitempty, so pre-Phase-6 reports still parse; `gateDecision` falls back to `DefaultGateSkill`.
- **Files modified:** `internal/eval/dataset.go`, `cmd/eval/main.go` (sets `report.Skill`)
- **Verification:** `TestGateDecision_NamesTheSkill` covers both the populated and legacy shapes.
- **Committed in:** `a056c89`

**2. [Rule 1 — Bug in planned code] `crossEvalDiff` flagged an exactly-5% delta**
- **Found during:** Task 2 (boundary test)
- **Issue:** `0.92 - 0.87` is `0.050000000000000044` in float64, so `*100 > 5.0` was true and a delta of exactly 5% tripped a ">5%" alarm.
- **Fix:** Compare against `deltaThresholdPct + 1e-9` (named `crossEvalEpsilon`, with the rationale in a comment). Fixed in the production function rather than by loosening the test.
- **Files modified:** `internal/eval/crosseval.go`, `internal/eval/gate_test.go`
- **Verification:** `TestCrossEvalDiff_NoFlag/exactly_5pct_is_not_>5pct` now passes; the 6% case still flags.
- **Committed in:** `a056c89`

**3. [Rule 2 — Missing Critical] `gateDecision` unexported but called from `cmd/eval`**
- **Found during:** Task 2/3
- **Issue:** The plan specified the pure fn as `gateDecision` (unexported) *and* required `cmd/eval --gate-check` to call it. Go forbids that across packages.
- **Fix:** Kept `gateDecision` unexported with its tests in-package; added the thin exported `GateDecision` (and `CrossEvalDiff`) seams for the CLI.
- **Files modified:** `internal/eval/gate.go`, `internal/eval/crosseval.go`
- **Verification:** Tests call the unexported fn by name; `cmd/eval` builds against the exported seam.
- **Committed in:** `a056c89` / `bbef53d`

**4. [Rule 3 — Blocking] `cmd/eval` main() could not loop over judges**
- **Found during:** Task 3
- **Issue:** `--judge-models` requires running the whole per-case loop once per judge, but the loop was inline in `main()`.
- **Fix:** Extracted it verbatim into `runEvalPass(ctx, evalPass)` (params bundled in a struct — a 10-positional-arg call would be unreadable). No metric logic changed.
- **Files modified:** `cmd/eval/main.go`
- **Verification:** `go build ./...`, `go vet`, full suite green.
- **Committed in:** `bbef53d`

**5. [Rule 2 — Missing Critical] Extra test files beyond the plan's file list**
- **Found during:** Tasks 1 and 3
- **Issue:** The plan's verify commands require `go test ./internal/server/ -run TestComputePromptHash` and `go test ./cmd/eval/`, but neither `internal/server/chat_hash_test.go` nor `cmd/eval/main_test.go` was in `files_modified`. Without them those verifies pass vacuously ("no tests to run" exits 0).
- **Fix:** Created both, plus `TestShippedFrozenLockIsInSync` / `TestShippedFrozenSetLoads` to make the shipped data artifacts self-verifying.
- **Files modified:** `internal/server/chat_hash_test.go`, `cmd/eval/main_test.go`, `internal/eval/frozen_test.go`
- **Verification:** All green; the verifies now actually assert something.
- **Committed in:** `f0e367d` / `bbef53d`

---

**Total deviations:** 5 auto-fixed (2 missing critical, 1 planned-code bug, 1 blocking, 1 test coverage)
**Impact on plan:** All five were required for the plan's own acceptance criteria to be satisfiable. No scope creep — nothing outside the three tasks was touched, and zero dependencies were added.

## Issues Encountered

- **`gofmt -l` flags nearly every file in the repo.** Pre-existing CRLF checkout condition on Windows, not caused by this plan (untouched files are equally flagged). Verified my files are correctly formatted by normalizing line endings before checking; `gofmt -w` was applied to the one file with a genuine alignment issue.
- **`systemPrompt(m)` currently ignores its manifest** (the Phase 3 prompt is a compile-time constant), so today a SKILL.md body edit would *not* move the hash. The plumbing is correct and the contract is pinned by tests — when Phase 5's Skill-driven prompt rendering starts consuming the manifest, the pin tightens automatically with no further change here. Worth knowing before relying on the pin to catch a SKILL.md edit.

## User Setup Required

None — no external service configuration. The live reproducibility layer needs `GLM_API_KEY` when explicitly run as a phase gate:
`go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility`

## Next Phase Readiness

**Ready for Wave 2:**
- **06-02 (API surface):** `EvalIndex` / `IndexRun` / `LockFile` / `LoadIndex` / `LoadReport` are the types `/api/eval/reports` and `/api/eval/frozen` will serve. Note `LoadIndex` returns an `os.IsNotExist`-inspectable error for "no history yet" — the handler should map that to an empty index, not a 500.
- **06-03 (CI workflow):** `cmd/eval --frozen=compliance --archive` then `--gate-check=eval/report.json` is the two-step the workflow needs. The gate step needs no secrets. `FormatGateMarkdown` output goes to stdout for `gh pr comment --body-file`.
- **06-04 (dashboard FE):** `INDEX.json` is the trend data source; `frozen_version` on each run plus the lock's `eval_set_version` back the D6-16 baseline overlay.

**Invariants preserved:** T-3-AUDIT INSERT-only (no UPDATE/DELETE added), T-3-SQLI (no SQL touched), D-04/D12 (chat SSE wire untouched), `cmd/eval` exit-0 contract (default mode unchanged; only `--gate-check` gates).

**Carry-forward concerns:**
- `eval/reports/` does not exist until the first `--archive` run. 06-02's handler and 06-04's UI both need an empty-state path.
- Existing audit rows carry stale `prompt_hash` values (accepted, T-06-02). If 06-02 surfaces `prompt_hash` in the audit UI, do not present it as comparable across the Phase 6 boundary.

---
*Phase: 06-eval-dashboard-production-hardening*
*Completed: 2026-08-13*
