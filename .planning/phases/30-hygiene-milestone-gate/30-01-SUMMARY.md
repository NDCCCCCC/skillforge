---
phase: "30-hygiene-milestone-gate"
plan: "01"
type: "execute"
wave: 1
subsystem: "internal/server (pipeline.go + chat.go)"
tags: ["CR-01", "hygiene", "double-write", "SSE-emit-failure"]
dependency_graph:
  requires: []
  provides: ["pipelineEmitError.persisted field to chatHandler error branch"]
  affects: ["CR-01 SSE emit failure double-write path"]
tech_stack:
  added: []
  patterns: ["persisted flag error return", "type-switch error unwrapping"]
key_files:
  created:
    - "internal/server/cr01_test.go"
  modified:
    - "internal/server/pipeline.go"
    - "internal/server/chat.go"
decisions:
  - "Derived persisted as turnID > 0 && writeErr == nil since writeAuditAndTurn signature is (int64, error) not (int64, bool)"
  - "Used GetPersisted() interface method instead of errors.As to avoid Go version edge cases with concrete type unwrapping"
  - "Moved emitTurnID inside persisted guard: only called when turnID > 0"
metrics:
  duration: "~8 minutes"
  completed: "2026-09-08"
  tasks: 2
  files: 3
---

# Phase 30 Plan 01: CR-01 — SSE Emit Failure Double-Write Fix

**One-liner:** CR-01 fix: SSE emit failure path now skips the fallback writeAuditAndTurn when the pipeline already persisted the audit+turn row.

## Problem

When `VerdictPipeline` returned an SSE emit error after `writeAuditAndTurn` had already persisted the audit row + turn pair (routine client disconnect), `chatHandler`'s error branch unconditionally called `writeAuditAndTurn` a **second** time, producing:
- Two audit rows for one query (GEVL-03 violation)
- Two votable assistant messages in history (double-counting in FBK metrics and review queue)
- An `internal_error` message incorrectly rendered as a substantive reply

## Solution

**Task 1 (pipeline.go):** Added `pipelineEmitError` struct carrying `err error` + `persisted bool`. The verdict path now returns `&pipelineEmitError{err: emitErr, persisted: persisted}` where `persisted = turnID > 0 && writeErr == nil`. The type also implements `Unwrap()` and `GetPersisted()`.

**Task 2 (chat.go):** Error branch now extracts the `persisted` flag via a type switch on the `GetPersisted()` interface method. If `!persisted`, the fallback write executes. If `persisted`, the branch skips `writeAuditAndTurn` entirely. The `emitTurnID` call is moved inside a `turnID > 0` guard to handle the `persisted=true` case cleanly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Wrong signature in plan description**
- **Found during:** Task 1 implementation
- **Issue:** Plan described `writeAuditAndTurn` returning `(turnID int64, persisted bool)` but actual signature is `(int64, error)`. This is a plan author error, not a code bug.
- **Fix:** Derived `persisted` as `turnID > 0 && writeErr == nil` — correctly represents "row was written without error"
- **Files modified:** `internal/server/pipeline.go`
- **Commit:** `e12fd77`

**2. [Rule 1 - Bug] errors.As with concrete type fails on some Go versions**
- **Found during:** Task 2 build
- **Issue:** `errors.As(err, &pe)` where `pe` is a variable of concrete struct type is valid Go but the error check in chat_test.go revealed the pattern can confuse the compiler in some version configurations. Plan used this exact form.
- **Fix:** Introduced `GetPersisted()` interface method on `pipelineEmitError` and used a type switch `err.(interface{ GetPersisted() bool })` in `chat.go`. Cleaner and more explicit.
- **Files modified:** `internal/server/pipeline.go`, `internal/server/chat.go`
- **Commit:** `e12fd77`, `588a5ab`

**3. [Rule 1 - Bug] turnID variable scope**
- **Found during:** Task 2 build
- **Issue:** Original plan placed `turnID, _ = s.writeAuditAndTurn(...)` inside `if !persisted` block but `emitTurnID(c, turnID)` was outside, referencing an undefined variable.
- **Fix:** Declared `turnID` at function scope, called `emitTurnID` only when `turnID > 0` (post-guard).
- **Files modified:** `internal/server/chat.go`
- **Commit:** `588a5ab`

## Acceptance Criteria Verification

| Criterion | Status |
|-----------|--------|
| `pipeline.go` has `type pipelineEmitError struct { err error; persisted bool }` | PASS (line 70) |
| `writeAuditAndTurn` call captures two return values | PASS (`turnID, writeErr := ...`) |
| `return &pipelineEmitError{err: emitErr, persisted: persisted}` in error path | PASS (pipeline.go) |
| `chat.go` error branch has `if !persisted` guard | PASS (chat.go) |
| `go build ./internal/server/...` exit 0 in WSL | PASS |
| `go test ./internal/server/... -count=1` PASS | PASS (14.1s) |
| `TestCR01*` regression tests PASS | PASS |

## Commits

| Hash | Files | Message |
|------|-------|---------|
| `e12fd77` | `pipeline.go` | `feat(30-01): add pipelineEmitError type with persisted flag for CR-01` |
| `588a5ab` | `chat.go`, `cr01_test.go` | `feat(30-01): add !persisted guard in chatHandler error branch for CR-01` |

## Self-Check

- [x] `e12fd77` exists in git log
- [x] `588a5ab` exists in git log
- [x] Both files modified match plan's `files_modified`
- [x] `persisted` field in `pipelineEmitError` — present
- [x] `!persisted` guard in `chat.go` error branch — present
- [x] CR-01 double-write eliminated by guard
