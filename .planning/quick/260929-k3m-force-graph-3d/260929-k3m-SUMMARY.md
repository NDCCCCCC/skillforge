---
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: unknown
---

# Quick 260929-k3m: 本体视图渲染层 → react-force-graph-3d Summary

**把本体视图（OntologyGraphView）的渲染层从 cytoscape 2D 画布整体换成 react-force-graph-3d 真三维力学图谱：节点拖拽沿 d3-force 实时传播、滚轮环绕相机、定向流动粒子标示关系方向；数据管线（focus 解析 / notes= fetch / toggle / 统计 / 详情侧栏）零改动。**

- **Date:** 2026-09-29
- **Commit:** `5480ac7` feat(quick-260929-k3m): swap ontology view rendering to react-force-graph-3d
- **Scope:** web/src/components/graph/OntologyGraphView.tsx（渲染层重写）+ 测试重写 + 宿主测试 mock 换血 + 依赖

## What Was Done

| # | Item | Detail |
|---|------|--------|
| 1 | 渲染层替换 | cytoscape mount/cy.on/style 全部删除，换成 `React.lazy(() => import('react-force-graph-3d'))` + Suspense（fallback「正在加载三维引擎…」）；three.js 不进主 chunk |
| 2 | 映射 | graphData: nodes `{id, name: label, kind, tier, val: note?6:3}`、links `{source, target, type}`；nodeColor/nodeLabel/linkColor 全按 kind/type 着色 |
| 3 | 配色 | CSS token 在渲染时解析为具体 rgba（three.js 读不了 `var()`）；note=--color-primary / table=needs-info / product=compliant / organization=#a78bfa；parent_org 2D 虚线 → 3D 半透明 0.5 alpha |
| 4 | 动态感 | `linkDirectionalParticles: 3, speed: 0.006`（流动粒子）+ `linkWidth: 1.5`；hover 1.8× 放大（onNodeHover → hoverId → nodeVal accessor） |
| 5 | 交互 | onNodeClick → 详情侧栏（照旧）；onBackgroundClick → 清除；节点拖拽力学联动（库内建 d3-force 实时传播）；左键旋转/右键平移/滚轮缩放（页脚提示文案已更新） |
| 6 | 布局/适配 | `d3Force('charge').strength(-120)` + `link distance 60`（160 节点可读密度）；zoomToFit(800, 40) 三触发点：适应按钮 / 首次 onEngineStop / ResizeObserver 首次真实高度（每数据集一次，drag reheat 不重触发） |
| 7 | 尺寸 | 宿主 ResizeObserver → 显式 width/height props（不传则退化默认尺寸）；零高度守卫语义与 cytoscape 版一致 |
| 8 | 保留不动 | focus sources/chunk 双路解析、fetch hold、仅当前问题/全库 toggle（共享 localStorage key）、legend 统计、空/加载/403 态、min-h 地板、详情侧栏、宿主 KnowledgeGraphBrowser 全屏/Esc |

## Verification Evidence

- **tsc**: `npx tsc --noEmit` → exit 0（全项目）
- **vitest**: `src/components/graph + src/components/admin + KnowledgeGraphBrowser.test` → **113/113 passed (13 files)**；全仓 sanity 跑 675/676（唯一失败为 pre-existing，见 Deferred）
- **build**: `npm run build` 成功；three 进独立 chunk：`dist/assets/react-force-graph-3d-*.js 1,399.67 kB (gzip 377.16 kB)`，主 chunk 不含 three
- **cytoscape 死引用**: grep 确认仅剩 governance/ProvenanceGraphView.tsx（其自身在用）；cytoscape 依赖按约束保留

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] npm --legacy-peer-deps 剪掉了两个未显式声明的 peer 依赖**
- **Found during:** tsc / build 验证
- **Issue:** 安装 react-force-graph-3d 时 `--legacy-peer-deps` 重解析锁文件，把先前以 auto-peer 形式存在于 lockfile 的 `@testing-library/dom@10.4.1`（RTL 16 peer）与 `zod@4.4.3`（ai@7 peer，`ai/dist/index.js` import `zod/v4`）剪掉 → 全仓 RTL 类型爆 2305、vite build 失败
- **Fix:** 显式入库 `@testing-library/dom@^10.4.1`（dev，解析 10.4.2）与 `zod@^4.4.3`（deps，解析 4.6.5），锁文件不再依赖 auto-peer
- **Files:** web/package.json, web/package-lock.json
- **Commit:** 5480ac7（同一提交）

**2. [测试排序修正] toggle 用例断言时序**
- **Issue:** 新写的 toggle 用例在 refetch 在途（loading 态整树替换）时断言 toggle 文案 → 必失败
- **Fix:** 先 `await findByTestId('force-graph-3d')` 待视图回来再断言文案与 localStorage
- **Files:** OntologyGraphView.test.tsx

其余：plan 执行与规格一致（版本取最新稳定 rfg3d 1.29.1 / three 0.186.1；3D 无虚线用 0.5 alpha 方案按 spec 落地）。

## Known Stubs

None — 渲染数据全部来自既有 `GET /api/admin/ontology/graph` 管线，无 placeholder/空数据渲染路径。

## Deferred Issues (out of scope)

- `src/__tests__/DetailPanel.test.tsx` 「knowledge-graph kind mounts the empty-state hero (SC-5)」**pre-existing 失败（g7x 遗留，非 k3m）**：该用例期望 BE 空库时出 `knowledge-graph-empty` hero，但 260929-g7x 起 browser 默认 mode=本体，hero 只在文档 mode 渲染（本用例 fetch stub 对 ontology 端点也返回 tiers 形状 → 归一化为空投影 → 出 `ontology-graph-empty`）。k3m 未触碰 mode 逻辑；修复属 g7x 测试期望更新（点文档 chip 后断言），留待 g7x 责任面或后续 quick。

## Notes for Verifier (活体验收)

3D 图验收点（orchestrator）：① 三维图谱渲染（透明底、kind 四色）② 拖节点 → 邻接节点力学跟随 ③ 滚轮=缩放、左键拖=旋转视角、右键拖=平移 ④ 边上粒子流动、方向=from→to ⑤ 悬停 tooltip「label · 类型」+ 节点放大 1.8× ⑥ 点节点 → 右侧详情、点空白清除 ⑦ 「适应」回到全图取景 ⑧ 仅当前问题/全库 toggle 重取数据并自动 refit。首次打开有一次性「正在加载三维引擎…」（three chunk 加载）。

## Self-Check: PASSED

- 所有声明文件存在（组件/两测试文件/SUMMARY/deferred-items）
- commit `5480ac7` 存在于 git log
- 重写文件零 cytoscape 代码引用（仅 4 处历史注释；governance ProvenanceGraphView 自身使用不受影响）
