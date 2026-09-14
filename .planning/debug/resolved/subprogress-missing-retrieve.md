---
slug: subprogress-missing-retrieve
status: resolved
created: 2026-09-14
updated: 2026-09-14
trigger: "TestVerdictPipeline_SubprogressOrder 在 pristine HEAD 上失败：缺 'retrieve' subprogress chunk（上一 debug 会话归档时记录的 pre-existing 失败），用户要求处理"
---

# Debug Session: subprogress-missing-retrieve

## Symptoms

- **Expected**: verdict 管线恰好发 4 个 data-subprogress chunk，顺序 retrieve → found_candidates → generate → self_verify（`pipeline_subprogress_test.go:103` 断言 `progress[0].Step == "retrieve"`）。
- **Actual**: 只发 3 个：`[{found_candidates authority 2} {generate 0} {self_verify 0}]`，开头的 `retrieve` 缺失。
- **Errors**: `"[{found_candidates authority 2} {generate 0} {self_verify 0}]" should have 4 item(s), but has 3`。
- **Timeline**: pre-existing（2026-09-12 归档时确认 pristine HEAD 同样失败）。
- **Reproduction**: WSL `go test ./internal/server/ -run TestVerdictPipeline_SubprogressOrder -v` 稳定复现（orchestrator 2026-09-14 实跑确认；测试走完整 chatHandler → VerdictPipeline，本地 Hertz 随机端口，不依赖 :8888）。

## Evidence

- timestamp: 2026-09-14 (debugger, post-fix)
  checked: WSL `go test -run "TestVerdictPipeline_SubprogressOrder|TestVerdictPipeline_RefusalNoSubprogress" -v`
  found: 两个测试均 PASS。Order 测试 4-chunk 断言满足；Refusal 测试仍零 chunk（retrieve 发射放在拒答闸门之后的位置约束成立）。
  implication: 原始症状消除，且恢复位置不破坏相邻契约。
- timestamp: 2026-09-14 (debugger, regression bisect)
  checked: 全包 `go test ./internal/server/ -count=1` 出现 3 个 FAIL（TestDataDecisionExtensionNoNewChunkTypes / TestFeedbackDispositionAdminGate / TestIsBackendUnavailable_NetErrorTimeout）→ stash 掉本修复后在 pristine HEAD 重跑同 3 个测试
  found: 3 个 FAIL 在 pristine HEAD 上逐一复现（与本修复无关的 pre-existing 失败；backend-unavailable 已有独立 debug 会话 .planning/debug/backend-unavailable-test-fail.md）。修复 delta = 零新增失败。
  implication: 回归测试通过（对比基线而非绝对绿）。
- timestamp: 2026-09-14 (debugger, stability)
  checked: `go test -count=3 -run "…SubprogressOrder|…RefusalNoSubprogress"`
  found: 3 次重复运行全 PASS（go vet + go build ./internal/... 亦通过）。
  implication: 修复稳定，非偶发。
- timestamp: 2026-09-14 (orchestrator)
  - 失败输出（含管线日志）：routing 为 USE_AGENT_PATH → Registry+Router → VerdictPipeline (Loader)；`[WARN] counter-evidence pass failed: KB registry not wired`（non-fatal，判定不受影响）。
- timestamp: 2026-09-14 (user, live wire-level verification)
  checked: orchestrator curl POST /api/chat on :8888（compliance skill，query「年假没修完怎么办？」），抓 SSE 原始帧
  found: 响应恰含 4 个 data-subprogress chunk 且严格有序——①{"step":"retrieve","tier":"authority"}（流的第一帧）→ ②{"step":"found_candidates","tier":"authority","count":5} → ③{"step":"generate"} → ④{"step":"self_verify"}。FE 契约（web/src/types/verdict.ts:119 step 联合类型 + fixtures）支持 retrieve，浏览器首步渲染「检索中」。
  implication: 修复在真实运行环境（非仅单测）下生效，原始症状彻底消除。
- timestamp: 2026-09-14 (user, stale-process incident — "fix not effective" false impression source)
  checked: 首次 live 验证时 :8888 上跑的是 pid=414（09:28:11 启动），早于 pipeline.go 修复时间 09:31:31 —— 旧二进制仍在服务
  found: `pkill -x skillforge-server-dev-new` 必然匹配失败：Linux comm 字段截断到 15 字符（`skillforge-serv`），-x 全名精确匹配永远无法命中；`ss -tln | grep 8888` 的 PORT_UP 检查也被旧进程骗过（端口在但不是新代码）。
  implication: 按 PID kill + 重启（新 pid=5120，09:47:07）后 curl 立即看到完整 4-chunk 序列。教训：重启后必须用 `ps -eo pid,lstart,cmd` 验证新 pid + 启动时间，不能只看端口存在；env-wsl-backend-runbook 记忆中的 pkill 命令是错的（orchestrator 负责更新该记忆）。
- timestamp: 2026-09-14 (grep 全仓库)
  - `grep '"retrieve"' --glob 'internal/server/*.go'` **只命中测试文件** pipeline_subprogress_test.go:103 —— 生产代码没有任何位置发射 step=="retrieve" 的 chunk。
- timestamp: 2026-09-14 (关联背景)
  - 上一 debug 会话（resolved/new-session-shows-old-messages.md）在 pipeline.go 去掉了拒答路径重复写入并移除 unused 参数 `classifierReason`，但本失败在那之前就存在（pristine HEAD 复现），与之无关。

## Current Focus

hypothesis: RESOLVED — root cause confirmed and fix verified both at unit level and live wire level.
test: n/a (session closed)
expecting: n/a (session closed)
next_action: archived — session moved to resolved/, fix committed, knowledge base entry appended.
reasoning_checkpoint:
  hypothesis: c264fac 删除 retrieve 发射行导致生产只发 3 个 data-subprogress chunk，测试 RED 是回归而非过时断言
  confirming_evidence:
    - "git show c264fac: diff hunk 展示 `- emitSubprogress(c, wire.SubprogressStep{Step: \"retrieve\", Tier: tiers[0]})`，commit message 自述 'Removed pre-Search emitSubprogress (preserve UX, skip for line savings)'"
    - "git log -S '\"retrieve\"' 仅 3 个 commit：6939050(RED 测试) → 6366eff(GREEN 实现 4 发射点) → c264fac(删除 retrieve)"
    - "grep 全仓库：生产 pipeline.go 仅 found_candidates(:240)/self_verify(:285)/generate(:372) 三处发射；'retrieve' 只存在于测试文件"
    - "FE 契约含 retrieve：web/src/types/verdict.ts:119 SubprogressStepName 联合类型、__tests__/DataSubprogressPart.test.tsx 渲染 step='retrieve'、__fixtures__/sub-progress.json fixture —— 改测试为 3-chunk 会破坏 FE 类型与渲染器"
  falsification_test: 恢复行后若 TestVerdictPipeline_RefusalNoSubprogress 变 RED（拒答路径出现 1 个 chunk）则位置放错；若 Order 测试仍 RED 则另有缺失
  fix_rationale: 逐字恢复被 c264fac 删除的那一行到原位置（0-chunk 拒答闸门后、found_candidates 前），同时满足 Order（retrieve→found_candidates→generate→self_verify）与 RefusalNoSubprogress（拒答路径零 chunk）两个测试；单行最小改动，直指回归根因
  blind_spots: 测试 docstring 的 'retrieve before Search' 文案与实际 GREEN 位置（Search 之后）不符——两测试联合约束证明闸门后位置才是权威契约，文案属愿景性描述；尚未跑全包套件确认无其他测试依赖 3-chunk 现状（修复后跑）

## Eliminated

- hypothesis: (b) 测试是 TDD aspirational，生产从未实现 retrieve chunk，规格已演化为 3-chunk
  evidence: git log -S 证明 6366eff（GREEN commit "VerdictPipeline streams per-stage data-subprogress chunks"）确实实现了 4 个发射点含 retrieve，随后 c264fac 的 diff 明确删除了该行且 commit message 自述 "Removed pre-Search emitSubprogress"；且 FE 类型/渲染测试/fixture 均含 retrieve，契约未演化。
  timestamp: 2026-09-14

## Resolution

root_cause: 回归 —— c264fac（refactor 33-01 压缩 VerdictPipeline 行数）删除了 6366eff 引入的 `emitSubprogress(Step:"retrieve", Tier:tiers[0])` 发射行，生产只剩 3 个 data-subprogress chunk；测试锚定的 4-chunk 契约与 FE 类型/渲染器/fixture 一致，属测试正确、实现回归。
  注：live 侧验证曾被 stale-process 干扰延迟（:8888 上 09:28 启动的旧进程未被 pkill -x 命中，comm 15 字符截断导致 -x 精确匹配必然失败，PORT_UP 端口检查亦被旧进程骗过）；按 PID 重启后，最终 4-chunk 序列（retrieve → found_candidates → generate → self_verify）经 curl 抓 SSE 帧实证确认。
fix: 在 internal/server/pipeline.go 的 `len(chunks)==0` 拒答闸门之后、found_candidates 发射之前，逐字恢复被删的 `emitSubprogress(c, wire.SubprogressStep{Step: "retrieve", Tier: tiers[0]})`。
verification: PASS（双层）—— 单测层：靶向两测试 PASS（-count=3 稳定）；`go vet` + `go build ./internal/...` 通过；全包回归对比 pristine HEAD 零新增失败（3 个 FAIL 均 pre-existing，有独立 debug 会话跟踪）。live 层：:8888 curl SSE 抓帧确认恰好 4 个 data-subprogress chunk 严格有序，retrieve 为流首帧（需先按 PID 杀掉旧进程重启，见 stale-process incident）。
files_changed:
  - internal/server/pipeline.go（恢复 1 行：len(chunks)==0 拒答闸门后、found_candidates 前，emitSubprogress Step:"retrieve" Tier:tiers[0]）

## Notes

- **Stale-process trap（运维教训，防止复发）**：
  - `pkill -x skillforge-server-dev-new` 永远匹配不到 —— Linux comm 截断到 15 字符（`skillforge-serv`），`-x` 全名精确匹配不可能成功。env-wsl-backend-runbook 记忆里记录的这条命令是错的，orchestrator 将更新该记忆。替代：`pkill -f 'skillforge-server-dev-new'` 或按 PID kill。
  - `ss -tln | grep 8888` 的 PORT_UP 检查会被旧进程骗过 —— 端口存在 ≠ 跑的是新代码。重启后必须验证新 pid + 启动时间（`ps -eo pid,lstart,cmd`），不能只看端口。
  - 本次事故表现：修复 09:31:31 落地，但 :8888 上仍是 09:28:11 启动的 pid=414，导致一次「修复无效」的误判；kill-by-PID + 重启（pid=5120, 09:47:07）后 curl 立即见完整 4-chunk 序列。
- 交叉验证锚点：前端 web/src 里 subprogress 渲染（若有 per-step label/icon 映射，查它认不认 "retrieve"）——这是判定 (a)/(b) 的最强外部证据。
- 该测试不依赖 :8888 / Milvus（测试内 KB registry 未 wired，检索走 stub），WSL 直接跑即可。
- 修复时注意与 resolved/new-session-shows-old-messages 中 pipeline.go 的最新改动同文件族，先 pull 最新工作区状态再动手。
