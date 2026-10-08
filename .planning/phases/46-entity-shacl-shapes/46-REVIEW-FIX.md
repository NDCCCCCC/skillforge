---
phase: 46-entity-shacl-shapes
fixed_at: 2026-10-08T05:35:00Z
review_path: .planning/phases/46-entity-shacl-shapes/46-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 46: Code Review Fix Report

**Fixed at:** 2026-10-08T05:35:00Z
**Source review:** .planning/phases/46-entity-shacl-shapes/46-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 9（fix_scope=critical_warning：CR-01 + WR-01..WR-08；IN-01..IN-07 不在本次范围）
- Fixed: 9
- Skipped: 0

所有修复在隔离 worktree（branch `gsd-reviewfix/46-1862`）完成并逐项原子提交；每项经过 Tier 1 重读 + Tier 2 编译/测试验证（Go 走 WSL CGO 路径，FE 走 Windows 侧 tsc + vitest）。

## Fixed Issues

### CR-01: RollbackTBox 清空整个 KG（含 ABox）且通过快照持久化删除

**Files modified:** `internal/ontology/induction/induction.go`, `internal/ontology/induction/induction_test.go`
**Commit:** 2682982
**Status:** fixed: requires human verification（逻辑级变更）
**Applied fix:** 采用 REVIEW 建议的最小修复方向。新增 Step 1b：覆盖指针文件**之前**用一次性 store（复用 `LoadFromTurtle` 的同一 Turtle 解析器，无文本级反解析）解析当前 `skillforge_tbox.ttl`，取旧 TBox subject 集合；Step 6 只对这些 subject 做 `DeleteSubject`（ABox triple 与隔离区 triple 天然存活）；Step 7 照旧 `LoadFromTurtle(newTTL)`。同时修复 changelog 诚实性缺口：`What.Added/Removed` 从硬编码空列表改为新旧 TBox subject 集合的真实 diff（排序后写入）。红线遵守：Step 8 快照、Step 9 `fireChange`、Step 11 SHACL 重验时序全部未动；`DeleteSubject` 只动主索引，不触碰 quarantined map。回归钉 `TestRollbackPreservesABox`（ABox 存活 + 被删 TBox subject 消失 + changelog Removed 如实报告）。

### WR-01: Approve 吞掉当前 TBox 读取错误

**Files modified:** `internal/ontology/induction/induction.go`
**Commit:** fe6d8ef
**Applied fix:** 按 REVIEW 建议区分 `os.IsNotExist`（全新部署，base=""）与其他读错误（返回 error，拒绝 approve，任何文件写入之前）。杜绝读错误下 `newTTL` 退化为仅含新提案、指针覆盖截断整个既有 TBox 的路径。

### WR-02: Approve 部分失败语义混乱

**Files modified:** `internal/ontology/induction/induction.go`
**Commit:** 6a801fb
**Status:** fixed: requires human verification（事务序变更）
**Applied fix:** 采用 REVIEW 的务实修复序：先 `persistLocked()` 固化 approved 状态（失败则回滚内存状态翻转，proposal 保持 pending，重试干净）→ 再写 v{N+1}/指针/版本文件（每步失败信息标注「已完成到哪一步」，含 ApprovedTriples 重放提示）→ 最后 KG 插入改 `InsertUnique`（部分失败后重试 approve 不再产生重复三元组）。`fireChange` 保持在全部成功后触发。

### WR-03: parseTripleLine 以 " a " 分割产生垃圾三元组

**Files modified:** `internal/ontology/induction/induction.go`, `internal/ontology/induction/induction_test.go`
**Commit:** f6a3aca
**Applied fix:** `parseTripleLine` 入口对 `rdfs:label ` 前缀直接跳过（REVIEW 建议的第二方案；未做 proposedTriples 结构化返回的接口变更，保持改动面最小）。回归钉：单测 `TestParseTripleLineSkipsLabelLine` + 端到端 `TestApproveDescriptionContainingAWordInsertsNoGarbage`（descEN 含 " a " 时 KG 中该 IRI 恰好 1 条 type triple）。

### WR-04: previewIRILocked 前缀子串误报 IRI 冲突

**Files modified:** `internal/ontology/induction/induction.go`, `internal/ontology/induction/induction_test.go`
**Commit:** d8e449e
**Applied fix:** 新增 `containsStandaloneToken`：needle 两侧字节必须不是名字延续字符（字母/数字/`_`/`-`/`:`，左界额外排除 `:` 以拒绝 `myskf:` 类更长前缀）。`skf:` 短探针与全 IRI 探针都走该函数——修掉 `…#Note` 误匹配 `…#NoteChunk` 的单向误报。回归钉 `TestPreviewIRINoPrefixSubstringFalsePositive`（Note 空闲 + NoteChunk/NoteChunkProperty 双形式仍冲突）。

### WR-05: 8 个 Global* shape 桶只写不读

**Files modified:** `internal/ontology/shacl/validate.go`, `internal/ontology/shacl/shapes.ttl`
**Commit:** 2069e2c
**Applied fix:** 采用 REVIEW 的方案 (b)（暂不实现通用执行，删除死代码并如实声明）：删除 `parsedShapes` 的 8 个 Global* 字段、`parseShapes` 的独立 PropertyShape 收集循环、合并循环的 8 行 append、`checkGlobalReferences` 的未用 `parsed` 参数；`checkGlobalReferences` 保持既有硬编码行为（唯一被执行的全局规则）并补文档注释；shapes.ttl GLOBAL PROPERTY SHAPES 段落加披露注释（仅 referencesTable 受引擎执行，其余为 declaration-only，勿依赖全局强制）。行为零变化（桶本来就没有消费点），「shapes 即数据」契约的全局层缺口从「静默」变为「显式声明」。方案 (a) 涉及激活全局约束（默认 error 级 → 隔离），属语义变更，不应由 review fix 顺手引入。

### WR-06: verify-phase-46.sh P2/P4 丢弃退出码

**Files modified:** `verify-phase-46.sh`
**Commit:** b188de8
**Applied fix:** P2/P4 捕获退出码并作为权威判据：空日志直接 fail（基建故障无证据即判绿的反模式）；rc≠0 时仅当 `waiver_ok` 确认唯一 `--- FAIL` 集合恰为 D-45-DEF-01 文档化测试才放行（走 WAIVED 分支并打印 rc）。与 P3 的 exit-code 纪律对齐。**已用完整 gate 对 worktree 实测：P1/P2/P3/P4 全部 PASS、gate exit 0，waiver 路径按预期工作。**

### WR-07: 治理页重试成功后不清除 shaclError

**Files modified:** `web/src/components/admin/governance/GovernanceTab.tsx`
**Commit:** 3ad0854
**Applied fix:** `onRetry` 成功回调先 `setShaclError(null)` 再 `setShaclReport(r)`（REVIEW 建议原样落地）。失败分支行为不变。验证：worktree web 下 `npx tsc --noEmit` 通过、`npx vitest run src/components/admin/governance` 13/13 通过。

### WR-08: Generate 双重 persistLocked

**Files modified:** `internal/ontology/induction/induction.go`
**Commit:** 90098cb
**Applied fix:** 删除 append 前的第一次 `persistLocked()`，只保留 append 后那次（其错误即 Generate 的错误），消除「next_id 已推进但提案缺失」的中间态落盘窗口与冗余 IO。

## Skipped Issues

None — 全部 9 项在范围内发现均已修复。

（IN-01..IN-07 按 `fix_scope: critical_warning` 不在本次修复范围，未处理、未提交。）

## 需要人工确认的两处逻辑级变更

1. **CR-01（2682982）**：回滚语义从「整库清空重建」改为「仅替换 TBox 层」。这是 REVIEW 判定的正确语义（D-LOG-03 未授权删除 ABox），但属于数据面行为变更，建议人工复核 `TestRollbackPreservesABox` 的断言是否符合预期。
2. **WR-02（6a801fb）**：approve 的失败窗口从「文件已改、状态未持久化」重组为「状态先固化、文件后写」。非事务前提下的最优序（REVIEW 同方向建议），但各失败窗口的恢复语义建议人工过目。

## 评审范围外的观察（未修，供排期参考）

- **pre-existing 闪烁测试**：`internal/knowledge` 的 `TestDeriveEdgesDeterministic` 以约 1/6 概率在**未含本修复的主仓库**上随机失败（两次 `ComputeEdges` 调用的浮点 Weight 精确比较，进程内非确定性）。它会让 P4（以及修复前的旧判据）随机红灯。非 Phase 46 引入、非本批修复引入，建议单独立项修复（让 ComputeEdges 的两次调用确定性化或测试放宽 delta）。
- **TestEmitSkillInstanceCount 是环境依赖测试**：emitter 扫描 `{repo}/skills/` 计数，worktree 只有 3 个 tracked skill，主仓库有 8 个（5 个 untracked）。在 worktree 里跑 gate 需先镜像 skills 目录；主仓库不受影响。
- 本次 gate 实测 P3 = 22 `--- PASS`（含新增的 3 个回归钉），远超 ≥7 下限。

---

_Fixed: 2026-10-08T05:35:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
