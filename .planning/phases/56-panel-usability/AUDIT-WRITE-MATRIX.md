# AUDIT-WRITE-MATRIX（56-04 Task 2 排查产物）

> 逐条回答「runner 完成路径的 audit verdict 行写了没 / plan_todos 行写了没 /
> 审计轨迹 tab 能看到吗 / 规划轨迹 tab 能看到吗」。证据 = 源码 grep +
> 回归测试（56-05 消费）。修复保持 T-3-AUDIT INSERT-only。

| # | 完成路径 | audit verdict 行 | plan_todos 行 | 审计轨迹 tab | 规划轨迹 tab | 判定 |
|---|---------|-----------------|---------------|-------------|-------------|------|
| 1 | chat 正常回答（pipeline verdict）| ✅ writeAuditAndTurn(chat.go:522) | 仅 deep+write_todos 时有（writeTodosAuditedTool）| ✅ | 深模式 ✅/chat 空=**预期空·引导** | 无洞 |
| 2 | refusal | ✅ writeAuditAndTurn(chat.go:502, refusal_reason) | 同上 | ✅ | 同上 | 无洞 |
| 3 | chitchat | ✅ writeAuditAndTurnWithContent(chitchat.go:134) | 预期空 | ✅ | 预期空 | 无洞 |
| 4 | internal_error | ✅ chat.go:486 | — | ✅ | — | 无洞 |
| 5 | **approval resume 后的回答** | ❌ **drainResumeEvents 绕过 writeAuditAndTurn（真洞）** → ✅ **已闭合**：INSERT `action_resumed_answer` 标记行（关联 approval_id/decision；verdict 提取 deferred——行存在且诚实不携带 verdict）| resume 后工具继续，deep+write_todos 照常写入（runner 内部路径）| ❌→✅ 已闭合（行存在后 tab 可见）| ✅（runner 路径不变）| **已修** |
| 6 | plan_approve resume | 同 #5 的判定（同一 drain 路径）→ ✅ 已闭合 | plan resume 不产 todos（approve 是门不是步骤）| ✅ 已闭合 | 预期空·引导 | **已修** |
| 7 | fanout 子代理 | action_fanout_* 独立写入（deep_audit.go）| fanout 自有 | ✅ | — | 无洞 |

## 结论
- **唯一真洞 = #5/#6（approval/plan resume 后回答缺 audit 行）**——已以
  `action_resumed_answer` INSERT-only 标记行闭合（56-04 Task 2）。
- plan_todos 的 chat 模式空 = **数据侧无洞、预期空**——落 Task 3 的 UI
  空态引导（「规划仅 deep 模式在 agent 写入 todo 时记录」），不修写入侧。
- 回归：server 层测试对 #5 断言 `action_resumed_answer` 行存在
  （TestResumeAuditMarker，56-05 消费进 verify-phase-56.sh）。
