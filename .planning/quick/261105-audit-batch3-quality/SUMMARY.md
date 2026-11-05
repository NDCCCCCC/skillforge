---
type: quick-task
status: complete
date: 2026-11-05
plan: .planning/quick/261105-audit-batch3-quality/PLAN.md
audit_source: .planning/reports/CODE-AUDIT-2026-11-05.md
fixes: [F-05, A-12, A-14, A-15, C-07, C-08, C-09, C-11, E-04, E-05, E-10, E-11, E-12, E-13]
---

# SUMMARY — 批次 3（第一波）：工程质量

## 仓库卫生

### F-05 — spike 二进制出库
- `git rm --cached` 移除 4 个 ~22MB 的 `.planning/spikes/**/spike-*` 编译产物；
  `.gitignore` 增加 `.planning/spikes/**/spike-*` 规则（注释说明历史体积
  需 filter-repo 另行决策）。pack 41.6MiB 的增量源头关闭。

## 后端

| # | 修复 |
|---|---|
| A-12 | classifier prompt 规则编号补齐 1..5（缺 3）；`empty` 从 reason 枚举移除（与 ValidReason 口径一致），allow 时 reason 置空写入规则 3 |
| A-14 | rewrite.Rewrite 加 `if timeout <= 0 { timeout = DefaultRewriteTimeout }`——Timeout 置 0 不再"立即过期、每次静默 fallback" |
| A-15 | history 截断改按 rune（`[]rune(...)[:4000]`），中文历史不再劈裂 UTF-8 出 U+FFFD；字节超限但 rune 未超限的边界显式跳过 |
| C-07 | explainSQL 概述统计在**截断前**计算（total + 全量 kind 分布），截断时措辞改「与前 N 条分析」——确定性卖点不再撒谎 |
| C-08 | 每 token INFO 日志降级为 `SKILLFORGE_DEBUG_STREAM_TOKENS=1` 门控 |
| C-09 | 流式中途失败在传播 error 前补发 `text-delta-draft-end`——前端 DraftStreamChip 不再永久挂在 🟡 待校验 |
| C-11 | X-Session-Id 校验 `^[A-Za-z0-9_-]{1,64}$`，非法值替换为服务端新生成 ID（不 400——头是优化不是契约），审计主键不再收任意字节 |

## 前端

| # | 修复 |
|---|---|
| E-04 | `useAdminData.error` boolean → **string**（`HTTP <status>: <后端文案>`，经 `adminFetchWithError`）；DataTable error prop 同步 string 并渲染原文——403（token 错）不再与 500 不可区分；全部消费方 truthiness 检查向后兼容 |
| E-13 | `url === null` 显式复位 loading（依赖式取数不再永久骨架屏）；AbortController + 15s 超时（cleanup abort + clearTimeout）；adminFetchResponse 的 localStorage 读取 try/catch |
| E-05 | Refresh 按钮接线：`dispatchAdminRefresh()` 全局事件，所有挂载的 useAdminData 视图统一 refetch；按钮加 spinner + 「刷新中…」反馈（data-refreshing / aria-busy） |
| E-10 | StatusBadge `drifted` 标签 → 「已漂移」（全中文标签体系） |
| E-11 | STAT_TONE 改 `as const` + keyof 联合（导出 StatTone 类型）——tone 拼写错误恢复编译期拦截 |
| E-12 | Toolbar：仅 `search` 与 `onSearchChange` 同时在场才受控渲染；只有回调时降级非受控——搜索框不再"打不进字" |

## 验证
- `go build ./...` ✅；rewrite / classifier / stubchat / retriever ✅；
  server 包（排除存量 duckdb flake 族）✅ exit=0
- `cd web && npx tsc --noEmit` ✅；admin 组件 **74/74** ✅

## 遗留（批次 3 第二波候选）
E-06/E-07/E-08/E-14（llmApi 类型/onSave 覆盖/轮询）、D-02（admin token key
统一）、G-02..G-05/G-07（面板竞态）、G-06/G-09/G-11（死代码与假数据）、
A-16/D-12（补测试）、B-04..B-09 其余、C-03/C-05、A-13/A-17/A-18、D-06 等。
