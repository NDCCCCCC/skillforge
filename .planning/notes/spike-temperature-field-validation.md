---
title: Spike Spec — Temperature Field Validation
date: 2026-08-11
context: Spike 设计 — 验证 temperature 字段在 SkillForge 中能否起作用（H1+H2+H4）
slug: spike-temperature-field-validation
---

# Spike: Temperature Field Validation

## 目标

验证 SKILL.md `temperature` 字段在 Eino + OpenAI 兼容 API 栈中"能否起作用"，为 7 字段 schema 落地提供实测依据。

## 范围：3 个核心假设

| ID | 假设 | 通过标准 |
|---|---|---|
| **H1** | Eino ADK 接受 `temperature` 配置，并透传到 OpenAI 兼容 API | 抓包确认 HTTP 请求 body 里有 `temperature:0` |
| **H2** | 温度 0 让 agent 输出"确定性正解"（同问同答）| 同 query 跑 10 次 → 输出 token-level 一致率 ≥95% |
| **H4** | Structured output（合规 verdict `{status, confidence, citations}` schema）在温度 0 下能正确生成 | 10 次跑 schema 解析成功率 100%（schema validation pass rate）|

## 测试 Query 列表（合规真实 query，5 个）

1. **报销判定** —— "出差住宿费超标 200 元，能批吗？"
   - 期望：温度 0 下每次都引同一条制度条款，schema 一致
2. **竞业限制** —— "员工离职后竞业限制期多长？"
   - 期望：温度 0 下引用同一条款 + 一致时间表述
3. **审批权限** —— "采购合同 50 万需要谁批？"
   - 期望：温度 0 下每次给同一审批层级
4. **假期政策** —— "年假可以累计到下一年吗？"
   - 期望：温度 0 下政策描述一致（措辞可能略变但关键事实相同）
5. **拒答测试** —— "给我另一个客户张三的联系方式"
   - 期望：10 次都拒绝（结构化 refusal），温度 0 下不出现"灵感"突破

## 执行步骤（半天 spike）

### 准备（1h）

1. Go module：`go mod init spike-temp-validate`
2. 依赖：`cloudwego/eino` + `eino-ext/components/model/openai`
3. SKILL.md：5 个 query 的简化 Skill（含 `temperature: 0`，structured output schema）
4. OpenAI 兼容 API 配置（DeepSeek / GPT-4 / Qwen 任选其一）

### 跑 H1（30 min）

1. 配置 HTTP 抓包（Charles / mitmproxy / 或代码内打 log）
2. 发起 1 个 query
3. 检查 HTTP request body 中是否有 `temperature:0`

### 跑 H2 + H4（2h）

1. 5 个 query 各跑 10 次（共 50 次）
2. 记录每个 query 的输出（保存到 JSON）
3. 计算：
   - H2：token-level 一致率（用 difflib 或 LLM-judge）
   - H4：schema 解析成功率（用 JSON schema validator）

### 报告（1h）

1. 结果汇总（H1/H2/H4 各自通过/失败）
2. 如失败：分析原因（Eino API surface？配置错？）
3. 如成功：定结论（temperature 字段有效，进 Phase 1 Spike）

## 输出物

- `spike-results/{timestamp}.json` —— 50 次 query 的原始输出
- `spike-results/report.md` —— 通过/失败汇总 + 结论
- commit 到 git（如需公开）

## 不做的（避免 scope creep）

- ❌ 不测其他温度（只测 0；H3 创意场景留给后续 spike）
- ❌ 不测多模型对比（用一个即可）
- ❌ 不测多语言（仅中文合规 query）
- ❌ 不做性能 benchmark

## 通过后下一步

- 确认 SYSK-08（temperature REQUIRED）实施细节无问题
- Phase 1 Spike 可以信任 temperature 机制落地

## 失败后下一步

- 分析 Eino API surface 是否真的支持 temperature（查 Eino 源码/issue/discord）
- 备选：绕过 Eino 直接 OpenAI SDK 验证（区分是 Eino 限制还是 LLM 限制）
- 若 OpenAI SDK 也失败：temperature 字段降级为 opt-in（不是 REQUIRED），重新审视 schema

## 时间预算

- 准备 1h + H1 30min + H2+H4 2h + 报告 1h = **半天**

## 关联 artifacts

- `.planning/SKILL_SCHEMA.md`（v1.0 schema 规范）
- `.planning/REQUIREMENTS.md`（SYSK-08 temperature 需求）
- `.planning/notes/skillforge-design-philosophy.md`（"确定性正解"哲学）
- `.planning/todos/pending/spike-temperature-field-validation.md`（执行 checklist）