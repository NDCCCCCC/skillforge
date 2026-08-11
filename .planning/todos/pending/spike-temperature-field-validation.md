---
title: Spike — Temperature Field Validation (H1+H2+H4)
date: 2026-08-11
priority: high
status: pending
trigger: 当 Phase 1 Spike 启动时（或独立 spike 时）
---

# Todo: Spike — Temperature Field Validation

## 目标

验证 SKILL.md `temperature` 字段在 Eino + OpenAI 兼容 API 栈中"能否起作用"。

## 范围（按 spike spec）

- **H1**：技术可行性（透传）
- **H2**：温度 0 确定性（variance）
- **H4**：Structured output 兼容

## 步骤

- [ ] 准备 Go module + Eino 依赖（`go mod init spike-temp-validate`）
- [ ] 配置 OpenAI 兼容 API（DeepSeek / GPT-4 / Qwen 任选）
- [ ] 写简化 SKILL.md（5 个 query，`temperature: 0`，structured schema）
- [ ] 配置 HTTP 抓包（Charles / mitmproxy / 代码 log 任选）
- [ ] 跑 H1：发起 1 query，检查 HTTP body 是否有 `temperature:0`
- [ ] 跑 H2：5 query × 10 次，记录输出
- [ ] 跑 H4：5 query × 10 次，schema 解析成功率
- [ ] 汇总 spike 报告到 `.planning/notes/spike-temperature-field-validation-results.md`
- [ ] commit spike 结果到 git

## 通过标准

- H1：HTTP body 有 `temperature:0` → 通过
- H2：同 query 10 次输出 token 一致率 ≥95% → 通过
- H4：schema 解析成功率 100% → 通过

## 通过后下一步

- 确认 SYSK-08（temperature REQUIRED）实施细节
- Phase 1 Spike 可以信任 temperature 机制

## 失败后下一步

- 分析 Eino API surface 是否真的支持 temperature
- 备选方案：绕过 Eino 直接 OpenAI SDK（看是 Eino 限制还是 LLM 限制）
- 若 OpenAI SDK 也失败：temperature 字段降级为 opt-in（不是 REQUIRED），重新审视 schema

## 时间预算

- 准备 1h + H1 30min + H2+H4 2h + 报告 1h = **半天**

## 关联

- `.planning/notes/spike-temperature-field-validation.md`（完整 spec）
- `.planning/SKILL_SCHEMA.md`（schema 规范）
- `.planning/REQUIREMENTS.md`（SYSK-08）
- `.planning/notes/skillforge-design-philosophy.md`（"确定性正解"哲学）