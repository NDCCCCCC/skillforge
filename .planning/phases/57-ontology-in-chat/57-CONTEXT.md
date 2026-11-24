# Phase 57 — 本体进 chat（ONT-02 / ONT-03）

## 目标
989 条三元组第一次参与每一次回答：query → rdfs:label 实体字典锚定 → KG 邻域
（1-2 hop）作为 prompt 上下文注入 → 向量检索 → grounding 实体校验。

## 已落地（57-01）
`internal/ontology/anchor`：实体字典 + 最长匹配 + tier 作用域 + 显式降级。

## 红线（本 phase 全部 plan 适用）
1. **正交原则（D2-03 邻域不变量）**：graph 输出只能作 prompt 上下文补充，
   **永不进 Citations**。graph-only chunk（向量相关性为零）升级为引用必须
   过向量相关性验证门 —— 否则就是 citation laundering（引用存在≠支撑成立）。
2. **KG 缺席显式降级**：nil/损坏 KG → `kg_available:false` + WARN + 纯向量，
   chat 不阻塞、不静默降质。空 KG 与「匹配到 0 个实体」是两件事。
3. **人审门不动**（ONT-04）：本体提案仍走人工批准，本 phase 不自动写 KG。
4. 零硬编码：KG 命名空间/谓词只在 `internal/ontology` 内定义一处。

## 拆分
| Plan | 内容 |
|------|------|
| 57-01 | 实体锚定字典（✅ 已交付）|
| 57-02 | KG 邻域上下文注入 + 向量相关性门（graph 不进 Citations）|
| 57-03 | grounding 实体校验（no-op stub 退役，entities:{verified,unverified} 进 verdict）|
| 57-04 | verify-phase-57.sh 收口 |
