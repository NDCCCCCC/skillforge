---
phase: 2
slug: compliance-vertical-slice
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-11
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test map derived from `02-RESEARCH.md` §Validation Architecture (lines 805-843).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) + testify v1.11.1 (already in go.mod from Phase 1) |
| **Config file** | none — go.mod is set; no extra test runner config |
| **Quick run command** | `go test ./internal/...` |
| **Full suite command** | `go test ./...` (includes `integration/contract` version-pinned suite <10s) |
| **Estimated runtime** | ~10 seconds (contract suite) / ~3 seconds (internal unit tests) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green (`go test ./...`)
- **Max feedback latency:** 10 seconds

---

## Requirement / Success-Criterion → Test Map

Task IDs fill in during planning (`02-NN-PLAN.md`). The planner MUST assign each test below to a task's `<acceptance_criteria>` and the executor MUST implement it.

| SC / Req | Threat / Pitfall | Test | Type | Status |
|----------|------------------|------|------|--------|
| SC-1 / SYSK-01, SYSK-02 | P-SKILL-1..6 | `TestLoader` (drop SKILL.md → registers) + `TestValidate` (7-field schema rejects missing temperature, `*` wildcard, <3 prohibitions) | unit | ⬜ pending |
| SC-2 / ARAG-05, GEVL-02 | P-COMPL-1, P-RAG-2 | `TestNoChunksRefusal` — 0 authority chunks → structured refusal emitted AND no LLM call made (mock Runner, assert never invoked) | unit | ⬜ pending |
| SC-3 / CMPL-02, ARAG-04 | P-COMPL-1 (TOP) | `TestGroundingChecker` — fabricated span (not in chunk) → downgrade; verbatim span → pass; 全角/半角 mixing → pass after Normalize | unit | ⬜ pending |
| SC-4 / SYSK-07, CMPL-04 | P-RAG-2 (tier bleed) | `TestPersonalTierRefusal` + retained `TestContractMilvusFilter` (contract suite) | unit + contract | ⬜ pending |
| SC-5 / CMPL-03, CMPL-05 | P-COMPL-5 (HITL) | `TestGroundingChecker` — confidence capped at 0.89 + status → 需补充 + review_required event payload | unit | ⬜ pending |
| SC-6 / AGNT-03, ARAG-04 | — | `TestChatHandlerSSE` — asserts `source-document` chunk present in wire output with tier="权威" + chunk IDs | unit | ⬜ pending |
| SYSK-08 (temperature) | Pitfall 4 (temp defaulting) | `TestValidate` — SKILL.md missing temperature → rejected (no implicit default); verdict pipeline passes WithTemperature(0) | unit | ⬜ pending |
| D2-06 (Citations upgrade) | T-1-16 drift | UPDATE `TestContractStructuredOutputSchema` (contract suite) — citations now `[{chunk_id, span, tier}]` not `[]string` | contract | ⬜ pending |
| Wire format (AGNT-03) | T-1-11 (snake_case drop) | retained `TestUIMessageChunkShape` + `TestEmitChunkRejectsSnakeCase` | unit | ✅ exists (Phase 1) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `go.mod` already has `testing` (stdlib) + `testify v1.11.1` (Phase 1) — no framework install needed
- [x] `integration/contract/contract_test.go` exists (Phase 1) — UPDATE for D2-06 citation schema, do not replace

*Existing infrastructure covers the framework requirement. Wave 0 only adds test files alongside their implementation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live Milvus tier-filter retrieval | SC-4 (real vector search) | Internal-network Milvus (`MILVUS_ADDRESS`) not reachable in CI; unit tests mock the retriever | Set `MILVUS_ADDRESS` + `BGE_M3_URL` + `GLM_API_KEY`, run `go run ./cmd/spiked milvus` then `curl POST /api/chat` with a contact-info query; assert refusal |
| Live GLM-5.2 cited verdict | SC-3 (real LLM verdict) | Costs API quota; CI mocks the ChatModel | Set `GLM_API_KEY`, `curl POST /api/chat` with the travel-expense query; assert verdict status + citations with spans verbatim in retrieved chunks |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
