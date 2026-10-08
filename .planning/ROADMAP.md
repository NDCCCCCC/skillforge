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
| v1.4 (Ontology Layer) | 38-41 | ✅ SHIPPED 2026-09-23 |
| v1.5 (Outstanding Closure) | 42-44 | ✅ SHIPPED 2026-10-02 (closure-only) |
| **v1.6 (SPARQL Schema Detection + Entity SHACL Shapes)** | **45-46** | **🚧 IN PROGRESS** |
| Backlog | 999.x | Backlog |

---

## ✅ Shipped Milestones (v0.7.7 – v1.5)

- **v0.7.7** Phases 1-7 — spike / compliance slice / agentic retrieval / streaming UI / skill platform / eval dashboard / OSS release
- **v0.8** Phases 8-13 — deps upgrade / history injection / multi-turn memory / write tools + approval gate / planning layer / sub-agent fan-out
- **v1.0** Phases 14-20 — CGO trust floor / KPI aggregation / dashboard / reports / scheduler / admin
- **v1.1** Phases 21-25 — 外部授权 + 审计链 + Eval CI + OSS
- **v1.2** Phases 26-30 — KBM / DOC / GRF / FBK / TKT (knowledge ops + feedback loop)
- **v1.3** Phases 31-37 — code quality remediation (23/26 REQ closed; trust semantics preserved)
- **v1.4** Phases 38-41 — Ontology Layer (TBox + KG store + ECL pipeline + SHACL governance + MCP/SPARQL/Hybrid platform). Phase detail records preserved below.
- **v1.5** Phases 42-44 — Outstanding Closure (42-03 operator runbook executed 2026-09-30/10-01: 8 PASS + L8 PARTIAL + L4/L7 env-SKIP; live 抽取 F1 0.39→0.87; Phase 44 SPARQL test fix 10/10 green, D-TIER-03 restored). Full details: `.planning/milestones/v1.5-ROADMAP.md`

<details>
<summary>v1.4 phase records (38-41) — archived reference for v1.6 work</summary>

### Phase 38: Foundation — TBox + Memory-map KG + Auto Schema Lift ✅ (2026-09-21)

6/6 plans. `internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab. Hermetic gate 13/13.

### Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction ✅ (2026-09-22)

9/9 plans. Section-aware chunker + LLM 抽取 + ABox 双写 KG/Milvus + /admin/pipeline tab. Golden F1 tables 0.9846 / related 1.0.

### Phase 40: Data Governance — SHACL + Provenance + TBox Versioning ✅ (2026-09-23)

10/10 plans. **D-SHACL-02: violations never block boot**（boot SHACL → quarantine 分流 → report cache → `/admin/governance` 三视图）; QueryScoped tier 隔离 (D-TIER-01/03/06/07); provenance 链; TBox changelog/rollback. Hermetic gate 17/17. **This is the quarantine flow Phase 46 wires entity shapes into.**

### Phase 41: Platform Layer — MCP Server + SPARQL Endpoint + Hybrid Retrieval ✅ (2026-09-23)

6/6 plans. MCP stdio (5 tools) + `POST /api/sparql` (X-Skill-Name tier isolation) + Hybrid Retrieval (KG 1-2 hop → Milvus → BGE-rerank) + /admin/platform 三视图.

</details>

---

## 🚧 v1.6 — SPARQL Schema Detection + Entity SHACL Shapes (In Progress)

**Milestone Goal:** 闭合 v1.5 两项信任/治理积压——D-SPARQL-05（skill `hasReadAccess` 经 subject-based schema detection 在 SPARQL/admin baseline 可见）+ 实体类 SHACL shapes（实体 ABox 纳入启动校验 + 既有 quarantine 流）——同时不削弱 D-TIER-03 fail-closed。

**Standards（继承 v1.4/v1.5）:**

- **零硬编码**：ontology IRI 命名空间、SHACL shapes 路径可配置
- **Go 最佳实践**：无 cgo（单 binary）；handler ≤ 200 行；不允许静默吞错
- **UI 一致性**：涉 web 的 phase 必须先产 UI-SPEC.md（shadcn/Tailwind v4 + `/admin/*` 路由约定）
- **全 SDLC**：每个 phase 配 verify-phase-NN.sh 验证脚本
- **三条护城河不可动摇**：Apache-2.0 单 binary / Skill 驱动 / Tier 信任隔离（D-TIER-03 fail-closed 不削弱）

**Source of truth:** `.planning/phases/44-sparql-test-fix/44-DECISIONS.md`（D-SPARQL-05 defer 记录）+ `44-RESEARCH.md` §5 Option α + Phase 40 SHACL quarantine 既有实现。

**Scope boundary:** STATE.md 积压其余 4 项（resolver 去重 / server lint 清扫 / live 抽取 prompt 微调 / DetailPanel+emitter deferred 测试）不进 v1.6，留 quick task。D-SPARQL-07（comment predicate hardening）默认不在范围，discuss-phase 可再议。

### Phases

- [x] **Phase 45: SPARQL Schema Visibility (D-SPARQL-05)** - Subject-based schema detection：skill 个体与 hasReadAccess triples 在 admin baseline 可见，D-TIER-03 fail-closed 不回退，schema 判定单点化 (completed 2026-10-03)
- [ ] **Phase 46: Entity SHACL Shapes** - 实体类 ABox SHACL shapes 交付（路径可配置）+ 启动校验接入既有 quarantine 流 + /admin/governance 报告呈现实体违规

### Phase 45: SPARQL Schema Visibility (D-SPARQL-05)

**Goal**: Admin/operator 可以在 SPARQL 与 admin baseline 中看到 skill 访问元数据（skill 个体 + `hasReadAccess` triples），而实体 ABox 仍严格 fail-closed——schema 可见性的扩张不打开任何 tier 泄漏面。

**Depends on**: Nothing（v1.6 首 phase；落地 Phase 44 D-SPARQL-05 defer）

**Requirements**: SPARQL-01, SPARQL-02, SPARQL-03

**Success Criteria** (what must be TRUE):

1. Admin baseline（无 tier 过滤的 `QueryScoped`）查询返回 skill 个体（如 `skill_sql`）及其 `hasReadAccess` triples——此前这些 triples 对 admin baseline 不可见（44-RESEARCH §5 Option α：`schemaSubjects` 集合 Insert 时填充，`isSchemaTriple` 按 subject ∈ schemaSubjects 判定）
2. Skill-scoped 查询（X-Skill-Name tier 隔离）仍然排除所有无 `belongsToTier` 的 ABox 实体——Phase 44 既有 fail-closed 回归测试（`TestSparqlQueryScoped_FailClosedNoBelongsToTier`、label/comment passthrough、Phase 40 store tier 套件）在 CGO_ENABLED=1 下全绿，D-TIER-03 无回退
3. Schema 可见性判定单点化：`/api/sparql` 与 `/api/admin`（KG query / governance 数据面）对同一 triple 的 schema/ABox 判定一致——服务端仅存在一处 `isSchemaTriple` 实现，判定修改一次即同时作用于两个端点
4. `verify-phase-45.sh` 全绿：subject-based detection 新增回归测试 + Phase 40/44 既有 SPARQL 套件在 CGO 构建环境（WSL gcc + emutls ABI）通过

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 45-01-PLAN.md — β schema-visibility landing（isSchemaTriple subject∩predicate arm + query-time schemaSubjects derivation + admin 断言 17/10 + store 双断言回归）

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 45-02-PLAN.md — 验证层（emitter conformance anti-drift 门 + SC3 handler 级 parity 测试 + verify-phase-45.sh SC4 门禁）

### Phase 46: Entity SHACL Shapes

**Goal**: 实体类 ABox（险种 Product / 机构 Org / TableSchema 等）在启动时按可配置的 SHACL shapes 接受约束校验：违规实体进入既有 quarantine 流而不阻断 boot（D-SHACL-02 保持），并在 governance 报告中可见。

**Depends on**: Nothing（与 Phase 45 相互独立；按序排第二）

**Requirements**: SHACL-01, SHACL-02, SHACL-03

**Success Criteria** (what must be TRUE):

1. 仓库交付实体类 SHACL shapes 文件（险种 Product / 机构 Org / TableSchema 等类的必填属性 + cardinality 级约束），shapes 加载路径经 env 可配置（延伸既有 `SKILLFORGE_SHACL_SHAPES` 机制，零硬编码路径、零硬编码 IRI 命名空间）
2. 启动校验覆盖实体 shapes：注入一个违规实体（如缺必填属性的 Product）→ 该实体进入既有 quarantine 流、不出现在正常 KG 查询结果中，server 携违规正常 boot（D-SHACL-02 violations never block boot 保持）
3. `/admin/governance` 校验报告可呈现实体类违规，且与 TBox 层违规可区分；涉 UI 改动先产出 UI-SPEC.md
4. `verify-phase-46.sh` 全绿：shapes 加载 + quarantine 分流 + boot 存活 + governance 报告测试

**Plans**: 5 plans / 3 waves

**Wave 1**

- [x] 46-01-PLAN.md — SHACL 引擎机制：severity 政策归位（sourcePath 特例退役）+ shapes 路径列表化（variadic + ShapesPathsFromEnv + Layer 源文件追踪）

**Wave 2** *(并行：零文件重叠)*

- [ ] 46-02-PLAN.md — 分层盖戳 + dangling_edge_count + entities.ttl 交付 + SC2 隔离语义测试组
- [ ] 46-03-PLAN.md — boot/rollback 装配统一 env 列表（induction.go:383 硬编码修复）+ rollback parity 回归

**Wave 3** *(并行：零文件重叠)*

- [ ] 46-04-PLAN.md — governance FE 分层呈现（46-UI-SPEC delta：徽章/区头计数/悬挂边列/组件测试）
- [ ] 46-05-PLAN.md — verify-phase-46.sh 门禁（P1-P4 照 45 模式，anti-vacuous）

---

## Progress

**Execution Order:** 45 → 46

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 45. SPARQL Schema Visibility (D-SPARQL-05) | v1.6 | 2/2 | Complete    | 2026-10-03 |
| 46. Entity SHACL Shapes | v1.6 | 0/5 | Not started | — |

---

*Last updated: 2026-10-04 — Phase 46 planned (5 plans / 3 waves: 46-01 引擎机制, 46-02 实体 shapes+分层 ∥ 46-03 装配, 46-04 FE ∥ 46-05 verify 门禁).*
