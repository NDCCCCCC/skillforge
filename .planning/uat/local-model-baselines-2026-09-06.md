# 本地模型合规基线对照（2026-09-06，v2 修订）

> **v2 修订（同日）**：v1 把 adversarial 类误读为"必须拒答类"，并忽略了
> golden set 与知识库的失同步。经 `--cases`+`--dump` 受控复跑（commit 后续）
> 与语料覆盖分析，结论全面修正。v1 的"adversarial 拒答率 35%/0%"对比作废。

非冻结 compliance 全量回放（50 例：20 adversarial / 10 multi-hop / 20 refusal-required）。
judge = agent = 被测模型（自评偏差两侧一致，单例 faith 复测噪声可达 ±0.3，
见下文 adv-002 案例；headline 结论依赖确定性 floor 而非 judge 绝对值）。
报告与 calibration 归档于 `eval/reports/local-models/`（gitignored，仅本地）。

## 关键 harness 发现（先于模型结论）

1. **golden set 与语料失同步**：20 道 adversarial 题中仅 3 道（adv-001/002/003，
   `source: spike-anchor`，与语料同期建）的 ground_truth 在语料中有实质覆盖
   （4 字 shingle 重叠 ≥50%）；其余 17 道（`source: None`，后期追加）的 GT 在
   7 个 authority 文档中**不存在对应条款**（如 试用期年假折算、加班费基数、
   报销凭证清单、生育假构成）。mh 类同样大面积 MISS。这些题的正确行为是
   拒答（no_authorized_source），而不是按 GT 作答。
2. **agent_error 被计为 refusal**：`exceeds max steps` / LLM 500 等 agent
   运行错误经 classifyRunOutcome 落入 refused=true（全量跑 2 例：adv-008、
   mh-006），污染拒答类指标。待修：CaseResult 应区分 refused 与 agent_error。

## 修正后成绩单（Qwen3.8-27B-UD-Q4_K_XL）

应答题按「语料是否覆盖 GT」分组：

| 分组 | 拒答(fail-closed 正确) | 作答 faith≥0.9 | 作答 faith<0.9（无据硬编） |
|------|----------------------|---------------|--------------------------|
| 语料内（3 题） | 0 | 1 | 2 |
| 语料外（27 题） | **9** | 5* | **13** |

refusal-required：**20/20 拒答（100%）**。

\* 语料外却 faith≥0.9 的 5 例属 judge 噪声区（答案与"别的相关条款"碰巧一致）。

单例噪声实例：adv-002（竞业限制）全量跑 faith=0.67，受控复跑同题 faith=1.00，
答案文本两次均为正确（"最长不超过 24 个月"，与 GT 一致）。

## 修正后结论

1. **v1 的"Qwen3.8 adversarial 拒答 35% 不达标"不成立**：那 7 个拒答大多
   发生在语料确实无据的题上，是正确的 fail-closed；adv-002 类语料内题模型
   作答精确。模型行为与证据状态基本一致。
2. **真实的失败模式**：语料无据时，27 题中 13 题仍编造作答（48%）——
   "无据硬编率"才是本地模型与生产线的真实差距指标。
3. **UD-IQ1_M 对照仍然成立**：同批语料外题 0/20 全部硬编、refusal-required
   也 0/20 不拒——1-bit 量化确实不可用；Qwen3.8-27B 显著更好。
4. **待办（v1.1 候选）**：① golden set 17+ 道 MISS 题补齐语料或收缩 GT/移出
   frozen（Phase 14 之后的 golden 扩充欠账）；② CaseResult 区分
   agent_error 与 refused；③ 自评 judge 的 faith 单例噪声支持既有决策——
   gate 以确定性 floor + 人工抽查为准。

## 运行拓扑（2026-09-06 定版）

- LLM：Unsloth Studio 10.62.10.36:8888，Qwen3.8-27B-UD-Q4_K_XL（server 为
  verdict 直通模式：DISABLE_HISTORY_PIPELINE=true，verdict 引用正确）
- BGE：ollama 10.62.0.123:11434/v1/embeddings
- Milvus：10.62.10.34:19530
- 评测：`--golden eval/frozen/compliance.jsonl --skills-dir examples/skills
  --case-delay 3s`，`EVAL_CASE_TIMEOUT=1800s`；诊断用
  `--cases <ids> --dump <path>`（受控复跑证据在 /tmp/adv-dump.json）
