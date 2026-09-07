# Phase 29: 反馈闭环 + 工单预留 (Feedback Loop + Ticket Interface) - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

把 `web/src/components/chat/parts/FeedbackRow.tsx` 从 toast 占位变成真实数据闭环：三按钮投票 INSERT-only 落库（一次一票幂等）+ outdated 报告进人工复核队列（/admin 复核视图）+ 反馈指标进 admin 仪表盘 + 「创建工单」调用真实 `POST /api/tickets`（TicketService 接口 + `none`/`webhook` provider 注册表）。工单外发尽力而为不阻塞主链路；反馈绝不自动改信任（FBK-05 红线）。

Requirements: FBK-01..05 + TKT-01..04（见 `.planning/REQUIREMENTS.md`）。真实工单系统对接（Jira/ServiceNow/自建）明确 Out of Scope——本里程碑只预留接口 + webhook provider。

</domain>

<decisions>
## Implementation Decisions

### 消息身份与投票幂等（FBK-01/04 地基）
- **D-29-01:** 消息身份 = **暴露既有 `session_turn.id`**——chat 流 + `GET /api/session/:id/history` 响应各增加一个 additive 字段把 turn id 发给 FE；投票时 FE 回传该 id 作为消息标识。零 schema 变更（`session_turn.id` 列已存在，Phase 24 PROV 导出已在用）。不用 turn ts（时间语义非身份、有碰撞风险）、不用 FE uuid（服务端无法验证，幂等最弱）。
- **D-29-02:** 一次一票幂等键 = `UNIQUE(session_id, message_turn_id)`；重复提交（含相同 verdict）幂等返回已记录状态。FE 现有「首次点击锁定」单选逻辑保留。
- **D-29-03:** 同会话回显已投状态 = **history 内嵌**——`historyEntry` 增加 `voted_verdict` 字段（该 turn 已投则返回 helpful/unhelpful/outdated，未投返回 null/缺省）；恢复会话一次请求同时拿到消息 + 已投状态，零额外请求。不用独立查询端点。
- **D-29-04:** 投票失败（网络断/5xx）时 FE **不锁定可重试**——toast 显示真实错误，按钮保持可点；重复提交风险由服务端幂等兜底。不做自动重试（服务端幂等已使重试无害，收益有限）。
- **D-29-05:** 投票行的被引 chunk ids 由**服务端**从该 turn 对应 audit 行（`audit.chunk_ids` CSV，chat 路径 `writeAuditAndTurn` 已写入）复制落库——服务端真相，不信任 FE 上报。

### 复核队列处置语义（FBK-02）
- **D-29-06:** outdated 投票附带**可选说明文本**——FE 选中「报告知识过时」后展开小输入框（可空、默认可直接提交，保持一键投票低摩擦）；文本随投票落库进复核队列条目。
- **D-29-07:** 「已更新」处置 = **仅状态标记**——管理员人工确认后的状态翻转 + INSERT-only 审计行，条目关闭。知识库变更本身仍走 Phase 27 上传流程，复核队列只管信号生命周期，不嵌入上传动作、不强制关联新文档。
- **D-29-08:** 待复核列表**按 chunk 聚合展示**——存储每票一行（INSERT-only 天然），视图一行 = 一个 chunk + 报告次数 + 可展开各条说明；一处处置关闭该 chunk 的全部待复核条目。报告次数本身是优先级信号。
- **D-29-09:** **三处置均终态**——已更新/误报/忽略都使条目离开待复核列表（处置历史可筛选查看）。误报 = 假阳性信号（chunk 没过时），忽略 = 不作判断；区分进审计，本期不进指标（误报率统计留给未来）。

### 仪表盘指标口径（FBK-03）
- **D-29-10:** 帮助率 = **helpful / (helpful + unhelpful + outdated) 全票分母**——outdated 本质是负面信号（用户没得到可靠答案），排除会虚高、破坏 per-Skill/per-知识库横向可比性；outdated 率单独展示。
- **D-29-11:** 趋势 = **日粒度可切周**——后端返回按日序列，周为切换视图；早期数据量小，日粒度才能看到每次 Skill/知识库变更后的反馈波动。
- **D-29-12:** 指标落位 = **AdminDashboard 新增第 6 个 tab「反馈」**——与现有 KPI tab（评测 KPI 定义管理，`KPIDefinitionList`）语义分离；与 eval 侧 EvalDashboard 分离（受众不同：评测门禁 vs 知识库运营）。

### 工单创建交互（TKT-02/03）
- **D-29-13:** 点「创建工单」后**展开式小表单**——描述输入框必填（预填回答摘要可改）+ 自动带上的会话/消息/chunk 引用展示；提交后展示受理结果。不做一击直建（自动描述对下游 IT 服务台没用处）、不做独立工单页（预留阶段过度设计）。
- **D-29-14:** 工单幂等 = **一消息一单**——`UNIQUE(session_id, message_turn_id)`，重复点击返回已建工单状态 + 既有编号；FE 回显「已建单」态。与投票同款幂等语义，防垃圾工单。

### Claude's Discretion
- webhook provider 细节：payload 格式、成功判定（如 2xx）、重试次数/退避/超时——配置驱动（TKT-01/04 纪律），planner 按既有 envOr/config 模式定。
- per-Skill / per-知识库 维度切换与时间范围筛选的具体 UI——按既有 admin 面板 raw fetch + localStorage token 模式定。
- `sf-toast` 监听器的全局挂载位置（当前 app 内无监听者，参照 `wizard-success` 在 `App.tsx` 的监听先例）。
- 复核视图/反馈 tab 内的具体组件拆分与状态管理。
- 工单受理编号格式与本地 id 生成方式（落库即有；webhook 成功后可附外部单号）。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope 与需求
- `.planning/ROADMAP.md` §Phase 29 — Goal / Success Criteria / Notes（起点代码与 Out of Scope 声明）
- `.planning/REQUIREMENTS.md` §反馈闭环 (FBK) + §工单系统预留 (TKT) — FBK-01..05 / TKT-01..04 逐条定义
- `.planning/phases/26-*/26-CONTEXT.md` 族 + ROADMAP §Phase 26 — 注册表/审计/admin-token 门纪律（本 phase 依赖面）

### 纪律与不变式
- `.planning/PROJECT.md` — INSERT-only 审计纪律、信任语义不削弱（KNOW-02 / D-09 / citation 白名单）、零硬编码配置驱动
- `.planning/phases/24-audit-chain-hardening/24-CONTEXT.md` — additive migration only（D14-E1）与 `session_turn.id` 在 PROV 导出中的既有用途

### 代码事实（scout 确认，plan 前无需重查）
- `web/src/components/chat/parts/FeedbackRow.tsx` — 起点代码：三按钮单选锁定逻辑已有，全部 action 为 `sf-toast` 占位
- `internal/store/sqlite/schema.go` — schemaSQL const + additive ALTER 循环 + migrate() 尾部索引惯例；无 feedback/tickets 表
- `internal/server/audit.go:114` `isAdminRequest` — admin 门（403 先于 503 的检查顺序见 `kb_admin.go:241`）
- `internal/server/kb_admin.go:330` `putKBAdminAuditRow` — INSERT-only 审计行写法范式
- `internal/server/handlers_session_history.go` — history 端点（`historyEntry{role, content, verdict, ts}`，本 phase 加 turn id + voted_verdict）
- `internal/server/chat.go:512` `writeAuditAndTurn` — chat 路径 audit.chunk_ids 写入点
- `internal/retriever/backend.go` `IndexerBackend` — 配置驱动 pluggable backend 的接口范式（TicketService provider 注册表参照）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session_turn.id`（autoincrement 列，已存在）——消息身份直接暴露，零 schema 变更
- `audit.chunk_ids`（CSV，chat 每轮已写入）——投票/工单的被引 chunk 来源（服务端复制）
- `FeedbackRow.tsx` 单选锁定骨架 + `data-testid` 套件——改造成真实 API 调用而非重写
- `AdminDashboard` tab 架构（zustand `adminSubroute`）——新「反馈」tab 与复核视图按此模式挂
- `KnowledgeBaseList` / `DocUpload` 的 raw fetch + `localStorage sf_admin_token` 模式——新 admin 面板照抄
- `historyEntry` / `chat-restore.ts` `SessionHistoryMessage`——history 内嵌回显的扩展点
- `wizard-success` dispatch→App.tsx listener 先例——`sf-toast` 真实监听器照此挂

### Established Patterns
- INSERT-only + per-table `TestXxxInsertOnly` 测试族（store 包内，如 `kpi_calls_test.go:152`）——feedback/tickets 表照此
- additive migration：`migrate()` 内 `columnExists` 门 + ALTER 循环 + 新表索引挂 migrate() 尾部（D14-E1）
- admin 门：handler 首行 inline `isAdminRequest`，403 先于 nil-dependency 503
- 可选依赖 nil → 503（`Deps` 字段 godoc 惯例）——TicketService 未配置 provider 时的行为落点
- envOr/parseIntEnvOrDefault（`cmd/server/main.go:156-181`）——新配置面（webhook URL、重试参数等）照此接线

### Integration Points
- `internal/server/server.go:442-628` `New()` 平铺路由块——`POST /api/feedback`、`POST /api/tickets`、admin 复核/指标端点在此注册
- `internal/server/handlers_session_history.go` historyResponse——voted_verdict 回显 + turn id 暴露
- `internal/store/store.go` Store interface（`store.go:583-827` 方法族）——新表方法挂接口 + sqlite 实现
- `cmd/server/main.go` Deps 装配——TicketService/provider 注册表 boot 绑定（参照 Phase 28 `SKILLFORGE_GRAPH_*` 模式）
- `MessageBubble.tsx:305` `<FeedbackRow messageId={message.id}>`——改传服务端 turn id

</code_context>

<specifics>
## Specific Ideas

- 指标口径是用户（金融 IT 数据支撑方向）明确关心的点：帮助率分母必须全票、横向可比性优先——不要在 plan 阶段改回二票分母。
- FE 锁定态一律由服务端驱动（history 回显 + 幂等响应），FE 本地状态只是缓存，不真相。
- 诚实展示原则：投票失败/工单未配置 provider/外发失败都如实告知用户，不静默、不假装成功。

</specifics>

<deferred>
## Deferred Ideas

- **工单列表的 admin 可见视图**（/admin 查看全部已落库工单及其外发状态）——本 phase scope 只有落库 + 审计 + 外发，admin 列表视图是独立能力，留待未来 phase。
- **误报率统计**（误报处置占比 → 用户反馈质量指标）——D-29-09 已保留误报/忽略区分的数据基础，指标本身留待未来。
- **「已更新」处置联动 Phase 27 上传的深度闭环**（预填目标知识库、上传成功自动关单）——D-29-07 选择仅状态标记，联动版是未来增强。

</deferred>

---

*Phase: 29-feedback-loop-ticket-interface*
*Context gathered: 2026-09-08*
