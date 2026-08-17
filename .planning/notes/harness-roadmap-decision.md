---
title: Harness 路线图决策 — 可信×行动融合
date: 2026-08-17
context: /gsd:explore 会话（多轮记忆 → harness 对标）；驱动来自 Claude Code 式 agent harness 差距分析
---

# Harness 路线图决策

## 定位：可信×行动融合

SkillForge 从「可信问答内核」向「可信行动 harness」演进。差异化声明：

> 别人的 agent 行动是黑盒；SkillForge 的每一步行动都可审计、可拒答、可回放。

每个 harness 维度都嫁接可信机制（"可信税"）：写操作走审批门+审计行，规划轨迹可回放，子 agent 继承 tier 知识边界。**不接受**护城河稀释路线（可信降级为"有引用"）。

## 支柱顺序（用户决策 2026-08-17）

1. **多轮对话记忆** — sqlite session history 喂进 agent（当前 `internal/server/chat.go:399` 单轮不喂历史）。最大风险：历史污染 trust floor 拒答判断 → 见 spike seed [[spike-history-injection-trust-floor]]
2. **写操作 + 审批门** — 首批写工具（memory:facts 写入、审查报告生成）+ `request_approval` + 审计行。用户感知最强，「可信×行动」的第一次正面验证
3. **规划层** — DeepAgent（`adk/prebuilt/deep`，自带 write_todos），规划轨迹可回放
4. **子 agent 扇出** — multi-hop 拆并行子 agent，子 agent 继承 tier 边界

## Eino 能力事实（研究确认 2026-08-17）

- **Interrupt/Resume 一等公民**（v0.7.0+ 重构）：`compose.Interrupt(ctx, info)` / `compose.ExtractInterruptInfo(err)` / `compose.ResumeWithData(ctx, id, data)`；checkpoint 经 `compose.WithCheckPointStore` + `WithCheckPointID`
- **ADK agent 级 HITL**：`adk.NewRunner(RunnerConfig{CheckPointStore})`、`Runner.ResumeWithParams(ctx, id, &ResumeParams{...})`；`ChatModelAgentResumeData.HistoryModifier` 可在恢复时改写历史。官方文档：eino_adk/agent_hitl
- **会话存储归应用**：`Runner.Run(ctx, messages)` 每轮收全量消息列表 —— 正好对接现有 sqlite session 表；历史压缩中间件 `adk/middlewares/summarization`（token 阈值触发摘要）属 **v0.8.x 时代**，需升级
- **DeepAgent 现成**：`adk/prebuilt/deep`，`deep.New(ctx, *Config)`，`write_todos` 中间件内建（pending/in_progress/completed）
- ⚠️ eino `main` 已到 v0.10.0-alpha；CLAUDE.md 的 "v0.7.7 latest" 已过时 → 见 todo [[upgrade-deps-latest]]

## 结论

四大支柱框架全部有原生支撑，无需自研原语。唯一实质风险 = 历史注入对拒答率的污染（spike 验证）。
