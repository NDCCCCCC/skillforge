# Requirements: SkillForge

**Defined:** 2026-08-10
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Skill System (SYSK)

- [ ] **SYSK-01**: User can create a Skill from a declarative `SKILL.md` file in a Skill directory
- [ ] **SYSK-02**: System parses and validates `SKILL.md` against the **7-field** schema (`description` / `agent` / `reads` / `writes` / `steps` / `guardrails` / **`temperature`**) — see `.planning/SKILL_SCHEMA.md`
- [ ] **SYSK-03**: User can browse and select installed Skills via the UI
- [x] **SYSK-04**: System hot-reloads Skills on file change (fsnotify)
- [x] **SYSK-05**: System routes user queries to the right Skill via intent matching on `description`
- [x] **SYSK-06**: User can describe a Skill need in natural language and the **Skill Wizard** auto-generates a compliant `SKILL.md`
- [ ] **SYSK-07**: System enforces `reads` field as a hard filter at retrieval time (retriever cannot query outside declared tiers)
- [ ] **SYSK-08**: System enforces `temperature` field as **required** (no default) and passes it to LLM calls via Eino `GenerateOption(temperature)` — supports per-Skill determinism level (compliance Skills use 0.0, creative Skills use 0.7+)

### Agent Core (AGNT)

- [ ] **AGNT-01**: System orchestrates multi-agent workflows via Eino ADK (`ChatModelAgent` / `SequentialAgent` / `ParallelAgent`)
- [ ] **AGNT-02**: Agent can invoke external tools (function calling)
- [ ] **AGNT-03**: System streams agent events (text / reasoning / tool-call / source) to client in real-time via Hertz SSE
- [ ] **AGNT-04**: User can cancel an in-flight agent run
- [x] **AGNT-05**: System maintains per-session conversation memory (with no cross-session bleed)
- [x] **AGNT-06**: User can inspect live agent state (turns / tools / retrievals) via the UI

### Knowledge Layer (KNOW)

- [x] **KNOW-01**: System supports 4-tier knowledge: 权威 / 历史 / 个人 / 草稿 (each chunk tagged with `tier`)
- [ ] **KNOW-02**: System indexes documents into Milvus with mandatory `tier` metadata (insert fails if tier missing)
- [ ] **KNOW-03**: Retrieval filters by `tier` BEFORE similarity search (per Skill's `reads` declaration)
- [ ] **KNOW-04**: System defaults to refusing when no tier-matched docs found (refusal-by-construction)
- [ ] **KNOW-05**: UI displays citation tier alongside each citation (`[权威]` vs `[历史]` etc.)
- [x] **KNOW-06**: User can browse the knowledge graph (tier + doc + chunk relationships) in the UI

### Agentic RAG (ARAG)

- [x] **ARAG-01**: Agent autonomously decides to retrieve, ask clarifying, refuse, or answer
- [ ] **ARAG-02**: System supports multi-hop retrieval (agent retrieves additional chunks based on partial answer)
- [ ] **ARAG-03**: System supports Self-RAG / CRAG self-verification — agent re-reads retrieved docs and verifies each claim before final answer
- [ ] **ARAG-04**: Every cited claim is a chunk ID + span (not free text); unverifiable claims are stripped or refused
- [ ] **ARAG-05**: Refusal includes structured reason `{reason, can_user_rephrase, suggested_alternative_query}`

### Guardrails & Eval (GEVL)

- [ ] **GEVL-01**: System refuses with structured reason when retrieval score < threshold
- [ ] **GEVL-02**: System refuses when no tier-matched docs exist (refusal-by-construction; no improvising)
- [x] **GEVL-03**: Every query logs audit record: `{ts, user, query, skill, chunk_ids[], model_ver, prompt_hash, answer, refusal_reason}`
- [x] **GEVL-04**: System integrates Ragas eval (faithfulness, context precision, answer relevance) as a sidecar Python service
- [x] **GEVL-05**: System maintains per-Skill eval dataset (≥50 cases) with adversarial / refusal-required / multi-hop queries
- [ ] **GEVL-06**: Eval dashboard surfaces per-Skill metrics (faithfulness / citation precision / refusal accuracy) with trend over time
- [ ] **GEVL-07**: CI gating on faithfulness < 0.9 fails Skill promotion

### Frontend (FEUI)

- [x] **FEUI-01**: User can chat with the agent and see streaming responses (React 19 + Vercel AI SDK v5)
- [x] **FEUI-02**: Reasoning chain is collapsible and visible by default (typed message parts: reasoning / tool-invocation / text / source)
- [ ] **FEUI-03**: Citations are clickable links opening side panel with retrieved chunk verbatim + tier + version + effective date
- [ ] **FEUI-04**: User can switch between installed Skills via a skill picker
- [ ] **FEUI-05**: Refusal UX distinguishes "out-of-scope" from "I don't know" with structured reason + suggested rephrasing
- [x] **FEUI-06**: Loading state shows sub-progress during tool calls ("Searching 权威 tier... Found 12 candidates... Re-ranking...")

### Open-Source Distribution (DIST)

- [ ] **DIST-01**: README with badges, value prop, demo GIF, quick start, architecture diagram, contributing, license
- [ ] **DIST-02**: Apache-2.0 license file present from day 1
- [ ] **DIST-03**: `.env.example` with every required variable
- [ ] **DIST-04**: `docker-compose up` brings up full stack (Go backend + Milvus + BGE-M3 + frontend)
- [ ] **DIST-05**: Live demo site deployed (HuggingFace Spaces / fly.io / free tier) with sample compliance docs
- [ ] **DIST-06**: Pre-bundled sample compliance docs (works without API key for retrieval)
- [ ] **DIST-07**: `CONTRIBUTING.md` with "how to write a Skill", "add a tool", "add a knowledge tier"
- [ ] **DIST-08**: `examples/` folder with 2-3 Skill templates (compliance, simple QA, calculator)
- [ ] **DIST-09**: Published eval results in README (faithfulness %, citation precision %, refusal accuracy %)
- [ ] **DIST-10**: Eval set sample included in repo

### Compliance Demo Skill (CMPL)

- [ ] **CMPL-01**: System includes a built-in `企业合规/制度审查` Skill as the showcase
- [ ] **CMPL-02**: Compliance Skill outputs structured verdict `{status: 合规 | 不合规 | 需补充, confidence, citations, review_required}`
- [ ] **CMPL-03**: Compliance Skill requires human review (`review_required: true`) when confidence < 0.9 OR claim is novel
- [ ] **CMPL-04**: Compliance Skill reads ONLY 权威 tier (hard filter via `reads`; no 其他 tier bleed)
- [ ] **CMPL-05**: UI displays compliance verdict with `needs human review` badge when triggered
- [x] **CMPL-06**: Counter-evidence highlighting — surface chunks that contradict the agent's verdict
- [x] **CMPL-07**: Redline suggestion — propose clause-level rewrites when policy gap detected
- [x] **CMPL-08**: Policy version tracking (chunk metadata `effective_from` / `effective_to`; superseded versions filtered out)

### Financial Data Layer (DATA)

> Added 2026-08-10 via /gsd:explore (finance compliance data architecture). Map to Phase 5+ (architecture integration TBD).

- [ ] **DATA-01**: System provides a data import pipeline for data personnel to ingest CSV / SQL dump / API data with tier + schema metadata
- [ ] **DATA-02**: System enforces mandatory desensitization at import time (L1 PII hashing + L2 bucketing) — raw L1/L2/L3 data does NOT persist in SkillForge for aggregation-tier Skills
- [ ] **DATA-03**: System exposes aggregation API (predefined KPI functions) — agent invokes KPI functions, never SELECTs raw data
- [ ] **DATA-04**: System supports external authorized channel for individual-identification Skills — raw data stays in user's enterprise data warehouse, agent calls via read-only DB user / proxy with TTL token
- [ ] **DATA-05**: External authorized channel enforces field-level second-layer desensitization (e.g., display only last-4 of account number) plus pre-authorization by data personnel
- [ ] **DATA-06**: System maintains dual audit log: internal (SkillForge query log) + external (data warehouse query log), reconcilable by query ID + timestamp
- [ ] **DATA-07**: Skill's `reads` field supports two new value patterns: `data:aggregated_only` and `data:external:authorized`
- [ ] **DATA-08**: System rejects any agent query that violates the Skill's data channel (e.g., aggregation skill trying to call external channel) — guardrail at platform layer
- [ ] **DATA-09**: Aggregation channel: agent can ONLY invoke predefined KPI functions / views — no SQL, no new KPI definition (NL2SQL safety layer 1)
- [ ] **DATA-10**: Individual channel: agent queries go through predefined views (not raw tables) — DB user restricted (read-only, limited schema, field-level view masking) (NL2SQL safety layer 2)
- [ ] **DATA-11**: Agent must generate SQL preview (NL2SQL output) BEFORE entering approval flow for individual-identification Skills (NL2SQL safety layer 3 — data officer audits the actual query intent)
- [ ] **DATA-12**: Rejection must include structured reason `{reason_code, human_explanation}` (compliance audit requirement)
- [ ] **DATA-13**: Approval timeout = 持续挂起 (no auto-reject, no auto-timeout) — each query must be explicitly approved or rejected by data officer
- [ ] **DATA-14**: Every approval creates audit row: `{query_id, user_id, business_reason, sql_preview, approve_or_reject_result, data_officer_id, ttl_token_id, timestamp, chunk_ids_returned}`

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Platform Capabilities

- **PLAT-01**: Skill marketplace with version pinning and dependency resolution
- **PLAT-02**: Multi-user real-time collaborative Skill editing
- **PLAT-03**: Cross-Skill chaining (compose Skill A → Skill B pipelines)

### Scale & Production

- **SCALE-01**: Multi-tenant deployment with per-tenant Skill libraries
- **SCALE-02**: K8s / Helm chart for enterprise SSO deployment
- **SCALE-03**: Cost / token dashboard with per-Skill usage analytics

### Domain Skills (sample, not exhaustive)

- **DOMAIN-01**: Codebase comprehension Skill (reads source tree + ADRs + commit history)
- **DOMAIN-02**: Decision-retrospective Skill (reads meeting notes + decision logs)
- **DOMAIN-03**: Industry compliance packs (financial / medical / manufacturing)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-tenant SaaS (billing / subscription / user mgmt) | v1 聚焦 agent 能力本身，不做商业化平台层 |
| Desktop-grade desktop app (denova-style binary) | v1 聚焦 Web 应用形态 |
| Full private-deployment ops (K8s / Helm / enterprise SSO) | v1 给本地启动 + Docker，运维套件后续 |
| Real-time multi-user Skill editing | v1 假设单人 / 小团队维护 Skill |
| Custom LLM training / fine-tuning | 只做 agent 编排与工程，不做模型训练 |
| Native mobile app | Web + PWA 足够，不做原生 |
| Self-hosted LLM training infrastructure | Out of core value; rely on OpenAI-compatible APIs |
| Height.app-style "AI wrapper" pattern | Closed 2025.9; prove proprietary value via Skill schema + eval |

## Traceability

Which phases cover which requirements.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SYSK-01 | Phase 2 | Pending |
| SYSK-02 | Phase 2 | Pending |
| SYSK-03 | Phase 4 | Pending |
| SYSK-04 | Phase 5 | Complete |
| SYSK-05 | Phase 5 | Complete |
| SYSK-06 | Phase 5 | Complete |
| SYSK-07 | Phase 2 | Pending |
| SYSK-08 | Phase 2 | Pending |
| AGNT-01 | Phase 3 | Pending |
| AGNT-02 | Phase 3 | Pending |
| AGNT-03 | Phase 2 | Pending |
| AGNT-04 | Phase 3 | Pending |
| AGNT-05 | Phase 3 | Complete |
| AGNT-06 | Phase 5 | Complete |
| KNOW-01 | Phase 3 | Complete |
| KNOW-02 | Phase 3 | Pending |
| KNOW-03 | Phase 3 | Pending |
| KNOW-04 | Phase 3 | Pending |
| KNOW-05 | Phase 4 | Pending |
| KNOW-06 | Phase 5 | Complete |
| ARAG-01 | Phase 3 | Complete |
| ARAG-02 | Phase 3 | Pending |
| ARAG-03 | Phase 3 | Pending |
| ARAG-04 | Phase 2 | Pending |
| ARAG-05 | Phase 2 | Pending |
| GEVL-01 | Phase 2 | Pending |
| GEVL-02 | Phase 2 | Pending |
| GEVL-03 | Phase 3 | Complete |
| GEVL-04 | Phase 3 | Complete |
| GEVL-05 | Phase 3 | Complete |
| GEVL-06 | Phase 6 | Pending |
| GEVL-07 | Phase 6 | Pending |
| FEUI-01 | Phase 4 | Complete |
| FEUI-02 | Phase 4 | Complete |
| FEUI-03 | Phase 4 | Pending |
| FEUI-04 | Phase 4 | Pending |
| FEUI-05 | Phase 4 | Pending |
| FEUI-06 | Phase 4 | Complete |
| DIST-01 | Phase 7 | Pending |
| DIST-02 | Phase 7 | Pending |
| DIST-03 | Phase 7 | Pending |
| DIST-04 | Phase 7 | Pending |
| DIST-05 | Phase 7 | Pending |
| DIST-06 | Phase 7 | Pending |
| DIST-07 | Phase 7 | Pending |
| DIST-08 | Phase 7 | Pending |
| DIST-09 | Phase 7 | Pending |
| DIST-10 | Phase 7 | Pending |
| CMPL-01 | Phase 2 | Pending |
| CMPL-02 | Phase 2 | Pending |
| CMPL-03 | Phase 2 | Pending |
| CMPL-04 | Phase 2 | Pending |
| CMPL-05 | Phase 2 | Pending |
| CMPL-06 | Phase 5 | Complete |
| CMPL-07 | Phase 5 | Complete |
| CMPL-08 | Phase 5 | Complete |
| DATA-01 | Phase 5 | Pending |
| DATA-02 | Phase 5 | Pending |
| DATA-03 | Phase 5 | Pending |
| DATA-04 | Phase 5 | Pending |
| DATA-05 | Phase 5 | Pending |
| DATA-06 | Phase 5 | Pending |
| DATA-07 | Phase 5 | Pending |
| DATA-08 | Phase 5 | Pending |
| DATA-09 | Phase 5 | Pending |
| DATA-10 | Phase 5 | Pending |
| DATA-11 | Phase 5 | Pending |
| DATA-12 | Phase 5 | Pending |
| DATA-13 | Phase 5 | Pending |
| DATA-14 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 70 total (was 55; +8 DATA +1 SYSK-08 +6 DATA-09..14 added via /gsd:explore 2026-08-10/11)
- Mapped to phases: 70
- Unmapped: 0 ✓

**Phase distribution:**

| Phase | Count | Requirements |
|-------|-------|--------------|
| Phase 1 (Spike) | 0 | (none — integration validation only) |
| Phase 2 (Compliance Vertical Slice) | 13 | SYSK-01, SYSK-02, SYSK-07, AGNT-03, CMPL-01..05, ARAG-04, ARAG-05, GEVL-01, GEVL-02 |
| Phase 3 (Agentic Retrieval & Trust Evaluation) | 14 | AGNT-01, AGNT-02, AGNT-04, AGNT-05, ARAG-01..03, KNOW-01..04, GEVL-03, GEVL-04, GEVL-05 |
| Phase 4 (Streaming Trust UI) | 8 | SYSK-03, KNOW-05, FEUI-01..06 |
| Phase 5 (Skill Platform & Full Knowledge Boundaries) | 8 | SYSK-04, SYSK-05, SYSK-06, AGNT-06, KNOW-06, CMPL-06, CMPL-07, CMPL-08 |
| Phase 6 (Eval Dashboard & Production Hardening) | 2 | GEVL-06, GEVL-07 |
| Phase 7 (Open-Source Release) | 10 | DIST-01..10 |
| **Total** | **55** | |

---
*Requirements defined: 2026-08-10*
*Last updated: 2026-08-10 after roadmap creation (7 phases, mvp mode)*
