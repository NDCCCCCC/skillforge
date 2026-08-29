# Feature Research — v1.0 Data-as-a-Knowledge-Dimension

**Domain:** Enterprise IT data-support platform — sensitive-data ingestion + KPI aggregation + chat-over-numbers + scheduled report publication, all Skill-driven.
**Researched:** 2026-08-21
**Confidence:** MEDIUM-HIGH (anchor = existing finance-data seed; cross-product baseline = Dify datasets, LangChain NL2SQL safety literature, Metabase / Preset / Cube.dev schema patterns, Claude Code scheduled-task patterns; MEDIUM on regulated-industry KPI platform specifics via secondary sources)
**Milestone:** SkillForge v1.0 — Business Data Support (succeeds v0.8 trusted-action harness)

> Existing capabilities referenced, NOT re-researched: Skill schema (6-field / 7-field w/ temperature) — SYSK; Eino + ADK agent core — AGNT; 4-tier knowledge + tier-filter retriever — KNOW; Agentic RAG with verbatim citation grounding (D2-01 moat) — ARAG; INSERT-only audit store + read-time redaction — GEVL; HITL write-tool approval + edit-before-approve — HITL; write_todos planning + plan confirmation gate — PLAN; one-level sub-agent fan-out with tier inheritance + per-sub-agent grounding — FAN; Go-native trust evaluator + frozen-set gate — GEVL-06/07.

## Feature Landscape

Cross-product baseline (for "chat over your own numbers" angle):

- **Dify datasets / knowledge base pipeline** — CSV/Excel upload with chunking + embedding; lacks first-class KPI function abstraction; HITL on writes is weak (gap, not bar)
- **LangChain NL2SQL safety literature** — three-layer defense (architectural / query preview / runtime token); SQL approval gates before execution
- **Metabase / Preset / Cube.dev** — predefined metric definitions (`metrics_view.yaml`-style) — *the* pattern for "agent cannot invent a metric"; semantic layer; version retention
- **Claude Code scheduled tasks / cron** — task list with persisted state; one-shot vs recurring
- **Anthropic / ChatGPT "Deep Research"** — multi-hop → static report publish (PDF/markdown artifact with provenance + KPI snapshot) — closest analog to Capability 4 + 5
- **Airbyte / Fivetran / dlt** — schema discovery + tier-aware ingestion; raw landing + curated mart pattern

SkillForge's distinctive claim is **the Skill manifest IS the safety contract**: `reads: ["data:aggregated_only"]` declares the channel at Skill authorship time, and the platform code enforces it — no separate "data policy" UI, no "tenant settings" tab.

---

## Capability 1: Data Import Pipeline + Import-Time Desensitization

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **Airbyte / Fivetran / dlt** | Connector-based ETL: schema discovery → raw landing → curated mart; L1/L2 masking as a destination-side transformer |
| **Dify Datasets** | CSV/Excel/PDF upload → chunking → embedding; **no first-class desensitization** (gap) |
| **Metabase / Preset semantic layer** | `metrics_view.yaml` defines *what* can be queried, not *how raw data is protected* |
| **Cube.dev** | Pre-aggregated rollup tables + JWT-scoped query API; raw never exposed to clients |
| **Snowflake / BigQuery column policies** | Tag-based masking policies (`PII_TAG` → `HASH()`, `L2_TAG` → `BUCKET(1000)`); policies evaluated at query time |
| **GDPR Art. 25 / 32 / PCI-DSS** | "Privacy by design", pseudonymization at the earliest point of processing, raw not retained unless lawful basis exists |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| CSV / Excel / SQL dump / API push ingestion channels | Basic data onboarding; finance users live in CSV/Excel dumps | MEDIUM | Local CLI + REST endpoint; mime validation |
| Schema discovery + tier-tag metadata on every row | Without metadata, tier-filter retriever (Phase 3) cannot apply | MEDIUM | `tier` field mandatory; schema validation at insert (mirrors Phase 2 Milvus `tier` mandatory pattern) |
| Idempotent re-import (hash-based dedup) | Finance refreshes weekly; safe re-runs are not optional | LOW | Content-hash + `(dataset, period)` dedup key |
| L1 hashing (PII: name / national ID / phone / account) | GDPR / Personal Information Protection Law / PCI-DSS baseline | MEDIUM | SHA-256 with deterministic salt per dataset; same input → same hash → joins still work |
| L2 bucketing (continuous sensitive fields: balance / amount / credit-score) | Internal access control + disclosure caps | MEDIUM | `bucket(1000)`, `bucket(0.05 quantile)`; same bucket = comparable but not identifiable |
| Raw files discarded after ingest | The moat — only desensitized data persists | LOW | S3-style staging dir; purged post-transform; audit row records the discard |
| Per-row provenance (source_file, source_row_hash, ingest_ts) | Auditable lineage for finance regulators | LOW | Mirrors `effective_from`/`effective_to` (Phase 5 CMPL-08) |
| Tier-routing decision at ingest (which Skill tier can see this data) | True-source separation extends to data dimension | LOW-MEDIUM | Mirrors Phase 3 `tier` mandatory metadata pattern |

### Differentiators (Competitive Advantage — Skill-driven + platform-tier moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Import-time desensitization is enforced by the platform, not the Skill** | The Skill says `reads: ["data:aggregated_only"]`; the platform rejects any Skill that tries to ingest raw rows. "Code-enforced, not prompt-enforced" — same trust posture as Phase 11 `writes:` enforcement | MEDIUM | New **platform-tier** role (distinct from Skill-authored); runs before Skill ever sees a row |
| **Channel guard at import time, not at query time** | Dify / Cube mask at query time; a clever query still leaks. SkillForge physically never ingests the raw row — desensitization is irreversible because raw was discarded | LOW-MEDIUM | Architectural moat; reduces attack surface vs masking policies |
| **Two-tier masking = composable policy** | L1 hash + L2 bucket are independent transforms; users can stack (`L1 + L2`) for max strict or pick one for lighter use cases | MEDIUM | Mirrors the L1/L2/L3 field-class taxonomy from `finance-compliance-data-architecture.md` |
| **Audit-trail that proves raw was discarded** | Each ingest batch produces an audit row `{batch_id, raw_size_bytes, desensitized_row_count, discard_ts, dataset}` — auditable proof, not a hopeful invariant | LOW | INSERT-only audit (T-3-AUDIT pattern); reuses existing redaction at read time |
| **CSV preview before commit** | Finance reviewer sees the first 100 rows + proposed desensitization + proposed tier before ingest commits | LOW-MEDIUM | Two-phase commit (`/import/preview` + `/import/commit`); reviewer can adjust bucket boundaries per field |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Drag-and-drop CSV ingestion in the admin UI with no preview | "Self-service" | Finance team accidentally ingests raw PII; even one slip = leak; "easy" hides irreversible harm | Two-phase preview + commit + reviewer approval |
| Re-identification / de-hash capability ("undo the L1 hash") | "We might need the raw later" | Defeats the entire moat; PCI-DSS / GDPR make this a hot-button audit failure | Re-import raw from original source; never store a re-identification key in SkillForge |
| LLM auto-classifies which columns are L1/L2/L3 | "Save the analyst's time" | Non-deterministic policy classification = audit nightmare; one misclassification leaks | Static schema declaration at ingest (data officer declares the field policy); LLM classifies only the *description* text, never the values |
| NL2SQL "explore the imported data" Skill | "Power users want ad-hoc analysis" | Direct path to bypass KPI aggregation guard; reintroduces DATA-09 risk | Defer to external authorized channel (v1.1) with full DATA-10..14 machinery |
| In-place masking on the source file (don't discard) | "Save space / audit completeness" | "Raw is still here, just masked" is not the same as "raw is gone"; auditors distrust this; de-anonymization research has shown even bucket outputs are re-identifiable with side info | Discard raw after transform; the moat is the absence, not the mask |
| Storing original column values encrypted at rest ("we can decrypt for legal holds") | Compliance use case | Same moat-breaking concern as re-hash; adds KMS dependency and key management scope creep | Legal holds operate on the desensitized dataset + immutable audit trail |

---

## Capability 2: KPI Aggregation Interface (Agent-Invokes-Only, Never-Writes-SQL)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **Cube.dev** | Pre-aggregated rollup tables + JWT-scoped query API; metrics defined in code (`cube.js`); clients query `/load?query={"measures":["Orders.count"]}` — *the* canonical "no raw SQL" pattern |
| **dbt + Semantic Layer** | `metricflow` defines metrics in YAML; query API returns canonical metric values; SQL is generated *inside* dbt |
| **Metabase / Preset** | Question library: saved queries are first-class objects; users pick from a list, not write SQL |
| **LangChain NL2SQL safety stack** | Three-layer defense: architectural (DB user scope) / query preview (SQL shown before execution) / runtime token (TTL); applied to *ad-hoc* NL2SQL |
| **Anthropic / OpenAI function-calling** | LLM selects a function from a registered set; function args are typed/constrained; no string-template SQL |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Predefined KPI functions (data officer defines; agent cannot define new ones) | "No NL2SQL" is the headline safety promise of Capability 2 | MEDIUM | KPI registry file (`kpi/*.yaml` or `internal/kpi/registry.go`); immutable at runtime |
| KPI function signatures are typed (`func_name(args) → result`) | LLM function-calling requires structured args | LOW | Mirrors Phase 11 HITL `name+args` pattern (structured, never raw string) |
| KPI functions are the **only** tool registered for `reads: ["data:aggregated_only"]` Skills | Channel guard at the tool-registry level | LOW-MEDIUM | Filters tool list per Skill's declared channel — same code-enforced posture as Phase 11 `writes:` enforcement |
| Time-window arguments (`period: 2026-Q2`, `cohort: 2026-03`) | KPI without time is meaningless | LOW | Typed args; LLM picks from `periods[]` known list |
| KPI call is logged with function name + args + result snapshot | "KPI引用 = 新引用类型" per PROJECT.md v1.0 decisions | LOW | Reuses T-3-AUDIT INSERT-only; new audit-row shape (extends existing) |
| Structured refusal when no KPI matches the query | "拒答 = 合规基线" | LOW-MEDIUM | Refusal card already shipped (Phase 4 FEUI-05) — new reason code: `no_matching_kpi` |

### Differentiators (Competitive Advantage — Skill-driven moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Channel guard via `reads: ["data:aggregated_only"]` is a Skill manifest extension, not a config UI** | The same Skill schema that gates `writes:` (Phase 11) now gates `reads:` for the data dimension. One declaration, one enforcement point, no parallel "data policy" UX to keep in sync | LOW-MEDIUM | DATA-07 already locks this schema extension; platform-layer enforcement mirrors Phase 11 `writes:` enforcement code path |
| **The platform rejects KPI definitions created by Skills at runtime** | Cube.dev / Metabase let users create ad-hoc queries via UI; SkillForge KPI registry is **only** mutable by the data officer role, never by an agent run. Deterministic invariant | LOW | New `PlatformTier` actor role in audit log; distinct from Skill-authored actions |
| **KPI snapshot frozen in audit row, not just result code** | Audit replay can reconstruct the exact KPI value at the exact moment it was returned to the LLM — even if upstream data is later purged. Ties to Capability 5 cron rerender's audit trail | MEDIUM | Mirrors Phase 6 `computePromptHash` provenance pattern; row stored as JSON blob, deterministic shape |
| **Same KPI, different Skills → comparable results** | All Skills that `read: ["data:aggregated_only"]` see the **same** KPI functions; Skill-specific overrides are impossible. Eliminates "this Skill says 4.2%, that one says 4.5%" trust failure | LOW | Singleton KPI registry; no per-Skill overrides |
| **Result-cache hit policy + freshness window** | Repeated identical KPI call within `cache_ttl` returns the frozen snapshot — deterministic, fast, auditable | MEDIUM | Optional per-KPI; default OFF (always live) for finance freshness |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| NL2SQL with auto-SQL-generation against an LLM guard model | "Power users want flexibility" | Even with guard model, one false negative leaks; **this is the exact pattern finance-data-architecture.md line 79 calls out as "防绕过脱敏"** | Agent cannot write SQL ever; KPI function only. NL2SQL reserved for v1.1 external channel with full DATA-10..14 stack |
| Drag-and-drop KPI builder UI for non-engineers | "Democratize analytics" | Each new KPI is a new attack surface; requires code review anyway; conflicts with "platform code owns the safety" posture | KPI functions live in `kpi/*.go` next to Skills; same review/eval discipline as Skill authorship |
| Real-time streaming aggregation ("live tail of trades") | "Live dashboards" | Adds CDC dependency + late-arrival correctness; conflicts with audit row + snapshot reproducibility | KPI functions compute on current snapshot; freshness is `as_of` parameter, not streaming |
| LLM auto-generates KPI definitions from natural-language prompts | "Let data officers be lazy" | LLM writing KPIs = LLM defining its own sandbox = the safety wall collapses | Data officer writes Go functions or versioned YAML; reviewed in PR |
| Per-tenant / per-Skill KPI overrides | "Customization" | Two Skills computing the same KPI = which one is right? Audit becomes unanswerable | Singleton registry; customization happens at the *aggregation* layer (Skill chooses which KPIs to call), not the *definition* layer |
| Auto-discovery of KPIs from data schema | "Just inspect the dataset" | Stats columns ≠ business KPIs; "average of column X" is not "Net Revenue Retention 2026-Q2" | Data officer hand-crafts KPI set with business semantics; UI just lists registered KPIs |

---

## Capability 3: KPI Dashboard Skill (In-Chat Q&A on Numbers)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **ChatGPT / Claude "data analyst" mode** | Upload CSV → chart in chat → textual answer; no persistent dashboards |
| **Metabase / Preset dashboards** | Static dashboards with filters; chat-style Q&A as a thin layer (Preset has this); Q&A is "ask about my dashboard" not "ask about my data" |
| **Cube.dev Playground** | Pre-aggregated rollup tables + chat UI; every answer is *a* query against the semantic layer |
| **Anthropic Deep Research / ChatGPT Deep Research** | Multi-hop → citation-anchored answer → static artifact (PDF); closest analog for "structured answer with mini-trend + reference" |
| **Notion / Linear Q&A (Lightdash-style)** | Embedded dashboard cards inside text answers |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| User asks in chat: "2026-Q2 逾期率是多少？" | The point of the milestone | LOW | LLM selects KPI function via existing function-calling (Phase 11 tooling) |
| Answer shows: number + definition + computation window + citation-back to KPI | "Numbers without provenance = noise" — finance-grade minimum | LOW-MEDIUM | KPI citation = `{func, args, as_of, snapshot}`; new CitationPart type reuses existing citation UI |
| Mini-trend chart inline in answer | Visualizes period-over-period without leaving chat | MEDIUM | Recharts (already in /eval) — small sparkline component |
| As-of timestamp on every number | "When was this computed?" — finance regulators care | LOW | Mandatory field in KPI snapshot |
| "Drill-down" affordance (click number → "show me monthly trend") | Power-user table stakes | LOW-MEDIUM | New follow-up question pattern; existing `useChat` handles it |
| Refusal with structured reason when no matching KPI | Already shipped (Phase 4 FEUI-05) — new reason code only | LOW | Reuses RefusalCard |

### Differentiators (Competitive Advantage — Skill-driven moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Skill `reads: ["data:aggregated_only"]` IS the dashboard config** | No separate dashboard config file; the Skill's description + reads gates *what numbers exist*. "Show me the dashboard for Skill X" = "let X's KPI subset answer" | LOW | One source of truth; impossible to have "dashboard says X, Skill refuses to discuss X" |
| **Every chart inherits the citation contract** | Sparkline tooltips show `{func_name, period, snapshot}` — same provenance as a chunk citation (MEM-03 invariant extended to data dimension) | MEDIUM | Reuses existing citation whitelist code path |
| **Trend line carries the same audit row as the single number** | A "mini-trend" is just N KPI snapshots linked together; each is independently auditable | LOW | N audit rows per trend call; existing audit UI shows them on click |
| **"Compare two periods" is one KPI call, not two** | Period argument is array-typed; one KPI snapshot returns `[periods, values]`; deterministic merge, no double-call audit confusion | LOW-MEDIUM | Typed args extend Capability 2 registry |
| **No-SQL constraint is visible in UI** | Hover on KPI citation → tooltip shows `通道: data:aggregated_only` (channel name), reinforcing the safety posture without lecture | LOW | New field on CitationPart; cheap UX signal |
| **Multiple KPIs in one answer compose via Skill steps** | "Show me Q2 收入 + 客户数 + 同比" — Skill orchestrates 3 KPI calls, all independently cited | LOW-MEDIUM | Reuses Phase 12 write_todos pattern for planning when ≥2 KPIs involved |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Custom dashboard layouts (drag widgets onto a grid) | "Tableau replacement" | A *Skill* answers questions; it doesn't render layouts. Conflicts with "Skill-driven, no UI config" thesis | Multi-KPI answers compose inline (text + sparklines); layouts come from Skill `steps`, not user drag-drop |
| Filter UI ("filter by region = 华南") | Power-user expectation | Filters become a config UI; the Skill's `reads` is already the filter; giving users filters = giving them a query tool | Filters are typed args on KPI calls (`{region: "华南"}`); LLM picks; user doesn't |
| Save & share dashboard view | "Bookmark my dashboard" | That's a separate product (Metabase); conflicts with "Skill is the customization" | Skills are bookmarkable; specific KPI-set Skills *are* saved views |
| Real-time data streaming into chat | "Live numbers" | Adds CDC infrastructure; late-arrival correctness; audit reproducibility cost | `as_of` parameter on KPI call; freshness is explicit, not magical |
| Forecast / projection ("predict next quarter") | "AI value-add" | Out of scope (PROJECT.md "Out of Scope: 自定义 LLM 训练 / 微调"); projection requires ML scope | Defer; can layer on later as a registered KPI function (a *predictive* KPI, registered by data officer) |
| "Show me the raw row" drill-down | "Trust but verify" | Defeats desensitization moat; raw is gone | Drill-down is into more KPI calls (more aggregations), never into raw rows |

---

## Capability 4: Business Monthly Report Skill (Chat-Triggered DeepAgent → Static-Page Publish)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **ChatGPT / Anthropic Deep Research** | Multi-step plan → multi-source retrieval → long-form answer → user clicks "Save as doc"; static artifact is generated post-hoc |
| **Google Workspace "帮我写" / Microsoft Copilot** | Templated report generation with embedded data; user picks template, AI fills |
| **Notion AI / Coda AI** | In-doc Q&A that populates the doc; the doc *is* the report |
| **Anthropic Claude "agentic reports" pattern** | Plan → retrieve data → format per template → publish artifact (the closest analog) |
| **LangChain Deep Agents** | Four-ingredient recipe (system prompt + write_todos + sub-agents + file system) — Capability 3 of v0.8 milestone |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| User triggers via chat: "出 2026-08 月报" | Headline interaction of Capability 4 | LOW | Skill description matches the trigger intent |
| DeepAgent flow visible to user (plan checklist before execution) | Phase 12 PLAN-02 ships exactly this; trust requirement | LOW-MEDIUM | Reuses existing write_todos + plan confirmation gate |
| Multi-KPI assembly (typically 5-15 KPIs per report) | Monthly reports are composite by definition | LOW | Reuses Capability 2 KPI registry |
| Static page output (Markdown + charts) | "可发布" per PROJECT.md v1.0 decisions | MEDIUM | Render to `reports/<period>/<skill>.md`; served via existing static route |
| Report is auditable: every number ties to a KPI citation | Finance regulator expectation | LOW | Reuses Capability 3 citation contract |
| User can request regeneration with edits ("重做，更突出 Q2 风险") | Conversational interaction baseline | LOW | New chat turn → DeepAgent reruns with refined instructions |

### Differentiators (Competitive Advantage — Skill-driven moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **The Skill manifest IS the report template** | Skill's `steps:` section declares the report structure (which KPIs, in what order, what narrative slots); no separate template DSL | MEDIUM | New `report_template` step type OR Skill step that takes a structured template; same `SKILL.md` parsing as Phase 2 |
| **Plan confirmation gate before DeepAgent execution (reuses Phase 12 PLAN-03)** | User sees "我将调用 5 个 KPI、写 3 段叙事、生成 1 个图表" before any KPI call. Same trust posture as Phase 11 HITL | LOW | Capability 4 = Capability 2 + Phase 12 planning + publish action |
| **One fan-out for parallel KPI retrieval (reuses Phase 13 FAN-01)** | 5-15 KPIs in one report → orchestrator fans out parallel KPI calls (all bounded to `data:aggregated_only` channel via tier inheritance) | MEDIUM | FAN-02 tier inheritance now extends to `data:aggregated_only` reads; per-sub-agent grounding (FAN-04) runs on each KPI snapshot |
| **Report publish is a write action that goes through HITL** | Publishing a report is *the* irreversible action of the milestone; reusing Phase 11 approval gate is the platform-pattern | MEDIUM | `report:publish` joins the registered write tools; user approves before static page becomes public |
| **The plan itself is a Skill artifact** | Plan template stored next to `SKILL.md`; updates to plan flow through the same eval gate as Skill changes | LOW | New file `skills/<skill>/report-plan.yaml` (sibling to `eval-gate.json` from Phase 6) |
| **Every KPI snapshot is frozen in audit; later data changes do not affect the report** | "Reproducibility" mirror of Phase 6 SC-6 frozen lock; same `computeKPIHash` discipline | MEDIUM | Reuses Phase 6 `reprohash` package |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| "Let user upload their own report template" | "Customization" | Conflicts with "Skill = customization"; same Skill-driven thesis violation as anti-features in Capability 2 | Template lives in the Skill; user picks which Skill's report, not which template |
| Free-form narrative generation ("write me a risk paragraph") | Creative use case | High hallucination risk; finance reports need deterministic structure | Narrative slots in template are typed (`{kpi_trend}`, `{exception_explanation}`); LLM fills slots, not freestyles |
| Auto-email the published report to a distribution list | "Push notifications" | Adds email infra, address book management; out of platform scope | Static page has URL; user bookmarks / shares URL externally |
| PDF generation | "Executive-ready format" | Adds headless browser / pandoc / typst dependency; layout drift; "executive-ready" is subjective | Markdown + charts is sufficient for v1.0; PDF deferred (can layer PDF-on-publish later) |
| Multi-section deep-research that pulls from web / external sources | "Comprehensive" | Conflicts with "true-source separation" (Cap 1 reads are data-only) | Defer to separate Deep Research Skill (a different `reads:` set) |
| Comparing reports side-by-side | "Trend across months" | Static-page storage + diff UI is a sub-product | Provide a list view (`/reports`) showing all published reports; user opens two tabs |
| LLM decides when to generate the report without explicit user request | "Proactive insights" | Surprise content is antithetical to trust positioning; finance regulators reject "AI chose to publish" | Trigger is always explicit user chat message; cron-driven is Capability 5 (with full audit) |

---

## Capability 5: Static Report Portal + Scheduled Update

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **Metabase / Preset subscriptions** | Cron-style scheduled dashboard refresh + email delivery; "send this dashboard every Monday" pattern |
| **Cube.dev + scheduled pre-aggregations** | Refresh rollup tables on cron; query API serves cached aggregates |
| **Claude Code / LangGraph scheduled tasks** | Cron-like triggers; persisted state with run history |
| **Anthropic / OpenAI scheduled tasks (Tasks API)** | One-shot + recurring; task result is a static artifact (sibling to Capability 5) |
| **GitHub Actions scheduled workflows** | Cron syntax + run history + artifact retention; the operational model |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Cron schedule per Skill (e.g., "每月 1 号 09:00 重渲染") | The headline of Capability 5 | LOW-MEDIUM | Cron expression stored in Skill manifest or sibling `report-schedule.yaml` |
| Each rerender is an audit-logged run | "审计基线" per PROJECT.md v1.0 cross-cutting | LOW | Reuses T-3-AUDIT INSERT-only + Phase 6 read-time redaction |
| Each rerender captures a KPI snapshot per KPI call | Ties Capability 4's snapshot discipline into the schedule | LOW-MEDIUM | New audit-row shape: `{run_id, skill, cron_expr, fired_at, kpi_snapshots[], output_url, status}` |
| Version retention — every rerender is kept, not overwritten | Finance regulator "可追溯" requirement | LOW-MEDIUM | Storage: `reports/<skill>/<period>/<run_id>/index.md` + `meta.json` |
| UI lists all reports, filterable by Skill / period / run | Discoverability baseline | LOW-MEDIUM | Reuses Phase 4 UI patterns; new `/reports` route (admin-mode under `/eval` is also fine) |
| Last-success timestamp visible per report | "When did this last refresh?" | LOW | New field in list view |
| Manual trigger from UI ("rerender now") | Operations baseline | LOW | New endpoint, reuses DeepAgent run path |
| Failed-run notification (UI / log) | Ops baseline | LOW | Audit row + status badge |

### Differentiators (Competitive Advantage — Skill-driven moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Cron expression is a Skill manifest field, not a UI config** | `report_schedule: "0 9 1 * *"` in `SKILL.md`; the Skill itself carries its refresh cadence. "Customize via Skill, no code change" applies to schedules too | LOW | New Skill field; validation against cron syntax at parse time |
| **Every rerender is independently reproable** | Run a Skill from any historical rerender's KPI snapshots and get the exact same report — deterministic re-execution proof | MEDIUM | Reuses Phase 6 SC-6 `compliance.lock.json` discipline; new `report.lock.json` per run |
| **Cross-cut scheduler shares data TTL cleanup** | Same scheduler that rerenders reports also expires stale desensitized data; one infrastructure, two use cases | MEDIUM | New shared scheduler package; cron + TTL cleanup both pluggable jobs |
| **Rerender respects current `eval-gate.json` and Skill version** | A Skill can be blocked from cron rerender if its eval metric regresses (e.g., faithfulness drops below 0.9 after a Skill update) | MEDIUM | Reuses Phase 6 SC-2 gate logic; cron job looks up gate + version before firing |
| **Drift detection: alert if this rerender's KPI values differ materially from last rerender's** | "Why did 逾期率 jump 2%?" auto-flag, surfaced in UI | MEDIUM | Per-KPI threshold (data officer configures); >threshold → drift badge on report |
| **Audit replay reconstructs the entire report from stored snapshots** | Even after data TTL purges raw rows, the report + its KPI snapshots + audit are immutable — "数据清了不影响回放" per PROJECT.md v1.0 cross-cutting | LOW-MEDIUM | Stored snapshots are the canonical record; desensitized rows are an optimization |
| **Multi-Skill composition for portfolio reports** | "Combine KPI dashboard + monthly report Skill outputs into one weekly executive summary" via Skill chaining (deferred to v2 per PLAT-03, but the static-portal allows rendering multi-Skill reports) | LOW (just rendering) | Defer composition to v2; static portal supports multi-source renders |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Cron UI ("drag to schedule") | "Self-service for data officers" | Schedule is a Skill-level property; if schedules were per-run-configurable they'd diverge from Skill code path | Schedule lives in Skill manifest; data officer updates Skill, eval gate approves |
| Real-time sub-second refresh | "Live dashboard" | Incompatible with audit + snapshot discipline; finance teams need deterministic `as_of`, not stream | Cron + manual rerender + `as_of` parameter on KPI call |
| Email/Slack/Teams push of the report | "Push notifications" | Out of scope (PROJECT.md v1.0 "Out of Scope: webhook / async delegation"); delivery infra is a separate product | Static portal URL; user bookmarks or shares externally |
| Per-user customized reports ("my version of the dashboard") | "Personalization" | Per-user config = config drift = "whose report is right?" | Skills are the customization; user picks which Skill |
| Auto-summarize the report into an email digest | "AI executive summary" | LLM summarizing LLM-generated reports = double-hallucination risk | User opens the report; reads the actual numbers + citations |
| Time-zone-aware cron per user | "Show 9am in *my* timezone" | Cron evaluates in UTC; timezone drift makes audit harder | UTC only; client renders in user's local TZ for display |
| Delete old runs ("retention policy on reports") | "Storage cost" | Auditors require the chain; finance regulators reject "we deleted last year's report" | Rerender produces new run, never overwrites; storage cost is the price of audit |
| LLM-generated "executive insights" appended to the report | "AI value-add" | Same hallucination risk as Capability 4 anti-feature | Defer; if added, treat as a typed narrative slot requiring explicit user opt-in |

---

## Capability 6: Admin Console (Imports + KPI Def + Skill Mgmt)

### How Production Products Do It

| Product | Pattern |
|---------|---------|
| **Dify** | Single admin console: datasets, tools, members, API keys, billing (billing is SaaS — gap, not bar) |
| **Cube.dev Cloud** | Deployment + schema + roles + scheduled refreshes in one admin UI |
| **Metabase / Preset admin** | Datasets, questions, dashboards, permissions, audit logs |
| **dbt Cloud** | Jobs, models, exposures, lineage — admin-as-code (`.yml`) + UI for visibility |
| **Existing `/eval` admin route** (Phase 6) | SkillForge precedent: admin = read-only eval dashboard + audit query + token-gated redaction override |

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Admin route (separate from user-facing chat) | Phase 6 `/eval` precedent; convention not to mix admin with chat | LOW | New `/admin` route (or extend `/eval`); token-gated (Phase 6 SC-5 `X-Admin-Token`) |
| List of ingest batches with status + preview row count + raw size discarded | Operational visibility into Capability 1 | LOW | Reuses audit table; new view |
| List of KPI definitions with their function signatures + last call timestamp | Operational visibility into Capability 2 | LOW | Reuses KPI registry |
| List of Skills installed with hot-reload status + last eval run | Operational visibility (extends existing skill management) | LOW | Skill registry already exists |
| List of scheduled jobs with last-run status + next-fire timestamp | Operational visibility into Capability 5 | LOW | Scheduler table |
| List of all published reports (cross-period / cross-Skill) | Discoverability for Capability 5 | LOW | Reuses static portal list view in admin context |
| Admin actions emit audit rows | Phase 6 SC-3 pattern: every admin query is auditable | LOW | New `actor_type: admin` in audit |

### Differentiators (Competitive Advantage — Skill-driven moat)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **KPI definitions are NOT editable in the admin UI** | Force the data-officer-as-developer workflow; KPI changes go through PR + eval gate, like Skill changes. UI shows them read-only. Same discipline as Phase 5 Skill authorship (no live edits) | LOW | Mirrors Phase 5 hot-reload: KPI definitions loaded from `kpi/*.go` files; UI is read-only |
| **Import wizard forces preview-before-commit, not direct upload** | Same safety posture as Capability 1; admin UI is a 2-step UX, not a one-click "ingest now" | LOW-MEDIUM | `/admin/imports/new` shows preview → confirm → ingest; preview is the API call from Capability 1 |
| **Scheduled-job management reuses Skill authoring flow** | To change a schedule, edit `SKILL.md`'s `report_schedule` field and let hot-reload pick it up. Admin UI shows *current effective* schedule but does not let you edit it inline | LOW | Skill-as-source-of-truth pattern; admin = visibility, Skills = truth |
| **Read-time redaction applies to admin views too** | Admin sees the same `[REDACTED (90d+)]` badges as chat; to see raw rows you set `SKILLFORGE_ADMIN_TOKEN` (Phase 6 SC-5) — and even then, raw *rows* never existed in the first place | LOW | Reuses Phase 6 `redact()` in `QueryAudit` and `c.JSON` path |
| **Admin shows audit row per Skill change** | When Skill `compliance/SKILL.md` is hot-reloaded (Phase 5), an audit row records `{prev_hash, new_hash, who_edited, diff_summary}` — admin UI surfaces this for compliance reviewers | MEDIUM | New audit event type; ties to Phase 5 SYSK-04 hot-reload |
| **Admin can trigger manual Skill eval from console** | One-click "re-run eval-gate" without going to CI; mirrors `cmd/eval --frozen` (Phase 6) but UI-wrapped | MEDIUM | Reuses existing `cmd/eval` binary |
| **Same `X-Admin-Token` gate as `/eval`** | No new auth model; admin token already proven in Phase 6 SC-5 | LOW | `crypto/subtle.ConstantTimeCompare`; ASVS V6.2.1 |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Drag-and-drop KPI builder | "Democratize" | Already called out as anti-feature for Capability 2; same reasoning | KPI definitions in code; reviewed in PR |
| In-place Skill editor in admin UI | "No-Code editing" | Same anti-feature; breaks the file-based, diff-able, eval-gated Skill discipline | Edit `SKILL.md` in repo; hot-reload picks it up; eval gate enforces quality |
| Per-user / per-team access control in admin UI | "RBAC" | PROJECT.md v1.0 "Out of Scope: 多租户 SaaS" — single admin token is the v1.0 posture | Single `SKILLFORGE_ADMIN_TOKEN`; defer RBAC to v2 (SCALE-01) |
| Billing / subscription management | "Commercial-ready" | Out of scope per PROJECT.md (multi-tenant SaaS explicitly out); personal-brand open-source doesn't bill | Defer; open-source status is the brand |
| User management ("add team member, set role") | "Team ops" | Out of scope (single-admin posture); admin token is single-actor | Defer to v2 |
| Live audit log stream (websocket) | "Real-time monitoring" | Audit is append-only and queried; real-time adds WebSocket dependency for marginal value | List view refreshes on action; manual refresh for now |
| "Reset Skill to default" button | Recovery | If a Skill is broken, deleting it is destructive; restore should be via Git (Skills are code) | Git checkout / revert; SkillForge doesn't own Skill lifecycle — Git does |

---

## Capability 7: Productionization (Fan-Out Classifier + D8-07 Eval Uplift + Platform-Tier Hidden System Prompt)

> Three cross-cutting concerns bundled in PROJECT.md v1.0 table as "生产化增强". They share a common shape: each takes an existing v0.8 mechanism and elevates it from "stub / placeholder" to "production-grade".

### 7A: Fan-Out LLM Classifier (Replace Deterministic Placeholder)

#### Current state

Phase 13 FAN-01 ships a deterministic classifier: `if len(query terms) >= 3 and len(retrieval candidates) >= 5: fan out; else: single hop`. This is brittle and known to over/under-fan-out.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| LLM-based classifier decides fan-out per query | LLM "understands" multi-hop vs single-hop better than heuristics | MEDIUM | New classifier agent; prompts evaluated against frozen set |
| Classifier output is structured: `{should_fan_out, num_subtasks, task_boundaries[]}` | Matches Eino's structured-output expectations | LOW | Typed JSON schema |
| Classifier decision is logged in audit (`why we did/didn't fan out`) | Reproducibility + eval gate | LOW | New audit-row shape |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Classifier uses Skill `description` + query embedding to decide** | Two Skills with same query may have different fan-out needs (compliance Skill: never fan out for compliance data; KPI Skill: always fan out for cross-period KPIs) | MEDIUM | Classifier becomes Skill-aware; per-Skill eval set captures the expected fan-out pattern |
| **Frozen set asserts fan-out rate (not just answer quality)** | Eval asserts "Skill X fanned out N times in eval set, target rate Y, delta ≤ Z%". Prevents classifier drift | MEDIUM | Extends Phase 6 frozen set with `expected_fan_out` per case |
| **Fan-out is mandatory (not optional) for KPI queries in v1.0** | Cross-period KPI queries *always* benefit from fan-out (each period is independent sub-task); deterministic but recorded as a Skill-level preference | LOW | New Skill field: `fan_out_policy: required_for_kpi_cross_period` |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| LLM classifier that recursively decides "should I spawn a classifier to decide whether to spawn more classifiers?" | Unbounded cost; same anti-pattern as v0.8 "Recursive spawning" | One-level classifier call per top-level run only |
| Classifier that returns free-text justification | Non-auditable | Structured `{should_fan_out, num_subtasks, task_boundaries[]}` only |
| Per-tenant custom classifier models | Out of scope (no multi-tenant) | Same classifier for all Skills; per-Skill eval captures differences |

### 7B: D8-07 Eval Uplift

> D8-07 is the trust-floor / faithfulness regression hardening requirement that spans v0.8 and v1.0. Existing faithfulness = 0.972 (Phase 6 frozen), refusal_accuracy = 95% (v0.8 baseline). The "uplift" is raising these numbers and adding new metrics for v1.0-specific risks.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Faithfulness regression gate stays at ≥0.9 (Phase 6 SC-2 baseline) | Promotion gate for any Skill update | LOW | Already shipped |
| Refusal accuracy stays at ≥0.95 | Compliance baseline | LOW | Already shipped |
| New metric: **KPI-call accuracy** (does the agent pick the *right* KPI for the query?) | KPI Skill correctness | MEDIUM | New judge prompt; golden set of 50 KPI-query → expected-KPI pairs |
| New metric: **citation provenance completeness** (every number has a `{func, args, as_of, snapshot}` citation) | New audit dimension | MEDIUM | Mirrors Phase 6 citation_precision but for data citations |
| New metric: **cross-period consistency** (same KPI computed at T and T+1 should be ≤X% drift unless data actually changed) | Catches silent computation bugs | MEDIUM | Replay-based; reuses Phase 6 `reprohash` |
| Frozen set extended to 50 + 50 (compliance + KPI) cases | Per-Skill gate | LOW-MEDIUM | `eval/frozen/kpi-dashboard.jsonl` (sibling to `compliance.jsonl`) |
| Eval gate blocks cron rerender (Capability 5) if Skill's KPI-accuracy drops below threshold | Tie eval into operational safety | LOW | Reuses Phase 6 gate logic + Capability 5 scheduler |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Faithfulness definition extended to data citations** | "The number I gave matches the KPI snapshot I cited" — currently faithfulness checks text ↔ chunk; v1.0 extends to text ↔ KPI snapshot | MEDIUM | New judge prompt; same GLM-as-judge pattern |
| **Eval dashboard surfaces new metrics on `/eval/kpi` tab** | Operators see per-Skill KPI accuracy + drift over time | LOW-MEDIUM | Recharts extension (Phase 6 TrendPanel precedent) |
| **Drift detection in Capability 5 ties into eval gate** | A cron rerender that produces drift > threshold auto-files an "eval anomaly" audit row; operators can re-run gate | LOW | Reuses Capability 5 drift-detection + Phase 6 eval gate |
| **Frozen set stored next to the Skill, not in central `eval/frozen/`** | Per-Skill eval versioning; matches Phase 6 `skills/<skill>/eval-gate.json` precedent | LOW | New convention: `skills/<skill>/frozen-eval.jsonl` |
| **Live-eval (build-tag `liveeval`) jitter tolerance carries over** | Phase 6 SC-6 already proved `1e-6` StubJudge vs `1e-3` live; v1.0 KPI metrics inherit | LOW | Reuses existing build tag pattern |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| LLM-as-judge for numerical equality (asks LLM "are these numbers equal?") | LLMs are bad at exact equality | Use programmatic equality (`==`, `±0.01` for floats); LLM-judge only on semantic dimensions |
| Per-tenant frozen sets | Out of scope | Single set per Skill |
| Optimizing eval set size upward ("1000 cases = better") | Cost without proportional benefit; Phase 6's 50 cases proved enough | 50 per Skill, well-curated adversarial cases |
| Auto-generated eval cases via LLM | Eval set becomes a circular function of the model being evaluated | Hand-curated by data officer |

### 7C: Platform-Tier Hidden System Prompt

#### Current state

Every Skill carries a `system_prompt` (or equivalent) that the Skill author controls. There is no platform-level "lower bound" — a malicious or buggy Skill could write a system prompt that says "ignore safety rules". This is the platform-tier trust floor problem.

#### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Platform-level system prompt injected **before** Skill prompt, **not visible** to Skill author | Code-enforced trust floor — Skill cannot disable | LOW-MEDIUM | New layer in Eino chat template; mirror existing pre-prompt slot |
| Platform prompt states non-negotiable rules (refuse when no evidence, never write raw SQL, etc.) | The D2-01 + DATA-09 + ARAG-04 invariants made durable | LOW | Static string in `internal/platform/system_prompt.go` |
| Platform prompt survives Skill hot-reload | Trust floor never lapses during reload | LOW | Eino template recomposes on Skill change |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Platform prompt is loaded from a separate file (`PLATFORM_PROMPT.md`), reviewable in PR but NOT editable by Skill authors** | Trust floor has its own code-review path; Skill authors can't accidentally weaken it; admins can update it via code review | LOW-MEDIUM | Sibling to `SKILL.md`; git-tracked; eval set asserts floor remains intact |
| **Data-dimension rules explicitly added: "never SELECT raw rows; only call registered KPI functions; treat any attempt at LLM-generated SQL as a refusal"** | DATA-09 made mechanically durable | LOW | Static text in platform prompt |
| **Eval set asserts platform-prompt invariants** | "Refuse if asked to write raw SQL" should have ≥99% accuracy in frozen set | MEDIUM | New category in `eval/frozen/platform-floor.jsonl` |
| **Platform prompt is the *only* place "never" rules live** | Single source for safety lower bound; Skill prompts focus on domain behavior, not safety | LOW | Convention: Skill prompts never duplicate "never" rules |
| **Platform prompt logs version on every audit row** | Audit replay can show "this answer was generated with platform_prompt v1.4" | LOW | Reuses `reprohash` pattern |
| **Operators can preview the effective combined prompt** | Admin UI: "Skill X + Platform v1.4 = effective prompt: <rendered>" — trust via visibility | LOW | New admin endpoint |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| "Override platform prompt" toggle for trusted Skills | Trust floor with bypass = no trust floor | Never bypassable |
| Per-tenant platform prompt | Out of scope (no multi-tenant) | Single platform prompt version |
| Skill author can append to platform prompt | Same as bypass | Platform prompt is appended-to, never edited-from |
| Make platform prompt user-visible in chat ("here's what the system says") | Privacy of safety prompt (some prompts are themselves a moat) | Hidden; visible only via admin preview endpoint |
| "Disable safety floor in dev mode" | Dev convenience bleeds into prod | Separate `SKILLFORGE_PLATFORM_PROMPT_OVERRIDE` env var, gated to non-production builds |

---

## Cross-Cutting Concerns (Spanning All 7 Capabilities)

### Scheduler (Shared by Capability 5 Rerender + Data TTL Cleanup)

| Concern | Resolution | Complexity | Notes |
|---------|------------|------------|-------|
| Cron job runner | New `internal/scheduler` package; cron syntax parsed at Skill load; jobs persisted in audit | MEDIUM | Reuse existing cron library (e.g., `robfig/cron`) — already in Phase 5 Skill reload ecosystem |
| TTL cleanup of stale desensitized data | Same scheduler runs `cleanup_old_datasets(cutoff)` periodically | LOW-MEDIUM | Job list in config; cleanup is idempotent |
| Concurrent-run guard (two rerenders of same Skill overlapping) | Lock per `(skill, period)`; second run skips with audit row | LOW | Single-instance scheduler; K8s CronJob semantics deferred |
| Failed-job retry policy | 3 retries with exponential backoff; final failure emits `run_failed` audit row | LOW | Standard pattern |

### Audit Row INSERT-Only (Compliance Baseline)

| Concern | Resolution | Complexity | Notes |
|---------|------------|------------|-------|
| New audit-row shapes for v1.0 | `ingest_batch`, `kpi_call`, `report_published`, `report_rerender_fired`, `platform_prompt_version` | LOW | Extend existing `audit.go`; no breaking changes |
| Read-time redaction extended to new row shapes | Reuse `redact()` pattern from Phase 6 | LOW | Single function; new field names registered |
| Audit row includes the platform-prompt version (Capability 7C) | Every audit row is fully reproducible | LOW | New column on `audit` table |
| Dual audit (internal + external warehouse) — DATA-06 | Defer to v1.1 (external authorized channel not in v1.0) | — | Out of v1.0 scope per PROJECT.md v1.0 decisions |

### Channel Guard (`reads: ["data:aggregated_only"]`)

| Concern | Resolution | Complexity | Notes |
|---------|------------|------------|-------|
| Manifest schema extends existing `reads` field | Two new value patterns per DATA-07 | LOW | `SKILL_SCHEMA.md` update |
| Platform enforcement rejects Skills whose tool registry includes both data + non-data tools when channel is set | Code-enforced, mirrors Phase 11 `writes:` enforcement | LOW-MEDIUM | Single check at Skill load |
| Channel guard test in frozen eval | "Attempt to write raw SQL via aggregation Skill → refusal" | LOW | New golden case in `platform-floor.jsonl` |

### Existing Capabilities Leveraged (Don't Re-Research)

| v0.8 Capability | Used By | Reuse Mechanism |
|------------------|---------|-----------------|
| Skill schema (SYSK-01..08) | All 7 | SKILL.md is the customization primitive |
| Hot-reload (SYSK-04) | Cap 5, 6 | Cron + KPI def updates reload without restart |
| Tier-filter retriever (KNOW-03) | Cap 1, 2 | `tier` mandatory metadata extends to data |
| Verdict + refusal pipeline (ARAG-04/05) | Cap 3, 4 | "No matching KPI" refusal |
| D2-01 grounding (chunk verbatim check) | Cap 3, 4 | KPI snapshot grounding (new check, same pattern) |
| HITL approval gate (HITL-01..08) | Cap 4 | `report:publish` is a write tool |
| Write_todos + plan confirmation (PLAN-01..05) | Cap 4 | Multi-KPI monthly report = DeepAgent flow |
| Sub-agent fan-out + tier inheritance (FAN-01..04) | Cap 4 | Parallel KPI retrieval; channel inheritance |
| `/eval` admin route + token gate | Cap 6 | Same `X-Admin-Token`; admin = visibility only |
| Read-time redaction (Phase 6 SC-5) | All 7 | `redact()` in every audit query path |
| Frozen eval set + gate (Phase 6 SC-2/6) | Cap 7B | Same gate; new metrics |

## Feature Dependencies

```
[Cap 1 Data Import]
    └──enables──────> [Cap 2 KPI Aggregation]      (data must exist for KPIs to compute on)
    └──enables──────> [Cap 5 Rerender]             (scheduler reads from imported data)
    └──requires────> [T-3-AUDIT INSERT-only]       (EXISTS: Phase 6 SC-3)
    └──requires────> [Skill manifest tier field]   (EXISTS: Phase 3 KNOW-02)

[Cap 2 KPI Aggregation]
    └──enables──────> [Cap 3 KPI Dashboard Skill]
    └──enables──────> [Cap 4 Monthly Report Skill]
    └──requires────> [data:aggregated_only channel guard]   (NEW: extends SYSK-07)
    └──requires────> [Phase 11 writes: enforcement code path] (EXISTS: same enforcement shape)

[Cap 3 KPI Dashboard Skill]
    └──enhances────> [Cap 4 Monthly Report Skill]  (KPIs compose into reports)
    └──requires────> [Cap 2 KPI registry]
    └──requires────> [Phase 4 chat UI patterns]    (EXISTS: citation UI)
    └──requires────> [MEM-03 citation whitelist]  (EXISTS: extended to data citations)

[Cap 4 Monthly Report Skill]
    └──publishes───> [Cap 5 Static Report Portal]
    └──requires────> [Cap 2 KPI registry]
    └──requires────> [Phase 12 PLAN-01..05]         (EXISTS: DeepAgent + write_todos)
    └──requires────> [Phase 11 HITL gate]           (EXISTS: report:publish is a write tool)
    └──requires────> [Phase 13 FAN-01..04]          (EXISTS: parallel KPI retrieval)

[Cap 5 Static Report Portal + Cron]
    └──requires────> [Cap 4 publish action]
    └──requires────> [scheduler package]            (NEW: shared with TTL cleanup)
    └──requires────> [T-3-AUDIT INSERT-only]       (EXISTS: per-run audit)
    └──requires────> [reprohash]                    (EXISTS: Phase 6 SC-6 provenance)
    └──requires────> [eval-gate]                    (EXISTS: blocks Skill from rerender if gate fails)

[Cap 6 Admin Console]
    └──requires────> [Cap 1 ingest UI]
    └──requires────> [Cap 2 KPI registry read-only]
    └──requires────> [Phase 6 /eval + X-Admin-Token] (EXISTS: same auth model)
    └──requires────> [Phase 5 hot-reload]           (EXISTS: shows reload events)

[Cap 7A Fan-out Classifier]
    └──enhances────> [Cap 4 Monthly Report Skill]
    └──requires────> [Phase 13 FAN-01..04]          (EXISTS: classifier replaces deterministic stub)
    └──requires────> [frozen eval set extended]    (EXISTS: Phase 6)

[Cap 7B D8-07 Eval Uplift]
    └──requires────> [frozen eval set + gate]      (EXISTS: Phase 6 SC-2/6)
    └──requires────> [Phase 6 redaction]            (EXISTS: eval rows audited + redacted)
    └──requires────> [scheduler]                    (NEW: ties into cron rerender gate)

[Cap 7C Platform-Tier Hidden System Prompt]
    └──enforces────> [ALL 7 capabilities]          (DATA-09, ARAG-04, refusal floors)
    └──requires────> [Eino chat template]          (EXISTS: pre-prompt slot)
    └──requires────> [PLATFORM_PROMPT.md file]      (NEW: sibling to SKILL.md)
    └──requires────> [eval set asserts invariants] (EXTENDS: platform-floor.jsonl)

[CROSS-CUTTING: Scheduler]
    └──shared by───> [Cap 5 cron rerender + TTL cleanup]

[CROSS-CUTTING: Audit row new shapes]
    └──shared by───> [Cap 1/2/4/5/7C]

[ALL 7] ──gated by──> [frozen eval + D8-07 uplift]   (Cap 7B)
[ALL 7] ──gated by──> [platform-tier trust floor]     (Cap 7C)
```

### Dependency Notes

- **Cap 1 must come first in implementation order:** Cap 2 cannot exist without data to compute on; Cap 3/4 cascade from Cap 2; Cap 5 cascades from Cap 4. The dependency chain is strictly linear: 1 → 2 → 3 → 4 → 5. Cap 6 (admin) and Cap 7 (productionization) are parallel tracks that touch all five.
- **Cap 2's channel guard reuses Phase 11 `writes:` enforcement code path:** Same trust posture (Skill manifest declares → platform enforces → frozen set asserts). Implementation is a copy of `writes:` enforcement with a different field.
- **Cap 4 + 5 are the "wow" capabilities** but they cannot ship without Cap 1 + 2 + 3. The milestone order is: 1 → 2 → 3 → 4 → 5 → 6 → 7 (parallel).
- **Cap 6 admin shares auth model with Phase 6 `/eval`:** No new `X-Admin-Token`; reuses `crypto/subtle.ConstantTimeCompare` + ASVS V6.2.1. Reuse reduces security surface area.
- **Cap 7C is the lowest-cost, highest-leverage capability:** A static text file + pre-prompt slot + frozen set additions. Should ship as part of Cap 1's first plan (trust floor from day 1 of data dimension).
- **Cap 7A classifier is medium-risk:** Replacing a deterministic stub with an LLM call adds non-determinism. Must be tied to frozen-set extension that asserts fan-out rate stability (else the eval gate itself becomes unstable).
- **All 7 gated by Cap 7B eval uplift:** No capability ships without a frozen-set regression showing its new metric (KPI accuracy / drift / platform-prompt invariance) stays green.

## MVP Definition (milestone scope)

### Launch With (v1.0)

- [ ] **Cap 1:** CSV + SQL dump ingestion; L1 hash + L2 bucket; two-phase preview/commit; audit row per ingest batch; raw discarded
- [ ] **Cap 2:** KPI registry (`internal/kpi/registry.go`); `data:aggregated_only` channel guard; KPI citation in audit; refusal when no match
- [ ] **Cap 3:** KPI Dashboard Skill (in-chat Q&A on numbers + mini-trend + data citation); `reads: ["data:aggregated_only"]`
- [ ] **Cap 4:** Monthly Report Skill (chat-triggered DeepAgent + multi-KPI + static page publish via HITL-gated `report:publish` write tool)
- [ ] **Cap 5:** Static report portal at `/reports`; cron rerender; per-run audit + version retention + drift detection; shared scheduler with TTL cleanup
- [ ] **Cap 6:** Admin console at `/admin` (extends `/eval` route pattern); ingest preview + KPI registry view + Skill mgmt + scheduled jobs; same `X-Admin-Token`
- [ ] **Cap 7A:** LLM fan-out classifier (replaces Phase 13 deterministic stub); per-Skill expected-fan-out in frozen set
- [ ] **Cap 7B:** KPI-accuracy metric + citation-provenance metric + cross-period consistency metric; extended frozen set
- [ ] **Cap 7C:** Platform-tier hidden system prompt (`PLATFORM_PROMPT.md`); data-dimension rules baked in; platform-floor.jsonl frozen cases

### Add After Validation (v1.0.x)

- [ ] KPI result-cache hit policy (if data officer wants it for performance)
- [ ] Drill-down chart types beyond sparkline
- [ ] Drift auto-notification (webhook) — depends on async infra (v2 PLAT-07)
- [ ] Multi-period KPI in single call (Capability 3 differentiator; can be retrofitted)

### Future Consideration (v2+)

- [ ] External authorized channel (DATA-04..14, v1.1) — needs identity + delivery + TTL token infra
- [ ] Cross-Skill Skill chaining (PLAT-03) — "executive summary Skill combining KPI dashboard + monthly report outputs"
- [ ] Predictive KPI (forecasting) — defers until ML scope is decided
- [ ] Email/Slack delivery of rerendered reports — defer until delivery infra is in scope
- [ ] Per-tenant platform prompt / multi-tenant admin RBAC — out of scope (SCALE-01)

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|---------|
| Cap 1 data import + desensitization | HIGH (foundation; everything else depends) | MEDIUM | P1 |
| Cap 1 two-phase preview/commit | HIGH (safety) | LOW-MEDIUM | P1 |
| Cap 2 KPI registry + channel guard | HIGH (foundation) | MEDIUM | P1 |
| Cap 2 KPI citation in audit | HIGH (audit baseline) | LOW | P1 |
| Cap 3 KPI Dashboard Skill | HIGH (headline demo) | LOW-MEDIUM | P1 |
| Cap 3 mini-trend chart | MEDIUM | MEDIUM | P2 |
| Cap 4 Monthly Report Skill | HIGH (showcase) | MEDIUM-HIGH | P2 |
| Cap 4 HITL-gated publish | HIGH (trust) | LOW | P2 |
| Cap 5 static portal + cron | MEDIUM-HIGH | MEDIUM | P2 |
| Cap 5 drift detection | MEDIUM | MEDIUM | P3 |
| Cap 6 admin console | MEDIUM | MEDIUM | P2 |
| Cap 6 KPI def read-only enforcement | HIGH (discipline) | LOW | P1 |
| Cap 7A LLM fan-out classifier | MEDIUM-HIGH | MEDIUM | P2 |
| Cap 7B D8-07 eval uplift | HIGH (gate) | MEDIUM-HIGH | P1 |
| Cap 7C platform-tier prompt | HIGH (moat) | LOW-MEDIUM | P1 |
| Cap 7C frozen set platform-floor cases | MEDIUM | LOW | P2 |

## Competitor Feature Analysis

| Capability | Dify | LangChain + Cube.dev | Metabase / Preset | ChatGPT Deep Research | SkillForge v1.0 Approach |
|------------|------|----------------------|-------------------|----------------------|--------------------------|
| Data import | CSV/PDF upload + embedding | ETL pipelines (dlt/Airbyte) + Cube rollups | Database connection | CSV upload only | **CSV/SQL/API + tier metadata + L1 hash + L2 bucket; raw discarded at import time** |
| Channel guard | None (single namespace) | Cube JWT scopes; not Skill-declared | Database roles | None | **`reads: ["data:aggregated_only"]` in Skill manifest; platform rejects violations** |
| KPI / metric def | None as first-class | Semantic layer in code (`cube.js` / `metricflow`) | Question library | Implicit | **KPI registry in Go; agent cannot define KPIs; only registered functions are callable** |
| Agent over numbers | Weak (RAG over docs) | Cube + LangChain ReAct NL2SQL (risky) | Metabot Q&A on saved questions | Deep Research; can write SQL | **KPI function calling only; no NL2SQL; refusal if no matching KPI; mini-trend + citation per number** |
| Reports | Workflows (design-time) | Multi-step agents; static output DIY | Subscribed dashboards (cron + email) | "Save as doc" post-hoc | **DeepAgent + write_todos + plan confirmation + HITL-gated publish + cron rerender + version retention** |
| Scheduled updates | Workflow cron | External scheduler | Email subscriptions | Tasks API | **Cron in Skill manifest; shared scheduler with TTL cleanup; per-run audit + drift detection** |
| Eval gate | None | LangSmith (vendor) | None | None | **Frozen-set + 4 metrics (faithfulness, KPI-accuracy, citation-provenance, cross-period consistency); blocks Skill promotion + cron rerender** |
| Trust floor | None | None (your job) | None | Constitutional AI (vendor) | **Platform-tier hidden system prompt; data-dimension rules baked in; frozen set asserts** |
| Admin | Datasets + tools + members (RBAC) | Deployment config | Datasets + permissions + audit | None (consumer) | **Read-only views over KPI registry + ingest batches + scheduled jobs + reports; `X-Admin-Token` reuse; KPI def not editable in UI** |

The rightmost column is the differentiation statement: every competitor ships *some* of these capabilities; nobody ships them with **the Skill manifest as the safety contract** + **import-time irreversible desensitization** + **platform-tier trust floor** + **frozen-set eval gate as the only path to cron rerender**.

Dify's weak HITL + no KPI semantic layer + single-namespace datasets is the clearest market gap for a finance-IT-positioned product. Cube.dev + LangChain covers the metric defal angle but the eval/trust posture is DIY (no shipped gate, no platform-tier prompt).

## Open Questions (for Phase 14 planning)

1. **L1 hashing determinism:** Per-dataset salt vs per-tenant salt vs system-wide salt? Trade-off = cross-dataset joins vs leak risk on dataset compromise. Recommendation: per-dataset salt, stored alongside the dataset (compromising the salt compromises the data anyway).
2. **L2 bucketing thresholds:** Who picks the bucket boundaries? Data officer at ingest time vs fixed schema (`bucket(amount, 1000)`)? Recommendation: data officer declares per field; UI shows defaults.
3. **Cron conflict resolution:** What if two Skills with same `report_schedule` fire at the same time? Single scheduler + per-(skill, period) lock; if second run starts before first completes, it skips.
4. **PLATFORM_PROMPT.md versioning:** How are platform prompt updates gated? Git PR + eval-floor assertion (run platform-floor.jsonl against new prompt; refuse merge if any case regresses).
5. **Skill reload impact on cron:** If a Skill is hot-reloaded mid-rerender, what happens to the in-flight run? Recommendation: finish current run with old version, next run uses new (deterministic version pinning per audit row).
6. **Frozen set for KPI accuracy:** Who hand-curates the 50 golden KPI queries? Data officer + finance SME; eval set lives next to Skill (Phase 6 precedent).
7. **Static portal auth:** Are reports public to all chat users, or admin-only? Recommendation: same chat-user visibility as the Skill (Skill `reads` already gates who can see the data); audit row gates who *published*.
8. **Drift threshold configuration:** Per-Skill or per-KPI? Recommendation: per-KPI (data officer configures); default ±5%.
9. **Capability 7A classifier cost:** Adds 1 LLM call per top-level run. Acceptable? Eval set should include cost as a metric; gate includes cost ceiling.
10. **Capability 7C prompt pre-emption:** If Skill prompt says "always answer in English" and platform prompt says "always include refusal reason in original query language", which wins? Platform prompt wins (it's prepended, not appended; LLM recency bias favors later, so we put Skill prompt after platform prompt). Needs eval set assertion.

## Sources

**HIGH confidence (official/primary):**
- Cube.dev — [Semantic Layer / Pre-Aggregations](https://cube.dev/docs/product/caching): the canonical "metric defined in code, agent cannot define" pattern
- dbt — [Semantic Layer / MetricFlow](https://docs.getdbt.com/docs/build/about-metricflow): YAML-defined metrics + canonical query API
- LangChain docs — [Human-in-the-loop](https://docs.langchain.com/oss/python/langchain/human-in-the-loop), [NL2SQL safety patterns](https://docs.langchain.com/oss/python/langchain/sql): approval gates + DB user scoping + TTL tokens
- Metabase — [Question Library / Subscriptions](https://www.metabase.com/docs/latest/questions/sharing/question-library): predefined queries as first-class objects; cron subscriptions
- Anthropic engineering — [Building effective agents](https://www.anthropic.com/engineering/building-effective-agents): orchestrator-worker + planning + sub-agents
- OpenAI — [Tasks API](https://platform.openai.com/docs/api-reference/tasks): scheduled-task semantics
- GDPR — [Art. 25 Data Protection by Design](https://gdpr-info.eu/art-25-gdpr/), [Art. 32 Security of Processing](https://gdpr-info.eu/art-32-gdpr/): pseudonymization at earliest processing point
- PCI-DSS v4.0 — [Requirents 3 + 4](https://www.pcisecuritystandards.org/): protect stored cardholder data + masking
- Apache Airflow — [Scheduling & Cron](https://airflow.apache.org/docs/apache-airflow/stable/scheduler.html): cron semantics + idempotent task runs

**MEDIUM confidence (multiple secondary sources agree):**
- Snowflake column masking — [Docs](https://docs.snowflake.com/en/user-guide/security-column-dynamic-data-masking) + [Blog](https://www.snowflake.com/blog/dynamic-data-masking/): tag-based column masking as runtime transformer
- LangChain NL2SQL safety — [Three-layer defense pattern](https://blog.langchain.com/sql-agents/): architectural / query preview / runtime token
- Preset (Apache Superset cloud) — [Semantic Layer + Alerts](https://preset.io/blog/2024-8-15-semantic-layer/): scheduled alerts + dashboard subscriptions
- Notion / Coda Q&A — [Notion AI Q&A](https://www.notion.com/product/ai), [Coda Brain](https://coda.io/product/brain): embedded dashboard cards in text answers
- dlt (data load tool) — [Schema inference + tier metadata](https://dlthub.com/docs/dlt-ecosystem/verified-sources/): schema contract pattern
- ChatGPT Deep Research — [Product docs](https://help.openai.com/en/articles/10500283-deep-research-in-chatgpt): multi-hop → static artifact post-hoc

**LOW confidence (single-source / industry blog; flagged):**
- "Finance teams prefer Sparkline + 1-sentence context over full chart" — anecdotal (vendor blog); treat as direction, not law
- "Direct-database NL2SQL is the leading cause of LLM data leaks in finance" — industry survey (single source); treat as risk to design against
- "Cron in Skill manifest is novel" — relative-to-competitor claim; verify with Phase 14 spike

---
*Feature research for: SkillForge v1.0 — Business Data Support*
*Researched: 2026-08-21*
*Anchor scenario: finance-IT department data-support platform; reusable for any regulated industry with sensitive-data + aggregation-KPI patterns*