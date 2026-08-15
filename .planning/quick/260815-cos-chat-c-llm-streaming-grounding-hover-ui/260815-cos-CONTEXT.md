# Quick Task 260815-cos: chat 流式 C 方案 + 引用 hover + 置信度隐藏 - Context

**Gathered:** 2026-08-15
**Status:** Ready for planning

<domain>
## Task Boundary

实现 chat 端到端流式输出（C 方案：LLM 草稿流式 + "待校验"标签，grounding 失败回退到 RefusalCard）+ 引用角标 + hover 弹层直接拉 chunk 全文 + VerdictCard 隐藏置信度 + HistoricalRefutationCard 仅冲突时显示。

约束：保留 SkillForge "无未验证结论"信任契约（结论只在 grounding 通过后渲染真文本）。

</domain>

<decisions>
## Implementation Decisions

### A. 占位文本（A2 — 用户选定）
- LLM streaming 草稿：流式输出 LLM 真实生成的结论草稿文本
- 主区顶部贴一个小 chip：grounding 未通过时显示「🟡 待校验」；grounding 通过后变为「✅ 已验证」并保留文本
- 与既有的 4 阶段 subprogress（检索/候选/生成/自校验）正交共存

### B. grounding 失败回退 UX（B1 — 用户选定）
- 草稿直接被 RefusalCard 替换（含拒绝原因）
- 用户看不到"未验证文本"，与架构契约一致
- 不做 Claude 风格的 strike-through 修订样式

### C. CitationCluster hover 弹层（C3 — 用户选定）
- 角标 `[1]` `[2]` 形式浮在结论正文右上角
- hover 触发弹层，按需 fetch `GET /api/knowledge/chunk/:id`（路由已有于 260813-dno）
- 弹层内直接渲染 chunk 全文 + doc 元数据 + 来源 chunk 编号

### D. ConfidenceScore 字段处理（沿用 260815-0xb 决策）
- VerdictCard UI 彻底移除绿色置信度条渲染
- 后端 verdict JSON 保留 confidence 字段不动（eval 仍用）
- 历史层反证卡片不显示置信度

### E. HistoricalRefutationCard（E1 — 用户选定）
- 现状：永远在结论底部出现空状态卡片
- 改造后：仅当真的检出冲突条目时才渲染卡片
- 无冲突时完全消失（不占位）

### Claude's Discretion
- chip「🟡 待校验 / ✅ 已验证」的精确文案与样式（沿用现有 token 风格）
- 角标 hover 弹层的触发方式（hover vs click）—— 沿用主流做法：hover 触发，鼠标移出消失
- 草稿流式的速率节流（throttle/coalesce）参数—— 按 SSE 原生频率透传，不人为节流
- VerdictCard 中移除 ConfidenceScore 后剩余空间的利用（如有额外置信度提示/不利用）

</decisions>

<specifics>
## Specific Ideas

- C 方案的关键技术挑战：BE pipeline 需要在 LLM 调用阶段切换为 streaming 模式（Eino ChatModel.Generate vs Stream），同时保持 grounding 校验 pipeline 结构不变
- 既有 FE 通道：`DataSubprogressPart` 已渲染 `data-subprogress` chunk；本次新增的"草稿文本"是另一种 chunk 类型（如 `text-delta-unverified`）还是扩展现有 `text-delta` 增加 `verified` 字段？需要在 research 阶段决定
- chunk 全文 GET API：`internal/server/handler_knowledge.go` 中已有 `GetChunk` 处理函数（260813-dno 任务落地），FE 可以直接调用

[If none: "No specific requirements — open to standard approaches"]

</specifics>

<canonical_refs>
## Canonical References

- `internal/server/pipeline.go` — 既有 VerdictPipeline，决定 LLM streaming 接入点
- `internal/wire/chunk.go` — 既有 wire 类型白名单，新增 chunk 类型需要扩这里
- `web/src/components/chat/VerdictCard.tsx` — 待改造的置信度条 + 引用展示
- `web/src/components/chat/CitationCluster.tsx` — 待改造为角标 + hover 弹层
- `web/src/components/chat/HistoricalRefutationCard.tsx` — 待按 E1 改造
- `internal/server/handler_knowledge.go` — 既有 GetChunk 路由，hover 弹层用

[If none: "No external specs — requirements fully captured in decisions above"]

</canonical_refs>