---
phase: 39-ecl-pipeline
fixed_at: 2026-09-22T16:05:00+08:00
review_path: .planning/phases/39-ecl-pipeline/39-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 7
skipped: 0
status: all_fixed
---

# Phase 39: Code Review Fix Report

**Fixed at:** 2026-09-22T16:05:00+08:00
**Source review:** .planning/phases/39-ecl-pipeline/39-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 7 (CR-01, WR-01..WR-06; Info findings excluded per fix scope)
- Fixed: 7
- Skipped: 0

**Verification (all in main tree at `phase-20-p10-6-admin-console` = b789bfb):**
- `go test ./... -count=1` (WSL): exit 0, 45 packages ok — full suite green
- `PHASE39_FE_GATE=skip-external bash scripts/verify-phase-39.sh` (WSL): 8 passed, 0 failed — PASS
- `go vet` on all touched packages: clean
- `web/` untouched (all fixes are Go-side)

## Fixed Issues

### CR-01: Non-terminal `stage` after batch-level failure bricks the admin pipeline UI

**Files modified:** `internal/knowledge/ecl/manager.go`, `internal/knowledge/ecl/manager_test.go`
**Commit:** ae84393
**Status:** fixed: requires human verification (state-machine change; covered by dedicated regression tests)
**Applied fix:** Deferred error-guard in both `Run` and the retry work function: on any error return, `setStatus(StageIdle, "", 0, 0, 0)` resets the stage to the idle terminal sentinel before `releaseRunning` clears the flag (named-error-return pattern from the review suggestion). The `RetryNote` guard installs only AFTER the pre-entry DEAD-row check, so a refused retry (404/409) never touches the previous run's terminal state. The now-redundant explicit idle reset on the merge-failure path was removed (single mechanism). Regression tests: `TestManagerRunFailureResetsStage` (seedWithRetry exhaustion), `TestManagerRunFailureResetsStageNoAllocator` (nil allocator), and `TestManagerRetryNoteFailureResetsStage` with subtests for all six retry failure modes (merge walk, corpus miss, extract, kg-load, milvus-load, nil allocator) — each asserts stage == idle AND the running flag is claimable again. D-Prog-01 DTO shape and the locked FE sentinels untouched.

### WR-01: RetryNote runs synchronously on the request context

**Files modified:** `internal/knowledge/ecl/manager.go`, `internal/server/admin_pipeline.go`, `internal/server/admin_pipeline_test.go`
**Commit:** 9a4e625
**Status:** fixed: requires human verification (handler concurrency semantics; covered by handler tests)
**Applied fix:** Mirrored the run handler's detached-goroutine precedent. `RetryNote` was split: `TryClaimRetry(noteID)` (CAS claim + DEAD-row check, synchronous and cheap) stays inline in the handler so the D-Route-01 mapping still answers 400/404/409 before the response; `RetryClaimed(ctx, noteID)` (the full extract → kg-load → milvus-load pipeline, with the CR-01 terminal-stage guard) runs on a detached `context.Background()` goroutine and WARN-logs failures. `RetryNote` remains as a synchronous claim+work wrapper for in-process callers/tests, so the manager-level API is unchanged. `RetryClaimed` re-checks the DEAD row under the claim (TOCTOU guard). Handler test `TestAdminPipelineRetryValidation` updated to poll for the detached work (`awaitRowStatus` / `awaitStage`) instead of asserting synchronous completion; all gate-order, status-shape, conflict, and run tests pass unchanged.

### WR-02: GetMaxIDCollection silently truncates at the 10,000-row Query limit

**Files modified:** `internal/retriever/milvus/client.go`, `internal/retriever/milvus/delete_tier_rows_test.go`
**Commit:** 380a835
**Status:** fixed
**Applied fix:** Extracted the shared `scanMaxID` helper (window constant `maxIDQueryWindow = 10000`) used by both `GetMaxID` (per-tier) and `GetMaxIDCollection` (collection-wide). When the result set fills the window (`rows >= 10000`), the scan can no longer prove it saw the true max — it now returns a wrapped error naming the op, the window size, and the operator remedy (compact / purge dead generations before running ECL) instead of silently under-reading the watermark and resurrecting the PK-collision incident class. Empty/short result sets behave exactly as before (0 with no error). Regression tests `TestGetMaxIDCollectionTruncationGuard` and `TestGetMaxIDTruncationGuard` drive a 10,000-row hermetic mock result set and assert the loud error (no watermark returned, window size + op named).

### WR-03: Extracted table names cross the LLM trust boundary; sanitizer collisions

**Files modified:** `internal/knowledge/ecl/stub.go`, `internal/knowledge/ecl/merge_test.go`, `internal/knowledge/ecl/abox_test.go`, `internal/ontology/emitter/emitter.go`, `internal/ontology/emitter/emitter_test.go`
**Commit:** 44fe6cd
**Status:** fixed (behavior change noted for human awareness, below)
**Applied fix:** Two layers. (1) `ValidateExtraction` now gates table names on `^etl_\w+$` (`tableNamePattern`) in addition to the kind enum — matching the D-Ext-01 prompt contract and the detection regexes' `\betl_\w+` capture; non-matching LLM output is rejected before it can mint `skf:tbl_` individuals, which simultaneously eliminates the sanitizer-collision class for LLM-sourced names (`\w` is exactly the characters the sanitizer passes through). (2) `emitter.SanitizeLocal` is now collision-free: when the non-alnum→`_` rewrite actually changed a non-underscore rune, a deterministic 8-hex sha256 suffix of the ORIGINAL input is appended, so distinct inputs that collapse to the same shape (`etl_费率A` / `etl_精算A`) can never merge into one IRI. Boot determinism preserved (same input → same IRI); already-clean names stay verbatim (the gated ECL path never pays the suffix). Tests: `TestValidateExtractionTableNameGate` (4 legal + 9 rejected shapes), `TestSanitizeLocalCollisionFree` (collision, determinism, verbatim-clean), `TestABoxNoteAndTableIRI` pin updated to the collision-guarded form. **Human awareness note:** refusal-phrase individuals whose text contains non-alphanumeric characters (e.g. CJK phrases) will get new IRIs on the next boot re-emit — deterministic and lossless (the literal text is unchanged), but the emitted TBox TTL differs from the previous pointer.

### WR-04: Debounced snapshot writer swallows write errors

**Files modified:** `internal/ontology/store/store.go`
**Commit:** 839122b
**Status:** fixed
**Applied fix:** Both failure branches of the `SnapshotDebounced` timer callback now WARN-log to stderr in the project's established posture (`fmt.Fprintf(os.Stderr, "[WARN] ontology: debounced snapshot %s: marshal|write: %v\n", path, err)`) — the same pattern the ecl manager uses for ledger-write failures. A marshal or `atomicWriteFile` failure can no longer silently turn every subsequent 5s write into an unexplained no-op. Log-only change; existing store tests pass unchanged.

### WR-05: Pipeline run/retry leave no audit trail

**Files modified:** `internal/server/admin_pipeline.go`, `internal/server/admin_pipeline_test.go`
**Commit:** 230b26f
**Status:** fixed
**Applied fix:** Followed the established audit pattern (the `putKBAdminAuditRow` shape): new `putPipelineAuditRow` helper writes one INSERT-only `store.PutAudit` row per ACCEPTED trigger — `action_ecl_run` (`{"trigger":"admin_api"}`) and `action_ecl_retry` (`{"note_id":"NNN"}`) — with `Timestamp` UTC, `UserID`/`ActorType` "admin" so GET /api/admin/audit (ADM-06) surfaces them. Nil Store = degraded mode (skip); failed writes WARN-log and never roll the trigger back. **Documented divergence from the review's suggestion:** the row is emitted by the handler AT ACCEPTANCE rather than riding the goroutine's completion — the auditable fact is the admin's decision to spend the budget, and it must not depend on the async outcome (the work is detached per WR-01 and may outlive the request or the process). The work's outcome remains in the `ecl_failures` ledger (per-note stage/status) and the goroutine's WARN logs. This rationale is recorded in the committed `putPipelineAuditRow` godoc (the commit itself is subject-style; the code doc carries the full choice documentation). Test `TestAdminPipelineAuditRows`: accepted run → exactly 1 `action_ecl_run` row; refused 404 retry → still 1; accepted retry → `action_ecl_retry` with the note id; detached work still really runs.

### WR-06: Emitter's Turtle escaping does not handle newlines

**Files modified:** `internal/ontology/emitter/emitter.go`, `internal/ontology/emitter/emitter_test.go`
**Commit:** b789bfb
**Status:** fixed
**Applied fix:** `escape` now replaces `\r` and `\n` with a space (after the existing backslash/quote escapes), matching and slightly strengthening induction's `escapeTurtle` (which handles LF only) — the two copies are aligned in behavior; unifying them into one shared helper was deliberately deferred (induction.go is pre-existing and outside this finding's blast radius). Regression tests: `TestEscapeNewlines` (LF, CR, mixed-with-quote/backslash) and `TestEmitMultiLineDescriptionParses` — a SKILL.md with a YAML block-scalar description containing CJK text, a double quote, and a backslash is emitted and round-tripped through `turtle.Parse` (the exact emit → UpgradeIfNeeded → Restore → LoadFromTurtle boot chain) successfully; the description literal survives line-joined.

## Skipped Issues

None — all 7 in-scope findings were fixed.

## Notes

- Info-level findings (IN-01..IN-09) were intentionally NOT fixed per the fix scope.
- The two "requires human verification" flags (CR-01, WR-01) mark control-flow/concurrency changes; both carry dedicated behavioral regression tests (not just syntax checks), and the full suite + phase gate are green, but a human should confirm the FE observes the expected idle-reset behavior on a real failed run.
- Work executed in an isolated git worktree (`gsd-reviewfix/39-4`), then fast-forwarded onto `phase-20-p10-6-admin-console` (d775c8e → b789bfb); the pre-existing dirty WIP files (`internal/knowledge/walk.go`, `internal/retriever/mongo/seed.go`, `internal/server/knowledge.go`, `.planning/config.json`) and all untracked files were untouched. `web/` untouched.
- Known environmental note: `TestEmitSkillInstanceCount` depends on 5 untracked `skills/*/` directories that exist only in the main working tree; it fails in a fresh worktree (proven identical on pre-fix HEAD) and passes in the main tree, which is where the final verification ran.

---

_Fixed: 2026-09-22T16:05:00+08:00_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
