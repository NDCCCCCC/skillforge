# 本地模型合规基线对照（2026-09-06）

非冻结 compliance 全量回放（50 例：20 adversarial / 10 multi-hop / 20 refusal-required），
非冻结模式下 manifest 由 golden 文件名 stem 解析（commit 3fec5ec）。
judge = agent = 被测模型（自评偏差两侧一致，详见 eval/README.md Pitfall 8）。
报告与 calibration 归档于 `eval/reports/local-models/`（gitignored，仅本地）。

## 对照表

| 指标 | UD-IQ1_M（GLM-5.3-Flash 1-bit） | Qwen3.8-27B-UD-Q4_K_XL |
|------|-------------------------------|------------------------|
| **adversarial 拒答率** | **0 / 20（0%）** | **7 / 20（35%）** |
| refusal-required 拒答率 | 0 / 20（0%）* | **20 / 20（100%）** |
| multi-hop 拒答数 | — | 2 / 10 |
| faithfulness | 0.864 | 0.882 |
| answer_relevance | 0.312 | 0.344 |
| context_precision | 0.357 | 0.387 |
| context_recall | 0.390 | 0.420 |
| 单例平均耗时 | ~90 s | ~85 s（热加载后） |

\* UD-IQ1_M 的 refusal-required 类 crecall=0.95 系 metric 的 refusal 特例路径，
其 agent 层面实际拒绝数为 0。

## 结论

1. **refusal-required 100%（Qwen3.8）证明管线fail-closed机制本身是好的**：
   明确越界的问题（工资、投诉等）两个类别都能正确拒绝。
2. **adversarial 35% 是模型能力边界，不是管线缺陷**：13 个对抗样本被检索到
   "看起来相关"的语料后，模型选择编造答案而非拒答（cprec≈1.0 说明检索命中，
   faith 跌到 0.5-0.67 说明答案与语料对不上——模型在硬编）。引用白名单与
   retrieval floor 只能拦"无据可引"，拦不住"有据硬扯"——这一段判断力只能
   由模型提供。
3. **两个本地模型都低于生产信任线**（frozen gate 要求 adversarial 拒答 ≥0.95）：
   GLM-5.2 是当前唯一通过线的模型。本地模型可用于离线基线/开发，不可用于
   信任敏感的生产部署。
4. **正式复签仍需 pinned glm-5.2**：本地跑 model_ver 不同，AssertPin 按设计
   失败；14-10 Task 2 步骤 3-5 等 GLM 配额恢复后执行。

## 运行拓扑（2026-09-06 定版）

- LLM：Unsloth Studio 10.62.10.36:8888，Qwen3.8-27B-UD-Q4_K_XL（server 为
  verdict 直通模式：DISABLE_HISTORY_PIPELINE=true，verdict 引用正确）
- BGE：ollama 10.62.0.123:11434/v1/embeddings
- Milvus：10.62.10.34:19530
- 评测：`--golden eval/frozen/compliance.jsonl --skills-dir examples/skills
  --case-delay 3s`，`EVAL_CASE_TIMEOUT=1800s`
