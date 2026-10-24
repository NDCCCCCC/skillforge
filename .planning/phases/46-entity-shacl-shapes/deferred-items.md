# Phase 46 — Deferred Items

## D-46-DEF-01 — Pre-existing DetailPanel SC-5 test failure（46-04 执行期发现，未修）

- **Status:** acknowledged

- **发现于：** 46-04 最终全量 FE 门（`npx vitest run`，2026-10-08）
- **现象：** `src/__tests__/DetailPanel.test.tsx > DetailPanel > knowledge-graph kind mounts the empty-state hero when BE returns an empty corpus (SC-5)` — `Unable to find an element by: [data-testid="knowledge-graph-empty"]`；隔离运行确定性失败（12/13）。
- **因果排除：** DetailPanel 及其测试零 import governance 模块；46-04 四个 commit 只触及 `web/src/components/admin/governance/` 四文件；测试文件最后修改 2026-08-20（45099af，Phase 11 期）。
- **归属：** STATE.md v1.6 积压已列「DetailPanel/emitter 两个 deferred 测试」——本条即 DetailPanel 侧实例。
- **处置：** 按 SCOPE BOUNDARY 不顺手修；留 v1.6 积压（与 emitter 侧 TestEmitSubCategoryInstanceCount D-45-DEF-01 并列）。

## D-46-DEF-02 — TestDeriveEdgesDeterministic 既有 flaky（46-REVIEW-FIX 期发现，未修）

- **Status:** acknowledged

- **发现于：** --fix 收尾 clean-HEAD 门禁复跑（2026-10-08）；fixer 在评审范围外观察中预警
- **现象：** `internal/knowledge` 的 `TestDeriveEdgesDeterministic` 约 1/6 概率随机失败（6 跑 1 败实测）——浮点 Weight 精确比较的非确定性闪烁；P4 全量随其随机红灯（`verify-phase-46.sh` 的 P4 waiver 只认 D-45-DEF-01 一项，flaky 命中即门禁 FAIL）
- **因果排除：** 未修改代码的主仓库同样随机失败；Phase 46 四个 wave 与 9 个 fix commit 均未触及 `internal/knowledge`
- **影响：** 门禁可信度（随机红灯需重跑确认），非功能缺陷；`verify-phase-45.sh` P4 同暴露
- **处置：** 单独立项（quick task 候选）：Weight 比较改容差/确定性排序后比较 + 补稳定性；修复后 P4 无需扩 waiver
