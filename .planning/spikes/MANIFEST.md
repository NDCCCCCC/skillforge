# Spike Manifest

## Idea

验证 SKILL.md `temperature` 字段在 SkillForge 技术栈（Go + CloudWeGo Eino + Eino ADK + OpenAI 兼容 API）中**能否起作用**。这是"确定性正解"设计哲学（`.planning/notes/skillforge-design-philosophy.md`）的工程化验证 —— 如果 temperature 字段无效，SKILL.md v1.0 第7字段必须降级。

## Requirements

- **Stack**: Go + CloudWeGo Eino (`cloudwego/eino`) + Eino ADK (`eino-ext/adk`) + OpenAI 兼容 API（DeepSeek / GPT-4 / Qwen 任选）
- **Query**: 5 个中文合规真实场景 query（报销判定 / 竞业限制 / 审批权限 / 假期政策 / 拒答测试）
- **Structured output**: JSON schema（合规 verdict `{status: 合规|不合规|需补充, confidence, citations, review_required}`）
- **独立验证**: 透传 / 确定性 / structured output 三个假设必须独立 spike
- **通过标准**（spike spec 见 `.planning/notes/spike-temperature-field-validation.md`）:
  - 透传：HTTP wire body 包含 `temperature:0`
  - 确定性：同 query 10 次输出 token-level 一致率 ≥95%
  - Structured output：JSON schema 解析成功率 100%
- **失败 fallback**: 若透传失败 → Eino API 排查 → 备选 OpenAI SDK 直接验证；若都失败 → temperature 字段降级为 opt-in

## Spikes

| # | Name | Type | Validates | Verdict | Tags |
|---|------|------|-----------|---------|------|
| 001 | eino-temperature-passthrough | standard | Given SKILL.md declares `temperature:0`, when Eino ChatModelAgent runs, then HTTP wire body contains `temperature:0` | ✓ VALIDATED | eino, openai-compat, temperature |
| 002 | temperature-zero-determinism | standard | Given `temperature:0` + 同 query 跑 10 次 × 5 合规 query, when 提取所有输出, then token-level 一致率 ≥95% | ✗ INVALIDATED | temperature, determinism, compliance, reasoning-model |
| 003 | structured-output-temperature-zero | standard | Given `temperature:0` + 合规 verdict schema, when 跑 10 次, then JSON schema 解析成功率 100% | PENDING | temperature, structured-output, json-schema |

## Related Artifacts

- `.planning/notes/spike-temperature-field-validation.md` — 完整 spike spec（query 列表 / 执行步骤 / 时间预算）
- `.planning/todos/pending/spike-temperature-field-validation.md` — 执行 checklist
- `.planning/SKILL_SCHEMA.md` — SKILL.md v1.0 schema（含 temperature 字段）
- `.planning/REQUIREMENTS.md`（SYSK-08）— temperature REQUIRED 需求
- `.planning/notes/skillforge-design-philosophy.md` — "确定性正解"哲学