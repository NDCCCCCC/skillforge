---
status: complete
quick_id: 261125-oeh
date: 2026-11-25
---

# Quick 261125-oeh Summary — 管线 Step:0 决策渲染为注解行，不再出现空 Hop 0 折叠条

## 结果

用户上报 Agent 状态面板三个现象：① "Hop 0 · 决策: X" 展开为空；② 与上方"执行步骤"树重复；③ Hop 0 编号令人困惑。

根因：VerdictPipeline（非多跳 agent 路径）的侧信道决策事件（历史改写/分类器/反证检查/最终回答）走 `data-decision` 但 step 恒为哨兵 0（`internal/server/pipeline.go:235/320/445`、`chat.go:377`），前端 `deriveHops` 把它们与真 agent 跳混在一起渲染成折叠条——而它们从不携带工具调用/检索/推理，折叠体必然为空；多条 step=0 还导致 `key={hop.step}` 冲突。

修复（`d62abc8`，全部 FE）：
- `deriveHops` 返回 `{ hops, annotations }`：显式 `step === 0` 的事件分流为 `PipelineAnnotation`，不再生成空 hop；真 agent 跳（step ≥ 1）渲染不变。
- 注解渲染为紧凑单行块（`data-testid="inspector-annotation"`）：决策中文标签（复用导出的 `formatDecisionLabel`）+ query + 置信度；streaming 时最后一条标「进行中」。无折叠体。
- 头部文案：有真跳 → 「当前回合 N 跳」；仅注解 → 「管线决策 N 条（无多跳）」。
- hop key 改为 `${step}-${index}` 防重复 step 冲突。
- 上方"执行步骤"树（StepTree）只聚合真 hop 的检索/工具/规划节点，管线注解回合自动落到其空态文案，重复感随之消除（双视图本身是有意设计：总览树 + 逐跳详情）。

## 验证

- `AgentInspector.test.tsx` 新增 2 例：pipeline-only（3 条 step:0 → 0 个 hop、3 条注解、头部「管线决策 3 条（无多跳）」）；混合回合（step:1 真跳 + step:0 注解各归其位）。10/10 过。
- 全量 web vitest 878/878 通过；`tsc --noEmit` 干净。

## 提交

- `d62abc8` fix(web): render pipeline Step:0 decisions as annotation rows instead of empty Hop 0 accordions
