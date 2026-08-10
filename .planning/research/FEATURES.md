# Feature Landscape: SkillForge

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Researched:** 2026-08-10
**Mode:** Ecosystem — features dimension
**Confidence:** HIGH for table-stakes features (well-established 2026 ecosystem); MEDIUM for differentiator scope (pivot from denova + 2026 trends); MEDIUM for anti-feature rationale (judgment calls).

---

## Executive Summary

SkillForge sits at the intersection of three 2026 mega-trends: **Agentic RAG** (autonomous retrieval with refusal), **Skills-as-a-platform** (post-MCP standardization led by Anthropic Skills spec, Oct 2025), and **enterprise guardrails** (audit + eval). The feature landscape is well-mapped for "agent framework" projects, but the **Skill-driven customization** is the novel axis — that's where differentiators live.

Key tension: **Anthropic already shipped Skills in Oct 2025**. SkillForge cannot compete on "we have Skills" — it must pivot to **enterprise** (compliance domain, audit trail, eval at scale) and **structured** (6-field SKILL.md with explicit reads/writes, not just unstructured markdown instructions). The differentiation is **discipline + structure**, not the existence of the abstraction.

**MVP must include**: Skill loading + parsing + intent routing (table stakes), one demo Skill end-to-end (table stakes), refusal + citation (table stakes for credibility), minimal eval dashboard (differentiator — most open-source agents skip this), one-click Docker + README (table stakes for open-source).

**Defer**: Skill marketplace UI, multi-agent orchestration v2, custom eval dashboards beyond basics, anything requiring multi-tenant.

---

## Table Stakes (Must Have)

These are baseline expectations. Missing them = product feels incomplete or unsafe.

### 1. Skill System

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Parse SKILL.md** (markdown + YAML frontmatter) | Anthropic Skills spec (Oct 2025) made this the industry standard. Without it, the platform has no abstraction. | **Low** | Required: `name`, `description`. SkillForge adds 4 more fields. |
| **Skill load/unload from disk** | Without hot-reload, iteration requires server restart — kills authoring UX. | **Low** | File watcher pattern (e.g., fsnotify in Go). |
| **Intent routing on `description`** | Skills must activate by semantic match. Without it, user picks skills manually → not a platform. | **Medium** | Embedding-based similarity; threshold tuning required. |
| **Validation against 6-field schema** | User-defined Skills must not break the system. Schema validation = safety net. | **Low** | Reject on load with clear error message. |
| **Skill enable/disable toggle** | Operators must be able to turn off misbehaving Skills without deleting them. | **Low** | Boolean flag persisted alongside Skill metadata. |

### 2. Agent Core

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Streaming responses (SSE)** | UX baseline for chat applications; users expect token-by-token rendering. | **Low** | Hertz supports SSE natively; pair with Vercel AI SDK `useChat` on frontend. |
| **Tool calling (function-calling protocol)** | Any non-trivial Agent needs tools (search, retrieve, write). Eino ADK provides this. | **Medium** | Standard JSON-schema tool definitions. |
| **Multi-turn conversation memory (short-term)** | Without it, the Agent forgets prior messages in the same session. | **Low** | In-context, capped by model window. |
| **Cancel / interrupt running response** | Users will hit "stop" mid-stream. Without graceful cancel, threads leak. | **Medium** | SSE abort + Eino ADK cancellation context. |
| **Structured output (JSON schema enforcement)** | Compliance verdicts, citations, eval results all need typed output. | **Low** | Eino's structured-output node. |

### 3. Knowledge / True-Source Layer

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Tiered knowledge (authoritative / historical / personal)** | Core denova concept; differentiates "real platform" from "RAG wrapper." | **High** | Requires per-tier storage, retrieval strategies, and provenance metadata. |
| **Per-Skill knowledge boundary (`reads` field)** | Without it, Skills leak across knowledge tiers → unsafe. | **Medium** | Enforcement at retrieval layer (reject queries against tiers not in `reads`). |
| **Citation to specific passage** (not just doc) | Compliance demo requires clause-level citation. Vague "source X" is not credible. | **Medium** | Chunks need stable IDs; UI needs deep-link to source. |
| **Source credibility annotation** | Authoritative vs historical tier differ in trustworthiness; user must see this. | **Low** | Display tier badge next to citation. |

### 4. Agentic RAG

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Autonomous decision: retrieve vs ask-clarifying vs refuse** | Adaptive RAG / Self-RAG patterns are 2025-2026 SOTA. Without it, system is a dumb pipeline. | **High** | LLM-as-router prompt; needs eval to tune threshold. |
| **Self-verification (check answer against retrieved context)** | Without it, hallucination rate is unmeasurable and high. | **Medium** | NLI-style or LLM-judge verification before returning. |
| **Refusal with reason** ("I cannot answer because X is not in the knowledge base") | The #1 credibility feature for compliance domain. Dumb hallucination kills trust. | **Low** | Branch in agent graph on low confidence. |

### 5. Guardrails & Eval

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Refusal accuracy metric** | Must quantify "how often does the system refuse when it should." Without a number, guardrails are vibes. | **Medium** | Eval dataset with known-unanswerable queries. |
| **Citation faithfulness metric** (RAGAS `faithfulness`) | Industry-standard; without it, eval claims are unsubstantiated. | **Medium** | RAGAS or equivalent LLM-judge; offline batch. |
| **Hallucination rate** | The metric users care about most. | **Medium** | Same as faithfulness, different name. |
| **Eval results view (basic)** | Numbers in README are nice; a live dashboard is credible. | **Medium** | Could use Langfuse self-hosted, or simple JSON + table view. |

### 6. Frontend

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Chat UI with message streaming** | Bare minimum for "Agent platform." | **Low** | Vercel AI SDK `useChat` + shadcn `<Message>`. |
| **Citation display (clickable to source)** | Without clickable citations, "grounded answers" is a lie. | **Medium** | Side-panel with highlighted passage. |
| **Skill switcher** | User must be able to choose / change the active Skill. | **Low** | Dropdown in header. |
| **Reasoning chain display** | "Why did the Agent do X?" — without this, debugging and trust both fail. | **Medium** | Collapsible panel per assistant turn. |

### 7. Open-Source Distribution

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **README with quickstart** | First impression for GitHub visitors. Missing = ignored. | **Low** | Hero shot, 5-min quickstart, badge row. |
| **Docker one-click run** | "Works on my machine" is the #1 reason OSS projects fail to spread. | **Low** | `docker compose up` + env file with API key. |
| **Apache-2.0 LICENSE** | Enterprise-compatible; matches denova lineage. | **Trivial** | Copy file. |
| **CONTRIBUTING.md** | Without it, contributions look like burden, not invitation. | **Low** | Section: how to add a Skill, how to file issues. |
| **Demo GIF in README** | 10× README conversion rate vs static screenshots. | **Low** | Record 30s of `skills-creator` flow. |

### 8. Demo Skill: Compliance Review (合规/制度审查)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Structured verdict (合规/不合规/需补充)** | Compliance demands auditable categorical judgments, not prose. | **Low** | JSON schema; LLM constrained to output one of three labels. |
| **Clause-level citation** | "Section 4.2 says X" is the entire value prop of compliance AI. | **Medium** | Required for credibility; shared with knowledge layer feature above. |
| **Refusal when question is out of knowledge scope** | Compliance cannot guess — guessing creates legal liability. | **Low** | Strict guardrail in Skill's `steps` field. |
| **Audit log entry per query** | Compliance officers must show "what did the system answer and why." | **Medium** | Persist query, retrieved chunks, verdict, citations, timestamp. |

---

## Differentiators (Competitive Edge)

These are the features that distinguish SkillForge from "yet another RAG chatbot" or "yet another agent framework." Most open-source projects skip these — that's the gap.

### 1. Skill System Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **6-field structured SKILL.md** (`description / agent / reads / writes / steps / guardrails`) | Anthropic Skills (Oct 2025) is unstructured prose + 2 fields. SkillForge's explicit `reads`/`writes` make Skills **machine-parseable + safer**. This is the core IP. | **Medium** | Schema design is the work; runtime enforcement is follow-on. |
| **Skill authoring wizard (NL → SKILL.md)** | denova has `skills-creator` (referenced in PROJECT.md). SkillForge must match this. Reduces time-to-first-Skill from hours to minutes. | **High** | LLM generates draft; user reviews; commit to disk. |
| **Skill validation + dry-run before publish** | "Will this Skill leak across knowledge tiers?" answer before runtime → safer enterprise rollout. | **Medium** | Static analysis + sample-query dry-run. |
| **Skill version management** (Git-style) | "Which version is in prod?" answer. Enterprise needs this. | **Medium** | Could leverage Git tags, or roll a simple version field. |

### 2. Knowledge Layer Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Per-Skill knowledge boundary enforcement at retrieval time** | Most RAG systems let any query hit any document. SkillForge's `reads` field *physically* restricts the retrieval index per Skill. This is the denova "true-source separation" idea, but made explicit and enforceable. | **High** | Index-per-Skill or filter-on-retrieve; both have trade-offs. |
| **Authoritative tier = no-write, no-override** | Compliance knowledge must be immutable. Hard constraint, not convention. | **Low** | Tier-level write policy enforced in code. |
| **Historical tier = append-only, time-travel queries** | "What did policy say in March?" — this needs append-only + time-indexed retrieval. | **High** | Snapshot per change; vector index per snapshot. |
| **Personal tier = scoped to user/session** | "My notes" vs "Company policy" — different blast radius. | **Medium** | Auth context gates this tier. |

### 3. Agentic RAG Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Multi-hop retrieval** (decompose query → multiple retrievals → synthesize) | Single-shot RAG fails on "compare policy A vs policy B." Multi-hop is the answer. | **High** | Query decomposition + intermediate verification. |
| **Self-verification with explicit confidence score** | Most agents don't show "I'm 40% sure." SkillForge returns `confidence` field, downstream UI can warn the user. | **Medium** | LLM-judge or calibration from RAGAS. |
| **Retrieval eval set per Skill** | "Is Skill X's retrieval good?" answer is per-Skill, not system-wide. Most projects evaluate the whole pipeline. | **High** | Each Skill ships with `eval.jsonl` of (query, expected_citation). |

### 4. Guardrails Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Eval dashboard (per-Skill breakdown)** | Langfuse is great but generic. Per-Skill eval = "this Skill is degrading" alerts. | **High** | Could integrate Langfuse + custom aggregation; or roll a minimal dashboard. |
| **Live refusal-accuracy regression test in CI** | Eval datasets drift; CI catches "Skill update broke refusal." | **Medium** | GitHub Action running eval set, fail on threshold drop. |
| **Citation preview before send** | Show user "I will cite X, Y, Z" → user can override. Reduces blind-trust hallucination. | **Medium** | Mid-stream UI component. |

### 5. Frontend Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Agent state inspector** (show tool calls, intermediate retrieval, decision branches) | Most chat UIs show only final answer. State inspector = debugging + trust + "wow" demo moment. | **High** | Side-panel with timeline view of agent steps. |
| **Skill diff viewer** (compare two versions of a Skill side-by-side) | "What changed in policy review v2?" — useful for both authors and auditors. | **Medium** | Markdown diff; reuse react-diff-viewer. |
| **Eval results view with per-question drilldown** | Click a metric → see which queries failed. Debug loop closes. | **Medium** | Eval set + run history table. |

### 6. Open-Source Distribution Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Live demo site** | A deployed SkillForge on a public URL = passive marketing. Most agent projects skip this. | **Medium** | Vercel/Fly/Railway for frontend; one Node for backend. |
| **Eval score badges in README** | "Faithfulness: 0.92" badge = credibility signal. | **Low** | GitHub Action + shields.io. |
| **Showcase Skills gallery** (3-5 built-in Skills beyond compliance) | Demonstrates breadth; gives users a copy-paste starting point. | **Medium** | E.g., HR Q&A, Code Review, Meeting Notes — each simple but credible. |

### 7. Compliance Demo Skill Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Risk score per verdict** (low/medium/high) | Beyond just 合规/不合规, gives reviewer a hint of severity. | **Low** | Add to JSON schema; rule-based or LLM-judge. |
| **Counter-evidence highlighting** ("This clause is contradicted by section X") | Goes beyond citation — actively surfaces conflicts. | **High** | Cross-passage comparison; high value but complex. |
| **Redline suggestion** (propose amendment text when non-compliant) | "Here's how to fix it" = concrete actionability. | **High** | Generative, but grounded in same tier-1 knowledge. |
| **Policy version diff** ("This answer changes if you use 2024 policy vs 2025") | Enterprise compliance always has version drift. | **Medium** | Requires the historical tier feature above. |

---

## Anti-Features (Deliberately NOT Build)

These are features that look attractive but would dilute the platform, bloat scope, or contradict the design philosophy.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **LLM fine-tuning / training pipelines** | PROJECT.md explicit "Out of Scope." Also: model training is a different discipline; competing with Hugging Face is suicide. | Use API-based LLMs; provide config for any provider. |
| **Multi-tenant SaaS (billing, orgs, user management)** | PROJECT.md explicit. Also: distracts from agent-engineering story; Auth0/Clerk exist. | Defer to v2. Single-user / single-org mode is fine. |
| **Custom vector DB / embedding infra** | Reinventing pgvector + sentence-transformers. Massive engineering cost. | Use pgvector (Postgres) + a hosted embedding API. |
| **Native mobile app** | PROJECT.md explicit. Also: React Native / Swift would double frontend scope. | Responsive web + PWA. Mobile users get a good-enough experience. |
| **Real-time multi-user Skill co-editing** (Google Docs style) | PROJECT.md explicit. Also: CRDTs are a deep rabbit hole. | "Edit SKILL.md, push to Git, agent reloads" workflow. |
| **Production Kubernetes / Helm deployment** | PROJECT.md explicit. Also: most OSS users are solo devs or small teams — K8s is overkill. | Docker Compose for v1; K8s manifests can be community-contributed later. |
| **Full enterprise SSO (SAML, OIDC federation)** | PROJECT.md explicit. Also: SSO vendors (Okta/Auth0) already exist. | Local-mode auth (single user); OAuth optional, but SAML is overkill. |
| **Voice input/output** | Modal mismatch — Skills are text. Voice would require Whisper + TTS pipeline + VAD. | Defer to v2 if ever. Text-only keeps focus. |
| **Agent-to-Agent protocol (A2A / ANP)** | 2026 hype but immature; protocol wars will burn months. | REST/SSE between Skills; revisit A2A when stable. |
| **Custom chat UI library / design system from scratch** | shadcn exists; rolling your own is anti-pattern. | shadcn/ui + Tailwind. |
| **"Universal" multi-modal (image/video input)** | Compliance domain is text. Multi-modal doubles model complexity. | Defer; only add if a Skill explicitly requires it. |
| **Auto-Skill-discovery across the web** (crawl + ingest Skills from random URLs) | Security nightmare; supply chain attack vector. | Manual Skill install + signed provenance later. |
| **Free-form "agent" mode with no Skill** | Without Skills, SkillForge is just another agent framework. Always require a Skill to be active. | The Skill IS the value. No-Skill mode is a non-goal. |
| **Custom reasoning model (replace the LLM's CoT)** | Cannot out-engineer frontier labs on reasoning. Use what they ship. | Rely on model's native CoT; surface it in UI. |
| **Per-user cost tracking / token billing dashboards** | PROJECT.md says no SaaS. Also: Langfuse already does this. | If needed, integrate Langfuse self-hosted. |

---

## Feature Dependencies

```
SKILL.md schema (6-field)
  ├── Skill validation (depends on schema)
  ├── Skill authoring wizard (depends on schema)
  └── Per-Skill knowledge boundary (depends on schema's reads field)

Tiered knowledge layer
  ├── Per-Skill knowledge boundary enforcement (depends on tier model)
  ├── Historical tier (depends on tier model + version mgmt)
  └── Personal tier (depends on tier model + auth context)

Agentic RAG
  ├── Self-verification (depends on retrieval pipeline)
  ├── Multi-hop retrieval (depends on self-verification + query decomposition)
  ├── Retrieval eval per Skill (depends on retrieval pipeline)
  └── Eval dashboard (depends on retrieval eval)

Guardrails
  ├── Refusal accuracy metric (depends on refusal branch in agent graph)
  ├── CI regression test (depends on refusal metric + retrieval eval)
  └── Eval dashboard (depends on metrics)

Demo Skill (Compliance)
  ├── Structured verdict (depends on structured-output feature)
  ├── Clause-level citation (depends on knowledge layer citation feature)
  ├── Audit log (depends on short-term memory + persistence)
  ├── Risk score (depends on structured verdict)
  └── Policy version diff (depends on historical tier)

Frontend
  ├── Reasoning chain display (depends on agent state introspection)
  ├── Citation display (depends on clause-level citation)
  ├── Agent state inspector (depends on streaming events with tool-call annotations)
  └── Skill diff viewer (depends on version management)
```

**Critical-path observations:**

1. **SKILL.md 6-field schema** blocks Skill authoring wizard, validation, AND knowledge boundary. Get this right first.
2. **Tiered knowledge + retrieval pipeline** blocks the demo Skill's compliance value. Tier model + citation are inseparable.
3. **Streaming events with tool-call annotations** block reasoning-chain UI AND agent-state-inspector. Eino ADK must emit structured events.
4. **Eval dataset** is a prerequisite for *any* eval dashboard. Without curated (query, expected) pairs, you can't claim metrics.

---

## MVP Recommendation

**Phase 1 (Must ship before "MVP"):**
1. SKILL.md 6-field schema + parser
2. Skill load + intent routing
3. Two-tier knowledge (authoritative + historical); personal tier deferred
4. Agent core: streaming + tool calling + structured output
5. Agentic RAG: retrieve / refuse / ask-clarifying (single-hop only)
6. Citation to passage level
7. Demo Skill: Compliance Review end-to-end (verdict + citation + refusal + audit log)
8. Frontend: chat UI + reasoning chain + citation panel + skill switcher
9. Eval: faithfulness + refusal accuracy on a small (50-100 query) eval set
10. Docker Compose + README + LICENSE

**Phase 2 (Credibility boosters):**
11. Skill authoring wizard (NL → SKILL.md)
12. Self-verification + confidence score
13. Eval dashboard (basic, per-Skill)
14. Live demo site deployed
15. 2-3 additional showcase Skills
16. Eval badges in README

**Phase 3 (Differentiators, if time):**
17. Multi-hop retrieval
18. Agent state inspector UI
19. Skill diff viewer
20. Skill version management (Git tags or similar)
21. Counter-evidence highlighting in compliance Skill
22. CI regression test for eval metrics

**Defer indefinitely:**
- Multi-tenant SaaS
- Native mobile
- K8s / Helm
- Custom vector DB
- Multi-modal
- A2A protocol

---

## Confidence Assessment

| Area | Confidence | Reason |
|------|------------|--------|
| Skill system table stakes | **HIGH** | Anthropic Skills spec (Oct 2025) is well-documented; the 6-field extension is opinionated but grounded in denova. |
| Agent core table stakes | **HIGH** | SSE + tool calling + cancel are mature patterns; Eino ADK confirmed available (v0.5.0, June 2026). |
| Knowledge / true-source features | **MEDIUM** | Tiered model is denova-derived; per-Skill enforcement details have multiple implementation paths. |
| Agentic RAG features | **HIGH** | Self-RAG / CRAG / Adaptive RAG are well-published (2024-2026); refusal-with-reason is industry-standard. |
| Guardrails & eval | **HIGH** | RAGAS metrics are standardized; Langfuse is the obvious OSS dashboard. |
| Frontend features | **MEDIUM** | Vercel AI SDK + shadcn patterns confirmed; reasoning-chain UI is more convention than standard. |
| Open-source distribution | **HIGH** | README/Docker/License/CONTRIBUTING are universal OSS hygiene. |
| Compliance demo Skill | **MEDIUM** | Verdict structure + citation + refusal confirmed in industry; counter-evidence/redline are speculative value-adds. |

---

## Gaps & Open Questions

- **Skill validation depth**: Static analysis of `reads` field vs sample-query dry-run — needs phase-specific research. Trade-off: speed vs safety.
- **Historical tier implementation**: Snapshot-per-change vs event-sourcing — performance vs simplicity trade-off not yet decided.
- **Eval dataset curation**: 50 queries is small but shippable; how to grow without manual curation burden?
- **Multi-hop vs single-hop for compliance**: Does multi-hop actually improve compliance answer quality, or just latency? Needs empirical testing.
- **Agent state inspector UX**: Timeline view vs tree view vs DAG? User research needed in a later phase.

---

## Sources

### Primary (HIGH confidence)

- Anthropic Skills (Oct 2025 launch) — `SKILL.md` YAML frontmatter spec (`name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`)
  - https://blog.csdn.net/javastack/p/19531825
  - https://www.cnblogs.com/javastack/p/19531825
  - https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md (referenced)
- CloudWeGo Eino ADK v0.5.0 release (June 2026) — multi-agent orchestration
  - https://github.com/cloudwego/eino/releases/tag/v0.5.0
- RAGAS framework — faithfulness, answer relevancy, context precision/recall metrics
  - https://blog.csdn.net/qq_36722887/article/details/148519357
  - https://www.cnblogs.com/pass-ion/p/19113902

### Secondary (MEDIUM confidence)

- Self-RAG / Corrective RAG / Adaptive RAG patterns
  - https://blog.csdn.net/qq_46094651/article/details/143078093
  - https://www.cnblogs.com/deephub/p/archive/2026/02/24
- Langfuse OSS eval dashboard reference
  - https://github.com/langfuse/langfuse
- Agent memory tiers (short-term / working / long-term)
  - https://arxiv.org/abs/2507.21428 (MemTool paper)
  - https://blog.csdn.net/yangzhihua/article/details/160419045
- Vercel AI SDK + shadcn chat UI patterns
  - https://tool.lu/article/7x4/detail
  - https://blog.csdn.net/weixin_63764436/article/details/161512313
- Enterprise compliance AI tooling patterns
  - https://blog.csdn.net/zzb1580/article/details/158735390
  - https://www.cnblogs.com/flyingsir/p/19225297
- Open-source agent platform best practices
  - https://github.com/heilcheng/awesome-agent-skills

### Project Context

- `D:\code\ClaudeCode\ai-agent\.planning\PROJECT.md` — SkillForge requirements, decisions, scope.
- denova reference (alfredxw/denova) — Skills / true-source separation / Eino lineage.
  - https://github.com/alfredxw/denova (referenced; could not fetch directly)

---

*End of FEATURES.md*
