---
phase: 08-p8-0-eino-v0-7-7-v0-9-13
plan: 01
subsystem: build-toolchain (Go module upgrade) + integration/contract (version drift gate)
tags: [dependency-upgrade, eino, go-mod, contract-test, regression-lock]
requires:
  - "go.mod pin eino v0.7.7 + eino-ext openai v0.1.5 (Phase 1, Plan 01-01)"
  - "integration/contract drift suite (SC-5, Phase 1)"
  - "internal/server TestCancelPropagationMidTool (SC-8 / AGNT-04 hermetic gate)"
provides:
  - "eino v0.9.14 + eino-ext openai v0.1.13 dependency base for ALL Phase 8-13 plans (08-02 sqlite CheckPointStore / 08-04 中断往返 / Phase 10 summarization middleware / Phase 11 HITL)"
  - "TestContractVersionPin 新断言（v0.9.14 / v0.1.13）——后续任何依赖漂移在此 fail-fast"
  - "8 处 godoc file:line 引用已对齐 v0.9.14 module cache 真实坐标（后续 phase 溯源源码不会看错位置）"
affects:
  - "全部 26 个 import eino 的 Go 文件（编译面，零源码改动实证）"
  - "Phase 9-13 全部后续 plan（站在新依赖地基上）"
tech-stack:
  added: []
  patterns:
    - "升级原子提交模式（Pattern 3）：go.mod/go.sum + 按设计变红的断言改写 + godoc 版本标注，零功能代码"
    - "module cache file:line 重核流程：$(go env GOMODCACHE)/github.com/cloudwego/eino@v0.9.14/ 下逐处 grep 定位"
key-files:
  created: []
  modified:
    - go.mod
    - go.sum
    - integration/contract/contract_test.go
    - internal/agent/agent.go
    - internal/agent/callbacks.go
    - internal/agent/retrieve_tool.go
    - scripts/spiked/e2e.go
    - scripts/spiked/wire.go
decisions:
  - "D8-01: 升级目标 v0.9.14（用户 2026-08-17 决策，覆盖研究期 v0.9.13；v0.10.0-alpha.x 明确不用）——TestContractVersionPin 断言锁 v0.9.14 / v0.1.13"
  - "D8-02: hermetic 回归四道门在 v0.9.14 下全绿且零修复提交——RF-2「编译同构」扩展为「行为同构」（本 plan hermetic 面实证；live 面 = 08-05 checkpoint）"
metrics:
  duration: 6m
  completed: 2026-08-17
---

# Phase 8 Plan 01: eino v0.7.7 → v0.9.14 主升级 Summary

**One-liner:** eino v0.9.14 + eino-ext openai v0.1.13 原子升级提交（零功能代码），TestContractVersionPin 重锁新 pin，8 处 godoc 引用按 module cache 重核，hermetic 回归四道门（build/vet/全量单测/contract 套件/cancel <200ms）全绿且零修复提交。

## What Was Built

### Task 1: 升级主提交（commit `36845c0`）

三步走，一个原子提交，恰好 8 个文件（`git show --stat` 与 plan 规定路径逐一致，零删除）：

**(1) 依赖升级** — `go get github.com/cloudwego/eino@v0.9.14 github.com/cloudwego/eino-ext/components/model/openai@v0.1.13 && go mod tidy`。`git diff go.mod` 核对结果：
- direct 段恰好两行变化：eino `v0.7.7 → v0.9.14`、ext openai `v0.1.5 → v0.1.13`
- indirect 段恰好两行联动（tidy 自动）：`eino-ext/libs/acl/openai v0.1.2 → v0.1.17`、`meguminnnnnnnnn/go-openai v0.1.0 → v0.1.2`
- hertz `v0.10.6` / milvus-sdk-go/v2 `v2.4.2` / modernc.org/sqlite `v1.56.0` 三行逐字未动（T-8-01 核对通过：零预期外 module path 变化）

**(2) TestContractVersionPin 改写** — 断言字串换新 pin；hertz v0.10.6 与 milvus v2.4.2 两条断言原样保留；函数 doc 注释按 v0.9.14 时代重写并注记用户 2026-08-17 决策（v0.9.14 覆盖研究期 v0.9.13；v0.10.0-alpha.x maintainer 警告 breaking，明确不用）。过时的「newer v0.1.13+ requires eino v0.7.13+」pin 理由已删。

**(3) 8 处 godoc 版本标注重核** — 每处在 `$(go env GOMODCACHE)/github.com/cloudwego/eino@v0.9.14/` 下重新定位真实坐标后改标：

| 文件 | 旧标注（v0.7.7） | 新标注（v0.9.14 实测） |
|------|------------------|------------------------|
| internal/agent/agent.go:52 | `eino@v0.7.7` 裸引用 | `eino@v0.9.14` |
| internal/agent/agent.go:167 | `option_test.go:71` | `option_test.go:73`（MaxStep:3 单工具调用流测试现位置） |
| internal/agent/agent.go:226 | `react.go:50` | `react.go:139`（研究预测 ~139，实测吻合；同块内 deprecated `Model` 字段 `react.go:53 → :142` 一并更新） |
| internal/agent/agent.go:236 | `compose/tool_node.go:121` | `compose/tool_node.go:186` |
| internal/agent/callbacks.go:103 | `template.go:430-433` + `502-503` | `template.go:525-528`（ModelCallbackHandler）+ `596-600`（ToolCallbackHandler） |
| internal/agent/retrieve_tool.go:182 | `invokable_func.go:101-105` | `invokable_func.go:139-143`（v0.9.14 NewTool 明确为显式 ToolInfo + typed fn，注释措辞同步修正） |
| scripts/spiked/e2e.go:244 | `adk/call_option.go:54` | `adk/chatmodel.go:97`（**WithChatModelOptions 在 v0.9.14 已迁移文件**） |
| scripts/spiked/wire.go:22 | 裸字串 `Eino v0.7.7 + eino-ext openai v0.1.5` | `Eino v0.9.14 + eino-ext openai v0.1.13`（grep `eino@` 搜不到的单独项） |

### Task 2: hermetic 回归重锁（验证性任务，零提交）

四道门全部 exit 0，**零行为回归 → 按设计无 fix 提交**（plan `<files>` 明确「无新文件；如出现回归才独立归因提交」）：

| 门 | 命令 | 结果 |
|----|------|------|
| 1 | `go build ./... && go vet ./...` | exit 0（升级对现有代码强制源码改动为零——RF-2 在仓库级复现） |
| 2 | `go test ./...` | 全绿（agent / eval / knowledge / milvus / mongo / server / skill / store / verdict / wire / spiked / eval_snapshot 全部 ok；contract 包因 build tag 空跑属预期） |
| 3 | `go test ./integration/contract -tags=contract -count=1 -timeout 10s -v` | `--- PASS: TestContractVersionPin`（新 pin）+ `--- PASS: TestContractRunsUnder10s`（81.8ms / 预算 10s）+ WireFormat/MilvusFilter/StructuredOutputSchema/CitationGrounding 全 PASS；milvus/e2e 两个 SKIP 为既有 live-stack 按设计跳过 |
| 4 | `go test ./internal/server -run TestCancelPropagationMidTool -v -count=1` | `--- PASS`（请求 50.6ms 返回，<200ms 计时断言未放松） |

## Must-Haves Truths 验证

| Truth | 状态 | 证据 |
|-------|------|------|
| eino v0.9.14 + ext v0.1.13 下 `go build ./...` 与全部既有单测全绿 | ✅ | 门 1 + 门 2 |
| `integration/contract` 漂移套件 `-tags=contract` 全绿 + VersionPin 断言新 pin | ✅ | 门 3（TestContractVersionPin PASS） |
| TestCancelPropagationMidTool 仍 <200ms 绿 | ✅ | 门 4（50.6ms） |
| 升级提交零功能代码 | ✅ | `36845c0` 仅含 go.mod/go.sum + 断言改写 + 注释版本标注；8 文件 42+/39-，无逻辑行变更 |

## Deviations from Plan

**1. [Rule 1 - 验收口径冲突] contract_test.go doc 注释中的历史版本字样**
- **Found during:** Task 1 步骤 (2)
- **Issue:** 验收标准要求 `grep -c "v0.7.7" contract_test.go = 0`，但迁移历史注释初稿含「eino v0.7.7 → v0.9.14」字样（1 处）。
- **Fix:** 改写为「eino upgraded to v0.9.14」措辞，避免旧字面量；复验 grep = 0。
- **Files modified:** integration/contract/contract_test.go
- **Commit:** 36845c0（与升级同一原子提交，属改写任务本身）

**2. [上下文注记，非偏差] 工作树比 plan 预期干净**
- plan Pitfall 6 描述的 internal/server/ + web/ 多文件 M 状态，在执行开始前已被用户以 `4cc4f05`（fix(server): verdict stream hang）自行提交。untracked `scripts/test`（含 live GLM key）/ `screenshots/` / `.claude/` / `du.exe.stackdump` 保持 untracked，未入任何提交（T-8-02 通过）。

其余：plan executed exactly as written。

## 提交卫生（T-8-01 / T-8-02 / T-8-03 复核）

- `git show --stat 36845c0` = 恰好 8 个规定路径；`internal/server/`、`web/`、`scripts/test`、`screenshots/` 零命中
- 零文件删除（`git diff --diff-filter=D --name-only HEAD~1 HEAD` 空）
- go.sum 仅含 4 个 module 的校验和联动（与 go.mod diff 一一对应），由 Go 工具链强制（供应链完整性）

## Auth Gates

None — 本 plan 纯 hermetic，无 live stack 依赖（live 重锁属 08-05 的 checkpoint:human-verify）。

## Known Stubs

None — 无 UI / 无占位数据（本 plan 零功能代码）。

## Threat Flags

None — 无新增安全面；威胁登记表 T-8-01/02/03 三项 mitigate 全部按 plan 落实（见「提交卫生」节）。

## Self-Check: PASSED

- 文件存在性：go.mod / go.sum / integration/contract/contract_test.go / internal/agent/agent.go / internal/agent/callbacks.go / internal/agent/retrieve_tool.go / scripts/spiked/e2e.go / scripts/spiked/wire.go — 全部 FOUND（git tracked @ 36845c0）
- 提交存在性：`36845c0` FOUND（git log 验证）
- 四道门输出留存于执行记录（本文件「Task 2」表）
