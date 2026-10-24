---
status: partial
phase: 29-feedback-loop-ticket-interface
source: [29-VERIFICATION.md]
started: 2026-09-08T06:30:00+08:00
updated: 2026-09-08T06:30:00+08:00
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  gap_snapshot: "partial::scenarios=9"
---

## Current Test

[awaiting human testing]

## Tests

### 1. 实时投票流程（29-UAT Test 1）

expected: 浏览器发起聊天轮次 → 助手回复携带 data-turn 帧 → 点击有帮助/没帮助 → 201 + pill 锁定 + 真实 toast（「已记录反馈：…」）
result: [pending]

### 2. 幂等重复点击 + 恢复回显（29-UAT Test 2）

expected: 重复点击返回 already_recorded toast（「该消息已反馈过：…」）；刷新/恢复会话后锁定态由 history voted_verdict 回显，无额外 POST
result: [pending]

### 3. outdated → /admin 复核 → 处置（29-UAT Test 3）

expected: 报告知识过时（可附说明）→ /admin「复核」tab 出现聚合行（报告次数）→ 处置（已更新/误报/忽略）→ closed_count toast + 信任横幅常驻
result: [pending]

### 4. 重开语义（29-UAT Test 4）

expected: 处置后同一 chunk 收到新的 outdated 报告 → 重新进入待复核（latest-disposition-wins）
result: [pending]

### 5. 指标 tab 视觉（29-UAT Test 5）

expected: /admin「反馈」tab：口径注常驻、0 票显示「—」非 0%、日/周趋势切换、待复核 cross-link 跳转
result: [pending]

### 6. 工单 provider=none（29-UAT Test 6）

expected: 创建工单 → TKT-{n} + 「未配置外发渠道，工单已留档」诚实文案；重复点击返回既有编号
result: [pending]

### 7. 工单 webhook 实发（29-UAT Test 7）

expected: 配置 SKILLFORGE_TICKET_* webhook → 建单 → 排队 → 外发携带 X-SkillForge-Ticket-Secret → dispatch 审计行可见于 /api/admin/audit；5xx 端点 → failed 审计行 + 主流程不受影响
result: [pending]

### 8. Phase 27→29 联合验收（29-UAT Test 8）

expected: 复核处置后知识库/检索零变化（处置 = 信号状态 + 审计）；知识库变更唯一来源仍是 27 上传流程（FBK-05 实机确认）
result: [pending]

### 9. 视觉走查（29-UAT Test 9）

expected: FeedbackRow 布局/间距按 29-UI-SPEC；toast 在 chat/eval/admin 三处均渲染；/admin 7 tab 顺序（反馈第 6、复核第 7）
result: [pending]

## Summary

total: 9
passed: 0
issues: 0
pending: 9
skipped: 0
blocked: 0

## Gaps

- （非 UAT，开发者决策项）CR-01 [29-REVIEW.md Critical]：SSE emit 失败路径的继承性双 audit/turn 写入在 Phase 29 后变得可投票（放大会影响 FBK-02/03 指标口径）。验证判定为继承缺陷非 phase gap；建议后续 quick fix（错误分支复用已落库 turn 而非重写）。`/gsd:code-review 29 --fix` 可自动处置。
