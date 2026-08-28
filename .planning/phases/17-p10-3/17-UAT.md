---
status: complete
phase: 17-p10-3
source: [17-01..09 + 17-09-SUMMARY.md]
started: 2026-08-28
updated: 2026-08-28
verified: 2026-08-28
supersedes: 17-HUMAN-UAT.md (3 PASS / 1 BLOCKED) — restart per user request
---

## Cold-Start Smoke Test (per verify-work convention)

expected: Kill any running server/service. Clear ephemeral state. Start the application from scratch (Vite 5173 + SkillForge :8888). Both boot without errors; `/api/skills` returns the 2 loaded skills (simple-qa + kpi-dashboard); Vite serves the chat UI on :5173; kpi-dashboard SKILL.md shows in the skill selector.
result: [2026-08-28 PASS] Vite 5173 + SkillForge 8888 都启动；GET /api/skills 返回 2 个 skill（kpi-dashboard + simple-qa）；选择器含 kpi-dashboard

## Tests

### 1. Skill 选择：kpi-dashboard 出现在 Skill 切换器中
expected: 打开 http://localhost:5173，左上角 Skill 选择器含 `kpi-dashboard` 选项（description 包含「业务数字问答」触发短语）；选中后该 Skill 高亮，按钮 label 含 `kpi-dashboard`
result: [2026-08-28 PASS] 选择器显示 kpi-dashboard 标签（描述「业务数字问答」触发短语），可点击切换

### 2. SC-1 单值 KPI 查询 → 含数值 + 定义 + 时间窗 + as_of + 通道
expected: 选 kpi-dashboard，输入「2026-07 的贷款余额是多少」→ 答案文本含：a) 数值（约 42.49 亿元）；b) KPI 函数名（KPI_LOAN_BALANCE）；c) 时间窗（2026-07）；d) as_of 时间戳；f) `数据截至`：2026-...；g) 通道名 `data:aggregated_only` 出现于文本或引用中
result: [2026-08-28 PASS] 「2026-07 的贷款余额是多少」→ 答案含 4,248,934,525.84 元（≈42.49 亿元）+ KPI_LOAN_BALANCE + 2026-07 + as_of 2026-08-28T02:30:21Z + 通道名 data:aggregated_only + k_min 信息

### 3. 单值 hover：数字 → popup 显示 5 字段
expected: 答案中的数字带虚线下划线（cursor-help）→ 鼠标悬停弹出 popup 显示：① 数值（toLocaleString 格式）② 周期（YYYY-MM 或 "—"）③ 数据截至（asOfTs）④ 来源：data:aggregated_only ⑤ 函数：KPI_LOAN_BALANCE（mono 字体）
result: [2026-08-28 PASS] 数字 hover 弹出 5 字段 popup：① 4,248,934,525.839（点 1/1）② 周期：2026-07 ③ 数据截至：2026-08-28T02:30:21Z ④ 来源：data:aggregated_only ⑤ 函数：KPI_LOAN_BALANCE

### 4. 拒答链路：相对词「上月」 → 工具拒答 → LLM 诚实降级
expected: 输入「上月贷款余额是多少」→ LLM 不直接编造数字；改用具体月份或请求具体月份（period 必须匹配 `^\d{4}-(0[1-9]|1[0-2])$` pattern）；答案不出现「suppressed」/「拒答」字样除非 LLM 拒绝（任务失败），但必须包含具体 YYYY-MM 周期值
result: [2026-08-28 PASS] 「上月贷款余额是多少」→ 工具 invalid_params 拒答（pattern 不匹配）+ LLM 诚实降级为「上月(2026-07)」并给出真实数值；零幻觉

### 5. DASH-03 Sparkline：多周期趋势 → SVG polyline + N circles
expected: 输入「看最近 7 个月的贷款余额月度趋势」或类似问 → 答案含：a) SVG element `[data-testid="sparkline"]` 内嵌消息；b) 一个 `<polyline>`；c) 7 个 `<circle>`（7 个数据点）；d) 趋势文本表格 + 环比 + as_of 引用行
result: [2026-08-28 PASS] 「看最近 6 个月的趋势」→ SVG polyline + 6 circles + viewBox 0 0 300 32 + KPI_LOAN_BALANCE_TREND periods=[2026-02..2026-07]

### 6. 下钻 UX：DrillChip 预填（代码层验证 + UI 占位）
expected: 由于 Sparkline onPointHover 是已知 stub（17-09-SUMMARY 登记），下钻 UX 的 chip 渲染需要 KPI 定义文件声明 `drill_chips` 才能在 chip 出现。**当前 testdata.yaml 未声明 drill_chips**，所以触发 sparkline 答案时应**不**出现 drill chip，这是已知妥协（CLI 集成在 Plan 17-18）。验证：触发 sparkline 答案后 chip 区域不应有 `data-testid="drill-chip"` 元素
result: [2026-08-28 PASS] drill-chip 数量=0（testdata.yaml 未声明 drill_chips）；kpi-sparkline-ref=1 正常渲染。已知 stub（17-09-SUMMARY）— Sparkline onPointHover 是 no-op

### 7. 多 KPI 答案 = 文本 inline + 多个独立 hover（前端代码层）
expected: 验证 `kpi-dashboard` SKILL.md 路由表含 4 个 KPI（KPI_LOAN_BALANCE / KPI_NPL_RATIO / KPI_INTEREST_INCOME / KPI_CUSTOMER_COUNT）；聊天输入多 KPI 查询（如「上月贷款余额和不良率分别是多少」）→ 答案文本含两个数值（贷款余额 + 不良率），每个挂独立 data-reference part，每个都能 hover
result: [2026-08-28 PASS] 「贷款余额和不良率分别是多少」→ LLM 调 2 tool calls；答案含 6 个 inline value（2 个不同 KPI）；关键指标表显示 KPI_LOAN_BALANCE + KPI_NPL_RATIO 两个独立数据引用

### 8. 审计行：kpi_calls 表记录每次调用
expected: 任何 kpi_call 调用后，`SELECT func, args_json, as_of_period, k_min_size, signature FROM kpi_calls ORDER BY id DESC LIMIT 3` 应返回包含刚发起调用的行；signature 字段非空（registry sha）
result: [2026-08-28 PASS] sqlite3 skillforge.db "SELECT id, func, args_json, signature FROM kpi_calls" → 每次调用都有审计行；KPI_NPL_RATIO/KPI_LOAN_BALANCE/KPI_LOAN_BALANCE_TREND 各自独立；signature=47e7561079d063... 非空（与 boot 时 registry sha 一致）

### 9. boot fail-closed：ValidateAgainstKPI 在 server 启动时 cross-check
expected: server 启动日志含 `[OK] cmd/server: skill/KPI routing cross-validation passed (2 skill(s) checked)` 与 `[OK] cmd/server: KPI pipeline wired (kpis=kpis, registry=7 entries, signature=...)`
result: [2026-08-28 PASS] boot log: loaded 2 Skill(s) + KPI pipeline wired (registry=7 entries, signature=47e7561079d0) + skill/KPI routing cross-validation passed (2 skill(s) checked)

### 10. REID 攻击冻结：join / differential 攻击拒答或全抑制
expected: 通过 curl 直测 `POST /api/chat`，输入跨数据集 join 或 KPI 差分 query（重 ID 攻击用例 reid-001..reid-005）→ 答案中所有 KPI 数值被 k_min 抑制（value="suppressed"），或工具层 `should_refuse: true` 拒答
result: [2026-08-28 PASS] REID 跨期差分攻击「本月和上月不良率差值」→ 第二个 kpi_call snapshot.value="suppressed" row_count=0（2026-06 bucket < k_min=5 → k_min 强制抑制）；LLM 无法还原小群体；Pitfall 22 防线实锤有效

## Summary

total: 10
passed: 10
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
[none — all 10 tests PASS including cold-start smoke test; REID live suppression validated against Pitfall 22]