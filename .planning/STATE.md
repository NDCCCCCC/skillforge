---
gsd_state_version: 1.0
milestone: v0.7.7
milestone_name: milestone
current_phase: 5
current_plan: 1
status: ready_to_plan
last_updated: 2026-08-11T14:54:45.813Z
progress:
  total_phases: 7
  completed_phases: 3
  total_plans: 10
  completed_plans: 13
  percent: 43
stopped_at: Phase 04 complete (3/2) — ready to discuss Phase 5
---

# State — SkillForge

**Last updated:** 2026-08-11
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 5 (skill-platform-&-full-knowledge-boundaries)
**Current Plan:** Not started
**Previous phase:** 04 (streaming-trust-ui) — COMPLETE 2026-08-11 (UAT 12/12, SECURITY verified / 0 open threats)
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** Phase 3 (Agentic Retrieval & Trust Eval) was deferred under MVP vertical-slice reordering — decide whether to (a) plan Phase 5 next per dependency chain, or (b) circle back to Phase 3 first. Run `/gsd:discuss-phase 5` or `/gsd:discuss-phase 3`.

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Ragas (Python sidecar) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4.

**License:** Apache-2.0.

**Current focus:** Phase 5 — Skill Platform & Full Knowledge Boundaries (semantic routing + hot reload + Skill Wizard + full 4-tier system + Agent state inspector + counter-evidence / redline). Note: Phase 3 (Agentic Retrieval & Trust Eval) was deferred under MVP reordering and remains open.
**See:** `.planning/PROJECT.md` (updated 2026-08-11 after Phase 4)

---

## Current Position

**Phase:** 5 (skill-platform-&-full-knowledge-boundaries) — READY TO PLAN
**Plan:** not started
**Status:** Ready to plan
**Just completed:** Phase 4 (streaming-trust-ui) — 3/3 plans, UAT 12/12, SECURITY verified (0 open threats)

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
| D6 | Ragas as Python sidecar for eval | Best-in-class faithfulness / context precision / answer relevance; Go bindings thin; sidecar is the established pattern |
| D7 | Milvus tier as mandatory metadata field | No collection-per-tier (couples ops); tier-filter expr before similarity is the hard barrier |
| D8 | Citation = chunk ID + span | Free-text citations are the #1 hallucination vector (P-COMPL-1) |
| D9 | Refusal-by-construction on missing tier-match | Default-deny; never improvise (GEVL-02, KNOW-04) |
| D10 | Self-RAG / CRAG self-verification | Re-read & verify claims before final answer (ARAG-03) |
| D11 | MongoDB 4.4+ as alternate retriever backend (`RETRIEVER_BACKEND=mongo`) | Internal Milvus is broken (image configured for external etcd). Tier filter via `{tier:{$in}}` query BEFORE in-Go cosine; D-09 holds structurally on both backends. Proves the `retriever.Retriever` abstraction is real. |
| D12 | D-04 transport-swap invariant (Phase 4) | Mock ↔ Default transport must produce byte-identical UIMessageChunk sequences from the same fixture; `transport-swap.test.tsx` gates the production swap to the real Hertz endpoint, preventing UI drift. |
| D13 | T-1-08 source-URL allowlist sanitization gate (Phase 4) | `source-url` href is the primary XSS vector; protocol+hostname allowlist at the SourceUrlPart boundary, rejected URLs render a blocked state, `<a>` always carries `rel=noopener noreferrer`. 7-assertion regression gate. |
| D14 | State boundary: Zustand store bans `messages` (Pitfall 7, Phase 4) | `useChat` owns the message stream; `app-store.ts` file-level comment forbids a `messages` field, eliminating the dual-source-of-truth class of bugs. |
| D15 | Wire-type compile-time drift alarm (Phase 4) | TS `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails the build if the SDK upstream renames any chunk discriminator — the compiler IS the drift alarm (cheaper than runtime contract tests alone). |

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

- Phases completed: 3 (Phase 1 spike; Phase 2 compliance vertical slice; Phase 4 streaming trust UI) — Phase 3 deferred under MVP reordering
- Requirements covered: 22 / 55 (Phase 2: 14; Phase 4: +FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08)
- Frontend tests: 83/83 green (12 files); UAT 12/12
- Plans run: 11 (4 Phase 1 + 4 Phase 2 incl. MONGO + 3 Phase 4)
- Last plan: 04-03 (Wave 3) — commits d65a3b8 + 081b743 + 73d26a7

---

## Session Continuity

**Resume point:** Phase 4 (streaming-trust-ui) is COMPLETE — UAT 12/12, SECURITY verified (0 open threats), transitioned to Phase 5. Next: decide Phase 3 (deferred Agentic Retrieval & Trust Eval) vs Phase 5 (Skill Platform) sequencing, then `/clear` → `/gsd:discuss-phase 5` (or `3`). Bidirectional tab nav + UI-SPEC §169 chunk empty-state + LeftSidebar real sessions + ChatInput `@`-search deferred to Phase 5 (see 04-03 Known Stubs + 260811-uaf-SUMMARY).

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions
- `.planning/REQUIREMENTS.md` — 55 v1 requirements + traceability
- `.planning/research/SUMMARY.md` — stack locked, pitfalls, recommended 7-phase order
- `.planning/ROADMAP.md` — phase structure (this file's sibling)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation. Last transition: Phase 4 → Phase 5 on 2026-08-11.*
