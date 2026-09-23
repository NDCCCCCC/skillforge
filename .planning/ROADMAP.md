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

---

## Phase 34: HITL Handler Template Extraction

**Goal**: Eliminate copy-paste duplication across four HITL handlers by extracting shared base class and audit builder

**Depends on**: Phase 33

**Requirements**: HITL-01, HITL-02, HITL-03

---

## Phase 35: HTTP Layer Deduplication

**Goal**: Remove copy-paste HTTP error structs and admin-request middleware from backend handlers

**Depends on**: Phase 34

**Requirements**: REPEATHTTP-01, REPEATHTTP-02, REPEATHTTP-03

---

## Phase 36: FE Deduplication + Silent Audit Errors

**Goal**: Extract shared frontend fetch/error helpers and fix silent audit write failures in backend

**Depends on**: Phase 35

**Requirements**: REPEATFE-01, REPEATFE-02, REPEATFE-03, SILENTAUDIT-01

---

## Phase 37: Store SQL Hygiene + Robustness + Concurrency

**Goal**: Fix SQL parameterization, add missing constraints, strengthen type-safety invariants, and add concurrency guarantees

**Depends on**: Phase 36

**Requirements**: STORE-01, STORE-02, ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-05, CONCUR-01

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

**Plans**: 6/6 complete — 38-01 emitter (29ad95b) / 38-02 store (568b819) / 38-03 induction (e47bb23) / 38-04 admin API (14463e5) / 38-05 web tab (0a29a92) / 38-06 verify gate (1bd93cc). Hermetic gate 13/13 PASS.

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

**Plans**: 9/9 complete — 39-01 store primitives / 39-02 Milvus schema / 39-03 ECL merge / 39-04 extraction / 39-05 failures + manager / 39-06 boot wiring / 39-07 admin API / 39-08 web tab / 39-09 verify gate

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

**Plans**: 10/10 complete — 40-01 SHACL shapes / 40-02 Provenance field / 40-03 ECL provenance / 40-04 QueryScoped / 40-05 TBox changelog / 40-06 Boot SHACL / 40-07a Governance tab shell / 40-07b Three views / 40-08 KGQueryPanel skill selector / 40-09 verify gate. Hermetic gate 17/17 PASS.

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
4. Hybrid Retrieval：query → KG 1-2 跳 subgraph expansion → Milvus rerank；Q1/Q4 类查询召回 ≥ 0.85（绝对门槛）
5. Admin UI 提供：SPARQL Playground (代码编辑器 + 结果面板) + MCP Inspector (tools list + 测试调用) + Hybrid 调试 (KG expansion 可视化 + Milvus 命中列表)
6. `verify-phase-41.sh` 全绿：MCP tools 可调用 + SPARQL endpoint 正常 + Hybrid Q1 召回 ≥ 0.85

**Plans**: 2/6 complete — 41-01 MCP stdio server / 41-02 SPARQL endpoint (bc02242+5e9c332)

**Wave 1**

- [ ] 41-01-PLAN.md — MCP server package (mark3labs/mcp-go, stdio transport, 5 tools: skf_query/skf_skills/skf_chunk_for/skf_schema/skf_neighbors)
- [ ] 41-02-PLAN.md — SPARQL-like REST endpoint (POST /api/sparql, X-Skill-Name header, SparqlQueryScoped shared handler)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 41-03-PLAN.md — Hybrid Retrieval pipeline (KG 1-2 hop expansion → Milvus hybrid → BGE-reranker → top-5)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 41-04-PLAN.md — Platform tab shell (AdminDashboard wiring, store union, 11th SubrouteTab)
- [ ] 41-05-PLAN.md — Three views (SPARQL Playground + MCP Inspector + Hybrid 调试) + platformApi.ts + types.ts

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 41-06-PLAN.md — verify-phase-41.sh hermetic gate (MCP stdio + SPARQL curl + Hybrid recall >= 0.85)

---

### v1.4 Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 38. Foundation (TBox + KG + Auto Lift) | 6/6 | Complete | 2026-09-21 |
| 39. ECL Pipeline | 9/9 | Complete    | 2026-09-22 |
| 40. Data Governance | 10/10 | Complete    | 2026-09-23 |
| 41. Platform Layer (MCP + SPARQL + Hybrid) | 0/6 | Not started | — |

---

*Last updated: 2026-09-23 — Phase 41 planned (6 plans, 4 waves). MCP server via mark3labs/mcp-go stdio transport. SPARQL endpoint POST /api/sparql with X-Skill-Name tier isolation. Hybrid retrieval pipeline KG expansion + Milvus rerank. UI: /admin/platform 11th tab with CodeMirror 6 + three-view switcher.*
