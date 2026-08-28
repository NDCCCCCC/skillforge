# Phase 18: 业务月报 Skill（P10.4）- Context

**Gathered:** 2026-08-28
**Status:** Ready for planning
**Source:** `/gsd-discuss-phase 18` — Default mode (4 areas × 4 questions = 16 decisions)
**Requirements:** RPT-01, RPT-02, RPT-03, RPT-04, RPT-05, FAN-05, FAN-06 (7 v1.0 requirements) — 5 Success Criteria (ROADMAP §Phase 18)

<domain>
## Phase Boundary

业务人员在聊天里触发月报生成（「出 2026-08 月报」），DeepAgent 计划清单先行可见（write_todos 复用）→ 执行前计划确认门（Phase 12 机制复用）→ 模板驱动生成（模板 = Skill steps：KPI 顺序 + 类型化叙事槽位，LLM 填槽不自由发挥）→ fan-out 并行取数（LLM 分类器结构化输出，决策进审计）→ `report:publish` 走 HITL 审批门（Phase 11 机制复用；冻结参数快照 + 审计生命周期）→ 产出静态页（vN 版本留存不覆盖）。带修改重生成 = 修改意见进入新一轮 DeepAgent run（非原地补丁）。FAN-06 冻结集 fan-out 率断言防分类器漂移。

**本 phase 交付：**
- 单一 `monthly-report` Skill（`examples/skills/monthly-report/SKILL.md`）：reads `["data:aggregated_only"]`，frontmatter 含 YAML 模板块（sections: [{title, kpis, slots}]）
- 模板槽位系统：固定 3 类型叙事槽位（narrative / insight / risk_note），LLM 填槽 + KPI 数字服务端注入
- 模板加载期 fail-closed 静态校验（KPI 名在 registry、槽位类型合法、章节顺序非空）
- FAN-05：LLM fan-out 分类器（主循环 `fan_out_dispatch` tool-call 自决，per-章节边界由模板定义，结构化输出校验失败回退顺序执行）
- `report:publish` 写工具：注册进 `AllowedWriteTargets`（第三写目标），走 Phase 11 HITL 审批门 + Phase 12 计划确认门，审批卡加「要求修改」第三态
- 静态页渲染沙箱：绑定→goldmark 转义→bluemonday 消毒（管线顺序不可换），KPI 值服务端格式化防 markdown 表格注入（Pitfall 15）
- 静态页产出：服务端 HTML 写 `reports/<period>/v<N>.html` + sqlite reports 元数据表 + Hertz `GET /reports/*` 静态路由
- 发布成功聊天卡片 + 「带修改重生成」chip；重生成 = 新一轮 DeepAgent run，计划清单显式继承上一版上下文
- FAN-06：月报场景用例填入现有 `eval/frozen/fanout.lock.json`（per-case expected_fan_out + num_subtasks 边界断言；GLM 实测漂移率进 lock 元数据）

**不交付（deferred）：**
- 静态报告门户 + cron 调度（门户列表页/历史版本对比页）— Phase 19
- 版本对比 diff UI（章节级 HTML diff 渲染）— 可能 Phase 19 门户顺手做，本 phase 用户自行开 tab 对比
- admin 报告管理视图 — Phase 20
- 跨 Skill 报告模板复用/模板市场 — v1.1+
- 报告定时自动生成（无人工触发）— Phase 19 调度器范围
- 报告导出 PDF/Word — v1.1+

</domain>

<decisions>
## Implementation Decisions

### A. 模板与槽位形态（RPT-03）

- **D18-A1: YAML 模板块。** 模板写在 SKILL.md frontmatter 新增 `template:` 段（`sections: [{title, kpis: [...], slots: [...]}]`）。结构化、程序可读，与 D16 KPI YAML（D16-A1）/ D17 路由表（D17-A1）同构——受控文件哲学第三次落地；loader 可静态校验。拒绝：Markdown 标记语法（解析器复杂、静态校验弱）、混合形态（两套校验，D15-D4 同构理由）。
- **D18-A2: 固定 3 槽位类型。** narrative（叙事段，LLM 写文字）+ insight（分析点评，LLM 基于已取 KPI 值写）+ risk_note（风险提示，LLM 写且必须引用具体 KPI 值）。类型决定 loader 校验规则和渲染位置；v1 不开放自定义类型。拒绝：作者自定义类型（校验弱、LLM 填槽边界模糊）、极简 2 类型（风险摘要和趋势点评的差异化要求无处安放）。
- **D18-A3: 叙事自由 + 数字锁死。** 「LLM 填槽不自由发挥」的边界：槽位内容由 LLM 生成（这是「叙事」的本意），但 KPI 数字本身全部服务端注入，LLM 不产数字；叙事文本中出现的数字必须与本次已取 KPI 快照一致（D17-D2 三步判定复用：去格式化 + 容差 ±0.5% + allowed_expressions）。拒绝：纯自由发挥（Pitfall 15 破防 + GEVL-10 破防）、句式模板化（月报机械化，叙事价值低）。
- **D18-A4: 加载期 fail-closed 静态校验。** Skill 加载时校验模板：KPI 名必须在 KPI registry 内、槽位类型 ∈ {narrative, insight, risk_note}、章节顺序非空；违规 Skill 拒载（启动期 + 热重载期都拒），错误信息点名冲突字段。与 D16 KPI loader / D17 路由表校验同构。拒绝：仅语法校验（配置错误不可见，违反 DATA-08 精神）、静态+dry-run（dry-run 要 LLM 调用，启动期成本高）。

### B. 静态页产出与版本（RPT-04）

- **D18-B1: 服务端 HTML + 静态路由。** `report:publish` handler 在服务端渲染完整 HTML（模板绑定→goldmark 转义→bluemonday 消毒，管线顺序不可换）写入 `reports/` 目录（如 `reports/2026-08/v1.html`），Hertz 加 `GET /reports/*` 静态路由提供预览。零前端渲染逻辑，链接可分享，Phase 19 门户直接复用同一目录约定。拒绝：纯文件落盘（预览体验差，Phase 19 还得补 HTTP 面）、JSON+前端渲染（「静态页」语义变动态渲染，与 RPT-04 原文相悖，XSS 面扩大到前端）。
- **D18-B2: vN 递增 + sqlite 登记。** 文件命名 `reports/<period>/v<N>.html` 递增（v1, v2, ...），发布即不可变（写入后不改）；sqlite `reports` 元数据表登记（period / version / 审批审计行 ID / kpi_calls 引用 / 文件路径 / 模板版本）。旧版本永远可访问，审计链完整（RPT-05 版本留存不覆盖）。拒绝：时间戳命名（无版本号语义，Phase 19 列表页难排序）、覆盖式（直接违反 RPT-05）。
- **D18-B3: 聊天卡片 + 链接。** 发布成功后聊天流内 emit 报告卡片消息（标题 + period + 版本 + 「查看报告」链接，新 tab 打开静态页 + 「带修改重生成」chip）。复用现有消息通道，不新增 SSE part 类型（D17-B2 字段扩展哲学延续）。拒绝：iframe 内嵌（XSS 面 + 高度自适应 + 移动端适配，v1 重）、纯文本链接（发布事件无视觉重量）。
- **D18-B4: 结构摘要 + 完整快照审批卡。** `report:publish` 审批卡展示：报告标题 + period + 版本号 + 报告结构摘要（章节列表 + 每章节 KPI 名）+ 完整参数快照（模板版本 + KPI 参数）。用户看清「批的是什么」再点批准——与 HITL-01 完整参数快照精神一致。拒绝：仅参数摘要（盲批，批的 A 可能渲染成 B）、卡内全文预览（审批前多一次渲染成本，预览 ≠ 发布物语义模糊）。

### C. fan-out 分类器（FAN-05/06）

- **D18-C1: per-章节边界由模板定义。** 月报 fan-out 的天然边界 = 模板章节：每个章节（含其 KPI 集 + 叙事槽位）= 一个 sub-task，并行取数后合并渲染。边界由模板定义（确定性），分类器只决定「是否 fan-out + 每章节任务描述」，不自由划分。漂移面最小，FAN-06 断言最稳。拒绝：LLM 自由划分（同输入不同边界，expected_fan_out 断言不稳，审计对账难）、per-KPI 粒度（调用 overhead 大且章节叙事需跨 KPI 上下文）。
- **D18-C2: 主循环 tool-call 自决。** 保留 Phase 13 `fan_out_dispatch` 工具形态：月报 DeepAgent 主循环 LLM 自己发结构化分类 tool call（args = `{should_fan_out, num_subtasks, task_boundaries[]}`）。零新组件，决策 + 理由自然进审计行；Skill steps 提示「本 Skill 章节独立，适合 fan-out」引导但不强制。拒绝：独立分类器 LLM 调用（多一次往返，上下文割裂）、确定性 fan-out（违反 FAN-05「LLM 分类器」原文，非月报场景无法复用）。
- **D18-C3: 填入现有 fanout.lock。** FAN-06 冻结集填入 Phase 13 已建的 `eval/frozen/fanout.lock.json`（v0.8 占位）：月报场景用例 per-case `expected_fan_out`（true/false + num_subtasks 边界断言）。GLM 实测漂移率 = N 次重跑分类结果一致率，进 lock 元数据。拒绝：新建独立锁（fanout.lock 占位已在，同类锁增殖违背惯例）、无冻结集（违反 FAN-06，无回归护栏）。
- **D18-C4: 分类失败回退顺序执行。** 分类器输出结构不合法（缺字段 / 章节名不在模板内 / JSON 坏）→ 结构化校验失败 → 回退顺序执行（不 fan-out，逐章节取数），审计行记录 fallback 原因。可用性最好（报告总能出），漂移可观测。拒绝：拒答重试（月报生成是重操作，重来代价大）、自动重试 N 次（延迟翻倍且掩盖系统性问题）。

### D. 修改重生成（RPT-05）

- **D18-D1: 自然语言 + chip 预填。** 聊天里自然语言直接说（「重做，更突出 Q2 风险」）或点报告卡片「带修改重生成」chip 预填输入框——两者同一路径：修改意见作为新一轮 chat 消息进 DeepAgent run，重新走 计划→确认→取数→审批 全流程。与 D17-C1 chip 模式同构（Skill 决定 chip 语义，UI 只渲染）。拒绝：结构化修改面板（多一套 UI + 意图解析层，v1 重）、仅自然语言（发现性差）。
- **D18-D2: 计划清单显式继承。** 新 run 的确认门计划清单显式携带上一版上下文：继承模板 + period，修改指令作为叙事倾向参数注入 sections 提示；KPI 重新取数（as_of 可能更新）但模板/章节结构默认继承不重划；用户在确认门可改。拒绝：从零开始（「更突出 Q2 风险」增量语义丢失，LLM 可能重划完全不同的结构）、旧版全文进上下文（token 成本高且可能照抄旧叙事）。
- **D18-D3: v1 不做版本对比 UI。** 报告卡片上列出历史版本链接，用户自行开两个 tab 对比。版本对比交互复杂（字段级 diff 高亮），非月报生成核心价值。拒绝：章节级 diff 页（LLM 叙事全变 vs 数字微调的可读性难保证，v1 风险大）。
- **D18-D4: 「要求修改」第三态。** 审批卡加「要求修改」按钮：不通过也不拒绝，退回 DeepAgent 带修改意见重新出计划（新一轮确认门）。与 HITL-02 批准/拒绝二元决策互补，覆盖「看到计划才发现要改」场景。拒绝：复用拒绝通道（「拒绝必填理由」语义被挪用为修改意见，审计行语义混杂）。

### Claude's Discretion

- **SKILL.md `template:` 段的具体 YAML schema 字段形态**（sections 数组元素的字段名、slots 结构、叙事倾向参数的注入语法）
- **月报 KPI 值服务端格式化的具体规则**（千分位/单位/百分比格式、markdown 表格单元格转义细节——Pitfall 15 的服务端执行面）
- **`reports` sqlite 表 DDL 列集与索引**（D18-B2 语义不变）
- **静态路由 `GET /reports/*` 的访问控制**（v1 本地部署是否需要鉴权、目录遍历防护）
- **D17-D2 三步判定在叙事数字一致性上的具体复用方式**（容差/allowed_expressions 是硬校验还是 eval 期断言）
- **fan_out_dispatch args 扩展章节边界的具体字段**（task_boundaries 如何引用模板章节名）
- **分类失败 fallback 的审计行格式**（fallback_reason 字段名与取值枚举）
- **报告卡片消息的具体形态**（复用哪个现有消息通道、卡片字段、chip 交互预填文案）
- **「要求修改」第三态的审计行生命周期**（action_pending_approval → action_change_requested 的新枚举值与 HITL-04 三行生命周期的兼容）
- **GLM 实测漂移率 N 次重跑的具体次数**（FAN-06 lock 元数据口径）
- **`examples/skills/monthly-report/` 的默认模板内容**（demo 用章节集：概览/贷款/风险/客户 等的具体划分）
- **报告 HTML 的样式模板**（内联 CSS 形态、打印友好性）

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 研究文档（实现级已研究）
- `.planning/research/STACK-VERIFICATION.md` — goldmark v1.8.5（默认转义 raw HTML，零依赖）+ bluemonday v1.0.27（**正确路径 `github.com/microcosm-cc/bluemonday`**，UGCPolicy 含 AllowTables）+ stdlib text/template 绑定；「绑定→渲染→消毒」顺序确认（bluemonday README: "always run bluemonday after any other processing"）
- `.planning/research/PITFALLS.md` Pitfall 15 — 模板注入/报告 RCE（模板 = 数据不是代码；渲染沙箱管线；LLM 只填槽——本 phase 核心威胁模型）
- `.planning/research/PITFALLS.md` Pitfall 19 — eval 审计漂移（FAN-06 冻结集断言延续此防御）
- `.planning/research/ARCHITECTURE.md` — Q2 kpi_call Tool archetype（report:publish 注册面镜像）；Q3 renderer 进程模型（v1 倾向进程内 + 消毒管线，plan 期定——本 CONTEXT 决策 D18-B1 已按进程内落地）
- `.planning/research/FEATURES.md` — Capability 4（报告生成 table stakes / differentiators，模板驱动为核心卖点）

### 需求 + 路线图锚
- `.planning/REQUIREMENTS.md` §业务月报 Skill (RPT) — RPT-01..05 原文（聊天触发 / 计划确认门 / 模板驱动 / HITL 发布 / 带修改重生成）
- `.planning/REQUIREMENTS.md` §Sub-Agent Fan-Out — FAN-05（LLM 分类器结构化输出）+ FAN-06（冻结集 fan-out 率断言）原文
- `.planning/REQUIREMENTS.md` §Write Tools + Approval Gate (HITL) — HITL-01..08（report:publish 审批门复用的机制母体）
- `.planning/REQUIREMENTS.md` §Planning Layer (PLAN) — PLAN-01..04（DeepAgent archetype + write_todos + 计划确认门复用的机制母体）
- `.planning/ROADMAP.md` §Phase 18 — Goal / 5 SC / Pitfalls addressed / Research flags（Q3 renderer 进程模型 + FAN-05 GLM 漂移率）

### 前置 phase 决策（直接继承）
- `.planning/phases/17-p10-3/17-CONTEXT.md` — D17-A1 Skill steps 路由表（模板块 YAML 同构）/ D17-A2 单 Skill 形态（monthly-report 适用）/ D17-B2 data-reference part 数组化不扩 union（聊天卡片通道）/ D17-B3 多周期 KPI 一次调用（月报取数形态）/ D17-C1 chip 模式（重生成 chip 同构）/ D17-D2 三步数值判定（叙事数字一致性复用）
- `.planning/phases/16-p10-2/16-CONTEXT.md` — D16-A1 YAML 受控文件（模板块第三次落地）/ D16-B1 单一 kpi_call / D16-B3 data-decision 通道 + data-reference part / D16-B4 as_of 系统自动生成 / D16-C1 reads 互斥 / D16-C2/C3 通道守卫双层执法 / D16-C4 singleton registry（同 KPI 跨 Skill 必同结果）/ D16-D3 result_snapshot 全量冻结 / D16-D4 kpi_calls INSERT-only
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-CONTEXT.md` — D11-B1 运行时动态拼 Tool whitelist / D11-B3 AllowedWriteTargets 单文件 source of truth（report:publish 注册 = append）；HITL-04 三行审计生命周期 + 冻结参数快照
- `.planning/phases/12-p8-3-planning-layer/12-CONTEXT.md`（如存在，否则 12-*SUMMARY）— PLAN-02/03 write_todos 3-effect contract + 计划确认门（月报计划清单先行可见复用）
- `.planning/phases/13-p8-4-sub-agent-fan-out/13-CONTEXT.md`（如存在，否则 13-*SUMMARY）— fan_out_dispatch 工具形态 + fanout.lock.json v0.8 占位（FAN-05/06 落点）

### 已交付核心资产（直接扩展）
- `internal/agent/hitl/registry.go` — `AllowedWriteTargets`（现含 memory:facts + review_report；report:publish = 第三写目标 append 点）+ handlers 单文件 source of truth
- `internal/agent/hitl/hitl_plan_approve.go` — Phase 12 计划确认门实现（RPT-02 复用对象）
- `internal/agent/deep_audit.go` — write_todos 3-effect contract（计划清单先行可见的 substrate）
- `internal/agent/fanout_tools.go` — `fan_out_dispatch` 工具（FAN-05 分类器落点；args 扩展点）
- `internal/runtime/deep_agent.go` + `factory.go` — DeepAgent archetype 工厂（`agent: deep` 分发）
- `internal/kpi/`（registry/loader/define/executor/multi_period/kmin）— KPI 通道完整基建（月报取数直接消费，只读不改）
- `internal/store/sqlite/kpi_calls.go` — INSERT-only 审计表模式（reports 表镜像对象）
- `internal/agent/runner_sse.go` — SSE 通道机制（聊天卡片 emit 落点，不扩 union）
- `web/src/components/ApprovalCard.tsx` + `PlanApprovalCard.tsx` — 审批卡 UI（report:publish 审批卡 + 「要求修改」第三态扩展点）
- `web/src/components/chat/MessageBubble.tsx` — 消息渲染（报告卡片 + chip 落点）
- `eval/frozen/fanout.lock.json` — v0.8 占位（FAN-06 填充对象）
- `examples/skills/compliance/SKILL.md` + `examples/skills/kpi-dashboard/SKILL.md` — Skill 模板镜像对象（monthly-report 同级放置）
- `internal/skill/loader.go` + `manifest.go` — Skill 加载/校验/fsnotify（template: 段校验扩展点）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Phase 11 HITL 全套**（interrupt / 审批卡 / 三行审计生命周期 / 冻结参数快照 / tool_call_id 幂等 / approve-and-remember）— report:publish 走完全相同的门；新增面 = AllowedWriteTargets append + handler + 审批卡第三态按钮
- **Phase 12 DeepAgent 计划层**（write_todos + 计划确认门 + SSE live trace）— 月报 计划清单先行可见（RPT-01）+ 执行前确认门（RPT-02）零新机制复用
- **Phase 13 fan_out_dispatch 工具** — FAN-05 不新建组件：工具已在，分类逻辑从「占位」升级为「主循环 LLM 结构化输出 + 模板边界校验 + fallback 顺序执行」
- **Phase 16/17 KPI 通道**（多周期 periods / kpi_calls 审计 / data-reference part / 三步判定）— 月报取数全部走 kpi_call；章节 KPI 集 = 模板声明
- **goldmark + bluemonday** — STACK-VERIFICATION 已验证选型；渲染管线 = handler 内进程内实现（Q3 倾向已落地为 D18-B1）

### Established Patterns
- **受控 YAML 第三次落地**（datarules loans.yaml → KPI 定义 → 模板块）— 文件是唯一真源 + loader fail-closed + git diff 评审纪律
- **D11-B1/B3 双层执法镜像** — template: 段加载期校验（D18-A4）+ 运行时 BuildToolsForSkill 拼 {kpi_call, report:publish} 工具集
- **INSERT-only 专用表模式**（import_batches → kpi_calls → reports）— 发布即不可变 + 审计行 ID 关联
- **D17-B2 字段扩展不扩 union** — 聊天卡片复用现有消息通道；wire union 编译期报警保持绿
- **D17-C1 chip 模式** — Skill 决定 chip 语义，UI 只渲染；重生成 chip 与下钻 chip 同构
- **冻结锁 byte-identity 模式**（TestShippedFrozenLockIsInSync 镜像）— fanout.lock.json 填充后进 phase 验收门

### Integration Points
- **`internal/agent/hitl/registry.go`** — `AllowedWriteTargets` append `"report:publish"` + 新 handler `hitl_report_publish.go`（镜像 hitl_review_report.go）
- **`internal/skill/manifest.go` + `loader.go`** — `template:` frontmatter 段解析 + 模板静态校验（fail-closed 拒载分支）
- **`internal/agent/fanout_tools.go`** — args 结构化校验（章节名 ∈ 模板 sections）+ fallback 顺序执行分支
- **新 `internal/report/` 包**（渲染沙箱：绑定→goldmark→bluemonday 管线 + KPI 值服务端格式化）+ `reports/` 目录约定
- **新 sqlite `reports` 表** — additive 迁移（D15-02 模式）
- **Hertz `GET /reports/*` 静态路由** — `cmd/server/main.go` 或 server 装配点注册
- **`web/src/components/ApprovalCard.tsx`** — 「要求修改」第三态按钮 + report:publish 结构摘要展示分支
- **`web/src/components/chat/MessageBubble.tsx`** — 报告卡片消息渲染 + 「带修改重生成」chip
- **`eval/frozen/fanout.lock.json`** — 月报用例填充 + expected_fan_out 断言 + GLM 漂移率元数据
- **`examples/skills/monthly-report/SKILL.md`** — 新 Skill 文件（template: 段 + steps 引导 + reads=["data:aggregated_only"] + writes=["report:publish"]）

</code_context>

<specifics>
## Specific Ideas

- **「模板 = 数据不是代码」是 Pitfall 15 的第一性防御** — 模板块是 Skill 作者声明的数据（章节/KPI 顺序/槽位类型），不是可执行逻辑；LLM 只填槽，模板本身无任何代码执行面；渲染沙箱三层（绑定转义→goldmark 默认转义 raw HTML→bluemonday 消毒）是纵深防御
- **「LLM 填槽不自由发挥」的精确语义 = 叙事自由 + 数字锁死**（D18-A3）— 用户反复确认的核心边界：叙事质量是月报价值，数字一致性是信任底线；两者不冲突因为数字根本不经过 LLM
- **per-章节 fan-out 边界是「确定性优先」哲学的延伸** — 分类器保留 LLM 判断（是否 fan-out）但边界来自模板（确定性），FAN-06 expected_fan_out 断言因此可稳定
- **「要求修改」第三态让审批门从终态判断变成协作循环** — 计划确认门/发布审批门都能自然回流到新一轮 DeepAgent run，与 RPT-05 重生成机制形成闭环
- **版本留存 = 审计链的物理形态** — vN 文件不可变 + sqlite 登记 + 审批审计行 ID 关联，任意历史版本可完整回溯「当时批的什么、数据从哪来」（kpi_calls 引用）
- **重生成 = 新 run 非原地补丁**（RPT-05 原文）— 修改意见走完整 计划→确认→取数→审批 流程；继承的是「上下文与结构倾向」不是「结果快照」；旧版本永不覆盖

## Specific test lock requirements

- **TestMonthlyReportTemplateLoadFailClosed**：模板块校验失败（KPI 名不在 registry / 槽位类型非法 / 章节为空）→ Skill 拒载，错误点名冲突字段（D18-A4）
- **TestTemplateSlotTypesFixed**：slots 类型 ∈ {narrative, insight, risk_note}；自定义类型加载期被拒（D18-A2）
- **TestKPINumbersServerInjected**：KPI 数字由服务端注入渲染输出，LLM 输出中的数字 token 不进渲染管线（D18-A3 结构性验证）
- **TestNarrativeNumberConsistency**：叙事文本数字与 KPI 快照三步判定一致；不一致 → 渲染失败/标记（D18-A3）
- **TestRenderPipelineOrder**：绑定→goldmark→bluemonday 顺序结构性断言；goldmark 输出前不得触碰 bluemonday 之后的代码路径（Pitfall 15）
- **TestMarkdownTableInjectionBlocked**：KPI 值含 markdown 表格语法字符（| 等）→ 服务端格式化转义，渲染输出无注入面（Pitfall 15 服务端执行面）
- **TestReportPublishHitlGate**：report:publish 未审批不执行；审批走三行审计生命周期 + 冻结参数快照（批的 A 渲染成 A）（D18-B4 + HITL-04 镜像）
- **TestApprovalCardShowsStructure**：审批卡 payload 含章节列表 + 每章节 KPI 名 + 模板版本（D18-B4）
- **TestRequestChangeThirdState**：「要求修改」产生独立审计枚举（非 approved 非 rejected）+ 触发新一轮计划确认门（D18-D4）
- **TestVersionIncrementNoOverwrite**：同 period 重复发布 v1→v2 文件共存；v1 字节不变（RPT-05 + D18-B2）
- **TestReportsTableAuditChain**：reports 行含审批审计行 ID + kpi_calls 引用；可完整回溯（D18-B2）
- **TestFanOutBoundaryFromTemplate**：task_boundaries 章节名必须 ∈ 模板 sections；越界 → 校验失败（D18-C1）
- **TestFanOutClassifierStructuredOutput**：分类器输出 {should_fan_out, num_subtasks, task_boundaries} 进审计行含理由（FAN-05）
- **TestFanOutFallbackSequential**：分类输出不合法 → 回退顺序执行 + 审计行记录 fallback_reason（D18-C4）
- **TestFanOutFrozenSetAssertion**：fanout.lock.json 月报用例 per-case expected_fan_out 全过；漂移可检测（FAN-06）
- **TestRegenerateNewRunNotPatch**：带修改重生成触发新一轮 DeepAgent run（新确认门），非原地修改已发布 HTML（RPT-05 + D18-D1/D2）
- **TestReportCardChatMessage**：发布成功 emit 报告卡片（标题+period+版本+链接+chip）；wire union 不扩（D18-B3）
- **TestStaticRouteServesReports**：GET /reports/2026-08/v1.html 返回已发布 HTML；目录遍历被拒（D18-B1）
- **TestNoTemplateCodeExecutionSurface**：模板块解析全程无代码执行面（WalkDir/结构性断言——模板只在渲染沙箱数据流中出现）（Pitfall 15 结构锁）

</specifics>

<deferred>
## Deferred Ideas

- **版本对比 diff UI**（章节级 HTML diff 渲染）— 可能 Phase 19 静态门户顺手做（门户本身要列表页 + 历史版本导航）；本 phase 用户开两个 tab 自行对比（D18-D3）
- **静态报告门户 + cron 调度**（定时自动生成 / 门户列表页 / 报告归档浏览）— Phase 19 PORT-01..07
- **admin 报告管理视图**（删除/下线已发布报告）— Phase 20 ADM
- **报告导出 PDF/Word** — v1.1+（渲染管线已产出 HTML，转换层后议）
- **跨 Skill 模板复用/模板市场** — v1.1+（模板随 Skill 走，暂无共享需求）
- **混合 Skill（tier 知识 + data 数据并读，月报引用制度条文）** — v1 互斥（D16-C1），业务明确后再开
- **旧版全文进重生成上下文**（更精准的修改理解）— D18-D2 拒绝项；token 成本高，若 D18-D2 实测理解偏差大再评估

</deferred>

---

*Phase: 18-skill-p10-4*
*Context gathered: 2026-08-28 via /gsd-discuss-phase 18 (Default mode, 4 areas × 4 questions = 16 decisions)*
