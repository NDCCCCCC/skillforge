---
phase: 29
slug: feedback-loop-ticket-interface
status: planned
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-08
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Rewritten post-planning 2026-09-08 (checker warning-7): Per-Task Verification Map now
> enumerates all 21 planned tasks across 9 plans / 6 waves, sourced from RESEARCH §Validation
> Architecture + each plan's `<automated>` verify command. Supersedes the pre-planning draft skeleton.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`（hermetic，零外部依赖；cgo 包在 WSL）+ vitest（FE） |
| **Config file** | `go.mod` / `web/package.json`（both exist） |
| **Quick run command** | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/... ./internal/ticket/... ./internal/server/... -count=1"` + `cd web && npx vitest run <touched> && npx tsc --noEmit` |
| **Full suite command** | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./... -count=1"` + `cd web && npm run test && npx tsc --noEmit` |
| **Estimated runtime** | ~60-120 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick run command (scoped to touched packages)
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

21 tasks / 9 plans / 6 waves — every row carries an `<automated>` command (no MISSING references).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 29-01-T1 | 29-01 | 1 | FBK-01/04 | T-29-06 | AppendTurn 返回 assistant 行 id + conflict→0（幂等键信任锚，Pitfall 2 守卫）+ 12 调用点/5 fake 原子涟漪 | build+vet+unit (Go, WSL) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go build ./... && go vet ./internal/store/... ./internal/server/ && go test ./internal/store/... ./internal/server/... -count=1"` | exists (13 files extended) | ⬜ pending |
| 29-01-T2 | 29-01 | 1 | FBK-01 | T-29-06 | fresh id / conflict 重放=0 / GetSession ID 递增 / closed store 报错 | unit hermetic (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/sqlite/ -run TestAppendTurnReturnsAssistantID -count=1 -v"` | task-created (tdd) | ⬜ pending |
| 29-02-T1 | 29-02 | 2 | FBK-01/02/03+TKT-02 | T-29-01 | 4 张 additive 新表 DDL + 尾部索引，零破坏性变更（schema 断言） | unit hermetic schema assert (Go, WSL) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/sqlite/ -run 'TestFeedbackSchema\|TestTicketsSchema' -count=1 -v"` | task-created | ⬜ pending |
| 29-02-T2 | 29-02 | 2 | FBK-01/02/03 | T-29-04/01/06 | 投票/refs/处置/队列/指标 SQL；pending 派生视图（latest-disposition-wins 重开）；100% 参数化 | unit hermetic (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/sqlite/ -run 'TestFeedback\|TestPendingReview\|TestResolvedDispositions\|TestVotedVerdicts\|TestGetSessionTurn' -count=1"` | task-created (tdd) | ⬜ pending |
| 29-02-T3 | 29-02 | 2 | TKT-02 | T-29-01/04 | 工单幂等落库（一消息一单）+ InsertOnly 结构锁（零 UPDATE 路径） | unit hermetic (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/store/sqlite/ -run 'TestTicket' -count=1"` | task-created (tdd) | ⬜ pending |
| 29-03-T1 | 29-03 | 1 | TKT-01 | T-29-02 | Provider 窄接口（单方法）+ none/webhook/未知值注册表分支 | unit (tdd) | `go test ./internal/ticket/ -run TestNewProvider -count=1 -v` | task-created (tdd) | ⬜ pending |
| 29-03-T2 | 29-03 | 1 | TKT-01 | T-29-02/07 | webhook 2xx 判定/有界重试/超时/密钥头/payload 只进 body（URL 零用户输入） | unit httptest (tdd) | `go test ./internal/ticket/ -count=1 -v && go vet ./internal/ticket/` | task-created (tdd) | ⬜ pending |
| 29-04-T1 | 29-04 | 3 | FBK-01/04 | T-29-06 | data-turn 帧契约 Go ValidTypes + TS union 同 commit（D15）；turnID<=0 零输出 | unit (Go, WSL) + vitest + tsc | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/wire/ ./internal/server/ -run 'TestValidTypes\|TestEmitTurnID\|TestChunkType' -count=1" && cd web && npx vitest run wire-contract && npx tsc --noEmit` | task-created | ⬜ pending |
| 29-04-T2 | 29-04 | 3 | FBK-01/04 | T-29-06/05 | data-turn 恰 11 发射点（5 拒绝路径先落库重排）严格先于 finish；turnId==落库 id；turnID==0 不发帧 | unit hermetic stream (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run 'TestVerdictPipeline\|TestRefusal\|TestChatHandler\|TestRunner\|TestEmitTurn' -count=1"` | exists (extended) | ⬜ pending |
| 29-04-T3 | 29-04 | 3 | FBK-04 | T-29-06/03 | history 一次请求带 turn_id+voted_verdict；截断不串位；store 报错降级 200 | unit + vitest (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestSessionHistory -count=1" && cd web && npx vitest run chat-restore && npx tsc --noEmit` | exists (extended) | ⬜ pending |
| 29-05-T1 | 29-05 | 3 | FBK-01 | T-29-01/06 | 幂等投票 + sessionIDRegex/turn 三重绑定 + verdict 闭集 + 服务端 chunk 归因（不信任 FE） | unit hermetic (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestFeedbackHandler -count=1 -v"` | task-created (tdd) | ⬜ pending |
| 29-05-T2 | 29-05 | 3 | FBK-02/03/05 | T-29-03/05/04/06 | admin 403 先于 503 + closed_count + 全票分母 0 票 null + FBK-05 source-grep 结构锁 | unit + source-grep (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run 'TestFeedbackReview\|TestFeedbackDisposition\|TestFeedbackMetrics\|TestFeedbackNoTrustMutation' -count=1 -v"` | task-created (tdd) | ⬜ pending |
| 29-06-T1 | 29-06 | 4 | TKT-02 | T-29-01/04 | 幂等落库 + TKT-{行 id} + dispatch_status 语义 + nil-provider 503 | unit hermetic (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestTicketsHandler -count=1 -v"` | task-created (tdd) | ⬜ pending |
| 29-06-T2 | 29-06 | 4 | TKT-04 | T-29-07/02/04 | 异步外发不阻塞响应 + defer recover + 结果只进审计行 + env 驱动装配（零硬编码） | unit + build + vet (tdd) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go build ./... && go test ./internal/server/ -run 'TestTicketDispatch\|TestTicketsHandler' -count=1 && go vet ./cmd/server/ ./internal/server/"` | task-created (tdd) | ⬜ pending |
| 29-07-T1 | 29-07 | 4 | FBK-04 | T-29-09 | sf-toast 监听 + latest-wins 单槽 + aria（JSX 文本渲染，零 dangerouslySetInnerHTML） | vitest + tsc (tdd) | `cd web && npx vitest run SfToast && npx tsc --noEmit` | task-created (tdd) | ⬜ pending |
| 29-07-T2 | 29-07 | 4 | FBK-04+TKT-03 | T-29-09/01/04 | 服务端驱动锁定 + 失败不锁定 + 工单 4 态诚实文案 + POST body 形状断言（无 messageId） | vitest + tsc (tdd) | `cd web && npx vitest run FeedbackRow MessageBubble && npx tsc --noEmit` | exists (rewritten) | ⬜ pending |
| 29-08-T1 | 29-08 | 5 | FBK-02/03+TKT-03 | T-29-09/03/05 | admin store union + 7 tab 接线（「反馈」第 6「复核」第 7）+ admin SfToast 挂载 | vitest + tsc | `cd web && npx vitest run AdminFeedbackTabs && npx tsc --noEmit` | task-created | ⬜ pending |
| 29-08-T2 | 29-08 | 5 | FBK-02 | T-29-09/05 | 复核按 chunk 聚合 + 三处置单点击终态 + 信任横幅（零「已自动改库」文案） | vitest + tsc (tdd) | `cd web && npx vitest run ReviewQueuePanel && npx tsc --noEmit` | task-created (tdd) | ⬜ pending |
| 29-08-T3 | 29-08 | 5 | FBK-03 | T-29-09/05 | 口径注常驻 + 5 汇总卡（0 票「—」非 0%）+ 日/周趋势 + per-KB 脚注 | vitest + tsc (tdd) | `cd web && npx vitest run FeedbackMetricsPanel && npx tsc --noEmit` | task-created (tdd) | ⬜ pending |
| 29-09-T1 | 29-09 | 6 | FBK-05 门 | T-29-05/06 | verify-phase-29.sh 4 stages（TODO 清零 / 结构锁 / D15 配对 / DO NOTHING，含负向自证）+ make 目标 | shell gate | `bash scripts/verify-phase-29.sh && make phase-29` | task-created | ⬜ pending |
| 29-09-T2 | 29-09 | 6 | 全 REQ 收口 | T-29-08/06 | STRIDE-29 每威胁有 Disposition + operator runbook 步骤完整 | grep gate + human runbook | `grep -c "T-29-0" docs/threat-model/STRIDE-29.md && grep -c "^### Test" .planning/phases/29-feedback-loop-ticket-interface/29-UAT.md` | task-created | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Sampling continuity:** 21/21 tasks carry `<automated>` verify — zero runs of 3 consecutive tasks without automated verification. 15/21 tasks are test-first (`tdd="true"`); the 6 non-tdd tasks are 29-01-T1 (atomic compile-ripple commit — verify = full build+vet+test of both touched packages), 29-02-T1 (schema DDL + index assertions, behavior halves carried by 29-02-T2/T3 tdd), 29-04-T1 and 29-08-T1 (the two D15/config wiring tasks), 29-09-T1 (gate script), and 29-09-T2 (docs pair, grep-gated).

---

## Wave 0 Requirements

All previously-flagged Wave-0 items are covered by owning tasks inside the wave plan — **no pre-wave scaffold debt, no MISSING references**:

- [x] feedback/tickets 表的 InsertOnly 测试族（per-table，store 包内惯例）→ **29-02-T1**（schema 断言）+ **29-02-T2/T3**（InsertOnly 读写测试，tdd）
- [x] history 端点 voted_verdict/turn id 的 handler 测试 → **29-04-T3**（扩展既有 handlers_session_history_test.go，tdd）
- [x] FE FeedbackRow 真实 API 调用的 vitest mock → **29-07-T2**（FeedbackRow.test.tsx 整文件重写，tdd，vi.stubGlobal(fetch)）

*Existing infrastructure covers framework needs — no new framework install.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| webhook provider 真实外发 | TKT-01/04 | 需要外部端点（内网间歇断，env-intranet-service-flakiness） | 配置 webhook URL 指向可观测端点，建单观察外发与重试日志、审计行（29-06-T2 落地后；29-09 29-UAT.md 收录步骤） |
| FE 锁定态真实回显 | FBK-04 | 跨请求 UI 状态 | 投票 → 刷新 → 从 history 回显已投态（29-07-T2 + 29-04-T3 落地后） |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies（21/21）
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references（0 MISSING — all test files task-created or pre-existing-extended）
- [x] No watch-mode flags
- [x] Feedback latency < 120s（quick commands are package-scoped `go test -run` / `vitest run <name>`）
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planner-approved 2026-09-08（post-planning rewrite；执行期由 execute-phase 按 Sampling Rate 维护行内 Status 列）
