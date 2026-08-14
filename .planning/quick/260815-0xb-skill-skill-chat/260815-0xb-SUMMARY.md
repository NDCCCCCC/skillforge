# Quick Task 260815-0xb: Skill 斜杠菜单 + chat 流式进度 Summary

**One-liner:** ChatInput 换用 Claude Code 式 `/` 斜杠命令 Skill 菜单（过滤 + 键盘导航），VerdictPipeline 打通 FEUI-06 `data-subprogress` 通道，LLM 等待期逐条流式发射 retrieve/found_candidates/generate/self_verify 进度。

- **Phase:** 260815-0xb-skill-skill-chat
- **Plan:** 01
- **Subsystem:** chat-interaction (BE pipeline SSE + FE composer)
- **Tags:** slash-menu, data-subprogress, sse, verdict-pipeline, chat-input

## What Was Done

### Task 1: 后端 — VerdictPipeline 分阶段流式发射 data-subprogress (TDD)

- `internal/wire/chunk.go`: `data-subprogress` 注册为第 20 个 `ValidTypes` 白名单类型（Count 注释 19→20，首行 "17" 陈旧注释一并修正）；新增导出 `wire.SubprogressStep{Step, Tier omitempty, Count omitempty}`，字段与 FE `SubprogressStep` 接口逐字段对齐。
- `internal/server/pipeline.go`: 新增 `emitSubprogress` helper（json.Marshal → `wire.EmitChunk`，marshal/emit 失败 log-and-continue — T-0xb-02 mitigate）；verdict 路径 4 个发射点：`retrieve`+`found_candidates`（Search 成功且 len(chunks)>0 后）、`generate`（Runner.Query 前）、`self_verify`（GroundingChecker 前）。refusal 短路与 runAgentPath 不动。
- 测试（先 RED 后 GREEN）：`chunk_test.go` 计数断言改 20 + `TestSubprogressStepRoundTrip`（含 omitempty 契约）；新文件 `pipeline_subprogress_test.go` 断言 verdict 路径 4 chunk 按序全部先于 text-start、found_candidates 携带 count/tier、refusal 路径零 subprogress。

### Task 2: 前端 — ChatInput 斜杠命令 Skill 菜单 (TDD)

- `web/src/components/chat/ChatInput.tsx` 重写 Skill 选择交互：
  - **移除** D5-IA-01 点击式 popover（`skillOpen` 状态 + `chat-input-skill-popover` 整段，`chat-input-skill-popover` 在 web/src 零残留）。
  - `chat-input-current-skill` 改为只读徽标（无 chevron、无 onClick）。
  - **斜杠菜单**：`menuOpen` 由文本派生（以 `/` 开头且无空白字符，仅非向导且非 disabled）；过滤为 name+description 小写 substring 匹配；`activeIndex` 高亮（data-active）随列表缩短 clamp。
  - **键盘语义**（优先于 Enter-to-send）：ArrowDown/Up 循环移动、Enter 选中（`setCurrentSkill` + 清空输入 + 关菜单，不触发 onSend）、Esc 关菜单保留文本（`dismissed` latch，再次输入重新打开）。
  - WR-02 stale-skill 清除 effect 原样保留；placeholder 更新为 `输入问题，或键入 / 选择 Skill，或拖放文件...`。
- `web/src/__tests__/ChatInput.test.tsx`: 重写为 8 个用例（打开/过滤/键盘导航+选中/Esc+外点/空态/向导模式禁用/WR-02/菜单关闭时 Enter 照常发送）。

### Task 3: 全量回归门

- `go test ./...` 全绿 + `go vet ./...` 零告警。
- `npx vitest run` 236/236 全绿（35 文件）；`npx tsc --noEmit` 0 错误；`npm run build` 成功。
- grep 门：chunk.go 含 `data-subprogress`；pipeline.go `emitSubprogress` 4 处调用；web/src `chat-input-skill-popover` 零命中。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] retrieve 进度 chunk 的发射位置与 refusal 契约冲突**
- **Found during:** Task 1
- **Issue:** 计划 action 写"Search 调用前发射 retrieve"，但 behavior Test 3 要求 0-chunk refusal 短路路径**零** data-subprogress —— Search 前发射会在 refusal 路径泄漏 1 个 chunk，且 SSE 发出后无法撤回。计划两处描述内部矛盾。
- **Fix:** 以 TDD 测试契约为准：retrieve 改在 Search 成功且通过 0-chunk 门之后、紧接 found_candidates 之前发射。检索是毫秒级 Milvus 调用（主导等待是 LLM，由 generate/self_verify 覆盖），UX 目标不受损。已在 pipeline.go 内注释注明。
- **Files modified:** internal/server/pipeline.go
- **Commit:** 6366eff

**2. [Rule 3 - Blocking] worktree 缺少 node_modules**
- **Found during:** Task 2 RED
- **Issue:** vitest 无法启动（worktree 新建，web 依赖未安装）。
- **Fix:** `npm ci`（既有 package-lock.json，非新增依赖）。
- **Commit:** 无（环境准备）

**3. [Rule 1 - Bug] chunk.go ValidTypes 首行注释陈旧（"17 kebab-case chunk types"）**
- **Found during:** Task 1
- **Issue:** 注释声明 17 个类型但实际已是 19 —— 与 Count 注释自相矛盾。
- **Fix:** 随 Count 19→20 一并修正为 20。
- **Files modified:** internal/wire/chunk.go
- **Commit:** 6366eff

### Out-of-Scope Discoveries (not fixed, logged)

**Bundle 基线漂移（预存在）**: 计划基线 466.50 KB / 135.02 KB gzip，本次 build 实测 848.77 KB / 247.73 KB gzip。核实：package.json/package-lock.json 相对 base 零改动、本次唯一新增 import 是 react 的 `useMemo`（既有依赖）；主仓库 8 月 13 日（本任务开始前）的 dist 产物即为 846 KB —— 漂移在本计划之前已存在，基线数字系沿用陈旧记录。按 SCOPE BOUNDARY 不处理，建议后续 quick task 排查（可能与近期引入的 markdown/代码高亮相关 bundle 增长有关，并考虑 manualChunks 拆分 —— build 已有 >500 kB chunk 警告）。

## Key Decisions

1. **retrieve 发射位置以测试契约为准**（post-Search、0-chunk 门后）——保住 refusal 快速返回不被装饰性 chunk 拖累/污染。
2. **斜杠菜单 menuOpen 由文本派生**（非独立 toggle state）+ `dismissed` latch 处理 Esc/外点 —— 菜单与输入框内容结构性同步，不会出现菜单开着但文本已是普通消息的 desync。
3. **menuRef 覆盖整个 composer 容器**：textarea 内点击（光标定位）不关菜单，仅真正外部点击关闭。

## Verification Results

| Gate | Result |
|------|--------|
| `go test ./internal/wire/ ./internal/server/` | PASS |
| `go vet ./internal/wire/ ./internal/server/` | PASS |
| `go test ./...` (full tree) | PASS |
| `go vet ./...` | PASS |
| `npx vitest run src/__tests__/ChatInput.test.tsx` | 8/8 PASS |
| `npx tsc --noEmit` | 0 errors |
| `npx vitest run` (full suite) | 236/236 PASS |
| `npm run build` | SUCCESS |
| grep: `chat-input-skill-popover` in web/src | 0 hits |

## Commits

| Task | Commit | Type |
|------|--------|------|
| Task 1 RED | 6939050 | test |
| Task 1 GREEN | 6366eff | feat |
| Task 2 RED | d55c852 | test |
| Task 2 GREEN | 498d051 | feat |

## TDD Gate Compliance

- Task 1: `test(...)` commit (6939050) RED — 构建失败（SubprogressStep 未定义，2 包编译失败，符合预期）；`feat(...)` commit (6366eff) GREEN — 全部转绿。PASS
- Task 2: `test(...)` commit (d55c852) RED — 8 用例中 5 失败（菜单不存在），3 个保留行为用例通过（WR-02/向导/普通 Enter —— 现状即应通过）；`feat(...)` commit (498d051) GREEN — 8/8。PASS

## Key Files

- Created: `internal/server/pipeline_subprogress_test.go`
- Modified: `internal/wire/chunk.go`, `internal/wire/chunk_test.go`, `internal/server/pipeline.go`, `web/src/components/chat/ChatInput.tsx`, `web/src/__tests__/ChatInput.test.tsx`

## Metrics

- Duration: ~11 min (2026-08-14T16:47Z → 16:58Z UTC)
- Tasks: 3/3 complete
- Files: 6 modified/created

## Self-Check: PASSED

- All 6 task files exist on disk (FOUND × 6, MISSING × 0)
- All 4 commits present on worktree-agent-a40048f446037df71: 6939050 / 6366eff / d55c852 / 498d051
- Working tree clean except this SUMMARY (intentionally uncommitted — orchestrator owns the docs commit)
