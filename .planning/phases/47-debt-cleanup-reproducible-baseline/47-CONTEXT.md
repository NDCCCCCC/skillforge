# Phase 47: 清债与可复现基线 - Context

**Gathered:** 2026-10-19
**Status:** Ready for planning

<domain>
## Phase Boundary

让 v1.6 遗留债务**全部闭合或显式处置**，产出一条**不闪红、可复现**的全量测试基线。这是 v1.7 的地基：Phase 48 的 CI 全量门（Go / FE / Docker 三线进 GitHub Actions，PR 公开可见绿灯）与 Phase 49 的 hermetic 套件（跑全量门）都依赖它。

本 phase 交付 4 件事：

1. **REL-01** — `TestDeriveEdgesDeterministic` 的 flaky 根因消除（真实根因是**边方向非确定性**，非浮点 Weight；见 D-01）
2. **REL-02** — rollback 成功后 `/admin/governance` 的 SHACL 报告与重校验结果一致（后端缓存刷新 + FE 死分支恢复）
3. **REL-04** — BL-01..04 补录进 REQUIREMENTS Traceability 表；v1.7 Traceability 100% 填充
4. **遗留项处置记录** — D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2、BL-01..04 各有明确去向

**不包含**：CI 工作流建设（Phase 48）、hermetic demo（Phase 49）、README/发布物（Phase 50/51）、任何新功能或新 UI。

**硬约束**：`verify-phase-47.sh` 在 WSL `CGO_ENABLED=1` 下 exit 0；全量门（P4 `./internal/...`）**归零**（空 waiver）。

</domain>

<decisions>
## Implementation Decisions

### D-01: REL-01 修复落点 = 实现确定化 + 测试加固（选项 D）

**根因已更正**：`TestDeriveEdgesDeterministic` 的 flake **不是**浮点 `Weight` 精度问题。是 `internal/knowledge/graph_edges.go` 中 `sourceTokens` 的 **map 迭代顺序随机**，导致同一对 source 的 `keyword_cooccur` 边在两个端点间翻转 `From`/`To`；测试末尾排序键 `(From, To, EdgeType)` 无法把 `(1→4)` 与 `(4→1)` 归一。

**实测证据**：`-count=200` → 34 败（~17%，与阶段记录「1/6」吻合）；失败**全部**落在 `From`/`To`/`EdgeType` 错位断言，**`Weight` 断言 0 次失败**（Jaccard 基于已排序 token 集合，`citation` 为 `1.0/float64(len(ids))`，无求值顺序依赖）。测试的严格比较是**对的**。

- **D-01a**：**实现侧** — 构造 `sources`（及 `groups` 键）时确定性排序后再做 O(n²) 配对；最终比较器补 `Weight` 作 tie-break（`citation_cooccur` 同一 `(From,To)` 可因不同 audit 行产生不同 Weight 的重复键，`sort.Slice` 不稳定有隐患）。
- **D-01b**：**测试侧** — 改为**完整 `Edge` 结构比较**（非逐字段），并**循环 N 次调用**断言与首轮一致。
- **D-01c**：**禁止**用放宽 `InDelta` 容差或改成无序比较来「修」——那会把真实的生产非确定性洗成绿灯。`ComputeEdges` 公开承诺「同输入 → byte-identical 输出」（GRF-02 契约），而 `EdgeGraph.Neighbors` 是有向遍历，方向翻转会改变可达集与图 API 结果，属**真实功能非确定性**。
- **D-01d**：**SC-1 / REL-01 措辞须更正** —「浮点 Weight 确定性化」与真实根因不符，应改写为「**边方向确定性（消除 map 迭代序）**」。否则验证记录与事实脱节。
- **D-01e**：**验收强度** — 继承 SC-4 的「P4 连续两跑无 flake」**不足**：`-count=30` 实测 22 PASS / 8 FAIL（~27%），两跑全绿仅约 53-69% 置信度。须叠加该测试 `-count=100` 全绿 + 一次 `-shuffle=on` 作为确定性证据。

### D-02: REL-02 缓存刷新落点 = 域层刷新 + FE refetch（选项 A + FE）

**已核实事实**：`SetShaclReport` 全仓**唯一**调用点是 boot（`cmd/server/main.go:947`）；`admin_governance.go:51` 只读缓存。`RollbackTBox` 在 `internal/ontology/induction/induction.go:468` **已经跑过** `ValidateWithTrigger(..., "rollback")`，但**只消费 `ErrorCount`、丢弃报告**。因此修复本质是「把这份**已有**报告写进缓存」，**不是**新增校验。

- **D-02a**：**域层写入** — `RollbackTBox` 内拿到 `report` 后直接 `m.kg.SetShaclReport(report)`。理由：复用已跑完的那次校验（零额外成本）；报告写入与版本/指针/快照变更处于同一临界区（无 TOCTOU）；HTTP / 测试 / 未来 CLI-MCP 调用者自动一致；无 wire 变更；FE 契约不变。induction 已持有 `m.kg` 并做 Insert/Delete/Load，写缓存不跨越新边界。
- **D-02b**：**FE 必须一并修，否则 SC-2 仍不成立** — `TimelineView` 回滚后**只**调 `fetchChangelog`；`GovernanceTab` 的 report **仅在 mount 时拉取**且切换视图**不重挂** → `trigger === 'rollback'` 在界面上**仍是死分支**。须加一条既有 callback 式的 report refetch（**不加新 UI**）。
- **D-02c**：**刷新失败姿态 = WARN 续行，不 fail-closed** — Step 3-5 已把 v{N+1}/指针/版本文件落盘，此时返 500 是向客户端**谎报 rollback 未发生**。沿用既有 D-PROV-09「log WARN + return 0 violations」姿态：成功才 Set，失败保留上一份成功报告（其 `trigger=boot` + 旧 `validated_at` 如实表明时点）。
- **D-02d**：**不得**用 `atomic.Value.Store(nil)` 清缓存（会 panic）；typed-nil 会让 GET 返回 `200 null`。若未来需要真正的 fail-closed 清空语义，须先给 store 增加 Clear/sentinel API — **本 phase 不做**。
- **D-02e**：**TimelineView toast 保留原状** — 其文案是「已回滚至 vX（当前 vY）· 新增 N 条 violation」，**不含**「缓存陈旧」披露；计数来自同一次重校验，缓存修复后依然准确；删除反而损失即时反馈。
- **D-02f**：**shape 复用前提成立** — boot 与 rollback 调同一 `ValidateWithTrigger`、同一 `Report` 结构、同一 `ShapesPathsFromEnv()` 路径列表，JSON 字段集完全相同，仅 `trigger` / `validated_at` / 由图数据派生的计数不同；`types.ts:53` 已含 `'rollback'`。

### D-03: 遗留项处置记录形态 = 单文件主表（选项 1）

- **D-03a**：新建 `.planning/phases/47-debt-cleanup-reproducible-baseline/47-DEBT-DISPOSITION.md` — 逐 ID 主表（ID / 来源 / 去向（v1.7 修复 / 延期+理由 / ack backlog）/ 证据）。
- **D-03b**：`47-VERIFICATION.md` frontmatter 加**一行指针**指向该文件（沿用 Phase 44 `deferred: addressed_in/evidence` 的机械可读先例），使审计工具与人工读路径都完整。
- **D-03c**：`STATE.md` 的 Deferred Items 表**保留索引行**（每个 ID 一行）并**链到**主表 — 作为活索引，**不作为**权威证据。
- **D-03d**：**不新增** PROJECT.md Key Decisions 行，除非某条处置**确实产出约束后续工作的长期决策**（那时只写指针行）。
- **D-03e**：**不改** `.planning/milestones/` 归档（v1.7 roadmap 既有决策）。
- **D-03f**：可被 `verify-phase-47.sh` 机械校验（每个 ID 至少出现一次，且去向字段非空）。
- **先例**：`42-triage.md`、`.planning/reports/v1.4-AUDIT-OPEN-CLOSE.md`、`44-DECISIONS.md`。

### D-04: 全量门 flake 边界 = 归零边界（选项 D）

**已核实事实**：`verify-phase-46.sh` 的 `waiver_ok` 是 **WSL shell 本地机制，GitHub Actions 裸跑 `go test ./...` 完全不读它**。clean `git archive HEAD` checkout 下有**两条**确定性失败：`TestEmitSkillInstanceCount`（Skill 3 vs 8 — 因 `skills/` 只有 3 个目录被 git 跟踪）+ `TestEmitSubCategoryInstanceCount`（6 vs 7）；而现有单条 waiver **只覆盖后者**。故「只修 REL-01」连**本地绿**都换不来 **CI 绿**。

- **D-04a**：**边界 = 归零** — REL-01 + D-45-DEF-01 + clean-clone 可复现 + **空 waiver**。目标：`git archive HEAD` clean checkout 下全量绿；本地门 = CI 门；Phase 48 的公开绿灯不依赖脚本口径。
- **D-04b**：**D-45-DEF-01 的诚实修法是改断言，不是改实现** — `1aaf853` 把 `其他/` 下 6 个文件 R100 移入 `操作手册/`，目录数 7→6，而 `emitter.go` 契约「每个 sql-guides 子目录发一个 SubCategory」**是对的**。**不要**把 `want 7` 简单改成 `want 6`（仍 pin 外部语料）；应改成 **temp fixture 的合成目录**（先例：`store_emitter_conformance_test.go`）或由枚举结果推导。「改实现凑 7」等于**凭空造一个子类**，不诚实。
- **D-04c**：**`TestEmitSkillInstanceCount` 同批处理** — 其环境依赖（依赖未跟踪的 `skills/*/` 目录）与 D-45-DEF-01 同源；干净 clone 下必红。须一并 hermeticize（temp fixture / 显式 fixture 目录）。
- **D-04d**：**清理 untracked `internal/server/platform/verify_main.go`** — 该文件自 Phase 44 起 runtime-stale（pre-44 fixture、owl:Class IRIs、raw counts、label-as-schema 断言，见 D-45-DEF-02），且未被 git 跟踪 → 造成本地/CI 差异。按 D-45-DEF-02 的既有处置（已从 verify-phase-45.sh 排除）落到处置记录并清理工作树。
- **D-04e**：**验收强度** — 见 D-01e：`-count=100` 全绿 + 一次 `-shuffle=on`；SC-4 的「两跑」只作冒烟门槛。
- **D-04f**：**不采用** `t.Skip` / `WAIVED` 维持表面绿灯 — 与「公开可见绿灯」的信任目标相悖。CI 若有残余项应落成**带 issue 链接的显式记录**，而非 shell 放行。

### D-05: boot vs rollback 的 quarantine 不对称 = 记录为已知限制（不夹带进 REL-02）

- **D-05a**：boot 会按 `report.Quarantined` **真正执行**隔离（`cmd/main.go:941-945`），**rollback 不执行** → banner 的「N 个实体已隔离」与 confirm 文案的「失效引用实体将进入隔离区」在 rollback 路径上**并不属实**。
- **D-05b**：处置 = **记录为已知限制**（写入 `47-DEBT-DISPOSITION.md` 的 ack backlog 或延期+理由），**不在** REL-02 内夹带修复。理由：REL-02 的范围是「报告与重校验结果一致」，quarantine 执行语义是**独立决策**，且扩张它会改变 rollback 的副作用面（需单独评估）。
- **D-05c**：若 planner 发现该缺口会让 SC-2 的验收陈述失真，须回头向用户升级，而非自行扩张范围。

### Claude's Discretion

- `sources` / `groups` 排序的**具体比较键**（source 名 / 代表 chunk ID / 二者复合）— planner 在实现时定，只要满足「同输入 → 同输出」且方向稳定
- D-45-DEF-01 / `TestEmitSkillInstanceCount` 的 **fixture 具体形态**（合成目录 vs 提交式 fixture 目录）— planner 定，须满足干净 clone 可复现
- `47-DEBT-DISPOSITION.md` 的**表列设计**与 `verify-phase-47.sh` 的 grep 校验**具体规则**
- FE report refetch 的**触发点选择**（回滚成功后 callback / GovernanceTab 订阅 changelog 变更事件 / 视图切换时重取）— planner 定，约束是「复用既有契约、不加新 UI」
- `verify-phase-47.sh` 的**分段结构**（沿用 45/46 的 P1..P4 惯例外，需新增确定性强度段）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 47 范围与要求
- `.planning/ROADMAP.md` § `### Phase 47: 清债与可复现基线` — 4 条 Success Criteria（本 phase 的验收锚点）
- `.planning/REQUIREMENTS.md` § v1.7 Requirements — REL-01 / REL-02 / REL-04 原文 + Traceability 表（REL-04 的补录对象）
- `.planning/STATE.md` § `Deferred Items` — v1.6 遗留项与去向映射（D-46-DEF-02 → REL-01；audit WARNING-1 → REL-02；BL-01..04 → REL-04；D-45-DEF-01/02/03 + D-46-DEF-01 + WARNING-2 → Phase 47 逐项处置记录）

### 遗留项原始证据（处置记录的数据源）
- `.planning/phases/45-sparql-schema-visibility-d-sparql-05/deferred-items.md` — D-45-DEF-01（emitter `TestEmitSubCategoryInstanceCount` 6 vs 7）、D-45-DEF-02（`verify_main.go` runtime-stale）、D-45-DEF-03（46-REVIEW WR-01/WR-02 + IN-02 候选）
- `.planning/phases/46-entity-shacl-shapes/deferred-items.md` — D-46-DEF-01（`DetailPanel.test.tsx` 空态 hero 测试确定性失败 12/13）、D-46-DEF-02（`TestDeriveEdgesDeterministic` flaky）
- `.planning/milestones/v1.6-MILESTONE-AUDIT.md` — WARNING-1（rollback 后治理报告缓存不刷新）、BL-01..04（REQUIREMENTS 正文有 ID 但缺 Traceability 表行）、owl:Class quirk WARNING-2、46-REVIEW Info 级 7 项未修

### REL-01 代码锚点
- `internal/knowledge/graph_edges.go` — `ComputeEdges` / `Edge`（`Weight float64`）/ `EdgeGraph.Neighbors`（有向）；根因位于 `sources` 构造的 map 迭代序
- `internal/knowledge/graph_edges_test.go` § `TestDeriveEdgesDeterministic` — flaky 测试本体（排序键 `(From,To,EdgeType)` + `InDelta(..., 0)`）

### REL-02 代码锚点
- `internal/ontology/induction/induction.go` § `RollbackTBox` — 已跑 `ValidateWithTrigger(..., "rollback")` 但丢弃 report
- `internal/ontology/store/store.go` § `SetShaclReport` / `GetShaclReport` — 缓存 API（`atomic.Value` 语义）
- `internal/ontology/shacl/validate.go` — `Report` 结构 + `Trigger` 字段 + `Validate()`（boot）/ `ValidateWithTrigger`
- `internal/server/admin_governance.go` — `GET /governance/shacl` 读缓存（含需修正的注释）
- `internal/server/admin_ontology.go` § rollback handler — 调 `RollbackTBox` 的位置
- `cmd/server/main.go` — boot 路径的 `SetShaclReport` 唯一调用点 + quarantine 执行点
- `web/src/components/admin/governance/GovernanceTab.tsx` — `report.trigger === 'boot' ? '启动时' : '回滚后'` 渲染分支
- `web/src/components/admin/governance/TimelineView.tsx` — 回滚动作 + toast 披露
- `web/src/components/admin/governance/types.ts` — `trigger: 'boot' | 'rollback'`
- `.planning/phases/40-data-governance-shacl-provenance-tbox-versioning/40-UI-SPEC.md` — governance UI 既有契约与数据 shape（REL-02 复用前提）

### 全量门与 waiver 机制
- 仓库根目录的 `verify-phase-45.sh` / `verify-phase-46.sh`（**不在 `scripts/` 下**，与 40/41 的既有位置一致）— P1..P4 分段结构 + `waiver_ok` 本地白名单机制的现状（本 phase 要归零/清理）
- 仓库根目录的 `verify-phase-46.sh` P4 waiver 白名单 — 现状只认 D-45-DEF-01 一项
- `internal/ontology/emitter/emitter_test.go` — D-45-DEF-01 与 `TestEmitSkillInstanceCount` 的 repo-root 依赖
- `internal/ontology/store/store_emitter_conformance_test.go` — temp fixture 合成目录的先例

### 处置记录形态先例
- `.planning/phases/42-v1-4-outstanding-closure/42-triage.md` — per-phase 单文件主表（Item / Bucket / Closure path）
- `.planning/reports/v1.4-AUDIT-OPEN-CLOSE.md` — milestone 级主表汇总先例
- `.planning/phases/44-sparql-test-fix/44-DECISIONS.md` — 「去向 + 证据」固化先例

### 项目级约束
- `.planning/PROJECT.md` § `Key Decisions` — 含 CGO_ENABLED=1 现实路径、文档与构建现实一致等约束
- `.planning/STATE.md` § `Blockers/Concerns` — 内网服务间歇断（Milvus/BGE/GLM）→ live verify 沿用「探测先行、断线记 env-SKIP」惯例

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/knowledge/graph_edges.go::ComputeEdges` — 待确定性化的实现本体；`Edge` 结构可整体比较（测试加固的落点）
- `internal/ontology/shacl/validate.go::ValidateWithTrigger` — rollback 路径**已在调用**，只需消费返回值而非 `ErrorCount`
- `internal/ontology/store/store.go::SetShaclReport` — 现成写入 API，无需新增
- `internal/ontology/store/store_emitter_conformance_test.go` — temp fixture 合成目录的**直接先例**（D-04b/04c 复用）
- `.planning/phases/42-v1-4-outstanding-closure/42-triage.md` — 单文件主表**直接先例**（D-03a 复用其表结构）

### Established Patterns
- per-phase `verify-phase-NN.sh` 用 P1..P4 分段 + `waiver_ok` 白名单容忍既有失败 — 本 phase 的整改对象（D-04a 归零）
- per-phase `deferred-items.md` 是**发现时**的原始记录；处置去向应落到**独立主表**并保留索引（D-03）
- GSD 审计链：`CONTEXT → PLAN → SUMMARY → VERIFICATION` + frontmatter 指针（D-03b 沿用 Phase 44 先例）
- 变更日志 `O_APPEND`-only / INSERT-only 审计不变量 — 本 phase 不触碰

### Integration Points
- REL-01 只触 `internal/knowledge`（`graph_edges.go` + 其测试）— 不跨包
- REL-02 触 `internal/ontology/induction`（写缓存）+ `web/src/components/admin/governance`（refetch）— 不新增 UI 组件、不改 wire shape
- D-04 触 `internal/ontology/emitter`（测试 hermeticize）+ 仓库根目录（新建 `verify-phase-47.sh`，与 40/41/45/46 一致；清理 45/46 的 waiver 引用）
- REL-04 只触 `.planning/REQUIREMENTS.md`（Traceability 补录）— 不改 `.planning/milestones/` 归档

</code_context>

<specifics>
## Specific Ideas

- 用户全程采纳 4 项推荐，对「**不把真实缺陷洗成绿灯**」这条标准反应一致 — D-01c（禁止放宽容差）与 D-04f（禁止 t.Skip/WAIVED）是同一原则的两处落地，planner 不应再软化
- 用户对「**本地门 = CI 门**」有明确诉求（Phase 48 要对外公开绿灯）— 这是 D-04a 选「归零」而非「waiver registry」的决定性理由
- 用户接受「**根因更正**」这类与既有文档冲突的结论（D-01d 允许改写 SC-1/REL-01 措辞）— planner 不必为了与旧措辞一致而迁就错误表述
- 用户认可「**域层拥有域内状态**」的分层原则（D-02a 选 A 而非 HTTP 层）

</specifics>

<deferred>
## Deferred Ideas

- **rollback 的 quarantine 执行语义**（boot 会隔离、rollback 不隔离 → banner/confirm 文案在 rollback 路径失真）— 按 D-05 记录为已知限制，不夹带进 REL-02；若后续要让 rollback 也执行隔离，须单独立项（会改变 rollback 副作用面）
- **SHACL 缓存清空 / fail-closed 语义**（`atomic.Value.Store(nil)` panic 问题）— D-02d 明确本 phase 不新增 Clear/sentinel API；若未来需要「刷新失败即让 GET 返 503」，须单独立项
- **`internal/server/platform/verify_main.go` 的重写**（自 Phase 44 起 runtime-stale）— D-04d 只做「记录 + 清理工作树」，重写该 harness 若被需要，属独立任务
- **46-REVIEW Info 级 7 项未修**（`--fix` 范围仅 Critical+Warning）— 归入 `47-DEBT-DISPOSITION.md` 的 ack backlog 或延期+理由，不在本 phase 修复
- **D-46-DEF-01（`DetailPanel.test.tsx` 空态 hero 测试）** — FE 侧确定性失败，不在 Go P4 门内；归入处置记录。若 Phase 48 CI 跑 FE 线则该测试会红，planner 须评估是否纳入 D-04 的归零边界或显式记录

</deferred>

---

*Phase: 47-debt-cleanup-reproducible-baseline*
*Context gathered: 2026-10-19 via discuss-phase (advisor mode; 4 gray areas researched in parallel, all 4 recommendations adopted)*
