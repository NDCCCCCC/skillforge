---
phase: 3
slug: agentic-retrieval-trust-evaluation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-12
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `03-RESEARCH.md` § Validation Architecture + § Security Domain. Task IDs / Plan / Wave columns are filled by the planner; req-level rows are pre-populated from research.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify/assert` (Phase 2 baseline); Python `pytest` for the eval sidecar |
| **Config file** | none (Go uses default `go test`); `eval/pyproject.toml` declares pytest config |
| **Quick run command** | `go test ./internal/agent/... ./internal/store/... ./internal/knowledge/... -count=1 -race` |
| **Full suite command** | `go test ./... -count=1 -race` + `pytest eval/` |
| **Estimated runtime** | ~30s Go (with stubs) + Python sidecar |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/agent/... ./internal/store/... ./internal/knowledge/... -count=1 -race`
- **After every plan wave:** Run `go test ./... -count=1 -race` + `pytest eval/`
- **Before `/gsd:verify-work`:** Full suite (Go + Python) must be green; SC-8 cancel-timing assertion MUST pass deterministically (not flaky)
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

> Req-level rows pre-populated from `03-RESEARCH.md`. The planner refines Task ID / Plan / Wave once `03-XX-PLAN.md` files exist.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | — | — | AGNT-01 | — | Agent autonomously decides retrieve/answer via ReAct tool-calls; no hardcoded retrieve step | unit | `go test ./internal/agent/ -run TestAgentRetrievesAndAnswers -count=1 -race` | ❌ W0 | ⬜ pending |
| TBD | — | — | AGNT-02 | T-2-21 | `retrieve` is an `InvokableTool`; tiers enforced from `Manifest.ResolvedTiers()`; chunks returned as `[CHUNK id]` DATA framing | unit | `go test ./internal/agent/ -run TestRetrieveToolInvokable -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | AGNT-04 / SC-8 | DoS (cancel) | ctx cancel propagates mid-tool-call; SSE closes cleanly < 200ms | integration | `go test ./internal/server/ -run TestCancelPropagationMidTool -count=1 -timeout 30s` | ❌ W0 | ⬜ pending |
| TBD | — | — | AGNT-05 / SC-9 | Info Disclosure | `session_id` SQL partitioning; two concurrent same-user sessions see only own history | integration | `go test ./internal/store/ -run TestSessionIsolationConcurrent -count=1 -race` | ❌ W0 | ⬜ pending |
| TBD | — | — | ARAG-01 / SC-1 | T-1-11 (spoof) | `data-decision` chunk gated by `IsValidType()`; per-decision SSE emission | integration | `go test ./internal/server/ -run TestDecisionSSEChunks -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | ARAG-02 / SC-2 | — | `MaxStep=6` caps at 2 hops; final answer cites chunks from BOTH hops; no 3rd-hop escape | integration | `go test ./internal/agent/ -run TestMultiHopCitationAggregation -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | ARAG-03 / SC-3 | Tamper (floor) | Self-RAG critique targets semantic-reversal; `maybeCritique` CANNOT mutate Confidence/Status | unit | `go test ./internal/agent/ -run TestCritiqueSemanticReversal -count=1` + `TestCritiqueCannotRaiseConfidence` | ❌ W0 | ⬜ pending |
| TBD | — | — | KNOW-02 / SC-4 | — | Indexer rejects missing/invalid `tier` at insert; no path around | unit | `go test ./internal/knowledge/ -run TestIndexerRejectsMissingTier -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | KNOW-04 / SC-5 | — | Trust floor: 0 tier-matched chunks across all hops → forced structured refusal | unit | `go test ./internal/agent/ -run TestTrustFloorForcesRefusal -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | GEVL-03 / SC-7 | SQLi (Tamper) | Audit row queryable via SQL; parameterized queries (`$1,$2,…`), never `fmt.Sprintf`; audit table INSERT+SELECT only | integration | `go test ./internal/store/ -run TestAuditRowSQLQuery -count=1` | ❌ W0 | ⬜ pending |
| TBD | — | — | GEVL-04 / SC-6 | — | Ragas golden-set run reproducible (same input → same metrics within tolerance); `ragas==0.1.21` pinned | smoke (Python) | `pytest eval/test_runner_reproducibility.py -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | GEVL-05 | — | Golden set covers 3 categories (adversarial / refusal-required / multi-hop) | smoke (Python) | `pytest eval/test_golden_category_coverage.py -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/agent/agent_test.go` — covers AGNT-01/02, ARAG-01/02/03, KNOW-04/SC-5. Needs stubChatModel + stubRetriever (extend `internal/server/testhelpers_test.go`).
- [ ] `internal/agent/callbacks_test.go` — covers SC-1 decision SSE emission. Needs a fake `decisionEmitFunc` capturing calls.
- [ ] `internal/store/store_test.go` — covers SC-7 audit query + SC-9 session isolation. Uses in-memory SQLite (`:memory:` DSN).
- [ ] `internal/knowledge/indexer_know02_test.go` — covers KNOW-02/SC-4 tier rejection at insert.
- [ ] `internal/server/chat_session_test.go` — covers AGNT-04 cancel mid-tool + AGNT-05 session plumbing.
- [ ] `eval/test_runner_reproducibility.py` — SC-6 Ragas golden-set reproducibility.
- [ ] `eval/test_golden_category_coverage.py` — GEVL-05 category split (researcher-proposed ratio; over-weight refusal-required).
- [ ] Wave 0 framework install — `go get modernc.org/sqlite@latest` + `pip install ragas==0.1.21 langchain-openai pydantic pytest`. **Both gated by `checkpoint:human-verify`** (slopcheck unavailable in this Windows dev env; new packages tagged `[ASSUMED]` in RESEARCH.md).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Human spot-check of Ragas judge sample | GEVL-04 (D3-07) | GLM self-evaluation bias counter-measure requires human judgment on a sample of (query, answer, faithfulness score) triples | Run sidecar on 50-case golden set; manually review ~5 cases where faithfulness > 0.9 but the deterministic Go floor flagged a stripped claim — confirm judge is not optimism-skewed |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
