---
phase: 10
slug: p8-1
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-18
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (BE)** | `go test` 1.22+ with `testify/assert` + `internal/testdata` golden fixtures |
| **Framework (FE)** | vitest + @testing-library/react |
| **Config file** | None — existing infrastructure ships with project (Phase 3 SC-6 + Phase 4 vitest 280/280 + Phase 8 eino v0.9.14 hermetic four-gate) |
| **Quick run command (BE)** | `go test ./internal/history/... ./internal/server/... -count=1` |
| **Quick run command (FE)** | `cd web && npx vitest run --reporter=basic` |
| **Full suite command (BE)** | `go test ./... -count=1` |
| **Full suite command (FE)** | `cd web && npx vitest run` |
| **Type-check (FE)** | `cd web && npx tsc --noEmit` |
| **Build (FE)** | `cd web && npx vite build` |
| **Spike gate command** | `go run ./cmd/spike-history --summarize --seed=42` |
| **Estimated runtime (quick)** | ~30s |
| **Estimated runtime (full suite)** | ~3 min (hermetic) |
| **Estimated runtime (spike gate)** | ~75 min (live GLM-5.2 + Milvus) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/history/... ./internal/server/... -count=1` + `cd web && npx vitest run --reporter=basic`
- **After every plan wave:** Run full suite (`go test ./... -count=1` + `cd web && npx vitest run`)
- **Before `/gsd:verify-work`:** Full suite green + spike gate exit 0
- **Max feedback latency:** 90s (BE full suite) / 60s (FE vitest) / 75min (spike live gate)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | MEM-04 | T-10-HIST-1 | Project() output has zero `[CHUNK ` literal | unit | `go test ./internal/history/... -run TestProject_ZeroChunkRefLeak -count=1` | ✅ | ⬜ pending |
| 10-01-02 | 01 | 1 | MEM-04 | T-10-HIST-1 | verdictJSON → conclusionText deterministic render | unit | `go test ./internal/history/... -run TestRenderVerdict -count=1` | ✅ | ⬜ pending |
| 10-01-03 | 01 | 1 | MEM-04 | T-10-HIST-1 | refusalReason → neutral placeholder (zero semantic words) | unit | `go test ./internal/history/... -run TestRenderRefusal -count=1` | ✅ | ⬜ pending |
| 10-01-04 | 01 | 1 | MEM-05 | T-10-BUDGET-1 | Token-budget-driven window truncate oldest first | unit | `go test ./internal/history/... -run TestBudgetTruncation -count=1` | ✅ | ⬜ pending |
| 10-01-05 | 01 | 1 | MEM-04 | T-10-ISOLATION-1 | RefusalClassifierInput struct has no History field | compile | `grep -E 'History\s+[]schema' internal/server/classifier/types.go` returns empty | ✅ | ⬜ pending |
| 10-01-06 | 01 | 1 | MEM-04 | T-10-ISOLATION-1 | grep guard against history leaks into classifier | unit | `go test ./internal/server/... -run TestClassifierNoHistoryLeak -count=1` | ✅ | ⬜ pending |
| 10-02-01 | 02 | 2 | MEM-01 | T-10-REWRITE-1 | Query rewriter produces rewrite for anaphoric input | unit | `go test ./internal/server/... -run TestRewriteAnaphora -count=1` | ❌ W0 | ⬜ pending |
| 10-02-02 | 02 | 2 | MEM-01 | T-10-REWRITE-1 | Query rewriter failure → best-effort original query | unit | `go test ./internal/server/... -run TestRewriteFallback -count=1` | ❌ W0 | ⬜ pending |
| 10-02-03 | 02 | 2 | MEM-04 | T-10-CLASSIFIER-1 | Refusal classifier decides refuse vs allow structurally | unit | `go test ./internal/server/... -run TestClassifierDecision -count=1` | ❌ W0 | ⬜ pending |
| 10-02-04 | 02 | 2 | MEM-03 | T-10-CITE-1 | Citation whitelist code-enforced (current-Run only) | unit | `go test ./internal/server/... -run TestGroundingWhitelist -count=1` | ✅ (existing) | ⬜ pending |
| 10-02-05 | 02 | 2 | MEM-01 | T-10-PIPELINE-1 | streamAndAssemble multi-turn messages prefix assembly | unit | `go test ./internal/server/... -run TestStreamMultiTurn -count=1` | ❌ W0 | ⬜ pending |
| 10-02-06 | 02 | 2 | MEM-05 | T-10-BUDGET-1 | Budget truncation emits history_budget_truncated data-decision | unit | `go test ./internal/server/... -run TestBudgetEvent -count=1` | ❌ W0 | ⬜ pending |
| 10-02-07 | 02 | 2 | MEM-01 | T-10-REWRITE-1 | Rewrite emits history_query_rewritten data-decision | unit | `go test ./internal/server/... -run TestRewriteEvent -count=1` | ❌ W0 | ⬜ pending |
| 10-03-01 | 03 | 3 | MEM-02 | T-10-FE-RESTORE-1 | GET /api/session/:id/history returns structured payload | integration | `go test ./internal/server/... -run TestSessionHistoryAPI -count=1` | ❌ W0 | ⬜ pending |
| 10-03-02 | 03 | 3 | MEM-02 | T-10-FE-RESTORE-1 | useChat initialMessages populates from API | unit | `cd web && npx vitest run --reporter=basic chat-restore` | ❌ W0 | ⬜ pending |
| 10-03-03 | 03 | 3 | MEM-02 | T-10-FE-RESTORE-1 | Skill auto-switch on history restore | unit | `cd web && npx vitest run --reporter=basic skill-auto-switch` | ❌ W0 | ⬜ pending |
| 10-03-04 | 03 | 3 | MEM-02 | T-10-FE-RESTORE-1 | FE TypeScript types remain zero-error | typecheck | `cd web && npx tsc --noEmit` exit 0 | ✅ | ⬜ pending |
| 10-04-01 | 04 | 4 | MEM-01..05 | T-10-SPIKE-1 | Live 35-case spike re-run: 4-metric drift <5% | spike | `go run ./cmd/spike-history --summarize --seed=42` exit 0 | ✅ | ⬜ pending |
| 10-04-02 | 04 | 4 | MEM-01..05 | T-10-SPIKE-1 | Live spike: refusal_accuracy drift <10pp | spike | (same command, gate-check) | ✅ | ⬜ pending |
| 10-04-03 | 04 | 4 | MEM-03 | T-10-SPIKE-1 | Live spike: SC-3 zero-penetration 0/10 PASS | spike | (same command, penetration check) | ✅ | ⬜ pending |
| 10-04-04 | 04 | 4 | MEM-04 | T-10-LOCK-1 | Frozen baseline lock sha256 50bb1688 byte-identical at HEAD | byte-compare | `git diff eval/frozen/compliance.lock.json` empty | ✅ | ⬜ pending |
| 10-04-05 | 04 | 4 | MEM-01..05 | T-10-REGRESSION | Hermetic regression: full suite green | suite | `go test ./... -count=1` + `cd web && npx vitest run` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/history/projection.go` + `projection_test_fixtures.go` (Phase 9 stub) — extend with verdict renderer + neutral placeholder + budget window
- [ ] `internal/server/classifier/types.go` — RefusalClassifierInput struct + var _ sentinel + grep guard
- [ ] `internal/server/classifier/classifier.go` — NewRefusalClassifier + Classify method + JSON response_format
- [ ] `internal/server/rewrite/rewrite.go` — NewQueryRewriter + Rewrite method + JSON response_format
- [ ] `internal/server/pipeline.go` — extend streamAndAssemble with multi-turn messages prefix
- [ ] `internal/server/chat.go` — extend chatHandler with rewrite → classifier → generate chain
- [ ] `internal/server/handlers_session_history.go` — GET /api/session/:id/history handler
- [ ] `internal/server/handlers_session_history_test.go` — in-process HTTP test
- [ ] `web/src/lib/chat-restore.ts` — fetch history + transform to initialMessages
- [ ] `web/src/lib/chat-restore.test.ts` — vitest unit test
- [ ] `web/src/hooks/use-skill-auto-switch.ts` — auto-switch on history restore
- [ ] `web/src/hooks/use-skill-auto-switch.test.ts` — vitest unit test
- [ ] `internal/server/pipeline_default_test.go` — extend TestStreamMultiTurn + TestBudgetEvent + TestRewriteEvent

*Existing infrastructure covers:** `internal/eval` library (Phase 3) + `cmd/spike-history` (Phase 9) + vitest 280/280 baseline (Phase 7) + Go-native test fixtures.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-of-phase spike live re-run | MEM-04 (refusal drift) + MEM-03 (cite-penetration) | Live GLM-5.2 + Milvus stack requires Docker compose + GLM_API_KEY + BGE_M3_URL + Milvus tier-filter setup; cannot be hermetic | `docker compose up -d milvus etcd minio` + `ollama pull bge-m3` + `pop .env` + `go run ./cmd/spike-history --summarize --seed=42`; observe `reports/summary.md` exit 0 + 4 gates PASS |
| Per-call cost monitoring | MEM-05 | Real $$; cost-cap threshold requires human confirmation before re-run | During spike, watch `[cost]` log; abort manually if >$5.00/segment |

*All other behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s (BE) / 60s (FE) / 75min (spike)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
