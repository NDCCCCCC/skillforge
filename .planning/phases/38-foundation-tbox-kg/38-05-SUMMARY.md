---
plan: 38-05
title: web AdminDashboard 第 8 个 tab — TBox browser + KG query + Schema Proposals Drawer
status: complete
requirements: [ONT-04, ONT-06]
---

# Plan 38-05 Summary — 本体 Admin Tab

## What Was Built

- `web/src/components/admin/ontology/` 7 个新文件:
  - `types.ts` — Go envelope 一一镜像
  - `ontologyApi.ts` — 8 路由 fetch 包装 (localStorage `sf_admin_token`)
  - `TBoxBrowser.tsx` — 6 Class 卡片 + live namespace 发现 (@prefix 解析, env-override 安全) + 实例 chips (CJK local tail 可读) (SC-4)
  - `KGQueryPanel.tsx` — s/p/o 通配查询 + monospace 表格 + Export JSON (SC-5)
  - `ProposalDrawer.tsx` — 右侧 fixed overlay (SessionDrawer 模式; 项目无 shadcn Sheet): 三段式 diff preview / reasoning / Approve+Reject; inline rename (IRI 冲突 409 渲染) (D-UI-01..04)
  - `SchemaProposalsTab.tsx` — generate textarea + 状态徽章行
  - `OntologyTab.tsx` — 三视图切换
- `AdminDashboard.tsx` — 第 8 个 tab 本体 (Network icon)
- `app-store.ts` — adminSubroute union 加 'ontology'

## Tests

```
7/7 新测试 PASS + admin 目录全量 37/37 PASS + tsc --noEmit 退出码 0
```

## Deviations from Plan

- **Drawer 用 SessionDrawer 的 fixed-overlay 自绘模式** (项目 ui/ 只有 badge/button/card/tabs, 无 Sheet) — CONTEXT D-UI-03 说 "复用 shadcn Sheet", 实际项目没有该组件; 沿用既有 overlay 模式保持一致性
- STATUS_BADGE 用现有 token (primary-dim/danger), @theme 无 warn/ok
- `?tab=ontology` query param 路由改为 store subroute (与现有 7 个 tab 同机制, App.tsx 无需改)

## Self-Check: PASSED

- [x] 11 个文件 (7 新 + 2 改 + store + test) 落盘
- [x] vitest 37/37 + tsc 干净
- [x] autonomous: false 的 UI 细节偏离均在 SUMMARY 记录
