# Phase 38 Discussion Log

**Phase:** 38 — Foundation: TBox + Memory-map KG + Auto Schema Lift
**Date:** 2026-09-21
**Mode:** discuss (default)
**Outcome:** 4 / 4 gray areas discussed and decided; CONTEXT.md written

---

## Prior Context Loaded

| Source | Key takeaways |
|--------|--------------|
| `PROJECT.md` v1.4 milestone scope | 4 phases 38-41; preserve Apache-2.0 single binary + Skill-driven + Tier trust isolation moats |
| `REQUIREMENTS.md` ONT-01..09 | 9 requirements: TBox emit / KG store / LLM schema induction / versioned / IRI env-config / no-cgo |
| `ROADMAP.md` Phase 38 SC-1~8 | Server boot generates 215 triples; KG < 1ms query; admin UI `/admin/ontology`; verify-phase-38.sh |
| `STATE.md` v1.4 phase map | Phase 38 first; depends on nothing |
| `SKILL_SCHEMA.md` | 7-field SKILL.md frontmatter (description / agent / reads / writes / steps / guardrails / temperature) |
| `spikes/005a/README.md` | goRDFlib v0.1.0 API validated; MustURIRef / URIRef / Literal.Value gotchas |
| `spikes/005b/README.md` | 215 triples emitted losslessly; 6 Classes / 6 ObjectProperty / 8 DatatypeProperty; CJK IRI verbatim |
| `spikes/005b/main.go` | TBox emitter reference implementation (yaml.v3 + string concat + goRDFlib) |
| `spikes/005b/skillforge_tbox.ttl` | 12.5 KB reference output |
| `spikes/005d-b/README.md` | 215 triples load 7.71ms / query 0.00ms / mem 43 KB; pure Go; no cgo |
| `spikes/005d-b/main.go` | KG store reference implementation (Insert / Query / Adjacent1 / Adjacent2 / Snapshot / LoadSnapshot) |
| `web/src/components/admin/AdminDashboard.tsx` | 7 existing SubrouteTab slots; 8th slot for "Schema Proposals" |
| `internal/server/server.go` | Existing `/api/admin/*` route registration style; admin token gate |
| `web/src/App.tsx` | AdminDashboard mount at `/admin` + `?tab=` query param routing |

---

## Gray Areas Discussed

### 1. LLM schema induction 人工审批 UI 形态 → Drawer 侧滑 + diff preview

**Options presented:**
- Drawer 侧滑 + diff preview (chosen)
- Modal 一次性 approve/reject
- 双 tab 对比 (Proposals vs Current TBox)
- 双面板同时呈现 (左提议队列 + 右 TBox)

**Decision:** D-UI-01 — Drawer 侧滑 + diff preview
- AdminDashboard 第 8 个 tab「Schema Proposals」
- 点击行项 → 右侧 Drawer (复用 shadcn Sheet)
- Drawer 三段式: diff preview (新增 triples 高亮) + reasoning (LLM 解释) + Approve/Reject 按钮
- 与现有 `/admin/*` 风格一致

**Rationale:** diff 可见降低 approve 错误率; 复用 shadcn Sheet 减少新组件; Drawer 不打断列表浏览流。

---

### 2. TBox 文件落盘 + 版本归档 → 版本号归档 (推荐)

**Options presented:**
- 仅内存 TTL (不写盘)
- 覆盖重生成 (无版本)
- 版本号归档 `skillforge_tbox.v{N}.ttl` + 软链 `skillforge_tbox.ttl` → 最新 (chosen)
- Git tracked TTL

**Decision:** D-TBox-01..04 — 版本号归档
- 每次 boot 重生成 + induction approve → `v{N+1}.ttl` + 软链更新
- 版本号持久化在 `{data_dir}/tbox_version.txt`
- Phase 40 rollback 直接读旧版本文件

**Rationale:** Phase 40 rollback 依赖此决策; 字节级 lossless 由 goRDFlib 保证; 每版 12.5 KB 磁盘开销可忽略。

---

### 3. KG snapshot 触发时机 → Boot 写 + debounce 5s (推荐)

**Options presented:**
- Boot 写 + Insert 即时写
- Boot 写 + debounce 5s (chosen)
- Admin 手动触发 + Boot 写

**Decision:** D-KG-01..04 — debounce 5s
- server boot 完成 KG 装载后立即写一次
- Insert/Update/Delete 进 debounce 队列, 5s 静默后落盘
- Phase 39 ECL Pipeline 高频 Insert 自动合并

**Rationale:** Phase 38 静态 + Phase 39 高频场景的最优平衡; 5s debounce 在 server crash 场景下最多丢失 5s Insert 数据 (TBox + 人工 approve 场景下几乎为 0)。

---

### 4. induction 命名 + 语义上限 → 自然语言 + 自动 IRI (推荐)

**Options presented:**
- 仅校名 + 全名 IRI
- 自然语言 + 自动 PascalCase IRI (chosen)
- OWL DL 推理验证

**Decision:** D-Induct-01..06 — 自然语言 + 自动 IRI
- LLM 输出 `{kind, description_zh, description_en, target_skill?, source_chunk_id?}`
- 后端从 description_en 生成 PascalCase 本地名 (去空格、首大写、去重追加 N)
- 强制 rdfs:label 双语 (@zh + @en)
- IRI = `{SKILLFORGE_ONTOLOGY_NS}{PascalCaseLocalName}`
- 不引入 OWL DL 推理 (goRDFlib 不含推理器)
- admin 可在 Drawer 内改本地名

**Rationale:** 出错成本低 (admin 可改名); 与 goRDFlib 纯解析库特性匹配; 不破坏单 binary 护城河。

---

## Implementation Plan Hints (for planner)

| Plan | Scope | Depends on |
|------|-------|------------|
| 38-01: internal/ontology/emitter | TBox emitter (复用 005b main.go), boot-time TTL 生成, version 归档 | — |
| 38-02: internal/ontology/store | KG store (复用 005d-b main.go), debounce 落盘, LoadSnapshot | — |
| 38-03: internal/ontology/induction | LLM 提议生成 (PascalCase + rdfs:label 双语), proposal 持久化 | 38-02 |
| 38-04: internal/server/admin_ontology.go | 8 个 API 路由 (D-Route-01), admin token gate | 38-01, 38-02, 38-03 |
| 38-05: web AdminDashboard 第 8 个 tab | Schema Proposals 列表 + Drawer + KG query panel | 38-04 |
| 38-06: verify-phase-38.sh | 8 SC 全自动验证 | 38-01..05 |

注: 实际 plan 拆分由 planner (gsd-plan-phase) 决定。

---

## Deferred Ideas (see CONTEXT.md `<deferred>`)

- Phase 39 ECL Pipeline
- Phase 40 SHACL + Provenance + TBox rollback UX
- Phase 41 MCP + SPARQL + Hybrid Retrieval
- OWL 2 RL/DL reasoning (out of scope, breaks single-binary moat)
- Auto induction (no human approval) — out of scope
- Multi-instance federation — out of scope

---

## Session End

- CONTEXT.md written: `.planning/phases/38-foundation-tbox-kg/38-CONTEXT.md`
- DISCUSSION-LOG.md written: `.planning/phases/38-foundation-tbox-kg/38-DISCUSSION-LOG.md`
- State update: pending (Phase 38 context gathered; next: gsd-plan-phase 38)
</content>
</invoke>