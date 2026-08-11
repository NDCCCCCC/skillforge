---
gsd_state_version: 1.0
milestone: v0.7.7
milestone_name: milestone
current_phase: 04
current_plan: 1
status: ready_to_plan
last_updated: 2026-08-11T14:41:00.705Z
progress:
  total_phases: 7
  completed_phases: 3
  total_plans: 10
  completed_plans: 13
  percent: 43
stopped_at: Phase 02 complete (6/4) — ready to discuss Phase 04
---

# State — SkillForge

**Last updated:** 2026-08-11
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 04
**Current Plan:** Not started
**Total Plans in Phase:** 3
**Plan Status:** Plan 1 COMPLETE 2026-08-11 — ready for Wave 2 (04-02)
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** `/gsd:verify-work 4` to close Phase 4 — UAT is now 12/12 after quick task `260811-uaf` (CenterPanel tab-indicator sync). Then `/gsd:secure-phase 04` if config requires, then advance.

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Ragas (Python sidecar) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4.

**License:** Apache-2.0.

---

## Current Position

Phase: 04 (streaming-trust-ui) — EXECUTING
Plan: 1 of 3
**Phase:** 2
**Plan:** 3 (verdict pipeline + cmd/server) — COMPLETE 2026-08-11
**Status:** Ready to plan
**Next action:** `/gsd:execute-phase 02` (continue to 02-04)

**02-01 deliverables:**

- internal/spike/ retired (graduated to internal/{verdict, wire, retriever})
- D2-06 Verdict.Citations upgraded to []Citation{{ChunkID, Span, Tier}}
- D2-01 GroundingChecker (Normalize + substring + D2-02 downgrade)
- 5 adversarial grounding tests + D2-05 normalize tests
- integration/contract test pinned to []Citation schema

**02-02 deliverables:**

- internal/skill/ package: Manifest (7-field, *float32 Temperature) + Validate + TierEnum + ResolveTiers + Loader.Scan + parseFrontmatter
- skills/compliance/SKILL.md (canonical 7-field showcase — CMPL-01)
- knowledge/authority/{travel_expense,contract_approval,non_compete,annual_leave,expense_reimbursement_scope,business_gift_prohibition,anti_corruption_clause}.md (7 hand-authored clauses)
- knowledge/{historical,personal,draft}/.gitkeep (empty tier dirs preserved — personal/ IS the SC-4 refusal mechanism)
- testdata/skills/*/SKILL.md reconciled to 7-field (D2-10)

**02-03 deliverables (THE Phase 2 heart):**

- internal/knowledge Index (Pattern 7 one-doc-one-chunk loader; BGE-M3 production embed; skill.TierEnum dir→tier mapping)
- internal/server: Deps + Embedder/Searcher/Runner interface seams + New() (Pattern 1)
- internal/server VerdictPipeline 10-step orchestrator (Pattern 6) — embed → retrieve → refuse-or-LLM → ground → emit
- internal/server chatHandler + emitVerdictStream + emitRefusal + drainFinalContent (Pattern 2 SSE wiring)
- cmd/server long-running Hertz binary reading 5 env vars (D2-09 / Pattern 1); h.Spin() blocks until SIGINT/SIGTERM
- 7 acceptance tests pinning SC-2/3/4/5/6 + AGNT-03 + ARAG-05 + GEVL-01/02 + SYSK-07 + SYSK-08 + CMPL-05
- 8 unit tests in internal/knowledge/indexer_test.go pinning the 4 indexer success criteria + static-analysis guard against EmbedQueryDeterministic fallback

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
| 02 | 3 | — | verdict pipeline + cmd/server |
| 02 | 2 | — | skill package + corpus |
| 02 | 1 | — | grounding + verdict schema |
| 02 | 04-MONGO | 13m | 5 commits — MongoDB retriever backend (RETRIEVER_BACKEND=mongo). See 02-04-MONGO-SUMMARY.md |
| 04 (quick) | 260811-uaf | ~20m | CenterPanel tab-indicator ↔ centerView.kind sync — closed UAT #8 (83/83 green). See quick/260811-uaf-*-SUMMARY.md |

- Phases completed: 1 / 7 (Phase 1 spike done; Phase 2 Plans 1-3 done)
- Requirements covered: 14 / 55 + FEUI-01/02/06, T-1-08 (CMPL-02/03/04/05, ARAG-04/05, AGNT-03, GEVL-01/02, SYSK-01/02/07/08, CMPL-01)
- Plans run: 8 (4 from Phase 1 spike + 3 from Phase 2 + 1 from Phase 4 Wave 1)
- Last plan: 02-04-MONGO (extension) — committed at 3585f30 + 4743a4b + 0546db2 + 2169266

---

## Session Continuity

**Resume point:** Phase 4 UAT is 12/12 (tab-sync fix `260811-uaf` done, commit `e757d6e`). Next: `/gsd:verify-work 4` to close the phase → check `/gsd:secure-phase 04` gate → advance. The `HANDOFF.json` + `.continue-here.md` pause artifacts were removed once the fix landed. Bidirectional tab nav + UI-SPEC 169 chunk empty-state deferred (see `260811-uaf-SUMMARY.md`).

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions
- `.planning/REQUIREMENTS.md` — 55 v1 requirements + traceability
- `.planning/research/SUMMARY.md` — stack locked, pitfalls, recommended 7-phase order
- `.planning/ROADMAP.md` — phase structure (this file's sibling)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation.*
