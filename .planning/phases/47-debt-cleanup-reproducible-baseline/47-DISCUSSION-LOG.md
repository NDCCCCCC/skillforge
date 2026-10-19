# Phase 47: 清债与可复现基线 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-19
**Phase:** 47-debt-cleanup-reproducible-baseline
**Areas discussed:** REL-01 flaky 修复落点, REL-02 缓存刷新落点, 遗留项处置记录形态, 全量门 flake 边界
**Mode:** advisor（4 个灰区并行研究 → 合成对比表 → 表格优先选择）

---

## REL-01 flaky 修复落点

| Option | Description | Selected |
|--------|-------------|----------|
| A. 只改测试：放宽容差 + 排序键补 Weight | 改动最小、不碰生产码；但容差/排序键都不作用于方向翻转 → SC-1 不成立（`-count=200` 仍败） | |
| B. 只改测试：对 `keyword_cooccur` 做无序比较 | 测试变绿但生产仍违反 GRF-02「同输入→byte-identical」契约，有向 `Neighbors` 每次可能换向 → 把真实缺陷洗成绿灯 | |
| C. 只改实现：`sources` 确定性排序 | 直击根因、~4 行、恢复 GRF-02 契约；但不新增回归防线 | |
| D. 实现确定化 + 测试加固 | 根因修复 + 全字段 `Edge` 比较 + 循环 N 次断言一致；直接支撑 Phase 48 CI 可信度 | ✓ |

**User's choice:** D（实现 + 测试）
**Notes:** 无追加说明。研究阶段的实测证据（`-count=200` → 34 败，失败全落在 `From`/`To`/`EdgeType`，Weight 断言 0 次失败）推翻了阶段文档原有的「浮点 Weight 确定性化」根因表述，用户接受该更正。

**研究阶段的关键纠正（进入 CONTEXT 的 D-01）：**
- 真实根因 = `internal/knowledge/graph_edges.go` 中 `sourceTokens` 的 map 迭代序 → `keyword_cooccur` 边方向随机翻转
- 测试的严格比较是**对的**，不应放宽
- SC-1 / REL-01 措辞须从「浮点 Weight 确定性化」改为「边方向确定性（消除 map 迭代序）」
- 验收强度：两跑不足（`-count=30` 实测 ~27% 失败率），须 `-count=100` + `-shuffle=on`

---

## REL-02 缓存刷新落点

| Option | Description | Selected |
|--------|-------------|----------|
| A. 域层刷新：`RollbackTBox` 内 `SetShaclReport` | 复用已跑的那次校验（零额外成本）；与版本/指针/快照变更同临界区（无 TOCTOU）；所有调用者自动一致 | ✓ |
| B. HTTP 层刷新：handler 再跑一次校验 | 同请求跑两次完整校验（成本翻倍）；两次结果可因并发 approve 分叉 → 响应计数与缓存不一致 | |
| C. 域层产出 / HTTP 写入（改签名返回 `*shacl.Report`） | 只校验一次；但破坏性签名变更（含 3 处测试）+ shacl 类型泄漏进契约 + 非 HTTP 入口仍漏写 | |
| D. 两者都做 | 双写/双校验、`validated_at` 可能不同；掩盖「谁拥有缓存」的设计问题 | |

**User's choice:** A + FE refetch
**Notes:** 用户接受研究带出的**附加必要条件**——仅修后端缓存不足以让 SC-2 成立。`TimelineView` 回滚后只调 `fetchChangelog`，而 `GovernanceTab` 的 report 只在 mount 时拉取且切视图不重挂，故 `trigger === 'rollback'` 在界面上仍是死分支；须同时加一条既有 callback 式 report refetch（不加新 UI）。

**研究阶段的关键核实（进入 CONTEXT 的 D-02）：**
- `SetShaclReport` 全仓唯一调用点确为 boot（`cmd/server/main.go:947`）；`RollbackTBox` 在 `induction.go:468` **已经跑过** `ValidateWithTrigger(..., "rollback")` 但只消费 `ErrorCount`、丢弃报告 → 修复本质是「写入已有报告」，非新增校验
- 刷新失败姿态 = WARN 续行（Step 3-5 已把 v{N+1}/指针/版本文件落盘，返 500 是谎报 rollback 未发生）
- `atomic.Value.Store(nil)` 会 panic → 「清缓存」需新增 API，本 phase 不做
- TimelineView toast **保留原状**（其文案不含「缓存陈旧」披露，计数仍准确）

---

## 遗留项处置记录形态

| Option | Description | Selected |
|--------|-------------|----------|
| 单文件 `47-DEBT-DISPOSITION.md` + VERIFICATION frontmatter 指针行 | 每个 ID 唯一权威位置；一张表看完全部去向；路径稳定可锚点引用；可被脚本机械校验；有 `42-triage.md` / `v1.4-AUDIT-OPEN-CLOSE.md` 先例 | ✓ |
| CONTEXT.md + VERIFICATION.md 内嵌处置节 | 不新增文件；但 CONTEXT 在执行前写成、不知道最终去向；真值分散两处易不同步 | |
| PROJECT.md Key Decisions 逐项追加 | 全局可见、跨 phase 引用稳定；但 Key Decisions 语义是「约束后续工作的决策」，backlog 级处置不是决策 | |
| STATE.md Deferred 表逐 ID 展开 | 复用既有跨 milestone 账本；但无证据列、milestone 切换时整表被替换（非稳定引用） | |

**User's choice:** 单文件主表
**Notes:** 无追加说明。推荐组合 = `47-DEBT-DISPOSITION.md` 作权威主表 + `47-VERIFICATION.md` frontmatter 一行指针 + `STATE.md` 保留索引行（链过去）。

---

## 全量门 flake 边界

| Option | Description | Selected |
|--------|-------------|----------|
| A. 最小边界（只修 REL-01） | 改动最小；但本地 P4 仍 1 条确定 FAIL，waiver 不进 CI → Phase 48 必红；clean clone 另有 1 条红 | |
| B. A + 修 D-45-DEF-01（waiver 仍内联） | 本地 P4 归零；但 clean clone 的 `TestEmitSkillInstanceCount` 未解 → CI 仍红；waiver 散落 3 个脚本无 owner/期限 | |
| C. B + waiver 单一 registry | waiver 唯一真源、可审计、到期强制复核；但仍有公开已知红灯，Phase 48 观感差 | |
| D. 归零边界：REL-01 + D-45-DEF-01 + clean-clone 可复现 + 空 waiver | `git archive HEAD` 下全量绿；本地门 = CI 门；Phase 48 公开绿灯不依赖脚本口径 | ✓ |

**User's choice:** D（归零边界）
**Notes:** 无追加说明。决定性事实：`verify-phase-46.sh` 的 `waiver_ok` 是 WSL shell 本地机制，**GitHub Actions 裸跑 `go test ./...` 完全不读它**；clean `git archive HEAD` 下有**两条**确定性失败（`TestEmitSkillInstanceCount` Skill 3 vs 8 + `TestEmitSubCategoryInstanceCount` 6 vs 7），而现有单条 waiver 只覆盖后者。

**研究阶段的关键核实（进入 CONTEXT 的 D-04）：**
- D-45-DEF-01 的**诚实修法是改断言**（temp fixture 合成目录，先例 `store_emitter_conformance_test.go`），**不是**改实现凑 7，也不是把 `want 7` 改成 `want 6`（仍 pin 外部语料）
- `TestEmitSkillInstanceCount` 同源（依赖未跟踪的 `skills/*/`），须一并 hermeticize
- untracked `internal/server/platform/verify_main.go` 造成本地/CI 差异，按 D-45-DEF-02 记录 + 清理
- **不采用** `t.Skip` / `WAIVED` 维持表面绿灯

---

## Claude's Discretion

- `sources` / `groups` 排序的**具体比较键**（source 名 / 代表 chunk ID / 二者复合）
- D-45-DEF-01 与 `TestEmitSkillInstanceCount` 的 **fixture 具体形态**（合成目录 vs 提交式 fixture 目录）
- `47-DEBT-DISPOSITION.md` 的**表列设计**与 `verify-phase-47.sh` 的 grep 校验**具体规则**
- FE report refetch 的**触发点选择**（回滚成功后 callback / 订阅 changelog 变更 / 视图切换时重取）
- `verify-phase-47.sh` 的**分段结构**（沿用 P1..P4 惯例 + 新增确定性强度段）

## Deferred Ideas

- **rollback 的 quarantine 执行语义**（boot 会隔离、rollback 不隔离 → banner/confirm 文案在 rollback 路径失真）— 记录为已知限制，不夹带进 REL-02
- **SHACL 缓存清空 / fail-closed 语义**（`atomic.Value.Store(nil)` panic 问题）— 本 phase 不新增 Clear/sentinel API
- **`internal/server/platform/verify_main.go` 的重写**（自 Phase 44 起 runtime-stale）— 只做记录 + 清理工作树
- **46-REVIEW Info 级 7 项未修** — 归入 ack backlog 或延期+理由
- **D-46-DEF-01（`DetailPanel.test.tsx` 空态 hero 测试）** — FE 侧确定性失败，不在 Go P4 门内；若 Phase 48 跑 FE 线则会红，planner 须评估是否纳入归零边界
