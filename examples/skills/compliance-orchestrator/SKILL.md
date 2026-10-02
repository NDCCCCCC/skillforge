---
description: 企业合规多跳检索编排（orchestrator 父 Skill）。当用户问题需要跨多个制度主题组合证据（如「关联方认定 + 董事会审批触发」「差旅政策 + 总经理审批阈值」）时触发：classifier 拆分 2-3 个 sub-query，扇出只读子 agent 并行检索，保守合并后输出单一 verdict。单点事实查询路由到 compliance Skill；本 Skill 不处理任何写操作，子 agent 结构性只读。
agent: orchestrator
reads:
  - authority/**
  - historical/**
writes: []
steps: |
  1. Classify: derive 2-3 sub-queries with overlapping but non-identical tier filters;
     trivial / out-of-domain queries emit 0 sub-queries (single-vertex fallback).
  2. Build + Fan-out: construct read-only sub-agents via runtime.BuildSubAgent —
     tier closure = parent ResolvedTiers ∩ sub_skill Reads; empty intersection is a
     LOAD-TIME error per FAN-02 SC-2 ("tier intersection empty"), never runtime silence.
  3. Merge: deterministic key-sorted pure merge (10x byte-identity contract);
     ANY refused/faulted sub-agent → needs_input + confidence 0 (conservative D2-14 posture).
  4. Apply Trust Stage: per-sub-agent grounding BEFORE merge; merged verdict re-grounded
     against the shared chunk accumulator (chunk_id + verbatim span, D2-01).
  5. Emit single merged verdict (compliant | needs_input) with deduped citations.
guardrails:
  prohibitions:
    - "Never invoke HITL/write tools from sub-agents (sub-agents are structurally read-only, writes=[])"
    - "Never expand a sub-agent's tier boundary beyond the parent's ResolvedTiers() intersection"
    - "Never cite clauses not retrieved from the intersected tier set (chunk_id + verbatim span only)"
  required_refusal_phrases:
    - "no_authorized_source"
    - "tier_intersection_empty"
    - "low_retrieval_score"
  tier_limits:
    authority: ["read-only"]
    historical: ["read-only"]
temperature: 0.0
---

# Compliance Orchestrator Skill (agent: orchestrator)

Phase 13 / P8.4 的 orchestrator 父 Skill 示例：同一个 agent 内核通过 `agent: orchestrator`
声明切换到 FanOutOrchestrator（5-step 管道：Classify → Build+Fan-out → Merge →
ApplyTrustStage → Emit），无需改任何代码。本文件是 `eval/golden/fanout.jsonl`
20-case fan-out UAT 的 `--sub-skill-config` 目标（Plan 13-05）。

## 子 Skill 契约（sub-skills）

子 Skill 由 golden set 的每行 `sub_skills` 字段按 case 合成（name + reads），
reads 必须与父 Skill 的 `ResolvedTiers()`（`权威`、`历史`）有非空交集，
否则 `runtime.BuildSubAgent` 在构造期直接报
`tier intersection empty`（FAN-02 SC-2 load-time error）。golden set 引用的三个
canonical 子 Skill 形态：

| 子 Skill | reads | ResolvedTiers | 用途 |
| --- | --- | --- | --- |
| `compliance-multihop-1` | `authority/**` | 权威 | 当年/现行制度条款检索 |
| `compliance-multihop-2` | `authority/**` + `historical/**` | 权威, 历史 | 跨 tier 组合证据（现行 + 历史口径） |
| `compliance-multihop-3` | `historical/**` | 历史 | 历史口径 / 跨年度一致性 |

## 不变量（由平台代码强制，非 prompt 约束）

- 子 agent 结构性只读（`subManifest.Writes = []`，P13-L4）——即使子 Skill 声明 writes 也被构造期清零。
- tier 闭包在构造期以闭包烘焙（P13-L2）：prompt injection 无法扩权。
- 任何子 agent refused/faulted → 合并结果保守降级 `needs_input` + confidence 0（P13-L5）。
- 每个子 agent 合并前独立过逐字引用验证（FAN-04 SC-4，复用 Phase 12 ApplyTrustStage）。
- 子 agent 递归被结构性禁止（P13-L8）：orchestrator 不会再扇出 orchestrator。
