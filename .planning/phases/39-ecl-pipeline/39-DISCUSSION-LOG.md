# Phase 39 Discussion Log

**Phase:** 39 — ECL Pipeline: Section-aware Chunker + LLM Entity Extraction
**Date:** 2026-09-21
**Mode:** discuss (default)
**Outcome:** 4/4 gray areas discussed (6 questions); CONTEXT.md written

---

## Prior Context Loaded

| Source | Key takeaways |
|--------|--------------|
| ROADMAP Phase 39 | SC-1..6; ONT-10..13; UI-SPEC 目标 /admin/pipeline |
| PROJECT.md v1.4 | UI-SPEC hard constraint (web phase 先产 UI-SPEC.md); 三条护城河 |
| REQUIREMENTS.md ONT-10..13 | chunker 替换 / 100% boundary / LLM 抽取 / ABox 双写 |
| STATE.md | Phase 38 COMPLETE (6/6 plans, hermetic 13/13); Phase 39 next |
| Phase 38 CONTEXT.md | D-TBox 版本归档 / D-KG debounce / admin handler 模式 — 全程沿用 |
| Phase 38 VERIFICATION.md | 衔接基线: store.Insert + SetOnChange + emitter classes 列表 |
| spike 005c README + main.go + chunk_metadata.json | 185→26 算法 + ChunkMeta 契约 + 3 表锚点 + sec01 病理说明 |
| internal/knowledge/indexer.go | Index() 签名 + maxDocBytes=4000 跳过 + replace-by-tier purge 契约 |
| internal/retriever/milvus/seed.go 71-80 | 现有 7 字段 schema (无 source_idx/section) |
| cmd/server/main.go 1133-1136 | SKILLFORGE_MILVUS_RECREATE 一次性重建门已存在 |
| memory: env-intranet-service-flakiness | 内网 LLM/BGE 间歇断 — 决定 boot 不自动跑 ECL |
| memory: user-role-it-data-support | sql-guides (ETL/取数) 是核心业务方向 |

---

## Gray Areas Discussed

### 1. Chunker 落位策略 → 独立 ECL 包 + 路由 (推荐)

**Options:** 独立 internal/knowledge/ecl/ 包 + indexer.go 路由分支 (chosen) / indexer.go 内部改造 / 后台任务 + 目录产物

**Decision: D-ECL-01/02** — 独立包 (005c 算法原样生产化) + indexer 路由 (`__secNN` + section frontmatter → ECL; 其余零变化) + `SKILLFORGE_ECL_ENABLED` env 回退开关。

**Rationale:** 老 KB (authority 等) 零风险; 回滚 = env 关; Phase 2 老逻辑 (KNOW-02 校验/purge 契约) 不被触碰。

---

### 2. Milvus schema 迁移 → 加原生字段 + RECREATE (推荐)

**Options:** +4 原生字段 + RECREATE 重建 (chosen) / source 字段塞 JSON / 并行新 collection

**Decision: D-Milvus-01/02** — source_idx (VarChar) / section (VarChar) / section_idx (Int64) / section_count (Int64) 原生字段; 部署时 RECREATE=1 一次性重建 + 全量 re-embed; 老 KB 字段填零值。

**Rationale:** Phase 41 hybrid 检索需要原生 filter; source 塞 JSON 破坏 filter 且污染语义; reseed 幂等已修 (daf98d1); 并行 collection 与 purge 模式冲突。

---

### 3. LLM 抽取 + 标注集 (3 问全选推荐)

**3a. 触发时机 → Admin 手动触发**

**Options:** POST /api/admin/pipeline/run + FE 按钮 (chosen) / boot 自动 + 手动重跑 / 上传钩子

**Decision: D-Run-01** — admin token 门控手动触发; boot 不自动 (内网 LLM 间歇断会把 boot 炸进重试风暴); 重复 run 幂等 (KG note 级 replace)。

**3b. 标注集构造 → Regex 起草 + 人工 review**

**Options:** regex 起草 + 人工 review (chosen) / LLM 起草 + 抽查 / 仅 3 表锚点

**Decision: D-GT-01** — 确定性 regex 起草 26 notes 表引用 + 关系 → 人工 review 一遍 → `eval/ecl-golden/sql-guides.jsonl`。LLM 永不参与标注构造 (自证循环)。"仅 3 表锚点" 因 n=3 算不出有意义的 F1 被否。

**3c. ABox 类扩展路径 → emitter 硬编码 6→8**

**Options:** emitter 硬编码扩 NoteChunk/TableSchema (chosen) / 走 Phase 38 induction approve 闭环 / 硬编码 + induction 分层

**Decision: D-TBox2-01** — emitter classes 列表加 skf:NoteChunk + skf:TableSchema + belongsToNote/referencesTable。boot 可重现、确定性。induction 闭环被否的原因: emitter boot 重生成会覆盖 induction 加的类 (机制未解决), 且部署时人工前置步骤不可接受——induction 留给 Phase 40+ 真正探索性提议。

---

### 4. 失败恢复 + 进度推送 (2 问全选推荐)

**4a. Dead-letter → SQLite 表**

**Options:** SQLite ecl_failures 表 (chosen) / JSON 文件 / 内存队列

**Decision: D-DL-01/RT-01** — ecl_failures 表 (note_id/stage/attempt_count/last_error/status) + 每段 retry 3 次指数退避 → dead; 单 note 失败不阻塞整批; `POST /api/admin/pipeline/retry/:note_id` 手动重触发; 重启安全。内存队列被否 (SC-5 可靠性不成立)。

**4b. 进度推送 → 轮询 status endpoint**

**Options:** 轮询 (chosen, 2s 进行中/10s 空闲) / SSE 事件流

**Decision: D-Prog-01** — GET /api/admin/pipeline/status → {stage, current_file, done, total, queue_len, failures[]}; FE 参照 CapacityMonitor。26 notes 量级轮询绰绰有余; SSE 引入生命周期管理复杂度不值得。

---

## Implementation Plan Hints (for planner)

| Plan | Scope | Depends on |
|------|-------|------------|
| 39-01: internal/knowledge/ecl/ chunker | walk+merge 生产化 + boundary tests (185→26, 3/3 表) | — |
| 39-02: Milvus schema +4 字段 | seed.go schema + Document 行扩展 + 零值回填 + recreate 文档 | — |
| 39-03: emitter TBox 6→8 类 | classes 列表 + NoteChunk/TableSchema/referencesTable + 测试更新 | — |
| 39-04: LLM 抽取 + ABox 编排 | extract prompt + KG note 级 replace (store 加删除原语) + milvus reseed | 39-01, 39-02, 39-03 |
| 39-05: ecl_failures + retry + status API | SQLite 表 + retry 退避 + 3 路由 | 39-04 |
| 39-06: /admin/pipeline tab (UI-SPEC 先行) | 轮询面板 + Run/Retry 按钮 | 39-05 |
| 39-07: golden 标注集 + verify-phase-39.sh | regex 起草脚本 + jsonl + F1 harness + hermetic gate | 39-01..06 |

注: 实际拆分由 planner 决定; **UI-SPEC.md 必须先于 39-06** (v1.4 hard constraint)。

---

## Deferred Ideas (see CONTEXT.md `<deferred>`)

- Phase 40: SHACL / Provenance / TBox 演进日志 / rollback UX / induction 实战
- Phase 41: MCP / SPARQL / Hybrid Retrieval (D-Milvus-01 原生字段铺路)
- 本 phase out-of-scope: SQL 语句级 boundary / sec01 病理 / 非 sql-guides KB ECL 化 / cancel API

---

## Session End

- CONTEXT.md: `.planning/phases/39-ecl-pipeline/39-CONTEXT.md`
- DISCUSSION-LOG.md: `.planning/phases/39-ecl-pipeline/39-DISCUSSION-LOG.md`
- Next: `/gsd:ui-phase 39` (web 涉及, hard constraint) → `/gsd:plan-phase 39`
