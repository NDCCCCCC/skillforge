---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
plan: 03
subsystem: integration/contract (repo-level lint gate) + docs (version narrative alignment)
tags: [lint-ban, test-as-gate, legacy-interrupt, sc-3, docs-alignment, eino-v0.9.14]
requires:
  - "integration/contract drift suite + findModuleRoot helper (contract_test.go:484, Phase 1)"
  - "eino v0.9.14 dependency base + TestContractVersionPin new-pin assertion (Phase 8 / Plan 08-01, commit 36845c0)"
provides:
  - "SC-3 repo 级 legacy interrupt 禁令门 TestNoLegacyInterruptAPI —— deprecated 无地址 API（NewInterruptAndRerunErr / compose.InterruptAndRerun / deprecatedInterruptAndRerun）在任意 .go 文件（含 _test.go）出现即 contract 套件 fail，带触发负向证明"
  - "版本叙事对齐：REQUIREMENTS/ROADMAP/CLAUDE.md 现行目标统一 v0.9.14（用户决策 2026-08-17 注记）；PLAT-05/SC-4 注册 API 措辞修正为 schema.RegisterName[T](name)"
affects:
  - "Phase 11 HITL（写工具代码被禁令门防串台——resume 定位只能走 v0.9 地址语义）"
  - "Phase 10/12/13 全部 interrupt 消费面（预防性禁令）"
tech-stack:
  added: []
  patterns:
    - "test-as-gate repo 级禁令（WalkDir 全仓扫描 + SkipDir 排除非产品路径 + 自身文件排除防自匹配）——与 TestContractVersionPin 同层先例的第二实例"
    - "负向证明协议：注入探针 → 断言 FAIL 点名 file:line → 删除探针 → 断言 PASS（证据入 SUMMARY）"
key-files:
  created:
    - integration/contract/no_legacy_interrupt_test.go
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md
    - CLAUDE.md
decisions:
  - "D8-04: 禁令门独立成门不并入 TestContractRunsUnder10s 聚合（planner 裁决，contract_test.go 零改动）；扫描含 _test.go，排除 .claude/web/.git/node_modules/.planning/screenshots + 门自身文件；WrapInterruptAndRerunIfNeeded 不禁（官方迁移辅助）"
  - "D8-05: 版本叙事对齐 = 三文件定点改 v0.9.14 + 双处决策注记（PLAT-04 覆盖研究期 / Goal 而非研究原锁）；CLAUDE.md:203/:227 历史研究出处与 STATE.md、phase 目录名一律不动（身份锚点）"
metrics:
  duration: 6m
  completed: 2026-08-17
---

# Phase 8 Plan 03: SC-3 legacy interrupt 禁令门 + 文档 v0.9.14 对齐 Summary

**One-liner:** repo 级 test-as-gate 禁令门（WalkDir 扫全仓 .go 文件，三标识符命中即 Fatalf 点名 file:line，负向证明探针注入 FAIL→删除 PASS）+ 一个纯文档提交把 REQUIREMENTS/ROADMAP/CLAUDE.md 的 v0.9.13/v0.7.7 现状表述对齐 v0.9.14 用户决策、并把 PLAT-05/SC-4 注册 API 修正为 schema.RegisterName[T](name)。

## What Was Built

### Task 1: legacy interrupt 禁令门（commit `fea6114`）

新建 `integration/contract/no_legacy_interrupt_test.go`（124 行）：

- **构建形态**：license header 1-15 行照 contract_test.go 逐字（SPDX:15）+ `//go:build contract`（:17）+ `package contract`（:19，同包复用 helper）——首 20 行三要素齐全
- **TestNoLegacyInterruptAPI**：`filepath.WalkDir` 从既有 `findModuleRoot(t)`（contract_test.go:484）出发扫全仓 `.go` 文件；目录排除 `.claude`（含 worktrees 自带 go.mod）/ `web` / `.git` / `node_modules` / `.planning` / `screenshots`（`filepath.SkipDir`）；文件级排除门自身（`no_legacy_interrupt_test.go` 必然含禁令字面量）；逐行 `strings.Contains` 匹配，命中即 `t.Fatalf("%s:%d 使用了被禁的 legacy interrupt API: %s — v0.9 地址语义 Interrupt/Resume 唯一（SC-3 / Pitfall 5）...")`
- **禁令清单**（RESEARCH Pitfall 5，来自 eino@v0.9.14 compose/interrupt.go:44-117）：`NewInterruptAndRerunErr`（deprecated func）/ `compose.InterruptAndRerun`（deprecated sentinel var）/ `deprecatedInterruptAndRerun`（内部名防绕行）。`WrapInterruptAndRerunIfNeeded` 仅在注释说明不禁原因（官方迁移辅助），不进断言逻辑
- **_test.go 在扫描内**：未来测试代码误用同样 fail；不并入 `TestContractRunsUnder10s` 聚合（planner 裁决，contract_test.go 零改动）
- **绿态**：`go test ./integration/contract -tags=contract -run TestNoLegacyInterruptAPI -v -count=1` → `--- PASS (0.03s)`（执行前独立 grep 预核全仓零命中）

### 负向证明（T-8-07 强制——门必须证明自己会咬人）

在 repo 根临时创建 `zz_legacy_gate_probe_test.go`（`//go:build ignore` + 一行注释含 `compose.NewInterruptAndRerunErr` 字样）：

```
=== RUN   TestNoLegacyInterruptAPI
    no_legacy_interrupt_test.go:114: zz_legacy_gate_probe_test.go:8 使用了被禁的 legacy interrupt API: NewInterruptAndRerunErr — v0.9 地址语义 Interrupt/Resume 唯一（SC-3 / Pitfall 5）；迁移用官方 WrapInterruptAndRerunIfNeeded 或地址式 tool.Interrupt / compose.Interrupt
--- FAIL: TestNoLegacyInterruptAPI (0.03s)
```

删除探针后复跑 → `--- PASS: TestNoLegacyInterruptAPI (0.04s)`。工作树无残留（`ls zz_legacy_gate_probe_test.go` → No such file or directory）。

### Task 2: 文档版本对齐（commit `996c33a`，纯文档 3 文件 +9/−9）

**(1) REQUIREMENTS.md**：
- PLAT-04：「eino v0.7.7 → v0.9.13 升级」→「eino v0.7.7 → v0.9.14 升级（用户决策 2026-08-17：v0.9.14 覆盖研究期 v0.9.13；不用 v0.10.0-alpha.x）」，其余逐字不动
- PLAT-05：`gob.RegisterName` → `schema.RegisterName[T](name)`（其内部完成 gob.RegisterName 双注册——RESEARCH Pitfall 1 修正）

**(2) ROADMAP.md Phase 8 节**：milestone 列表行（:32）/ Phase 8 标题（:224）/ SC-1（:231）v0.9.13 → v0.9.14；Goal（:226）改 v0.9.14 并行尾追加「（用户决策 2026-08-17：v0.9.14 而非研究原锁的 v0.9.13）」；SC-4（:234）`gob.RegisterName` → `schema.RegisterName[T](name)`；:237 Research flag 行已含 RF-2 已解决注记（无需补写）。Plan list / wave 结构行保持 08-01/08-02 提交状态未回退。

**(3) CLAUDE.md**：栈表 Agent framework 版本格（:27）→「v0.9.14（locked 2026-08-17；v0.10.0-alpha.x 明确不用）」；:52「**v0.7.7** is the latest tagged release」→「**v0.9.14** is the current locked release（2026-08-13 发布，升级决策 2026-08-17）」。:203/:227 历史研究出处记录逐字未动（`git diff CLAUDE.md | grep -c "^+.*latest release tag v0.7.7"` = 0）。

## Must-Haves Truths 验证

| Truth | 状态 | 证据 |
|-------|------|------|
| legacy interrupt API 在代码库出现即测试 fail——双语义不并存 | ✅ | 负向证明：探针注入 → FAIL 点名 `zz_legacy_gate_probe_test.go:8` + 标识符 `NewInterruptAndRerunErr` |
| 禁令门真实可触发（负向证明），移除后恢复绿 | ✅ | FAIL（0.03s）→ 删除探针 → PASS（0.04s），输出片段留存上文 |
| 版本叙事文档不再说谎：三文档现状表述对齐 v0.9.14 | ✅ | REQ v0.9.14 ×1 / ROADMAP v0.9.14 ×7 / CLAUDE v0.9.14 ×2；残留 v0.9.13 仅 2 处且均为决策注记上下文（覆盖研究期/而非研究原锁），过滤门 exit=1（无违规残留） |

## Verification（plan `<verification>` 三项）

| # | 命令 | 结果 |
|---|------|------|
| 1 | `go test ./integration/contract -tags=contract -count=1 -timeout 10s -v` | 全绿 6.306s（VersionPin / WireFormat / MilvusFilter / StructuredOutputSchema / CitationGrounding / RunsUnder10s 0.12s / FullSpikeSuite / **TestNoLegacyInterruptAPI 0.03s**；milvus/e2e 既有 SKIP 按设计） |
| 2 | `go build ./... && go vet ./...` | exit 0（gofmt -l 空） |
| 3 | 文档 grep 门（Task 2 acceptance 全列） | 全过（见 Must-Haves 表 + counts：REQ schema.RegisterName ×1 / ROADMAP ×2） |

## Deviations from Plan

**1. [微偏差 - 探针文件加固] 负向证明探针加 `//go:build ignore` + `package main`**
- **Found during:** Task 1 触发证明
- **Issue:** plan 只要求「一行注释含禁令字样」；裸无 package 子句的 .go 文件若被并发 go 命令解析会报错。
- **Fix:** 探针加 `//go:build ignore` + `package main` 保护，禁令字面量在注释行、被门命中语义不变（FAIL 输出证明等效）。
- **Files modified:** 无入库文件（探针已删除，未进任何提交）。

其余：plan executed exactly as written（两提交各自原子；禁令门未并入 RunsUnder10s 聚合按 planner 裁决；:237 无需补写）。

## 提交卫生（T-8-07 / Pitfall 6 延续）

- `git show --stat fea6114` = 仅 `integration/contract/no_legacy_interrupt_test.go`（+124，新建）；`git show --stat 996c33a` = 恰好 3 个文档文件（+9/−9），零代码文件
- 零文件删除（diff-filter=D 空）；untracked `.claude/`、`scripts/test`（含 live key）、`screenshots/`、`du.exe.stackdump`、`07-REVIEW.md` 未入任何提交
- 零新依赖（标准库 io/fs / os / path/filepath / strings / testing——T-8-SC accept 面维持）

## Deferred（out-of-scope 发现，已记录不修）

contract 套件 10s 包级 timeout 在**冷缓存**首跑时可能超时（TestContractWireFormat 内 buildSpiked 首次编译 ~4-10s；warm cache 全套 6.3s 绿）。本 plan 未触碰 contract_test.go，属既有套件特性——记录于 `.planning/phases/08-p8-0-eino-v0-7-7-v0-9-13/deferred-items.md`，不在本 plan 修。

## Auth Gates

None — 纯 hermetic（文件扫描 + 文档编辑），无 live stack 依赖。

## Known Stubs

None — 无 UI / 无占位数据。

## Threat Flags

None — 无新增安全面。威胁登记表 T-8-07（假绿防护：负向证明 + 显式排除清单）、T-8-08（resume 串台防护：三标识符 + _test.go 在扫描内）全部按 plan 落实。

## Requirements Mark-Complete 决策

plan frontmatter `requirements: [PLAT-04]` **不 mark-complete**：PLAT-04 覆盖整个 Phase 8（升级 ✅ + lint 禁令 ✅ + 冻结 eval 基线/live verify 重锁 = 08-05 human-verify 未完成）。与 08-01（同 frontmatter）处置一致——REQUIREMENTS 勾选留待 08-05 收口。

## Self-Check: PASSED

- 文件存在性：integration/contract/no_legacy_interrupt_test.go（git tracked @ fea6114）/ .planning/REQUIREMENTS.md / .planning/ROADMAP.md / CLAUDE.md（@ 996c33a）— 全部 FOUND
- 提交存在性：`fea6114`（禁令门）+ `996c33a`（文档对齐）FOUND（git log 验证）
- 三项 verification 输出留存于本文件「Verification」表；负向证明 FAIL 片段留存于「负向证明」节
