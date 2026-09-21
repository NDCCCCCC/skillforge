---
gsd_state_version: 1.0
milestone: v1.4
milestone_name: Ontology Layer (本体层)
status: "Phase 39 UI-SPEC approved (6/6); next /gsd:plan-phase 39"
last_updated: "2026-09-21T14:35:48.158Z"
last_activity: 2026-09-21 — Phase 39 UI-SPEC approved by gsd-ui-checker (d6abe05); discuss-phase 6 decisions locked earlier same day
progress:
  total_phases: 11
  completed_phases: 1
  total_plans: 6
  completed_plans: 6
  percent: 9
---

# State — SkillForge

## Current Position

Phase: 39 (UI-SPEC approved, awaiting plan)
Plan: —
Status: Phase 39 UI-SPEC approved (6/6); next /gsd:plan-phase 39
Last activity: 2026-09-21 — Phase 39 UI-SPEC approved by gsd-ui-checker (d6abe05); discuss-phase 6 decisions locked earlier same day

## v1.4 Phase Map

| Phase | Domain | Key Deliverables | UI-SPEC Target |
|-------|--------|------------------|----------------|
| 38 | Foundation (TBox + KG store + auto lift) | internal/ontology/ 目录；TBox emitter (server boot); memory-map KG store + JSON snapshot; auto schema lift (LLM 提议 → human approve) | `/admin/ontology` (TBox 类层次 + KG 三元组浏览器) |
| 39 | ECL Pipeline (section-aware chunker + entity extraction) | 替换 internal/knowledge/indexer.go byte-cap; LLM 抽 etl_* tables + 关系; ABox 自动写入 + 双向 KG↔Milvus 同步 | `/admin/pipeline` (ECL Pipeline 实时状态) |
| 40 | Data Governance (SHACL + Provenance) | SHACL shapes 启动校验; Provenance 链 (source_path); TBox 版本号 + 演进日志 | `/admin/governance` (校验报告 + provenance 图 + 版本日志) |
| 41 | Platform Layer (MCP + SPARQL + Hybrid Retrieval) | MCP server 暴露 KG; SPARQL-like REST /api/sparql; Hybrid Retrieval 1-2 跳 expansion | `/admin/platform` (SPARQL Playground + MCP Inspector + Hybrid 调试) |

## v1.4 Standards

- **UI consistency hard constraint:** 每个 web 涉及 phase 必须先产 UI-SPEC.md（约定 + 信息架构 + 关键交互 + 验收标准），再进入实现
- **UI 沿用** shadcn/Tailwind v4 + Vercel AI SDK v5 + `/admin/*` 路由约定
- **零硬编码**：ontology IRI 命名空间、SHACL shapes 路径必须可配置
- **Go 最佳实践**：无 cgo；handler ≤ 200 行；不允许静默吞错
- **三条护城河不可动摇**：Apache-2.0 单 binary + Skill 驱动 + Tier 信任隔离
- **全 SDLC**：每个 phase 配 verify-phase-NN.sh 验证脚本

## v1.4 Source of Truth

- 005 系列 5 个 spike 全 VALIDATED (`.planning/spikes/005a` 到 `005e`)
- Spike 005b TBox emitter 215 triples 已可机器生成
- Spike 005c section-aware chunker 解决 sql-guides 切分痛点
- Spike 005d-b memory-map KG 比 Kuzu 快 9× 轻 1500×
- Spike 005e hybrid retrieval: KG 门控 + Milvus 重排

## Deferred Items (carried from v1.3)

| Category | Item | Status |
|----------|------|--------|
| operator_gate | BGE-M3 endpoint restore + compliance replay re-sign | Deferred |
| operator_gate | Docker live build+run verification | Deferred |
| operator_gate | Phase 18 live E2E | Deferred |
| env_issue | gcc linking (MSYS2 emutls ABI) | Deferred |

## v1.3 Summary (closed 2026-09-09)

**Phases:** 31-37 (7 phases, 26 requirements)
**Audit-driven:** 18 critical + 27 warning + 14 info findings
**Trust semantics preserved:** KNOW-02 fail-closed, D-09 拒答构造, T-3-AUDIT INSERT-only, D11-C3 audit-first, D14 wire 字节不变, HITL-07 3-way frozen_args
**Key fixes:** REALBUG-01/02/03, PERF-01, SRP-01..04, HITL-01..03, REPEATHTTP-01..03, REPEATFE-01..03, SILENTAUDIT-01, STORE-01/02, ROBUST-01..05, CONCUR-01

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema.

**Demo Skill:** 企业合规 / 制度审查

**Audience:** Open-source personal brand.

**Stack (locked):** Go + Eino v0.9.14 + Hertz SSE + Milvus + BGE-M3 + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn/Tailwind v4

**License:** Apache-2.0

## Accumulated Context

### Roadmap Evolution

- 2026-09-09: **v1.3 milestone closed** — code quality remediation from project-audit-2026-09-09.md. Phase 31-37 derived from 26 REQ-IDs. 23/26 REQ closed (88%); 4 项 tech debt.
- 2026-09-21: **v1.4 milestone started** — Ontology Layer based on 005 series (5 spikes all VALIDATED). Phase 38-41 derived from 4 capabilities (Foundation / ECL / Governance / Platform). UI 一致性 hard constraint: 每 web phase 先产 UI-SPEC.md.
- 2026-09-21: **Phase 38 COMPLETE** — Foundation (TBox + memory-map KG + induction). 6 plans / 6 commits (29ad95b..1bd93cc); hermetic gate 13/13; ONT-01..09 verified; `internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab. Notables: triple baseline 176 (源演化, spike 快照 215); rollback UX → Phase 40 (D-TBox-03); approve 闭环含 v{N+1}.ttl 归档 + 5s debounce snapshot.

---

*Last updated: 2026-09-21 — Phase 38 complete (1/4 v1.4 phases, 6/6 plans)*
