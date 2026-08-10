# SkillForge — Research Summary

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Research date:** 2026-08-10
**Mode:** Ecosystem — greenfield
**Audience:** Open-source personal brand
**Demo Skill:** 企业合规 / 制度审查
**Confidence:** MEDIUM-HIGH

---

## Research Artifacts

| File | Purpose | Confidence |
|------|---------|------------|
| `STACK.md` | Prescriptive stack: Eino v0.7.7, Hertz SSE, Milvus, BGE-M3, Vercel AI SDK v5, Ragas sidecar | HIGH |
| `FEATURES.md` | Table stakes vs differentiators vs anti-features; MVP 16 features | HIGH table stakes; MEDIUM differentiators |
| `ARCHITECTURE.md` | 6-layer system; Skill 6-field schema; Eino ADK mapping; 3 data flows | HIGH architecture; MEDIUM Eino API surface |
| `PITFALLS.md` | 35+ pitfalls across 8 groups; phase mapping | MEDIUM-HIGH |

---

## Key Findings

### Stack (locked)

- **Backend:** Go + Eino v0.7.7 (ADK GA since v0.5.0/2026.6) + Hertz v0.10+ (first-party SSE in `pkg/protocol/sse`) + Milvus (native Eino adapter) + BGE-M3 (CN RAG SOTA, self-hostable) + Ragas (Python sidecar for eval)
- **Frontend:** React 19 + Vite 8 + TS + Vercel AI SDK v5 (`UIMessageChunk` for text/reasoning/tool-call/source citation; 12KB vs LangChain.js 200-800KB) + shadcn + Tailwind v4
- **Skill format:** Anthropic Skills spec (name + description frontmatter) + 6-field extension (agent / reads / writes / steps / guardrails)
- **True-source separation:** Milvus metadata `tier` field, hard-filtered at retrieval (refusal-by-construction)
- **Anti-stack:** ❌ LangChain.js (bundle), ❌ OpenAI embeddings (CN wrong), ❌ pgvector (>1M hybrid fails), ❌ Custom SSE, ❌ Code-defined Skills, ❌ Height.app pattern (closed 2025.9)

### Table Stakes (MVP must include)

- Skill loader / registry / dispatcher / hot-reload
- Multi-agent orchestration via Eino ADK (ChatModelAgent / SequentialAgent / ParallelAgent)
- Streaming responses via Hertz SSE → Vercel AI SDK v5 wire format
- Tiered knowledge (权威 / 历史 / 个人) with per-Skill `reads` enforcement
- Agentic RAG: autonomous decide (retrieve / refuse / ask-clarifying / multi-hop)
- Citation tracking (chunk IDs + spans, not free text)
- Refusal with structured reason
- Per-Skill eval dashboard (Ragas faithfulness, context precision, answer relevance)
- Audit log per query

### Differentiators (the moat)

- **Skill 6-field schema** with explicit `reads/writes/guardrails` (denova's implicit boundaries made auditable + wizard-generatable)
- **Skill Wizard** (NL → SKILL.md generation; open-source killer feature)
- **Tiered knowledge as hard filter** (refusal-by-construction; can't accidentally mix knowledge tiers)
- **Ragas eval gating in CI** (faithfulness threshold blocks Skill promotion)
- **Compliance demo with structured verdicts** (合规 / 不合规 / 需补充 + clause citations + refusal + audit log + HITL for low-confidence)

### Watch Out For (top pitfalls)

- **P-COMPL-1 (TOP RISK): Compliance hallucination with fake citations** — citations must be chunk IDs; every claim must be verbatim-verifiable against retrieved text; refusal when no chunk supports
- **P-RAG-2: Knowledge tier bleed** — tier metadata mandatory at index time; filter injected BEFORE similarity search; default-deny if no tier-matched docs
- **P-EVAL-1: No measurable eval pipeline** — ship without RAGAS = no proof "we reduce hallucination"; kills demo's story
- Eino + AI SDK v5 stream churn — pin versions; contract tests
- CN compliance text retrieval / refusal thresholds need empirical calibration (not hard-coded)
- Audit log captures sensitive content — explicit retention / redaction / access policy required
- LLM-as-judge bias — calibrate Ragas faithfulness against human spot-check before trusting

---

## Synthesized Roadmap (7 phases)

1. **Integration Spike & Trust Contract** (1-2 days) — validate Eino v0.7.7, Hertz SSE, AI SDK v5 stream framing, Milvus tier filters, cancellation, citation events
2. **Compliance Vertical Slice** — one Skill / one authoritative tier / one endpoint / verifiable cited answer or structured refusal
3. **Agentic Retrieval & Trust Evaluation** — hybrid retrieval + reranking + self-verification + 50-100-case golden set + Ragas quality gates
4. **Streaming Trust UI** — React chat with visible retrieval progress + source passages + structured verdicts + explainable refusals
5. **Skill Platform & Full Knowledge Boundaries** — semantic routing + hot reload + Skill authoring + additional tiers + multi-Skill
6. **Eval Dashboard & Production Hardening** — CI regression gates + tracing + metrics + queryable audits + session isolation + retention / redaction
7. **Open-Source Release** — Docker Compose + sample data + docs + live demo + showcase Skills + published eval results

---

## Phase-Pitfall Mapping

| Phase | Primary Pitfalls Addressed |
|-------|----------------------------|
| 1 (Spike) | Version pinning; stream contract test; upfront risk on Eino API surface |
| 2 (Vertical slice) | P-SKILL-1..6 (Skill spec design); P-COMPL-1 (core compliance risk) |
| 3 (Retrieval + Eval) | P-RAG-1..4, P-AGENTIC-1,3,4, P-EVAL-1..4, P-COMPL-1,3,6 |
| 4 (UI) | P-FE-1..5, P-COMPL-3 (UI side-by-side), P-COMPL-5 (HITL badge) |
| 5 (Platform) | Full P-SKILL-*, P-EINO-1..3, full tier system (P-RAG-1) |
| 6 (Hardening) | P-RAG-2 (monitoring), P-COMPL-4 (audit trail), P-EINO-2 (multi-tenant), P-OSS-5 |
| 7 (Release) | P-OSS-1..6 (README / demo / Docker / contributing / license / eval results) |

---

## Research Flags

**Needs targeted phase-specific research:**

- Phase 1: Eino APIs in depth, Hertz-to-AI-SDK v5 transport, Milvus filter expression syntax
- Phase 3: Chinese eval dataset sourcing, BGE-M3 reranker choice (self-host via TEI vs API), Ragas judge calibration
- Phase 5: hot reload for in-flight conversations, cross-tier conflict policy, historical tier storage, multi-agent value proof
- Phase 6: audit privacy (retention / redaction / access control), reproducibility, CI evaluation cost

**Standard patterns (low research risk):**

- Phase 2: SKILL.md schema parsing, registry, read-only tools, structured outputs (well-established patterns)
- Phase 4: React chat + citation UX (Vercel AI SDK v5 stable + mature patterns)
- Phase 7: Docker Compose + OSS packaging (universal hygiene)

---

## Open Questions (consolidated from 4 researchers)

1. Whether `name` is part of the 6-field schema or an external manifest identity (Architecture)
2. Public / legally-safe CN compliance dataset acquisition strategy (Architecture, Pitfalls)
3. Exact citation-entailment verification method (Architecture)
5. Retrieval / refusal / confidence / HITL threshold values (Architecture, Pitfalls)
5. Metadata filter vs separate Milvus collections for tier isolation trade-off (Architecture)
6. Policy version conflict semantics (when 权威 and 历史 disagree) (Pitfalls)
7. Audit retention + redaction + access control requirements (Pitfalls)
8. Eval set composition: synthetic vs real-user sampling for compliance domain (Pitfalls)

---

## Confidence Summary

| Area | Level | Reason |
|------|-------|--------|
| Stack choices (Eino / Hertz / Milvus / BGE-M3 / Vercel AI SDK v5) | HIGH | Direct Context7 docs + Eino GA v0.5.0 confirmation |
| Eino ADK mapping to Skills | HIGH | All needed agent types documented in official docs |
| True-source separation pattern | MEDIUM-HIGH | Sound; specific Milvus filter syntax needs Phase 1 spike |
| Ragas-in-Go sidecar pattern | MEDIUM | Standard; specific dev workflow needs design |
| Skill 6-field schema | HIGH | Anthropic-published baseline + denova pattern validated |
| Eval & guardrails patterns | MEDIUM-HIGH | RAGAS limitations documented; LLM-judge bias known |
| Compliance demo architecture | HIGH | Top risk (P-COMPL-1) identified; mitigation well-defined |
| OSS distribution readiness | HIGH | Universal hygiene established |
| **Overall** | **MEDIUM-HIGH** | Strong stack; clear pitfalls; explicit unknowns flagged for phase-specific research |

---

## Files

- `D:\code\ClaudeCode\ai-agent\.planning\research\STACK.md`
- `D:\code\ClaudeCode\ai-agent\.planning\research\FEATURES.md`
- `D:\code\ClaudeCode\ai-agent\.planning\research\ARCHITECTURE.md`
- `D:\code\ClaudeCode\ai-agent\.planning\research\PITFALLS.md`
- `D:\code\ClaudeCode\ai-agent\.planning\research\SUMMARY.md` (this file)

---

*Note: gsd-research-synthesizer subagent policy blocks writing markdown report files; this SUMMARY.md was persisted by the orchestrator from the synthesizer's synthesized output (no content lost).*