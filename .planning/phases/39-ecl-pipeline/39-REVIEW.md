---
phase: 39-ecl-pipeline
reviewed: 2026-09-22T12:00:00Z
depth: standard
files_reviewed: 53
files_reviewed_list:
  - cmd/server/main.go
  - eval/ecl-golden/sql-guides.jsonl
  - internal/knowledge/ecl/abox.go
  - internal/knowledge/ecl/abox_test.go
  - internal/knowledge/ecl/chunk.go
  - internal/knowledge/ecl/chunk_test.go
  - internal/knowledge/ecl/extract.go
  - internal/knowledge/ecl/extract_golden_test.go
  - internal/knowledge/ecl/extract_test.go
  - internal/knowledge/ecl/f1.go
  - internal/knowledge/ecl/f1_test.go
  - internal/knowledge/ecl/failures.go
  - internal/knowledge/ecl/failures_test.go
  - internal/knowledge/ecl/manager.go
  - internal/knowledge/ecl/manager_test.go
  - internal/knowledge/ecl/merge.go
  - internal/knowledge/ecl/merge_test.go
  - internal/knowledge/ecl/meta.go
  - internal/knowledge/ecl/stub.go
  - internal/knowledge/indexer.go
  - internal/knowledge/indexer_test.go
  - internal/llm/extract.go
  - internal/llm/extract_test.go
  - internal/ontology/emitter/emitter.go
  - internal/ontology/emitter/emitter_test.go
  - internal/ontology/emitter/upgrade.go
  - internal/ontology/emitter/upgrade_test.go
  - internal/ontology/induction/induction.go
  - internal/ontology/store/restore.go
  - internal/ontology/store/restore_test.go
  - internal/ontology/store/store.go
  - internal/ontology/store/store_test.go
  - internal/retriever/milvus/client.go
  - internal/retriever/milvus/config.go
  - internal/retriever/milvus/delete_expr.go
  - internal/retriever/milvus/delete_tier_rows_test.go
  - internal/retriever/milvus/seed.go
  - internal/retriever/milvus/seed_test.go
  - internal/retriever/retriever.go
  - internal/server/admin_pipeline.go
  - internal/server/admin_pipeline_test.go
  - internal/server/server.go
  - internal/store/sqlite/ecl_failures.go
  - internal/store/sqlite/schema.go
  - scripts/verify-phase-39.sh
  - web/src/__tests__/AdminFeedbackTabs.test.tsx
  - web/src/components/admin/AdminDashboard.tsx
  - web/src/components/admin/ontology/OntologyTab.test.tsx
  - web/src/components/admin/pipeline/PipelineTab.test.tsx
  - web/src/components/admin/pipeline/PipelineTab.tsx
  - web/src/components/admin/pipeline/pipelineApi.ts
  - web/src/components/admin/pipeline/types.ts
  - web/src/store/app-store.ts
findings:
  critical: 1
  warning: 6
  info: 9
  total: 16
status: issues_found
---

# Phase 39: Code Review Report

**Reviewed:** 2026-09-22T12:00:00Z
**Depth:** standard
**Files Reviewed:** 53
**Status:** issues_found

## Summary

Phase 39 (ECL Pipeline: section-aware chunker + LLM entity extraction + admin surface) is a large, well-tested slice: 53 files, strong test coverage (hermetic corpus-backed tests, golden-set F1 harness, handler gate-order tests, vitest UI contract suite), and disciplined seam boundaries (SeedFunc / Extractor / FailureStore narrow interfaces; milvus kept out of the ecl package).

The phase's explicit threat-model items largely hold up under adversarial reading:

- **403-before-503 gate order** (admin_pipeline.go:73-82) is correct and regression-locked (TestAdminPipelineGateOrder exercises all three routes with ECL nil).
- **409 run-claim race** — `CASRunning` is a mutex-guarded test-and-set (manager.go:166-174); both the run handler and RetryNote claim it before doing work; concurrent claims are refused. No TOCTOU.
- **note_id validation** — `^[0-9]{3}$` fires before the manager is consulted (admin_pipeline.go:148-152) and the id never reaches a filesystem path; the manager additionally requires an existing DEAD ledger row.
- **DeleteByExpr caller-side escaping** — both production call sites are safe: main.go:968 uses a compile-time literal expression; indexer.go `purgeExprFor` (indexer.go:504-509) interpolates the tier via `strconv.Quote`. DeleteByExpr itself never interpolates (delete_expr.go:62-95).
- **KG store index symmetry** — `DeleteSubject` rebuilds byS/byP/byO from scratch in one write-locked pass (store.go:115-140); `InsertUnique` shares one write-lock acquisition for check+insert (no TOCTOU; subject-candidate scan is sound because an exact match must share S). Both are regression-tested for phantom entries across all three indices.
- **Upgrade no-op** — the NoteChunk sentinel short-circuit (upgrade.go:108-111) never re-emits, even when skills/knowledge trees changed (TestUpgradeIfNeeded_SentinelIsNoOp pins this); the one-time upgrade replays approved induction triples.
- **Restore fail-loud** — a corrupt/unreadable snapshot returns an error, never a TTL-only fallback (restore.go:68-75; TestRestore_UnreadableSnapshotErrors).
- **FE** — no secrets in logs; admin token rides the `X-Admin-Token` header per the CapacityMonitor pattern; the poll interval is correctly re-armed via the `[cadence, fetchOnce]` effect dependency; errors never stop the loop.

The one Critical defect is a state-machine hole in the Manager's terminal-status discipline: every batch-level failure path after the merge stage leaves `stage` non-terminal while releasing the running flag, which — per the LOCKED FE mapping (`isRunning = stage !== 'idle' && stage !== 'done'`) — permanently disables the run button in the admin UI until the server restarts. The Warnings cluster around retry-path latency/cancellation, a silent watermark under-read that can resurrect the exact PK-collision incident class the phase was designed to prevent, an incomplete table-name trust-boundary gate, silent snapshot-write failures, a missing admin audit trail, and an unvalidated Turtle emission edge.

Pre-existing files touched only for exports (induction.go, retriever.go, config.go) were weighted accordingly; their findings are noted only where they interact with the new Phase 39 paths.

## Critical Issues

### CR-01: Non-terminal `stage` after batch-level failure bricks the admin pipeline UI (run button permanently disabled)

**File:** `internal/knowledge/ecl/manager.go:337-359` (Run milvus-load error paths), `internal/knowledge/ecl/manager.go:384-440` (RetryNote error paths); consequence at `web/src/components/admin/pipeline/PipelineTab.tsx:96,179-180`

**Issue:** `Run` resets the stage to `idle` only on the merge-stage failure (manager.go:282). Every later error return leaves the stage at its last in-flight value while `releaseRunning()` (deferred) clears the running flag:

- nil ID allocator → returns at manager.go:343 with stage = `milvus-load`
- `BuildDocuments` error → manager.go:355, stage = `milvus-load`
- `seedWithRetry` exhaustion (e.g. Milvus outage — the documented flaky-intranet failure mode) → manager.go:358, stage = `milvus-load`

`RetryNote` has the same hole on every error path: merge walk failure (stage stuck `merge`, line 387), corpus-miss (stuck `merge`, line 401), extract failure (stuck `extract`, line 408), kg-load failure (stuck `kg-load`, line 415), milvus-load failure (stuck `milvus-load`, lines 430/434).

The stage sentinels are a LOCKED FE contract (manager.go:35-36): `isRunning = stage !== 'idle' && stage !== 'done'`. After any of the above, `GET /api/admin/pipeline/status` reports a running stage while no run is running. PipelineTab then: shows the pulsing `运行中…` badge forever, polls at the 2s running cadence forever, and — decisively — disables the run button (`runDisabled = eclOff || isRunning || runInFlight`, PipelineTab.tsx:180). The operator cannot start a new run to clear the state; the only recovery is a server restart. A single Milvus blip during milvus-load takes the entire Phase 39 admin surface out of action, which contradicts D-RT-01's core promise (single-note/batch failure must leave the system operable).

**Fix:** Guarantee a terminal status on every exit. The minimal-diff shape is a completion guard in both entry points:

```go
func (m *Manager) Run(ctx context.Context) (err error) {
	defer m.releaseRunning()
	defer func() {
		if err != nil {
			// batch-level failure: leave the UI operable (idle = not running)
			m.setStatus(StageIdle, "", 0, 0, 0)
		}
	}()
	...
}
```

and the same deferred reset in `RetryNote` (or factor a shared `finishRun(err)`). If product semantics prefer distinguishing "failed" from "never run", add a `failed` sentinel — but that requires re-locking the FE stage mapping in 39-UI-SPEC and PipelineTab; the `idle` reset is the contract-safe fix today. Add a regression test asserting `Status().Stage == StageIdle` after a forced `seedWithRetry` failure and after each RetryNote failure mode.

## Warnings

### WR-01: `RetryNote` runs synchronously in the HTTP handler on the request context — timeout/cancellation aborts mid-pipeline

**File:** `internal/server/admin_pipeline.go:153`, `internal/knowledge/ecl/manager.go:371-441,446-466`

**Issue:** The retry handler calls `mgr.RetryNote(ctx, noteID)` with the Hertz request context. A retry's worst case is 3 LLM calls + up to 15s of un-cancellable backoff sleeps (`time.Sleep(m.backoffFor(attempt))` — 1s+4s per retry stage, three stages) + a Milvus purge/embed/seed round-trip — easily past typical proxy/client timeouts. If the request context cancels first, the response never arrives, the note is left dead/pending in the ledger, and (with CR-01) the stage is left stuck. The handler's own doc says RetryNote "runs synchronously in the request goroutine" while its 200 body claims `{"status":"queued"}` — the semantics are neither fire-and-forget like run nor instant like a queue.

**Fix:** Mirror the run handler: claim via `RetryNote`'s internal CAS on a detached `context.Background()` goroutine and answer 200 immediately (retry progress already flows through the same status polling + ledger row), or keep it synchronous but (a) pass a detached context and (b) make the backoff sleeps ctx-aware (`select { case <-ctx.Done(): ...; case <-time.After(d): }`).

### WR-02: `GetMaxIDCollection` silently truncates at the 10,000-row Query limit — watermark under-read resurrects the PK-collision failure class

**File:** `internal/retriever/milvus/client.go:141-169` (same pattern in `GetMaxID`, lines 91-123)

**Issue:** The watermark is computed by one Query with `client.WithLimit(10000)` and a client-side max scan. Milvus Query has no ORDER BY, and the code neither paginates nor detects that the result was truncated: once the collection exceeds 10,000 physical rows (tombstoned generations count toward the window until compaction — and this collection has a documented append-amplification history: 2931 physical rows for a 256-row corpus on 2026-09-15), the scan can miss the true max ID. ECL chunk allocation then starts below existing PKs, and since Milvus does not dedupe Insert (seed.go:136-151), both generations become searchable — precisely the incident class Plan 39-02 built this function to prevent. The failure is silent (no error, no log).

**Fix:** Detect truncation and fail loudly: request the row count (`mc.Query` with `client.WithOutputFields([]string{})` + a count call, or `GetCollectionStatistics`) and return an error when it exceeds the scan limit; or paginate with `client.WithOffset`/search-after until a short page is seen; at minimum, when the result set length == 10000, return a wrapped error telling the operator to compact/recreate before running ECL.

### WR-03: Extracted table names cross the LLM trust boundary without the `etl_` prefix/charset gate — sanitizer collisions can conflate distinct tables

**File:** `internal/knowledge/ecl/stub.go:52-65` (ValidateExtraction), `internal/knowledge/ecl/abox.go:43-45` (TableIRI)

**Issue:** The D-Ext-01 prompt says "Include only tables whose name starts with etl_", but `ValidateExtraction` — the defense-in-depth gate the docs lean on for T-39-06 — checks only the kind enum and non-emptiness. A hallucinating or injected LLM output can mint `tbl_` individuals from arbitrary strings. Worse, `emitter.SanitizeLocal` maps every non-`[A-Za-z0-9]` rune to `_`, so distinct names collide: `etl_费率A` and `etl_精算A` both sanitize to `etl___A` (likewise `etl a` / `etl-a` / `etl.a` → `etl_a`). Collapsed IRIs merge two distinct tables into one TableSchema individual and fuse their `referencesTable` edges — silent KG data corruption, not a crash.

**Fix:** In `ValidateExtraction`, gate names on the corpus contract the detection regexes already assume, e.g. `var tableNamePattern = regexp.MustCompile(`^etl_\w+$`)` and reject non-matches (this matches `sqlTableName`'s `\betl_\w+` and the golden-set shapes, all of which are ASCII). That simultaneously eliminates the sanitizer-collision class for LLM-sourced names.

### WR-04: Debounced snapshot writer swallows write errors with no trace — ABox durability can fail silently

**File:** `internal/ontology/store/store.go:262-281` (`SnapshotDebounced` timer callback)

**Issue:** The debounced writer — the ONLY recurring durability trigger for ECL ABox writes and induction mutations (both managers' `SetOnChange` wire it, main.go:926-927/982) — returns silently on both `Snapshot()` marshal failure and `atomicWriteFile` failure. Unlike the manager's ledger writes (WARN-logged, manager.go:544-546) and the boot snapshot (WARN-logged, main.go:919-921), a failing disk/permission/rename makes every subsequent 5s write a no-op with zero log output; ABox accumulated since the last successful snapshot is then lost on the next crash/restart with nothing in the operator's log to explain it.

**Fix:** Log both error branches (`fmt.Fprintf(os.Stderr, "[WARN] ontology: debounced snapshot %s: %v\n", path, err)`), matching the established WARN posture. Optionally track a last-error for the admin KG stats endpoint.

### WR-05: Pipeline run/retry trigger LLM spend, Milvus deletes, and KG writes with no audit trail

**File:** `internal/server/admin_pipeline.go:103-118,143-165`, `internal/knowledge/ecl/manager.go` (no store.AuditRow writes anywhere)

**Issue:** Every comparable admin mutation surface in this codebase writes an INSERT-only audit row (kb create/deactivate per KBM-02, doc upload via `SetDocUploadAuditHook`, feedback dispositions, external-auth). The ECL run/retry routes are the most expensive and most destructive admin actions in the system (whole-generation `DeleteByExpr` purges, ~26 LLM calls per run, TBox-adjacent KG mutations) yet leave no `action_*` audit row. The repo's own threat model (T-3-AUDIT, SC-3 audit query) treats un-audited admin actions as a gap; today an operator cannot answer "who ran the pipeline when" from the audit table.

**Fix:** Wire an audit hook (the `SetDocUploadAuditHook` pattern) and emit one row per accepted run (`action_ecl_run`, detail: stage summary/notes count) and per retry (`action_ecl_retry`, detail: note_id + outcome). The handler is the right emission point; the CAS-claimed outcome can ride the run goroutine's completion.

### WR-06: Emitter's Turtle string escaping does not handle newlines — a multi-line SKILL.md description bricks the next boot

**File:** `internal/ontology/emitter/emitter.go:331-335` (`escape`), consumed by `emitter.go:284` and the boot chain `emitter/upgrade.go:85-98` → `store/restore.go:49-62` → `cmd/server/main.go:900-913`

**Issue:** `escape` escapes only `\` and `"` — not LF/CR, which Turtle's `STRING_LITERAL_QUOTE` production forbids. `parseFrontmatter` (emitter.go:159-179) happily yields multi-line strings from YAML block scalars (`description: |`), so one edited SKILL.md makes `Emit` produce invalid Turtle. `UpgradeIfNeeded` writes it as the pointer unvalidated; on the same boot `Restore` → `LoadFromTurtle` → `turtle.Parse` fails → `log.Fatalf` in main.go. The deployment is bricked by a controlled-file edit and the operator's only signal is a parse error at restore, not at emission. Pre-existing `escape()`, but it is now load-bearing on the Phase 39 UpgradeIfNeeded→Restore boot path (induction's own `escapeTurtle` at induction.go:532-537 already does the `\n`→space replacement — the two copies have drifted).

**Fix:** Make `escape` also do `strings.ReplaceAll(s, "\n", " ")` (and `\r`), aligning with `escapeTurtle`; or validate the freshly emitted TTL by round-tripping it through `turtle.Parse` before `writeFileAtomic` swaps the pointer, so a bad emit fails the boot with the SKILL.md named.

## Info

### IN-01: `buildExtractionPrompt` is dead code

**File:** `internal/knowledge/ecl/extract.go:83-87`

**Issue:** Defined and documented but never called — `ExtractNote`/`extractBody` use `joinSectionBodies` + `promptForBody` directly; no test references it.
**Fix:** Delete it, or route `ExtractNote`'s under-cap path through it so the helper has one caller.

### IN-02: Section-cap fallback is single-level — an over-cap section is still sent whole to the LLM

**File:** `internal/knowledge/ecl/extract.go:148-176`

**Issue:** The 40,000-rune cap decides whole-note vs per-section extraction, but the per-section path (line 156) passes `s.Body` with no cap check and no sub-split. A single section over the cap (RESEARCH measured a 93,993-byte section) produces an oversized prompt; the failure surfaces as a provider truncation/parse error → 3 retries → dead letter, i.e. degraded extraction rather than the guardrail the pitfall describes.
**Fix:** In the oversized path, reuse `subSplit` (chunk.go) for any section still over the cap, or at least document the single-level limitation next to `DefaultSectionCap`.

### IN-03: Batch milvus-load failure overstates the ledger's attempt counts

**File:** `internal/knowledge/ecl/manager.go:351-355,519-521`

**Issue:** When `BuildDocuments` fails, every note in the batch is parked `dead` with `AttemptCount: maxAttempts` (3) even though the batch seed may have failed on its very first attempt; `seedWithRetry`'s terminal loop does the same for all notes in the batch. The UI renders this as `3/3` attempts.
**Fix:** Record the real attempt number (`attempt` from the retry loop) for batch-terminal failures, or document that `maxAttempts` at batch level means "batch budget exhausted", not per-note attempts.

### IN-04: Ledger store errors masquerade as "unknown note"; 500 responses leak internal error strings

**File:** `internal/knowledge/ecl/manager.go:377-380`; `internal/server/admin_pipeline.go:159-161`

**Issue:** `RetryNote` collapses any `GetFailure` error (including sqlite I/O failures) into `ErrUnknownNote` → 404, indistinguishable from a genuinely unknown note for the admin. The retry handler's default branch returns `"retry: " + err.Error()` verbatim to the client — internal error text (paths, SQL fragments) in an HTTP body.
**Fix:** Distinguish "no row" (`errors.Is(err, sqlite.ErrNoFailureRecord)`) from store failures (→ 500); return a generic message for 500 and keep the detail in the server log.

### IN-05: Detached run goroutine has no panic recovery

**File:** `internal/server/admin_pipeline.go:112-116`

**Issue:** `Run` executes in a bare `go func()`; any panic inside the LLM adapter, splitter, or SDK kills the entire server process (Go semantics), converting a pipeline bug into a full outage.
**Fix:** `defer func() { if r := recover(); r != nil { log.Printf("[ERROR] ecl run panicked: %v\n%s", r, debug.Stack()) } }()` at the top of the goroutine (and reset the running flag/status — the deferred `releaseRunning` in Run does run during panic unwinding, but the stage would stay stuck per CR-01).

### IN-06: Retry backoff sleeps are not context-aware

**File:** `internal/knowledge/ecl/manager.go:461,485,516`

**Issue:** `time.Sleep(m.backoffFor(attempt))` (up to 16s for the LLM stage ladder) ignores `ctx`; cancellation waits out the full sleep. Compounds WR-01's handler latency; harmless under Run's `context.Background()` today.
**Fix:** Replace with a `select` on `ctx.Done()` / `time.After(d)` (then re-check `ctx.Err()` at loop top, which the loops already do).

### IN-07: Watermark query failure silently degrades ECL ID allocation to 1

**File:** `cmd/server/main.go:955-960`

**Issue:** If `GetMaxIDCollection` fails at boot, the manager is still armed with the counter starting at 1, guaranteeing PK collisions with boot-seeded rows (WARN-logged, but the failure mode contradicts D-Milvus-01's structural PK-integrity posture — the deployment would be better served by leaving `eclManager` nil so the routes 503 until the watermark is readable).
**Fix:** On watermark error, skip arming the manager (log the reason); the admin surface already has a clean nil-posture (503).

### IN-08: Non-stable sort underpins the "deterministic downstream order" pin

**File:** `internal/knowledge/ecl/manager.go:317-321`

**Issue:** `sort.Slice` (not `SliceStable`) orders `loaded` by `strconv.Atoi(SourceIdx)` with the error discarded. Keys are unique 3-digit numerics on this corpus, so behavior holds; but any non-numeric source_idx (Atoi → 0, several such keys) makes the kg write order and ID allocation nondeterministic — quietly violating the pin in the comment two lines above.
**Fix:** `sort.SliceStable`, and skip (or dead-letter) notes whose SourceIdx is non-numeric instead of colliding them at key 0.

### IN-09: `ApplyNote`'s replace sequence is not transactional against concurrent readers

**File:** `internal/knowledge/ecl/abox.go:109-150`, `internal/ontology/store/store.go:151-184`

**Issue:** ApplyNote performs capture → DeleteSubject → orphan sweep → inserts as separate per-op-locked store calls (correctly validated-before-mutation, and production callers serialize via the manager's CAS). But a concurrent reader — notably the manual `POST /api/admin/ontology/snapshot` handler — can snapshot the intermediate state (note deleted, orphans unswept) and persist it; a crash before the insert completes then leaves that half-replaced state as the restored KG. Bounded impact: the state is valid RDF and the note-level replace converges on the next successful run/retry of that note.
**Fix:** Acceptable to document; if hardening, route manual snapshots through a store-level RLock held across Snapshot (already the case) plus an ApplyNote-side "in-flight" flag, or snapshot from the debounced writer only.

---

### Verified-clean (phase focus items with no findings)

- **403-before-503 order** and its regression test: correct on all three routes.
- **CAS run-claim / 409**: mutex-guarded, claim-before-launch protocol documented and tested.
- **note_id `^[0-9]{3}$` gate** before manager lookup; id never used in a path.
- **DeleteByExpr expression contract**: both callers escape or use literals; helper passes expr verbatim (test-pinned).
- **KG store `DeleteSubject`/`InsertUnique`**: index symmetry and TOCTOU-freedom verified in code and tests.
- **Upgrade no-op when NoteChunk sentinel present**; restore fails loudly on corrupt snapshot (no silent TTL fallback).
- **sqlite `ecl_failures` upsert**: fully parameterized (T-3-SQLI), sentinel-wrapped not-found, restart-persistence tested; mutable status column is explicitly sanctioned in the DDL comment.
- **FE PipelineTab/pipelineApi**: token header handling, cadence re-arm, honesty rules, and per-row retry disable all match the spec; no secrets or verbose logging.

---

_Reviewed: 2026-09-22T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
