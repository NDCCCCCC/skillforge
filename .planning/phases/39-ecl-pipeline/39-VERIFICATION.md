---
phase: 39-ecl-pipeline
verified: 2026-09-22T13:35:00Z
status: human_needed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Live LLM extraction F1 gate (ONT-12 / SC-2 live half): probe GLM reachability, start server, POST /api/admin/pipeline/run with admin token, poll status to done, run the F1 scorer against live extraction output vs eval/ecl-golden/sql-guides.jsonl"
    expected: "macro-F1 >= 0.8 on live output; if intranet LLM is down, record env-STOP per VALIDATION.md disposition (do not retry-score against stub)"
    why_human: "Requires live intranet LLM availability; verify script is hermetic by locked design (env-intranet-service-flakiness)"
  - test: "Live Milvus E2E (ONT-13 / SC-3): SKILLFORGE_MILVUS_RECREATE=1 boot -> admin Run ECL -> unset RECREATE -> restart -> query sql-guides retrieval + /admin/ontology graph"
    expected: "Retrieval returns merged-section chunks with consistent metadata (tier/source_idx/section/section_idx/section_count); graph shows note_NNN / tbl_* individuals; legacy rows coexist without cross-deletion"
    why_human: "Requires intranet Milvus + BGE-M3 and the one-shot RECREATE deploy sequence; hermetic suites cover schema/purge logic but not the live round-trip"
  - test: "UI-SPEC visual pass (SC-4): walk the 13-item Visual Checklist in 39-UI-SPEC.md against a running server across idle / running / done / failures / 403 / 503 states"
    expected: "Verbatim copy, stage badges, progress meter, queue/failure chips, 6-column failures table, per-row retry styling all match the approved spec"
    why_human: "Visual appearance and copy rendering cannot be verified programmatically"
  - test: "CR-01 failure-path operability check (SC-4/SC-5): force a milvus-load failure (e.g. stop Milvus mid-run), then observe GET /api/admin/pipeline/status and the FE run/retry buttons"
    expected: "KNOWN DEFECT (39-REVIEW CR-01): stage stays non-terminal (milvus-load) while no run is active — run button disabled and pulsing badge until a per-note retry succeeds or restart; dead rows remain retryable per-note. Confirm the defect reproduces and decide whether to schedule the minimal fix (deferred idle reset on error in Manager.Run/RetryNote)"
    why_human: "Requires a real partial-failure scenario against live services; outcome is a product decision on fix scheduling"
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: human_needed
---

# Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction — Verification Report

**Phase Goal:** 替换 `internal/knowledge/indexer.go` byte-cap chunker；LLM 自动从 sql-guides markdown 抽取 etl_* 实体 + 关系；ABox 自动写入 KG + Milvus 双向同步。
**Verified:** 2026-09-22T13:35:00Z
**Status:** human_needed (all 6 roadmap SCs verified; 4 items require live/visual human checks)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (roadmap SC) | Status | Evidence |
|---|---|---|---|
| 1 | Section-aware chunker: 185 `__secNN` fragments → 26 notes, 100% boundary, note 001 3/3 corpus SQL targets | ✓ VERIFIED | `internal/knowledge/ecl/merge.go` (WalkAndMerge, IsSecFragment dual-condition); own test run: "contribution completeness: 185/185 fragment-contributions across 26 notes, 69 section records" + "note 001 …: 3/3 corpus SQL targets present" (TestMergeBoundary, TestNote001SQL PASS) |
| 2 | LLM 实体抽取 etl_* table refs + 关系，F1 ≥ 0.8 vs golden | ✓ VERIFIED (hermetic half) | `ecl/extract.go` LLMExtractor (temp-0, response_format json_object, ExtractJSON, ValidateExtraction, 40K-rune cap); `ecl/f1.go` pure math; own test run: macro-F1 tables 0.9846 / related 1.0 vs 26-row human-reviewed golden (TestStubVsGoldenF1 PASS). Live-LLM half = documented operator gate → human_verification #1 |
| 3 | 新 ABox triples 自动写入 memory-map KG + Milvus 索引同步 | ✓ VERIFIED (code + tests) | `ecl/abox.go` ApplyNote (validate → capture → DeleteSubject → orphan sweep → Insert/InsertUnique → one onChange snapshot trigger); `ecl/chunk.go` BuildDocuments (consistent metadata + 3500-rune sub-split post-condition); boot wiring `cmd/server/main.go:942-980` (SeedFunc closure: marker purge `section_count > 0` → embed → SeedDocuments; watermark ID allocator); snapshot-first restore `main.go:911`. Live E2E → human_verification #2 |
| 4 | Admin UI 实时显示：当前文件 / 进度 / 队列长度 / 失败重试按钮 | ✓ VERIFIED | `PipelineTab.tsx`: progress meter (done/total), 当前文件 row, 待加载队列 chip, 失败 chip, 6-column failures table, per-row 重试 enabled only on dead; polling 2s running / 10s idle re-armed on flip; 9th tab wired (`AdminDashboard.tsx:52,209,239`, `app-store.ts` union). Own vitest run: 13/13 PASS; full FE suite 628/628 (orchestrator). Caveat: CR-01 staleness after batch-level failure — see Warnings |
| 5 | ECL 失败 → 自动 retry 3 次 → dead-letter → 可手动触发 | ✓ VERIFIED | `ecl/manager.go`: maxAttempts=3, backoff 1s/4s/16s, per-note pending→dead rows while batch continues; `sqlite ecl_failures` restart-durable ledger; RetryNote (DEAD-row required, re-arm, full-note idempotent re-run, resolves on success); HTTP `POST /api/admin/pipeline/retry/:note_id` (400/404/409 semantics test-pinned); FE retry button dead-only. Caveat: CR-01 RetryNote error paths leave stage stuck — see Warnings |
| 6 | `verify-phase-39.sh` 全绿 (100% boundary + F1 ≥ 0.8) | ✓ VERIFIED | Own run (WSL, PHASE39_FE_GATE=skip-external): "8 passed, 0 failed", exit 0; SC-1 + SC-2 stages re-confirmed; FE delegation acknowledged and compensated by own vitest runs (13/13 pipeline + 628 total) |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/knowledge/ecl/merge.go` | 005c walk+merge port, IsSecFragment | ✓ VERIFIED | Substantive (273 lines); wired into indexer.go:236 |
| `internal/knowledge/ecl/meta.go` | ChunkMeta contract | ✓ VERIFIED | section_count key present |
| `internal/knowledge/ecl/stub.go` | regex stub extractor / golden drafter | ✓ VERIFIED | D-Ext-01 kinds; line-anchored etl_ regexes |
| `internal/knowledge/ecl/extract.go` | Extractor interface + LLM impl | ✓ VERIFIED | LLM + Stub behind one seam; compile-time pins |
| `internal/knowledge/ecl/f1.go` | LoadGolden + MacroF1 | ✓ VERIFIED | Pure; zero-denominator convention documented |
| `internal/knowledge/ecl/abox.go` | BuildTriples + ApplyNote | ✓ VERIFIED | Note-level replace, orphan sweep, single onChange |
| `internal/knowledge/ecl/chunk.go` | sections → Documents + sub-split | ✓ VERIFIED | eino recursive splitter, hard-cap post-condition |
| `internal/knowledge/ecl/manager.go` | stage orchestrator | ✓ VERIFIED | Run/Status/RetryNote/CASRunning; CR-01 defect noted |
| `internal/knowledge/ecl/failures.go` + `internal/store/sqlite/ecl_failures.go` | dead-letter ledger | ✓ VERIFIED | Narrow FailureStore; idempotent DDL; parameterized |
| `internal/retriever/milvus/{seed,delete_expr,client}.go` | 11-field schema, DeleteByExpr, GetMaxIDCollection | ✓ VERIFIED | 4 ECL fields; marker-scoped purge; collection-wide watermark |
| `internal/ontology/{emitter,store}` (emitter.go, upgrade.go, restore.go, store.go) | 8-class TBox, UpgradeIfNeeded, Restore, DeleteSubject/InsertUnique | ✓ VERIFIED | NoteChunk/TableSchema/referencesTable/belongsToNote present; sentinel no-op; snapshot-first |
| `internal/knowledge/indexer.go` | ECL routing branch + marker purge | ✓ VERIFIED | IsSecFragment skip BEFORE maxDocBytes (lines 236→239); purgeExprFor |
| `cmd/server/main.go` | boot wiring | ✓ VERIFIED | SetECLMode → UpgradeIfNeeded → Restore → ecl.NewManager (never auto-run); deps.ECL |
| `internal/server/admin_pipeline.go` + `server.go` | 3 gated routes | ✓ VERIFIED | 403-before-503 locked order; 409/404/400; routes at server.go:681-683 |
| `web/src/components/admin/pipeline/{types,pipelineApi,PipelineTab}.tsx` | UI-SPEC tab | ✓ VERIFIED | Verbatim copy; X-Admin-Token; typed errors; 2s/10s cadence |
| `eval/ecl-golden/sql-guides.jsonl` | 26-row human-reviewed golden | ✓ VERIFIED | Exactly 26 lines; review sign-off 2026-09-22 in golden-review-checklist.md (policy A) |
| `cmd/ecl-draft-golden/main.go` | deterministic drafter | ✓ VERIFIED | StubExtractor over WalkAndMerge, LLM-free |
| `scripts/verify-phase-39.sh` | hermetic SC-1..6 gate | ✓ VERIFIED | Own run: 8 passed / 0 failed, exit 0; vacuous-run guards present |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| indexer.go per-file loop | ecl.IsSecFragment | skip check before maxDocBytes | ✓ WIRED | indexer.go:236 branch precedes :239 guard (ordering load-bearing, regression-tested) |
| indexer.go seedDocs | milvus.DeleteByExpr | purgeExprFor marker expr | ✓ WIRED | `section_count == 0` legacy purge in ECL mode; %q-escaped tier |
| cmd/server/main.go | ecl.NewManager | cm + KG + sqlite FailureStore + SeedFunc closure | ✓ WIRED | main.go:962-980; watermark allocator; marker purge `section_count > 0`; embed before insert |
| cmd/server/main.go | emitter.UpgradeIfNeeded / onstore.Restore | boot reconcile chain | ✓ WIRED | main.go:900/911; snapshot-first, fail-loud |
| ecl.Manager | ecl_failures SQLite | FailureStore interface | ✓ WIRED | sqlite store injected at main.go:963; map fake in tests |
| server.go Deps.ECL | ecl.Manager + 3 routes | nil-posture 503 | ✓ WIRED | Deps.ECL main.go:1093; routes server.go:681-683 |
| admin_pipeline.go requireECL | requireAdmin | 403 before 503 | ✓ WIRED | admin_pipeline.go:73-82; gate-order test suite passes |
| PipelineTab.tsx | pipelineApi.ts → /api/admin/pipeline/* | X-Admin-Token + typed errors | ✓ WIRED | 3 fetch wrappers; server-truth-only lifecycle |
| AdminDashboard.tsx | PipelineTab | 9th SubrouteTab + ViewBody branch | ✓ WIRED | Lines 52/209/239; store union widened |
| ecl.Manager applyNote | abox.ApplyNote → store.DeleteSubject/InsertUnique | kg-load stage | ✓ WIRED | manager.go:135 default seam; onChange → shared kgDebounce |
| extract_golden_test.go | eval/ecl-golden/sql-guides.jsonl | LoadGolden + MacroF1 gate ≥ 0.8 | ✓ WIRED | Own run: 0.9846 / 1.0 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| PipelineTab.tsx | status (stage/done/total/queue_len) | GET status → Manager.Status() live counters | Yes (per-stage counters from actual run state) | ✓ FLOWING |
| PipelineTab.tsx failures table | failures[] | SQLite ecl_failures ledger via ListFailures | Yes (durable rows, restart-tested) | ✓ FLOWING |
| merge_test.go / extract_golden_test.go | notes / golden rows | real in-repo corpus `knowledge/sql-guides` + reviewed jsonl | Yes (185 real fragments, 26 reviewed rows) | ✓ FLOWING |
| PipelineTab.tsx | status prop at call site | none (tab fetches its own server truth) | n/a | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| SC-1 chunker boundary + note 001 | `go test ./internal/knowledge/ecl/ -run 'TestMergeBoundary\|TestNote001' -count=1 -v` | 185/185, 26 notes, 3/3 tables — PASS | ✓ PASS |
| SC-2 F1 vs reviewed golden | `go test ./internal/knowledge/ecl/ -run TestStubVsGoldenF1 -count=1 -v` | tables F1=0.9846, related F1=1.0 — PASS | ✓ PASS |
| SC-6 full hermetic gate | `PHASE39_FE_GATE=skip-external bash scripts/verify-phase-39.sh` (WSL) | 8 passed / 0 failed, exit 0 | ✓ PASS |
| SC-4 FE contract suite | `node node_modules/vitest/vitest.mjs run src/components/admin/pipeline/PipelineTab.test.tsx` | 13/13 passed | ✓ PASS |
| Full Go suite | `go test ./... -count=1` (orchestrator run, post-final-wave) | all green | ✓ PASS |

### Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `scripts/verify-phase-39.sh` | `PHASE39_FE_GATE=skip-external bash scripts/verify-phase-39.sh` (WSL) | exit 0; 8 [SC-PASS] stages; SC-4 FE stage explicitly delegated out-of-band with operator acknowledgment | ✓ PASS (FE delegation compensated by independent vitest runs) |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| ONT-10 | 39-03, 39-06 | Replace byte-cap chunker with section-aware markdown chunker (source_idx + section grouping) | ✓ SATISFIED | ecl/merge.go + indexer.go routing branch (ECL-off posture byte-identical legacy) |
| ONT-11 | 39-03, 39-06 | 100% boundary detection, 185→26 notes, complete SQL across boundaries | ✓ SATISFIED | TestMergeBoundary hermetic on real corpus; 69 merged sections |
| ONT-12 | 39-01, 39-04, 39-05, 39-09 | LLM-driven entity extraction → etl_* + relationships → ABox triples | ✓ SATISFIED | Extractor seam + LLMExtractor + ABox writer; hermetic F1 0.9846 vs reviewed golden (live half = operator gate) |
| ONT-13 | 39-01..09 (all) | ABox → KG + Milvus indexing with consistent metadata; retry/dead-letter + admin surface | ✓ SATISFIED | ApplyNote + BuildDocuments + SeedFunc closure + ecl_failures + admin routes + FE tab |

No orphaned requirements: REQUIREMENTS.md maps exactly ONT-10..13 to Phase 39; all four appear in plan frontmatter `requirements:` fields.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `internal/knowledge/ecl/manager.go` | 343/355/358 (Run), 387-434 (RetryNote) | Non-terminal stage on batch-level error paths (CR-01, 39-REVIEW) | ⚠️ Warning | After e.g. a milvus-load outage, status reports a running stage with no active run → FE run button disabled + pulsing badge until a per-note retry succeeds or server restart. Per-note retry buttons stay live and a successful RetryNote resets stage to done, so recovery without restart is possible — but D-RT-01's "system stays operable" promise is degraded under the documented intranet-flakiness scenario. Does NOT falsify SC-4/SC-5 (all four UI elements exist and update; retry×3 → dead-letter → manual trigger works end-to-end and is test-pinned). Minimal fix: deferred `setStatus(StageIdle, …)` on error return in Run/RetryNote |
| `internal/retriever/milvus/client.go` | 141-169 | Watermark Query truncates at 10k rows silently (WR-02) | ⚠️ Warning | Latent PK-collision risk at scale; collection has append-amplification history |
| `internal/knowledge/ecl/stub.go` | 52-65 | ValidateExtraction does not gate `etl_` prefix/charset (WR-03) | ⚠️ Warning | Hallucinated non-etl_ names can mint tbl_ individuals; SanitizeLocal collisions can merge distinct tables |
| `internal/ontology/store/store.go` | 262-281 | Debounced snapshot writer swallows errors (WR-04) | ⚠️ Warning | Silent ABox durability failure on disk errors |
| `internal/server/admin_pipeline.go` | 103-165 | Run/retry leave no audit row (WR-05) | ⚠️ Warning | Most destructive admin actions unaudited vs repo audit posture |
| `internal/ontology/emitter/emitter.go` | 331-335 | Turtle escape misses newlines (WR-06) | ⚠️ Warning | Multi-line SKILL.md description can brick next boot via UpgradeIfNeeded→Restore |

Debt-marker scan: zero TBD/FIXME/XXX/HACK/PLACEHOLDER markers in any phase-39 file. Empty-implementation scan: all `return nil` hits are legitimate success returns. No stub patterns.

### Human Verification Required

See frontmatter `human_verification:` — 4 items:
1. Live LLM extraction F1 gate (documented Manual-Only verification; hermetic harness is the automated evidence)
2. Live Milvus E2E round-trip with RECREATE deploy sequence
3. UI-SPEC 13-item visual checklist walkthrough
4. CR-01 failure-path operability reproduction + fix-scheduling decision

### Gaps Summary

No gaps. All six roadmap success criteria are verified against the codebase, with independent re-runs of the chunker tests, the F1 harness, the FE pipeline suite, and the full hermetic gate (exit 0). The golden-set human review (D-GT-01) is recorded and signed off (2026-09-22, policy A writes-only, corrections applied to row 039). Requirements ONT-10..13 are fully accounted for with no orphans.

The phase's one critical code-review finding (CR-01) is confirmed real in `manager.go` but does not falsify any must-have: the retry button, the retry×3 → dead-letter mechanics, and the manual trigger all function; the defect is that certain batch-level failure paths leave the reported stage non-terminal, degrading failure-path UX rather than removing capability. It is carried as a warning with a minimal-diff fix shape, plus a human verification item to reproduce it and schedule the fix.

Note for the orchestrator: the working tree carries unrelated uncommitted modifications (`internal/knowledge/walk.go`, `internal/retriever/mongo/seed.go`, `internal/server/knowledge.go`) that are not phase-39 files; phase-39 code itself is verified as committed-on-branch state.

---

_Verified: 2026-09-22T13:35:00Z_
_Verifier: Claude (gsd-verifier)_
