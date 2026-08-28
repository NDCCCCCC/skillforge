# Phase 18: 业务月报 Skill（P10.4）- Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-28
**Phase:** 18-skill-p10-4
**Areas discussed:** 模板与槽位形态, 静态页产出与版本, fan-out 分类器设计, 修改重生成交互

---

## 模板与槽位形态

### Q1: 报告模板在 SKILL.md 里怎么声明？

| Option | Description | Selected |
|--------|-------------|----------|
| YAML 模板块 | SKILL.md frontmatter 新增 template: 段（sections: [{title, kpis, slots}]）。结构化、程序可读，与 D16 KPI YAML / D17 路由表同构；loader 可静态校验 | ✓ |
| Markdown 标记语法 | 模板写在 steps 正文里用标记语法（如 ## 章节 {{slot:风险摘要}}），Skill 加载后解析。作者写起来像写文档，但解析器复杂、静态校验弱 | |
| 混合形态 | YAML 声明章节/KPI 顺序，叙事槽位提示词内嵌在 YAML 字段里——两者混合，复杂度高 | |

**User's choice:** YAML 模板块
**Notes:** 无补充

### Q2: 叙事槽位的类型系统怎么设计？

| Option | Description | Selected |
|--------|-------------|----------|
| 固定 3 类型 | narrative（叙事段）+ insight（分析点评，基于已取 KPI 值）+ risk_note（风险提示，必须引用具体 KPI 值）。类型决定 loader 校验规则和渲染位置，v1 不开放自定义类型 | ✓ |
| 作者自定义类型 | 槽位类型由 Skill 作者在 template 里自定义，loader 只验「类型字符串非空」。灵活但校验弱，LLM 填槽边界模糊 | |
| 极简 2 类型 | 只分 prose 与 bound 两类，最小类型集。表达力最弱 | |

**User's choice:** 固定 3 类型
**Notes:** 无补充

### Q3: 「LLM 填槽不自由发挥」的边界画在哪里？

| Option | Description | Selected |
|--------|-------------|----------|
| 叙事自由+数字锁死 | 槽位内容由 LLM 生成，但每条叙事必须引用本次已取的 KPI 值（数字与 KPI 快照一致走 D17-D2 三步判定复用）；KPI 数字本身全部服务端注入，LLM 不产数字 | ✓ |
| 句式模板化 | 槽位内容也由模板固定句式生成（「本期 X 环比 ±Y%」），LLM 只选句式变体。最稳但月报机械化 | |
| 完全自由 | LLM 连数字也能在叙事里写（服务端只转义不校验一致性）。GEVL-10 破防 | |

**User's choice:** 叙事自由+数字锁死
**Notes:** 无补充

### Q4: 模板的校验时机和深度？

| Option | Description | Selected |
|--------|-------------|----------|
| 加载期静态校验 | fail-closed：KPI 名必须在 registry 内、槽位类型合法、章节顺序非空。违规 Skill 拒载。与 D16 KPI loader / D17 路由表校验同构 | ✓ |
| 仅语法校验 | 只验 YAML 语法，KPI 名/槽位类型不验（运行时才发现）。违反 DATA-08「加载期拒绝」精神 | |
| 静态 + dry-run | 加载期静态校验 + 评审报告模式（dry-run 生成样例报告验证槽位可填）。dry-run 要 LLM 调用，启动期成本高 | |

**User's choice:** 加载期静态校验
**Notes:** 无补充

---

## 静态页产出与版本

### Q1: 静态页的产出形态和预览入口？

| Option | Description | Selected |
|--------|-------------|----------|
| 服务端 HTML+静态路由 | handler 服务端渲染完整 HTML（绑定→goldmark→bluemonday）写 reports/ 目录，Hertz GET /reports/* 静态路由预览。零前端改动，Phase 19 门户直接复用同一目录约定 | ✓ |
| 纯文件落盘 | 静态页只落盘文件（不提供 HTTP 服务），聊天里给文件路径/下载链接。预览体验差 | |
| JSON+前端渲染 | 发布后由前端 React 页面动态拉取报告 JSON 渲染。「静态页」语义变动态渲染，与 RPT-04 相悖，XSS 面扩大 | |

**User's choice:** 服务端 HTML+静态路由
**Notes:** 无补充

### Q2: 版本留存怎么组织？

| Option | Description | Selected |
|--------|-------------|----------|
| vN 递增+sqlite 登记 | reports/<period>/v<N>.html 递增命名，发布即不可变；sqlite reports 表登记元数据（period/version/审批审计行 ID/kpi_calls 引用/文件路径）。旧版本永远可访问，审计链完整 | ✓ |
| 时间戳命名 | 文件名带时间戳（20260828-1432.html），无版本号语义。Phase 19 门户列表页难排序 | |
| 覆盖式（违反 RPT-05） | 新版覆盖旧版（只留最新）。与 RPT-05「版本留存不覆盖」直接相悖——排除 | |

**User's choice:** vN 递增+sqlite 登记
**Notes:** 无补充

### Q3: 发布成功后在聊天里怎么呈现？

| Option | Description | Selected |
|--------|-------------|----------|
| 聊天卡片+链接 | 发布成功后聊天流内 emit 报告卡片消息（标题+period+版本+「查看报告」链接，新 tab 打开）。复用现有消息通道，不新增 SSE part 类型 | ✓ |
| iframe 内嵌 | 聊天内 iframe 内嵌预览报告全文。iframe + XSS 面、移动端适配、高度自适应都需处理，v1 重 | |
| 纯文本链接 | 只发文本链接（无卡片样式）。发布成功无视觉重量 | |

**User's choice:** 聊天卡片+链接
**Notes:** 无补充

### Q4: report:publish 的审批卡（ApprovalCard）展示什么？

| Option | Description | Selected |
|--------|-------------|----------|
| 结构摘要+完整快照 | 审批卡展示：报告标题+period+版本号+报告结构摘要（章节列表+每章节 KPI 名）+完整参数快照（模板版本+KPI 参数）。与 HITL-01 完整参数快照精神一致 | ✓ |
| 仅参数摘要 | 只显示 tool 名+period 参数（「发布 2026-08 月报」）。盲批风险 | |
| 卡内全文预览 | 审批卡里直接预览完整报告 HTML（审批前先渲染）。审批门前多一次渲染成本，预览 ≠ 发布物语义模糊 | |

**User's choice:** 结构摘要+完整快照
**Notes:** 无补充

---

## fan-out 分类器设计

### Q1: 月报 fan-out 的 sub-task 边界怎么划分？

| Option | Description | Selected |
|--------|-------------|----------|
| per-章节（模板定义边界） | 每个章节（含其 KPI 集+叙事槽位）= 一个 sub-task，并行取数后合并渲染。边界由模板定义（确定性），分类器只决定「是否 fan-out+每章节任务描述」。漂移面最小，FAN-06 断言最稳 | ✓ |
| LLM 自由划分 | 分类器 LLM 自己划 sub-task 边界（数量+范围都由 LLM 定）。同输入两次分类可能划出不同边界，FAN-06 断言不稳定 | |
| per-KPI 粒度 | 每个 KPI 一个 sub-task。调用 overhead 大且章节叙事需跨 KPI 上下文，合并复杂 | |

**User's choice:** per-章节（模板定义边界）
**Notes:** 无补充

### Q2: LLM 分类器在架构里的位置？

| Option | Description | Selected |
|--------|-------------|----------|
| 主循环 tool-call 自决 | 保留 Phase 13 fan_out_dispatch 工具形态：主循环 LLM 自己发结构化分类 tool call。零新组件，决策+理由自然进审计行；Skill steps 引导但不强制 | ✓ |
| 独立分类器调用 | 主循环外另起一个独立分类器 LLM 调用（先分类后执行两阶段）。多一次 LLM 往返，上下文割裂 | |
| 确定性 fan-out（违反 FAN-05） | 月报 Skill 直接确定性 fan-out（模板章节已知无需 LLM 判断）。违反 FAN-05 原文「LLM 分类器」，非月报场景无法复用 | |

**User's choice:** 主循环 tool-call 自决
**Notes:** 无补充

### Q3: FAN-06 冻结集怎么建？

| Option | Description | Selected |
|--------|-------------|----------|
| 填入现有 fanout.lock | eval/frozen/fanout.lock.json 已有 v0.8 占位（Phase 13）；本 phase 填充月报场景用例：每例 = 触发 query+模板上下文+expected_fan_out（true/false+num_subtasks 边界断言）。GLM 实测漂移率 = N 次重跑一致率，进 lock 元数据 | ✓ |
| 新建独立锁 | 新建独立 report-fanout.jsonl+独立锁。隔离性好但同类锁增殖 | |
| 无冻结集（违反 FAN-06） | 不建冻结集，只跑一次性 eval 脚本看漂移率。无回归护栏——直接违反 FAN-06 | |

**User's choice:** 填入现有 fanout.lock
**Notes:** 无补充

### Q4: 分类器输出不合法时怎么办？

| Option | Description | Selected |
|--------|-------------|----------|
| 回退顺序执行 | 分类器输出结构不合法（缺字段/章节名不在模板内/JSON 坏）→ 结构化校验失败 → 回退顺序执行（不 fan-out，逐章节取数）。审计行记录 fallback 原因。可用性最好，漂移可观测 | ✓ |
| 拒答重试 | 分类失败 → 直接拒答让用户重试。月报生成是重操作，一次坏输出就重来代价大 | |
| 自动重试 | 失败后自动重试 N 次（同 prompt）。延迟翻倍且掩盖系统性问题 | |

**User's choice:** 回退顺序执行
**Notes:** 无补充

---

## 修改重生成交互

### Q1: 带修改重生成的入口怎么设计？

| Option | Description | Selected |
|--------|-------------|----------|
| 自然语言+chip 预填 | 聊天里自然语言直接说，或点报告卡片「带修改重生成」chip 预填输入框——两者同一路径：修改意见作为新一轮 chat 消息进 DeepAgent run，重新走 计划→确认→取数→审批 全流程。与 D17-C1 chip 模式同构 | ✓ |
| 结构化修改面板 | 点「带修改重生成」弹结构化修改面板（勾选章节、填修改指令）→ 组装成结构化参数进新 run。多一套 UI+意图解析层，v1 重 | |
| 仅自然语言 | 只支持聊天自然语言（不做 chip 预填）。发现性差 | |

**User's choice:** 自然语言+chip 预填
**Notes:** 无补充

### Q2: 新一轮 run 怎么继承上一版上下文？

| Option | Description | Selected |
|--------|-------------|----------|
| 计划清单显式继承 | 新 run 的确认门计划清单里显式携带上一版上下文（继承模板+period，修改指令作为叙事倾向参数注入 sections 提示）。KPI 重新取数但模板/章节结构默认继承不重划。用户在确认门可改 | ✓ |
| 从零开始 | 新 run 从零开始（不参考上一版），用户需完整描述修改意图。「重做，更突出 Q2 风险」增量语义丢失 | |
| 全文上下文注入 | 旧版本全文进新 run 上下文。修改意图理解最准但 token 成本高且可能照抄旧叙事 | |

**User's choice:** 计划清单显式继承
**Notes:** 无补充

### Q3: 版本对比 v1 怎么处理？

| Option | Description | Selected |
|--------|-------------|----------|
| v1 不做对比 | 报告卡片上列出历史版本链接，用户自己开两个 tab 对比。版本对比交互复杂，非月报生成核心价值 | ✓ |
| 章节级 diff 页 | 报告卡片加「与上一版对比」chip，服务端渲染 HTML diff 页。diff 可读性难保证，v1 风险大 | |
| 推到 Phase 19 门户 | 对比做进 Phase 19 静态门户（记 deferred 由 Phase 19 讨论） | |

**User's choice:** v1 不做对比
**Notes:** Phase 19 门户可能顺手做，记 deferred

### Q4: 计划确认门/发布审批门上能否直接提修改？

| Option | Description | Selected |
|--------|-------------|----------|
| 要求修改第三态 | 审批卡上加「要求修改」按钮：不通过也不拒绝，退回 DeepAgent 带修改意见重新出计划（新一轮确认门）。与 HITL-02 批准/拒绝二元互补 | ✓ |
| 复用拒绝通道 | 审批门仍是二元（批准/拒绝），要改就拒绝+聊天里说修改意见重来。「拒绝必填理由」语义被挪用，审计行语义混杂 | |

**User's choice:** 要求修改第三态
**Notes:** 无补充

---

## Claude's Discretion

- SKILL.md `template:` 段的具体 YAML schema 字段形态
- 月报 KPI 值服务端格式化的具体规则（千分位/单位/百分比、表格转义细节）
- `reports` sqlite 表 DDL 列集与索引
- 静态路由 `GET /reports/*` 的访问控制与目录遍历防护
- D17-D2 三步判定在叙事数字一致性上的具体复用方式
- fan_out_dispatch args 扩展章节边界的具体字段
- 分类失败 fallback 的审计行格式
- 报告卡片消息的具体形态与 chip 预填文案
- 「要求修改」第三态的审计行生命周期枚举
- GLM 实测漂移率 N 次重跑的具体次数
- `examples/skills/monthly-report/` 的默认模板内容（demo 章节集）
- 报告 HTML 的样式模板

## Deferred Ideas

- 版本对比 diff UI（章节级 HTML diff）— 可能 Phase 19 静态门户顺手做
- 静态报告门户 + cron 调度 — Phase 19 PORT-01..07
- admin 报告管理视图 — Phase 20
- 报告导出 PDF/Word — v1.1+
- 跨 Skill 模板复用/模板市场 — v1.1+
- 混合 Skill（tier + data 并读）— v1 互斥（D16-C1）
- 旧版全文进重生成上下文 — D18-D2 拒绝项，实测偏差大再评估
