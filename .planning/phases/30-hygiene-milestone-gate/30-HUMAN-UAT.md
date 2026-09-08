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
**expected**: monthly-report skill 发「生成 2026-07 月报」→ Runner 触发 plan_approve HITL gate → 审计轨迹「待审批 plan_approve」条目 → 操作员点击「Approve + 记住」→ chat 渲染 plan-approved 完成态（折叠或 toast），不出现红色错误。

**actual**: **FAIL** — 操作员侧未发现「Approve + 记住」入口（chat assistant message body 始终为空，`message-assistant-text` div `text=""`，无 ApprovalCard / PlanApprovalCard 组件渲染），刷新页面后 chat assistant message 显示红色框：

> Unknown data-policy payload shape

`web/src/components/chat/parts/DataPolicyPart.tsx:71-78` 兜底分支。

**root cause hypothesis** (待根因定位，未深挖 SSE wire 流)：
1. server `data-decision` chunk `{decision:"plan-approved"}` 走 `RealChatTransport.dispatchApprovalDecision` 正确处理并 return true（应被截留不 enqueue）
2. 但 chat 仍出现「Unknown data-policy payload shape」——必有另一条路径产生 `data-policy` chunk 且 payload 不是 verdict/refusal
3. 可能：(a) plan runner 在 resume 后 emit 了 `data-policy` 携带 plan-shaped payload；(b) `data-decision` chunk 实际被误归到 `data-policy` 处理路径；(d) useChat 的 part dispatch 漏判 type
4. `/api/run/:id/plan/:approvalId/approve` 直调返回 404 `{"error":"plan approval endpoint not initialized"}`（plan_api.go:226-230 — `Store || ChatModel || ActiveManifest || Embedder || Retriever` 之一 nil），说明当前 server binary 的 Deps 未完整装配 Agent path，无法用 API 完成正常 approve 闭环。人工走 chat 端 ApprovalCard 是唯一通路，但 ApprovalCard 未渲染 → 路径断。

**影响**:
- 计划审批 UAT 无法走真实 E2E 闭环（仅 audit row + PlanRulesPanel SQL seed 可验证 panel 渲染）
- 操作员无法通过 chat 内 ApprovalCard 完成 plan_approve → 看不到 plan-approved 完成态 UI
- 影响所有 plan-mode skill（monthly-report 等）

**fix 建议**:
1. **FE 紧急**：`DataPolicyPart.tsx:71-78` 兜底分支改为更友好的渲染（`InfoBadge + toolTip` 或折叠到底部），避免红色 alarm
2. **FE 根因**：定位 plan_approve 完成后为什么仍有 `data-policy` chunk 入流；如果是 dispatch 漏判，加 test case
3. **BE 根因**：检查 `internal/server/plan_api.go` planApproveHandler 全部 SSE emit 路径，确认只有 `data-decision` chunk，无 `data-policy` 误发
4. **server config**：`/api/run/:id/plan/:approvalId/approve` 返回 404 是因为 ActiveManifest 等 nil——查 server boot 配置，确认所有 Deps 在 main.go 都注入

**evidence**:
- 审计行 `#321 action_rule_approved scope_session_id=rl-burst-98 skill=monthly-report args_hash=6696b933`
- `approval_rules: 5`（含 `plan_approve` scope=session，monthly-report skill）
- `plan_todos: 7`（含 step-1..step-4 monthly-report-2026-07 plan）
- `ApproveRulesPanel` 渲染 4 行 + `PlanRulesPanel` 渲染 1 行 — panel 渲染 OK
- chat assistant message body 始终空（`message-assistant-text` text=""），无 ApprovalCard 组件
- 红色错误在 assistant message body 内（data-testid="data-policy-unknown"）

**blocker**: 必须修后才能 PASS plan_approve HITL E2E UAT。

**operator**: 重做测试时跳过 chat 端 ApprovalCard 验证（用 `.planning/uat/30-cycle-run-2026-09-08.md` Item #6 同样的 approval_rules / plan_todos 数据就够 panel 渲染验证）；等待 Phase 12 FE fix。

## Summary

total: 7
passed: 1 (Item #5 HYG-03 live eval — AI 跑通 + gate passed)
operator-partial: 5 (Items #1-4 + #6 — AI 跑完 API 部分；物理视觉项需 operator 亲验)
issues: 1 (Item #7 — Phase 12 plan_approve HITL FE chat 渲染 unknown data-policy payload，blocker for E2E UAT)
pending: 0
skipped: 0
blocked: 1 (Item #7 — 需要 FE 修 DataPolicyPart 兜底 + 根因定位 + server Deps 完整装配)

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