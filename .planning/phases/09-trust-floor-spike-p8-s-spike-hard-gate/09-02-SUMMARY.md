---
phase: 09-trust-floor-spike-p8-s-spike-hard-gate
plan: 02
subsystem: history-projection
tags: [go, eino, history-sanitization, trust-floor, spike, GEVL-08]

requires:
  - phase: 09-trust-floor-spike-p8-s-spike-hard-gate
    provides: "Plan 09-01 75-row history-injection golden set and spike row contract"
  - phase: 03
    provides: "Go-native Eino v0.9.14 dependency and trust-evaluation conventions"

provides:
  - "Project(prev []store.SessionMessage) []schema.Message enforcing D9-B1, D9-B2, and D9-B3"
  - "Seven exported projection fixtures covering three positive and four negative edge cases"
  - "Phase 10-safe history contract without chat pipeline or store integration"

affects: [phase 09-03, phase 10, MEM-01, MEM-02, MEM-03, MEM-04, MEM-05, GEVL-08]

tech-stack:
  added: []
  patterns:
    - "Pure read-time history projection with explicit role whitelist and text-only schema mapping"
    - "Per-user-message ordered chunk-marker replacement using one case-insensitive regular expression"
    - "Non-test fixture contract file consumed by later phase test generation"

key-files:
  created:
    - path: "internal/history/projection.go"
      role: "D9-B1/B2/B3 projection implementation"
    - path: "internal/history/projection_test_fixtures.go"
      role: "Exportable positive/negative projection contract fixtures"
    - path: ".planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-02-SUMMARY.md"
      role: "Plan completion record"
  modified: []

key-decisions:
  - "B1 discards every role outside user/assistant and maps only Role and Content into Eino messages."
  - "B2 scans only user content and uses [REF-N] without length padding; assistant citations remain untouched."
  - "B3 retains at most the final six filtered messages and removes a leading orphan assistant."
  - "The fixture file deliberately uses package history rather than history_test and is a contract handoff, not a unit test."

requirements-completed: [GEVL-08]

patterns-established:
  - "Projection is a pure function with no store, SQLite, chat, or agent dependencies."
  - "Phase 10 can consume Fixtures directly and translate them into projection_test.go tests."

metrics:
  duration: "3m10s"
  completed: "2026-08-18T02:54:14Z"
  tasks: 2
  files: 3
  commits: 2

---

# Phase 9 Plan 2: History projection sanitization and fixtures Summary

**A pure Eino history projection now drops non-chat roles, replaces user-only chunk markers, and bounds history to three user/assistant pairs, with seven executable contract fixtures ready for Phase 10.**

## Performance

- **Duration:** 3m10s (2026-08-18T02:51:04Z to 2026-08-18T02:54:14Z)
- **Started:** 2026-08-18T02:51:04Z
- **Completed:** 2026-08-18T02:54:14Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Implemented `history.Project` with explicit D9-B1 role/field filtering, D9-B2 user-only ordered marker replacement, and D9-B3 six-message tail limit with orphan-assistant removal.
- Exported `Fixture` and `Fixtures` data with three positive and four negative cases, including tool/system-role exclusion, assistant citation preservation, truncation, and orphan handling.
- Kept the package isolated from chat/agent/store integration; Phase 10 can consume the same function and fixtures.

## Task Commits

Each task was committed atomically:

1. **Task 1: `projection.go` — Project() skeleton + three rules** - `7aa8a3c` (feat)
2. **Task 2: `projection_test_fixtures.go` — 7 positive/negative fixtures** - `7fe80db` (feat)

**Plan metadata:** `9b23b3f` (state, roadmap, requirements).

## Files Created/Modified

- `internal/history/projection.go` — Pure D9-B1/B2/B3 projection contract; 78 lines, Apache-2.0 header, Eino `schema.Message` mapping.
- `internal/history/projection_test_fixtures.go` — Exported fixture contract data; 150 lines, seven cases, Phase 10 test handoff comment.
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-02-SUMMARY.md` — This completion record.

## Decisions Made

- `Project` uses a single case-insensitive regex for `[CHUNK id]`, `[CH-xxx]`, and `[id:N]`, numbering replacements from 1 per user message.
- `[REF-N]` intentionally is not padded to the source marker length, as required by the Phase 9 planner ruling; `Normalize` later collapses whitespace.
- The fixture file is intentionally not recognized as a Go `_test.go` file, so it is available to Phase 10 without expanding this spike's scope.

## Verification

- `go build ./internal/history/...` — PASS.
- `go vet ./internal/history/...` — PASS.
- `go test ./internal/history/...` — PASS; package reports `[no test files]`.
- `go list -f '{{.TestGoFiles}}' ./internal/history` — `[]`.
- `gofmt -l internal/history/` — empty.
- Fixture gates — 7 unique names, 3 positive, 4 negative, 7 inputs, 7 expected outputs, 0 `func Test` entries.
- Production-path scope check — no diff in `internal/server/`, `internal/agent/`, or `cmd/`.

## Deviations from Plan

None - plan executed exactly as written. No architecture, auth, or package-install deviations were encountered.

## Threat Posture

The plan's threat register is fully addressed by the implementation: tool/system roles are discarded, assistant metadata is not copied, user chunk markers are replaced, and fixtures lock each trust boundary. This plan introduces no new network endpoint, file access pattern, auth path, or schema trust boundary beyond the declared projection trust boundary.

## Known Stubs

None. `scrubChunkRefs` is the complete minimal helper required by D9-B2; no placeholder, TODO, FIXME, empty-return, or unimplemented contract exists in the created source files.

## Issues Encountered

None. The repository's `gsd-sdk` executable was not on PATH, so execution used the installed local GSD SDK CLI entrypoint at `C:/Users/CPIC/AppData/Roaming/nvm/v22.17.1/node_modules/@gsd-build/sdk/dist/cli.js`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for Plan 09-03. The spike runner can now consume `history.Project` and its exported fixtures; the package remains deliberately isolated from chat integration until the Phase 9 hard gate passes and Phase 10 begins.

## Self-Check: PASSED

All two task commits and all three created files were found on disk; the claimed acceptance gates passed before closeout.

---
*Phase: 09-trust-floor-spike-p8-s-spike-hard-gate*
*Completed: 2026-08-18*
