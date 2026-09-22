# Phase 40: Data Governance — SHACL + Provenance + TBox Versioning - Context

**Gathered:** 2026-09-22
**Status:** Ready for planning

<domain>
## Phase Boundary

给 v1.4 Ontology Layer 加治理层。本 phase 交付:

- **SHACL 校验**: 自写轻量 Go 校验引擎 + 标准 TTL shapes 文件（路径可配置），boot 时全量校验 KG；error 级 violation 实体进隔离区（不进主索引），boot 继续
- **Provenance**: `Triple{S,P,O}` 扩展可选第 4 字段；ABox 全部 triples 带来源（source path + 行号区间 + 时间戳 + source_kind）
- **Tier 过滤 (SC-5)**: 内部 `QueryScoped` 函数按 skill `reads` 声明过滤 KG 查询（schema 层豁免 + ABox 数据层 fail-closed）；admin 查询面板演示；Phase 41 SPARQL/MCP 复用
- **TBox 版本演进日志**: `tbox_changelog.jsonl`（who/when/what/why）+ 任意历史版本 rollback（复用 D-TBox-03 机制）+ rollback 后自动重跑 SHACL
- **`/admin/governance` UI**: SHACL 报告（绿/黄/红）+ provenance graph（Cytoscape.js，新 FE 依赖）+ TBox 版本 timeline
- **`verify-phase-40.sh`**: SHACL violation detection + provenance coverage 100%（主索引+隔离区）+ rollback < 5s + tier 过滤单测/集成

**SC-1..7 (ROADMAP 259-279 行)**: (1) boot SHACL 校验 violation 清晰（source file + entity URI + 约束）; (2) 每条 ABox triple 带 `skf:provenance`; (3) TBox 版本号 + 演进日志; (4) rollback snapshot→验证→切换→KG rebuild < 5s; (5) KG query 自动 belongsToTier 过滤; (6) Admin UI 三件套; (7) verify 脚本全绿。

**不包含**: MCP + SPARQL endpoint + Hybrid Retrieval (Phase 41)；Milvus schema 变更（D-PROV-08 KG-only）；隔离区修复的具体 UI 交互（UI-SPEC 粒度）；induction 产实例数据的 tier 挂载（未来 phase）。

**UI 一致性 hard constraint**: `/admin/governance` 涉及 web — **plan-phase 前先跑 `/gsd:ui-phase 40` 产 UI-SPEC.md** (v1.4 Standards, STATE.md 36-41 行)。

</domain>

<decisions>
## Implementation Decisions

### SHACL 校验器与违规处置
- **D-SHACL-01 (LOCKED):** 自写轻量 SHACL 子集。shapes 文件用标准 SHACL TTL 表达（goRDFlib 解析），校验引擎纯 Go 实现，支持约 7 个 constraint type（`sh:minCount` / `sh:maxCount` / `sh:class` / `sh:datatype` / `sh:nodeKind` / `sh:pattern` / `sh:in`），零新 Go 依赖。后续加 constraint type 只改一个 switch。**不引**第三方 Go SHACL 库（生态空白）、**不用** JSON/Go struct shapes（丢标准兼容性，违反零硬编码精神）。
- **D-SHACL-02 (LOCKED):** 分级隔离。violation 分 error/warning 两级：error（结构性：class 不存在、约束类型错）→ 该实体相关 triples 进隔离区、不进 KG 主索引，boot 继续；warning（缺可选属性等）→ 放行 + 记录。服务永不因数据质量拒启（内网间歇断环境教训，参照 D-Run-01）。隔离实体在 `/admin/governance` 红区可见。
- **D-SHACL-03 (LOCKED):** 三色直接映射隔离状态：红 = 存在 error 级 violation（有实体被隔离）；黄 = 仅 warning 级（数据在 KG 但有瑕疵，含 provenance-missing 存量）；绿 = 0 violation。UI 三色与 KG 实际状态严格一致。
- **D-SHACL-04 (LOCKED):** 初始 shapes 全覆盖：8 个 Class 各一个 `sh:NodeShape`（属性基数/类型约束）+ ABox 关系完整性（`referencesTable` 指向必须存在、`belongsToTier` 指向存在的 Tier、NoteChunk 必带 provenance）。shapes 文件路径可配置（v1.4 Standards）。

### Provenance 数据模型
- **D-PROV-01 (LOCKED):** `Triple` 加可选第 4 字段 `Provenance` struct（`store.go:43` 扩展）。JSON snapshot 向后兼容（旧格式无字段照读）。KG triple 数量不膨胀、查询零额外跳数。reification / 旁路存储被否决（4× 膨胀 / 双写一致性负担）。
- **D-PROV-02 (LOCKED):** Provenance 带 `source_kind` 三分类：`file`（Skill individuals → `skills/{name}/SKILL.md`；ECL note/tbl → sql-guides 文件路径+行号）/ `emitter`（Tier/KBCategory 等纯生成物 → `emitter:v{N}`）/ `induction`（LLM approve 产物 → proposal id）。能追溯真实文件的就用文件，纯生成物诚实标 emitter — coverage 100% 可断言。
- **D-PROV-03 (LOCKED):** KG 查询结果默认携带 provenance（struct 层零成本，HTTP 响应默认含），无开关参数。admin 查询面板每条 triple 直接可见来源。
- **D-PROV-04 (LOCKED):** provenance graph 主视图 = 溯源链：源文件 → note_N → tbl_X 分层 + Skill → SKILL.md 关系；顶部搜索定位实体高亮其 provenance 邻域；点节点侧栏出明细（source/line/时间）。数据量可控（26 notes + 若干表）。布局交互细节留给 UI-SPEC。
- **D-PROV-05 (LOCKED):** line range 存 1-based 行号区间 `{start_line, end_line}`（与 ECL ChunkMeta 现有行信息对齐；人类可读、可直跳编辑器）。
- **D-PROV-06 (LOCKED):** ECL note 重跑 replace 后 provenance 覆盖为最新（新 statement 携带本次抽取时间）。provenance 永远描述"当前 statement 的来源事实"；抽取历史归 ECL 运行日志。
- **D-PROV-07 (LOCKED):** `provenance coverage 100%` 断言范围 = 主索引 + 隔离区全部 ABox triples（`source_kind` ∈ {file, emitter, induction} 即合格）。隔离区必须带 provenance 才能出 SC-1 要求的 source file 报告。
- **D-PROV-08 (LOCKED):** provenance KG-only，Milvus schema 不动（39 刚做过 RECREATE 迁移，再迁移成本高收益零 — 39 已有 source/section 字段满足检索过滤；KG 是溯源真源，Milvus 是检索副本）。
- **D-PROV-09 (LOCKED):** TBox rollback 完成后自动重跑一次 SHACL 全量校验，dangling 引用实体（如被回滚 Class 的 individuals）按 D-SHACL-02 标准分级流程进隔离区 — 零特殊分支，治理行为全局一致；rollback 成功提示带本次新增 violation 数。
- **D-PROV-10 (LOCKED):** pre-40 存量 snapshot 的无 provenance triples 处理：boot emitter 类 ABox 每次 boot 重生成时天然携带新 provenance（无需迁移）；snapshot 里的 ECL note/tbl 存量缺口 → 加载时标 warning 级 `provenance-missing` 放行进主索引 + governance 黄区提示 "Run ECL 回填"；admin 点 Run ECL（39 已有 API）note 级 replace 自动补全。boot 永不被迁移拖死。
- **D-PROV-11 (LOCKED):** `extracted_at` = server 本地时钟 `time.Now().UTC()`，RFC3339 格式。

### Tier 过滤落地范围 (SC-5)
- **D-TIER-01 (LOCKED):** 落地形态 = 内部 Go 函数（`QueryScoped(pattern, allowedTiers)` 类签名由 planner 定）+ 单元测试锁行为 + admin KG 查询面板加 tier/skill 选择器演示隔离效果。Phase 41 SPARQL/MCP 直接复用函数 — SC-5 验收不依赖尚不存在的消费方。**不建** skill-scoped API（40 无 chat 侧消费方）；**不做** FE 过滤（隔离逻辑必须服务端强制）。
- **D-TIER-02 (LOCKED):** ECL 写入时自动挂 `belongsToTier`：note_N 按所属 KB 的 tier（walk 时 KB→tier 映射已有）；tbl_N 继承引用它的 note（多 note 引用 → 多条 belongsToTier triple，任一匹配即可见）。
- **D-TIER-03 (LOCKED):** 过滤豁免语义 = 两层：TBox/schema 层（owl:Class/ObjectProperty 声明、Skill/SkillAgent/Tier/KBCategory 元数据）永不过滤（"字典"层）；ABox 数据实体（note/tbl 及未来业务实体）强制 tier 过滤，无 belongsToTier triple = 不可见（fail-closed）。
- **D-TIER-04 (LOCKED):** SC-5 verify 验收 = 单元测试（fixture KG：3 tier × 若干实体 + 3 个 skill reads 组合，断言可见集精确匹配，含 fail-closed 用例）+ 集成测试（boot → admin API 带 skill 参数 → 断言过滤后响应）双覆盖；admin 面板演示留 UAT 手测。
- **D-TIER-05 (LOCKED):** `allowedTiers` 从 skill registry（SKILL.md frontmatter `reads` 字段，fsnotify 热加载真源）读取后传参；KG 里的 `hasReadAccess` triple 仅作可视化投影。避免过滤器自引用（查 KG 需要 filter、filter 若查 KG 鸡生蛋），skill 改 reads 立即生效。
- **D-TIER-06 (LOCKED):** 过滤单位 = 实体级：先按 tier 筛出可见实体集，实体可见则其全部出边/入边可见（图不碎裂，一次集合交实现）。
- **D-TIER-07 (LOCKED):** note replace 后 tbl 继承 tier 的级联清理：ECL note replace 事务内重算受影响 tbl 的继承集（删该 tbl 全部继承 triple → 按现存 referencesTable 边重建）+ boot 时全量重建一次继承作幂等自愈兜底。隔离语义永不残留（残留 = 可见性放宽 = 越权风险）。
- **推论 (LOCKED):** 39 存量 note/tbl 既无 provenance 也无 belongsToTier → 40 上线后立即不可见（fail-closed）。**Run ECL 一次同时回填两者** — 与 D-PROV-10 的黄区引导合并为同一动作，升级路径闭环。

### TBox 演进日志 + rollback UX
- **D-LOG-01 (LOCKED):** 演进日志存 `data/ontology/tbox_changelog.jsonl` 追加写，与现有文件家族（proposals.json / tbox_version.txt / skillforge_tbox.v{N}.ttl）同域同生命周期（整目录拷贝即完整备份）。记录量小，timeline UI 全读即可。
- **D-LOG-02 (LOCKED):** 字段语义：`who` = 触发者枚举（`admin`（token 名，默认 "admin"）/ `induction:llm`（proposal 来源）/ `system:rollback`）；`what` = 本次变更 triples 清单（新增/删除的 Class/Property IRI 列表 + 计数）；`why` = approve 时存 LLM reasoning（proposal 已有）+ admin 可选备注，rollback 存目标版本号 + 原因备注。
- **D-LOG-03 (LOCKED):** timeline 上任何历史版本可点"回滚到此版本"（读 v{M} → 写 v{N+1} → 切软链 → 自增 — 与 D-TBox-03 单步机制完全同构，零额外机制）；SC-4 验收用单步（N-1）。跳多步 = 一条日志一条意图。
- **D-LOG-04 (LOCKED):** 仅 TBox 版本实际变化的事件写日志（approve / rollback）。reject 不写（proposal 自身有 rejected 状态）；boot 重生成版本不变不写。timeline = 纯版本演进史。

### Claude's Discretion
- QueryScoped 具体签名、过滤实现（集合交/索引选择）、缓存策略（当前 ~3000 triples 全扫 <1ms，缓存可能过度设计）
- SHACL 引擎包落位（`internal/ontology/shacl/` 新子包 or 其他）+ shapes 文件默认路径
- 隔离区的 store 侧实现（独立区/标记位）与恢复 API 细节
- Provenance struct 字段命名与 JSON 序列化细节
- tbox_changelog.jsonl 条目 struct 与读写 helper 落位
- verify-phase-40.sh 的具体测试编组（参照 verify-phase-38/39 模式）
- ECL 写入路径挂 provenance+belongsToTier 的具体改造点（abox.go 内）
- UI-SPEC 后的 `/admin/governance` 视觉与交互细节（交给 ui-phase）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 项目级 spec + roadmap
- `.planning/REQUIREMENTS.md` 第 692-697 行 — ONT-14..17 需求原文（本 phase 的验收锚点）
- `.planning/ROADMAP.md` 第 259-279 行 — Phase 40 SC-1..7
- `.planning/STATE.md` 第 36-41 行 — v1.4 Standards（UI-SPEC hard constraint / 零硬编码 / 无 cgo / handler ≤200 行）
- `.planning/phases/38-foundation-tbox-kg/38-CONTEXT.md` — D-TBox-01..04（版本归档/软链/rollback 机制）+ D-KG-01..04（snapshot/debounce）+ D-Induct-05（SHACL boot 校验定位）全程沿用
- `.planning/phases/38-foundation-tbox-kg/38-VERIFICATION.md` — rollback UX 延期项（per D-TBox-03 落地点）+ 交付基线
- `.planning/phases/39-ecl-pipeline/39-CONTEXT.md` — D-ECL/D-Run/D-DL/D-Prog（ECL 包结构/admin 手动触发/dead-letter/轮询）+ D-TBox2-01（8 类 TBox）
- `.planning/SKILL_SCHEMA.md` — SKILL.md 7 字段规范（`reads` 字段 = D-TIER-05 的真源）

### 现有代码资产
- `internal/ontology/store/store.go` — Triple struct（43 行，D-PROV-01 扩展点）/ Insert/InsertUnique/DeleteSubject/Query/Adjacent1/Adjacent2/Snapshot/LoadSnapshot/LoadFromTurtle/Stats 全 API
- `internal/ontology/emitter/emitter.go` — 8 Class/8 ObjectProperty/8 DatatypeProperty 生成（D-SHACL-04 的 shapes 覆盖对象）+ `emitter.go:204` NS 前缀模式
- `internal/ontology/emitter/upgrade.go` — 39 的 TBox 扩展先例（6→8 类）
- `internal/ontology/induction/induction.go` — dataDir 文件家族持久化模式（proposals.json 读写/原子写/writeFileAtomic）+ approve 流程（D-LOG-02 的 reasoning 来源）
- `internal/knowledge/ecl/abox.go` — ABox triples 产出点（D-PROV 挂载 + D-TIER-02 belongsToTier 写入点）；`manager.go` — note 级 replace 编排（D-TIER-07 事务内重算挂点）
- `internal/server/server.go` 第 659-672 行 — ontology admin 路由注册段（governance 新路由插入点）
- `internal/server/admin_ontology.go` — admin handler 模式样板（Deps 注入/JSON envelope/token 门控）
- `web/src/components/admin/ontology/` — OntologyTab / TBoxBrowser / KGQueryPanel / ontologyApi.ts（governance UI 的复用基座）
- `internal/knowledge/ecl/failures.go` — ecl_failures 持久化先例（若隔离区需 SQLite）

### Spike 验证
- `.planning/spikes/005a-owl-tbox-toolchain/README.md` + `main.go` — goRDFlib v0.1.0 API（turtle.Parse 解析 SHACL shapes 文件的工具）

### 数据文件家族
- `data/ontology/`（proposals.json / tbox_version.txt / skillforge_tbox.v{N}.ttl + 软链 skillforge_tbox.ttl）— D-LOG-01 的 tbox_changelog.jsonl 加入此家族

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **KG store 全 API**: Insert/DeleteSubject/Query/Adjacent1/Adjacent2/LoadFromTurtle — SHACL 校验遍历、QueryScoped 集合交、tbl 继承重建都基于现有原语组合，无需新存储层
- **emitter/upgrade.go 先例**: 39 刚做过 TBox 类扩展，同样的确定性生成模式可用于 provenance 相关 DatatypeProperty 声明（若 TBox 需声明 skf:provenance 属性）
- **dataDir 文件家族模式**: proposals.json 的 readFile/writeFileAtomic 是 JSONL 日志读写的直接模板
- **admin_ontology.go + ontology FE 组件**: governance 三面板与现有 TBoxBrowser/KGQueryPanel 同构，KGQueryPanel 加 skill 选择器即演示入口
- **39 ECL 管线**: abox.go 产 triples 的单点挂载 + manager.go replace 编排 = provenance/belongsToTier/继承重算的注入位置

### Established Patterns
- **boot 顺序**: emitter(TBox) → LoadSnapshot → TBox 灌入 → 立即 snapshot（D-KG-02）；SHACL 校验步骤插入此链（snapshot load 后、对外服务前）
- **admin token 门控 + JSON envelope**: 全部新路由沿用（403 before 503）
- **分级处置哲学**: 参照 D-Run-01（boot 不自动跑 ECL）— 治理动作不阻断 boot、可运维触发
- **INSERT-only 审计 vs 可变状态**: ecl_failures 用可变 status 字段的先例 — 隔离区状态演进同构（这不是审计表）
- **真源分离**: SKILL.md = reads 真源，KG triple = 投影（D-TIER-05）；KG = 溯源真源，Milvus = 检索副本（D-PROV-08）

### Integration Points
- **boot 链新步骤**: LoadSnapshot → [NEW] SHACL 全量校验（隔离区分流）→ TBox 灌入 → [NEW] tbl 继承全量重建（D-TIER-07 兜底）→ snapshot
- **rollback API**: `POST /api/admin/ontology/rollback`（D-TBox-03 命名）— 当前零实现，本 phase 新建；完成钩子 = 自动重跑 SHACL（D-PROV-09）+ 写演进日志（D-LOG-04）
- **approve 流程钩子**: induction approve → 写 v{N+1} + 追加 changelog 一条（who=induction:llm, what=新增 IRI 清单, why=LLM reasoning）
- **ECL 写入钩子**: abox.go 产 triple 时挂 provenance + note/tbl 挂 belongsToTier + replace 事务内重算受影响 tbl 继承
- **Cytoscape.js**: 新 FE 依赖（SC-6 点名），`web/package.json` 现无 — planner 加依赖 + React 包装组件

</code_context>

<specifics>
## Specific Ideas

- **决策联动闭环（讨论中推导，实施时必须保持）**:
  1. D-PROV-10 + D-TIER-03 → 存量升级 = governance 黄区一个 "Run ECL" 按钮，一键同时回填 provenance + belongsToTier（fail-closed 让缺 tier 实体不可见，Run ECL 是唯一恢复路径 — UI 必须把这个引导做明显）
  2. D-PROV-09 + D-LOG-04 + D-SHACL-02 → rollback = 换 TBox + KG rebuild + 自动 SHACL + dangling 进隔离 + changelog 一条记录，全程零人工分支
  3. D-TIER-02 + D-TIER-07 → belongsToTier 的写入与清理都在 ECL 事务边界内，boot 重建只是兜底不是常规路径
- **verify-phase-40.sh 四段参照**: (a) SHACL violation detection（fixture 注入坏数据 → 断言隔离分流）; (b) provenance coverage 100%（主索引+隔离区）; (c) rollback < 5s 计时; (d) tier 过滤单测/集成 — 全部 Go test 编组，无 live LLM 依赖
- 用户全程选择推荐项，讨论以"治理哲学 = boot 永不因数据质量拒启 + fail-closed 隔离 + 真源分离"为主轴

</specifics>

<deferred>
## Deferred Ideas

- **induction 产实例数据的 tier 挂载** — 40 阶段 ABox 数据实体仅 ECL 产出；未来 phase 若 induction 开始产 individuals，需定其 belongsToTier 挂载规则（当前 D-Induct-02 只产 schema）
- **Milvus provenance 字段** — 若 Phase 41 hybrid retrieval 证明需要 Milvus 侧直接携带溯源（当前 D-PROV-08 KG-only，查 KG 可得）
- **shapes 热加载 / 运行时重校验 API** — 当前 shapes 仅 boot 加载 + rollback 后重跑；运行期手动"Re-validate"按钮若 admin 有需求再加
- **隔离区自动修复策略** — 当前隔离实体人工经 Run ECL / 数据修复恢复；自动化修复（如按 shape 建议 auto-fix）属未来增强

</deferred>

---

*Phase: 40-Data Governance — SHACL + Provenance + TBox Versioning*
*Context gathered: 2026-09-22*
