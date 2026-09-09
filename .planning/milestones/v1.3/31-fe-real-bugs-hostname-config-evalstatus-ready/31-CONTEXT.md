# Phase 31: FE Real Bugs — Hostname Config + evalStatus Ready - Context

**Gathered:** 2026-09-09
**Status:** Ready for planning
**Mode:** Auto-generated (audit-driven; no discuss needed for known bug fixes)

<domain>
## Phase Boundary

修复两个真实生产前端 bug：

1. **REALBUG-01** — `web/src/lib/sanitize-url.ts` 的 `ALLOWED_HOSTNAMES` 当前硬编码 `{localhost, 127.0.0.1, ::1, spike.local, skillforge.local}`，生产部署（如 `*.company.com`）的引文 URL 全部被拒。需改为运行时配置（env + admin API），保留 dev 白名单兜底。
2. **REALBUG-02** — `web/src/hooks/useEvalReports.ts:42` 的 `fetchOnce` 成功路径（line 57-59）从未调用 `setEvalStatus('ready')`，导致 `evalStatus` 永远卡在 `'loading'`，`EvalDashboard` 三态（loading/error/ready）逻辑不可达。

两个修复均来自 `.planning/audits/project-audit-2026-09-09.md` 报告 P0 必修清单。

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion

所有实现选择归 Claude 自决。决策依据：

- **REALBUG-01 配置方式**：选 `import.meta.env.VITE_ALLOWED_HOSTNAMES`（逗号分隔）+ 模块级默认值兜底；不动 admin API（admin API 改造属 v1.4 范围，避免 scope creep）。新增单元测试覆盖：默认列表 / env 覆盖 / 运行时刷新（暴露 `__setHostnameAllowlistForTesting` 仅 vitest 可见）。
- **REALBUG-02 修复位置**：line 59 后补 `setEvalStatus('ready')`；不改 status 类型 / 不重构 useEffect。
- **测试门**：`web/src/__tests__/sanitize-url.test.ts` 已有，加 1 case 覆盖 env 覆盖路径；`useEvalReports.test.ts`（不存在，新建）覆盖 status 切换路径。
- **信任语义保留**：sanitize-url 仍为 HARD GATE；evalStatus 切换不影响任何 audit / chunk 流。

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/lib/sanitize-url.ts:22-29` — `ALLOWED_PROTOCOLS` / `ALLOWED_HOSTNAMES` 模块级常量；`sanitizeSourceUrl(raw)` 返回 `{ ok, href | reason }` discriminated union。
- `web/src/__tests__/sanitize-url.test.ts` — 现有单元测试套件（含 protocol/hostname/invalid 覆盖）。
- `web/src/store/app-store.ts` — Zustand store，`setEvalStatus` action 已存在；`evalStatus` 字段已定义。
- `web/src/components/eval/EvalDashboard.tsx` — dashboard shell，依赖 `useEvalReports` 返回的 `status`。

### Established Patterns
- Zustand 切片模式（`useAppStore((s) => s.field)`）。
- env via `import.meta.env.VITE_*`（Vite 默认暴露）。
- 测试通过 `vi.spyOn` / `vi.fn()` 注入。
- `__setXxxForTesting` helper 用于 module-level mutation 测试可见。

### Integration Points
- `sanitize-url.ts` 是唯一 source-URL 消毒门（`SourceUrlPart` 组件、`MessageBubble.tsx:134` 引用 `sanitizeSourceUrl`）。
- `useEvalReports` 被 `EvalDashboard.tsx` 通过 `useEvalReports(pollIntervalMs)` 调用。
- 任何 hostname 添加需同步：env `VITE_ALLOWED_HOSTNAMES` + `.env.example` + README `T-1-08 source-URL 白名单消毒门` 段落。

</code_context>

<specifics>
## Specific Ideas

无新增 specific idea——bug 修复按 audit 报告 file:line 锚点严格执行即可。无需设计决策。

</specifics>

<deferred>
## Deferred Ideas

- admin API 运行时改 hostname 白名单 → 留 v1.4（不在本 phase scope）
- 生产 hostname 自动发现（`.company.com` 通配）→ 留 v1.4
- `EvalDashboard` 加载/错误/就绪三态 UI 美化 → 留后续（功能层先修通）

</deferred>