---
gsd_state_version: 1.0
milestone: v0.7.7
milestone_name: milestone
current_phase: 03
current_plan: "3 (just completed: 03-02)"
status: executing
last_updated: "2026-08-12T04:49:21.950Z"
progress:
  total_phases: 7
  completed_phases: 3
  total_plans: 17
  completed_plans: 19
  percent: 43
---

# State — SkillForge

**Last updated:** 2026-08-12 (Plan 04-04 gap-closure complete — Phase 4 TS-drift debt closed)
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 03
**Current Plan:** 3 (just completed: 03-02)
**Previous phase:** 04 (streaming-trust-ui) — COMPLETE 2026-08-11 (UAT 12/12, SECURITY verified / 0 open threats)
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** Continue Phase 3 Wave 2 — run `/gsd:execute-phase` for Plan 03-03 (agent emits data-decision via Eino callbacks) and Plan 03-04 (KNOW-02 indexer calls TierEnumByValue). 03-02 unblocked Plan 03-05 (server Store wiring) and Plan 03-06 (eval sidecar AuditRow reads).

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Go-native trust evaluator (internal/eval; faithfulness ports Ragas v0.1.21 prompt, Apache-2.0) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4. D6 reversed 2026-08-12: Python/Ragas sidecar eliminated in favor of single-language Go stack.

**License:** Apache-2.0.

**Current focus:** Phase 03 — agentic-retrieval-trust-evaluation
**See:** `.planning/PROJECT.md` (updated 2026-08-11 after Phase 4)

---

## Current Position

Phase: 03 (agentic-retrieval-trust-evaluation) — EXECUTING
Plan: 3 of 6 (just completed 03-02 — Store interface + SQLite audit/session backend, 3 TDD commits, 18m)
**Phase:** 5 (skill-platform-&-full-knowledge-boundaries) — READY TO PLAN (deferred; Phase 3 in progress)
**Plan:** not started
**Status:** Executing Phase 03 (Plans 03-01 + 03-02 COMPLETE; Wave 2 siblings 03-03 / 03-04 next)
**Just completed:** Plan 03-02 — `internal/store.Store` interface + `SQLiteStore` default impl (modernc.org/sqlite@v1.56.0, pure-Go no cgo). D11 mirror of retriever.Retriever + mongo (compile-time `var _ store.Store = (*SQLiteStore)(nil)`). audit table INSERT+SELECT only (T-3-AUDIT); `QueryAudit` 100% `?`-parameterized (T-3-SQLI regression: `'; DROP TABLE audit; --` survives in UserID/SkillID/RefusalReason); `GetSession WHERE session_id=?` = SC-9 isolation; `AppendTurn` idempotent via `UNIQUE+ON CONFLICT DO NOTHING`. 3/3 tasks, 3 commits (`76787c2` RED, `3e24bda` GREEN, `0810a7b` verify/harden). GEVL-03/SC-7 + AGNT-05/SC-9 unlocked at the store layer; Plan 03-05 (server wiring) + Plan 03-06 (eval sidecar AuditRow reads) unblocked.

**Phase 4 shipped (trust visualization front-end):**

- Three-panel React 19 UI (资料库 / 审查结论 / 对话) — `web/src/components/layout/{ThreePanel,LeftSidebar,RightChat}.tsx`
- VerdictCard (D-06) + RefusalCard (D-07) + ReasoningPart (D-08) + ChunkSourceView (D-01/FEUI-03) + SkillPicker (FEUI-04) + DataSubprogressPart (FEUI-06)
- Wire-type compile-time drift alarm + MockChatTransport (D-04 swap invariant) + T-1-08 source-URL sanitization gate
- 83/83 unit tests green; UAT 12/12 (chrome-devtools MCP + unit evidence); 1 gap fixed (commit `e757d6e`)
- Requirements covered: FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08

**Deferred to Phase 5 (from 04-03 Known Stubs + 260811-uaf):**

- LeftSidebar real multi-session + file tree (current = mock skeleton)
- ChatInput `@`-mention fuzzy search (current = drag-drop + 10MB validation only)
- Skill manager / settings UI (topbar chevron is a no-op stub)
- Bidirectional tab nav + UI-SPEC §169 chunk empty-state

**Open dependency note:** Phase 3 (Agentic Retrieval & Trust Eval) was skipped under MVP vertical-slice reordering (Phase 2 → Phase 4). Its scope (Self-RAG self-verification, multi-hop, Ragas golden-set gates, audit row) is still open and may need to be sequenced before or alongside Phase 5.

---

## Decisions (from PROJECT.md + SUMMARY.md)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Go + Eino (not Python / full TS) | User fluency in Go; denova fork as mature reference; surfaces agent engineering depth over web iteration speed |
| D2 | Skill-driven, not fixed-function agent | Promotes app → platform; updates adapt enterprise needs with no code change |
| D3 | Built-in demo Skill = 企业合规 / 制度审查 | Sharp pain (no tolerable hallucination); natural fit for true-source separation; refusal mechanism shines |
| D4 | Audience = OSS personal brand | Polish docs + easy start + measurable outcomes (not just interview-project) |
| D5 | Skill schema = 6 fields with explicit `reads` / `writes` | Makes denova's implicit boundaries auditable + wizard-generatable |
| D6 | Go-native trust evaluator (`internal/eval`) | **REVERSED 2026-08-12** (was: "Ragas as Python sidecar for eval"). Single-language stack preserved; faithfulness ports the Ragas v0.1.21 claim-extraction + entailment prompts VERBATIM (Apache-2.0, attributed in `eval/NOTICE`) so the headline number is industry-comparable ("Ragas 口径"); the other 3 metrics (answer_relevancy / context_precision / context_recall) use simple Go-authored yes/no judge prompts. Reversal rationale: 03-RESEARCH.md own build-vs-buy verdict estimated ~2000 LOC to match faithfulness alone for a FULL reimplementation, but that cost concentrates in the prompts — only faithfulness's claim extraction is genuinely calibration-sensitive (handles negation / modality / temporal claims / multi-sentence answers). Decision: port ONLY the Ragas faithfulness prompt; author Go judge prompts for the other 3; keep the stack single-language. Eliminates T-3-SC supply-chain checkpoint (no PyPI audit), drops ragas/langchain-openai dependency tree, avoids ragas 0.1.21 ↔ Python 3.13 install friction. |
| D7 | Milvus tier as mandatory metadata field | No collection-per-tier (couples ops); tier-filter expr before similarity is the hard barrier |
| D8 | Citation = chunk ID + span | Free-text citations are the #1 hallucination vector (P-COMPL-1) |
| D9 | Refusal-by-construction on missing tier-match | Default-deny; never improvise (GEVL-02, KNOW-04) |
| D10 | Self-RAG / CRAG self-verification | Re-read & verify claims before final answer (ARAG-03) |
| D11 | MongoDB 4.4+ as alternate retriever backend (`RETRIEVER_BACKEND=mongo`) | Internal Milvus is broken (image configured for external etcd). Tier filter via `{tier:{$in}}` query BEFORE in-Go cosine; D-09 holds structurally on both backends. Proves the `retriever.Retriever` abstraction is real. |
| D12 | D-04 transport-swap invariant (Phase 4) | Mock ↔ Default transport must produce byte-identical UIMessageChunk sequences from the same fixture; `transport-swap.test.tsx` gates the production swap to the real Hertz endpoint, preventing UI drift. |
| D13 | T-1-08 source-URL allowlist sanitization gate (Phase 4) | `source-url` href is the primary XSS vector; protocol+hostname allowlist at the SourceUrlPart boundary, rejected URLs render a blocked state, `<a>` always carries `rel=noopener noreferrer`. 7-assertion regression gate. |
| D14 | State boundary: Zustand store bans `messages` (Pitfall 7, Phase 4) | `useChat` owns the message stream; `app-store.ts` file-level comment forbids a `messages` field, eliminating the dual-source-of-truth class of bugs. |
| D15 | Wire-type compile-time drift alarm (Phase 4) | TS `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails the build if the SDK upstream renames any chunk discriminator — the compiler IS the drift alarm (cheaper than runtime contract tests alone). |
| D16 | `data-decision` SSE chunk discriminator (Phase 3 / Plan 03-01) | Registers the ARAG-01 / SC-1 observability substrate — a per-hop wire-level slot for agent autonomous decisions (retrieve/answer/refuse/forced_stop). Mirrored Go + TS in one commit per the D15 two-PR rule; emitted by Plan 03-03's Eino callbacks (`newDecisionCallback`), parsed by the Phase 4 React UI's `SkillForgeChunk` union. |
| D17 | TierEnum reverse-lookup helpers (Phase 3 / Plan 03-01) | `skill.TierEnumByValue` + `skill.TierEnumValues` expose the 4-tier enum for backend-agnostic validation. KNOW-02 enforcement at `internal/knowledge.Index` (Plan 03-04) calls `TierEnumByValue(doc.Tier)` — fail-closed default-deny on any tier outside {权威, 历史, 个人, 草稿}. |
| D18 | Store interface-first + SQLite default (Phase 3 / Plan 03-02) | `internal/store.Store` + `internal/store/sqlite.SQLiteStore` mirror the D11 retriever pattern (interface in parent pkg, impl in subpkg, compile-time `var _ store.Store = (*SQLiteStore)(nil)`). modernc.org/sqlite@v1.56.0 = pure-Go (no cgo) zero-ops default; PG-ready drop-in without caller changes. audit table is INSERT+SELECT only (T-3-AUDIT); `QueryAudit` is 100% `?`-parameterized (T-3-SQLI); `GetSession WHERE session_id=?` is the SC-9 isolation boundary; `AppendTurn` idempotent via `UNIQUE(session_id,role,ts)+ON CONFLICT DO NOTHING`. Plan 03-05 (server wiring) + Plan 03-06 (eval sidecar AuditRow reads) unblocked. |

---

## Accumulated Context

**Open questions (from research SUMMARY.md):**

1. Whether `name` is part of the 6-field schema or an external manifest identity
2. Public / legally-safe CN compliance dataset acquisition strategy
3. Exact citation-entailment verification method
4. Retrieval / refusal / confidence / HITL threshold values (empirical calibration required)
5. Metadata filter vs separate Milvus collections for tier isolation trade-off
6. Policy version conflict semantics (when 权威 and 历史 disagree)
7. Audit retention + redaction + access control requirements
8. Eval set composition: synthetic vs real-user sampling for compliance domain

**Research flags (phase-specific research needed):**

- Phase 1: Eino APIs in depth, Hertz-to-AI-SDK v5 transport, Milvus filter expression syntax
- Phase 3: Chinese eval dataset sourcing, BGE-M3 reranker choice (self-host via TEI vs API), Ragas judge calibration
- Phase 5: hot reload for in-flight conversations, cross-tier conflict policy, historical tier storage, multi-agent value proof
- Phase 6: audit privacy (retention / redaction / access control), reproducibility, CI evaluation cost

---

## Blockers

(none)

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260811-uaf | Fix CenterPanel tab indicator sync with centerView.kind (04-UAT #8) | 2026-08-11 | e757d6e | [260811-uaf-fix-centerpanel-tab-indicator-sync-with-](./quick/260811-uaf-fix-centerpanel-tab-indicator-sync-with-/) |
| 260812-cng | Finalize v0.7.7 dep-upgrade tail (drop scripts/test, reconcile STATE.md drift, track milestone audit) | 2026-08-12 | 6bbfa61 | [260812-cng-go-scripts-test-state-md-frontmatter-hea](./quick/260812-cng-go-scripts-test-state-md-frontmatter-hea/) |

---

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| 04 | 1 | 75m | 4 tasks / 38 files — Wave 1 foundation (wire types + mock transport + parts) |
| 04 | 2 | 50m | 3 tasks — Wave 2: VerdictCard + RefusalCard + ReasoningPart + CenterPanel + locked-file reconciliation (24 new / 8 mod) |
| 04 | 3 | 50m | 3 tasks — Wave 3: ThreePanel + SkillPicker + ChunkSourceView wire + transport-swap integration test (17 new / 8 mod) |
| 02 | 3 | — | verdict pipeline + cmd/server |
| 02 | 2 | — | skill package + corpus |
| 02 | 1 | — | grounding + verdict schema |
| 02 | 04-MONGO | 13m | 5 commits — MongoDB retriever backend (RETRIEVER_BACKEND=mongo). See 02-04-MONGO-SUMMARY.md |
| 04 (quick) | 260811-uaf | ~20m | CenterPanel tab-indicator ↔ centerView.kind sync — closed UAT #8 (83/83 green). See quick/260811-uaf-*-SUMMARY.md |
| 03 | 01 | 7m | 2 TDD tasks / 4 commits / 5 files (2 created + 3 modified) — `data-decision` SSE chunk discriminator (Go `wire.ValidTypes` 16→17 + exported `DecisionPayload` + TS `SkillForgeChunk` variant) + `skill.TierEnumByValue`/`TierEnumValues` reverse-lookup helpers (additions only). ARAG-01/SC-1/KNOW-01 substrates unlocked. See 03-01-SUMMARY.md |
| 03 | 02 | 18m | 3 TDD tasks / 3 commits / 8 files (6 created + 2 modified) — `internal/store.Store` interface + `SQLiteStore` default impl (modernc.org/sqlite@v1.56.0 pure-Go, no cgo). D11 mirror of retriever + mongo. audit INSERT+SELECT only (T-3-AUDIT); `QueryAudit` 100% `?`-parameterized (T-3-SQLI regression: DROP TABLE survives in 3 filter fields); SC-9 isolation at `WHERE session_id=?`; `AppendTurn` idempotent via UNIQUE+ON CONFLICT. GEVL-03/SC-7 + AGNT-05/SC-9 unlocked at store layer. See 03-02-SUMMARY.md |
| 04 | 04 | 12m | 3 tasks / 3 commits / 6 modified — wire-contract TS realignment (gap closure, `requirements: []`). `cd web && npx tsc --noEmit` 21 → 0 errors; vitest 84/84; build 411.12 KB / 120.47 KB gzip (04-03 band). D15 alarm STRENGTHENED (`finishReason?: string` → `FinishReason`); Pitfall 1/3 guards preserved via string-cast; ToolPart toolName structural-type read; transport-swap `FixtureDrivenTransport` zero-arg contract. Closes Phase 4 TS-drift debt. See 04-04-SUMMARY.md |

- Phases completed: 3 (Phase 1 spike; Phase 2 compliance vertical slice; Phase 4 streaming trust UI) — Phase 3 deferred under MVP reordering, now in progress
- Requirements covered: 24 / 55 (Phase 2: 14; Phase 4: +FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08; Phase 3 03-01: +ARAG-01 substrate, +KNOW-01 substrate)
- Frontend tests: 84/84 green (14 files); UAT 12/12; `cd web && npx tsc --noEmit` exit 0 (debt-free since 04-04)
- Plans run: 14 (4 Phase 1 + 4 Phase 2 incl. MONGO + 4 Phase 4 incl. 04-04 gap-closure + 2 Phase 3)
- Last plan: 04-04 (Phase 4 gap-closure) — commits d3ac4ed + 6857996 + e68b942

---

## Session Continuity

**Resume point:** Plan 03-02 (internal/store Store interface + SQLite audit/session backend) is COMPLETE — 3/3 tasks, 3 TDD commits (`76787c2` RED, `3e24bda` GREEN, `0810a7b` verify/harden). GEVL-03/SC-7 (audit SQL-queryable, T-3-SQLI regression-locked) and AGNT-05/SC-9 (session_id SQL partitioning under -race) unlocked at the store layer. Next: run Wave 2 plans — Plan 03-03 (agent emits data-decision via Eino callbacks, consumes 03-01's `wire.DecisionPayload`) and Plan 03-04 (KNOW-02 indexer calls 03-01's `skill.TierEnumByValue`). Plan 03-05 (server wires `Store.PutAudit` + `GetSession` + `AppendTurn`) and Plan 03-06 (eval sidecar reads `AuditRow` for JSONL) also unblocked. **Phase 4 TS-drift debt CLOSED 2026-08-12 via Plan 04-04** — `cd web && npx tsc --noEmit` exits 0 (21 → 0 errors), vitest 84/84, build 411.12 KB / 120.47 KB gzip. See `.planning/phases/04-streaming-trust-ui/04-04-SUMMARY.md`.

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions
- `.planning/REQUIREMENTS.md` — 55 v1 requirements + traceability
- `.planning/research/SUMMARY.md` — stack locked, pitfalls, recommended 7-phase order
- `.planning/ROADMAP.md` — phase structure (this file's sibling)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation. Last transition: Phase 4 → Phase 3 (resumed) on 2026-08-11. Plan 03-01 complete 2026-08-11. Plan 03-02 complete 2026-08-12. Plan 04-04 (Phase 4 gap-closure, wire-contract TS realignment) complete 2026-08-12 — `cd web && npx tsc --noEmit` now exits 0; Phase 4 debt-free.*
