# Phase 19: 静态报告门户 + 调度器（P10.5）- Context

**Gathered:** 2026-08-29
**Status:** Ready for planning
**Source:** ROADMAP.md Phase 19 + REQUIREMENTS.md PORT-01..07, DATA-18, GEVL-11

<domain>
## Phase Boundary

Phase 18 产出的是静态 HTML 报告（`reports/<period>/vN.html`）和 HITL 审批发布流程。Phase 19 在此基础上增加：

1. **报告门户**（`/reports`）：集中展示所有已发布报告，支持按 Skill / 周期 / run 筛选，显示最后成功时间和失败状态徽标
2. **共享调度器**：每个 Skill 的 `report_schedule` manifest 字段驱动 cron 重渲染；两类 job——cron 重渲染 + TTL 清理
3. **重渲染纪律**：每次运行有审计行 + 全量 KPI 快照 + 版本留存永不覆盖
4. **手动重渲染 + drift 检测**：用户可手动触发，带漂移徽标
5. **并发守卫**：同 (skill, period) 重叠运行跳过并记审计
6. **eval 门阻塞调度**：Skill 指标回退时阻止 cron 重渲染
7. **TTL 清理**：只作用于脱敏数据集，DETACH → 删除（Windows 文件锁），审计/快照永不删

**Depends on:** Phase 18（重渲染的是 Phase 18 产出的报告形态）

</domain>

<decisions>
## Implementation Decisions

### Report Portal (PORT-01)
- `GET /reports` 列表页：按 skill_id / period 筛选，返回 [{period, skill_id, version, title, file_path, ts, status}]，无认证（复用 /eval 模式）
- `GET /reports/<period>/v<N>.html` 已有（Phase 18）

### report_schedule Manifest Field (PORT-02)
- Skill frontmatter 新增 `report_schedule:` 字段，类型为字符串（cron 表达式）
- loader.go 加载期校验 cron 语法（使用 robfig/cron/v3 标准 Cron 格式）
- 调度器热重载：Skill 文件变更 → 重新解析 schedule → 更新 cron job

### Shared Scheduler (PORT-02)
- 进程内单一 cron 实例（`robfig/cron/v3`），`WithChain(Recover, SkipIfStillRunning)` + `Stop()` 优雅关停
- 两类 job：cron 重渲染 job + TTL 清理 job
- 调度器在 server 启动时初始化，所有 Skill 的 schedule job 注册

### Rerun Audit + KPI Snapshot (PORT-03)
- 每次 cron 触发产生审计行 `{run_id, skill_id, cron_expr, fired_at, kpi_snapshots[], output_url, status}`
- kpi_snapshots：重渲染前的全量 KPI 值快照（调用同一 kpi_call 逻辑）
- 快照绑定 + 输出原子改名：写临时名 → 快照行落库 → rename（防 TOCTOU）

### Version Persistence (PORT-04)
- 同 Phase 18，每次发布新增 `v<N>.html` 不覆盖
- 重渲染不删除旧版本

### Manual Rerender (PORT-05)
- `POST /api/reports/rerender` 触发立即重渲染，复用 DeepAgent 运行路径
- 参数：skill_id, period（与 cron 触发共享同一路径）

### Drift Detection (PORT-06)
- 本次 KPI 值与上次重渲染快照对比，默认 ±5% 阈值（per-KPI 可配）
- 超阈值报告带 drift 徽标（amber）
- drift 阈值可配置在 Skill manifest 或全局配置

### Concurrent Run Guard (PORT-07)
- 同 (skill_id, period) 的重渲染重叠时，第二次跳过并记审计行 `status: skipped_duplicate`
- 使用内存锁或 DB 记录实现

### Eval Gate Blocking Scheduler (GEVL-11)
- Skill eval 指标回退时该 Skill 的 cron 重渲染被阻止
- 机制：scheduler 触发前查询 eval store，技能评分低于阈值 → 跳过本次触发 + 记审计 `status: eval_gate_blocked`
- as_of 陈旧窗：数据超 X 天未更新标黄（与 Phase 17 仪表盘共用 as_of 逻辑）

### Dataset TTL Cleanup (DATA-18)
- TTL 清理 job 只作用于脱敏数据集文件
- DETACH → 删除（Windows 文件锁处理）
- 审计行与 KPI 快照永不删除

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `internal/server/server.go` — existing `/reports/*` route and `reportPathAllowlist` pattern
- `internal/report/publish.go` — existing `Publish()` with version increment logic
- `internal/store/sqlite/reports.go` — existing `PutReport`, `ListReports` store methods
- `internal/agent/hitl/hitl_report_publish.go` — existing HITL two-pass contract for report publishing
- `examples/skills/monthly-report/SKILL.md` — existing monthly-report Skill with template field
- `internal/eval/store.go` — eval store for GEVL-11 gate check
- `internal/agent/kpi_admin.go` — existing kpi_call tool for KPI snapshot capture
- `.planning/REQUIREMENTS.md` lines 165, 190-216 — PORT-*, DATA-18, GEVL-11 requirements

</canonical_refs>

<specifics>
## Specific Ideas

- `robfig/cron/v3` 标准 cron 格式：`* * * * *`（分 时 日 月 周）
- cron TOCTOU 防法：快照绑定 + 原子改名（写临时 → 快照落库 → rename）
- SkipIfStillRunning：防止上一次重渲染未完成时再次触发
- drift 徽标：amber 边框 + "drift" 标签
- eval gate 查询：scheduler 触发前调用 eval store 的 skill_score 方法
- as_of 标黄窗：可配置（如 7 天），Phase 17 仪表盘已有 as_of 逻辑可复用
- TTL 清理只对脱敏数据集：dataset.type == "anonymized" 才清理，raw 不动

</specifics>

<deferred>
## Deferred Ideas

- 独立 worker 部署形态（进程内 vs 独立 worker — v1 倾向进程内，Q4 已在 ARCHITECTURE.md 验证）
- 报告下载为 PDF（只做 HTML 打印样式，不做服务端 PDF 生成）
- 多语言报告模板
- 报告分享链接（短期 token）

*None — PORT-01..07, DATA-18, GEVL-11 fully cover phase scope*

</deferred>
