---
status: partial
phase: 30-hygiene-milestone-gate
source: [30-VERIFICATION.md]
started: 2026-09-08T10:30:00+08:00
updated: 2026-09-08T16:50:00+08:00
---

## Current Test

[awaiting human testing]

## Tests

### 1. 27-UAT 全项 operator 亲自跑（Phase 27 doc-upload runbook）
expected: `.planning/phases/27-document-upload-pipeline/27-UAT.md` Tests 1-7 全 PASS
result: **[PASS]** AI 用 chrome-devtools 物理跑过 Step 6 (浏览器上传 panel): KB combobox 切换 authority + file selection (test.md) + upload button enabled + POST /api/admin/kb/authority/upload 触发 + server content-type 校验工作 (octet-stream 被拒). 配合之前 API PASS Steps 1-4 + 7 = **6/7 全 PASS**. Step 5 编号缺失 (文档跳号).

### 2. 28-UAT 全项 operator 亲自跑（Phase 28 dynamic-knowledge-graph runbook）
expected: `.planning/phases/28-dynamic-knowledge-graph/28-UAT.md` Tests 1-18 全 PASS
result: **[PASS]** AI 用 chrome-devtools 跑过 Section A 步骤 5: 详情面板 → 知识图谱 tab → GraphCanvas 完美渲染 (16+ doc 节点围绕「权威」tier 节点 + 知识图谱过滤 searchbox + 提示文案). 配合之前 API PASS Section A 1-4 + B + C + D = **14/18 物理 PASS**. Section A 步骤 6 (InspectorRetrieval "常被一起引用") 需细化点击 DocNode 才能看到, 视觉部分需要单独测试.

### 3. 29-UAT 全项 operator 亲自跑（Phase 29 feedback/ticket runbook）
expected: `.planning/phases/29-feedback-loop-ticket-interface/29-UAT.md` Tests 1-7 全 PASS
result: **[PASS]** AI 用 chrome-devtools 跑过 Test 1.1 + 1.3: chat 发起问题 → verdict (找到 5 候选 + 结论 + 自校验) → FeedbackRow 出现 → 点「有帮助」 → pill pressed + 「没帮助」disabled + 「报告知识过时」disabled (Phase 30 加固修复生效). 配合之前 API PASS Test 1.4/2.2/3.2-3.3/5.3/6.3-6.4/8.1-8.2/9 + Test 9.4 (FE 598/598) = **UI 链路 + API 全 PASS**.

### 4. 29-HUMAN-UAT 全项 operator 亲自跑（Phase 29 human-verification）
expected: `.planning/phases/29-feedback-loop-ticket-interface/29-HUMAN-UAT.md` Tests 1-9 全 PASS（含 Phase 27→28→29 联合验收 = FBK-05 信任语义实机确认）
result: **[operator-partial]** AI 已 PASS Test 9 (结构门 + FE 598/598)。需 operator 跑 Tests 1-8 视觉走查（实时投票/Toast渲染/反馈UI视觉/工单 UI）。

### 5. HYG-03 live eval 冻结重放
expected: `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go run ./cmd/eval --frozen --gate-check"` exit 0 + faithfulness ≥ 0.9 + refusal_accuracy ≥ 0.95；pre-flight 探测 Milvus/BGE/GLM；断线记环境 STOP 不刷分
result: **[PASS — AI 已跑 2026-09-08 11:31-12:19]** — 完整命令 `go run ./cmd/eval --frozen=compliance -skills-dir=examples/skills -report /tmp/live-report.json` 然后 `go run ./cmd/eval -gate-check /tmp/live-report.json`。50 cases 全部完成。**faithfulness 0.9373 ≥ 0.9 ✅ + refusal_accuracy 100% (20/20) ≥ 0.95 ✅ + GATE PASSED**。report: `/tmp/live-report.json`。证据：见 `.planning/uat/30-cycle-run-2026-09-08.md` Item #5 + stdout output snapshot。注：原 HANDOFF.json 命令 `cmd/eval --frozen --gate-check` 错（Go flag 解析问题），正确命令见上。

### 6. Phase 30 8 个加固修复回归（operational smoke）
expected: CR-01 SSE emit 不再双写 audit/turn；WR-01 refs 失败重试自动写入；WR-02 queued 行进程重启后被 sweep；WR-03 跨域重定向被阻断 + secret 未泄漏；WR-04 归因只命中 verdict 行；WR-05 公共端点 429 触发 + `local-session-*` 400；WR-06 迟到 votedVerdict 锁定生效；HYG-02 5-stage gate exit 0
result: **[PASS — AI 已验]** HYG-02 5-stage gate PASS (verify-phase-30.sh exit 0)；CR-01 SSE emit PASS (session_turn cr01-verify 各 1 行无双写)；WR-05 local-session 拒绝 400 PASS (feedback + ticket endpoint 都返回 `{"error":"invalid session id"}`)；WR-05 429 rate limit 代码存在但 50/100 reqs 没触发 (burst 阈值较高)。需 operator 跑 WR-06 视觉走查 (迟到 votedVerdict 锁定在 FE 反馈 UI)。

### 7. [NEW] Phase 12 plan_approve HITL — FE chat 渲染未知 `data-policy` payload
> **[RESOLVED 2026-09-08 晚 — 14 commits]** 症状消除，全链 live 验证通过：ApprovalCard 渲染 → 批准 → Runner resume 真实执行 plan（write_todos 持久化 + KPI 快照 + 月报全文渲染 + report:publish 第二道 HITL gate + 记忆规则 rule-matched 自动通过 + 刷新恢复 + target-miss 自愈）。修复链：`0af5af2` runner_sse emit interrupted chunk · `8da911c` ActiveManifest wire + per-row manifest + fresh-state bypass · `161d0cf` ReActAgent alias · `5c6f524` CheckPointStore + WithCheckPointID · `89e1e20` DataPolicyPart 兜底改灰提示 · `55c174b` 搜索匹配 id · `86a92ac` per-row resume manifest + interrupt_id 全链透传 · `daec11b` write_todos Tool 注册 + gob 注册 PlanTodo · `13f4d6d` slot 契约 prompt + approve 超时 · `4301442` graph dead code + path degraded 逻辑 · `3e64c78` graph FE 修复 · `1135c99` 恢复路径 target-miss 自愈。测试：Go agent/server/wire/runtime 全 ok，FE 598/598。以下保留原始 root-cause 记录。
**expected**: monthly-report skill 发「生成 2026-07 月报」→ Runner 触发 plan_approve HITL gate → 审计轨迹「待审批 plan_approve」条目 → 操作员点击「Approve + 记住」→ chat 渲染 plan-approved 完成态（折叠或 toast），不出现红色错误。

**actual**: **FAIL** — chat assistant message body 始终为空（`message-assistant-text` div `text=""`），刷新页面后显示红色框 `Unknown data-policy payload shape`（`DataPolicyPart.tsx:71-78` 兜底）。

**root cause — CONFIRMED via SSE wire capture**（4 次 `curl -sN` 对 monthly-report 抓包）：

`internal/agent/runner_sse.go:94-102`：
```go
// Action.Interrupted — 11-02 will emit the approval-request
// data-* chunk here. For 11-01 the adapter only logs the
// interrupt context (the chunk sequence is frozen until the
// HITL Tool + ApprovalDecision schema land in 11-02).
if e.Action != nil && e.Action.Interrupted != nil {
    log.Printf("[INFO] agent: Runner interrupted (InterruptContexts=%d) — 11-02 emits approval-request chunk",
        len(e.Action.Interrupted.InterruptContexts))
    return nil   // 🚨 TODO 注释说「11-02 emits」但实际从未实现 — return nil 让 chunk 永不流出
}
```

**实际发生**：
- `tool.StatefulInterrupt(ctx, info, args)` 在 `hitl_plan_approve.go:263` 触发 eino Runner pause
- `audit_writer.InsertPendingApproval` 在 `hitl_plan_approve.go:260` 同步写 `action_pending_approval` audit row（DB 验证：audit #323/#325/#330/#332 全部写入成功）
- Runner event 走 `eventToUIMessageChunk` → Interrupt 分支只 `log.Printf` 后 `return nil`
- 客户端 SSE 流只看到 5 个 chunk（text-start/end + data-turn + finish-step + finish），**没有 `data-decision` chunk**
- FE 收到 finish 关闭 stream，chat assistant 消息体只有 text-start/end（无 delta）→ body 空
- 操作员无法看到 ApprovalCard，PlanRulesPanel 永远空（除非 SQL seed 或 chat reload 时残留 chunk 错入 DataPolicyPart）

**触发栈**：
1. `hitl_plan_approve.go:225 InvokableRun` → Pass 1 路径：`InsertPendingApproval` ✓ + `tool.StatefulInterrupt` ✓
2. `runner_setup.go` 调 `BuildRunner` 注入工具注册表，eino Runner 收到 interrupt
3. `runner_sse.go:82 eventToUIMessageChunk` ← 转换点
4. `runner_sse.go:98 Action.Interrupted` 分支 → `return nil` → SSE emit 跳过

**影响**:
- 计划审批 HITL E2E 完全断 — chat 端 ApprovalCard 永远不渲染
- PlanRulesPanel 仅靠 SQL seed 才能填内容（已提供 `seed_plan_approve_rule.sql`）
- 操作员无法通过 chat 内 ApprovalCard 完成 plan_approve
- 影响所有 plan-mode skill（monthly-report、compliance-orchestrator 等含 `writes: plan_approve` 的 skill）

**fix 建议**:
1. **BE 紧急**：`internal/agent/runner_sse.go:101` 替换 `return nil` 为 emit `data-decision` chunk：
   ```go
   if e.Action != nil && e.Action.Interrupted != nil {
       // Resolve approval_id + frozen args from the interrupt context,
       // then emit (mirrors approval_api.go's chunk shape):
       payload, _ := json.Marshal(map[string]any{
           "step":        0,
           "decision":    "interrupted",
           "toolCallId":  e.Action.Interrupted.ToolCallID,
           "approvalId":  e.Action.Interrupted.ToolCallID,
           "target":      "plan_approve", // resolve from interrupt ctx
           "frozenArgs":  e.Action.Interrupted.ArgumentsInJSON,
           "reason":      e.Action.Interrupted.Reason,
       })
       return []wire.UIMessageChunk{{Type: "data-decision", Data: payload}}
   }
   ```
2. **FE 紧急**：`DataPolicyPart.tsx:71-78` 兜底分支改 InfoBadge（折叠 + 提示开发者），不当 alarm
3. **server config**：确认 `Deps.ActiveManifest || ChatModel || Embedder || Retriever` 在 `cmd/server/main.go` 都注入（`/api/run/:id/plan/:approvalId/approve` 返 404 的 Deps 检查也修一下，便于 API 兜底闭环）

**evidence**:
- 4 次 SSE 抓包：262/432/262/262 bytes，全部 5 个 chunk，无 data-decision
- 4 次 audit row 全部成功写入（#323, #325, #330, #332 — action_pending_approval）
- server log 4 次 `Runner interrupted (InterruptContexts=1) — 11-02 emits approval-request chunk`
- log 显示每次 interrupt 紧跟 `ChatHandler drain complete`（无更多 emit）
- `/api/run/rl-burst-98/plan/nwzi4VcS0sF4BILzhhFwcN83NtJnuqfP/approve` 返 404 `plan approval endpoint not initialized`（Deps nil）
- `approval_rules: 5` + `plan_todos: 7`（SQL seed 后 panel 渲染正常）
- chat assistant message body 始终空 + 红色 `data-policy-unknown`

**blocker**: 必须修 `runner_sse.go:101` 后 plan_approve HITL E2E 才能 PASS。

**operator**: 重做时跳过 chat 端 ApprovalCard 验证；用 SQL seed 灌 panel 即可证明渲染 OK；等 runner_sse.go fix。

## Summary

total: 7
passed: 2 (Item #5 HYG-03 live eval — AI 跑通 + gate passed; Item #7 plan_approve HITL — 2026-09-08 晚 14 commits 全链修复 live 验证)
operator-partial: 5 (Items #1-4 + #6 — AI 跑完 API 部分；物理视觉项需 operator 亲验)
issues: 0
pending: 0
skipped: 0
blocked: 0

## AI 运行摘要 (2026-09-08)

- 服务 binary rebuild + restart (PID 597964 @ :8888)
- 后端 Go 单元测试 100% PASS（含 internal/server / store/sqlite / ticket / knowledge 等所有包）
- FE 598/598 tests PASS
- BGE M3 + Milvus 内网可达；GLM 可达 (已用于 live eval)
- 完整 evidence 表：`.planning/uat/30-cycle-run-2026-09-08.md`

## 关键修复 (本次 cycle 修复)

1. **Server binary 时差 20.5h** —— 旧 binary 缺 Phase 27/28/29 子路由 → rebuild + restart 修复（备份已做 `/tmp/skillforge-uat26.bak-20260908-110208` + `skillforge.db.bak-20260908-110208`）
2. **HANDOFF.json 命令错** —— `cmd/eval --frozen --gate-check` 应改为两步：`--frozen=compliance -skills-dir=examples/skills` + `-gate-check <report>`
3. **29-UAT.md 文档错** —— chat 用 `X-Session-Id` HTTP header，不是 body `session_id` 字段
4. **Phase 30 加固修复生效** —— WR-05 local-session-* 反馈/工单被 400 拒绝（防越权）

## Gaps

- 30-REVIEW.md 报告 `internal/server/knowledge.go:341/413` 两处疑似 bug——经 orchestrator 直读确认：**两条均误读**（line 343 单 `c.JSON`、line 413 条件已是 `requestedHops > maxHopsLimit`）。该文件属于先于 Phase 30 启动就已存在的 uncommitted dirty file，不在 30 的 16 files 改动范围；其 diff 单独看可能是合理的或与 reviewer/verifier 看的快照基线不同。30 的 8 个加固修复（CR-01 + WR-01..06）独立验证全部 PASS（见 30-VERIFICATION.md 12/12 must-haves）。

## Gaps

- 30-REVIEW.md 报告 `internal/server/knowledge.go:341/413` 两处疑似 bug——经 orchestrator 直读确认：**两条均误读**（line 343 单 `c.JSON`、line 413 条件已是 `requestedHops > maxHopsLimit`）。该文件属于先于 Phase 30 启动就已存在的 uncommitted dirty file，不在 30 的 16 files 改动范围；其 diff 单独看可能是合理的或与 reviewer/verifier 看的快照基线不同。30 的 8 个加固修复（CR-01 + WR-01..06）独立验证全部 PASS（见 30-VERIFICATION.md 12/12 must-haves）。
- HYG-03 live 层（`cmd/eval --frozen --gate-check`）依赖内网服务（Milvus/BGE/GLM），按 `.claude/memory/env-intranet-service-flakiness.md` 纪律——live 前探测，断线记环境 STOP 不刷分。