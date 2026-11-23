---
phase: "55"
slug: ingest-pipeline-ui
status: draft
shadcn_initialized: false
preset: none
created: 2026-11-23
reviewed_at: pending
---

# Phase 55 — UI Design Contract

> Visual and interaction contract for Phase 55 摄取流水线前端视图 — 新增知识的过程前端可见：八步时间线（状态/耗时/失败原因悬停可见）、文档×步骤 Grid 矩阵、progress_msg 日志流展开、failed 步骤重试 + 审计提示、上传后自动浮出 job 览图。本文档由 FE 规划 agent 撰写，锚定 v1.8-ROADMAP Phase 55 SC-1–SC-5 与 D-55-01（ECL stage 哨兵 LOCKED 不改名）；格式沿用 39-UI-SPEC（项目 UI-SPEC 权威格式）。本 surface 是 39 已验证的「五段状态机 + 失败表 + retry + 2s/10s 自续轮询」模式向整个摄取链路的推广，**不引入新设计语言**。

---

## Design System

| Property | Value |
|----------|-------|
| Tool | Tailwind CSS v4 (CSS-first, `@theme` in `web/src/styles/theme.css`; no tailwind.config.js) |
| Preset | not applicable — no `components.json`; shadcn CLI preset automation NOT adopted（同 Phases 20/29/39 的 gate 结果：theme.css 的 pinned token set 就是设计系统） |
| Component library | admin console 手写惯例（PageHeader / DataTable / StatusBadge / States 三态 / Button）；vendored `web/src/components/ui/`（button/card）可用；不引入新 UI 库 |
| Icon library | lucide-react（已在用）；新 tab 图标 `FileStack`（摄取任务） |
| Font | Plus Jakarta Sans Variable（`--font-sans`）；JetBrains Mono（`--font-mono`）用于 job_id、doc_id、step 键、文件路径、耗时秒数、attempt 计数、HTTP 状态 |

**零新依赖。** 不加 npm 包、不加 shadcn CLI 组件、不加 CSS 文件、不在 theme.css 加 token。新文件只有 `web/src/components/admin/ingest/{IngestTab.tsx, IngestJobTimeline.tsx, IngestStepMatrix.tsx, ingestApi.ts, types.ts}`（镜像 `pipeline/` 文件夹模式）+ `DocUpload.tsx` 入口改造 + `AdminDashboard.tsx` sidebar 注册 + `app-store.ts` 加 `ingestFocusJobId`。

**一致性基线（39 模式推广）：** 每个新元素映射到既有 console 模式：

| 新元素 | 继承模式（事实来源） |
|--------|---------------------|
| Sidebar 注册（数据组第 3 位） | `NAV_GROUPS` 数据组条目（2610 sidebar 惯例），同 导入批次/知识库 |
| 控制条 / 提示行 | PipelineTab 控制条 idiom（`rounded-md border border-border bg-surface-2 p-3` + 12px hint） |
| 步骤状态徽章 | 共享 `StatusBadge`（status-token 着色）+ running `animate-pulse`（39 amber-pulse 先例） |
| 时间线行 | PipelineTab Failures 表行 idiom（`border-t hover:bg-surface-2`，cells `px-3 py-1.5 text-xs`） |
| 进度条 | CapacityBar 几何（`h-2 bg-surface-2 rounded` track + `bg-primary` fill + mono counter），`role="meter"` |
| Matrix 单元格 | StatusBadge tone 色板（`--color-status-*` /15 底色），复用色板不加新颜色 |
| 失败明细表 | PipelineTab 表 idiom（`overflow-hidden rounded-md border border-border`，`thead bg-surface-2`） |
| 数据获取 + token + 403 | KnowledgeBaseList / PipelineTab（`localStorage sf_admin_token` → `X-Admin-Token`；403 → 缺少或错误的 X-Admin-Token） |
| 轮询循环 | PipelineTab 专用 interval effect（2s/10s re-arm on flip；useAdminData 无 re-arm，不适用） |
| Loading / error / empty 三态 | 共享 `States.tsx`（LoadingSkeleton / ErrorState / EmptyState） |
| Toasts | 既有 SfToast viewport（AdminDashboard root 已挂载）— dispatch `sf-toast`，不挂第二个 viewport |

---

## Spacing Scale

继承 theme.css `--space-*` token。无新例外。

| Token | Value | Usage in this phase |
|-------|-------|---------------------|
| space-1 | 4px | 徽章/图标 gap（`gap-1`） |
| space-2 | 8px | 时间线行内 gap、hint margin、日志行距 |
| space-3 | 12px | 控制条 padding（p-3）、展开面板 padding（p-3）、section heading margin |
| space-4 | 16px | ViewBody padding（p-4）、section 间距（space-y-4） |
| space-12 | 48px | 空态垂直 padding（EmptyState 默认 py-14，向下兼容 py-12 惯例） |

唯一例外：matrix 单元格间 `gap-0.5`（2px）Grid 密度特例（Airflow Grid view 模式）；单元格本身 ≥12px 见方。除此之外全部为 4 的倍数。

---

## Typography

继承 `--text-*` token：4 sizes × 2 weights，无新增。

| Role | Size | Weight | Line Height |
|------|------|--------|-------------|
| Body（提示行、空态文案） | 14px (--text-base) | 400 | 1.5 |
| Label / Meta / 表格 cells / 徽章 / 按钮 / 时间线行 | 12px (--text-xs) | 400 / 600 | 1.4–1.5 |
| 日志行 | 12px (--text-xs) mono | 400 | 1.5 |
| Section heading（运行日志 / 失败明细） | 14px (--text-base) | 600 | 1.2 |
| Heading（PageHeader title） | 18px (--text-lg) | 600 | 1.2 |

徽章字面：共享 StatusBadge 的 `text-[11px] font-medium` 原样继承（console 标准）。日志流不用 10px（CapacityMonitor `text-[10px]` 是 grandfathered 孤例，不复用）。

Font families：`--font-sans` 全部散文；`--font-mono` 用于 job_id、doc_id、step 键、文件路径（file_name / artifact_ref）、耗时秒数、attempt 计数、`done/total`、日志行、HTTP 错误串。

禁止：新 markup 使用表外字号/字重（唯一豁免是继承的 `text-[11px]` 徽章字面，原样或不用）。

---

## Color

继承 theme.css `--color-*` token。浅色主题镜像自动生效——组件内不硬编码 dark hex。

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | #0c0e12 (--color-bg) | Admin canvas、ViewBody 背景 |
| Secondary (30%) | #14171e (--color-surface) | 控制条/展开面板内表面（`bg-surface-2`）、表头、行 hover |
| Accent (10%) | #6366f1 (--color-primary) | 见 reserved-for 清单 |
| Destructive | #ef4444 (--color-danger) | 行内错误行（`text-danger`）+ error 徽章色；永不做按钮填充 |

**Accent reserved for：** 活跃 sidebar 项（bg-primary-dim）、进度条 fill、新交互元素 focus ring。**不用于状态。**

**派生状态 token 映射（不新增 status 色，全部走共享 StatusBadge tone 色板）：**

| 派生态 | Tone | Treatment |
|--------|------|-----------|
| queued（等待） | gray → --color-status-unanswered | `bg-status-unanswered/15 text-status-unanswered` |
| running（进行中） | amber → --color-status-needs-info | `bg-status-needs-info/15 text-status-needs-info animate-pulse` |
| done（完成） | green → --color-status-compliant | `bg-status-compliant/15 text-status-compliant` |
| error（失败） | red → --color-status-non-compliant | `bg-status-non-compliant/15 text-status-non-compliant` |
| skipped（已跳过） | gray → --color-status-unanswered | 同 queued；matrix 格子加 `border-dashed border-border` |
| stale（待重跑） | amber，**无 pulse** | `bg-status-needs-info/15 text-status-needs-info`；pulse 只属 running |

矩阵格子背景 = 同族 tone /15 底色（queued/skipped 格子 `bg-surface-2`）。opacity-modified status classes（`/15` 族）已在库内证明（StatusBadge 本体、ProposalDrawer error box）。

---

## Layout & Structure

### 入口一：上传动作打通（DocUpload 改造）

```
DocUpload POST /api/admin/kb/:slug/upload
        │ 2xx { job_id }
        ▼
app-store.setIngestFocusJobId(job_id)  +  setAdminSubroute('ingest')
        ▼
IngestTab 挂载读 ingestFocusJobId → 选中该 job → 时间线视图 + 2s 轮询
```

- Phase 54 后 upload POST 返回 `job_id`（异步 job 模型，ING-04）。DocUpload 收到 2xx：store 写 `ingestFocusJobId`，切 subroute 到 `ingest`。
- **DocUpload 假状态机退役**：现有 `uploading → parsing → indexing` 本地中间态（`setTimeout 200ms` 假延时）属「状态与实际处理不符」（ROADMAP Industry References 的 OpenWebUI 反例）——移除 parsing/indexing 假态，只留 `正在上传…`（POST 在途）与 2xx 后立即交接。本 spec 唯一的既有行为移除，理由见 §Honesty Rules。
- done 行改为交接提示（§Copywriting）；「再传一份」重置回 idle。上传失败分支（error + 重试）不变。

### 入口二：管理面板入口（sidebar 注册）

```
┌────────────────────────────────────────────────────────────────┐
│ Sidebar（数据组）: [导入批次] [知识库] [摄取任务]                  │  ← 新增第 3 位
│           （运营/智能层/平台组不变；ECL 管线仍在智能层组）           │
├────────────────────────────────────────────────────────────────┤
│ TopBar (h-11): Admin Console · /admin · [Refresh]               │
├────────────────────────────────────────────────────────────────┤
│ ViewBody (p-4)                                                  │
│  ingest → <IngestTab />                                         │
│    ├─ 无选中 job → Jobs 表（默认视图）                            │
│    └─ 选中 job → 详情（时间线 + 矩阵 + 日志 + 失败明细）           │
└────────────────────────────────────────────────────────────────┘
```

- `AdminSubroute` union 加 `'ingest'`；`NAV_GROUPS` 数据组追加 `{ key: 'ingest', label: '摄取任务', icon: <FileStack className="h-4 w-4" />, testId: 'admin-tab-ingest' }`，置于 知识库 之后。
- URL 恒为 `/admin`（store-only subroute，同全部既有 tab）。

### IngestTab 视图结构

```
┌──────────────────────────────────────────────────────────────────┐
│ PageHeader: 摄取任务 / 上传与目录扫描的异步摄取流水线（状态、耗时、   │
│             失败原因、步骤级重试）                                  │
├──────────────────────────────────────────────────────────────────┤
│ [Jobs 视图]  DataTable: Job · 文档 · 状态 · 进度 · 创建时间 · 操作  │
├──────────────────────────────────────────────────────────────────┤
│ [Job 详情视图]（查看 或 上传交接进入）                              │
│   [← 返回任务列表]                                                │
│   控制条: job_id(mono) · job 徽章 · 进度 meter + counter            │
│   ┌ 八步时间线 IngestJobTimeline（固定 8 行，顺序 LOCKED）────────┐ │
│   │ ● 解析     [等待]      —                                     │ │
│   │ ● 分块     [进行中⟳]   —                                     │ │
│   │ ● 向量化   [完成]      3.2s                                  │ │
│   │ ● 入向量库  [完成]      1.4s                                  │ │
│   │ ● 切分合并  [待重跑]   —                                      │ │
│   │ ● 实体抽取  [失败]     —（悬停见失败原因） [重试此步骤]          │ │
│   │ ● KG 写入  [待重跑]   —                                      │ │
│   │ ● Milvus 索引 [待重跑] —                                     │ │
│   └──────────────────────────────────────────────────────────────┘ │
│   文档×步骤矩阵 IngestStepMatrix（docs.length > 1 时渲染）           │
│   运行日志（progress_msg 日志流，可展开，默认折叠）                  │
│   失败明细（DTO 携带 failures[] 时渲染；SC-7 item 态分离）           │
│   审计提示行（12px muted）                                         │
└──────────────────────────────────────────────────────────────────┘
```

---

## Component Inventory

### 1. AdminDashboard 注册（modify: `web/src/components/admin/AdminDashboard.tsx`）

- `AdminSubroute` union 加 `'ingest'`；`NAV_GROUPS` 数据组追加摄取任务条目；ViewBody 加 `{adminSubroute === 'ingest' && <IngestTab />}`。
- TopBar Refresh 已广播 `admin-refresh` 全局事件——IngestTab 轮询 hook 订阅该事件触发立即 fetch。

### 2. app-store（modify: `web/src/store/app-store.ts`）

新增 `ingestFocusJobId: string | null`（默认 null）+ setter。唯一写入方：DocUpload 交接；唯一读取+清零方：IngestTab 挂载 effect。

### 3. Types + API（new: `web/src/components/admin/ingest/types.ts`, `ingestApi.ts`）

镜像 `pipeline/types.ts` + `pipelineApi.ts` 模式（BE 原样下发、FE mirror 不改名不重嵌；字段以 Phase 54 wire 为准，本表是 FE 期望形状）：

```typescript
// types.ts
export interface IngestJob {
  job_id: string;
  /** BE job 级状态（SC-7 job/item 分离：个别文档失败不整体标败）。
   * 期望词表 running|completed|partial|failed|queued — 未知值渲染原文。 */
  status: string;
  created_at: string;
  docs: IngestDoc[];
  /** RAGFlow progress_msg 模式：追加式日志行，FE 原样渲染不解析。 */
  progress_msg: string[];
  /** SC-7 失败明细单独可查；absent → 不渲染失败明细 section。 */
  failures?: IngestStepFailure[];
}
export interface IngestDoc {
  doc_id: string;
  file_name: string;
  /** 该文档的 8 步状态（每步独立状态+完成时间戳落库，ROADMAP SC-5）。 */
  steps: IngestStep[];
}
export interface IngestStep {
  /** LOCKED 扩展哨兵，§Step Mapping。 */
  step: string;
  /** 机内状态，§Derived State Mapping；未知值渲染原文。 */
  status: string;
  attempt: number;
  started_at?: string;
  completed_at?: string;
  error?: string;
  /** 每步产物定位（staging 持久化，D-54-02）——重试从上游最近成功步骤的
   *  artifact_ref 续跑。FE 以 mono 渲染，不做任何解释。 */
  artifact_ref?: string;
  /** Dify Variable Inspect 模式：服务端摘要键值。FE 只渲染，不推导。 */
  summary?: Record<string, number | string>;
  skip_reason?: string;
}
export interface IngestStepFailure {
  doc_id: string;
  step: string;
  error: string;
}
```

> FE mirror 说明：`docs[]` + per-doc `steps[8]` 是「文档×步骤矩阵」的数据前提。Phase 54 planner 若落为「一 doc 一 job」形态，FE 以服务端实际 DTO 为准改 mirror 并同步 §Derived State Mapping——不变式只有一条：**矩阵格子只渲染服务端下发的 per-doc×per-step 状态，FE 永不聚合推导**。

`ingestApi.ts` 三个调用，全部 `X-Admin-Token`（`localStorage.getItem('sf_admin_token') ?? ''`）：
- `fetchJobs()` → `GET /api/admin/ingest/jobs`
- `fetchJob(id)` → `GET /api/admin/ingest/jobs/:id`
- `retryStep(id, step)` → `POST /api/admin/ingest/jobs/:id/steps/:step/retry`

非 2xx 映射为 typed error `{ status, message }`（PipelineApiError 同型）：403 → 缺少或错误的 X-Admin-Token；404 → 任务不存在或已被清理；其余 → server `error` 原文；网络层 → 网络错误。

### 4. IngestTab（new: `web/src/components/admin/ingest/IngestTab.tsx`）

Root: `<section data-testid="ingest-tab" className="space-y-4">`。

**Jobs 视图**（无选中 job）：PageHeader + DataTable（`data-testid="ingest-jobs-table"`），列：

| 列 | 内容 | Style |
|----|------|-------|
| Job | `job_id` | mono，truncate + title |
| 文档 | 1 篇 → 文件名（mono truncate + title）；多篇 → `{docs.length} 篇文档` | text-xs |
| 状态 | job 徽章（§Derived State Mapping 的 Job 级映射） | StatusBadge |
| 进度 | 全部文档全部步骤 done 数 / (8×docs 数)，mono `done/total`；total 0 → 「—」 | mono |
| 创建时间 | `formatDate` zh-CN（ImportBatchList 惯例） | 12px muted |
| 操作 | 查看 button | secondary |

- 空态：EmptyState（icon `FileStack`）title 「暂无摄取任务」，hint 「上传文档后自动出现在这里」。

**Job 详情视图**：`[← 返回任务列表]` ghost button（回 Jobs 视图）。

- **控制条**（PipelineTab 控制条 idiom）：`job_id`（mono truncate + title）+ job 徽章 + CapacityBar 几何进度 meter（`role="meter"` + aria-valuenow/min/max + `aria-label="摄取进度: {done} of {total}"`，counter `{done}/{total}` mono，total 0 → 「—」）。
- **IngestJobTimeline**（§Component 5）。
- **IngestStepMatrix**（§Component 6；`docs.length > 1` 才渲染）。
- **运行日志**：h2 「运行日志」 + 折叠按钮（默认折叠）。展开容器 `max-h-48 overflow-y-auto`，行 = `progress_msg[]` 原样渲染（mono 12px，`whitespace-pre-wrap`）。空数组 → 「本任务暂无过程日志」。FE 不解析日志、不按日志行判状态——状态只来自 step 状态机。
- **失败明细**（`failures[]` 存在才渲染）：表 idiom：文档 · 步骤 · 错误（`max-w-xs truncate` + title）。section 不存在时**不渲染**（不是空表——存在性即信号）。
- **审计提示行**：12px muted：「每次重试记入审计（操作者、时间、步骤前后状态）。」

### 5. IngestJobTimeline（new: `…/ingest/IngestJobTimeline.tsx`）

固定 8 行，LOCKED 顺序（Phase 54 SC-1）：`parse → chunk → embed → vector_insert → ecl_merge → ecl_extract → ecl_kg_load → ecl_milvus_load`。默认渲染 `docs[0]`（multi-doc 时首篇；完整 per-doc 状态在矩阵）。

每行（`data-testid="ingest-step-{step_key}"`）：

```
● 解析        [等待]      —
● 向量化      [完成]      3.2s
● 实体抽取    [失败]      —   [重试此步骤]
```

- 行头是 `<button aria-expanded>`：点击展开该步 Variable Inspect 面板（下方插入行，`aria-controls` 指向面板 id）。
- 徽章 = StatusBadge，label/tone 按 §Derived State Mapping；running 加 `animate-pulse`。
- 耗时：`started_at`+`completed_at` 齐备 → 差值秒数 mono（`3.2s`）；running 或缺时间戳 → 「—」（零推导：不本地秒表跳动；随轮询刷新）。
- 失败原因：行头 tooltip `title={error}`（**悬停可见**，SC-1 验收原话）+ 展开面板 `text-danger` 全文。
- **重试按钮**（`data-testid="ingest-step-retry-{step_key}"`）：secondary idiom（`rounded-md border border-border bg-surface-2 px-3 py-1.5 text-xs font-semibold hover:bg-surface-3 disabled:opacity-50`），label 重试此步骤。**仅派生态 = error 的步骤渲染**；其余步骤不渲染按钮（不渲染 disabled 摆设——不发明假能力）。
- **Variable Inspect 展开面板**（`data-testid="ingest-step-expand-{step_key}"`，控制条 box idiom）：
  - `summary` 键值对渲染（键的 zh 标签查 §Summary Key Labels；未知键渲染原文 key + value）。
  - 「产物定位」行：`artifact_ref` mono truncate + title。
  - 「尝试 {n}」行：`attempt` mono。
  - `error` → `text-danger` 全文（verbatim）。
  - `skip_reason` → 12px muted 全文。
  - ecl_* 四步附加链接行：「ECL 全局状态见 ECL 管线 tab」→ `setAdminSubroute('pipeline')`（store-only）。

**Summary Key Labels**（已知键的 zh 标签；未知键原文渲染）：

| summary key | zh 标签 | 出处 |
|-------------|--------|------|
| `parsed_chars` | 产物字数 | ROADMAP SC-1 |
| `chunk_count` | 块数 | ROADMAP SC-1 |
| `embedded_count` | 向量数 | ROADMAP SC-1 |
| `embedded_failed` | 失败块数 | ROADMAP SC-1（per-chunk 状态聚合计数） |
| `inserted_rows` | 入库行数 | ROADMAP SC-1 |
| `entity_count` | 抽取实体数 | ROADMAP SC-1 |
| `triples_written` | 三元组写入数 | Phase 38/39 KG 语义 |
| `purged_before_insert` | 插入前清理行数 | D-53-03 upsert 语义 |

### 6. IngestStepMatrix（new: `…/ingest/IngestStepMatrix.tsx`）

`docs.length > 1` 时渲染。表 idiom：header = 8 步 zh 标签（**全称**，不发明缩写；容器 `overflow-x-auto` 允许横向滚动），行 = 文档（`file_name` mono truncate + title）× 8 格。

- 格子：12px 见方 `rounded-sm`，色板按 §Color 派生态映射；running 格子 `animate-pulse`。格子**非交互**（`role="img"` + `aria-label="文档 {file_name} · {步骤label} · {派生态label}{耗时?}"`）。
- 矩阵下方 12px muted 图例行：六个样例格子 + 标签（完成 / 进行中 / 失败 / 等待 / 已跳过 / 待重跑）。
- 标题行右侧 mono 计数 `{done_docs}/{total_docs} 篇完成`（`done_docs` = 8 步全 done 的文档数）。
- **矩阵不可点**：交互面是时间线与失败明细表——不发明「点格子切文档」的假交互（§Discretion：v1 不做文档切换器）。

### 7. Polling hook（IngestTab 内专用 interval effect，PipelineTab 同型）

- PipelineTab 形状：`fetchOnce` in `useCallback` + `window.setInterval` + cadence flip re-arm（`vi.useFakeTimers` 可注入 interval/fetchImpl）。**不用 useAdminData**（无 re-arm cadence）。
- **Cadence（39 D-Prog-01 推广，LOCKED）**：选中 job 派生 running（任一步 running / 任一 error 待处理）→ **2000ms**；Jobs 列表视图 / job 终态 → **10000ms**。cadence flip 立即 re-arm。轮询错误不停循环；行内错误行仅在 fetch 失败时渲染，下次成功轮询清除。
- retry POST 不走 poller；2xx → 立即 `fetchJob` 重同步。订阅全局 `admin-refresh` 触发立即 fetch。

### 8. Loading / Error / Empty

- Loading：`LoadingSkeleton rows={3}`。
- 详情 404 → error toast 「任务不存在或已被清理」 + 回落 Jobs 列表。
- 403：行内 `text-danger` 「缺少或错误的 X-Admin-Token」，轮询不停（idle cadence 继续）。
- 网络/其他错误：行内 `text-danger` 「摄取任务加载失败（点击重试）」（摄取任务加载失败 是 re-fetch button）。
- ECL off：ecl_* 四步由 server 声明 `skipped` + `skip_reason` → 步骤行渲染 已跳过 徽章 + 展开面板显示 skip_reason 原文（摄取视图无 503 契约，FE 不发明 503 面板）。

---

## Step Mapping（LOCKED）

39-UI-SPEC §Stage Mapping 的 ECL stage 哨兵 `idle / merge / extract / kg-load / milvus-load / done` 是 **LOCKED FE contract**（D-55-01：stage 哨兵不改名，新增步骤是扩展不是替换）。本 phase 新增四个扩展哨兵，映射如下，**后四步沿用 39 的 zh 标签不改字**：

| ingest step key（Phase 54 SC-1） | 39 LOCKED 哨兵（ECL manager `stage`） | zh label | 契约归属 |
|----------------------------------|----------------------------------------|----------|----------|
| `parse` | — | 解析 | 55 新增哨兵 |
| `chunk` | — | 分块 | 55 新增哨兵 |
| `embed` | — | 向量化 | 55 新增哨兵 |
| `vector_insert` | — | 入向量库 | 55 新增哨兵 |
| `ecl_merge` | `merge` | 切分合并 | 39 LOCKED，不改名 |
| `ecl_extract` | `extract` | 实体抽取 | 39 LOCKED，不改名 |
| `ecl_kg_load` | `kg-load` | KG 写入 | 39 LOCKED，不改名 |
| `ecl_milvus_load` | `milvus-load` | Milvus 索引 | 39 LOCKED，不改名 |

> 渲染规则：ingest job 视图的数据源是 ingest job DTO；39 的 `/api/admin/pipeline/status` 端点与 ECL 管线 tab **不受影响**（ECL 管线 tab 仍是 manager 级监控面）。ECL 四段由既有 ECL manager 执行时，manager status DTO 的 `stage` 仍是 LOCKED 哨兵；ingest job DTO 的 step key 用 `ecl_*` 前缀——FE 的 `ecl_* → 39 哨兵` 映射是**纯展示映射**，ECL manager 契约零改动。未知 step key 渲染原文 key（39 §Stage Mapping 规则：unknown values render the raw string — never blank, never a crash）。

---

## Derived State Mapping（双层状态派生，Dify display_status 模式）

机内层（Phase 54 wire 的 step 状态字段）→ UI 派生层（避免用户看到内部抖动）：

| 机内（step.status 等 server 字段） | → | UI 派生态 | 徽章 label | Tone | pulse |
|-----------------------------------|---|----------|-----------|------|-------|
| `pending` | → | queued | 等待 | gray | 否 |
| `running` | → | running | 进行中 | amber + pulse | 是 |
| `running` + `attempt ≥ 2`（瞬时错误自动退避重试 ×3 中） | → | running（重试折叠进 running） | 进行中 | amber + pulse | 是（attempt 数只在展开面板 「尝试 {n}」） |
| `done` | → | done | 完成 | green | 否 |
| `failed`（瞬时错误耗尽） | → | error | 失败 | red | 否 |
| `dead`（确定性错误 dead-letter） | → | error | 失败 | red；展开面板加注 「确定性错误 — 不自动重试」 | 否 |
| `skipped` | → | skipped | 已跳过 | gray | 否 |
| `stale` | → | stale | 待重跑 | amber，无 pulse | 否 |

> FE 不发明状态：每个派生态由 server 字段唯一决定（status + attempt + dead-letter 分类）。Phase 54 planner 落定的状态枚举若与本表有出入（如 retrying/dead 是 `status` 值还是 `retryable` 标志），**以 BE wire 为准同步更新本表**——39 §Stage Mapping 同款姿态。
> 派生态词表 = {queued, running, error, done, skipped, stale}；徽章 label 恒为：等待 / 进行中 / 失败 / 完成 / 已跳过 / 待重跑。
> 抖动抑制（Dify display_status 的存在理由）：`retrying` 不独立成态——自动退避重试期间用户只看到「进行中」，attempt 数收进展开面板，避免「失败→进行中→失败」闪烁。

**Job 级徽章（jobs 表，server 声明的 job.status，FE 只做 label/tone 映射，不做聚合推导）：**

| BE job.status | label | Tone |
|---------------|-------|------|
| running | 运行中 | amber + pulse |
| completed | 已完成 | green |
| partial | 部分成功 | amber |
| failed | 失败 | red |
| queued | 等待 | gray |

---

## Interaction Behaviors

### 上传交接 lifecycle

1. DocUpload POST 2xx → store 写 `ingestFocusJobId` + `setAdminSubroute('ingest')`；DocUpload 渲染交接行（§Copywriting）。
2. IngestTab 挂载 effect 读 `ingestFocusJobId` → 立即 `fetchJob(id)` → 详情视图 + 2s cadence；读后清零 focus 字段（刷新页面回默认 Jobs 列表——无持久化 deep-link，39 「URL stays /admin」 同款行为）。
3. `fetchJob` 404 → error toast 「任务不存在或已被清理」，回落 Jobs 列表。
4. 上传中只显示 正在上传…；2xx 后**立即交接，不等待摄取完成**。DocUpload 不再阻塞等 chunks 计数——chunks 计数出现在时间线 chunk 步骤展开面板（summary.chunk_count）。

### 重试 lifecycle（per failed step）

1. error 步骤行点 「重试此步骤」 → `POST /api/admin/ingest/jobs/:id/steps/:step/retry`。**单击、无 confirm**（39 per-note retry 先例：有界单步动作；幂等语义 Phase 54 SC-2 钉死，成本告知走展开面板 hint）。
2. 2xx → success toast 「已重新入队步骤 {步骤label}」 + 立即 `fetchJob` 重同步。后续状态转移全部经轮询抵达（server truth）。
3. RAGFlow 教训（ROADMAP SC-3）：failed 步骤的 retry 先隐式 stop 上游残留再重跑——BE 行为，FE 文案如实转述：展开面板 hint 「重试从上游最近成功步骤的产物继续，下游步骤将标记待重跑」。
4. 失败 → error toast 「重试失败：{真实原因}」；行不变，按钮恢复。
5. **无「全部重试」**：唯一重试端点是 per-step；批量按钮会 404——假能力禁止（39 §Honesty Rule 5 同款）。
6. **无 cancel 控制**：cancel API 未实装，渲染不存在端点的控制 = 假能力。
7. 重试成功后下游 stale 标记经轮询抵达——FE 不本地预标 stale。
8. 失败明细表无重试入口——重试入口唯一：时间线步骤行。

### 轮询

§Component 7。轮询错误不停循环。

---

## Copywriting Contract

全部 user-facing copy zh-CN；技术标识（job_id / doc_id / step 键 / 文件路径 / 耗时 / attempt / HTTP 状态）保持英文/mono。Server `error` / `skip_reason` / 日志行 verbatim 渲染。

| Element | Copy |
|---------|------|
| Sidebar label | 摄取任务 |
| PageHeader title / description | 摄取任务 / 上传与目录扫描的异步摄取流水线（状态、耗时、失败原因、步骤级重试） |
| Upload handoff line（DocUpload done 行） | 已创建摄取任务，正在打开流水线视图… |
| Upload error reset link | 重试 |
| Jobs 表列头 | Job · 文档 · 状态 · 进度 · 创建时间 · 操作 |
| Job row CTA | 查看 |
| Jobs empty state | 暂无摄取任务 — 上传文档后自动出现在这里 |
| Job detail back link | ← 返回任务列表 |
| Progress counter | {done}/{total}（mono；total 0 → 「—」） |
| Progress meter aria-label | 摄取进度: {done} of {total} |
| Step labels | parse=解析 · chunk=分块 · embed=向量化 · vector_insert=入向量库 · ecl_merge=切分合并 · ecl_extract=实体抽取 · ecl_kg_load=KG 写入 · ecl_milvus_load=Milvus 索引 |
| Derived state labels | queued=等待 · running=进行中 · error=失败 · done=完成 · skipped=已跳过 · stale=待重跑 |
| Job badge labels | running=运行中 · completed=已完成 · partial=部分成功 · failed=失败 · queued=等待 |
| Retry button | 重试此步骤 |
| Retry aria-label | 重试步骤 {步骤label} |
| Retry success toast | 已重新入队步骤 {步骤label} |
| Retry failure toast | 重试失败：{真实原因} |
| Retry hint（error 步骤展开面板） | 重试从上游最近成功步骤的产物继续，下游步骤将标记待重跑 |
| Dead-letter 展开面板加注 | 确定性错误 — 不自动重试 |
| Audit hint line | 每次重试记入审计（操作者、时间、步骤前后状态）。审计可在治理 → 时间线查看 |
| Expand panel labels | 输入摘要 / 产物定位 / 尝试 {n} / ECL 全局状态见 ECL 管线 tab |
| Log section heading | 运行日志 |
| Log expand button（折叠/展开） | 展开 / 收起 |
| Log empty | 本任务暂无过程日志 |
| Matrix 计数 | {done_docs}/{total_docs} 篇完成 |
| Matrix 图例行 | 完成 / 进行中 / 失败 / 等待 / 已跳过 / 待重跑 |
| Jobs load error line | 摄取任务加载失败（点击重试） |
| 403 line | 缺少或错误的 X-Admin-Token |
| 404 toast | 任务不存在或已被清理 |
| 网络错误 | 网络错误 |

---

## Accessibility

- **Sidebar 项：** 既有 SidebarItem markup 不变（no ARIA tab-role retrofit — 39 同款姿态）；nav 保持 `data-testid="admin-subroute-tabs"`。
- **Progress meter：** `role="meter"` + aria-valuenow/min/max + `aria-label="摄取进度: {done} of {total}"`（CapacityBar 模式）。
- **时间线容器：** `aria-live="polite"`——步骤状态转移经轮询抵达时礼貌播报，不抢焦点。
- **时间线行头按钮：** `aria-expanded` + `aria-controls`（展开面板 id）；重试按钮 `aria-label="重试步骤 {步骤label}"`；disabled 态保留可见 label（never `visibility:hidden`）；非 error 步骤不渲染按钮。
- **Matrix：** `<table>/<thead>/<tbody>` + `scope="col"`；格子 `role="img"` + aria-label（文档 · 步骤 · 派生态 · 耗时?）；图例行 `aria-hidden` 装饰语义由相邻文字承担。
- **日志流：** `max-h-48 overflow-y-auto` 容器；行 mono 12px。
- **Keyboard：** 全部交互元素 native buttons — Tab/Enter/Space 原生行为，无自定义快捷键。

---

## Honesty & Trust-Semantics Rules（console red lines）

1. **Server truth only.** 时间线/矩阵/进度/状态/耗时全部来自轮询响应；无乐观计数、无乐观状态转移、无本地秒表。FE 唯一允许的推导是**计数**（jobs 进度 counter、矩阵 `{done_docs}/{total_docs} 篇完成`）——计数不是状态推导。
2. **No fake states.** DocUpload 假态退役：移除 parsing/indexing 本地假中间态（「状态与实际处理不符」是 ROADMAP 存档的 OpenWebUI 反例）。上传中只有 正在上传…；2xx 后立即交接。
3. **No fake capabilities.** 无「全部重试」（唯一端点 per-step）、无 cancel 控制（cancel API 未实装）、无 pipeline 编辑器（ROADMAP Out of Scope：只可视化既有固定流水线）、失败明细表无重试入口（入口唯一性）、非 error 步骤不渲染重试按钮。
4. **Errors verbatim.** server `error` / `skip_reason` / `progress_msg` 原文渲染；网络失败渲染 网络错误，从不伪造状态码。
5. **stale ≠ 废弃，dead ≠ 永久失败.** stale 说 待重跑（上游重试后需重跑）；dead-letter 展开面板说 确定性错误 — 不自动重试。禁止字符串：「已废弃」「已删除」「已丢弃」「已永久失败」。
6. **Idempotency stated, not oversold.** hint 说 重试从上游最近成功步骤的产物继续（幂等，Phase 54 SC-2 转述）——不承诺 零风险 / 即时完成。
7. **Unknown ≠ blank.** 未知 step key / status / job status 渲染原文（never blank, never crash——39 §Stage Mapping 规则推广到全部枚举字段）。
8. **部分成功显式化.** job 徽章 部分成功（SC-7）——个别文档失败不隐藏、不整体标败；失败明细 section 按存在性渲染。

---

## Forbidden List

- No new design tokens：不加颜色/字号/字重/间距/radius/shadow 到 theme.css，组件内不写 raw hex（唯一继承豁免：`text-[11px]` 徽章字面）。
- 不复用 ImportBatchList 的 raw Tailwind palette literals（`bg-green-500/20` 族，grandfathered）——徽章/格子一律 `--color-status-*` utilities。
- No new npm dependencies、no new shadcn CLI components、no third-party registries、no new CSS files。
- No SSE / WebSocket for this surface——polling only（39 D-Prog-01 同款 LOCKED）。
- No cancel、no 全部重试、no auto-run on tab mount、no pipeline 编辑器、no 矩阵格子点击交互。
- No `window.confirm`（步骤重试是单击；本 surface 无 cost-bearing 批量动作——Run ECL 的 confirm 留在 ECL 管线 tab）。
- No changes to TopBar / CapacityMonitor / 其余 12 个 sidebar 视图（唯一改动：`NAV_GROUPS` 数据组一条目 + ViewBody branch + store union widening + `ingestFocusJobId`）。
- DocUpload 改动仅限：移除假态、done 行换交接文案、2xx 后 store 交接——file input / KB dropdown / 错误分支 wire 不动。

---

## Discretion Resolutions（默认已选，planner 可在 PLAN 覆盖）

| Open point | Resolution | Rationale |
|------------|-----------|-----------|
| 上传交接形态：跳转 tab vs 浮层 | **跳转**（setAdminSubroute('ingest') + focus 字段） | SC-2 允许「跳转/浮出」二选一；跳转复用 store-only subroute 既有机制，浮层需要新 overlay 层（39 ProposalDrawer 模式）成本更高 |
| 时间线默认渲染哪篇文档 | `docs[0]`；per-doc 全量在矩阵 | 单文档 job（主路径）时间线即全部；多文档时矩阵承担密度视图 |
| 多文档文档切换器 | **v1 不做** | ROADMAP 无此需求；矩阵 + 失败明细已覆盖 per-doc 可观测性，避免 scope creep |
| 矩阵格子是否可点 | **不可点**（role="img"） | 格子点击→切文档是假交互密度（矩阵是密度视图）；交互面收敛在时间线 |
| 步骤重试是否 confirm | **单击无 confirm** | 39 per-note retry 先例（有界单步 + 幂等）；成本告知在展开面板 hint；Run 级 cost-bearing confirm 留在 ECL tab |
| 耗时渲染 | done/failed → 时间戳差值秒数；running → 「—」 | 零推导红线（不本地秒表）；ROADMAP SC-5 「时间戳差本身就是进度条数据」指服务端时间戳可算，FE 做差值算术 |
| `stale`/`dead` 是否独立派生态 | stale 独立（待重跑）；dead 折入 error + 展开面板加注 | Dify display_status 抖动抑制；dead 的「确定性错误」语义由加注承载，不加第七个徽章色 |
| job 进度 counter 的聚合 | done steps / (8×docs) 纯计数 | 计数不是状态推导（§Honesty Rule 1） |
| Phase 54 DTO 与 FE mirror 出入 | 以 BE wire 为准改 mirror，同步本 spec 两张映射表 | 39 §Stage Mapping 同款姿态 |

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official | none（无 CLI 组件新增；vendored `ui/` 不动） | not required |
| third-party | none declared | not applicable |

`components.json` absent（39 验证结论沿用，2026-11-23 复核 web/ 无新增）——gate 结论 **Tool: none (existing token system is the design system)**，与 Phases 20/29/39 一致。

---

## Visual Checklist

- [ ] Sidebar 数据组显示 摄取任务（FileStack icon）；active 态 bg-primary-dim；`data-testid="admin-tab-ingest"`
- [ ] Store union + NAV_GROUPS + ViewBody branch 接线；URL 恒 `/admin`
- [ ] Jobs 视图：6 列 DataTable；空态 EmptyState 文案 verbatim；job 徽章 5 label 映射
- [ ] 上传交接：DocUpload 2xx → 摄取任务 tab 自动打开该 job；DocUpload 假态已移除
- [ ] 时间线：固定 8 行 LOCKED 顺序；zh 标签与 §Step Mapping 一致；ecl 四步不改名
- [ ] 徽章：6 派生态 label/tone 正确；running 唯一带 pulse；stale 无 pulse
- [ ] 耗时：双时间戳齐备渲染 mono 秒数；running/缺戳 → 「—」
- [ ] 失败原因：行头 tooltip 悬停可见 + 展开面板 text-danger 全文
- [ ] 重试：仅 error 步骤渲染按钮；单击无 confirm；toast + 立即重同步；下游 stale 经轮询抵达
- [ ] Variable Inspect：summary 键值（已知键 zh 标签 / 未知键原文）+ 产物定位 + 尝试 {n} + ECL tab 链接
- [ ] 矩阵：8 列全称标签；6 派生态格子色板正确；running 格 pulse；图例 6 态；不可点
- [ ] 日志流：默认折叠；行 verbatim mono；空态文案 verbatim
- [ ] 轮询：running 2s / 其余 10s，flip re-arm；错误不停循环
- [ ] 403 / 404 / 网络错误各自 verbatim 文案
- [ ] 全部 copy 与 §Copywriting Contract verbatim 一致；无禁用字符串
- [ ] No new tokens / deps / cancel / 批量重试 / 矩阵交互

---

## Pre-Populated From

| Source | Decisions Used |
|--------|---------------|
| v1.8-ROADMAP Phase 55 | 5（SC-1 时间线+Variable Inspect+重试按钮；SC-2 上传入口打通+2s/10s 轮询复用；SC-3 ECL 四段 LOCKED 哨兵+新增四段同时间线；SC-3b 双层状态+Grid 矩阵+progress_msg 日志流；SC-4 重试审计；SC-5 verify 含 FE 测试段）+ D-55-01 |
| ROADMAP Industry References | 4（Dify display_status 双层派生 + Variable Inspect + 文档级状态悬停-失败原因-Retry；RAGFlow progress_msg 日志流 + 重试先 stop 上游；Airflow Grid 矩阵；OpenWebUI 反例→假态退役） |
| 39-UI-SPEC | 8（文档骨架与格式；Stage Mapping LOCKED 哨兵与 unknown-render-raw 规则；2s/10s re-arm 轮询模式；control strip/表格/badge/CapacityBar idiom；403 文案族；Honesty 红线家族（server truth / no fake capabilities / errors verbatim / idempotency stated not oversold）；Discretion 分辨表格式；single-vendor token 立场） |
| Implemented console code | 10（PipelineTab.tsx 轮询+错误分支+toast dispatch；StatusBadge tone 色板；States.tsx 三态；PageHeader；DataTable 列式表；ImportBatchList formatDate；DocUpload 既有 wire；AdminDashboard NAV_GROUPS/store union；SfToast；app-store 既有字段模式） |
| sketch-findings skill | locked（dark denova 主题、status 色价目家族——继承不重问） |
| User input this session | 0（全部实质问题已由 ROADMAP SC/D 条目回答或经 §Discretion 以上游为锚定给默认值） |

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: pending review
- [ ] Dimension 2 Visuals: pending review
- [ ] Dimension 3 Color: pending review
- [ ] Dimension 4 Typography: pending review
- [ ] Dimension 5 Spacing: pending review
- [ ] Dimension 6 Registry Safety: pending review

**Approval:** pending — 待 gsd-ui-checker 审查（本 spec 由 FE 规划 agent 产出，2026-11-23）。
