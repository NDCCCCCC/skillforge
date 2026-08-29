# Architecture: SkillForge v1.0 — Business Data Support (数据维度)

**Project:** SkillForge — Skill-driven Enterprise Agent Platform
**Researched:** 2026-08-21
**Milestone:** v1.0 — IT-department data platform upgrade (业务数据支撑)
**Base:** v0.8.7 codebase (Eino v0.9.14 + adk + DeepAgent + FanOutOrchestrator + multi-turn memory + HITL + sqlite audit/session/checkpoint)
**Confidence:** HIGH on layering/integration points; MEDIUM on KPI function interface contract (flagged for phase spike); HIGH on static-page renderer (existing patterns + FanOutOrchestrator handle 90%)

> This document EXTENDS `.planning/research/ARCHITECTURE.md` (v0.7.7 baseline) and `.planning/research/ARCHITECTURE-v08.md` (v0.8 Harness). It does NOT redesign. The v0.8 trust anchors (per-Run `ChunkAccumulator` → trust floor → grounding → critique) and the 3-archetype dispatch (`chat` / `deep` / `orchestrator`) are INVARIANTS — v1.0 integrates INTO them.

---

## 1. Executive Summary

The v1.0 milestone adds **7 capability blocks** onto a v0.8 harness that already owns trust-anchored Q&A, multi-turn memory, write-tool approval, DeepAgent planning, and sub-agent fan-out. The single most important architectural fact: **the existing dispatch surface (Phase 12/13) is already data-channel-agnostic**. `Manifest.Reads` is currently a `[]string` of tier patterns; v1.0 extends the *vocabulary* of that field (two new `data:*` patterns) without touching the dispatch mechanics. New capability blocks are mostly **NEW packages** (data layer, KPI registry, scheduler, static renderer, admin handlers) wired into the existing `server.Deps` container and `internal/runtime.DispatchAgent` funnel — never reaching inside `(*Agent).Run` or `(*FanOutOrchestrator).Run`.

| Capability | Architectural seam | New vs modified |
|---|---|---|
| 1. Data import + desensitization | NEW `internal/data/` (importer, desensitizer, dataset store) under `internal/store/sqlite` DSN; raw L1/L2 bytes NEVER persisted | NEW package; MODIFY `internal/store/sqlite/schema.go` (additive `dataset` / `dataset_row` / `kpi_function` / `kpi_snapshot` / `report` tables) |
| 2. KPI aggregation interface | NEW `internal/kpi/` registry + Tool archetype `kpi_call`; Skill declares `reads: ["data:aggregated_only"]` → loader resolves to `[]string{"data:aggregated_only"}`; tool whitelist intersects with `Manifest.Reads` | NEW package; MODIFY `internal/skill/manifest.go` (extend `validReadPatterns`), `internal/agent/tools` (new Tool factory) |
| 3. KPI Dashboard Skill | NEW `skills/kpi-dashboard/SKILL.md` + `agent: chat` archetype; uses `kpi_call` Tool; no new server routes | NEW skill file + supporting frozen eval set |
| 4. Monthly Report Skill | NEW `skills/monthly-report/SKILL.md` + `agent: deep` archetype (write_todos + multiple KPI calls + render template); publishable to static page via Data Report Tool | NEW skill file + markdown template |
| 5. Static report portal + scheduled re-render | NEW `internal/scheduler/` (cron via `robfig/cron/v3`) + `cmd/static-renderer` (binary that invokes DeepAgent + KPI calls + writes `report/<id>/<ts>/index.html`); HTTP `/reports/*` route under `cmd/server` | NEW package + binary; MODIFY `cmd/server/main.go` (static-file middleware if same binary, or HTTP client to renderer) |
| 6. Admin console (`/admin/*`) | NEW `internal/server/admin_*` handlers (import / KPI / Skill mgmt) + `/web/src/components/admin/*` UI; extends existing `/eval` admin token gate (`SKILLFORGE_ADMIN_TOKEN`) | NEW package + route group; MODIFY `internal/server/server.go` (add 6 routes) |
| 7. Production hardening | (a) MODIFY `internal/runtime/orchestrator.go` Step 1 — replace placeholder classifier with REAL LLM classifier (uses `cfg.ClassifierChatModel`); (b) NEW eval set + gate threshold for D8-07; (c) NEW `internal/prompt/platform_floor.go` that prepends a Skill-IMMUTABLE floor to every Skill's system prompt | MODIFY factory + Manifest prompt assembly; NEW `internal/prompt/` package |

**Build order (dependency-driven):** Phase 14 (data layer + KPI interface) → Phase 15 (KPI Dashboard Skill) → Phase 16 (Monthly Report Skill + static renderer) → Phase 17 (admin console + scheduler + production hardening) → Phase 18 (productionization = operator-driven UAT + eval gate raise + open-source release).

---

## 2. Current State → Target State

### 2.1 Current state (verified against code, 2026-08-21)

```
POST /api/chat (internal/server/chat.go chatHandler)
  ├─ decode → lastUserQuery (ai@7 {messages} body)
  ├─ session_id resolve (X-Session-Id header / crypto/rand)
  ├─ route: Router (Plan 05-02) OR findSkill fallback
  ├─ branch:
  │   ├─ s.Agent != nil  → runAgentPath
  │   │     → Agent.Run(ctx, sessionID, query)
  │   │        [system, user, ...history]
  │   │        ReAct loop → trust floor → grounding → critique
  │   └─ default         → VerdictPipeline
  │         → Retrieve(tiers=Manifest.ResolvedTiers())
  │         → LLM call → verdict JSON / refusal
  └─ writeAuditAndTurn → audit row + session_turn (user, assistant) pair
         assistant content = verdictJSON or refusal reason

Skill manifests (skills/<name>/SKILL.md):
  description: string
  agent: "chat" | "deep" | "orchestrator"
  reads: ["authority/**", "historical/**"]   ← ASCII tier pattern list
  writes: ["memory:facts", "skill_state"]    ← enumerated write targets
  steps: markdown body
  guardrails: {prohibitions, refusal_phrases, tier_limits}
  temperature: *float32  (required, pointer)

Runtime archetypes (internal/runtime/factory.go DispatchAgent):
  "chat"         → buildChatModelAgent (adk.NewChatModelAgent)
  "deep"         → buildDeepAgent (eino/adk/prebuilt/deep.New, MaxIteration:10)
  "orchestrator" → buildOrchestrator (5-step pipeline: Classify → Build → Fan-out → Merge → ApplyTrustStage)

Tool whitelist per Skill (D11-B1 BuildToolsForSkill in internal/agent):
  intersect(declared_writes, AllowedWriteTargets) + retrieve Tool (closes over ResolvedTiers)

Audit (internal/store/sqlite/audit.go):
  INSERT-only (T-3-AUDIT); 24 columns; HITL 3-row lifecycle; fanout RunPath/Transfer substrate; plan_todo cross-reference
```

Key facts that constrain v1.0:

- `Manifest.Reads` is a `[]string` of tier patterns validated by `ResolveTiers` (`internal/skill/tier.go:120`) against a hardcoded `TierEnum` map (`authority/historical/personal/draft` → CN-char VARCHARs). Adding `data:*` patterns requires a NEW resolver path — see §3.1.
- `BuildToolsForSkill` (D11-B1) hardcodes the Tool whitelist as `retrieve + declared_write_targets`; adding `kpi_call` requires a new branch gated by `reads:["data:aggregated_only"]`.
- `internal/runtime.DispatchAgent` is a PURE dispatcher (factory.go:175): no audit write, no SSE emit, no checkpoint write. That's the funnel v1.0 inserts INTO — never below it.
- `FanOutOrchestrator.classify()` (orchestrator.go:363) is a placeholder returning a single sub-query derived from the parent — v1.0 Phase 17 replaces it with a real LLM classifier call.
- The audit table's 24-column schema is the substrate every v1.0 lifecycle row extends (DATA-01 import audit → KPI call audit → static-page publish audit). Additive migration posture, mirroring Phase 11/12/13.
- `cmd/server/main.go` constructs Deps in 6 steps (env → loader → embedder → retriever → chatmodel → agent → store → registry → rewriter/classifier). Data + KPI + scheduler + admin follow the same fail-fast-on-misconfigure posture.

### 2.2 Target state (v1.0)

```
POST /api/chat                          POST /api/admin/imports
  │                                       POST /api/admin/kpis
  ▼                                       POST /api/admin/skills/:name
chatHandler                              GET  /api/admin/reports
  │ route by Manifest.Reads:              (NEW — admin token gated)
  │   ["authority", ...]    → VerdictPipeline (existing)
  │   ["data:aggregated_only"] → KPI pipeline (NEW)
  ▼                                          
runtime.DispatchAgent                      
  │ switch manifest.Agent:                  GET  /reports/<id>/<ts>/index.html
  │   "chat"         → ChatModelAgent          (NEW — served by static-renderer)
  │   "deep"         → DeepAgent (P12)        GET  /reports/index.json  (catalog)
  │   "orchestrator" → FanOutOrchestrator
  ▼                                          
shared ApplyTrustStage (P12-L1)              cron trigger (internal/scheduler)
  trust floor → grounding → critique         │
  Tools include retrieve + kpi_call           ▼
  (only when Skill's reads allows)         cmd/static-renderer
                                            │ build DeepAgent(skill=monthly-report)
                                            │ Run → multi KPI calls → render template
                                            │ write report/<id>/<ts>/index.html
                                            │ audit row: action_static_published

Additive audit substrate (T-3-AUDIT preserved):
  + ActionKpiCall         (per-KPI invocation)
  + ActionDataImported    (import batch lifecycle)
  + ActionStaticPublished (publish event with version)
  + ActionPlatformFloorHit (platform prompt override fired)
  + 4 columns: kpi_function, kpi_params_json, kpi_result_snapshot, kpi_version
```

---

## 3. Integration Questions

### 3.1 Q1 — Data layer: same store? new `internal/data` package? new connection pool?

**Decision: NEW package `internal/data/`; SAME sqlite store via `Store` interface; NO new connection pool.**

```
internal/data/
├── importer.go            — CSV/SQL/API ingest pipeline + L1 hash + L2 bucket
├── desensitizer.go        — L1 PII hash + L2 bucketing (极严 policy)
├── dataset_store.go       — high-level facade over store.Store (dataset + row tables)
├── kpi_registry.go        — pre-registered KPI function catalog (DATA-09 enforcement)
├── kpi_runner.go          — KPI function invocation engine (calls into registry; LLM never writes SQL)
├── audit_lifecycle.go     — ActionDataImported / ActionKpiCall / ActionStaticPublished audit helpers
└── policy.go              — central policy enum (TierL1PII | TierL2Business | TierL3Open)

internal/store/sqlite/schema.go:
  + dataset       (id, name, tier, imported_at, imported_by, row_count, schema_json)
  + dataset_row   (dataset_id, row_pk, bucket_key, hashed_pii_json, bucketed_l2_json, raw_l3_json, ts)
  + kpi_function  (id, name, description, params_schema_json, sql_template, definition_user, ts)
  + kpi_snapshot  (id, kpi_function_id, params_json, result_json, computed_at, computed_by)
  + report        (id, skill_id, slug, current_version, current_ts, schedule_cron, schedule_enabled)
  + report_version (report_id, ts, template_hash, kpi_snapshots_json, published_at, audit_id)

internal/store/store.go:
  + PutDataset / ListDatasets / GetDatasetRows (L1+L2 only — raw L1 NEVER persisted)
  + PutKpiFunction / ListKpiFunctions / GetKpiFunction
  + PutKpiSnapshot / QueryKpiSnapshots
  + PutReport / ListReports / GetReportCurrent / PutReportVersion
```

Rationale:

1. **Same sqlite store, additive tables.** The Phase 3/8/11/12/13 pattern is "additive `ALTER TABLE` migrations + new sibling tables" — v1.0 follows it. The `dataset_row` table stores ONLY `hashed_pii_json + bucketed_l2_json + raw_l3_json` — raw L1 NEVER persists (DATA-02). The "only desensitized data" invariant is a **schema invariant**, not a runtime check: there is no `raw_l1_json` column anywhere in the schema.
2. **NEW package `internal/data/` (not `internal/store/data`).** The existing `internal/store` package is a thin SQL substrate (audit + session + checkpoint + plan_todos); v1.0's KPI registry, desensitizer, and importer are **domain logic** — they import the store interface but the store doesn't import them. Mirrors `internal/agent` vs `internal/store`: agent depends on store, not vice versa.
3. **No new connection pool.** SQLite WAL allows 4 readers + 1 writer concurrent; the existing `MaxOpenConns: 4` (sqlite.go:60) handles the new tables. KPI calls are short-lived (LLM never SELECTs raw — pre-registered functions only), so connection contention is not a concern. If Milvus-style horizontal scaling becomes a need later, the store.Store interface is the swap point (Postgres/MySQL drop-in, mirroring the retriever pattern).
4. **KPI registry tables are separate from audit.** `kpi_function` and `kpi_snapshot` live in their own tables (not inside `audit`) because (a) KPI definitions need transactional CRUD (admin edits a function → version row + audit), (b) snapshots are queryable independently (DATA-09 "audit still works after dataset is wiped" requires the snapshot to NOT depend on `dataset_row` rows). Audit row carries only a `kpi_snapshot_id` FK.
5. **Desensitization is policy-driven, not config-driven.** `policy.go` exports a single const `DefaultPolicy = PolicyExtremeStrict` (L1 hash + L2 bucket + L3 raw). Future "中等" / "轻度" modes are switchable via Skill-level `data:` metadata, but the default is fixed.

### 3.2 Q2 — KPI function interface: parallel to `fan_out_dispatch` Tool? new Tool archetype? Skill-defined action vs platform-defined?

**Decision: NEW Tool archetype `kpi_call` (sibling to `retrieve`); PLATFORM-DEFINED (the function catalog lives in the `kpi_function` table, not in `SKILL.md`).**

```
internal/agent/tools/
├── retrieve.go       (existing — closes over ResolvedTiers)
├── kpi_call.go       (NEW — closes over KPIRegistry; signature: call(kpi_name, params_json))
├── write_tools.go    (existing — Phase 11 memory:facts / report generation)
└── whitelist.go      (MODIFIED — adds kpi_call Tool when reads includes "data:aggregated_only")

internal/skill/manifest.go:
  + validReadPatterns: tier OR data:aggregated_only OR data:external:authorized (v2.0)
  + Loader validates: at most ONE data:* pattern (DATA-08 channel conflict guard)
```

Rationale:

1. **PLATFORM-DEFINED (not Skill-defined).** DATA-09 is the load-bearing constraint: "agent invokes KPI functions, never SELECTs raw data". If the function catalog lived in `SKILL.md`, an adversarial Skill author could define `kpi: "DROP TABLE …"` and the agent would faithfully execute it. By making KPI definitions live in the `kpi_function` table (administered through `/admin/kpis` under the admin token gate), the Skill body never contains SQL or arbitrary function code. The Skill's `kpi_call` Tool can only reference **names from the registry**; the registry validates the `sql_template` against a read-only DB user at definition time.
2. **Tool archetype `kpi_call` is sibling to `retrieve`** because both close over a channel (data channel vs tier slice). They share the BuildToolsForSkill seam but each is gated by a different `reads` pattern. The `kpi_call` Tool's signature is `Call(ctx, name string, paramsJSON string) (snapshotJSON, error)` — it looks up the `kpi_function` row, executes the `sql_template` against the read-only connection, and writes a `kpi_snapshot` row (immutable). The snapshot ID becomes a citation (DATA "数据引用 = 新引用类型") in the audit row.
3. **NOT new `ToolArchetype` enum** (there's no ToolArchetype enum today; tools are produced by factories, not enumerated). The cleanest seam is `BuildToolsForSkill` (D11-B1) gains one branch: if `Manifest.Reads` contains `"data:aggregated_only"` AND the Skill's `writes` doesn't contain write targets that would conflict (DATA-08), append a `kpi_call` Tool to the whitelist.
4. **Conflict guard (DATA-08).** A Skill declaring both `reads: ["authority/**", "data:aggregated_only"]` is structurally ambiguous (which channel?). Loader's `Validate` rejects this combination at load time — a Skill is either tier-only OR data-only OR (future) external-only. No "hybrid" Skills in v1.0 (DATA-04/05/10..14 external channel → v2.0).
5. **`kpi_call` is READ-ONLY from the agent's perspective.** It does NOT route through HITL (no `write_todos`/`memory:facts` involved); the only side effect is a `kpi_snapshot` row + an audit row. This preserves the HITL-05 invariant: write tools need approval; read tools don't.

### 3.3 Q3 — Static-page renderer: cron-triggered subprocess of `cmd/server`? separate `cmd/static-renderer`? library-only with a cron file?

**Decision: SEPARATE binary `cmd/static-renderer` + library `internal/report/render.go`. The cron lives in `cmd/server` (embedded scheduler — see Q4). The renderer is invoked via in-process Go call OR via `os.Exec` for isolation.**

```
cmd/server/main.go
  └─► internal/scheduler.New(cronSpec, jobs)         ← embedded scheduler
        └─► JobFunc: "0 8 * * *" → render.Report(skillID="monthly-report", slug="monthly")
              └─► internal/report/render.go Render()
                    ├─► loads skill manifest
                    ├─► builds DeepAgent (same path as runtime.DispatchAgent "deep")
                    ├─► runs DeepAgent with KPI calls + template render
                    └─► writes report/<id>/<ts>/index.html to disk
                          + audit row: action_static_published

internal/report/
├── render.go              — Render(ctx, skillID, params) — produces HTML + audit
├── template.go            — markdown → HTML template engine (no new dep; use html/template + goldmark)
├── catalog.go             — index.json generator (lists all report versions)
└── audit.go               — ActionStaticPublished lifecycle (3-row: triggered → rendered → published)

web/public/reports/        ← static files served by cmd/server via embed.FS
  └── <report-id>/
        └── <ts>/
              ├── index.html         (rendered)
              ├── data.json          (KPI snapshots embedded for transparency)
              └── audit.json         (cross-link to audit trail)
```

Rationale:

1. **Library `internal/report/render.go` is the canonical renderer.** It can be invoked from (a) `cmd/server`'s scheduler job, (b) a manual HTTP endpoint `POST /api/admin/reports/:slug/render` (admin-only), (c) a future `cmd/static-renderer` standalone CLI for one-off renders. Three invocation paths share the same library — single source of truth, multiple entry points.
2. **Cron lives in `cmd/server`** (not a separate `cmd/scheduler` binary) because (a) the scheduler needs access to the same `Store` + `Embedder` + `Retriever` + `ChatModel` already wired in `cmd/server`, (b) splitting into `cmd/scheduler` would require cross-process coordination of secrets, db handles, and LLM client lifecycle, (c) the static-file serving can be co-located with the HTTP API via `embed.FS` (zero external HTTP fetch round-trip). The scheduler goroutine runs until `ctx` cancel — mirroring `Registry.Watch` (cmd/server/main.go:286).
3. **Static files served from `cmd/server` via `embed.FS`** (Go 1.16+ standard library). The `/reports/*` route serves `web/public/reports/...` from the embedded filesystem; updates happen via the renderer's `os.WriteFile` to a directory the embedded FS also reads on next request (no need to rebuild the binary). For production, the same files can be served from any static-file CDN — `web/public/reports/` is the canonical source of truth.
4. **NOT `os.Exec` of a separate binary.** Process isolation is appealing for blast radius (a DeepAgent crash can't take down the API server), but it doubles operational complexity (two binaries, two systemd units, two log streams). The Go scheduler goroutine + defer-recover pattern (Pitfall 4 mitigation, mirrors `FanOutOrchestrator.runSubAgent` recover) gives equivalent safety without the ops tax. If v2.0 needs horizontal scaling of the renderer, the library can be lifted into `cmd/static-renderer` later — the seam is clean.
5. **Audit lineage (DATA "数据引用 = 新引用类型").** Each rendered report version carries an `audit.json` cross-link to the audit row(s) of the underlying KPI calls. The audit row already carries `kpi_snapshot_id` (Q1 schema); the renderer's audit row `action_static_published` carries `report_id + version_ts + kpi_snapshot_ids[]`. Full replay from audit trail alone — DATA's "数据清了不影响回放" invariant holds.

### 3.4 Q4 — Scheduler: embed in `cmd/server`? separate `cmd/scheduler`? library-only with a cron file?

**Decision: EMBEDDED in `cmd/server` via `internal/scheduler/`; library + cron file + optional HTTP trigger.**

```
internal/scheduler/
├── scheduler.go       — New(spec string, store Store, executor Executor) — wraps robfig/cron/v3
├── jobs.go            — JobFunc signature; ReportJob / DataTTLCleanupJob / EvalRecalcJob
├── triggers.go        — JobTrigger interface (cron | manual | on-event)
└── store.go           — schedule_config table (report_id, cron_spec, enabled, last_run, next_run)
```

Rationale:

1. **Embedded scheduler is the v0.7.6+ posture** (the existing `Registry.Watch` is an embedded fsnotify goroutine). Adding a second embedded scheduler (cron) follows the same pattern: one process, one store, one config surface, one log stream. The operator sees "[OK] scheduler: 3 jobs registered" in the same startup log they already read.
2. **`robfig/cron/v3` (MIT, no transitive deps, mature)** is the standard. JobSpec uses the standard 5-field cron syntax with second-precision extension if needed. Job functions are `func(ctx) error` — they receive the scheduler's context (cancelled on server shutdown, mirroring `Registry.Watch`'s `ctx` cancellation pattern).
3. **Schedule config is in sqlite (`schedule_config` table), not a cron file.** A cron file (`config/schedule.yaml`) is a deployment artifact that requires a deploy to change; a sqlite row is editable via the admin console (`PUT /api/admin/schedules/:id`) and audit-logged (every cron change is an INSERT into audit with `action_schedule_changed`). Mirrors how approval_rules and budget_layer config are stored (sqlite-backed, admin-editable).
4. **Manual + on-event triggers are first-class** (not just cron). `POST /api/admin/reports/:slug/render` triggers an immediate render; the same trigger fires when a dataset row is imported (so a KPI dashboard's "live preview" updates within seconds, not on the next cron tick). Cron is one of three trigger modes sharing the same `JobFunc` signature.
5. **Concurrency: one scheduler instance, job-level locking.** SQLite + WAL means concurrent writers serialize; the scheduler uses a per-job advisory lock (a `schedule_run` row with `status=running`) to prevent overlapping runs of the same report (a slow render that overruns the next cron tick must NOT start a parallel render). This mirrors the existing `TestSessionIsolation50x5` discipline — concurrent jobs must NOT corrupt state.

### 3.5 Q5 — Admin console: extend `/eval`? new `/admin` route? sibling route group?

**Decision: NEW route group `/api/admin/*`; SHARED admin token gate with `/api/eval/*`; UI is `web/src/components/admin/*` mounted under `/admin` route.**

```
internal/server/
├── server.go               (MODIFIED — add 6 admin routes + admin-token middleware)
├── admin_imports.go        (NEW — POST /api/admin/imports, GET /api/admin/imports/:id)
├── admin_kpis.go           (NEW — CRUD on kpi_function table)
├── admin_skills.go         (NEW — CRUD wrapping the existing /api/skills surface, gated by admin token)
├── admin_reports.go        (NEW — list + manual render trigger + schedule config)
├── admin_schedules.go      (NEW — CRUD on schedule_config table)
└── admin_middleware.go     (NEW — X-Admin-Token via crypto/subtle.ConstantTimeCompare, mirrors eval admin gate)

web/src/components/admin/
├── AdminShell.tsx          (NEW — top nav: Imports | KPIs | Skills | Reports | Schedules)
├── ImportsPanel.tsx        (NEW — CSV upload + dataset listing + TTL config)
├── KPIPanel.tsx            (NEW — KPI definition editor + dry-run)
├── SkillsPanel.tsx         (NEW — reuses existing skill list + admin-only "delete" button)
├── ReportsPanel.tsx        (NEW — report list + render now + schedule edit)
└── SchedulesPanel.tsx      (NEW — cron expression editor + last-run log)
```

Rationale:

1. **NEW route group `/api/admin/*`** (not nested under `/api/eval/*` — `/eval` is eval-specific and `/admin` is broader). Mirrors how `/api/eval/*` is its own group today (server.go:306-308). The eval admin gate becomes a SHARED middleware (`admin_middleware.go`) consumed by both `/api/eval/*` and `/api/admin/*`. Same `SKILLFORGE_ADMIN_TOKEN` env var → same auth posture (ASVS V6.2.1 via `crypto/subtle.ConstantTimeCompare`).
2. **Six admin routes total** — bounded by DATA-01..06 + DATA-08/09 admin visibility needs: imports, KPIs, skills (extended), reports, schedules, audit (reuses existing `/api/audit` with admin-token gate). The skill CRUD reuses the existing wizard (`/api/skills/wizard`) with an admin-only "delete" extension; no new skill authoring surface is needed for v1.0.
3. **Admin UI is a separate route `/admin` in the FE** (mirrors how `/eval` is `/eval` today). The Zustand store gain is minimal — admin UI doesn't need `messages` (preserves Pitfall 7 / D14 invariant); admin state is purely form-state held in component-local `useState` + TanStack Query for server state. NO `messages` field touches the admin UI (D14 unbroken).
4. **No new wire types.** Admin UI is JSON over REST + SSE is not used (admin actions are short-lived — upload, edit, trigger). The D-04 transport-swap invariant stays green because no `UIMessageChunk` change occurs in admin routes.

### 3.6 Q6 — Fan-out LLM classifier: replace Step 1 `Classify` or wrap it?

**Decision: REPLACE the placeholder `classify()` (orchestrator.go:363) with a REAL LLM classifier call inside the existing Step 1. Wrap with retry + fail-closed fallback (orchestrator refuses when classifier fails).**

```
internal/runtime/orchestrator.go::classify()
  CURRENT (13-03 placeholder):
    → return single sub-query = extractFirstUserText(input)
  TARGET (Phase 17 / v1.0 production hardening):
    → cfg.ClassifierChatModel.Generate(prompt)            [real LLM call]
    → parse JSON plan [{sub_query, sub_skill_id}, ...]
    → validate each plan.SubSkillID is in registry        [DATA-08 channel conflict guard]
    → validate plan.SubSkillID's reads intersect parent   [FAN-02 tier-closure invariant]
    → return plan (or fault + emitDecision("fanout_failed") on parse/validation error)

New file: internal/runtime/classifier.go
  - ClassifierPrompt: contains parent manifest's intent + list of sub-skills + tier closure constraints
  - ClassifierOutput: []SubQueryPlan (matches subQueryPlan struct)
  - ClassifierValidator: rejects empty/oversized plans; rejects sub-skills outside registry
```

Rationale:

1. **REPLACE, not WRAP.** The current `classify()` is a deliberate placeholder (orchestrator.go:362-370 comment: "the production classifier LLM call lands in 13-05"). v1.0 IS the production landing. Wrapping would add a layer of indirection without value — the placeholder has no callers that depend on its semantics.
2. **Retry policy: 1 retry on parse error, then fail-closed.** The classifier is the orchestrator's first step (no sub-agents have run yet); if the LLM emits malformed JSON twice, the orchestrator refuses with `Reason: "classifier: parse failed after 1 retry"` and writes `action_fanout_failed` to audit. This mirrors the existing `emitFailure` path (orchestrator.go:797).
3. **Sub-skill validation per FAN-02.** Each `plan.SubSkillID` is checked against `cfg.Registry` (loaded from `internal/skill/registry`); each must have a non-empty `ResolvedTiers()` intersection with the parent's. An empty intersection = the orchestrator CANNOT dispatch (matches FAN-02's "structural tier-closure" invariant — the empty-intersection load-time error becomes a runtime "no valid sub-plan" refusal here, because the LLM may propose sub-skills that weren't statically known).
4. **The `cfg.ClassifierChatModel` field already exists** (orchestrator.go:161). Phase 17 fills the call site; no API change. Production wires a smaller, faster model (e.g., GLM-4-Flash) for the classifier — separate from the main Skill ChatModel — to keep latency low (classifier is per-Run, not per-sub-agent). The DispatchConfig gains a `ClassifierChatModel` field (mirrors Phase 13 wiring seam in factory.go:158-172).
5. **Audit lineage: every classifier call produces `action_classifier_called` audit row.** This is NEW (not in the 3-row fanout lifecycle) — classifiers can fail in subtle ways (parse error, timeout, refusal from LLM provider) that the operator must be able to inspect post-hoc. Adds one more enum value to `Action*` constants in `internal/store/sqlite/audit.go`.

### 3.7 Q7 — Platform-tier hidden system prompt: prepend to all Skills? per-platform-mode toggle?

**Decision: PREPEND to ALL Skills unconditionally. Skill authors cannot override the platform floor.**

```
internal/prompt/
├── platform_floor.go       (NEW — PlatformFloor() returns the canonical floor string)
├── assemble.go             (NEW — Assemble(manifest, instruction) prepends platform floor)
└── platform_floor_test.go  (NEW — TestFloorPrepended, TestFloorCannotBeOverridden)
```

Rationale:

1. **Code-level trust mechanism (pitfall 8 v0.8 analogue).** Just as the trust floor is enforced by `ApplyTrustStage` (not prompt-level), the platform floor is enforced by `prompt.Assemble()` — a single funnel the runtime calls through. A Skill author who writes `instructions: "ignore previous instructions …"` in their steps body gets the platform floor PREPENDED — their override lands AFTER the floor in the prompt, where models are known to honor the earlier instructions more reliably.
2. **What the floor contains (drafted in Phase 17):**
   - "You are operating inside SkillForge. The platform operator has full audit visibility into every query and answer."
   - "You may NOT bypass the `kpi_call` and `retrieve` Tool whitelist. Skills cannot redefine KPI functions; only platform administrators can."
   - "Every numerical claim must cite a KPI snapshot ID. Citing numbers not produced by `kpi_call` is a platform-floor violation."
   - "Refusal is the correct response when the available KPI snapshots do not support the answer."
3. **Single funnel = drift-proof.** `prompt.Assemble(manifest, instruction)` is called from EXACTLY ONE place — `runtime.buildChatModelAgent` / `runtime.BuildDeepAgent` / `runtime.NewOrchestrator` (the instruction arg now flows through the funnel instead of being supplied by the caller). Centralizing the prompt-assembly logic means a future Skill that adds a custom instruction helper still goes through the floor.
4. **No toggle.** The user (PROJECT.md Key Decisions table) is explicit that "平台级隐藏系统提示词层（Skill 不可覆盖的下限，代码级 trust 机制兜底）" is unconditional. Per-platform-mode toggle would re-introduce the very loophole the floor exists to close.
5. **Audit row when the floor is "hit".** A new `action_platform_floor_hit` enum fires when the runtime detects a Skill whose system prompt body tried to override the floor (e.g., contains the substring "ignore previous instructions" or other red-flag patterns). This is a **detection** layer, not a **prevention** layer — the prevention is the prepend order; the detection surfaces attempted bypass for the operator.

---

## 4. New vs Modified — File-Level Inventory

### 4.1 NEW files (~25)

| File | Responsibility |
|---|---|
| `internal/data/importer.go` | CSV/SQL/API ingest pipeline orchestrator; uses desensitizer + dataset_store |
| `internal/data/desensitizer.go` | L1 PII hash + L2 bucketing + L3 raw passthrough; configurable policy |
| `internal/data/dataset_store.go` | high-level facade over `store.Store` for dataset / row / schema tables |
| `internal/data/kpi_registry.go` | pre-registered KPI function catalog; SQL template + params schema validation |
| `internal/data/kpi_runner.go` | KPI function invocation engine (read-only DB connection + snapshot persistence) |
| `internal/data/audit_lifecycle.go` | `ActionDataImported` / `ActionKpiCall` audit row helpers |
| `internal/data/policy.go` | policy enum (extreme / medium / mild — v1.0 ships extreme only) |
| `internal/agent/tools/kpi_call.go` | the `kpi_call` Tool factory (closes over `KPIRunner`) |
| `internal/agent/tools/whitelist.go` (MODIFIED) | adds `kpi_call` Tool when `reads` includes `data:aggregated_only` |
| `internal/skill/manifest.go` (MODIFIED) | `validReadPatterns` extension + cross-channel guard (DATA-08) |
| `internal/skill/tier.go` (MODIFIED) | `ResolveTiers` extended to handle `data:*` patterns (separate path) |
| `internal/scheduler/scheduler.go` | robfig/cron/v3 wrapper + job lifecycle |
| `internal/scheduler/jobs.go` | JobFunc signature; `ReportJob` / `DataTTLCleanupJob` / `EvalRecalcJob` |
| `internal/scheduler/triggers.go` | JobTrigger interface (cron | manual | on-event) |
| `internal/scheduler/store.go` | `schedule_config` table + `schedule_run` lock table |
| `internal/report/render.go` | canonical renderer (library) — invoked from scheduler + admin endpoint |
| `internal/report/template.go` | markdown → HTML template engine |
| `internal/report/catalog.go` | index.json generator |
| `internal/report/audit.go` | `ActionStaticPublished` lifecycle |
| `internal/prompt/platform_floor.go` | `PlatformFloor()` canonical string |
| `internal/prompt/assemble.go` | `Assemble(manifest, instruction)` — single prompt-assembly funnel |
| `internal/runtime/classifier.go` | real LLM classifier for FanOutOrchestrator Step 1 |
| `internal/runtime/classifier_test.go` | parse-error retry, empty-intersection refusal, audit lineage |
| `cmd/static-renderer/main.go` | standalone binary for one-off manual renders (optional v1.0; library is canonical) |
| `skills/kpi-dashboard/SKILL.md` | NEW Skill — KPI queries + mini trends + audit-linked citations |
| `skills/kpi-dashboard/eval-frozen.json` | frozen eval set (GEVL-05 pattern) |
| `skills/monthly-report/SKILL.md` | NEW Skill — DeepAgent + multi KPI calls + template render |
| `skills/monthly-report/template.md` | markdown template with `{{kpi:revenue_q1}}` placeholders |
| `web/src/components/admin/AdminShell.tsx` | top nav + auth check |
| `web/src/components/admin/ImportsPanel.tsx` | CSV upload + dataset listing |
| `web/src/components/admin/KPIPanel.tsx` | KPI definition editor |
| `web/src/components/admin/SkillsPanel.tsx` | extends existing skill list |
| `web/src/components/admin/ReportsPanel.tsx` | report list + manual render trigger |
| `web/src/components/admin/SchedulesPanel.tsx` | cron expression editor |
| `web/src/app/admin/page.tsx` (or `routes/admin.tsx`) | Next-style nested route for `/admin` |

### 4.2 MODIFIED files (~15)

| File | Change |
|---|---|
| `internal/store/store.go` | +6 new methods (PutDataset, ListDatasets, GetDatasetRows, PutKpiFunction, ListKpiFunctions, GetKpiFunction, PutKpiSnapshot, QueryKpiSnapshots, PutReport, ListReports, GetReportCurrent, PutReportVersion) |
| `internal/store/sqlite/schema.go` | +6 new tables (dataset, dataset_row, kpi_function, kpi_snapshot, report, report_version, schedule_config, schedule_run) |
| `internal/store/sqlite/audit.go` | +5 new `Action*` constants (ActionDataImported, ActionKpiCall, ActionStaticPublished, ActionClassifierCalled, ActionPlatformFloorHit, ActionScheduleChanged) + 4 new columns (kpi_function, kpi_params_json, kpi_result_snapshot, kpi_version) |
| `internal/skill/manifest.go` | `validReadPatterns` accepts `data:aggregated_only` (DATA-07) |
| `internal/skill/tier.go` | `ResolveTiers` routes `data:*` patterns to a separate resolver; no CN-char tier emitted |
| `internal/skill/validate.go` | cross-channel guard: rejects reads mixing tier + data:* patterns (DATA-08) |
| `internal/agent/tools/whitelist.go` (or equivalent) | adds `kpi_call` Tool to whitelist when reads allows |
| `internal/server/server.go` | +6 `/api/admin/*` routes + admin middleware registration |
| `internal/runtime/factory.go` | `DispatchConfig.ClassifierChatModel` field (Phase 17); `BuildDeepAgent` instruction now flows through `prompt.Assemble` |
| `internal/runtime/orchestrator.go` | Step 1 `classify()` replaced with real LLM call |
| `cmd/server/main.go` | +scheduler init + admin token env + initial schedule seeding |
| `go.mod` | +`github.com/robfig/cron/v3` (cron); +`github.com/yuin/goldmark` (markdown rendering) |
| `eval/frozen/` | +`kpi-dashboard.lock.json` + `monthly-report.lock.json` |
| `web/src/store/app-store.ts` | NO `messages` field added (D14 preserved); admin state is component-local |
| `web/src/App.tsx` | +`/admin` route |

### 4.3 File count summary

- **NEW: ~33 files** (28 Go + 2 SKILL.md + 3 web components)
- **MODIFIED: ~15 files**
- **Total: ~48 files touched**
- **New Go dependencies: 2** (`robfig/cron/v3` + `yuin/goldmark`); both are MIT, both have minimal transitive closure, both are already approved by `.planning/REQUIREMENTS.md` "open-source distribution" posture

---

## 5. Data Flow — Happy Path + Audit/Replay Path

### 5.1 Happy path: import → desensitize → store → KPI query → Skill invoke → answer → audit

```
┌─────────────────────────────────────────────────────────────────────┐
│ 1. DATA IMPORT (admin)                                              │
│                                                                     │
│ POST /api/admin/imports  (admin token gated)                        │
│   body: {name: "transactions_2026q1", source_type: "csv",           │
│          file: <binary>, schema: [...], policy: "extreme_strict"}   │
│                                                                     │
│ handler:                                                            │
│   ├─► admin_middleware.go → X-Admin-Token check                     │
│   ├─► data.Importer.Run(ctx, req)                                   │
│   │     ├─► desensitizer.Apply(row)                                 │
│   │     │     L1 (PII):  hash("user_42") → "h:7f3a…"                │
│   │     │     L2 (biz):  bucket(amount, 1000) → "[1000,2000]"       │
│   │     │     L3 (open): passthrough                                 │
│   │     ├─► dataset_store.PutDataset + PutDatasetRow × N            │
│   │     │     (raw L1 NEVER persisted — schema-level invariant)     │
│   │     └─► audit_lifecycle.Write(ActionDataImported)               │
│   └─► return {dataset_id, row_count, kpi_candidates[]}              │
│                                                                     │
│ Audit row (T-3-AUDIT):                                              │
│   action="action_data_imported", kpi_function="", kpi_params_json=  │
│   "{dataset_name, row_count, policy}", verdict_json="{}"            │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│ 2. KPI REGISTRATION (admin)                                         │
│                                                                     │
│ POST /api/admin/kpis  (admin token gated)                           │
│   body: {name: "monthly_revenue", description: "...",               │
│          params_schema: {year: "int", month: "int"},                │
│          sql_template: "SELECT SUM(bucketed_amount) ...",           │
│          dry_run_params: {year: 2026, month: 1}}                    │
│                                                                     │
│ handler:                                                            │
│   ├─► dry_run against read-only DB user → result preview           │
│   ├─► kpi_registry.Validate(sql_template, params_schema)            │
│   │     ├─► SQL syntax check (sql/parser)                           │
│   │     ├─► SQL safety: must be SELECT-only (no INSERT/UPDATE/DDL)  │
│   │     └─► params schema type-check against test execution         │
│   └─► store.PutKpiFunction(validated_def) + audit row               │
│                                                                     │
│ Audit row:                                                          │
│   action="action_kpi_registered", kpi_function="monthly_revenue",   │
│   kpi_params_json="{}", kpi_version="v1"                            │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│ 3. CHAT QUERY — "本月营业额?" (user)                                 │
│                                                                     │
│ POST /api/chat  {messages: [...], skill_id: "kpi-dashboard"}        │
│                                                                     │
│ chatHandler:                                                        │
│   ├─► router picks skill="kpi-dashboard"                            │
│   │     (intent match on description, Plan 05-02)                   │
│   ├─► Manifest.Reads = ["data:aggregated_only"]                     │
│   │     → BuildToolsForSkill appends kpi_call Tool                  │
│   ├─► runtime.DispatchAgent(manifest.Agent = "chat")                │
│   │     → ChatModelAgent                                           │
│   │     → instruction = prompt.Assemble(manifest, rawInstruction)   │
│   │     → platform floor PREPENDED (Q7)                             │
│   ├─► ChatModelAgent Run loop:                                      │
│   │     user: "本月营业额?"                                         │
│   │     agent decides → calls kpi_call Tool:                        │
│   │       name="monthly_revenue", params={year:2026,month:8}        │
│   │     → internal/data/kpi_runner.Call()                           │
│   │         ├─► look up kpi_function row                            │
│   │         ├─► execute sql_template against read-only DB user      │
│   │         ├─► result = {"revenue": 1234567}                       │
│   │         ├─► store.PutKpiSnapshot(immutable)                    │
│   │         └─► audit row: action_kpi_call                          │
│   │     agent receives kpi_snapshot_id + result                     │
│   │     agent emits verdict:                                        │
│   │       {status: "ok", confidence: 0.95,                          │
│   │        citations: [{kind:"kpi_snapshot", id:"snap_abc123",      │
│   │                     params:{...}, result:{...}}]}               │
│   ├─► ApplyTrustStage (existing — trust floor passes because        │
│   │     citations are within-allowed-channel)                       │
│   ├─► SSE emits UIMessageChunk verdict (D-04 invariant: no new     │
│   │     chunk types; citation kind="kpi_snapshot" rides in the      │
│   │     existing source-document part shape)                        │
│   └─► writeAuditAndTurn (existing — verdict JSON + audit row)       │
│                                                                     │
│ Audit chain for one query:                                          │
│   - action="" (verdict row) + verdict_json + kpi_snapshot_ids[]     │
│   - action="action_kpi_call" + kpi_function="monthly_revenue" +     │
│     kpi_params_json + kpi_result_snapshot + kpi_version            │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│ 4. REPLAY PATH — operator inspects audit                             │
│                                                                     │
│ GET /api/audit?session_id=... (admin or owner)                      │
│   → returns audit rows including the kpi_snapshot_id references     │
│                                                                     │
│ GET /api/admin/kpis/snapshots/:id  (admin)                          │
│   → returns snapshot JSON verbatim (the answer was computed from    │
│     this exact snapshot; dataset may have been wiped since)         │
│                                                                     │
│ Invariant: the answer can be reconstructed from audit + snapshot    │
│ even if dataset_row rows have been TTL-cleaned (DATA "数据清了       │
│ 不影响回放"). The snapshot is the immutable record.                 │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.2 Audit/replay path: full lineage for a static page

```
┌─────────────────────────────────────────────────────────────────────┐
│ 5. SCHEDULED REPORT RENDER                                          │
│                                                                     │
│ internal/scheduler fires "0 8 1 * *" (monthly 1st @ 08:00):         │
│   ├─► ReportJob.Execute(ctx, {skill: "monthly-report"})             │
│   ├─► schedule_run row INSERT (status=running, advisory lock)       │
│   ├─► internal/report/render.go::Render(ctx, skill, params)         │
│   │     ├─► loads skill="monthly-report" manifest                   │
│   │     │     Manifest.Agent = "deep" → DeepAgent                   │
│   │     │     Manifest.Reads = ["data:aggregated_only"]             │
│   │     ├─► runtime.DispatchAgent(archetype="deep")                 │
│   │     │     → BuildDeepAgent (P12-L2 MaxIteration:10)             │
│   │     │     → instruction = prompt.Assemble (floor prepended)     │
│   │     ├─► DeepAgent Run:                                          │
│   │     │     todo: [load kpi:revenue, load kpi:active_users,       │
│   │     │           load kpi:churn_rate, render template]          │
│   │     │     each todo → kpi_call Tool → snapshot + audit row      │
│   │     │     final todo: render template.md with snapshot values   │
│   │     │     → emits: {rendered_html, kpi_snapshot_ids[]}         │
│   │     ├─► writeFile report/<id>/<ts>/index.html                   │
│   │     ├─► writeFile report/<id>/<ts>/data.json (snapshots embed) │
│   │     ├─► writeFile report/<id>/<ts>/audit.json (cross-link)      │
│   │     ├─► store.PutReportVersion({ts, kpi_snapshot_ids,           │
│   │     │                              template_hash, audit_id})    │
│   │     └─► audit_lifecycle.Write(ActionStaticPublished)            │
│   ├─► schedule_run row UPDATE (status=completed, duration_ms)       │
│   └─► emitDecision("report_rendered") → SSE (if admin connected)   │
│                                                                     │
│ Audit chain for one report render:                                  │
│   - action="action_static_published" + kpi_snapshot_ids[] +         │
│     template_hash + ts + audit_id                                   │
│   - action="action_kpi_call" × N (one per KPI invoked)              │
│   - action="action_classifier_called" (DeepAgent's intermediate     │
│     classifier calls — NEW Phase 17 audit row)                      │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│ 6. STATIC PAGE SERVING + INSPECT                                    │
│                                                                     │
│ GET /reports/<id>/<ts>/index.html  (public, no auth)                │
│   → serves rendered HTML                                            │
│                                                                     │
│ GET /reports/<id>/<ts>/data.json   (public, no auth)                │
│   → snapshot values + audit cross-link                              │
│                                                                     │
│ GET /api/audit?action=action_static_published (admin)               │
│   → audit lineage shows: which KPI snapshots → which template      │
│     version → which user-visible HTML                                │
│                                                                     │
│ GET /api/audit?action=action_kpi_call&since=... (admin)             │
│   → all KPI calls in the time range (operational transparency)      │
│                                                                     │
│ Invariant: the entire report can be reconstructed from audit        │
│ alone — every KPI snapshot is immutable; every template hash is     │
│ pinned in the audit row; the rendered HTML is reproducible by       │
│ re-running the DeepAgent with the same snapshots (SC-6 fidelity).  │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.3 End-to-end lineage: a single rendered report

```
Audit row 1: action="" (verdict row for one KPI query inside the render)
  verdict_json="...", kpi_snapshot_ids=["snap_42"]
Audit row 2: action="action_kpi_call"  kpi_function="monthly_revenue"
  kpi_params_json='{"year":2026,"month":7}'
  kpi_result_snapshot='{"revenue":1234567}'
  kpi_version="v3"
  snapshot_id="snap_42" (immutable; stored in kpi_snapshot table)
Audit row 3: action="action_static_published"  report_id="rpt_monthly_2026_07"
  kpi_snapshot_ids=["snap_42","snap_43","snap_44"]
  template_hash="sha256:abc..."
  ts="2026-08-01T08:00:00Z"
  audit_id=3 (self-reference for cross-link)
```

Replaying: admin runs `GET /api/audit?action=action_kpi_call&id=42` → snapshot → input to template → same HTML output (deterministic given same model + same template + same snapshots).

---

## 6. Suggested Build Order (Dependency-Ordered)

| # | Phase | Deliverable | Unblocks | Depends on |
|---|---|---|---|---|
| **14** | **Data layer + KPI interface** | `internal/data/` (importer, desensitizer, dataset_store); `internal/store/sqlite` schema additions (6 tables); `internal/agent/tools/kpi_call.go`; admin routes for imports + KPIs; admin UI for imports + KPIs; `internal/skill/tier.go` extends with `data:*` patterns; `internal/skill/validate.go` adds cross-channel guard | KPI Dashboard Skill (15), Monthly Report Skill (16) | v0.8 baseline |
| **15** | **KPI Dashboard Skill** | `skills/kpi-dashboard/SKILL.md` (agent=chat, reads=data:aggregated_only); frozen eval set `eval/frozen/kpi-dashboard.jsonl` + `kpi-dashboard.lock.json`; `transport-swap` regression test for `data-kpi-citation` UI part | Monthly Report Skill (16) | 14 |
| **16** | **Monthly Report Skill + static renderer** | `skills/monthly-report/SKILL.md` (agent=deep, reads=data:aggregated_only); template.md; `internal/report/` (render, template, catalog, audit); `/reports/*` HTTP route via embed.FS; `internal/scheduler/` library + initial cron registration | Scheduler admin (17), production hardening (17) | 14, 15 |
| **17** | **Admin console + production hardening** | `/api/admin/*` routes (schedules + reports + skills extension); admin UI panels (schedules, reports, skills); `internal/prompt/platform_floor.go` + `assemble.go`; `internal/runtime/classifier.go` real LLM; `action_classifier_called` + `action_platform_floor_hit` audit rows; production eval gate raise (D8-07) | Production deployment (18) | 14, 15, 16 |
| **18** | **Productionization** | operator-driven UAT on live 35-case set; docker-compose v2 update with `cron_spec` config + sample schedules; README v1.0 features update; DIST-09 update with KPI dashboard + report eval numbers; live human checkpoint (FINAL) | open-source release | 14-17 |

**Ordering rationale:**

1. **Phase 14 first because both Skills need it.** The data layer (importer + desensitizer + dataset tables) AND the KPI interface (registry + runner + Tool) are the substrate; nothing else can exist without them.
2. **Phase 15 before 16** because the chat-archetype KPI Dashboard Skill is structurally simpler (single-turn, single KPI call, no template rendering). Validating the Tool → audit → snapshot → citation path on the simpler Skill de-risks the deep-archetype Monthly Report's compound complexity (multi KPI + template + static publish).
3. **Phase 16's scheduler ships in 16, not 17** because the static renderer is USELESS without a way to trigger it. The scheduler library + `ReportJob` ship together; the admin UI for schedule editing ships in 17.
4. **Phase 17's classifier + platform floor are production hardening** — they improve robustness but do not add new user-visible features. They ship after the new features are validated end-to-end.
5. **Phase 18 is the milestone close-out** — operator UAT + open-source packaging. It cannot start until all 4 capability blocks ship.

**Parallel work windows (single developer, multi-branch):**

- Wave A (14, 15, 16) is sequential — each unblocks the next.
- Wave B (17's classifier) is INDEPENDENT of the data layer — it only touches `internal/runtime/orchestrator.go` and the audit table. Can ship as a parallel branch from main.
- Wave C (17's admin console + scheduler admin UI) depends on 14-16.
- Wave D (Phase 18) is sequential to 17.

---

## 7. Component Dependencies — What Cannot Be Built Without What

| Component | Hard dependency (cannot ship without) |
|---|---|
| `internal/data/importer.go` | `internal/data/desensitizer.go` (must exist for L1 hash + L2 bucket); `internal/store/sqlite/schema.go` `dataset` + `dataset_row` tables |
| `internal/data/kpi_runner.go` | `internal/store/sqlite/schema.go` `kpi_function` + `kpi_snapshot` tables; read-only DB connection (env var `KPI_DB_DSN`, separate from main `SQLITE_DSN`) |
| `internal/agent/tools/kpi_call.go` | `internal/data/kpi_runner.go` (Tool closes over the runner) |
| `internal/skill/manifest.go` `data:aggregated_only` support | `internal/store/sqlite/schema.go` kpi_function table (registry must exist for validation) |
| `skills/kpi-dashboard/SKILL.md` | `internal/data/kpi_runner.go` + `internal/agent/tools/kpi_call.go` + `internal/skill/manifest.go` data:* support |
| `skills/monthly-report/SKILL.md` | `internal/report/render.go` + `internal/scheduler/jobs.go` ReportJob + `internal/agent/tools/kpi_call.go` |
| `internal/report/render.go` | `internal/runtime.DispatchAgent` deep archetype + `internal/data/kpi_runner.go` |
| `internal/scheduler/scheduler.go` | `robfig/cron/v3` dependency + `internal/store/sqlite/schema.go` `schedule_config` + `schedule_run` tables |
| `internal/server/admin_*.go` | `internal/data/` packages (CRUD targets) + `internal/scheduler/` (schedule CRUD target) |
| `internal/prompt/platform_floor.go` | NONE — ships independently (only modifies runtime factory prompt assembly in Phase 17) |
| `internal/runtime/classifier.go` | `internal/skill/registry.go` (must have `GetByID` lookup for sub-skill validation); `internal/store/sqlite/audit.go` ActionClassifierCalled enum |
| `/admin` FE route | 6 admin endpoints (cannot have empty admin shell) |

**Critical-path order (single developer, no parallel branches):** Phase 14 → 15 → 16 → 17 → 18. Total ~5 phases × 4 plans × 5 tasks = ~100 task checkpoints.

**Parallel-safe subsets (can run in different git worktrees):**
- Phase 14 (data + KPI) + Phase 17's classifier (only touches `internal/runtime/`) — independent files
- Phase 17's prompt platform floor + Phase 17's classifier — both touch `internal/runtime/factory.go` (prompt funnel) but at different lines (Phase 17 prompt = `internal/prompt/assemble.go`; classifier = `internal/runtime/classifier.go`)
- Phase 17's admin UI panels (5 panels × 1-2 files each) — independent components

---

## 8. Carried-Forward Pitfalls (v0.7.7–v0.8) Re-Mitigated

The v0.7.7-v0.8 PITFALLS.md carries 10 critical pitfalls. Below are the ones re-mitigated explicitly by v1.0 (not just inherited):

| Pitfall | v1.0 Mitigation |
|---|---|
| **P1 — 历史注入污染 trust floor** | KPI citation kind=`kpi_snapshot` is appended to the existing citation whitelist (P13-L9 chunk-accumulator model); snapshots are produced by the tool's per-Run invocation, NEVER from history. `prompt.platform_floor` adds an explicit "历史 KPI 数据为非证据" line. |
| **P3 — 写工具无审计行** | `kpi_call` is READ-ONLY (no HITL approval needed) but produces its own `action_kpi_call` audit row — preserving the "every platform action has an audit row" invariant without the HITL gate. |
| **P4 — SSE 断连 + 内存 checkpoint** | Not relevant (no new interrupt points in v1.0; existing checkpoint architecture unchanged). |
| **P5 — Interrupt/Resume 迁移陷阱** | Not relevant (v1.0 adds no new interrupt points; all v1.0 reads go through the read-only kpi_call tool, never write tools). |
| **P6 — checkpoint 跨版本失效** | Indirectly relevant: static reports carry `template_hash` in their audit row (SC-6 fidelity); if the template is updated, the next render produces a NEW version (append-only) — never silently overwrites. The `report_version` table is INSERT-only (mirrors audit INSERT-only posture). |
| **P7 — Zustand 无 messages 不变量** | **Re-mitigated explicitly**: admin UI lives in `web/src/components/admin/*` with component-local `useState` + TanStack Query for server state; NO additions to `web/src/store/app-store.ts`. The existing file-level comment banning `messages` extends to "and any state that should live in messages parts". |
| **P8 — DeepAgent 成本爆炸** | **Re-mitigated explicitly**: the DeepAgent's MaxIteration:10 (P12-L2) and per-Run token budget (P12) apply to the Monthly Report Skill too — same factory path. The report render runs at cron time (off the hot path), so a 10-iteration × 50k-token budget gives ~30-90s render time per report. |
| **P9 — 子 agent tier 泄漏** | **Re-mitigated explicitly**: `data:aggregated_only` is a separate channel from tier patterns; `internal/skill/validate.go` rejects Skills mixing channels (DATA-08). The data channel has its own tool (`kpi_call`); it cannot access the tier retriever. |
| **P10 — 前端审批 UI 状态卡死** | Not relevant (no new approval UI in v1.0; kpi_call is read-only). |

**New v1.0-specific pitfalls (to be added to PITFALLS.md in Phase 17):**

- **P11 — KPI registry drift**: a Skill that calls `kpi_call` for a function later DELETED from the registry → execution fails at runtime. Mitigation: registry deletes are soft (set `enabled=false`, keep row); call-time check rejects disabled functions with a structured refusal.
- **P12 — Desensitization regression**: a policy change to "中等" would let L1 raw data persist. Mitigation: the `dataset_row` schema has NO `raw_l1_json` column — physically impossible to persist raw L1, regardless of policy.
- **P13 — Static report freshness**: a stale report with no visual "last rendered at" badge is misleading. Mitigation: every rendered HTML carries a `<meta name="skillforge:rendered_at">` + a "view live data" link back to the chat path.
- **P14 — Classifier hallucinated sub-skills**: the LLM classifier (Q6) might propose sub-skill IDs that don't exist. Mitigation: validator rejects unknown IDs at parse time, refuses the orchestrator Run.
- **P15 — Scheduler time skew**: cron runs at the operator's local time but renders against UTC-stored data. Mitigation: cron strings are interpreted in UTC; the UI shows "Next run at: 2026-09-01 08:00 UTC" explicitly.

---

## 9. Quality Gate Verification

### 9.1 Integration points identified with specific existing package names

✓ Q1: `internal/store/sqlite/schema.go` (additive tables), `internal/store/store.go` (interface methods), `internal/data/` (NEW package)
✓ Q2: `internal/agent/tools/whitelist.go` (Tool whitelist seam), `internal/skill/manifest.go` (read pattern vocabulary), `internal/skill/tier.go` (resolver)
✓ Q3: `cmd/server/main.go` (embedded), `internal/report/` (NEW library), `web/public/reports/` (embed.FS)
✓ Q4: `internal/scheduler/` (NEW package), `robfig/cron/v3` (dep), `cmd/server/main.go` (init)
✓ Q5: `internal/server/server.go` (route registration), `internal/server/admin_middleware.go` (NEW auth), `web/src/components/admin/*` (NEW UI)
✓ Q6: `internal/runtime/orchestrator.go` Step 1 `classify()`, `internal/runtime/classifier.go` (NEW), `internal/skill/registry.go` (sub-skill validation)
✓ Q7: `internal/prompt/platform_floor.go` (NEW), `internal/prompt/assemble.go` (NEW), `internal/runtime/factory.go` (instruction funnel)

### 9.2 New vs modified explicit count

- NEW: 33 files (28 Go + 2 SKILL.md + 3 web components)
- MODIFIED: 15 files
- TOTAL: 48 files touched
- New Go deps: 2 (`robfig/cron/v3` + `yuin/goldmark`)

### 9.3 Data flow covers full happy path AND audit/replay path

✓ §5.1 — happy path: import → desensitize → store → KPI query → Skill invoke → answer → audit
✓ §5.2 — audit/replay path: scheduled report render → static page → audit cross-link
✓ §5.3 — single report lineage: 3 audit rows linked by `kpi_snapshot_id`

### 9.4 Build order respects existing Phase dependencies

✓ 11 HITL → 12 planning → 13 fan-out are v0.8 INPUT capabilities (used in Phase 14+17)
✓ Phase 14 (data + KPI) → 15 (KPI Skill) → 16 (Report Skill + static renderer) → 17 (admin + hardening) → 18 (productionization)
✓ Hard dependencies enumerated in §7 (KPI runner requires schema; Skill requires Tool; renderer requires scheduler; admin requires both)

### 9.5 Pitfalls 1-7 from v0.7.7-v0.8 carried forward and explicitly re-mitigated

✓ §8 table maps all 10 v0.7.7-v0.8 pitfalls to v1.0 mitigation (or "not relevant" with rationale)
✓ 5 new v1.0-specific pitfalls identified (P11-P15)

### 9.6 D-04 transport-swap invariant must remain unbroken (no new SSE chunk types)

✓ §5.1 step 3 explicitly notes "D-04 invariant: no new chunk types; citation kind='kpi_snapshot' rides in the existing source-document part shape"
✓ §5.2 step 6 admin UI does NOT use SSE (short-lived JSON over REST)
✓ The existing `SkillForgeChunk` union + `AssertAssignable<SkillForgeChunk, UIMessageChunk>` compile-time guard (Phase 4 lock) stays green

### 9.7 D8-04 legacy-interrupt ban must remain unbroken (no `NewInterruptAndRerunErr`)

✓ v1.0 adds no new interrupt points; `kpi_call` is read-only, never triggers HITL
✓ Static renderer does NOT use the agent interrupt API (DeepAgent's budget gate is the existing 3-layer budget, not interrupt)
✓ Classifier is a plain LLM call, not an interrupt path

### 9.8 D14 Zustand no-messages must remain unbroken

✓ §8 P7 explicitly re-mitigates: admin UI uses component-local state; NO additions to `app-store.ts`
✓ Existing file-level comment banning `messages` extends to "and any state that should live in messages parts"
✓ KPI citation kind rides in message parts (where it belongs); no store changes

---

## 10. Risks / Research Flags for Phases

| Flag | Phase | Question | Fallback |
|---|---|---|---|
| RF-1 | 14 | Desensitizer L1 hash collision rate under high cardinality (real names → same hash bucket) | Use SHA-256 truncated to 16 chars + salt per dataset; collision test in Phase 14 spike |
| RF-2 | 14 | `sql/parser` rejection of legitimate read-only SQL with subqueries / CTEs / window functions | Allow SELECT-with-CTE + window functions; reject anything matching `\b(INSERT\|UPDATE\|DELETE\|DROP\|CREATE\|ALTER\|TRUNCATE\|GRANT\|REVOKE)\b` case-insensitive (allowlist for SELECT, blocklist for DML/DDL); spike to validate parser coverage |
| RF-3 | 14 | KPI snapshot storage growth (immutable snapshots accumulate forever) | Snapshot TTL = 90 days default; admin can extend; audit chain references snapshot_id; expired snapshots are DELETEd (snapshot-only — audit rows remain, just lose the FK target) |
| RF-4 | 15 | Citation kind="kpi_snapshot" rendering in FE — does it conflict with the existing source-document part? | Treat as a discriminated sub-type of source-document; render with `<DataSourceBadge kind="kpi">` + value chip + parameter echo (per DATA "数据引用 = 新引用类型") |
| RF-5 | 16 | Static page rendering performance for reports with 20+ KPI calls | Parallel KPI calls via per-render goroutine pool (limit 4, mirrors P13-L1 errgroup limit); merge results before template render |
| RF-6 | 16 | `goldmark` extension API for custom `{{kpi:name}}` placeholders | Implement as goldmark AST walker post-parse; template substitution happens in `template.go` after markdown → HTML conversion |
| RF-7 | 17 | LLM classifier latency (extra LLM call before every orchestrator Run) | Use a smaller model (GLM-4-Flash); cache classifier output by (parent_skill + parent_query_hash) for 5 minutes |
| RF-8 | 17 | Platform floor "hit" detection (substring match is brittle; semantic detection is overkill) | Substring match for known red-flag patterns + structural check (steps body contains `system:` line outside an example); human review queue for ambiguous cases |
| RF-9 | 18 | Operator UAT on live 35-case re-run (BLOCKED-ENV from v0.8 ledger) | Same operator-gated path as v0.8; produce LIVE_EVAL_REPORT.md with PASS/FAIL per requirement; partial-PASS does not block milestone close |
| RF-10 | 14 | Cross-channel guard (DATA-08) rejection of mixed reads creates Skill authoring friction | Document the constraint clearly in SKILL_SCHEMA.md; provide wizard templates for "data-only" Skills separately from "tier-only" Skills |

---

## 11. Sources

- **Codebase (HIGH)**: `internal/runtime/orchestrator.go` (5-step pipeline + classify placeholder), `internal/runtime/factory.go` (DispatchConfig + DispatchAgent + 3 archetypes), `internal/runtime/sub_agent_builder.go` (FAN-02 tier-closure), `internal/runtime/merge.go` (FAN-03 deterministic merge), `internal/runtime/budget.go` (P12-L2 MaxIteration:10 hardcoded), `internal/runtime/deep_agent.go` (BuildDeepAgent factory), `internal/skill/manifest.go` (7-field schema + validAgentTypes), `internal/skill/tier.go` (ResolveTiers + TierEnum), `internal/store/store.go` (Store interface + AuditRow schema), `internal/store/sqlite/schema.go` (22 columns + sibling tables), `internal/store/sqlite/audit.go` (5 Action* enum constants + 3-row lifecycle), `internal/server/server.go` (24 routes + Deps container), `internal/server/eval.go` (admin token gate pattern), `internal/wire/sse.go` (Hertz SSE writer + D-04 invariant), `cmd/server/main.go` (6-step startup + embed.FS-ready posture), `internal/agent/agent.go` (trust anchors + accCtxKey pattern)
- **Existing research (HIGH)**: `.planning/research/ARCHITECTURE.md` (v0.7.7 baseline 6-layer model), `.planning/research/ARCHITECTURE-v08.md` (v0.8 Harness 5-step pipeline + per-Run accumulator), `.planning/research/PITFALLS.md` (v0.8 10 critical pitfalls), `.planning/research/FEATURES.md` (DATA-01..14 requirements), `.planning/research/STACK.md` (Go + Eino + Hertz stack decisions)
- **Project (HIGH)**: `.planning/PROJECT.md` (v1.0 milestone definition + 7 capability blocks + key decisions), `.planning/REQUIREMENTS.md` (DATA-01..14 + SYSK + AGNT + KNOW + ARAG + GEVL + FEUI + DIST traceability), `.planning/notes/finance-compliance-data-architecture.md` (dual-channel architecture + L1/L2/L3 layering + DATA-09..14 NL2SQL safety)
- **Decisions (HIGH)**: D-04 transport-swap, T-3-AUDIT INSERT-only, T-3-SQLI parameterized placeholders, T-1-08 source-URL whitelist, D8-04 legacy-interrupt ban, D14 Zustand no-messages, D18 WHERE session_id partition, SC-6 frozen-pin reproducibility, P12-L2 MaxIteration:10 hardcoded, P12-L4 plan_todos separate table, P12-L9 loop detect window=3, P13-L1 manual errgroup NOT adk.NewParallelAgent, P13-L2 tier-closure empty-intersection load-time error, P13-L4 sub-agent writes=[], P13-L6 audit RunPath substrate, P13-L8 orchestrator recursion guard, P13-L9 shared ChunkAccumulator, FAN-01..04 invariants
- **Unverified (LOW-MEDIUM)**: goldmark custom-placeholder extension API specifics (RF-6 — Phase 16 spike); robfig/cron/v3 latest API surface (RF-7 — Phase 17 spike); KPI registry delete semantics (P11 mitigation needs Phase 14 spike)

---

*Last updated: 2026-08-21 — v1.0 Business Data Support milestone research. 7 capability blocks mapped to existing packages; 48 files touched (33 NEW + 15 MODIFIED); 2 new Go deps; 5 phases (14-18) sequentially ordered with 2 parallel-safe windows.*
