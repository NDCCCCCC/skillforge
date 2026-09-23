# SkillForge Roadmap

## Milestones

| Milestone | Phase Range | Status |
|-----------|-------------|--------|
| v0.7.7 (Vertical Slice) | 1-7 | ✅ SHIPPED 2026-08-11 |
| v0.8 (Memory + HITL) | 8-13 | ✅ SHIPPED 2026-09-06 |
| v1.0 (Business Data) | 14-20 | ✅ SHIPPED 2026-09-06 |
| v1.1 (Eval Trust-Loop) | 21-25 | ✅ SHIPPED 2026-09-06 |
| v1.2 (Knowledge Ops) | 26-30 | ✅ SHIPPED 2026-09-08 |
| v1.3 (Code Quality) | 31-37 | ✅ SHIPPED 2026-09-09 |
| **v1.4 (Ontology Layer)** | **38-41** | **🚧 Planned** |
| Backlog | 999.x | Backlog |

---

## Phase 31: FE Real Bugs — Hostname Config + evalStatus Ready

**Goal**: Fix two production-impacting frontend bugs — hardcoded hostname whitelist and stuck loading state

**Depends on**: Nothing (first v1.3 phase)

**Requirements**: REALBUG-01, REALBUG-02

**Success Criteria** (what must be TRUE):

1. `sanitize-url.ts` reads hostname whitelist from a runtime-configurable source (env or admin API), not hardcoded list; production deployment domains work without code changes
2. `useEvalReports.ts` calls `setEvalStatus('ready')` on successful fetch; `EvalDashboard` transitions through loading/error/ready states correctly
3. Existing vitest suite for `sanitize-url.ts` passes with the new configurable whitelist
4. `vi.spy` regression test for `evalStatus` state transitions is green

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [x] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 32: BE Real Bug + Performance — Typed Sentinel + O(n²) Fix

**Goal**: Fix backend production bug (string-substring error matching) and a performance regression (O(n²) string concat in hot path)

**Depends on**: Phase 31

**Requirements**: REALBUG-03, PERF-01

**Success Criteria** (what must be TRUE):

1. `upload.go:isBackendUnavailable` uses `errors.Is(err, syscall.ECONNREFUSED)`, `errors.Is(err, context.DeadlineExceeded)`, `errors.Is(err, context.Canceled)`, and `net.Error.Timeout()` — no `strings.Contains` substring matching on error messages
2. `chat.go:joinStrings` is replaced with `strings.Join(parts, "")`; every LLM response drain path calls the fixed version
3. Unit tests cover all branches of `isBackendUnavailable` (connection refused, deadline exceeded, canceled, timeout, other error)
4. Go vet/benchmark regression confirms no performance degradation on joinStrings path

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 33: SRP — BE Long Function Split

**Goal**: Split four oversized Go handlers to meet SRP constraints (handler ≤ 200 lines, pipeline ≤ 100 lines)

**Depends on**: Phase 32

**Requirements**: SRP-01, SRP-02, SRP-03, SRP-04

**Success Criteria** (what must be TRUE):

1. `pipeline.go:VerdictPipeline` main function ≤ 100 lines; `handleRefusal()`, `handleVerdict()`, `streamWithRetry()` extracted as named methods; Step 0..12 comments preserved
2. `chat.go:chatHandler` main function ≤ 80 lines; `runDataChannelPath`, `runAgentPath`, `runRegistryRouterPath`, `runVerdictPipelinePath` extracted; 9 DEBUG log blocks merged into named helpers
3. `upload.go:uploadHandler` main function ≤ 80 lines; `onFile` closure split into `verifyMagic`, `dedupCheck`, `writeIndex`, `auditHook` named functions
4. All four files compile with `go vet` zero warnings; no new cyclomatic complexity introduced

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 34: HITL Handler Template Extraction

**Goal**: Eliminate copy-paste duplication across four HITL handlers by extracting shared base class and audit builder

**Depends on**: Phase 33

**Requirements**: HITL-01, HITL-02, HITL-03

**Success Criteria** (what must be TRUE):

1. New `baseHITLTool` struct is embedded in all four HITL handlers (`memory_facts`, `review_report`, `plan_approve`, `report_publish`); `auditOrDefault()` lives in the base class
2. `buildPendingAuditAndInterrupt(ctx, info, args, audit)` is a shared function callable by all four handlers; the 4-step sequence (GetToolCallID → ApprovalInfo → InsertPendingApproval → StatefulInterrupt) is no longer duplicated
3. `noopAuditWriter` is defined in `hitl/audit_writer.go` and imported by `hitl_memory_facts.go`; no duplicate `noopAuditWriter` methods remain
4. All four files continue to pass existing unit tests after refactoring

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 35: HTTP Layer Deduplication

**Goal**: Remove copy-paste HTTP error structs and admin-request middleware from backend handlers

**Depends on**: Phase 34

**Requirements**: REPEATHTTP-01, REPEATHTTP-02, REPEATHTTP-03

**Success Criteria** (what must be TRUE):

1. `graphResponseError` struct is defined once in `internal/server/server_response.go`; all six+ duplicated definitions (`knowledge.go`, `feedback.go`, `upload.go`, `tickets.go`, `approval_api.go`, `plan_api.go`) are replaced by imports
2. Admin-gated handlers use a shared `requireAdmin(c) error` middleware function; the 403 response + audit-write logic appears in exactly one place
3. `sessionIDRegex` is exported from `server.go` and imported by all eight+ files that currently define it locally; `handlers_session_history.go` no longer re-exports a private copy
4. All affected routes continue to return identical JSON error shapes after refactoring

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 36: FE Deduplication + Silent Audit Errors

**Goal**: Extract shared frontend fetch/error helpers and fix silent audit write failures in backend

**Depends on**: Phase 35

**Requirements**: REPEATFE-01, REPEATFE-02, REPEATFE-03, SILENTAUDIT-01

**Success Criteria** (what must be TRUE):

1. `lib/api.ts` exports `apiFetch<T>(url, opts)` used by `FeedbackRow.postJson`, `useEvalReports.fetchOnce`, `useEvalFrozen.refetch`, and other fetch sites — no independent fetch+JSON+error implementations remain
2. `lib/errors.ts` exports `extractReason` used by both `FeedbackRow` and `useEvalReports` — error text extraction logic appears in exactly one place
3. `useEvalReports.fetchOnce` is wrapped in `useCallback` and accepts `fetchImpl` as an injected parameter; tests use `vi.fn()` injection instead of global fetch spy
4. Four backend audit write sites that silently swallow errors with `_ = ...` now log at WARN level with session/audit_id/context; the "audit hiccup must not fail Run" invariant is preserved (log does not throw)

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Phase 37: Store SQL Hygiene + Robustness + Concurrency

**Goal**: Fix SQL parameterization, add missing constraints, strengthen type-safety invariants, and add concurrency guarantees

**Depends on**: Phase 36

**Requirements**: STORE-01, STORE-02, ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-05, CONCUR-01

**Success Criteria** (what must be TRUE):

1. All five `INSERT INTO audit` statements share a single parameterized SQL statement with `action=?` placeholder; adding a new audit action column requires changes in exactly one place
2. `putKPICallSQL` includes `ON CONFLICT(func, args_json, ts) DO NOTHING`; re-calling the same KPI with identical args is idempotent
3. All 11 LRU type assertions (`data/preview.go:121,141,208`, `duckdb/attach_lru.go:124`, `knowledge/graph_cache.go:122,150`, `runtime/fanout_runtime.go:266`) use the `, ok` idiom or have explicit comments documenting why the invariant holds
4. `SessionStore.Save` purge logic runs outside the mutex critical section (async or TTL-heap)
5. `hitl_report_publish.go` frozen_args uses raw `argsBytes` string, not `json.Marshal(parsed)` — sha256 hash of frozen_args matches LLM's original input byte-for-byte
6. `hitl_review_report.go` performs three-way frozen args comparison (preview vs. audit row vs. executed) or documents why two-way is sufficient
7. `RealChatTransport.ts` uses `DecisionForcedStop` and other `Decision*` constants from `wire-types.ts` instead of string literals
8. `hitlHandlers` registry uses `sync.Once` during `InjectDeps` to guarantee single initialization; documentation states the process-wide initialization invariant

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 31. FE Real Bugs | 0/2 | Not started | — |
| 32. BE Real Bug + PERF | 0/2 | Not started | — |
| 33. SRP Split | 0/4 | Not started | — |
| 34. HITL Template | 0/3 | Not started | — |
| 35. HTTP Deduplication | 0/3 | Not started | — |
| 36. FE Deduplication + SILENTAUDIT | 0/4 | Not started | — |
| 37. STORE + Robustness + CONCUR | 0/8 | Not started | — |

---

## v1.4 — Ontology Layer (本体层)

**Goal:** 在 SkillForge 引入 OWL/RDF 本体层，把"Skill 驱动的 RAG"升级为
"Skill 驱动的本体操作系统"——保留 Apache-2.0 单 binary + Skill 驱动 +
Tier 隔离三条护城河。

**Hard constraint:** 每个 phase 涉及 web 时，必须先产出 UI-SPEC.md，
再进实现；UI 沿用 shadcn/Tailwind v4 + Vercel AI SDK v5 现有体系，与
`/admin/*` 四视图路由约定保持一致。

**Source of truth:** 005 系列 5 个 spike 全 VALIDATED（`.planning/spikes/005a` 到 `005e`）

---

### Phase 38: Foundation — TBox + Memory-map KG + Auto Schema Lift ✅ (2026-09-21)

**Goal:** Server boot 时自动生成 RDFS/OWL TBox；纯 Go 内存 map KG store；LLM 提议新 Class/Property 的人工审批闭环。

**Depends on:** Nothing (first v1.4 phase)

**Requirements:** ONT-01, ONT-02, ONT-03, ONT-04, ONT-05, ONT-06, ONT-07, ONT-08, ONT-09

**UI-SPEC Target:** `/admin/ontology` — TBox 类层次浏览器 + KG 三元组查询面板

**Success Criteria** (what must be TRUE):

1. Server boot 生成 `skillforge_tbox.ttl` (215 triples) + `skillforge_kg.json` snapshot；reload 字节级一致
2. KG query API 在 215 triples 上 < 1ms；memory 估计 < 50KB
3. `internal/ontology/` 目录无 cgo 依赖；`go build` 单 binary 通过
4. Admin UI 列出 8 Skills + 5 Tiers + 7 SubCategories（来自 TBox），可点击查看 Class 详情
5. KG 三元组查询面板支持 `?x a skf:Skill` 模式查询，结果可导出 JSON
6. LLM schema induction：Claude 提议新 Class/Property 写入 `/admin/ontology/proposals`，需人工点击 approve 才进 TBox
7. TBox 版本号自增 1；rollback 到上一版本可一键还原
8. `verify-phase-38.sh` 全绿：单 binary 启动 + KG query < 1ms + reload lossless

**Plans**: 6/6 complete — 38-01 emitter (29ad95b) / 38-02 store (568b819) / 38-03 induction (e47bb23) / 38-04 admin API (14463e5) / 38-05 web tab (0a29a92) / 38-06 verify gate (1bd93cc). Hermetic gate 13/13 PASS. Verification: `38-VERIFICATION.md` (status: passed; 4 recorded non-blocking deviations — triple baseline 176 vs spike 215, rollback UX → Phase 40 per D-TBox-03, Windows cgo env, live-LLM manual gate).

---

### Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction

**Goal:** 替换 `internal/knowledge/indexer.go` byte-cap chunker；LLM 自动从 sql-guides markdown 抽取 etl_* 实体 + 关系；ABox 自动写入 KG + Milvus 双向同步。

**Depends on:** Phase 38

**Requirements:** ONT-10, ONT-11, ONT-12, ONT-13

**UI-SPEC Target:** `/admin/pipeline` — ECL Pipeline 实时状态（chunker / extraction / load 进度 + 失败重试）

**Success Criteria** (what must be TRUE):

1. Section-aware chunker 处理 185 sql-guides `__secNN` 片段 → 26 完整 notes；100% 边界识别；note 001 包含 3/3 corpus SQL 目标表
2. LLM 实体抽取从 sql-guides 自动产出 etl_* table references + 关系（≥ 80% precision@recall vs 005c 手工标注）
3. 新 ABox triples 自动写入 memory-map KG + Milvus 索引同步
4. Admin UI 实时显示：chunker 当前文件 / extraction 进度 / load 队列长度 / 失败重试按钮
5. ECL 失败 → 自动 retry 3 次 → 仍失败入 dead-letter 队列，可手动触发
6. `verify-phase-39.sh` 全绿：100% chunker boundary + entity extraction F1 ≥ 0.8

**Plans**: 9 plans / 6 waves (planned 2026-09-21)

Plans:
**Wave 1**

- [x] 39-01-PLAN.md — KG store DeleteSubject/InsertUnique primitives + TBox emitter 6→8 classes (D-TBox2-01)
- [x] 39-02-PLAN.md — Milvus schema +4 native fields, DeleteByExpr marker purge, collection-wide ID watermark
- [x] 39-03-PLAN.md — ECL merge package: 005c walk+merge port, ChunkMeta contract, regex stub, corpus boundary tests

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 39-04-PLAN.md — Extraction layer: Extractor interface (LLM+stub), F1 math, ABox note-level replace

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 39-05-PLAN.md — ecl_failures ledger + section→Document chunking + Manager stage orchestrator (retry×3 → dead-letter)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 39-06-PLAN.md — Indexer routing branch + boot: TBox upgrade trigger, snapshot-first KG restore, ECL manager wiring
- [x] 39-07-PLAN.md — Admin pipeline API: run/status/retry routes, 403-before-503 gate, handler test suite

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 39-08-PLAN.md — /admin/pipeline 9th tab per 39-UI-SPEC (Run ECL, progress, failures table, 2s/10s polling)

**Wave 6** *(blocked on Wave 5 completion)*

- [x] 39-09-PLAN.md — Golden set (D-GT-01 human review) + F1 harness + verify-phase-39.sh hermetic gate

---

### Phase 40: Data Governance — SHACL + Provenance + TBox Versioning

**Goal:** 启动时 SHACL 校验 ABox 一致性；每条 statement 带 source provenance；TBox 版本演进日志。

**Depends on:** Phase 39

**Requirements:** ONT-14, ONT-15, ONT-16, ONT-17

**UI-SPEC Target:** `/admin/governance` — SHACL 校验报告 + Provenance 链路图 + TBox 版本演进

**Success Criteria** (what must be TRUE):

1. Server boot 运行 SHACL validator 校验 ABox，violation 列表清晰（source file + entity URI + 约束）
2. 每条 ABox triple 带 `skf:provenance` → source file path + line range + extraction timestamp
3. TBox 版本号 + 演进日志：每次 schema induction 写入一条记录（who / when / what / why）
4. TBox rollback 到上一版本：snapshot → 验证 → 切换 → KG rebuild in < 5s
5. KG query endpoint 自动追加 `belongsToTier` 过滤（基于 caller skill 的 `reads` 声明）
6. Admin UI 展示：SHACL 报告（绿/黄/红）+ provenance graph (Cytoscape.js) + TBox 版本 timeline
7. `verify-phase-40.sh` 全绿：SHACL violation detection + provenance coverage 100% + rollback < 5s

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [x] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [x] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment (9e2a799 / fd740e5)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [x] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api) (496bfbd / e3554f3)
- [x] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep (358cfec / 5044031 / d84557f / 2a0a154)
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

### Phase 41: Platform Layer — MCP Server + SPARQL Endpoint + Hybrid Retrieval

**Goal:** MCP server 暴露 KG 给外部 agent；SPARQL-like REST endpoint；Hybrid Retrieval 1-2 跳 subgraph expansion。

**Depends on:** Phase 40

**Requirements:** PLAT-ONT-18, PLAT-ONT-19, PLAT-ONT-20

**UI-SPEC Target:** `/admin/platform` — SPARQL Playground + MCP Inspector + Hybrid Retrieval 调试面板

**Success Criteria** (what must be TRUE):

1. MCP server 启动并暴露 Tools: `skf_query(pattern)` / `skf_skills()` / `skf_chunk_for(entity)` — 外部 agent 可直接 query
2. SPARQL-like REST endpoint `POST /api/sparql` 接受 pattern match queries，response 为 JSON triples
3. KG query 自动 tier 隔离：MCP + SPARQL 强制按 caller 上下文过滤（不可绕过）
4. Hybrid Retrieval：query → KG 1-2 跳 subgraph expansion → Milvus rerank；Q1/Q4 类查询召回 ≥ 0.85（vs v1.3 0.70）
5. Admin UI 提供：SPARQL Playground (代码编辑器 + 结果面板) + MCP Inspector (tools list + 测试调用) + Hybrid 调试 (KG expansion 可视化 + Milvus 命中列表)
6. `verify-phase-41.sh` 全绿：MCP tools 可调用 + SPARQL endpoint 正常 + Hybrid Q1 召回 ≥ 0.85

**Plans**: 10 plans / 4 waves (planned 2026-09-22)

**Wave 1**

- [ ] 40-01-PLAN.md — SHACL shapes TTL + pure-Go validator (7 constraint types)
- [x] 40-02-PLAN.md — Triple.Provenance field + backward-compatible JSON snapshot
- [ ] 40-03-PLAN.md — ECL ABox provenance + belongsToTier attachment

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 40-04-PLAN.md — QueryScoped tier filtering (schema exempt, fail-closed, entity-level)
- [x] 40-05-PLAN.md — TBox changelog JSONL + RollbackTBox + admin routes (f753c40 / 8fdcdb5)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 40-06-PLAN.md — Boot SHACL + quarantine + governance admin API (shacl/provenance) (584b58b / 156a23e)
- [ ] 40-07a-PLAN.md — Governance tab shell (tab registration + HealthBanner + types/api)
- [ ] 40-07b-PLAN.md — Three views (SHACL report + provenance graph + timeline) + cytoscape dep
- [x] 40-08-PLAN.md — KGQueryPanel skill selector + 来源 column (5a7520f / c2c1425)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 40-09-PLAN.md — verify-phase-40.sh hermetic gate + unit tests

---

### v1.4 Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 38. Foundation (TBox + KG + Auto Lift) | 6/6 | Complete | 2026-09-21 |
| 39. ECL Pipeline | 9/9 | Complete    | 2026-09-22 |
| 40. Data Governance | 1/10 | In Progress|  |
| 41. Platform Layer (MCP + SPARQL + Hybrid) | 0/? | Not started | — |

---

*Last updated: 2026-09-21 — v1.4 roadmap created (4 phases 38-41, 17 ONT + 3 PLAT-ONT requirements mapped). Source: 005 series 5 spikes all VALIDATED. UI consistency hard constraint: each web-touched phase produces UI-SPEC.md before implementation. Prior: v1.3 shipped 2026-09-09 (phases 31-37, 26 requirements).*
