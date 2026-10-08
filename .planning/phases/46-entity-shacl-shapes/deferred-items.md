# Phase 46 — Deferred Items

## D-46-DEF-01 — Pre-existing DetailPanel SC-5 test failure（46-04 执行期发现，未修）

- **发现于：** 46-04 最终全量 FE 门（`npx vitest run`，2026-10-08）
- **现象：** `src/__tests__/DetailPanel.test.tsx > DetailPanel > knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)` — `Unable to find an element by: [data-testid="knowledge-graph-empty"]`；隔离运行确定性失败（12/13）。
- **因果排除：** DetailPanel 及其测试零 import governance 模块；46-04 四个 commit 只触及 `web/src/components/admin/governance/` 四文件；测试文件最后修改 2026-08-20（45099af，Phase 11 期）。
- **归属：** STATE.md v1.6 积压已列「DetailPanel/emitter 两个 deferred 测试」——本条即 DetailPanel 侧实例。
- **处置：** 按 SCOPE BOUNDARY 不顺手修；留 v1.6 积压（与 emitter 侧 TestEmitSubCategoryInstanceCount D-45-DEF-01 并列）。
