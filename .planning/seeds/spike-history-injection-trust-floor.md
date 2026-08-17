---
title: Spike — 历史注入对 trust floor 拒答率的影响
trigger_condition: 多轮记忆 phase 启动前（v0.8 harness milestone 内）
planted_date: 2026-08-17
---

# Spike：历史注入对 trust floor 拒答率的影响

## 假设

把 sqlite session history 喂进 agent（修复 `internal/server/chat.go:399` 的单轮限制）后，历史内容可能污染 trust floor 判断：

- **H1**：历史中的旧 chunk 引用让 agent 误以为当前轮已检索（跳过 retrieve → 引用悬空）
- **H2**：历史中的拒答上下文让 agent 过度拒答（refusal_accuracy 虚高但 answer_relevance 崩）或反之不再拒答
- **H3**：历史中的用户表述被当作新证据（tier 边界穿透）

## 方法

用 golden set（eval/sample.jsonl，50 例）跑两组对比：

- **baseline**：当前单轮行为，冻结基线 eval/frozen/compliance.lock.json
- **history-injected**：相同 query 前拼接构造的多轮历史（含善意追问、恶意注入、旧拒答上下文三类）

对比 faithfulness / context_precision / answer_relevance / refusal_accuracy 四指标。

## 通过标准

history-injected 组四指标相对 baseline 漂移 < 5%（或明确识别出哪类历史有毒，产出过滤规则——如「历史只保留 user/assistant 文本，剥离 chunk 引用」）。

关联：[[harness-roadmap-decision]]
