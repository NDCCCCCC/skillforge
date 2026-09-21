# Phase 39: ECL Pipeline — Section-aware Chunker + LLM Entity Extraction - Context

**Gathered:** 2026-09-21
**Status:** Ready for planning

<domain>
## Phase Boundary

替换 sql-guides 的 byte-cap 切分痛点：section-aware chunker 把 185 个 `__secNN` 片段合并回 26 个完整 logical notes；LLM 从 merged notes 抽取 etl_* 表引用 + 关系成 ABox triples；ABox 双写 (memory-map KG + Milvus 原生元数据字段)；`/admin/pipeline` 实时状态面板 + 失败重试。本 phase 交付:

- `internal/knowledge/ecl/` 独立包: walk + merge (005c 算法) + LLM 抽取 + ABox 双写编排
- `internal/knowledge/indexer.go` 路由分支 (`__secNN` + section frontmatter → ECL 路径; 其余走现有逻辑零变化)
- Milvus schema +4 字段 (source_idx / section / section_idx / section_count) + RECREATE 迁移门
- emitter.go TBox 类 6→8 (+ skf:NoteChunk, skf:TableSchema) + 新 ObjectProperty
- SQLite `ecl_failures` 表 (retry ×3 → dead-letter → 手动重触发)
- `/admin/pipeline` admin tab (轮询 status endpoint)
- `eval/ecl-golden/sql-guides.jsonl` ground truth 标注集 (regex 起草 + 人工 review)
- `verify-phase-39.sh` (100% boundary + F1 ≥ 0.8)

**SC-1..6 (ROADMAP)**: (1) 185→26 100% 边界 + note 001 含 3/3 etl_* 表; (2) LLM 抽取 ≥80% precision@recall vs 标注集; (3) ABox 自动写 KG + Milvus 同步; (4) Admin UI 实时进度 + 重试按钮; (5) retry 3 次 → dead-letter 手动可触发; (6) verify 脚本全绿。

**不包含**: SHACL 校验 + Provenance + TBox 版本演进日志 (Phase 40)；MCP + SPARQL + Hybrid Retrieval (Phase 41)；SQL 语句级 boundary 检测 (005c out-of-scope)；sec01 病理修复 (上游数据质量，005c 已注明非 chunker 职责)。

**UI 一致性 hard constraint**: `/admin/pipeline` 涉及 web — **plan-phase 前先跑 `/gsd:ui-phase 39` 产 UI-SPEC.md** (v1.4 Standards)。

</domain>

<decisions>
## Implementation Decisions

### Chunker 落位策略
- **D-ECL-01 (LOCKED):** 新建独立 `internal/knowledge/ecl/` 包 (walk + merge + 抽取编排)；`indexer.go` 只加路由分支——文件名含 `__secNN` 且 frontmatter 有 `section` 字段走 ECL 路径，否则走现有 Phase 2 逻辑 (authority 等其他 KB **零变化**)。env 开关 `SKILLFORGE_ECL_ENABLED` (默认 on) 可整体回退老路径。
- **D-ECL-02:** 005c 算法原样生产化: filename regex `__sec(\d{2})_(.+)\.md$` + frontmatter `section` 分组 + first-seen 顺序; `ChunkMeta` 契约 (source_idx / title / section / section_idx / section_count / source_paths / body_bytes) 与 spike `chunk_metadata.json` 一一对应。

### Milvus schema 迁移
- **D-Milvus-01 (LOCKED):** `internal/retriever/milvus/seed.go` schema 加 4 个**原生字段**: `source_idx` (VarChar) / `section` (VarChar) / `section_idx` (Int64) / `section_count` (Int64)。部署时 `SKILLFORGE_MILVUS_RECREATE=1` 一次性重建 + 全量 re-embed (202 文件量级; reseed 幂等已由 replace-by-tier purge 保证, daf98d1)。**不用** source 字段塞 JSON (破坏 filter 能力，Phase 41 hybrid 检索需要原生字段)。
- **D-Milvus-02:** 老 KB (无 section 元数据) 的字段填零值 (`source_idx=""` / `section_idx=0` / `section_count=0`)，检索 filter 语义不变。

### LLM 抽取 + 标注集
- **D-Run-01 (LOCKED):** ECL 由 **admin 手动触发** `POST /api/admin/pipeline/run` (token 门控) + FE「Run ECL」按钮。boot **不**自动跑 (内网 LLM 间歇断，memory env-intranet-service-flakiness——自动跑会在断线时把 boot 炸进重试风暴)。重复 run 幂等: KG 按 note 级 replace (先删该 note 旧 triples 再插新)。
- **D-GT-01 (LOCKED):** ground truth = **regex 起草 + 人工 review**: 用确定性 regex (005c 已验证 `update etl_*` / table 模式) 抽全部 26 notes 的表引用 + note 间关系，人工 review 一遍产出 `eval/ecl-golden/sql-guides.jsonl` (每 note: 表引用清单 + 关系)。**LLM 永不参与标注集构造** (自证循环)。verify-phase-39.sh 用它算 F1。
- **D-TBox2-01 (LOCKED):** ABox schema 走 **emitter 硬编码扩展 6→8 类**: + `skf:NoteChunk` + `skf:TableSchema` + 对应 ObjectProperty (`skf:belongsToNote` / `skf:referencesTable`)。boot 可重现、确定性; induction approve 闭环留给 Phase 40+ 的真正探索性提议 (且 emitter 尚无保留 approved 类的机制，现在走 induction 会被 boot 重生成覆盖)。
- **D-Ext-01:** LLM 抽取 prompt schema 固定 JSON: `{tables: [{name, kind: "update|select|insert|create"}], related_notes: [{source_idx, relation}]}` per merged note; temperature 0; 解析走 ExtractJSON 防御路径 (复用 internal/llm 模式)。抽出的 triples 形态: `skf:note_{source_idx} a skf:NoteChunk` / `skf:note_{source_idx} skf:referencesTable skf:tbl_{name}` / TableSchema individuals 去重全局共享。

### 失败恢复 + 进度推送
- **D-DL-01 (LOCKED):** dead-letter 存 **SQLite 新表 `ecl_failures`** (note_id / stage / attempt_count / last_error / status `pending|dead|resolved`)，复用现有 store migration + INSERT-only 模式; 重启后 dead 记录仍在。手动重触发: `POST /api/admin/pipeline/retry/:note_id`。
- **D-RT-01:** 每段 (extract / kg-load / milvus-load) 失败自动 retry 3 次 (指数退避)，3 次后置 `status=dead` 并继续处理其余 notes (单 note 失败不阻塞整批)。
- **D-Prog-01 (LOCKED):** 进度走**轮询** `GET /api/admin/pipeline/status` → `{stage, current_file, done, total, queue_len, failures[]}`；FE 2s 轮询 (run 进行中) / 10s (空闲)，参照 CapacityMonitor / batches 模式。26 notes 量级轮询绰绰有余，不引入 SSE 生命周期管理复杂度。

### 其他
- **D-Route-01:** 新增 API: `POST /api/admin/pipeline/run` / `GET /api/admin/pipeline/status` / `POST /api/admin/pipeline/retry/:note_id`。全部 admin token 门控 (403 before 503)。
- **D-Boot-01:** ABox KG 写入复用 Phase 38 `store.Insert` + debounce snapshot (SetOnChange 同款 hook 模式)；note 级 replace 需要给 store 加 `DeleteWhere(pred, obj)` 辅助 (或等价删除原语) — planner 定具体 API。

### Claude's Discretion
- ECL stage 编排的并发度 (26 notes 串行或小并发; LLM 调用建议 ≤3 并发)
- ecl_failures 表 DDL 细节 + migration 挂载点 (跟随现有 store 模式)
- run 取消机制 (cancel API) 是否纳入本 phase
- retry 退避的具体间隔 (e.g. 1s/4s/16s)
- KG 删除原语的具体签名
- 标注集 review 流程的操作方式 (一次性 checklist 文档 vs 分批)
- UI-SPEC 后的 `/admin/pipeline` 视觉细节 (交给 ui-phase)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spike 验证 (源码 + 产物)
- `.planning/spikes/005c-ontology-aware-chunker/main.go` — section-aware walk+merge 参考实现 (直接生产化蓝本)
- `.planning/spikes/005c-ontology-aware-chunker/README.md` — 185→26 验证数据 + section-boundary 规则 4 条 + sec01 病理说明
- `.planning/spikes/005c-ontology-aware-chunker/chunk_metadata.json` — ChunkMeta 契约样本 (69 records)
- `.planning/spikes/005c-ontology-aware-chunker/merged_notes/` — 26 个 merged note 样本 (note 001 含 3/3 etl_* 表)

### 现有代码资产
- `internal/knowledge/indexer.go` — 要加路由分支的入口; `Index(ctx, knowledgeDir, emb, mc, cfg, reg)` 签名 + replace-by-tier purge 契约注释 (125-160 行)
- `internal/retriever/milvus/seed.go` 第 71-80 行 — collection schema 定义处 (加 4 字段的位置)
- `cmd/server/main.go` 第 1133-1136 行 — SKILLFORGE_MILVUS_RECREATE 一次性重建门 (已有)
- `internal/server/pipeline.go` / `admin_batches.go` — server 侧 pipeline 模式 + admin 轮询面板样板
- `web/src/components/admin/CapacityMonitor.tsx` — FE 轮询样板 (localStorage token + useEffect fetch)
- `internal/ontology/store/store.go` — Phase 38 KG store (Insert/Query/Snapshot; 需加删除原语)
- `internal/ontology/induction/induction.go` — SetOnChange/Generate 模式参考 (ECL 不直接用 approve 闭环，但 LLM 调用模式同构)
- `internal/ontology/emitter/emitter.go` — classes 列表 (6→8 扩展点, render 函数)
- `internal/llm/extract.go` — ExtractJSON + temperature-0 + response_format 模式 (ECL 抽取 prompt 复用)

### 项目级 spec
- `.planning/REQUIREMENTS.md` 第 687-690 行 (ONT-10..13)
- `.planning/ROADMAP.md` 第 209-228 行 (Phase 39 SC-1..6)
- `.planning/phases/38-foundation-tbox-kg/38-CONTEXT.md` — Phase 38 决策 (TBox 版本归档 / debounce / admin 模式) 本 phase 全程沿用
- `.planning/phases/38-foundation-tbox-kg/38-VERIFICATION.md` — Phase 38 交付面 (衔接基线)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **005c main.go**: walk (regex) + group (source_idx, section) + sort (__secNN) + merge — 直接搬进 `internal/knowledge/ecl/`
- **Phase 38 ontology stack**: `store.Insert/Query` + debounce + boot 集成点全部可复用; emitter classes 列表是唯一要改的 schema 点
- **milvus seed.go**: schema 加字段 + Document 行结构扩展; RECREATE 门现成
- **store migration 模式**: ecl_failures 表跟随现有 SQLite migration 挂载

### Established Patterns
- **admin handler**: requireAdmin + Deps 字段 nil-posture + JSON envelope (Phase 38 admin_ontology.go 是最近样板)
- **轮询面板**: CapacityMonitor (useEffect + localStorage token)
- **LLM 调用**: temperature 0 + ExtractJSON 防御解析 + 固定 JSON prompt schema (induction / verdict 两处同构)
- **INSERT-only 持久化 + 审计**: ecl_failures 状态演进用新行或状态字段 (planner 定，倾向 status 字段 + 更新时间戳——重试语义需要可变状态，与 T-3-AUDIT 的 INSERT-only 约束不冲突，因这不是审计表)

### Integration Points
- **boot 顺序**: emitter (TBox v2 基线 8 类) → KG load → ECL 不在 boot 自动跑，但 ecl_failures 表 migration 在 boot 建
- **KG note 级 replace**: 需要删除原语 (`DeleteWhere(pred, obj)` 类似) — store 当前只有 Insert/Query
- **Milvus re-embed**: ECL run 的 milvus-load 段走 reseed 路径 (purge sql-guides tier → insert merged sections)

</code_context>

<specifics>
## Specific Ideas

- **verify-phase-39.sh 三段**: (a) chunker boundary 185/185 + note 001 含 3/3 表 (纯 Go test); (b) F1 vs golden jsonl (LLM stub 模式下跑确定性假抽取器算 harness 正确性; 真 LLM F1 是 live operator gate); (c) ABox 双写 + dead-letter 流转 (Go test + stub)
- **标注集格式**: 每行 `{"source_idx": "001", "tables": [{"name": "etl_payment_due_t", "kind": "update"}], "related": ["006"]}` — 26 行
- **Run ECL 按钮**在 `/admin/pipeline` tab 顶部; 进行中显示进度条 + 当前文件; failures 列表行内 Retry 按钮
- **sec01 病理 (section==title)** 不修 — 005c 已注明是上游数据质量; ECL 照常合并 (多一个冗余 section 无害)

</specifics>

<deferred>
## Deferred Ideas

### Phase 40 待办
- SHACL 启动校验 + Provenance (source_path/line range) + TBox 版本演进日志 + rollback UX (D-TBox-03 from Phase 38)
- induction approve 闭环的实战使用 (真正探索性 schema 提议)

### Phase 41 待办
- MCP server + SPARQL endpoint + Hybrid Retrieval (KG 1-2 跳 + Milvus 重排——D-Milvus-01 的原生字段正是为此铺路)
- 按 section filter 的 Milvus 检索

### 明确 out-of-scope (本 phase)
- SQL 语句级 boundary 检测 (005c out-of-scope)
- sec01 section==title 病理修复 (上游数据质量)
- 非 sql-guides KB 的 ECL 化 (按需后议)
- ECL run 的 cancel API (若实施中发现需要可加，不阻塞)

</deferred>

---

*Phase: 39-ECL Pipeline*
*Context gathered: 2026-09-21*
*Source: spike 005c VALIDATED + Phase 38 交付基线; 6 决策 D-ECL/Milvus/Run/GT/TBox2/DL/Prog 锁定*
