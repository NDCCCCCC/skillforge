# Phase 6: Eval Dashboard & Production Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 6-Eval-Dashboard-Production-Hardening
**Areas discussed:** Dashboard UI, Audit + Redaction, CI gate, Reproducibility + Cross-eval

---

## Area A — Dashboard UI surface (SC-1, GEVL-06)

### A1 — `/eval` route layout

| Option | Description | Selected |
|--------|-------------|----------|
| React 应用内 /eval 路由 (推荐) | 在已有 web/ Vite + React 19 应用内加 /eval 路由,复用三栏布局 | ✓ (top-level) |
| 独立全屏路由 (推荐) | /eval 独立全屏,隐藏三栏,进入 admin mode;Sidebar 加 "Eval Dashboard" 项 | ✓ (drill) |
| 复用 RightChat 作为 dashboard | /eval 把 RightChat 替换为 dashboard 面板 | |
| Modal/overlay 覆盖 | modal 覆盖不独立路由 | |

**User's choice:** React 应用内 /eval 路由 → 独立全屏路由
**Notes:** 全屏 admin mode — trend 图 + audit 表需要横向空间。三栏布局 D-04/D12 transport-swap 不变量通过隔离保留(/eval 用独立非-SSE transport)。

### A2 — Trend line data source

| Option | Description | Selected |
|--------|-------------|----------|
| eval/reports/*.json + INDEX.json manifest (推荐) | cmd/eval --archive append + INDEX.json;dashboard 读 INDEX 拉趋势 | ✓ |
| 新增 sqlite 表 EvalRun | SQL query 灵活但 eval metadata ≠ audit,语义拥挤 | |
| GitHub API 拉 Actions 历史 | 免本地存储但需网络 + 限流 | |

**User's choice:** eval/reports/*.json + INDEX.json manifest
**Notes:** 文件型,git 友好,零额外依赖。同时这批 reports 也是 Phase 7 DIST-09 README published eval results 的数据源 — Phase 6 为 Phase 7 铺数据。

### A3 — Refresh model

| Option | Description | Selected |
|--------|-------------|----------|
| 手动刷新按钮 + 轮询 (推荐) | Refresh 按钮 + 30s 轮询 INDEX.json + audit rows | ✓ |
| SSE 主动推送 eval 完成事件 | 及时但需第二 SSE channel,客户端 transport 复架 | |
| 仅手动刷新按钮 | 最轻但多窗口 stale | |

**User's choice:** 手动刷新按钮 + 30s 轮询
**Notes:** Eval run 本身分钟级(5-10min),30s 轮询足够;不引入第二 SSE channel。

### A4 — Dashboard panels

| Option | Description | Selected |
|--------|-------------|----------|
| Skills 表 + Trend 面板 + Audit 入口 (推荐) | 主页 Skills 表 + Trend;audit 走 /eval/audit 子路由 | ✓ |
| Dashboard 嵌入 audit 表 | 同页信息密集 scroll 过多 | |
| 只有 metric + trend,不做 audit 表 | 跳过违反 SC-3 | |

**User's choice:** Skills 表 + Trend 面板 + Audit 入口(audit 走独立子路由)
**Notes:** 主页聚焦 metric+trend,audit 表走 /eval/audit 子路由,仍属同一 admin mode。

---

## Area B — Audit query + redaction (SC-3 + SC-5)

### B1 — `/api/audit` selection API

| Option | Description | Selected |
|--------|-------------|----------|
| 参数化 query store.QueryAudit (推荐) | query params → AuditFilter,LIMIT/OFFSET 分页,复用 03-02 SQLi 防护 | ✓ |
| RESTful 游标 + 服务端倒排 | 需 cursor + sort,50 rows 不值得 | |
| GraphQL endpoint | 加 GraphQL 依赖不划算 | |

**User's choice:** 参数化 store.QueryAudit
**Notes:** 1:1 HTTP 映射到已测试的 AuditFilter。

### B2 — Retention + redaction policy

| Option | Description | Selected |
|--------|-------------|----------|
| QueryAudit +retention_days 参数 (推荐) | RetentionDays 默认 90 (env 可改);过期 row query/answer 返回 [REDACTED];X-Admin-Token 区分 raw/redacted | ✓ |
| 定时清理 task + 不可逆删除 | 不可逆丢 meta 信息 | |
| 无 retention,只字段级开关脱敏 | 失去审计 "过去发生什么" 能力 | |

**User's choice:** QueryAudit RetentionDays + X-Admin-Token 访问二分
**Notes:** redaction 是 READ-time projection,不是 write — audit row 保持不可变(T-3-AUDIT)。dual property:能证明发生过 + 过期不留 raw PII。

### B3 — Redaction frontend UX

| Option | Description | Selected |
|--------|-------------|----------|
| 可视化 badge [REDACTED] + hover 详情 (推荐) | 灰 badge "REDACTED (90d+)" + hover 提示 row 时间/retention 边界;raw 不可从 UI 展开 | ✓ |
| 纯文本 [REDACTED] | 不告知为什么 | |
| 完全隐藏过期 row | 丢 meta 可见性 | |

**User's choice:** 可视化 badge + hover 详情

### B4 — Access policy granularity

| Option | Description | Selected |
|--------|-------------|----------|
| 只看 admin vs non-admin 二分 (推荐) | X-Admin-Token 二分;v1 dev 足够,真 RBAC 留 v2 | ✓ |
| Per-row access_policy 列 | schema 变动违反 T-3-AUDIT INSERT-only | |
| Per-user ABAC | 需 user 系统 — v2 SCALE-* | |

**User's choice:** admin vs non-admin 二分
**Notes:** Phase 6 无 user 表,ABAC 推 v2。

---

## Area C — CI gate integration (SC-2, GEVL-07)

### C1 — Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| PR-only 触发 (推荐) | pull_request 事件,skills/eval/internal 改动时触发;push main 不跑 | ✓ |
| PR + push main 均跑 | secret 暴露 + report 污染 | |
| 定时 weekly run | 重复告警 + GLM 费用 | |

**User's choice:** PR-only
**Notes:** path-filtered 触发;push main 不跑避免 GLM_API_KEY 在默认分支暴露。

### C2 — Per-Skill threshold expression

| Option | Description | Selected |
|--------|-------------|----------|
| 每 Skill 独立阈值 + 默认 0.9 (推荐) | .claude/skills/<skill>/eval-gate.json 声明;compliance 0.9;无文件默认 0.9 | ✓ |
| 全部统一 0.9 | 不同 Skill 质量要求不同 | |
| 动态阈值(从 calibration.json) | D2-15 recommended_threshold 是 retrieval 阈值,语义错位 | |

**User's choice:** 每 Skill 独立阈值文件
**Notes:** Phase 6 只 1 个 Skill (compliance) → 1 阈值 0.9。阈值文件与 SKILL.md 同目录,镜像 D2-07 file-driven philosophy。

### C3 — PR feedback

| Option | Description | Selected |
|--------|-------------|----------|
| gh pr comment 拼 markdown (推荐) | 末 step 解析 report.json,贴 per-Skill 表 + ❌/✅;上传 report.json/calibration.json artifact | ✓ |
| 只看 status check + red X | SC-2 要求评论点名 Skill | |
| PR 评论 + Slack webhook | 多 secret 过重 | |

**User's choice:** gh pr comment + artifacts

### C4 — Cost control

| Option | Description | Selected |
|--------|-------------|----------|
| 全部 PR 都跑,后续加 manual approval (推荐) | Phase 6 全跑;Phase 7 看 fork 贡献频次再加 [skip-eval] / workflow_dispatch | ✓ |
| [skip-eval] 标签跳过 | 多 yaml,贡献模式未观察前过早加 | |
| 只对 main 跑 | 违反 SC-2 PR-stage block | |

**User's choice:** 全部 PR 都跑(manual override 推 Phase 7)

---

## Area D — Reproducibility + cross-eval (SC-6 + Pitfall 8)

### D1 — Frozen eval set storage

| Option | Description | Selected |
|--------|-------------|----------|
| eval/frozen/<skill>.jsonl + .lock.json (推荐) | 每 Skill 一份 frozen set + 锁 metadata (model_ver/prompt_hash/seed/version);cmd/eval --frozen | ✓ |
| go:embed 进 binary | 变更需重发 binary,不灵活 | |
| Git LFS | repo 复杂度对小项目过重 | |

**User's choice:** eval/frozen/<skill>.jsonl + .lock.json
**Notes:** 扩展现有 frozen_5.jsonl 模式到 full N=50。

### D2 — Cross-eval workflow

| Option | Description | Selected |
|--------|-------------|----------|
| cmd/eval --judge-models <a,b> 双报告 diff (推荐) | 每 judge 独立 eval/<model>/<ts>.json;dashboard cross-eval tab diff;>5% delta 警告 (Pitfall 8) | ✓ |
| Dashboard 手动启 cross-eval run | 需 service 化,out of scope | |
| docs 描述手动 JUDGE_MODEL 切换 | dashboard 上无落点 | |

**User's choice:** cmd/eval --judge-models 双报告
**Notes:** 把文档里"看不见"的 JUDGE_MODEL slot 变成可见 bias 检测工具 — OSS 品牌叙事 beat。

### D3 — Reproducibility tolerance

| Option | Description | Selected |
|--------|-------------|----------|
| 分层:StubJudge 1e-6 + live 1e-3 (推荐) | 现有 TestRunnerReproducibility 强化为 1e-6;新增 live --frozen judge 模式 ±1e-3 (GLM API jitter) | ✓ |
| 只 StubJudge 1e-6 | SC-6 要 live-frozen | |
| 全部 1e-9 | live GLM API jitter 达不到 | |

**User's choice:** 分层容差
**Notes:** 严格层证明 pipeline 确定性;jitter 层证明实践中稳定。

### D4 — Trend × frozen-set integration

| Option | Description | Selected |
|--------|-------------|----------|
| Trend 含 frozen-set baseline 绿线 (推荐) | trend panel 叠加 frozen-set baseline 灰虚线;"diff vs baseline" 按钮;统一 SC-1 + SC-6 | ✓ |
| Trend 只看 live run 历史 | 交叉验证需手工 | |
| Trend 完全独立 | 弱化复现叙事 | |

**User's choice:** Trend 叠加 baseline + diff 按钮

---

## Claude's Discretion

(从 06-CONTEXT.md `<decisions>` Claude's Discretion 节复制要点)

- **"Citation precision" 语义**:SC-1 列 "faithfulness, citation precision, refusal accuracy",但 internal/eval 只有 4 个指标(无 "citation precision"/"refusal accuracy")。Researcher 解决:最可能是 citation precision = context_precision;refusal accuracy = refusal-required category 派生率。除非 golden set 可算,Phase 6 不引入新指标。
- **SC-4 50×5 并发测试定位**:Phase 3 已 ship chat_session_test.go (2-session)。researcher/planner 决定 50×5 是参数化扩展还是新 integration test。invariant 不变(D18/SC-9 row-level session_id partition)。
- **Per-Skill golden-set seeding for 新 Skills**:out of Phase 6 scope(只证明多 Skill dashboard 架构,用单 compliance Skill)。wizard 加 "author golden set" 步骤推后续 phase。
- **`/api/eval/*` endpoint 设计**:确切 route 形状 + handler 包放置(扩 internal/server vs 新 internal/evalapi)→ planner 决定;lean 扩 internal/server 避免第二 Hertz server。
- **`cmd/eval` flag taxonomy**:`--archive` / `--frozen` / `--judge-models` 交互 → planner 设计 flag matrix;确保无冲突组合。
- **Admin token 缺省行为**:SKILLFORGE_ADMIN_TOKEN 未设时 /api/audit 是 (a) 全 401 还是 (b) 返回 retention-redacted row?researcher/planner 选 — lean (b) 让 demo 开箱即用。

## Deferred Ideas

(从 06-CONTEXT.md `<deferred>` 复制)

- 真 multi-role RBAC / SSO / per-row access_policy 列 → v2 SCALE-02
- DATA-* 金融数据层(DATA-01..14)→ 仍 deferred,自有 phase 或 v2(⚠️ REQUIREMENTS.md traceability 仍把 DATA-* 映射到 Phase 5 + ROADMAP Coverage 55 vs 70 不一致 — docs drift,opportunistic fix)
- Per-Skill golden-set seeding for wizard-generated Skills → 后续 wizard 步骤
- Real-user-sampled golden-set augmentation → D3-06 已 deferred
- `/api/skills/reload` rescan route → 显式不加(Phase 5 D5-IA-05)
- PostgreSQL production store backend → D3-09/D18 PG-ready slot 已留,后续 production-hardening
- Live SSE push of eval-completion to Dashboard → D6-03 选 poll,SSE push deferred
- `[skip-eval]` PR label / workflow_dispatch manual override → Phase 7
- Open-source packaging (DIST-*) → Phase 7
- `spike-temperature-field-validation.md` todo(评分 ~0.4-0.6 recurring keyword false-match)— reviewed 但不 folded
