---
gsd_state_version: 1.0
milestone: v0.7.7
milestone_name: milestone
status: executing
last_updated: "2026-08-11T16:00:00.000Z"
progress:
  total_phases: 7
  completed_phases: 1
  total_plans: 8
  completed_plans: 6
  percent: 22
---

# State — SkillForge

**Last updated:** 2026-08-11
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Ragas (Python sidecar) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4.

**License:** Apache-2.0.

---

## Current Position

Phase: 02 (compliance-vertical-slice) — EXECUTING
Plan: 2 of 4 completed
**Phase:** 2
**Plan:** 2 (skill package + corpus) — COMPLETE 2026-08-11
**Status:** Ready to execute Plan 3 (verdict pipeline + cmd/server)
**Next action:** `/gsd:execute-phase 02` (continue to 02-03 / 02-04)

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

---

## Performance Metrics

- Phases completed: 1 / 7 (Phase 1 spike done; Phase 2 Plans 1-2 done)
- Requirements covered: 8 / 55 (CMPL-02/03/04, ARAG-04 from 02-01; SYSK-01/02/08, CMPL-01 from 02-02)
- Plans run: 6 (4 from Phase 1 spike + 2 from Phase 2 = 02-01 + 02-02)
- Last plan: 02-02 (skill package + corpus) — committed at db43180 + 76ef812 + corpus + 5c0ae74 on branch worktree-agent-a4f798f416a88c35b

---

## Session Continuity

**Resume point:** Open `.planning/ROADMAP.md` to see the current phase. Run `/gsd:plan-phase 1` to generate the Phase 1 spike plan.

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions
- `.planning/REQUIREMENTS.md` — 55 v1 requirements + traceability
- `.planning/research/SUMMARY.md` — stack locked, pitfalls, recommended 7-phase order
- `.planning/ROADMAP.md` — phase structure (this file's sibling)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation.*
