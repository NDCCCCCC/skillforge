---
phase: 48-ci
plan: 16
subsystem: ci
tags: [github-actions, sigpipe, pipefail, vitest, jsdom, dockerfile, glibc, duckdb, cgo]

requires:
  - phase: 48-ci
    provides: "the four-job ci.yml matrix, verify-phase-48.sh partitions, and the round-3 verification report classifying the three persistent reds"
provides:
  - "a deterministic, pipe-free P5a determinism guard whose P2 pin lands in the same commit"
  - "a platform-portable Sparkline dependency test"
  - "a glibc-only backend image that can link the static duckdb archive"
affects: [48-17, 48-18, 48-19, phase-51]

actuals:
  tokens: 2914
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Materialise producer output to a file and grep the file; never pipe into grep -q under pipefail"
    - "Resolve colocated test fixtures through import.meta.url using node:url's URL class (jsdom replaces the global)"
    - "CGO dependencies with glibc symbol references force glibc on both build and runtime stages"

key-files:
  created: []
  modified:
    - ".github/workflows/ci.yml"
    - "verify-phase-48.sh"
    - "web/src/components/chat/parts/Sparkline.test.tsx"
    - "Dockerfile.backend"

key-decisions:
  - "The P5a guard writes go test -list output to $RUNNER_TEMP/go-test-list.txt and greps that file; no pipeline remains, so no SIGPIPE/exit 141 is possible"
  - "The P2 verifier pin changes in the same commit as the workflow shape it gates, so every commit in this plan keeps bash verify-phase-48.sh green"
  - "The Sparkline test imports URL from node:url because jsdom's global URL class is rejected by Node's fs when passed to readFileSync"
  - "CICD-01/02/03 remain Pending: the source fixes are proven locally, but only 48-19's real four-green run closes the requirements"

patterns-established:
  - "No-pipe determinism guard: materialised list file + grep on the file"
  - "Module-relative file reads in vitest/jsdom via node:url"

requirements-completed: []

coverage:
  - id: D1
    description: "P5a determinism guard has no producer/consumer pipe and cannot exit 141 on Linux; P2 pins the materialised shape"
    requirement: "CICD-01"
    verification:
      - kind: integration
        ref: "WSL Ubuntu-22.04 go1.25.5: go test -list > file && grep -qx on the file => MATERIALISED-LIST-OK"
      - kind: unit
        ref: "bash verify-phase-48.sh => PASS P2 go-line (materialised list)"
    human_judgment: false
  - id: D2
    description: "TestSparklineNoDeps resolves its subject from import.meta.url with no drive-letter path; four assertions unchanged"
    requirement: "CICD-02"
    verification:
      - kind: unit
        ref: "web: npx vitest run src/components/chat/parts/Sparkline.test.tsx => 4 passed"
      - kind: integration
        ref: "web: npx vitest run => 85 files / 689 tests passed"
      - kind: unit
        ref: "grep for drive-letter paths in Sparkline.test.tsx => SPARKLINE-PORTABLE-OK"
    human_judgment: false
  - id: D3
    description: "Dockerfile.backend builds and ships on glibc; Dockerfile.frontend untouched"
    requirement: "CICD-03"
    verification:
      - kind: other
        ref: "static grep assertions => DOCKERFILE-GLIBC-OK; git diff --quiet HEAD -- Dockerfile.frontend => FRONTEND-UNTOUCHED-OK"
      - kind: unit
        ref: "bash verify-phase-48.sh => PASS P7 docker-line"
    human_judgment: false
  - id: D4
    description: "The three published CI lines (go / fe / docker) are green on a real GitHub Actions run"
    verification: []
    human_judgment: true
    rationale: "Docker is absent from this host and only a real runner can exercise the Linux link and the published check; the run is owned by 48-19"

duration: 9min
completed: 2026-10-20
status: complete
---

# Phase 48 Plan 16: Close the Three Persistent Source-Side Reds (SC-1 / SC-2 / SC-3) Summary

**The P5a determinism guard no longer pipes into `grep -q`, the Sparkline test resolves its subject from the test module, and the backend image is built and shipped on glibc — the three causes that kept every published CI line red.**

## Performance
- **Duration:** 9 min
- **Started:** 2026-10-20T05:07:20Z
- **Completed:** 2026-10-20T05:16:06Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Removed the only SIGPIPE source in the go line: the `-list` output is materialised to `$RUNNER_TEMP/go-test-list.txt` and grepped there, so `grep -q` can no longer close a pipe the producer is still writing to.
- Landed the P2 verifier pin in the same commit as the `ci.yml` shape change: the gate now requires the materialised list and rejects a pipe after `-list`, so no commit in this plan was red.
- Made `TestSparklineNoDeps` platform-independent by resolving `Sparkline.tsx` from `import.meta.url`; all four dependency assertions and the test name are unchanged and the full suite is 689/689.
- Moved both `Dockerfile.backend` stages to glibc (`golang:1.25-bookworm`, `debian:bookworm-slim`) with `libstdc++6` and `libgcc-s1`, so the static duckdb archive's `backtrace` / `backtrace_symbols` / `malloc_trim` / `__res_init` references resolve.
- Kept every gate intact: no step removed, no `continue-on-error`, no `|| true`, no step- or job-level `if:`, no skipped test, no build step skipped.

## Task Commits
Each task was committed atomically:
1. **Task 1: remove the pipe from the determinism guard and land its P2 pin** - `ea4185e` (fix)
2. **Task 2: resolve the Sparkline component path from the test module** - `886dec9` (fix)
3. **Task 3: move the backend image to a glibc base and runtime** - `93b01c8` (fix)

**Plan metadata:** committed immediately after this file (`docs(48-16): complete ...`).

## Files Created/Modified
- `.github/workflows/ci.yml` - P5a step materialises the `-list` output and greps the file; docker job timeout comment now names `golang:1.25-bookworm`
- `verify-phase-48.sh` - P2 requires `go-test-list.txt` and `grep -qx 'TestDeriveEdgesDeterministic'`, rejects a pipe after `-list`, and no longer requires `set -o pipefail`; partition descriptions updated to match
- `web/src/components/chat/parts/Sparkline.test.tsx` - `TestSparklineNoDeps` reads the colocated component via `new NodeURL('./Sparkline.tsx', import.meta.url)`
- `Dockerfile.backend` - builder `golang:1.25-bookworm`; runtime `debian:bookworm-slim` with ca-certificates, tzdata, libstdc++6, libgcc-s1

## Decisions Made
- The determinism guard is fixed by materialising the list rather than dropping `-q`: both lose the SIGPIPE, but the file form keeps the rename-proof `grep -qx` semantics exactly, which is what R2-WR-04 requires.
- The P2 pin was rewritten in the same commit as `ci.yml`; this is the only ordering that keeps `bash verify-phase-48.sh` green at every commit in the plan.
- CICD-01/02/03 were deliberately **not** marked complete. Only a real four-green run closes them, and that run belongs to 48-19.
- Docker remains unbuildable on this host, so the image change is proven statically (`DOCKERFILE-GLIBC-OK`, gate P7) and by inspection of the exact glibc references recorded in `48-VERIFICATION.md`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] jsdom's global URL class is rejected by Node's fs**
- **Found during:** Task 2
- **Issue:** The plan's literal expression `readFileSync(new URL('./Sparkline.tsx', import.meta.url))` fails under vitest's jsdom environment with `TypeError: The URL must be of scheme file`. `import.meta.url` is a correct `file:///...` URL; the problem is that jsdom replaces the global `URL` class, and Node's `fs` does not accept that instance.
- **Fix:** Import `URL` from `node:url` (`const { URL: NodeURL } = await import('node:url')`) and construct the same module-relative URL with it. The path is still derived from `import.meta.url`, is still relative to the test module, and contains no drive-letter literal.
- **Files modified:** `web/src/components/chat/parts/Sparkline.test.tsx`
- **Verification:** targeted run 4/4, full suite 85 files / 689 tests passed, `SPARKLINE-PORTABLE-OK`.
- **Commit:** `886dec9`

**2. [Rule 2 - Documentation accuracy] Stale `pipefail` descriptions in the verifier**
- **Found during:** Task 1
- **Issue:** After replacing the P2 requirement, three descriptive strings in `verify-phase-48.sh` (header, `done_partition` message, banner) still claimed the determinism guard ran "under pipefail". Leaving them would make the gate's own output describe a shape it no longer requires.
- **Fix:** Reworded them to say the guard runs against a materialised list. No assertion logic beyond the intended P2 change was altered.
- **Files modified:** `verify-phase-48.sh`
- **Verification:** `bash verify-phase-48.sh` exits 0 with `ALL PASSED`; P11 still finds exactly one banner site.
- **Commit:** `ea4185e`

**Total deviations:** 2 auto-fixed (1 bug, 1 documentation accuracy). **Impact:** Both are local to the plan's own files; neither weakens a gate nor changes any runner command semantics beyond the intended fix.

## Issues Encountered
- The plan's WSL verify command could not be passed through Git Bash as a single quoted argument (`$(mktemp)` was lost in the interop layer). It was run as a heredoc piped to `wsl.exe ... bash -s`, with the same commands and the same Linux Go 1.25.5, and exited 0.

## User Setup Required
None.

## Next Phase Readiness
- 48-17 can harden the remaining verifier partitions against the classes this plan's fixes belong to.
- 48-18 can reconcile the three records that still carry the false "runner environment" root cause for P5a (this plan deliberately did not edit `.planning/` records).
- 48-19 is the only plan that can close CICD-01/02/03: push `main` once and confirm the go, fe and docker jobs are green and that the frontend image build is reached for the first time.

---
*Phase: 48-ci*
*Completed: 2026-10-20*
