# Phase 38: Foundation — TBox + Memory-map KG + Auto Schema Lift - Context

**Gathered:** 2026-09-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Server boot 自动生成 RDFS/OWL TBox (215 triples, 6 Classes / 6 ObjectProperties / 8 DatatypeProperties)，纯 Go 内存 map KG store (byS/byP/byO 索引 + JSON snapshot)，LLM 提议新 Class/Property 的人工审批闭环。本 phase 交付:

- `internal/ontology/` 目录：TBox emitter、KG store、schema induction orchestrator、admin handlers
- 启动期 TBox 生成 → KG 装载 → snapshot 落盘 (lossless reload)
- `/admin/ontology` 路由：TBox 类层次浏览器 + KG 三元组查询面板 + Schema Proposals 审批 Drawer
- `verify-phase-38.sh`: 单 binary 启动 + KG query < 1ms + reload lossless + approve 闭环 + 版本回滚

**三条护城河不可动摇**: Apache-2.0 单 binary (无 cgo, 无外部 process) + Skill 驱动 + Tier 信任隔离。

**Phase 38 SC-1~8 (来自 ROADMAP.md)**:
1. Server boot 生成 `skillforge_tbox.ttl.v{N}` (215 triples) + `skillforge_kg.json` snapshot; reload 字节级一致
2. KG query API 在 215 triples 上 < 1ms; memory 估计 < 50KB
3. `internal/ontology/` 目录无 cgo 依赖; `go build` 单 binary 通过
4. Admin UI 列出 8 Skills + 5 Tiers + 7 SubCategories (来自 TBox), 可点击查看 Class 详情
5. KG 三元组查询面板支持 `?x a skf:Skill` 模式查询, 结果可导出 JSON
6. LLM schema induction: Claude 提议新 Class/Property 写入 `/admin/ontology/proposals`, 需人工点击 approve 才进 TBox
7. TBox 版本号自增 1; rollback 到上一版本可一键还原
8. `verify-phase-38.sh` 全绿: 单 binary 启动 + KG query < 1ms + reload lossless

**不包含**: ECL Pipeline (Phase 39)、SHACL 校验 + Provenance + TBox 版本演进日志 (Phase 40)、MCP server + SPARQL endpoint (Phase 41)。这些是后续 phase。

</domain>

<decisions>
## Implementation Decisions

### TBox 文件落盘 + 版本归档
- **D-TBox-01 (LOCKED):** TBox TTL 文件按版本号归档。每次 boot 重生成 + induction approve 写入后, 文件名为 `skillforge_tbox.v{N}.ttl`, 同时维护软链 `skillforge_tbox.ttl` → 最新版 (`skillforge_tbox.v{N}.ttl`)。
- **D-TBox-02:** 版本号从 `SKILLFORGE_TBOX_VERSION` 文件 (`{data_dir}/tbox_version.txt`) 读取; 启动时若文件不存在则初始化为 1, 每次 approve 后自增 1 并重写文件 + 生成新 TTL。
- **D-TBox-03:** Phase 40 rollback 复用本机制: admin `POST /api/admin/ontology/rollback` 读取 `v{N-1}` 版本的 TTL 文件, 写入新 `v{N+1}` 并更新软链 + 自增版本号 (避免覆盖历史)。
- **D-TBox-04:** TBox TTL 字节级 lossless 由 goRDFlib 的 `turtle.Serialize` 保证 (005a spike 验证)。每次重生成字节一致 (因输入 deterministic)。

### KG snapshot 触发时机
- **D-KG-01 (LOCKED):** KG snapshot `skillforge_kg.json` 写盘采用「boot 加载完成即写 + Insert 事件 debounce 5s」双触发。
- **D-KG-02:** server boot 流程: `LoadSnapshot(skillforge_kg.json)` → 若失败则用空 Store 初始化 → 用 TBox 215 triples 灌入 → 立即写 snapshot (确保首次启动也有文件)。后续 reload 走 `LoadSnapshot` 路径。
- **D-KG-03:** Insert/Update/Delete 调用进入 debounce 队列, 5s 静默后触发 `Snapshot()` 写盘。Phase 38 阶段 Insert 几乎静态 (TBox 215 triples + 人工 approve), debounce 实际只触发一次 boot snapshot。Phase 39 ECL Pipeline 高频 Insert 由 debounce 自然合并, 无需额外逻辑。
- **D-KG-04:** snapshot 路径可配置 (`SKILLFORGE_ONTOLOGY_SNAPSHOT_PATH`), 默认 `{data_dir}/skillforge_kg.json`。

### LLM 提议审批 UI 形态
- **D-UI-01 (LOCKED):** Schema Proposals 审批采用 **Drawer 侧滑 + diff preview** 模式。
- **D-UI-02:** AdminDashboard 新增第 8 个 tab「Schema Proposals」, 与现有 7 个 tab (Import Batches / KBs / KPI / Skills / Audit / Schedules / Feedback) 并列。
- **D-UI-03:** Proposals 列表行项展示: LLM 提议的新 Class/Property 名称 (短) + 自然语言描述 + 提议时间 + 提议来源 (哪个 chunk 触发了 induction)。点击行项触发右侧 Drawer 滑出 (复用 shadcn `Sheet` 组件, 与 `/admin/*` 风格一致)。
- **D-UI-04:** Drawer 内容三段式:
  1. **Diff preview** — 新增 triples 的 before/after 对比 (现有 TBox 不变, 提议新增 3-8 个 triples, 高亮新增部分)
  2. **Reasoning** — LLM 解释为什么提议 (引用哪段 chunk 文本)
  3. **Approve / Reject** — 底部固定按钮; approve 触发 `POST /api/admin/ontology/proposals/:id/approve` (后端生成新 IRI + 写入 v{N+1}.ttl + Insert KG), reject 触发 `POST /api/admin/ontology/proposals/:id/reject` (仅标记, 不删记录)
- **D-UI-05:** `/admin/ontology` 路由本身就是 AdminDashboard 内的子路由 (沿用现有 `/admin/audit`、`/admin/skills` 等约定, 通过 `?tab=ontology` query param 切换 tab)。

### induction 命名 + 语义上限
- **D-Induct-01 (LOCKED):** LLM 输出自然语言描述 + 后端自动生成 PascalCase IRI 本地名 + 强制 `rdfs:label` 双语 (@zh + @en)。
- **D-Induct-02:** LLM 提议的 prompt schema 固定为 `{kind: "Class"|"ObjectProperty"|"DatatypeProperty", description_zh: string, description_en: string, target_skill?: string, source_chunk_id?: string}`。本地名由后端从 description_en 用确定性算法生成 (去除空格、特殊字符、首字母大写、去重追加 N)。
- **D-Induct-03:** IRI 命名空间遵循 `SKILLFORGE_ONTOLOGY_NS` env, 默认 `http://skillforge.local/ontology#`。生成的 IRI = `{ns}{PascalCaseLocalName}` (e.g., `http://skillforge.local/ontology#BusinessRule`)。
- **D-Induct-04:** admin 可在 Drawer 内修改本地名 (LRU 缓存防冲突: 后端 `POST /api/admin/ontology/proposals/:id/preview-iri` 返回冲突检查结果)。
- **D-Induct-05:** Phase 38 不引入 OWL DL 推理校验 (与 goRDFlib 纯解析库特性匹配)。Phase 40 SHACL 校验会在 boot 时统一验证 (覆盖 TBox + ABox)。
- **D-Induct-06:** LLM 提议生成时机: Phase 38 仅暴露 admin 手动触发入口 (`POST /api/admin/ontology/proposals/generate`, 接收一段文本输入 → Claude 提议); Phase 39 ECL Pipeline 会把每个 merged note 自动喂给 induction。

### 其他决策
- **D-Lib-01:** TBox emitter 复用 005b spike 的纯字符串拼接 + goRDFlib 序列化 (无额外 RDF-emit 库依赖)。KG store 复用 005d-b spike 的 API (`Insert/Query/Adjacent1/Adjacent2/Snapshot/LoadSnapshot`)。
- **D-Lib-02:** TBox IRI namespace 通过 `SKILLFORGE_ONTOLOGY_NS` env 可配 (默认 `http://skillforge.local/ontology#`)。TBox 数据目录通过 `SKILLFORGE_ONTOLOGY_DATA_DIR` 可配, 默认 `{project}/data/ontology/`。
- **D-Route-01:** 新增 API 路由:
  - `GET /api/admin/ontology/tbox` — 返回当前 TBox TTL 全文 + 版本号 + 三元组计数
  - `GET /api/admin/ontology/kg/stats` — 返回 KG 三元组数 / byS/byP/byO 索引大小 / 内存估算
  - `GET /api/admin/ontology/kg/query?s=&p=&o=` — 模式匹配查询, 支持 `?x` 通配
  - `POST /api/admin/ontology/proposals/generate` — body `{text: string}` → Claude 提议 → 写入 proposals 队列
  - `GET /api/admin/ontology/proposals` — 列出待批 proposals
  - `POST /api/admin/ontology/proposals/:id/approve` — approve, 触发 TBox 版本自增
  - `POST /api/admin/ontology/proposals/:id/reject` — reject, 仅标记
  - `POST /api/admin/ontology/snapshot` — 手动触发 KG snapshot (调试用, debounce 自动触发为主路径)
- **D-Test-01:** 单元测试覆盖: TBox emitter roundtrip lossless / KG Insert-Query-Snapshot / debounce 行为 / LLM 提议生成 PascalCase 本地名 / IR I 命名空间 env 可配 / 7 字段 SKILL.md frontmatter 解析。
- **D-Test-02:** E2E 测试 (verify-phase-38.sh): boot 单 binary → curl /api/admin/ontology/kg/query → 8 个 Skill 返回 → 模拟 approve proposal → 重新加载 TBox v{N+1} → reload 后字节级一致。

### Claude's Discretion
- Drawer 内 diff preview 的具体展示 (行级 / 树级 / 节点卡片) — 在 UI-SPEC 中具体化, 实施时按 shadcn 风格统一
- LLM 提议生成用的 model 选择 (`SKILLFORGE_ONTOLOGY_MODEL`, 默认沿用 `SKILLFORGE_LLM_MODEL`)
- debounce 5s 的精确值是否在 verify-phase-38.sh 测试中可调
- `internal/ontology/` 包内子包划分 (emitter / store / induction / handler — 4 个子包, 每个 < 200 行 handler 原则)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spike 验证 (源码 + README)
- `.planning/spikes/005a-owl-tbox-toolchain/README.md` — goRDFlib v0.1.0 API (MustURIRef / URIRef / Literal.Value / turtle.Parse / turtle.Serialize / g.Triples) + 50 triples 验证
- `.planning/spikes/005a-owl-tbox-toolchain/main.go` — 完整 goRDFlib 用法样例
- `.planning/spikes/005b-skillschema-rdfs-lift/README.md` — TBox emitter 算法 (215 triples, 6 Classes / 6 ObjectProperties / 8 DatatypeProperties, roundtrip lossless)
- `.planning/spikes/005b-skillschema-rdfs-lift/main.go` — TBox emitter 源码 (yaml.v3 + 字符串拼接 + goRDFlib)
- `.planning/spikes/005b-skillschema-rdfs-lift/skillforge_tbox.ttl` — 215 triples 输出样本
- `.planning/spikes/005b-skillschema-rdfs-lift/go.mod` — 依赖列表 (yaml.v3 + tggo/goRDFlib v0.1.0)
- `.planning/spikes/005d-b-memorymap/README.md` — memory-map KG store 验证 (215 triples load 7.71ms / query < 1ms / mem ~43KB)
- `.planning/spikes/005d-b-memorymap/main.go` — KG store API (Insert / Query / Adjacent1 / Adjacent2 / Snapshot / LoadSnapshot) 完整实现

### 项目级 spec + roadmap
- `.planning/REQUIREMENTS.md` 第 670-683 行 (v1.4 Requirements: ONT-01..09) — Phase 38 锁定 9 条 ONT 需求
- `.planning/ROADMAP.md` 第 184-205 行 (Phase 38 description + SC-1~8)
- `.planning/PROJECT.md` 第 110-147 行 (v1.4 milestone scope + 护城河 + UI 一致性 hard constraint)
- `.planning/STATE.md` 第 25-32 行 (v1.4 phase map)
- `.planning/SKILL_SCHEMA.md` — 7 字段 SKILL.md 规范 (description / agent / reads / writes / steps / guardrails / temperature)

### 现有 admin UI 约定
- `.planning/ROADMAP.md` 第 192 行 (UI-SPEC Target: `/admin/ontology`)
- `web/src/components/admin/AdminDashboard.tsx` 第 8-50 行 (Tab 结构 + SubrouteTab 组件)
- `web/src/App.tsx` 第 39 行 + 330 行 + 390-397 行 (AdminDashboard 路由挂载点)
- `internal/server/server.go` 第 569-640 行 (现有 admin route 注册风格: `/api/admin/platform-prompt` / `/api/admin/kpis` / `/api/admin/kb` / `/api/admin/feedback/...` / `/api/admin/table-counts` / `/api/admin/batches` / `/api/admin/audit` / `/api/admin/skills` / `/api/admin/schedules`)
- `internal/server/admin_skills.go` (admin handler 风格样板)

### 三条护城河硬约束
- `.planning/PROJECT.md` 第 142-147 行 (Apache-2.0 单 binary / Skill 驱动 / Tier 信任隔离)
- `.planning/REQUIREMENTS.md` 第 706-710 行 (Out of scope: 完整 OWL 推理 / 自动模式 induction / 多实例 federation / Leiden)
- `.planning/STATE.md` 第 36-41 行 (v1.4 Standards: UI 一致性 / 零硬编码 / Go 最佳实践)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/server/admin_*.go`** 模式样板: `Deps` 注入、handler 函数 `func (s *Deps) xxxHandler(ctx, c *app.RequestContext)`、JSON envelope 结构。Phase 38 新增的 `internal/server/admin_ontology.go` 沿用此模式。
- **`internal/knowledge/`**: 现有 `internal/knowledge/indexer.go` 是 Phase 39 ECL Pipeline 要替换的 byte-cap chunker, Phase 38 不动。
- **`internal/server/server.go` 第 569-640 行**: 现成 admin route 注册段 (hertz `h.GET / h.POST` 注册风格), 新增 ontology routes 顺势插入。
- **`web/src/components/admin/AdminDashboard.tsx`**: 7 个 SubrouteTab 复制粘贴式的现成组件, 第 8 个 tab 是「Schema Proposals」+ 默认 view 是「TBox 类层次 + KG 三元组查询」双面板。
- **005b spike main.go** 是 `internal/ontology/emitter` 子包的蓝本; **005d-b spike main.go** 是 `internal/ontology/store` 子包的蓝本。

### Established Patterns
- **YAML frontmatter parsing**: 005b 用 `gopkg.in/yaml.v3` 解析 SKILL.md 7 字段; Phase 38 TBox emitter 复用此 parser。
- **TTL string 拼接 + goRDFlib 校验**: 005b 算法是先字符串拼 TTL, 再用 `turtle.Parse(g, r)` 解析校验; Phase 38 沿用。
- **byS/byP/byO 三索引 + JSON snapshot**: 005d-b 的 store API; Phase 38 沿用。
- **admin handler ≤ 200 行**: 现有 admin_*.go 全部 < 200 行 (Phase 30 SRP 修复后), Phase 38 ontology handler 沿用。
- **JSON envelope + admin token gate**: 现有 admin API 全部通过 `SKILLFORGE_ADMIN_TOKEN` + `crypto/subtle.ConstantTimeCompare` 门控, Phase 38 沿用。
- **`r.GET / r.POST` 风格 + 错误处理**: hertz `app.RequestContext` + `c.JSON(consts.StatusOK, ...)` 风格。

### Integration Points
- **server boot 流程** (`cmd/server/main.go` 第 275-`main()` 行): TBox emitter + KG store + proposal queue 在 boot 时初始化, 注入到 `Deps`。
- **AdminDashboard 路由**: Phase 38 UI-SPEC 在 `web/src/App.tsx` 第 390-397 行 AdminDashboard 挂载逻辑内扩展第 8 个 tab。
- **`internal/llm/`**: 现有 LLM client (e.g., GLM adapter), LLM schema induction 复用其 `Generate` 接口。
- **`internal/store/sqlite/`**: proposals 持久化可走 SQLite (轻量, 不增加新依赖); 也可走 `data_dir/proposals.json` 文件 — 实施时由 planner 选。

</code_context>

<specifics>
## Specific Ideas

- **TBox 类层次浏览器**: 复用 005b 的 6 Classes (Tier / KBCategory / SubCategory / Skill / SkillAgent / RefusalPhrase) 作为根节点, ObjectProperty 作为边; 点击 Class 节点右侧 Drawer 展示该 Class 的所有 instances (e.g., 点击 `skf:Skill` → 8 个 skill 实例列表, 每条展开看 reads/writes/temperature/agent 详情)。
- **KG 三元组查询面板**: 输入框支持 `?x a skf:Skill` 模式; 提交后表格展示 (subject / predicate / object) 三列; 顶部「Export JSON」按钮触发 `?format=json` 下载。
- **Schema Proposals Drawer**: 顶部 sticky 头部 (proposal ID + 时间 + 来源 chunk ID), 中部 diff 视图 (左侧「Current TBox」右侧「After Approval」), 底部 reasoning 折叠区 + Approve/Reject 按钮。Reject 后 proposal 标记 `status=rejected` 保留 7 天后清理。
- **rollback UX (虽属 Phase 40 范围, Phase 38 需预留 API)**: Phase 38 暴露 `GET /api/admin/ontology/tbox/versions` 列出所有 v{N}.ttl; Phase 40 添加 `POST /api/admin/ontology/tbox/rollback` 调用。
- **CLI 调试入口 (实施时由 planner 决定是否纳入)**: `cmd/spiked/ontology-emit` + `cmd/spiked/ontology-query` 两个 spike 风格的子命令, 验证 005b/005d-b spike 在生产代码路径上仍然有效。

</specifics>

<deferred>
## Deferred Ideas

### Phase 39 待办
- Section-aware markdown chunker 替换 `internal/knowledge/indexer.go` byte-cap
- LLM 自动从 merged note 抽取 etl_* 实体 + 关系
- ABox 自动写入 KG + Milvus 双向同步
- ECL Pipeline 实时状态 UI (`/admin/pipeline`)

### Phase 40 待办
- SHACL 校验启动时执行
- Provenance 链 (每条 ABox statement 带 source_path)
- TBox 版本演进日志 (who / when / what / why)
- TBox rollback 完整 UX (Phase 38 仅暴露 versions 列表 API)
- `/admin/governance` 路由

### Phase 41 待办
- MCP server 暴露 KG (`skf_query` / `skf_skills` / `skf_chunk_for` tools)
- SPARQL-like REST endpoint `POST /api/sparql`
- Hybrid Retrieval 1-2 跳 subgraph expansion
- `/admin/platform` 路由

### 明确 out-of-scope (来自 REQUIREMENTS.md v1.4 Out of Scope)
- 完整 OWL 2 RL/DL 推理引擎 — goRDFlib 不含推理器, 引入会破坏单 binary 护城河
- Schema induction 自动模式 (无人工审批) — 现阶段强制 human-in-the-loop
- 多 SkillForge 实例 KG federation — 单进程 only
- Leiden community detection — KB 规模 < 500 notes 不值得

### 已折叠但待用户决定的具体 UI 细节 (UI-SPEC 阶段细化)
- Drawer 宽度 (建议 50% viewport, 与 shadcn Sheet 默认一致)
- diff preview 的视觉呈现 (行级文字 diff vs 节点卡片 vs JSON tree)
- Approve 后 TBox 详情刷新策略 (乐观更新 vs 重新拉取)
- Proposal 列表的默认排序 (时间倒序 vs 状态分组)

</deferred>

---

*Phase: 38-Foundation — TBox + Memory-map KG + Auto Schema Lift*
*Context gathered: 2026-09-21*
*Source: 005 系列 5 spike 全 VALIDATED; v1.4 milestone initialized 2026-09-21*
</content>
</invoke>