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
| v1.6 (SPARQL Schema Detection + Entity SHACL Shapes) | 45-46 | ✅ SHIPPED 2026-10-08 |
| Backlog | 999.x | Backlog |

---

## ✅ Shipped Milestones (v0.7.7 – v1.6)

- **v0.7.7** Phases 1-7 — spike / compliance slice / agentic retrieval / streaming UI / skill platform / eval dashboard / OSS release
- **v0.8** Phases 8-13 — deps upgrade / history injection / multi-turn memory / write tools + approval gate / planning layer / sub-agent fan-out
- **v1.0** Phases 14-20 — CGO trust floor / KPI aggregation / dashboard / reports / scheduler / admin
- **v1.1** Phases 21-25 — 外部授权 + 审计链 + Eval CI + OSS
- **v1.2** Phases 26-30 — KBM / DOC / GRF / FBK / TKT (knowledge ops + feedback loop)
- **v1.3** Phases 31-37 — code quality remediation (23/26 REQ closed; trust semantics preserved)
- **v1.4** Phases 38-41 — Ontology Layer (TBox + KG store + ECL pipeline + SHACL governance + MCP/SPARQL/Hybrid platform). Phase detail records preserved below.
- **v1.5** Phases 42-44 — Outstanding Closure (42-03 operator runbook executed 2026-09-30/10-01: 8 PASS + L8 PARTIAL + L4/L7 env-SKIP; live 抽取 F1 0.39→0.87; Phase 44 SPARQL test fix 10/10 green, D-TIER-03 restored). Full details: `.planning/milestones/v1.5-ROADMAP.md`
- **v1.6** Phases 45-46 — SPARQL Schema Detection (D-SPARQL-05 β 语义：schemaSubjects ∩ 13 谓词 allowlist，skill `hasReadAccess` admin 可见，D-TIER-03 fail-closed 保持) + Entity SHACL Shapes (entities.ttl + severity 数据驱动 + boot/rollback env 列表化 + quarantine 分流 + governance 三层呈现；7 plans 全绿，verify-phase-45/46.sh 双门禁 exit 0，milestone audit 6/6 需求 + 11/11 wiring)。Full details: `.planning/milestones/v1.6-ROADMAP.md`

<details>
<summary>v1.4 phase records (38-41) — archived reference</summary>

### Phase 38: Foundation — TBox + Memory-map KG + Auto Schema Lift ✅ (2026-09-21)

6/6 plans. `internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab. Hermetic gate 13/13.

### Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction ✅ (2026-09-22)

9/9 plans. Section-aware chunker + LLM 抽取 + ABox 双写 KG/Milvus + /admin/pipeline tab. Golden F1 tables 0.9846 / related 1.0.

### Phase 40: Data Governance — SHACL + Provenance + TBox Versioning ✅ (2026-09-23)

10/10 plans. **D-SHACL-02: violations never block boot**（boot SHACL → quarantine 分流 → report cache → `/admin/governance` 三视图）; QueryScoped tier 隔离 (D-TIER-01/03/06/07); provenance 链; TBox changelog/rollback. Hermetic gate 17/17. **This is the quarantine flow Phase 46 wired entity shapes into.**

### Phase 41: Platform Layer — MCP Server + SPARQL Endpoint + Hybrid Retrieval ✅ (2026-09-23)

6/6 plans. MCP stdio (5 tools) + `POST /api/sparql` (X-Skill-Name tier isolation) + Hybrid Retrieval (KG 1-2 hop → Milvus → BGE-rerank) + /admin/platform 三视图.

</details>

---

## 📋 Next Milestone

（未定义 —— 运行 `/gsd-new-milestone` 开始下一个里程碑：questioning → research → requirements → roadmap）

v1.6 遗留 tech debt（见 `.planning/v1.6-MILESTONE-AUDIT.md`）已 ack 至 backlog：
D-45-DEF-01/02/03、D-46-DEF-01/02（TestDeriveEdgesDeterministic flaky 为最佳 quick task 候选）、rollback 报告缓存刷新缺口（audit WARNING-1）、owl:Class 声明 quirk（WARNING-2）、BL-01..04 Traceability 补录。

---

*Last updated: 2026-10-08 — v1.6 milestone archived（Phases 45-46 shipped；audit tech_debt 无 blocker）*
