---
phase: 29-feedback-loop-ticket-interface
plan: 08
subsystem: admin-fe
tags: [react, admin-console, feedback-loop, review-queue, metrics, sparkline, fbk-02, fbk-03, tdd]

# Dependency graph
requires:
  - phase: 29-feedback-loop-ticket-interface (29-05)
    provides: the three frozen admin wire shapes (review pending/dispositions halves, disposition POST {closed_count}, metrics totals/by_key/trend with number|null rates) + pending_count for the cross-link
  - phase: 29-feedback-loop-ticket-interface (29-07)
    provides: SfToast (listener + viewport) for the /admin mount, and the 'sf-toast' dispatch contract the two panels fire
  - phase: 26 (KBM)
    provides: the admin shell pattern family — KnowledgeBaseList token'd fetch / error-verbatim / empty-state trio + SubrouteTab + LoadingSkeleton
provides:
  - AdminDashboard 7-tab shell (反馈 6th per D-29-12, 复核 7th) with both ViewBody branches and its own /admin SfToast viewport
  - ReviewQueuePanel (FBK-02 render half): chunk-aggregated pending queue, three single-click terminal dispositions with closed-count toast, disposition history + filter, verbatim FBK-05 trust banner
  - FeedbackMetricsPanel (FBK-03 render half): verbatim full-vote caliber note, 5 summary cards with null→「—」, client-side 日/周 re-bucket, per-KB double-count footnote, 待复核 cross-link
  - adminSubroute union += 'feedback' | 'review' (the ONLY store change — zero new zustand fields)
affects: [29-09 verify gate (greps the verbatim copies + zero-dangerouslySetInnerHTML + forbidden-trust-string locks this plan ships), operator UAT of the feedback loop]

# Tech tracking
tech-stack:
  added: [] # zero new dependencies — Sparkline / lucide icons / Button / cn all pre-existing
  patterns:
    - "KnowledgeBaseList mirror: token'd fetch (localStorage sf_admin_token + X-Admin-Token) + loading/error/data trio + body.error rendered verbatim"
    - "Server-truth refetch after a 201 (never local list splicing); a refused mutation keeps the row and reports what the gate said"
    - "CSS-var injection for series color: --color-sparkline set inline on the row wrapper so the Phase-17 Sparkline is tinted without a prop change"
    - "component-local useState for panel-local knobs (dim/days/granularity/status tabs) — zustand stays a routing store"
key-files:
  created:
    - web/src/components/admin/ReviewQueuePanel.tsx
    - web/src/components/admin/FeedbackMetricsPanel.tsx
    - web/src/__tests__/AdminFeedbackTabs.test.tsx
    - web/src/__tests__/ReviewQueuePanel.test.tsx
    - web/src/__tests__/FeedbackMetricsPanel.test.tsx
  modified:
    - web/src/store/app-store.ts
    - web/src/components/admin/AdminDashboard.tsx

key-decisions:
  - "Task 1 committed FIRST with minimal placeholder panel exports (explicitly sanctioned by the plan) so the shell commit compiles green and the tab-order test is meaningful; Tasks 2/3 GREEN replace the placeholders file-for-file"
  - "the rate cards consume server number|null ONLY — the FE never derives a rate, so a 0-vote denominator renders 「—」 and 0%/NaN are structurally unreachable (tested: document.body contains neither)"
  - "bucketWeekly anchors 7-day buckets at the NEWEST day of the series (label = bucket's earliest day), so the weekly view is a pure client-side re-bucket of the same daily cells — granularity is deliberately not a fetch dependency"
  - "sparkline series are 0-filled across the UNION day axis and tinted through an inline --color-sparkline var on the row wrapper — zero changes to the shared Phase-17 Sparkline component"
  - "the expanded sub-row lists each report's note (empty → 「（未填写说明）」) with ONE row-level 最近报告 marker: the frozen wire shape carries no per-comment timestamps, so a per-line relative time would fabricate data (29-05 frozen contract wins over plan shorthand)"
  - "disposition success toasts the server closed_count verbatim (D-29-08/09 batch effect) and refetches pending from server truth; failure keeps the row + renders body.error + error toast — the row is never optimistically removed"
  - "REQUIREMENTS rows stay Pending per the phase-close convention (29-02/29-04/29-05/29-07 precedent): FBK-02/FBK-03 render halves and TKT-03's admin half are substantively delivered here, rows flip at 29-09 closeout"
  - "no per-row disposition note input: UI-SPEC defines no such field for v1 and the POST contract makes note optional — adding a textarea would be unrequested surface"

patterns-established:
  - "admin panel segmented controls: SubrouteTab styling copied inline as a local SegmentedButton (bg-primary-dim active) — same family, no shared abstraction extraction"
  - "clock-relative test fixtures for relative-time assertions (29-05 lesson carried to the FE): isoAgo(ms) keeps 小时前/分钟前 stable across wall-clock drift"

requirements-completed: [] # phase-close convention — FBK-02/FBK-03/TKT-03 substance delivered, REQUIREMENTS.md rows flip at 29-09

# Metrics
duration: 15min
completed: 2026-09-08
---

# Phase 29 Plan 08: admin FE —「反馈」第 6 tab +「复核」第 7 tab Summary

**FBK-02/FBK-03 的渲染半面一次成型：复核队列按 chunk 聚合 + 三处置终态 + 逐字信任横幅，反馈仪表盘全票分母口径 + null 非 0% + 日/周趋势 + per-KB 双计脚注 + 待复核 cross-link——store 唯一变更是 adminSubroute union 扩展。**

## Performance

- **Duration:** 15 min
- **Started:** 2026-09-07T21:43:13Z
- **Completed:** 2026-09-07T21:57:33Z
- **Tasks:** 3
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments
- `app-store.ts`: adminSubroute union + setter 签名各追加 `'feedback' | 'review'`（两处），default 仍 `'batches'`，pass-through 实现零改动 —— store 层唯一变更，零新 zustand 字段
- `AdminDashboard.tsx`: 7 tab（反馈第 6 `admin-tab-feedback`/MessageSquare、复核第 7 `admin-tab-review`/Inbox，顺序锁定于 D-29-12）+ 两个 ViewBody 分支 + 根 div 挂载 `<SfToast />`（App.tsx 对 /admin early-return，admin 分支自带 toast 通道）；URL sync useEffect 零改动
- `ReviewQueuePanel.tsx`: pending 按 chunk 聚合（报告次数 ≥2 → `text-status-needs-info font-semibold` 优先级信号，D-29-08）、三处置单点击终态（busy 按行禁用 → 201 `{closed_count}` → 服务端真相 refetch + success toast 报关闭条数）、失败行保留 + 服务端 error 原文行内 + error toast、说明（n）aria-expanded 展开每条报告 + 既有 `fetchChunkContent` 预览（加载态 + 「无法加载 chunk 内容」回退，零新读面）、处置历史 + disposition 过滤 + badge 三色映射、逐字 FBK-05 信任横幅常驻
- `FeedbackMetricsPanel.tsx`: 逐字口径注常驻（全票分母 D-29-10）、5 张汇总卡（null → 「—」，DOM 无 0%/NaN——测试直接扫 document.body）、维度/时间范围切换重取 + 趋势粒度纯客户端重分桶（`bucketWeekly` 纯函数，零第二请求）、三条 Sparkline 经 `--color-sparkline` 注入 compliant/non-compliant/needs-info 三色、per-KB 双计脚注逐字、`待复核 {pending_count} 条` cross-link → `setAdminSubroute('review')`
- 测试 3 文件 32 用例：tab 顺序/默认批次/两 tab 挂载/`sf-toast` 在 admin 分支可见；复核全矩阵（横幅、amber 加粗、aria-label 契约、closed_count toast、busy、历史 badge、逐字空态/错误态）；指标全矩阵（口径注、卡片、0 票「—」、bucketWeekly 双单测、切粒度零请求、series 色值、cross-link 集成、重试恢复）

## Task Commits

Each task was committed atomically (TDD RED → GREEN where tdd="true"):

1. **Task 1: store union + 7 tab 接线 + SfToast 挂载** - `ef2343c` (feat, 含占位面板 + AdminFeedbackTabs 6 用例)
2. **Task 2 RED: ReviewQueuePanel 全矩阵测试** - `09ced1d` (test, 14 用例对占位全失败)
3. **Task 2 GREEN: ReviewQueuePanel 复核视图落地** - `99456a8` (feat)
4. **Task 3 RED: FeedbackMetricsPanel 全矩阵测试** - `caf9981` (test, 12 用例对占位全失败)
5. **Task 3 GREEN: FeedbackMetricsPanel 反馈仪表盘落地** - `13d1acd` (feat)

## Files Created/Modified
- `web/src/store/app-store.ts` — adminSubroute union + setter 签名（'feedback' | 'review'），default 'batches'
- `web/src/components/admin/AdminDashboard.tsx` — 7 tab + 2 ViewBody 分支 + SfToast 挂载 + handleSubroute 类型放宽 + lucide MessageSquare/Inbox import
- `web/src/components/admin/ReviewQueuePanel.tsx` — 复核视图（含 `formatRelativeTime` 组件内纯函数：分钟/小时/天三档）
- `web/src/components/admin/FeedbackMetricsPanel.tsx` — 反馈仪表盘（含导出 `bucketWeekly` 纯函数 + `buildSeries` 0 填充）
- `web/src/__tests__/AdminFeedbackTabs.test.tsx` — 6 用例
- `web/src/__tests__/ReviewQueuePanel.test.tsx` — 14 用例
- `web/src/__tests__/FeedbackMetricsPanel.test.tsx` — 12 用例

## Verification
- `cd web && npx vitest run AdminFeedbackTabs ReviewQueuePanel FeedbackMetricsPanel` → 32/32 绿
- `cd web && npx tsc --noEmit` → 干净（exit 0）
- `cd web && npm run test` → 全量回归 **74 文件 / 595 用例全绿**（29-07 基线 71/563 → +3 文件 +32 用例，零回归）
- grep 证据：7 个 plan 文件 `dangerouslySetInnerHTML` == 0；4 个禁用信任字符串（已降权/已下线/已从知识库移除/已自动更新知识库）== 0；新面板 `text-[10px]/text-[11px]/font-medium/window.confirm/showModal` == 0；横幅「不会自动修改知识库内容」、口径注「分母不含排除项」、per-KB 脚注「同一票在每个被引知识库各计一次」各恰 1 处逐字落地
- 零新依赖：package.json / lockfile 零 diff；零新 design token（theme.css 零 diff）

## Decisions Made
- **占位面板随 Task 1 提交**：plan 明示「先建占位最小导出」候选形态，被采纳——shell commit 自身编译绿、tab 顺序断言即时有意义，Task 2/3 GREEN 原地整文件替换（两占位文件各存活恰好两个 commit）。
- **null 语义只在服务端判定**：帮助率/outdated 率的比率计算完全不进 FE；`formatRate(null) → '—'` 是唯一分支。0 票测试用 `document.body.textContent` 全文断言无 `0%`/`NaN`，比逐卡断言更强。
- **周桶锚定最新日**：`bucketWeekly` 以 series 内最新日为 7 天桶右端（桶名取桶内最早日），最新桶允许不满 7 天。选择依据：与服务器日桶无关的确定性分桶，8 天跨度 → 2 桶的边界行为有单测锁定。
- **趋势 series 0 填充并集日轴**：某 verdict 当天无票以 0 计入（而非缩短折线），保证三条 series 共享同一 x 轴可比。
- **展开子行的相对时间是行级不是条级**：29-05 冻结线形 `{chunk_id, kb_slug, report_count, last_reported_at, comments[]}` 不含逐条报告时间戳，逐条渲染相对时间即编造数据——改为每条说明 + 行级「最近报告：{relative}」标记（plan 速记与冻结契约冲突时契约优先，同 29-04 D-29-04-A 裁决姿态）。
- **`--color-sparkline` 注入而非改 Sparkline**：Phase 17 组件读 `var(--color-sparkline, var(--color-primary))`，包装行内联注入 CSS 变量即得三色 series，共享组件零改动（零回归面）。
- **REQUIREMENTS 行保持 Pending**：phase-close 约定（29-02/29-04/29-05/29-07 先例）——本 plan 交付 FBK-02/FBK-03 的 admin 渲染半面与 TKT-03 的 admin 半面，行翻转归 29-09 收尾。

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Task 2 测试断言了一个不存在的独立 retry testid**
- **Found during:** Task 2 (GREEN run)
- **Issue:** 测试点击 `review-retry`，但实现按 UI-SPEC 文案「复核队列加载失败（点击重试）」把整行错误文案本身做成重试链（更贴合「+ 重试链」原文且少一个元素）
- **Fix:** 测试改为点击 `review-error`（即重试链本身）
- **Files modified:** web/src/__tests__/ReviewQueuePanel.test.tsx
- **Committed in:** 99456a8

**2. [Rule 1 - Bug] Task 2 busy 用例的 pending stub 未按二次调用翻转**
- **Found during:** Task 2 (GREEN run)
- **Issue:** 处置成功后断言空态，但 stub 恒返同一行列表——refetch 发生却永远非空
- **Fix:** stub 改为二次调用返回 `{pending: [], total: 0}`（与成功用例同形态）
- **Files modified:** web/src/__tests__/ReviewQueuePanel.test.tsx
- **Committed in:** 99456a8

**3. [Rule 1 - Bug] Task 3 测试的日 series 点数算术错误 + Sparkline label 复数形态**
- **Found during:** Task 3 (GREEN run)
- **Issue:** (a) 期望 helpful series 2 点，实际 buildSeries 按 3 天并集日轴 0 填充为 3 点；(b) 期望 "sparkline with 1 point"，Phase 17 Sparkline 的 aria-label 恒为复数 `${n} points`
- **Fix:** 测试期望改为 3 点（并在注释中记录 0 填充语义）与 "1 points"（不为此改共享组件）
- **Files modified:** web/src/__tests__/FeedbackMetricsPanel.test.tsx
- **Committed in:** 13d1acd

**4. [Rule 3 - Blocking] 测试文件 `respond` 需支持 deferred Promise（TS 编译门）**
- **Found during:** Task 2 (GREEN run, tsc)
- **Issue:** busy-in-flight 用例需 POST 挂起，`stubFetch` 的 `respond: () => Response` 签名拒收 Promise
- **Fix:** 签名放宽为 `() => Response | Promise<Response>`（fetch stub 本就 `Promise.resolve` 包装，行为不变）
- **Files modified:** web/src/__tests__/ReviewQueuePanel.test.tsx
- **Committed in:** 99456a8

---

**Total deviations:** 4 auto-fixed，全部测试侧。**Impact on plan:** 零范围变更；全部 acceptance criteria 成立。

## Issues Encountered
- Task 1 的两个占位面板在 vitest 输出中留下 ImportBatchList fetch 的 act() 警告——与既有 `AdminDashboard.test.tsx` 同源（shell 测试渲染会 fetch 的兄弟视图），非本 plan 引入的失败，未处理。
- 既有 `web/src/components/admin/AdminDashboard.test.tsx`（26-08）在 7 tab 下零改动通过——其 3 用例只锁 bases tab 的激活/挂载/卸载，与新增 tab 无断言冲突。

## User Setup Required

None — 无新 env、无新端点、无新依赖。admin 三端点复用既有 `SKILLFORGE_ADMIN_TOKEN` + `X-Admin-Token` 门。

## Known Stubs

None — Task 1 的两个占位导出已被 Task 2/3 GREEN 整文件替换（`git log --follow` 可见占位存活期 = ef2343c..99456a8 / ef2343c..13d1acd）；HEAD 上两面板均为全量实现，数据源接真实 29-05 端点。

## Next Phase Readiness
- 29-09（verify gate 收尾）可直接 grep 本 plan 落地的锁：7 文件零 `dangerouslySetInnerHTML`、4 禁用信任字符串 == 0、三段逐字文案各恰 1 处、`requirements.mark-complete` 时 FBK-02/FBK-03/TKT-03 行翻转条件已全部满足（HTTP 面 29-05/29-06 + chat 面 29-07 + admin 面 29-08）
- operator UAT 可走全闭环：聊天 outdated 投票（29-07）→ /admin「反馈」看口径与趋势（本 plan）→ cross-link「待复核 n 条」→「复核」tab 三处置 → toast 报关闭条数 → 处置历史留痕
- 重开语义（处置后再报重新入队）由 29-05 服务端 latest-disposition-wins 承担，FE 无特判——复核 tab 无需再改

## Self-Check: PASSED

- Files verified on disk: ReviewQueuePanel.tsx / FeedbackMetricsPanel.tsx / AdminDashboard.tsx / app-store.ts / 3 test files / 29-08-SUMMARY.md — all FOUND
- Commits verified in git log: ef2343c (Task 1), 09ced1d (Task 2 RED), 99456a8 (Task 2 GREEN), caf9981 (Task 3 RED), 13d1acd (Task 3 GREEN) — all FOUND
- Plan-level verification re-run at HEAD: 3 plan test files 32/32 + `tsc --noEmit` clean + full FE regression 74 files / 595 tests green + acceptance grep sweep clean

---
*Phase: 29-feedback-loop-ticket-interface*
*Completed: 2026-09-08*
