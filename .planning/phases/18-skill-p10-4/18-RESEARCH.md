# Phase 18: 业务月报 Skill（P10.4）- Research

**Researched:** 2026-08-28
**Domain:** 模板驱动报告生成（goldmark/bluemonday 渲染沙箱）+ HITL 第三写目标 + DeepAgent data-channel fan-out 分类器 + 静态页服务
**Confidence:** HIGH（全部实现锚点来自本仓 HEAD 源码逐行核对 + 官方 README + goproxy.cn 版本实测）

## Summary

Phase 18 的**新代码面很小，解锁面很大**。渲染沙箱（绑定→goldmark 转义→bluemonday 消毒）、`report:publish` 第三写目标、reports INSERT-only 表、静态路由、聊天卡片——每一项都有 Phase 11/12/13/16/17 的直接镜像母体（[VERIFIED: 本仓源码] hitl_review_report.go / deep_audit.go / kpi_calls.go / fanout.lock.json / wire.DecisionPayload）。真正的研究发现集中在**一个此前未被任何文档点名的结构性冲突**：monthly-report 是全仓第一个 `data-channel + agent:deep + writes:[report:publish]` 的 Skill，而 Phase 16 建立的 fail-closed 守卫**当前会在加载期直接拒绝这个组合**（`internal/skill/validate.go:121-126` 显式禁止 deep+data；`internal/agent/runner_setup.go:238-248` 把 data-channel 工具集硬编码为恰好 `{kpi_call}`；`internal/runtime/deep_agent.go:167-177` 的 deep 提示词硬编码 retrieve 工具承诺）。解锁这三处守卫（而非绕过）是本 phase 的架构主轴——CONTEXT 的 D18 决策隐含了这一点，但没有指名代码位置；本研究完成了定位并给出解锁方案。

第二优先发现：FAN-05/06 的落点比 CONTEXT 预想的更完整——fan-out 决策审计**不需要新表**（Phase 13 的 fanout 事件本来就骑在 `audit` 表 `action_fanout_*` 枚举 + run_path 列上，`Reason` 列天然承载 fallback_reason）；`eval/golden/fanout.jsonl` 的 per-case 结构已有 `expected_dispatched_count_floor` 等期望字段，月报用例只需**加法扩展**两个字段（`expected_fan_out` + `expected_num_subtasks`）。渲染管线方面，goldmark v1.8.5 / bluemonday v1.0.27 版本今日经 goproxy.cn 实测仍为最新；x/net 已在 v0.43.0（STACK-VERIFICATION 预期的 v0.26.0 抬升早已被后续升级覆盖，零依赖摩擦）；Hertz v0.10.6 源码核对确认 `StaticFS` 可用但**不存在** WebSearch 声称的 `PathTraversalRestricted` 内建中间件——推荐用正则白名单自定义 handler 实现「目录遍历按构造被拒」。

**Primary recommendation:** 按「先解锁三守卫 → 再建 internal/report 渲染沙箱 → 再挂 report:publish HITL 门 → 最后填 FAN-05/06 冻结集」的依赖序规划；渲染沙箱的 KPI 值服务端格式化采用「数字格式化到最终呈现形态 + markdown 语法字符全转义」的双保险规则；「要求修改」第三态走 `action_change_requested` 加法枚举 + 复用 deny 型终态行为（当前 run 优雅收尾 + chip 预填新 run）。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions（16 项，全部 LOCKED — 研究只实现，不重开）

**A. 模板与槽位形态（RPT-03）**
- **D18-A1: YAML 模板块。** 模板写在 SKILL.md frontmatter 新增 `template:` 段（`sections: [{title, kpis: [...], slots: [...]}]`）。与 D16 KPI YAML / D17 路由表同构——受控文件哲学第三次落地；loader 可静态校验。拒绝：Markdown 标记语法、混合形态。
- **D18-A2: 固定 3 槽位类型。** narrative（叙事段）+ insight（分析点评，基于已取 KPI 值）+ risk_note（风险提示，必须引用具体 KPI 值）。类型决定 loader 校验规则和渲染位置；v1 不开放自定义类型。拒绝：作者自定义类型、极简 2 类型。
- **D18-A3: 叙事自由 + 数字锁死。** 槽位内容由 LLM 生成，但 KPI 数字全部服务端注入，LLM 不产数字；叙事文本中出现的数字必须与本次已取 KPI 快照一致（**D17-D2 三步判定复用：去格式化 + 容差 ±0.5% + allowed_expressions**）。拒绝：纯自由发挥、句式模板化。
- **D18-A4: 加载期 fail-closed 静态校验。** KPI 名必须在 KPI registry 内、槽位类型 ∈ 3 值域、章节顺序非空；违规 Skill 拒载（启动期 + 热重载期都拒），错误点名冲突字段。拒绝：仅语法校验、静态+dry-run。

**B. 静态页产出与版本（RPT-04）**
- **D18-B1: 服务端 HTML + 静态路由。** handler 在服务端渲染完整 HTML（模板绑定→goldmark 转义→bluemonday 消毒，**管线顺序不可换**）写 `reports/<period>/v<N>.html`，Hertz 加 `GET /reports/*` 静态路由。零前端渲染逻辑。拒绝：纯文件落盘、JSON+前端渲染。
- **D18-B2: vN 递增 + sqlite 登记。** 文件命名 `reports/<period>/v<N>.html` 递增，发布即不可变；sqlite `reports` 元数据表登记（period / version / 审批审计行 ID / kpi_calls 引用 / 文件路径 / 模板版本）。拒绝：时间戳命名、覆盖式。
- **D18-B3: 聊天卡片 + 链接。** 发布成功后聊天流内 emit 报告卡片（标题 + period + 版本 + 「查看报告」链接新 tab + 「带修改重生成」chip）。复用现有消息通道，**不新增 SSE part 类型**（D17-B2 字段扩展哲学）。拒绝：iframe 内嵌、纯文本链接。
- **D18-B4: 结构摘要 + 完整快照审批卡。** 审批卡展示报告标题 + period + 版本号 + 报告结构摘要（章节列表 + 每章节 KPI 名）+ 完整参数快照。拒绝：仅参数摘要、卡内全文预览。

**C. fan-out 分类器（FAN-05/06）**
- **D18-C1: per-章节边界由模板定义。** 每个章节（含其 KPI 集 + 叙事槽位）= 一个 sub-task；边界由模板定义（确定性），分类器只决定「是否 fan-out + 每章节任务描述」。拒绝：LLM 自由划分、per-KPI 粒度。
- **D18-C2: 主循环 tool-call 自决。** 保留 Phase 13 `fan_out_dispatch` 工具形态：主循环 LLM 发结构化分类 tool call（args = `{should_fan_out, num_subtasks, task_boundaries[]}`）。零新组件，决策 + 理由自然进审计行。拒绝：独立分类器 LLM 调用、确定性 fan-out。
- **D18-C3: 填入现有 fanout.lock。** 月报用例填入 `eval/frozen/fanout.lock.json`（v0.8 占位）：per-case `expected_fan_out`（true/false + num_subtasks 边界断言）。GLM 实测漂移率 = N 次重跑分类结果一致率，进 lock 元数据。拒绝：新建独立锁、无冻结集。
- **D18-C4: 分类失败回退顺序执行。** 分类输出不合法（缺字段 / 章节名不在模板内 / JSON 坏）→ 结构化校验失败 → 回退顺序执行（逐章节取数），审计行记录 fallback 原因。拒绝：拒答重试、自动重试 N 次。

**D. 修改重生成（RPT-05）**
- **D18-D1: 自然语言 + chip 预填。** 自然语言或点 chip 预填输入框——同一路径：修改意见作为新一轮 chat 消息进 DeepAgent run，重新走 计划→确认→取数→审批 全流程。与 D17-C1 chip 模式同构。拒绝：结构化修改面板、仅自然语言。
- **D18-D2: 计划清单显式继承。** 新 run 的确认门计划显式携带上一版上下文：继承模板 + period，修改指令作为叙事倾向参数注入 sections 提示；KPI 重新取数但模板/章节结构默认继承不重划；用户在确认门可改。拒绝：从零开始、旧版全文进上下文。
- **D18-D3: v1 不做版本对比 UI。** 报告卡片列出历史版本链接，用户自行开 tab 对比。拒绝：章节级 diff 页。
- **D18-D4: 「要求修改」第三态。** 审批卡加「要求修改」按钮：不通过也不拒绝，退回 DeepAgent 带修改意见重新出计划（新一轮确认门）。拒绝：复用拒绝通道。

### Claude's Discretion（CONTEXT 原文列出，本研究给出建议）
- SKILL.md `template:` 段的具体 YAML schema 字段形态（本研究给出建议 schema，见 §Architecture Patterns Pattern 1）
- 月报 KPI 值服务端格式化的具体规则（千分位/单位/百分比 + markdown 表格单元格转义细节——见 Pattern 2）
- `reports` sqlite 表 DDL 列集与索引（D18-B2 语义不变——见 Pattern 6）
- 静态路由 `GET /reports/*` 的访问控制（v1 本地部署鉴权 + 目录遍历防护——见 Pattern 7）
- D17-D2 三步判定在叙事数字一致性上的复用方式（硬校验 vs eval 期断言——见 Open Questions）
- fan_out_dispatch args 扩展章节边界的具体字段（task_boundaries 如何引用模板章节名——见 Pattern 5）
- 分类失败 fallback 的审计行格式（fallback_reason 字段名与取值枚举——已定位到 audit 表 Reason 列）
- 报告卡片消息的具体形态（复用哪个通道、卡片字段、chip 预填文案——见 Pattern 8）
- 「要求修改」第三态的审计行生命周期（见 Pattern 4）
- GLM 实测漂移率 N 次重跑的具体次数（建议 N=5，见 Open Questions）
- `examples/skills/monthly-report/` 的默认模板内容（demo 章节集——planner/executor 定）
- 报告 HTML 的样式模板（内联 CSS、打印友好）

### Deferred Ideas（OUT OF SCOPE — 忽略）
版本对比 diff UI（Phase 19 顺手）；静态报告门户 + cron 调度（Phase 19 PORT-01..07）；admin 报告管理视图（Phase 20）；报告导出 PDF/Word（v1.1+）；跨 Skill 模板复用/模板市场（v1.1+）；混合 Skill tier+data 并读（v1 互斥）；旧版全文进重生成上下文（D18-D2 拒绝项）。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RPT-01 | 聊天触发月报生成，DeepAgent 计划清单先行可见 | write_todos 3-effect contract 已就位（deep_audit.go）；**但需先解锁 deep+data 加载禁令**（F1）+ data-channel 工具集扩容（F2） |
| RPT-02 | 计划确认门复用（PLAN-03 模式），执行前批准 KPI 集与报告结构 | hitl_plan_approve.go 双 pass 契约 + plan_api.go 服务端面直接复用；monthly-report 需要 writes 声明 `plan_approve` |
| RPT-03 | 报告模板 = Skill 的一部分（steps 声明 KPI 顺序 + 叙事槽位）；LLM 填槽不自由发挥 | template: 段 YAML schema（Pattern 1）+ loader fail-closed 校验（Pattern 3，镜像 parseRoutingTable/ValidateAgainstKPI）+ 数字服务端注入（Pattern 2） |
| RPT-04 | report:publish 写工具走 HITL 审批门，产出静态页 | 第三写目标 append 点（registry.go 注释明示 add-point）+ handler 镜像 hitl_review_report.go（Pattern 4）+ 渲染沙箱（Pattern 1/2）+ 静态路由（Pattern 7） |
| RPT-05 | 带修改重生成——新一轮 DeepAgent run | chip 预填镜像 D17-C1；新 run 继承模板+period+修改意见（D18-D2）；版本 vN 不覆盖（Pattern 6） |
| FAN-05 | LLM 分类器结构化输出 {should_fan_out, num_subtasks, task_boundaries[]}，决策进审计 | fanout_tools.go args 扩展（Pattern 5）；审计骑 audit 表 action_fanout_* 枚举（F4）；fallback_reason 走已有 Reason 列 |
| FAN-06 | 冻结集断言 fan-out 率稳定性（per-case expected_fan_out） | eval/golden/fanout.jsonl 加法扩展两字段 + fanout.lock.json 元数据扩展（F8）；GLM 漂移率实测方法见 Open Questions |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| template: 段解析 + fail-closed 校验 | API/Backend（internal/skill loader） | — | 受控文件加载期执法是既有哲学（D16-A4/D18-A4）；热重载同路径 |
| DeepAgent 计划清单 + 计划确认门 | API/Backend（hitl plan_approve） | Browser（PlanApprovalCard） | HITL 门在服务端执法，UI 只渲染 |
| fan-out 分类器 + 边界校验 + fallback | API/Backend（agent 工具层） | — | D18-C2 主循环 tool-call 自决；边界校验是工具层结构锁 |
| 并行取数（fan-out 或顺序） | Database/Storage（DuckDB KPI 通道） | API/Backend（kpi.Executor） | 取数必须经 Executor（kpi_calls 审计 + k_min + DataRef 三路输出单一真源） |
| 叙事槽位填写（LLM 产文字） | API/Backend（DeepAgent run） | — | LLM 只产槽位文字，数字不经过 LLM |
| KPI 数字一致性判定 | API/Backend（internal/report 渲染前） | Eval（frozen set） | 渲染侧服务端执法 + eval 期冻结断言双层 |
| 渲染沙箱（绑定→goldmark→bluemonday） | API/Backend（新 internal/report） | — | Q3 进程内模型已由 D18-B1 落地；无子进程无前端渲染 |
| 版本 vN 递增 + reports 表登记 | Database/Storage（sqlite） + API/Backend | — | INSERT-only 审计表模式第 4 张镜像 |
| GET /reports/* 静态服务 + 遍历防护 | API/Backend（Hertz 路由） | CDN/Static（文件系统） | hertz 静态服务 + 白名单校验；v1 本地部署无独立 CDN |
| 发布审批门（第三态「要求修改」） | API/Backend（hitl + approval_api） | Browser（ApprovalCard） | 三行审计生命周期服务端执法；第三态按钮在 FE |
| 报告卡片 + chip 预填 | Browser（MessageBubble） | API/Backend（data-decision 通道） | D18-B3 复用通道；UI 只渲染 Skill 决定的 chip 语义 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/yuin/goldmark` | **v1.8.5**（2026-08-28 goproxy.cn 实测最新）[VERIFIED: goproxy.cn + 官方 README] | markdown → HTML 渲染（GFM 表格 + CJK 扩展） | CommonMark 0.31.2 合规；**默认不渲染 raw HTML/危险 URL**（不开启 `html.WithUnsafe()`）；零传递依赖；官方 Security 节点名 bluemonday 为配套 |
| `github.com/microcosm-cc/bluemonday` | **v1.0.27**（同日实测最新；≤v1.0.26 被 retract 必须 pin）[VERIFIED: goproxy.cn + 官方 README] | HTML 消毒（UGCPolicy 含 AllowTables） | allowlist 式消毒器事实标准；README 铁律 "always run bluemonday after any other processing" 确认管线顺序 |
| stdlib `text/template` | go 1.25.0 [VERIFIED: 本机] | 模板绑定（数据只作参数，绝不拼进模板源） | **text/template 不做转义**（html/template 才做）——转义职责在 goldmark+bluemonday，避免双重转义（STACK.md §e.3 决策） |

### Supporting（已在 go.mod，零新增）
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync/errgroup` | v0.21.0（indirect）[VERIFIED: go.mod] | fan-out 并行取数 SetLimit | D18-C1 per-章节并行；Phase 13 已用同模式 |
| `gopkg.in/yaml.v3`（既有） | — | template: 段解析 | loader 既有依赖 |
| `github.com/stretchr/testify` | v1.11.1 [VERIFIED: go.mod] | 测试断言 | 全仓测试惯例 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 自定义正则白名单静态 handler（推荐） | hertz `StaticFS` + app.FS | StaticFS 可用（v0.10.6 源码核实 `route/routergroup.go:210`，注册 `/*filepath` 通配），但 `GenerateIndexPages` 默认关闭之外无按目录级白名单能力；自定义 handler 20 行内实现「路径不匹配 `^[A-Za-z0-9-]+/v[0-9]+\.html$` 即 404」——遍历按构造被拒，直接满足 TestStaticRouteServesReports |
| `PathTraversalRestricted` 中间件（WebSearch 声称存在于 hertz） | — | **不在 hertz v0.10.6 core**（全模块 grep 零命中）[VERIFIED: 模块源码 grep]；属 hertz-contrib 生态（未入 go.mod）——不建议为单一目录约定引入新依赖 |
| html/template 做绑定 | text/template | html/template 会在模板层转义 + goldmark 再转义 = 双重转义破版（STACK.md §e.3 已定性，维持） |

**Installation:**
```bash
go get github.com/yuin/goldmark@v1.8.5 github.com/microcosm-cc/bluemonday@v1.0.27
```
（x/net 已在 v0.43.0 ≥ bluemonday 需要的 v0.26.0，**零依赖抬升** [VERIFIED: go.mod]）

**版本验证方法：** `go list -m -versions github.com/yuin/goldmark`（GOPROXY=goproxy.cn，2026-08-28 实测 v1.8.5 / v1.0.27 为最新——STACK-VERIFICATION 2026-08-21 结论未漂移）。

## Package Legitimacy Audit

> slopcheck 本机不可用（pip 环境未装且本次研究未安装）。按优雅降级协议，两个新包标 `[ASSUMED]` 交 planner 的 checkpoint 把关；**但需注明**：两者在 `.planning/research/STACK-VERIFICATION.md`（2026-08-21）中已经过对抗式审计——官方 README 核对 + 模块源码下载 grep（bluemonday policies.go:217 `AllowTables()` 逐行确认）+ 双代理路径验证，证据强度高于 slopcheck 的启发式检查。

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `github.com/yuin/goldmark` | Go modules | ~7 yrs（2019 起） | Go 生态 markdown 解析事实标准（Hugo 渲染引擎） | github.com/yuin/goldmark [VERIFIED: go-import 元数据] | [ASSUMED]（不可用；STACK-VERIFICATION 源码级审计通过） | Approved — planner 保留 human-verify checkpoint |
| `github.com/microcosm-cc/bluemonday` | Go modules | ~10 yrs（2016 起） | 3.7k stars；OWASP Java Sanitizer 移植 | github.com/microcosm-cc/bluemonday [VERIFIED: 官方 README] | [ASSUMED]（不可用；STACK-VERIFICATION 源码级审计通过） | Approved — planner 保留 human-verify checkpoint；**必须 pin v1.0.27**（≤v1.0.26 被 retract） |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none
**生态注册表核对：** `github.com/microcosmohq/bluemonday`（STACK.md 历史笔误路径）不存在——两代理 404 已在 STACK-VERIFICATION 实测；正确路径 `microcosm-cc` [VERIFIED: goproxy.cn]。

## Architecture Patterns

### System Architecture Diagram

```
用户:「出 2026-08 月报」(或点重生成 chip 预填)
        │
        ▼
POST /api/chat ──► chatHandler 找到 monthly-report Skill (data+deep+writes)
        │                [F1] validate.go: deep+data 禁令须解锁
        ▼
BuildRunner (runner_setup.go)
   ├─ [F2] data-channel 分支扩容: {kpi_call, report_publish, plan_approve,
   │        write_todos, fan_out_dispatch}
   └─ DispatchAgent ──► deep.New (MaxIteration=10, WithoutWriteTodos)
        │                [F3] deep 提示词需 data-channel 变体 (kpi_call 而非 retrieve)
        ▼
DeepAgent run
   1. write_todos ──► 计划清单 SSE 可见 (3-effect: audit→SSE→session)
   2. plan_approve_review ──► StatefulInterrupt ──► 计划确认门 (RPT-02)
   3. fan_out_dispatch ──► 分类器 tool-call (FAN-05)
   │      args {should_fan_out, num_subtasks, task_boundaries[]}
   │      ├─ 合法: per-章节 kpi.Executor.Call 并行 (errgroup SetLimit)
   │      └─ 不合法: fallback 顺序执行 + audit Reason=fallback_* (D18-C4)
   │      每条路径都产 kpi_calls INSERT 行 + Snapshot (D16-D3 三路单一真源)
   4. LLM 填叙事槽位 (narrative/insight/risk_note) — 只有文字, 数字来自快照
   5. report_publish tool-call ──► StatefulInterrupt ──► 发布审批门 (RPT-04)
   │      审批卡: 结构摘要 + 完整参数快照 (D18-B4) + [要求修改] 第三态 (D18-D4)
   ▼
批准 → report:publish handler (Pass 2 resume)
   ├─ internal/report 渲染沙箱:
   │    text/template 绑定 (LLM 槽位文字 + 服务端格式化 KPI 值)
   │      → goldmark.Convert (raw HTML 默认转义, 第 1 层)
   │      → bluemonday.UGCPolicy().SanitizeBytes (第 2 层)
   │    [TestRenderPipelineOrder: 顺序结构性锁, 不可换]
   ├─ reports/<period>/v<N>.html 写盘 (vN 递增, 不可变, D18-B2)
   ├─ sqlite reports INSERT-only 行 (审批审计行 ID + kpi_calls 引用 + 模板版本)
   └─ data-decision{decision:"report_published"} ──► FE 报告卡片 (D18-B3)
        标题+period+版本+链接(新tab)+[带修改重生成]chip (D18-D1)
        │
        ▼ (重生成) 新一轮 chat 消息 ──► 新 DeepAgent run (继承模板+period+修改意见)
                                        旧版文件永不覆盖 (RPT-05)
```

### Recommended Project Structure
```
internal/report/                 # 新包 — 渲染沙箱（无 agent 依赖, 纯函数为主）
├── template.go                  # template: 段 Go 类型（ReportTemplate/Section/Slot）+ fail-closed 校验
├── format.go                    # KPI 值服务端格式化 + markdown 表格转义（Pattern 2）
├── render.go                    # 绑定→goldmark→bluemonday 管线（顺序结构性锁在此）
├── publish.go                   # vN 递增 + 不可变写盘 + reports 行登记编排
└── *_test.go                    # 19 test locks 中的渲染面测试
internal/skill/
├── manifest.go                  # Manifest 加 Template *ReportTemplate 字段（yaml:"template"）
└── validate.go                  # [F1] deep+data 禁令解锁 + template 校验挂载点
internal/agent/
├── fanout_tools.go              # [扩展] 数据通道分类器 args 变体（Pattern 5）
└── hitl/
    ├── registry.go              # AllowedWriteTargets append "report:publish"（注释明示的 add-point）
    └── hitl_report_publish.go   # 新 handler — 镜像 hitl_review_report.go
internal/runtime/
└── deep_agent.go                # [F3] deepArchetypePrefix 按通道分支
internal/agent/runner_setup.go   # [F2] data-channel 工具集扩容
internal/store/sqlite/
├── schema.go                    # reports CREATE TABLE（additive, 镜像 kpi_calls 段）
└── reports.go                   # PutReport INSERT-only + ListReports（镜像 kpi_calls.go）
cmd/server/main.go               # GET /reports/* 路由装配（server 段 6c-3 KPI 段之后）
examples/skills/monthly-report/SKILL.md   # 新 Skill（template: 段 + reads/writes/steps）
eval/golden/fanout.jsonl         # 月报用例 append（expected_fan_out/expected_num_subtasks）
eval/frozen/fanout.lock.json     # 元数据扩展（月报 case 计数 + glm_drift_rate）
web/src/components/ApprovalCard.tsx        # [要求修改] 第三态按钮 + 结构摘要分支
web/src/components/chat/MessageBubble.tsx  # report_published 决策渲染分支 + chip
```

### Pattern 1: template: 段 YAML schema（Claude's Discretion — 建议）

**What:** SKILL.md frontmatter 的模板块。字段名与 KPI 定义 YAML（D16-A1）/ 路由表（D17-A1）同构：snake_case、程序可读、可静态校验。

**建议 schema：**
```yaml
template:
  title: "{period} 业务月报"            # 报告标题；{period} 占位符服务端替换
  sections:
    - title: 业务概览
      kpis: [KPI_LOAN_BALANCE, KPI_CUSTOMER_COUNT]
      slots:
        - type: narrative              # 叙事段 — LLM 写文字
        - type: insight                # 分析点评 — LLM 基于已取 KPI 值写
        - type: risk_note              # 风险提示 — LLM 写且必须引用具体 KPI 值
    - title: 贷款质量
      kpis: [KPI_NPL_RATIO]
      slots:
        - type: narrative
```

**校验规则（D18-A4 fail-closed，启动期 + 热重载期）：**
1. `sections` 非空且有序（章节顺序非空）
2. 每个 section：`title` 非空、`kpis` 每个 KPI 名在 registry（复用 `loader.go:164 ValidateAgainstKPI` 的交叉校验模式——Phase 17 已建该注入点）、`slots` 非空
3. 每个 slot：`type` ∈ {narrative, insight, risk_note}，未知类型**拒绝**（TestTemplateSlotTypesFixed）
4. 错误信息点名冲突字段（`section[1].kpis: unknown KPI "KPI_FOO"` 形态）

**落点：** `manifest.go` 加 `Template *ReportTemplate` 字段（`yaml:"template"`）；ReportTemplate 类型定义建议放 `internal/report`（渲染沙箱的类型真源）或 `internal/skill`（避免 skill→report 反向依赖；**注意** skill 包被 hitl 反向引用的先例（validate.go→hitl.KnownTargets），report 应为零依赖叶子包，skill→report 单向引用合法）。

**Skill 文件形态（monthly-report）：**
```yaml
description: 业务月报生成。触发短语：「出月报」「月报」「2026-08 月报」...
agent: deep
reads:
  - data:aggregated_only
writes:
  - report:publish
  - plan_approve
steps: |
  （月报生成流程引导 + 章节独立性说明 + fan-out 适用性提示——D18-C2
   「Skill steps 提示『本 Skill 章节独立，适合 fan-out』引导但不强制」）
template:
  （如上）
guardrails:
  prohibitions:
    - "Never write a number that is not server-injected from a KPI snapshot"
    - "Never SELECT raw rows or attempt SQL"
    - "Never overwrite an existing report version file"
  ...
temperature: 0.0
```

### Pattern 2: KPI 值服务端格式化 + markdown 表格注入转义（Pitfall 15 服务端执行面）

**What:** KPI 数字从 Snapshot（`internal/kpi/executor.go:72`）到表格单元格的转换全部在服务端完成；LLM 产出的文字中**任何数字 token 不进渲染管线的数值路径**（TestKPINumbersServerInjected 的结构语义：渲染函数的数值入参类型应为服务端快照类型，而非 string——由类型系统保证 LLM 文本不可能被当作数值绑定）。

**转义规则（Claude's Discretion — 建议）：**

| 层 | 规则 | 防什么 |
|----|------|--------|
| 数值格式化 | `千分位 + 2 位小数 + 单位`（`4,231.50 亿元`）；百分比 `12.34%`。**只输出 `[0-9.,%亿万元-]` 白名单字符** | 数值路径永不含语法字符（结构性） |
| markdown 语法字符转义（若 KPI 值或行名作为文本单元格） | `\` 转义 GFM 特殊字符集 `` \` *_{}[]()#+-.!| `` → `\|`、`\`` 等；或更简单：单元格内容走 goldmark 前置转义函数 | `1.23 \| </td>` 类值破表 / 注入标记 |
| 叙事文字（LLM 产） | 原样绑定（text/template 不转义）→ goldmark 把任何内嵌 `<script>` 当文本转义（默认无 `html.WithUnsafe()`）→ bluemonday 兜底剥离 | 槽位文字中的 HTML/危险标签（双层防御） |

**关键实现事实：** text/template 的绑定是**数据绑定非字符串拼接**——`tmpl.Execute(w, data)` 中 data 是 Go 值，模板源来自 Skill 文件加载期解析结果，LLM 输出永远不会成为模板源（TestNoTemplateCodeExecutionSurface 的结构基础：模板只从 `Manifest.Template` 数据流经渲染函数，无 `template.New(llmText)` 形态）[VERIFIED: stdlib text/template 语义 + STACK.md §e.3]。

### Pattern 3: deep+data 禁令解锁（本研究最重要的新发现 F1）

**What goes wrong if missed:** monthly-report Skill 一写出来就拒载——`internal/skill/validate.go:121-126` 现状：

```go
// (b) Deep archetype + data channel is structurally incompatible:
// deep's system prompt hardcodes a retrieve tool promise
// (internal/runtime/deep_agent.go:171-174), but data channel's
// tool set is {kpi_call} — prompt-inject amplification surface.
// v1 fail-closed; author must use "chat" archetype instead.
if m.Agent == "deep" {
    return fmt.Errorf("agent type %q is incompatible with data channel: ...", m.Agent)
}
```

**Why it exists:** 禁令的根因是 **F3 的提示词问题**（deep 前缀硬编码 "retrieve_compliance_chunks"），不是 deep 与 data 的本质不兼容。Phase 18 的修复是把「根因」修掉然后解除禁令，而非绕过守卫。

**解锁方案（三处联动，缺一即 half-run）：**
1. **validate.go** — 将 (b) 分支改为条件放行：data-channel + deep 仅当 `writes` 含 `report:publish`（或引入显式能力标记）且 `Template != nil` 时合法；否则维持拒载（fail-closed 收窄而非放开）。配套修改 `tools_for_channel_test.go` / `runner_setup_test.go` 的既有断言。
2. **deep_agent.go** — `deepArchetypePrefix()` 按通道分支：data 通道变体声明 `{kpi_call, fan_out_dispatch, report_publish, plan_approve, write_todos}`，删除 retrieve 承诺；tier 通道保持原文（既有测试 `TestRenderDeepSystemPromptPrependsFloor` 不破——平台地板包裹不动，D14-A3）。
3. **runner_setup.go** — 见 Pattern 5 前置的 F2 工具集扩容。

**Warning signs（漏改任一处的症状）：** 启动即拒载（漏 1）；LLM 幻觉调用 retrieve 报工具不存在（漏 2）；LLM 看不到 write_todos/审批工具、计划清单不可见（漏 3）。

### Pattern 4: report:publish 第三写目标 + 「要求修改」第三态审计

**注册 append 点（registry.go 注释明示的 add-point，[VERIFIED: internal/agent/hitl/registry.go:44-47]）：**
```go
var AllowedWriteTargets = []string{
    "memory:facts",
    "review_report",
    "plan_approve",
    "report:publish",  // 18-XX: fourth write target — 月报发布 HITL 门 (RPT-04)
    "skill_state",
}
var hitlHandlers = map[string]tool.InvokableTool{
    // ...
    "report:publish": &reportPublishTool{}, // hitl_report_publish.go
}
```

**handler 镜像 hitl_review_report.go 的双 pass 契约：** Pass 1 = `InsertPendingApproval`（fail-closed 先行）→ `tool.StatefulInterrupt`；Pass 2 = `GetResumeContext[*ApprovalDecision]` → deny/approve 分支。**Args 建议：** `{title, period, sections: [{title, kpis, slot_texts}], kpi_call_ids: [...]}` —— args 即完整参数快照（冻结进 pending 行的 frozen_args），审批卡 FE 直接从 frozen args 渲染结构摘要（D18-B4「批的 A 渲染成 A」由 HITL-07 既有 TestApprovalArgsByteEquality 机制保证）。**approve 分支的真实现**（review_report 是 stub，本 phase 是第一个有真实副作用的 publish）：调 `internal/report.Render + Publish` → 写 vN 文件 + reports 行 → 返回结构化成功串（含 url + version），chatHandler/回调据此 emit 卡片。

**「要求修改」第三态（D18-D4）审计生命周期：**
- **枚举：** `validDecisions`（`approval_decision.go:49`，现 approve/deny 两值）加第三值 `"request_change"`；audit.action 加法枚举 `action_change_requested`。**兼容性：** HITL-04 三行生命周期 = pending 行 + 1 条终态行；request_change 是**终态**（不是第四行），partial unique index（`WHERE action='action_pending_approval'`）不受影响，`TestAuditChainThreeRowLifecycle` 按 decision 分支断言不破。AuditWriter 加 `MarkChangeRequested`（镜像 MarkRejected 但 reason 允许语义为修改意见）。
- **运行时行为建议：** resume 注入 request_change 决策 → tool Pass 2 返回 `"change requested: <reason>"` 消息（与 deny 同型的优雅收尾）→ 当前 run 自然结束 → chatHandler 据终态 emit `data-decision{decision:"plan_change_requested"}`（加法枚举）→ FE 渲染修改卡 + 预填输入框 → 用户发送即新 run（D18-D1 路径）。**为何不复用 deny：** deny 审计语义 = 拒绝必填理由终局；request_change = 协作循环中间态，两者在审计对账（`action` 分布统计）和 UI 呈现上必须可区分（CONTEXT 拒绝项原文）。
- **适用面：** 发布审批门必带（D18-D4）；计划确认门是否同带第三态由 planner 按 TestRequestChangeThirdState 的「触发新一轮计划确认门」语义定（建议 v1 只做发布门，计划门的 deny→重写循环已由 PLAN-03 既有语义覆盖）。

### Pattern 5: fan_out_dispatch 数据通道变体 + fallback（FAN-05, D18-C1/C2/C4）

**现状（[VERIFIED: internal/agent/fanout_tools.go]）：** 现工具是 orchestrator archetype 专属（构造需 parentManifest，args = `sub_queries[]{id,query,tier_filter}`，tier 白名单校验）。月报场景 args 形态不同（章节边界非 tier 过滤）。

**建议扩展形态（同一工具名，构造参数区分通道；零新工具名避免 D11-B3 面扩容）：**
```go
type FanOutDispatchArgsV2 struct {
    ShouldFanOut   bool     `json:"should_fan_out"`
    NumSubtasks    int      `json:"num_subtasks"`
    TaskBoundaries []string `json:"task_boundaries"` // 每项 = 模板 section title（逐字匹配）
    Reason         string   `json:"reason"`          // 分类理由 — 进审计行（FAN-05 原文）
}
```
- **校验（结构性，TestFanOutBoundaryFromTemplate）：** `task_boundaries[i]` 必须 ∈ `Manifest.Template.sections[].title`（构造时闭包注入模板章节名集）；`num_subtasks == len(task_boundaries)`；越界/缺字段/JSON 坏 → 返回 error（工具层校验失败即触发 fallback 分支）。
- **执行模型建议：** 工具 InvokableRun 内部直接并行调 `kpi.Executor.Call`（errgroup `SetLimit(4)`，Phase 13 同模式）——**不启用 sub-agent**（`WithoutGeneralSubAgent: true` 维持，P13-L8 不破；data 通道无 tier 闭包需求，sub-agent 机制无必要）。每路调用走同一个 Executor → kpi_calls 审计行 + Snapshot + DataRef 三路输出天然完整（16-06 单一真源）。
- **fallback（D18-C4）：** 校验失败时工具返回**非 error 的结构化结果** `{status:"fallback_sequential", reason:"<枚举>"}`——LLM 收到后逐章节顺序发 kpi_call（主循环自然顺序执行）；audit 行（action_fanout_failed 或新 `action_fanout_fallback`）+ 既有 `Reason` 列记录枚举值。**fallback_reason 建议枚举：** `invalid_json` / `missing_field` / `boundary_not_in_template` / `num_subtasks_mismatch`。**为何返回结构化结果而非 Go error：** Go error 会让 eino 渲染工具噪声、LLM 无法区分「该回退」与「系统坏了」（kpi.Refusal 的既有设计哲学，executor.go:86-89 IRON RULE 同理）。
- **审计（F4 已核实）：** fanout 事件骑 `audit` 表 `action_fanout_*` 枚举 + run_path/transfer 列（**不是独立表**；`fanout_events.go` 是 audit 行的查询投影，`Reason` 字段映射 audit 的 reason/refusal_reason 列）[VERIFIED: schema.go:76-77 + fanout_events.go:43-45]。分类决策 = `action_fanout_dispatched` 行，args JSON 进 frozen_args，Reason 进分类理由。

### Pattern 6: reports 表 additive 迁移（D18-B2）

**镜像对象：** `internal/store/sqlite/kpi_calls.go`（import_batches 整文件复刻换列集的 D16-03-A 先例）。**建议 DDL（Claude's Discretion）：**
```sql
CREATE TABLE IF NOT EXISTS reports (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    ts                 TEXT NOT NULL,              -- RFC3339Nano 发布时刻
    period             TEXT NOT NULL,              -- "2026-08"
    skill_id           TEXT NOT NULL,
    version            INTEGER NOT NULL,           -- vN 递增, 同 period 内唯一
    title              TEXT NOT NULL,
    file_path          TEXT NOT NULL,              -- reports/2026-08/v1.html
    approval_audit_id  TEXT NOT NULL,              -- 审批审计行 ID (D11-C4 tool_call_id)
    kpi_call_ids       TEXT NOT NULL,              -- JSON 数组: 关联 kpi_calls.id 列表
    template_version   TEXT NOT NULL,              -- 模板内容 sha256 前 16 位（快照用, 而非行号）
    result_digest      TEXT NOT NULL DEFAULT ''    -- HTML sha256（防篡改 + Phase 19 diff 入口）
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_period_version ON reports(period, version);
CREATE INDEX IF NOT EXISTS idx_reports_period ON reports(period);
```
- 写路径 `PutReport` 唯一；零行变异/删除动词（`TestReportsInsertOnly` source-grep 锁，镜像 `TestKPICallsInsertOnly`）。
- **vN 递增语义（TestVersionIncrementNoOverwrite）：** `version = SELECT COALESCE(MAX(version),0)+1 WHERE period=?`；文件写入用 `os.WriteFile` + 写前存在性检查（存在即拒绝——发布即不可变）；测试断言 v1 写后字节哈希在 v2 发布后不变。
- **template_version 建议内容哈希** 而非版本号：Skill 无版本字段（audit_writer.go SkillSnapshot DEVIATION 注释实证），prompt body 哈希是唯一「内容变了它就变」的稳定源（`sha256:<16 hex>` 形态沿用 NewSkillSnapshot 惯例）。

### Pattern 7: GET /reports/* 静态路由 + 遍历防护（D18-B1 + Claude's Discretion）

**推荐（主方案）：自定义 handler + 正则白名单：**
```go
// hertz v0.10.6: 路由通配形态（StaticFS 内部即用此形态, routergroup.go:215）
server.GET("/reports/*filepath", func(ctx context.Context, c *app.RequestContext) {
    p := string(c.Path())                       // 形如 /reports/2026-08/v1.html
    rel := strings.TrimPrefix(p, "/reports/")
    if !reportPathAllowlist.MatchString(rel) {  // ^[A-Za-z0-9_-]+/v[0-9]+\.html$
        c.SetStatusCode(404); return             // 遍历（../、%2e%2e、绝对路径）按构造被拒
    }
    b, err := os.ReadFile(filepath.Join("reports", filepath.FromSlash(rel)))
    // ... 200 + Content-Type: text/html; charset=utf-8 + Cache-Control
})
```
- 白名单正则使 `..`、`/` 层级越界、空节、符号链接等全部落 404——「目录遍历被拒」无需依赖框架行为（TestStaticRouteServesReports 直测）。
- **v1 访问控制建议：** 本地部署不做鉴权（与聊天端点同级开放面）；报告内容本身是已脱敏聚合数据（DATA-02 前提）。若 planner 想加门，复用 `X-Admin-Token` 常量时间比较模式（/api/admin 先例）即可，但会破坏「链接可分享」的 D18-B1 意图——**建议不加**。
- **备选：** `server.StaticFS("/reports", &app.FS{Root: "./reports"})`（v0.10.6 源码核实签名；注意 `StaticFS` 对含 `:`/`*` 的 pattern 直接 panic——传 `/reports` 而非 `/reports/*`）。缺点：目录级白名单能力弱、无法做 reports 行存在性联动。**不推荐为主方案。**
- **WebSearch 声称的 `PathTraversalRestricted` 中间件不在 hertz v0.10.6 core**（全模块 grep 零命中）[VERIFIED: 模块源码]——勿按搜索结果照抄 import。

### Pattern 8: 报告卡片通道（D18-B3 + D17-B2 哲学）

- **通道：** `data-decision` chunk，`DecisionPayload` 加法枚举 `report_published`（chunk.go:157 Decision 注释明示 "extending the catalog is ADDITIVE — never repurpose"；P13-L7 fanout_* 先例）。不扩 ValidTypes union（wire 编译期报警保持绿）。
- **payload 建议：** `DecisionPayload` 加可选字段 `ReportCard *ReportCard`（json.RawMessage Data 已有，但类型化字段走 D16-B3 DataRef 同款先例）：
```go
type ReportCard struct {
    Title   string `json:"title"`
    Period  string `json:"period"`
    Version int    `json:"version"`
    URL     string `json:"url"`              // /reports/2026-08/v2.html
    Versions []string `json:"versions,omitempty"` // 历史版本链接列表 (D18-D3)
    RegenerateHint string `json:"regenerateHint,omitempty"` // chip 预填文案
}
```
- **FE：** MessageBubble `data-decision` 分支加 `report_published` 渲染（卡片 + 「查看报告」`<a target="_blank" rel="noopener noreferrer">` + 「带修改重生成」chip → onClick 预填 ChatInput——D17-C1 chip 同构）。TS 侧 `wire-types.ts` 同步镜像（D15 双 PR 规则）。

### Anti-Patterns to Avoid
- **绕过守卫（注释掉 validate.go 禁令）：** 禁令的三个组成部分是同一根因的三张面孔——只删检查不修提示词/工具集 = prompt-inject 放大面重新打开（守卫作者注释原文）。正确顺序：先备齐 data-channel deep 提示词 + 工具集，再收窄条件放行。
- **LLM 数字进渲染管线：** 任何「LLM 输出数字字符串 → 直接绑表格」的实现都击穿 TestKPINumbersServerInjected 的类型结构锁。数值入参类型必须是服务端快照类型。
- **管线倒序（bluemonday 在 goldmark 前）：** markdown 语法字符转义后消毒会把转义实体再破坏，且 goldmark 渲染引入的新结构未经消毒。README 铁律 + TestRenderPipelineOrder 结构锁双保险。
- **审批后重新渲染：** 批准的对象必须是 frozen args（渲染在批准后一次执行、产物不可变）；若 approve 后因模板热重载触发二次渲染，批的 A 可能渲染成 B。渲染输入 = frozen args + 发布时点 registry 快照。
- **reports/ 目录当临时区：** `reports/` 已在 .gitignore（Phase 9 spike 产物目录，现有 fanout_results.json 等陈旧文件）[VERIFIED: .gitignore:24-25 + 目录实测]。发布物落同一根目录有被清理习惯误删的风险——建议 period 子目录隔离（`reports/<period>/`）并在 SUMMARY 记录该共存事实，或 planner 决策迁移 spike 产物。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| markdown→HTML | 自写正则/状态机转换器 | goldmark v1.8.5（GFM+CJK 扩展） | CommonMark 0.31.2 全合规需 AST 级实现；GFM 表格/CJK 断行边界 case 以百计（官方 fuzz 测试背书） |
| HTML 消毒 | 自写 tag/attr 过滤 | bluemonday v1.0.27 UGCPolicy | XSS 向量库级深度（href scheme、CSS、嵌套绕过）；UGCPolicy 已含 AllowTables/AllowImages 全套 |
| 模板绑定 | 字符串 replace / fmt.Sprintf 拼接 | text/template 数据绑定 | 字符串拼接 = 模板注入直通车（模板源混入数据即 RCE 面）；数据绑定从结构上隔离模板源与数据 |
| HITL 中断/恢复/审计 | 新写一套审批流 | hitl 双 pass 契约 + AuditWriter 三行生命周期 | HITL-01..08 已过 11/11 验收门；幂等/冻结快照/expire 检查都是已锁契约 |
| 版本管理 | 时间戳/覆盖式文件名 | vN 递增 + unique index + INSERT-only 行 | RPT-05 原文 + Phase 19 门户排序依赖 |
| 并行取数 | 裸 goroutine + WaitGroup | errgroup.SetLimit(4) | Phase 13 已用；故障传播 + 并发上限语义现成 |
| 冻结集断言 | 运行时 if 检查 | golden jsonl + lock byte-identity 模式 | TestShippedFrozenLockIsInSync（internal/eval/frozen_test.go:229）已建 pattern |
| 目录遍历防护 | 信任框架默认 | 正则白名单（按构造拒绝） | hertz core 无白名单中间件（F 上文）；白名单比黑名单防御少一类绕过 |

**Key insight:** 本 phase 的「新轮子诱惑」集中在渲染与审批两端，而这两端恰恰是项目护城河（Pitfall 15 模板注入 + HITL-04 审计链）所在——所有手写替代方案都会同时损失安全性测试锚点与既有验收门覆盖。

## Common Pitfalls

### Pitfall 1: deep+data 禁令解锁不完整（三守卫只改一两个）
**What goes wrong:** Skill 拒载（漏 validate.go）/ LLM 幻觉 retrieve（漏 deep prompt）/ 计划清单与审批工具不可见（漏 runner_setup）。
**Why it happens:** 三处分散在三包，且 validate.go 禁令注释把根因归因于提示词——只读注释不读 prompt 实现会漏改 F3。
**How to avoid:** 一个 plan 内三个文件联动改 + `tools_for_channel_test.go` 断言同步更新 + 新增 TestDeepDataChannelToolsAssembly。
**Warning signs:** `go test ./internal/skill/ ./internal/agent/ ./internal/runtime/` 任一红。

### Pitfall 2: 数据通道工具集扩容破 D16-C3 既有断言
**What goes wrong:** runner_setup.go data 分支加工具后，Phase 16/17 的 TestToolsForChannel（断言 data Skill 工具集恰为 {kpi_call}）变红。
**Why it happens:** 「恰为 {kpi_call}」是当时 fail-closed 的正确断言，Phase 18 使其从「恰为」变「恰为 {kpi_call} ∪ writes 声明集 ∪ deep 配套」。
**How to avoid:** 断言按 Skill writes 声明参数化重写，保留「未声明 writes 的 data Skill 仍是 {kpi_call}」的负向 case（fail-closed 方向不放松）。
**Warning signs:** 16/17 回归红。

### Pitfall 3: 叙事数字一致性判定过严导致月报必失败
**What goes wrong:** LLM 叙事写「余额 4,231 亿」而快照是 423150000000，三步判定若只对 expected_value 严格比对，正常叙事也 FAIL。
**Why it happens:** D17-D2 三步判定的原始场景是 eval 断言（可预置 allowed_expressions），月报是运行时无预置白名单。
**How to avoid:** 运行时复用第 1 步（去格式化 + ±0.5% 容差）为主；allowed_expressions 由 Skill 作者在 template 段可选预置（如「近万亿/千亿级」模糊表达映射）；不一致时**标记不失败**（报告正常出，卡片/页脚带 `数字一致性未通过` 徽标）或失败——二选一进 discuss-phase 澄清或 planner 定（见 Open Questions）。
**Warning signs:** 真实 GLM 冒烟时 narrative 槽位大面积 FAIL。

### Pitfall 4: goldmark 表格扩展未启用 / CJK 断行
**What goes wrong:** KPI 表格渲染成纯文本段落；中文段落换行处出现多余空格。
**Why it happens:** 默认 `goldmark.Convert` 只开 CommonMark——GFM 表格需 `extension.GFM` 显式传入；CJK 需 `extension.CJK`。
**How to avoid:** 构造时 `goldmark.New(goldmark.WithExtensions(extension.GFM, extension.CJK), ...)`；**绝不传 `html.WithUnsafe()`**。
**Warning signs:** 渲染产物无 `<table>` 元素。

### Pitfall 5: bluemonday policy 每请求重建 / 跨 goroutine 构建
**What goes wrong:** 数据竞争或性能浪费。
**Why it happens:** 官方 README：policy 构建**非**goroutine 安全，构建后 Sanitize 并发安全。
**How to avoid:** 包级 `sync.OnceValue(UGCPolicy 构建+AllowTables)` 或构造期建好存字段；Sanitize 调用方并发无锁。
**Warning signs:** `-race` 红或渲染 p95 异常。

### Pitfall 6: FAN-06 冻结集变成 flaky 测试
**What goes wrong:** TestFanOutFrozenSetAssertion 在 CI 反复红/绿。
**Why it happens:** 分类器是真 LLM——把 live 分类结果写进 hermetic 测试就是 flakiness 直通车（Pitfall 19 eval 审计漂移的测试面变体）。
**How to avoid:** 双层分离：hermetic 层断言 golden jsonl **schema**（每 case 有 expected_fan_out/expected_num_subtasks + 章节名 ∈ 模板）+ lock byte-identity；live 层（operator gate）跑 N 次分类重跑一致率 → 写 lock 元数据。CI 永不真调 LLM。
**Warning signs:** 测试依赖 GLM_API_KEY 才绿。

### Pitfall 7: 「要求修改」复用 deny 通道
**What goes wrong:** 审计行 `action_rejected` + reason=修改意见——CONTEXT D18-D4 明确拒绝（语义混杂，无法对账「拒了多少 vs 改了多少」）。
**How to avoid:** 独立枚举 `action_change_requested` + validDecisions 第三值（Pattern 4）。
**Warning signs:** code review 发现 decision 枚举仍是两值。

### Pitfall 8: 内网 GLM 探测纪律失守
**What goes wrong:** live 分类器漂移实测跑一半断线，部分结果污染 lock 元数据。
**Why it happens:** Milvus/BGE/GLM 内网间歇断（MEMORY: env-intranet-service-flakiness）。
**How to avoid:** 先 curl 探测端点再起测；断线即记环境 STOP 不刷分；N 次重跑逐次落盘（单次完成即可入账，允许分多次会话凑 N）。
**Warning signs:** 连续 2 次 endpoint 超时。

## Code Examples

### 渲染沙箱管线（Pattern 1/2 的完整实现骨架）
```go
// Source: goldmark/bluemonday 官方 README (2026-08-28 抓取) + STACK-VERIFICATION §2.3/2.4
package report

var md = goldmark.New(
    goldmark.WithExtensions(extension.GFM, extension.CJK), // GFM 表格 + 中文断行
    // 刻意不传 html.WithUnsafe() — raw HTML 默认转义是第一层防御
)

var policy = sync.OnceValue(func() *bluemonday.Policy {
    p := bluemonday.UGCPolicy()      // 含 AllowTables (policies.go:217 源码核实)
    p.AllowElements("section")       // 如模板外壳需要, 逐项显式追加
    return p
})

// Render 的数值入参是快照类型 — LLM 文本在类型上不可能成为数值来源
func Render(tmpl *ReportTemplate, slots SlotTexts, snap *kpi.Snapshot, fmtFn ValueFormatter) ([]byte, error) {
    body := buildMarkdown(tmpl, slots, snap, fmtFn) // 1. 绑定: 数据绑定, LLM 文字原样, KPI 值已服务端格式化+转义
    var buf bytes.Buffer
    if err := md.Convert([]byte(body), &buf); err != nil { // 2. goldmark: raw HTML 转义
        return nil, err
    }
    return policy().SanitizeBytes(buf.Bytes()), nil // 3. bluemonday: 永远最后
}
```

### markdown 单元格转义（表格注入防护）
```go
// Source: GFM spec 6.2 + STACK-VERIFICATION §3.3
var mdCellEscaper = strings.NewReplacer(
    "|", `\|`, "`", "\\`", "*", `\*`, "_", `\_`, "\n", " ",
    "<", `\<`, "[", `\[`, "]", `\]`,
)
// 规则: 数值先过白名单格式化(仅 [0-9.,%亿万元-]); 任何非格式化文本进单元格必过 escaper
```

### report:publish 工具双 pass 骨架（镜像 review_report，仅差异点）
```go
// Source: internal/agent/hitl/hitl_review_report.go (HEAD) — Pass 1/Pass 2 契约逐行同构
// 差异 1: Args 含结构摘要字段 (D18-B4)
type ReportPublishArgs struct {
    Title       string             `json:"title"`
    Period      string             `json:"period"`
    Sections    []SectionDraft     `json:"sections"`     // 章节+KPI 名+槽位文字
    KPICallIDs  []int64            `json:"kpi_call_ids"` // 审计链关联
}
// 差异 2: Pass 2 approve 分支执行真实副作用
//   → report.Render(frozenArgs 解析) → report.Publish (vN + reports 行)
// 差异 3: Pass 2 增加 request_change 终态分支
if data.Decision == "request_change" {
    return fmt.Sprintf("change requested: %s", data.Reason), nil // 优雅收尾, 新 run 由 chip 驱动
}
```

### 冻结集 case 格式（fanout.jsonl 加法扩展）
```json
{"id":"rpt-fanout-01","scenario":"monthly_report","parent_skill":"monthly-report",
 "query":"出 2026-08 月报","template_sections":["业务概览","贷款质量","风险提示","客户情况"],
 "expected_fan_out":true,"expected_num_subtasks":4,
 "expected_fallback":false,
 "notes":"4 章节 12 KPI — 章节独立, 分类器应 fan-out 且边界=模板章节逐字匹配"}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| STACK.md 设想 x/net v0.21→v0.26 抬升（bluemonday 前置） | x/net 已 v0.43.0 | Phase 8-14 期间多次升级 | bluemonday 引入零依赖摩擦 [VERIFIED: go.mod] |
| STATIC 渲染进程模型未决（ARCHITECTURE Q3） | D18-B1 进程内 handler 落地 | 2026-08-28 CONTEXT | 零子进程/零 IPC；渲染在审批后一次执行 |
| fanout 事件独立表假设 | 实为 audit 表 action_fanout_* 枚举投影 | Phase 13 实际实现 | FAN-05 审计零新表零新迁移 |
| hertz PathTraversalRestricted 中间件（WebSearch 声称） | v0.10.6 core 不存在；白名单 handler 替代 | 本研究实测 | 不引入 hertz-contrib 新依赖 |

**Deprecated/outdated:** 无。goldmark/bluemonday/text/template 三选型全部现行且活跃。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | 「要求修改」运行时语义 = 优雅收尾当前 run + chip 预填新 run（而非挂起等待） | Pattern 4 | 若 planner 选挂起语义，resume/expire 检查面要重设计；审计枚举不变 |
| A2 | v1 计划确认门不加第三态（仅发布门带） | Pattern 4 | 若计划门也要第三态，plan_api.go 需同步扩 request_change 分支（小增量） |
| A3 | 叙事数字不一致运行时行为 = 标记不失败（徽标）为建议默认 | Pitfall 3 / Open Questions | 若用户要硬失败，月报可用性下降；需要用户澄清 |
| A4 | fan-out 执行 = 工具内并行调 Executor（无 sub-agent） | Pattern 5 | 若坚持真 sub-agent，需动 WithoutGeneralSubAgent + sub_agent_builder 数据通道适配（显著增量，且 D18-C1 未要求） |
| A5 | GLM 漂移实测 N=5 次 | Open Questions | N 偏小则漂移率置信度低；lock 元数据口径需 operator 认可 |
| A6 | reports/ gitignore 共存可接受（period 子目录隔离） | Anti-patterns | 若 operator 有清理 habits，发布物可能被误删；备选 = 迁移 spike 产物或改名目录（D18-B1 名字已锁） |
| A7 | `PathTraversalRestricted` 属 hertz-contrib（非 core） | Pattern 7 | 若 planner 找到 core 内等价物可替换白名单方案，行为等价即可 |
| A8 | request_change 终态不破 partial unique index / 三行生命周期测试 | Pattern 4 | 若 TestAuditChainThreeRowLifecycle 断言了 action 全集闭包，需同步扩展断言（加法） |

## Open Questions (RESOLVED)

1. **叙事数字不一致的运行时处置（硬失败 vs 标记放行）**
   - What we know: D18-A3 说「不一致 → 渲染失败/标记」（CONTEXT test lock 原文两者并列）；TestNarrativeNumberConsistency 措辞同。
   - What's unclear: 默认行为选哪个；risk_note 的「必须引用具体 KPI 值」校验强度（缺引用 = 拒载期不可查，只能运行期标记）。
   - Recommendation: 标记放行为默认（月报可用性优先），render 产物带未通过徽标 + 审计行记录；硬失败作为 Skill 模板可配开关。planner 定或 discuss 补一轮。
2. **月报 fan-out 的审计粒度**
   - What we know: 分类决策进 audit 行（FAN-05 原文）；kpi_calls 每调用一行既有。
   - What's unclear: `action_fanout_dispatched` 行的 frozen_args 是否足够（含 args+reason），还是再加 `action_fanout_fallback` 新枚举。
   - Recommendation: 加 `action_fanout_fallback` 枚举（对账查询直接 WHERE action 匹配，无需解析 reason 字符串）；Reason 列存 fallback_reason 枚举值。
3. **GLM 漂移率 N 与口径**
   - What we know: N 次重跑一致率；内网间歇断；13-05 live UAT 花费 $2.92/20 case 先例。
   - What's unclear: N=5 还是 N=10；一致率判定对象（仅 should_fan_out？还是含 num_subtasks 全等）。
   - Recommendation: N=5、判定三元组全等（should_fan_out + num_subtasks + task_boundaries 集合相等）、一致率进 lock 元数据 `glm_drift_rate`；operator gate 执行。
4. **reports/ 与 Phase 9 spike 产物的目录共存**
   - What we know: reports/ 已 gitignore 且有陈旧 spike 文件。
   - What's unclear: 是否清理/迁移。
   - Recommendation: 不动（gitignore 对发布物无害——sqlite 是真源）；SUMMARY 记录。operator 若清理自行决定。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | 全部 | ✓ | 1.25.0 windows/amd64 [VERIFIED: 本机] | — |
| MSYS2 gcc | cmd/server CGO 构建（goldmark/bluemonday 纯 Go 不需要；既有 CGO 链路需要） | ✓ | 16.1.0 (Rev5) [VERIFIED: 本机] | WSL2（16-09 先例） |
| goproxy.cn 网络 | go get goldmark/bluemonday | ✓ | 2026-08-28 实测 [VERIFIED] | proxy.golang.org |
| GLM 端点 | FAN-05 分类器 live 漂移实测 | ⚠ 间歇断 | glm-5.2 | 先探测；断线记环境 STOP（不阻塞 hermetic 面） |
| BGE-M3 / Milvus | 本 phase 不需要（月报无 tier 检索） | — | — | — |
| vitest | FE 测试（审批卡/消息卡） | ✓ | ^2.0.0 [VERIFIED: web/package.json] | — |
| sqlite (mattn CGO) | reports 表 | ✓ | v1.14.50 [VERIFIED: go.mod] | — |

**Missing dependencies with no fallback:** none
**Missing dependencies with fallback:** GLM live 面依赖内网稳定性——所有 live 步骤设计为 operator gate + 逐次落盘（Pitfall 8）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing + testify v1.11.1（BE）；vitest ^2.0.0（FE） |
| Config file | 无独立配置（go test 惯例）；web/vitest 配置既有 |
| Quick run command | `go test ./internal/report/ ./internal/skill/ ./internal/agent/... -count=1` |
| Full suite command | `go test ./... -count=1` + `cd web && npx vitest run` |

### Phase Requirements → Test Map（CONTEXT 19 条 test lock 全覆盖）

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TestMonthlyReportTemplateLoadFailClosed | 模板校验失败→拒载+点名字段 | unit | `go test ./internal/skill/ -run TestMonthlyReportTemplateLoadFailClosed` | ❌ Wave 0（新 loader_template_test.go） |
| TestTemplateSlotTypesFixed | 槽位类型闭包 + 自定义类型拒载 | unit | `go test ./internal/skill/ -run TestTemplateSlotTypesFixed` | ❌ Wave 0 |
| TestKPINumbersServerInjected | 数值路径类型结构锁（LLM 文本不可能成数值源） | unit（类型+结构断言） | `go test ./internal/report/ -run TestKPINumbersServerInjected` | ❌ Wave 0 |
| TestNarrativeNumberConsistency | 叙事数字三步判定 | unit | `go test ./internal/report/ -run TestNarrativeNumberConsistency` | ❌ Wave 0 |
| TestRenderPipelineOrder | 绑定→goldmark→bluemonday 顺序结构锁 | unit（source-grep + API 顺序断言） | `go test ./internal/report/ -run TestRenderPipelineOrder` | ❌ Wave 0 |
| TestMarkdownTableInjectionBlocked | 单元格转义 | unit（表驱动：`\|` `</td>` 等注入样本） | `go test ./internal/report/ -run TestMarkdownTableInjectionBlocked` | ❌ Wave 0 |
| TestReportPublishHitlGate | 未审批不执行 + 三行审计 + 冻结快照 | unit（镜像 hitl_review_report_test.go 面） | `go test ./internal/agent/hitl/ -run TestReportPublishHitlGate` | ❌ Wave 0 |
| TestApprovalCardShowsStructure | 审批卡 payload 含章节+KPI 名+模板版本 | FE vitest | `cd web && npx vitest run -t ApprovalCardShowsStructure`（或 ApprovalCard.test.tsx 内新 case） | ❌ Wave 0 |
| TestRequestChangeThirdState | 独立审计枚举 + 触发新一轮确认门 | unit（hitl + server） | `go test ./internal/agent/hitl/ ./internal/server/ -run TestRequestChangeThirdState` | ❌ Wave 0 |
| TestVersionIncrementNoOverwrite | v1→v2 共存 + v1 字节不变 | unit | `go test ./internal/report/ -run TestVersionIncrementNoOverwrite` | ❌ Wave 0 |
| TestReportsTableAuditChain | reports 行审批 ID + kpi_calls 引用可回溯 | unit（store 层） | `go test ./internal/store/sqlite/ -run TestReportsTableAuditChain` | ❌ Wave 0（reports.go + test） |
| TestFanOutBoundaryFromTemplate | task_boundaries ∈ 模板章节 | unit | `go test ./internal/agent/ -run TestFanOutBoundaryFromTemplate` | ❌ Wave 0 |
| TestFanOutClassifierStructuredOutput | 结构化输出 + 决策+理由进审计 | unit | `go test ./internal/agent/ -run TestFanOutClassifierStructuredOutput` | ❌ Wave 0 |
| TestFanOutFallbackSequential | 校验失败→顺序执行 + fallback_reason | unit | `go test ./internal/agent/ -run TestFanOutFallbackSequential` | ❌ Wave 0 |
| TestFanOutFrozenSetAssertion | golden schema + lock byte-identity（hermetic 层） | unit（镜像 frozen_test.go 模式） | `go test ./internal/eval/ -run TestFanOutFrozenSetAssertion`；live N 次重跑 = operator gate | ❌ Wave 0 |
| TestRegenerateNewRunNotPatch | 重生成=新 run 非原地补丁 | integration（hermetic spike 或 server 层） | `go test ./internal/server/ -run TestRegenerateNewRunNotPatch` | ❌ Wave 0 |
| TestReportCardChatMessage | report_published 卡片 + wire union 不扩 | unit（wire 加法断言 + FE vitest） | `go test ./internal/wire/ -run TestReportCard` + `cd web && npx vitest run -t ReportCard` | ❌ Wave 0 |
| TestStaticRouteServesReports | 正常 200 + 遍历拒 | unit（httptest 或 hertz test） | `go test ./internal/server/ -run TestStaticRouteServesReports` | ❌ Wave 0 |
| TestNoTemplateCodeExecutionSurface | 模板无代码执行面（WalkDir 结构锁：禁 `template.New(` 非常量源等） | structural gate（镜像 TestNoSQLGenerationSurface / TestNoLegacyInterruptAPI 模式） | `go test ./internal/report/ -run TestNoTemplateCodeExecutionSurface`（或 integration/contract） | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/report/ ./internal/skill/ ./internal/agent/... -count=1`（触及包级）
- **Per wave merge:** `go test ./... -count=1` + `cd web && npx vitest run`
- **Phase gate:** 全套绿 + `bash scripts/verify-phase-*.sh`（沿 13/14/15/16 惯例建 verify-phase-18.sh；stage 含 19 test locks 抽查 + frozen lock byte-identity）+ FE `npx tsc --noEmit`（D15 wire 镜像）

### Wave 0 Gaps
- [ ] `internal/report/` 包骨架（template.go/format.go/render.go/publish.go + 4 个渲染面测试文件）
- [ ] `internal/skill/loader_template_test.go` — 2 条模板校验锁
- [ ] `internal/agent/hitl/hitl_report_publish.go` + 对应测试 — 3 条 HITL 锁
- [ ] `internal/agent/fanout_tools` 数据通道变体测试文件 — 3 条 FAN 锁
- [ ] `internal/store/sqlite/reports.go` + reports_test.go — 2 条 store 锁
- [ ] `eval/golden/fanout.jsonl` 月报 case append + `internal/eval` schema 测试 — 1 条 FAN-06 锁
- [ ] wire 加法枚举测试 + `web/src` 镜像类型 — 2 条卡片锁
- [ ] Framework install: 无（全部既有）

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | v1 本地部署既有姿态（无新增认证面） |
| V3 Session Management | no | 无新会话语义（复用 chat session） |
| V4 Access Control | yes | HITL-05 writes: 代码级强制（BuildToolsForSkill 交集）；report 路由白名单 |
| V5 Input Validation | yes | template: fail-closed 校验（D18-A4）；fan_out_dispatch args 结构校验；ApprovalDecision.Validate |
| V6 Cryptography | no | 无新密码学面（result_digest 为完整性哈希非加密） |
| V14 HTML/XML Output | yes | **核心**：goldmark 默认转义 + bluemonday UGCPolicy 双层；管线顺序结构锁 |

### Known Threat Patterns for 渲染沙箱 + 写工具

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 模板注入（LLM/Skill 文本混入模板源） | Tampering/EoP | text/template 数据绑定（模板源=加载期解析产物，非运行期字符串）；TestNoTemplateCodeExecutionSurface 结构锁 |
| 存储型 XSS（KPI 值/叙事文字带 HTML） | Tampering | 双层：goldmark 默认转义 raw HTML（无 WithUnsafe）+ bluemonday UGCPolicy 终端消毒；管线顺序锁 |
| markdown 表格注入（破版/伪表格） | Tampering | KPI 值白名单格式化 + 单元格语法字符转义（Pattern 2） |
| 目录遍历（GET /reports/../../） | EoP/InfoDisclose | 正则白名单 `^[A-Za-z0-9_-]+/v[0-9]+\.html$` 按构造拒绝；404 不泄露存在性 |
| 写工具越权（Skill 调未声明写目标） | EoP | AllowedWriteTargets ∩ manifest.Writes 交集（D11-B1 既有）；report:publish 走同门 |
| 审批后渲染漂移（批 A 渲染 B） | Repudiation | frozen args 快照（D11-C3）+ 发布即不可变 + template_version 内容哈希 |
| 分类器输出越界（章节名伪造） | Tampering | task_boundaries ∈ 模板章节闭包（工具层构造校验，LLM 不可重绑） |
| 审计漂移（fan-out 决策不落审计） | Repudiation | action_fanout_* 枚举 + Reason 列 + frozen_args（P13-L6 既有）+ FAN-06 冻结断言 |

## Sources

### Primary (HIGH confidence)
- 本仓 HEAD 源码逐行核对（2026-08-28）：`internal/agent/hitl/{registry,hitl_review_report,hitl_plan_approve,audit_writer,approval_decision}.go`、`internal/agent/{fanout_tools,deep_audit,runner_setup,hitl_tools,deep_registry}.go`、`internal/runtime/{deep_agent,factory}.go`、`internal/skill/{validate,manifest,loader}.go`、`internal/kpi/executor.go`、`internal/store/sqlite/{kpi_calls,schema,fanout_events}.go`、`internal/wire/chunk.go`、`internal/eval/frozen_test.go`、`eval/frozen/fanout.lock.json`、`eval/golden/fanout.jsonl`、`examples/skills/kpi-dashboard/SKILL.md`
- goldmark 官方 README（github.com/yuin/goldmark，2026-08-28 抓取）— New/Convert API、GFM/CJK 扩展、默认 raw HTML 转义、Security 节
- bluemonday 官方 README（github.com/microcosm-cc/bluemonday，2026-08-28 抓取）— UGCPolicy、AllowTables、"always run after"、并发模型
- hertz v0.10.6 模块源码（本机 module cache）— `pkg/route/routergroup.go:206-220`（Static/StaticFS + 通配 panic 行为）、`pkg/app/fs.go:105-152, 800-830`（FS struct、/../ 拒绝、GenerateIndexPages 默认关）
- goproxy.cn 版本实测（2026-08-28）— goldmark v1.8.5 / bluemonday v1.0.27 最新
- `.planning/research/STACK-VERIFICATION.md` — 三选型对抗审计（2026-08-21）
- `.planning/research/STACK.md` §(e) — 渲染管线实现级研究（text/template vs html/template 决策、UGCPolicy 理由、静态文件模式）

### Secondary (MEDIUM confidence)
- WebSearch（hertz StaticFS/PathTraversal）— 已用本机源码 grep 复核并修正（PathTraversalRestricted 不在 core）
- `.planning/ROADMAP.md` §Phase 18 — Pitfall 15 定性（模板=数据不是代码；渲染沙箱管线）

### Tertiary (LOW confidence)
- 无。所有 load-bearing 断言均有 Primary 源。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 两新包官方 README + 代理实测 + 前期对抗审计三重印证
- Architecture: HIGH — 全部集成点定位到 file:line（含 3 处守卫解锁的准确行号）
- Pitfalls: HIGH — 均源自本仓既有测试/守卫的可观察约束，非推测

**Research date:** 2026-08-28
**Valid until:** 2026-09-27（30 天；goldmark/bluemonday 均低频发布，风险低）

---
*Phase: 18-skill-p10-4*
*Researched: 2026-08-28 via /gsd:plan-phase 18 (researcher)*
