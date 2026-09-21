---
phase: 39
slug: ecl-pipeline
status: filled
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-21
filled: 2026-09-21
---

# Phase 39 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + testify v1.11.1 (backend); vitest ^2.0.0 + @testing-library (web) — both pre-existing, Wave 0 install not needed |
| **Config file** | none for Go (stdlib); `web/package.json` scripts for vitest |
| **Quick run command** | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./internal/knowledge/ecl/... ./internal/ontology/... ./internal/server/ -count=1"` |
| **Full suite command** | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && /usr/local/go/bin/go test ./... -count=1"` + `pnpm -C web test -- --run` + `bash scripts/verify-phase-39.sh` |
| **Estimated runtime** | ~60–120 s full suite; ~10–20 s per-package quick runs |

---

## Sampling Rate

- **After every task commit:** Run the task's `<automated>` command from the Per-Task Verification Map (per-package, `-count=1`)
- **After every plan wave:** Run the full suite command (Go `./...` + web vitest)
- **Before `/gsd:verify-work`:** Full suite + `bash scripts/verify-phase-39.sh` must be green
- **Max feedback latency:** ~20 s (per-package quick runs)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 39-01-01 | 01 | 1 | ONT-13 | T-39-02 | DeleteSubject keeps byS/byP/byO symmetric (no phantom KG query results) | unit | `go test ./internal/ontology/store/ -count=1` | ✅ (suite; new cases this task) | ⬜ pending |
| 39-01-02 | 01 | 1 | ONT-12/13 | T-39-01 | TBox classes compile-time only; NoteChunk sentinel string testable | unit | `go test ./internal/ontology/... -count=1` | ✅ | ⬜ pending |
| 39-02-01 | 02 | 1 | ONT-13 | T-39-02/T-39-03 | Legacy docs seed zero-value section fields; 11 columns typed (VarChar not String) | unit (mock client) | `go test ./internal/retriever/... -count=1 && go build ./...` | ✅ (extend existing) | ⬜ pending |
| 39-02-02 | 02 | 1 | ONT-13 | T-39-01/T-39-02 | DeleteByExpr passes caller expr verbatim (caller %q-escapes); watermark is collection-wide (`id >= 0`) | unit (mock client) | `go test ./internal/retriever/milvus/ -run 'TestDeleteByExpr\|TestGetMaxIDCollection' -count=1 -v` | ❌ W1 (39-02 T2 creates) | ⬜ pending |
| 39-03-01 | 03 | 1 | ONT-10 | — | IsSecFragment dual-condition routing predicate | build/vet | `go build ./internal/knowledge/ecl/ && go vet ./internal/knowledge/ecl/` | ❌ W1 (39-03 T1 creates) | ⬜ pending |
| 39-03-02 | 03 | 1 | ONT-12 | T-39-03 | Stub kind-enum validated (update\|select\|insert\|create only) | unit | `go test ./internal/knowledge/ecl/ -run 'TestStub\|TestValidateExtraction' -count=1 -v` | ❌ W1 (39-03 T2 creates) | ⬜ pending |
| 39-03-03 | 03 | 1 | ONT-10/11 | T-39-04 | 185/185 contribution completeness; 26 notes; note 001 3/3 tables; ChunkMeta key contract | unit (real corpus, hermetic) | `go test ./internal/knowledge/ecl/ -run 'TestMergeBoundary\|TestNote001SQL\|TestSec01\|TestChunkMetaContract' -count=1 -v` | ❌ W1 (39-03 T3 creates) | ⬜ pending |
| 39-04-01 | 04 | 2 | ONT-12 | T-39-05 | LLM output parsed defensively (fences/invalid/enum violations rejected); 40K-rune guardrail | unit (fake Generator) | `go test ./internal/knowledge/ecl/ ./internal/llm/ -run 'TestLLMExtract\|TestExtract\|TestJSONObject' -count=1` | ❌ W2 (39-04 T1 creates) | ⬜ pending |
| 39-04-02 | 04 | 2 | ONT-12 | — | F1 math: identity=1.0, kind-mismatch=no-match, macro includes zero-scored notes | unit | `go test ./internal/knowledge/ecl/ -run TestF1 -count=1 -v` | ❌ W2 (39-04 T2 creates) | ⬜ pending |
| 39-04-03 | 04 | 2 | ONT-13 | T-39-06/T-39-08 | ABox replace idempotent + isolated per note + orphan cleanup; IRIs sanitized | unit (real store) | `go test ./internal/knowledge/ecl/ ./internal/ontology/... -count=1` | ❌ W2 (39-04 T3 creates) | ⬜ pending |
| 39-05-01 | 05 | 3 | ONT-13 | T-39-09 | ecl_failures parameterized SQL only; status lifecycle persists across reopen | unit (temp sqlite) | `go test ./internal/store/... ./internal/knowledge/ecl/ -run 'TestECLFailures\|TestFailureStore' -count=1` | ❌ W3 (39-05 T1 creates) | ⬜ pending |
| 39-05-02 | 05 | 3 | ONT-13 | T-39-13 | Sub-split keeps all Document Contents <= 3500 runes; metadata consistent across parts | unit | `go test ./internal/knowledge/ecl/ -run TestBuildDocuments -count=1 -v` | ❌ W3 (39-05 T2 creates) | ⬜ pending |
| 39-05-03 | 05 | 3 | ONT-12/13 | T-39-11/T-39-10 | Retry x3 → dead, batch continues; RetryNote busy/unknown sentinels; locked status DTO keys | unit | `go test ./internal/knowledge/ecl/ -run TestManager -count=1 -v` | ❌ W3 (39-05 T3 creates) | ⬜ pending |
| 39-06-01 | 06 | 4 | ONT-10/13 | T-39-01 | Routing branch precedes maxDocBytes; marker purge expr `section_count == 0` with quoted tier | unit | `go test ./internal/knowledge/ -count=1 && go build ./...` | ✅ (extend indexer_test.go) | ⬜ pending |
| 39-06-02 | 06 | 4 | ONT-13 | T-39-14/T-39-15 | Upgrade no-op when NoteChunk sentinel present; restore dedupes (Len stable across re-restore) | unit | `go test ./internal/ontology/... -run 'TestUpgrade\|TestRestore' -count=1 -v` | ❌ W4 (39-06 T2 creates) | ⬜ pending |
| 39-06-03 | 06 | 4 | ONT-10/11/13 | T-39-16/T-39-17 | Boot never auto-runs ECL (grep gate); ECL-off → deps.ECL nil | build/vet + regression | `go build ./... && go vet ./cmd/server/ && go test ./internal/knowledge/ ./internal/ontology/... -count=1` | ✅ | ⬜ pending |
| 39-07-01 | 07 | 4 | ONT-13 | T-39-18/T-39-19/T-39-10 | 403-before-503 code order; note_id ^[0-9]{3}$ gate before manager lookup | build/vet | `go build ./... && go vet ./internal/server/` | ❌ W4 (39-07 T1 creates) | ⬜ pending |
| 39-07-02 | 07 | 4 | ONT-13 | T-39-18/T-39-19 | Gate-order regression (403 with nil ECL); exact DTO key sets; 409/404/400 strings | unit (httptest) | `go test ./internal/server/ -run 'TestAdminPipeline' -count=1 -v` | ❌ W4 (39-07 T2 creates) | ⬜ pending |
| 39-08-01 | 08 | 5 | ONT-13 | — | API layer types + token header + typed errors | tsc | `cd web && pnpm exec tsc --noEmit -p tsconfig.json` | ❌ W5 (39-08 T1 creates) | ⬜ pending |
| 39-08-02 | 08 | 5 | ONT-13 | T-39-21/T-39-23 | Verbatim copy rendering as text nodes; no cancel/batch/auto-run controls (negative grep) | tsc + suite | `cd web && pnpm exec tsc --noEmit -p tsconfig.json` | ❌ W5 (39-08 T2 creates) | ⬜ pending |
| 39-08-03 | 08 | 5 | ONT-13 | T-39-21/T-39-23 | Cadence 2s/10s re-arm (fake timers); retry enablement only on dead; 403/503/409 copy | component (vitest) | `cd web && pnpm test -- --run` | ❌ W5 (39-08 T3 creates) | ⬜ pending |
| 39-09-01 | 09 | 6 | ONT-12 | T-39-24 | Drafter is LLM-free (grep gate); emits exactly 26 jsonl rows | CLI | `go run ./cmd/ecl-draft-golden -kb knowledge/sql-guides -out /tmp/ecl-golden-draft.jsonl && wc -l /tmp/ecl-golden-draft.jsonl` | ❌ W6 (39-09 T1 creates) | ⬜ pending |
| 39-09-02 | 09 | 6 | ONT-12 | T-39-24 | HUMAN REVIEW checkpoint (D-GT-01) | manual (blocking checkpoint) | — (checkpoint task) | — | ⬜ pending |
| 39-09-03 | 09 | 6 | ONT-10/11/12/13 | T-39-25/T-39-26 | F1 >= 0.8 vs reviewed golden; hermetic SC-1..6 gate exits 0 | harness + script | `go test ./internal/knowledge/ecl/ -run TestStubVsGoldenF1 -count=1 -v && bash scripts/verify-phase-39.sh` | ❌ W6 (39-09 T3 creates) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] Frameworks pre-existing (go test + testify in go.mod; vitest in web/package.json) — no installs needed
- [x] Existing suites cover regression baselines: `go test ./... -count=1`, `pnpm -C web test`
- [ ] Deferred-by-design (not Wave 0): golden jsonl + F1 harness test (39-09 T1/T3 — requires human review checkpoint per D-GT-01) and `scripts/verify-phase-39.sh` (39-09 T3 — requires all suites it gates). All other MISSING entries in the map are created by their owning task in the same plan, per the table.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live LLM extraction F1 (real GLM over the 26 notes) | ONT-12 / SC-2 | Intranet LLM is intermittent (env-intranet-service-flakiness); verify script stays hermetic — same disposition as Phase 38's live induction gate | Probe GLM, start server, POST /api/admin/pipeline/run with admin token, poll status to done, then run the F1 scorer against live extraction output; record macro-F1 in the phase verification doc |
| Live Milvus E2E: RECREATE=1 boot → Run ECL → retrieval shows merged sections | ONT-13 / SC-3 | Requires intranet Milvus + BGE-M3 + the one-shot RECREATE deploy sequence (documented in .env.example) | SKILLFORGE_MILVUS_RECREATE=1 boot → admin Run ECL → unset RECREATE → restart → confirm sql-guides retrieval returns merged-section chunks and graph shows note/tbl individuals |
| Visual pass of /admin/pipeline against UI-SPEC Visual Checklist | SC-4 | Visual confirmation | Walk the 13-item Visual Checklist in 39-UI-SPEC.md against a running server (idle / running / done / failures / 403 / 503 states) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (39-09-02 is the sanctioned blocking checkpoint)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (frameworks exist; deferred items are dependency-locked late-wave deliverables, documented above)
- [x] No watch-mode flags (all commands are one-shot `-count=1` / `--run`)
- [x] Feedback latency < 20 s for per-package runs
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planner-filled 2026-09-21 — executor updates Status column per task commit
