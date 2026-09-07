# Phase 29: 反馈闭环 + 工单预留 - Research

**Researched:** 2026-08-08（代码事实均来自本仓库 HEAD `6852e70` 的直接检查）
**Domain:** Hertz SSE wire 扩展（turn id 暴露）+ sqlite INSERT-only 反馈/工单表 + 幂等写端点 + admin 复核/指标读模型 + Go webhook provider（best-effort 外发）
**Confidence:** HIGH（本 phase 零新外部依赖，全部结论锚定在仓库现有代码先例上；少量语义选择已显式标记 [ASSUMED]/开放问题）

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions（D-29-01..14，plan 必须逐条遵守）

**消息身份与投票幂等（FBK-01/04 地基）**
- **D-29-01:** 消息身份 = **暴露既有 `session_turn.id`**——chat 流 + `GET /api/session/:id/history` 响应各增加一个 additive 字段把 turn id 发给 FE；投票时 FE 回传该 id 作为消息标识。零 schema 变更（`session_turn.id` 列已存在）。不用 turn ts、不用 FE uuid。
- **D-29-02:** 一次一票幂等键 = `UNIQUE(session_id, message_turn_id)`；重复提交（含相同 verdict）幂等返回已记录状态。FE 现有「首次点击锁定」单选逻辑保留。
- **D-29-03:** 同会话回显已投状态 = **history 内嵌**——`historyEntry` 增加 `voted_verdict` 字段；恢复会话一次请求同时拿到消息 + 已投状态，零额外请求。不用独立查询端点。
- **D-29-04:** 投票失败（网络断/5xx）时 FE **不锁定可重试**——toast 显示真实错误，按钮保持可点；重复提交风险由服务端幂等兜底。不做自动重试。
- **D-29-05:** 投票行的被引 chunk ids 由**服务端**从该 turn 对应 audit 行（`audit.chunk_ids` CSV）复制落库——服务端真相，不信任 FE 上报。

**复核队列处置语义（FBK-02）**
- **D-29-06:** outdated 投票附带**可选说明文本**——FE 选中「报告知识过时」后展开小输入框（可空、默认可直接提交）；文本随投票落库进复核队列条目。
- **D-29-07:** 「已更新」处置 = **仅状态标记**——管理员人工确认后的状态翻转 + INSERT-only 审计行，条目关闭。知识库变更本身仍走 Phase 27 上传流程，复核队列不嵌入上传动作。
- **D-29-08:** 待复核列表**按 chunk 聚合展示**——存储每票一行（INSERT-only），视图一行 = 一个 chunk + 报告次数 + 可展开各条说明；一处处置关闭该 chunk 的全部待复核条目。报告次数本身是优先级信号。
- **D-29-09:** **三处置均终态**——已更新/误报/忽略都使条目离开待复核列表（处置历史可筛选查看）。误报/忽略区分进审计，本期不进指标。

**仪表盘指标口径（FBK-03）**
- **D-29-10:** 帮助率 = **helpful / (helpful + unhelpful + outdated) 全票分母**——outdated 不排除；outdated 率单独展示。plan 阶段不得改回二票分母。
- **D-29-11:** 趋势 = **日粒度可切周**——后端返回按日序列，周为切换视图。
- **D-29-12:** 指标落位 = **AdminDashboard 新增第 6 个 tab「反馈」**——与 KPI tab、EvalDashboard 语义分离。

**工单创建交互（TKT-02/03）**
- **D-29-13:** 点「创建工单」后**展开式小表单**——描述输入框必填（预填回答摘要可改）+ 自动带上的会话/消息/chunk 引用展示；提交后展示受理结果。不做一击直建、不做独立工单页。
- **D-29-14:** 工单幂等 = **一消息一单**——`UNIQUE(session_id, message_turn_id)`，重复点击返回已建工单状态 + 既有编号。与投票同款幂等语义。

### Claude's Discretion
- webhook provider 细节：payload 格式、成功判定（如 2xx）、重试次数/退避/超时——配置驱动，按既有 envOr/config 模式定。
- per-Skill / per-知识库 维度切换与时间范围筛选的具体 UI——按既有 admin 面板 raw fetch + localStorage token 模式定。
- `sf-toast` 监听器的全局挂载位置（参照 `wizard-success` 在 `App.tsx` 的监听先例）。
- 复核视图/反馈 tab 内的具体组件拆分与状态管理。
- 工单受理编号格式与本地 id 生成方式（落库即有；webhook 成功后可附外部单号）。

### Deferred Ideas (OUT OF SCOPE)
- 工单列表的 admin 可见视图（/admin 查看全部已落库工单及外发状态）——未来 phase。
- 误报率统计（误报处置占比指标）——数据基础保留，指标留待未来。
- 「已更新」处置联动 Phase 27 上传的深度闭环——D-29-07 已选仅状态标记。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FBK-01 | `POST /api/feedback` 三按钮投票 INSERT-only 落库（session/message/verdict/被引 chunk ids），一次一票幂等 | §Pattern 1（turn id 暴露）+ §Pattern 3（feedback 表 DDL + ON CONFLICT DO NOTHING 幂等）+ §Pattern 4（服务端 chunk_ids 复制 = audit 邻近回溯） |
| FBK-02 | outdated 生成复核队列条目；/admin 复核视图按 chunk 聚合 + 三处置 + 审计 | §Pattern 5（派生 pending 视图 + `feedback_dispositions` INSERT-only 表 + admin 门顺序） |
| FBK-03 | per-Skill / per-知识库帮助率、过时报告数与趋势进 admin 仪表盘 | §Pattern 6（refs 归一化表 + 全票分母 GROUP BY + 日粒度 trend + 第 6 tab 挂载点） |
| FBK-04 | FE FeedbackRow 接真；锁定态服务端驱动；toast 反映真实结果 | §Pattern 2（`data-turn` part + MessageBubble 部件扫描先例）+ §Pattern 7（FE 状态机：提交中/已记录/已存在/失败不锁定） |
| FBK-05 | 反馈绝不自动改信任；知识库变更只能由复核处置产生 | §Pattern 5 结构红线 + §Security Domain（source-grep 结构锁：反馈/复核代码零 retriever/KB 变更动词） |
| TKT-01 | TicketService Go 接口 + 配置驱动 provider 注册表（none/webhook） | §Pattern 8（接口形态 + `NewProvider` 注册表 + env 接线，参照 `IndexerBackend`/BGE client 先例） |
| TKT-02 | `POST /api/tickets` INSERT-only 落库 + 外发结果进审计 | §Pattern 8（落库即响应 + 异步外发 + `action_ticket_dispatch_*` 审计行） |
| TKT-03 | FE 创建工单接真，诚实展示（受理编号/排队中/未配置 provider） | §Pattern 7（三态诚实文案由 `dispatch_status` 驱动） |
| TKT-04 | 工单外发不阻塞主链路（尽力而为 + 配置驱动重试） | §Pattern 8（goroutine + `context.Background()+WithTimeout` + RowsAffected/重试旋钮；hermetic 非阻塞测试设计） |
</phase_requirements>

## Summary

本 phase 是一个**纯代码库内接线**任务：零新外部依赖、零 schema 破坏性变更（两张新表 + 一张归一化 refs 表全部 additive），全部技术难点都已被仓库既有先例解决过一次。最关键的非显然发现有三个。**其一，turn id 的暴露路径**：`AppendTurn` 现签名只返回 `error`，`session_turn.id` 从未回流到 server 层——必须在同一 commit 内改签名返回 assistant 行 id（`LastInsertId`），并同步修 5+ 个 server 测试 fake（D16-03 先例：接口签名与实现分 commit 落地会撞碎 `var _ store.Store` 编译门）。SSE 侧 pipeline 在 `writeAuditAndTurn`（pipeline.go:528）**之后**、`finish`（:580）**之前**还有干净的发射窗口，新增 `data-turn` chunk type（ValidTypes + TS union 配对提交）即可让 live 流与 history 恢复两条路径共用同一个 FE part。**其二，FBK-05 红线的结构落点**：反馈/处置代码与 retriever/KB 之间没有任何必要的写路径，用 source-grep 结构锁（`TestAdminKPIs/retrieval_only` 先例）把「反馈绝不自动改信任」从纪律变成编译/门级事实。**其三，指标与复核队列的归一化**：`audit.chunk_ids` 是 CSV，sqlite 里拆 CSV 聚合很别扭；投票时服务端顺手把每个 chunk 的 KB slug（`retriever.GetChunk().Tier` 即注册表 slug，Phase 26 后语义）写进 `feedback_chunk_refs` 归一化表，per-chunk 聚合与 per-KB 指标都变成平凡 GROUP BY，admin 读路径零 retriever 依赖。

**Primary recommendation:** 按三段推进——(1) store 层：`AppendTurn` 返回 turn id + 三张新表 + InsertOnly 测试族；(2) server 层：`data-turn` chunk + `/api/feedback` + `/api/tickets` + admin 复核/指标端点（admin 门 403-先于-503、INSERT-only 审计行照抄 `putKBAdminAuditRow`）；(3) FE：FeedbackRow 真实化 + data-turn part 扫描 + AdminDashboard 第 6 tab + sf-toast 监听器。工单外发 = 落库即 201、goroutine 异步 webhook（`context.Background()` + 配置超时/重试）、结果只写审计行不回写工单行（保住 INSERT-only）。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| 消息身份（turn id 生成与暴露） | API/Backend（Go） | Browser（FE 缓存展示） | 身份必须服务端真源（D-29-01）；FE 只透传回传 |
| 投票/工单幂等与落库 | API/Backend + Database | — | UNIQUE 约束是幂等唯一真源，FE 锁定态只是缓存（CONTEXT specifics） |
| 被引 chunk/KB 归因 | API/Backend | Database | 服务端从 audit 行复制（D-29-05）；KB slug 来自 retriever 元数据，投票时落库 |
| 复核队列状态机 | API/Backend + Database | Browser（视图） | pending 是派生读模型；处置是 INSERT-only 表 + 审计 |
| 指标聚合 | Database（SQL） | API/Backend（HTTP 封装）+ Browser（图表） | 纯 sqlite GROUP BY；FE 只做展示与维度切换 |
| 工单外发 | API/Backend（goroutine） | External webhook（外部依赖） | best-effort、不阻塞（TKT-04）；外发结果进审计 |
| 反馈对知识库的作用 | 无（结构禁止） | — | FBK-05：唯一合法知识库变更入口是人工复核处置 → Phase 27 上传；反馈代码零 KB 写路径 |

## Standard Stack

### Core（全部已在本仓库，零新增）
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go + mattn/go-sqlite3 | go 1.25.0 / v1.14.50 | 新表 DDL + 幂等 INSERT + 聚合 SQL | 既有 Store 抽象与 InsertOnly 测试族 [VERIFIED: go.mod + internal/store/sqlite/schema.go] |
| cloudwego/hertz `pkg/app` | v0.10.6 | 三个新端点 + admin 门 | 既有路由块与 handler 范式 [VERIFIED: internal/server/server.go:442-628] |
| internal/wire（自研 SSE 封装） | in-repo | `data-turn` chunk 发射 | ValidTypes 白名单 + D15 TS 配对提交纪律 [VERIFIED: internal/wire/chunk.go:428-456] |
| net/http stdlib | go 1.25.0 | webhook provider 外发 | BGE embedder 的 per-instance `*http.Client` + 超时/重试先例 [VERIFIED: internal/embedder/bge/bge.go:90-130] |
| React 19 + `ai`（UIMessage parts） | web/package.json（ai ^7.0.59） | data-turn part 消费 + FeedbackRow 真实化 | data-decision/data-policy part 先例 [VERIFIED: MessageBubble.tsx:280-305] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | InsertOnly 测试族 + handler hermetic 测试 | 全部新测试 |
| vitest + @testing-library/react | web devDeps | FeedbackRow / 工单表单 / AdminDashboard tab 测试 | 全部 FE 测试 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `data-turn` 新 chunk type | `message-metadata` chunk（AI SDK 官方类型） | message-metadata 需要给 Go `UIMessageChunk` 加 Metadata 字段 + FE metadata 通道；data-turn 与 data-decision/data-subprogress 同族、FE 已有 part 扫描消费模式（verdictStatus），改动面更小。**选 data-turn** |
| `AppendTurn` 签名改 `(int64, error)` | AppendTurn 后补 `SELECT max(id)` 回查 | 回查零接口破坏但有同会话并发竞态窗口；签名改是事务内 `LastInsertId` 真相。**选签名改**（一个原子 commit 内含全部 fake 修复） |
| refs 归一化表 | feedback 行内 CSV + Go 侧拆分 | CSV 使 per-KB GROUP BY 退化为 Go 全表扫描；refs 表一次写入换两种聚合的纯 SQL。**选 refs 表**（投票路径多 ≤5 行 INSERT，dev-scale 无压力） |
| 工单行 UPDATE 回写外发状态 | INSERT-only + 结果只进审计行 | UPDATE 会破坏「工单请求 INSERT-only 落库」承诺与 source-grep 锁；本期 UI 不需要读回外发终态（admin 工单列表 deferred）。**选 INSERT-only + 审计行** |

**Installation:** 无 —— 本 phase 零新包，不需要 install 步骤。

**Version verification:** 无需外部 registry 校验（零新依赖）；上述版本均从 `go.mod` / `web/package.json` 直接读取 [VERIFIED: 本仓库清单文件]。

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| （无）本 phase 零新增外部包 | — | — | — | — | — | 不适用 |

**Packages removed due to slopcheck [SLOP] verdict:** none（无安装面，slopcheck 不适用）
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
【live 流路径】
POST /api/chat ──► VerdictPipeline
                      │  emitVerdictStream(text/data-policy/data-decision…)
                      ▼
              writeAuditAndTurn ──► audit 行(chunk_ids CSV) + session_turn(user+assistant)
                      │                 │返回 assistant turn id（新增）
                      ▼                 │
              emit data-turn{turnId} ◄──┘   （finish 之前，additive 帧）
                      ▼
                  finish → FE useChat message.parts += data-turn part
                                   │
【history 恢复路径】                ▼
GET /api/session/:id/history ──► historyEntry{turn_id, voted_verdict}
        │  （feedback 表 JOIN：turn→已投 verdict）
        ▼
historyToUIMessages ──► 合成 data-turn part ──► 同一 FE 消费点

【投票/工单写路径】
FeedbackRow ──► POST /api/feedback {session_id, turn_id, verdict, comment?}
                │ 1. 校验 verdict 枚举 + turn 存在且 role=assistant（session 绑定）
                │ 2. audit 邻近回溯 → 复制 chunk_ids + skill_id（服务端真相 D-29-05）
                │ 3. retriever.GetChunk(chunk)→Tier=KB slug → feedback_chunk_refs
                │ 4. INSERT feedback ON CONFLICT(session_id,turn_id) DO NOTHING
                │    └ 冲突 → 读回既有行 → 200 already_recorded（幂等 D-29-02）
                ▼ 200 {status, verdict}
创建工单 ──► POST /api/tickets {session_id, turn_id, description}
                │ INSERT tickets（UNIQUE(session_id,turn_id)，dispatch_status=none|queued）
                ├─► 201 {ticket_number, dispatch_status}   ← 立即返回，不等待外发
                └─► goroutine: TicketProvider.Dispatch（webhook：超时+重试）
                        └► 审计行 action_ticket_dispatch_{succeeded|failed}（INSERT-only）

【admin 读/处置路径（全部 admin 门 403 先于 503）】
GET /api/admin/feedback/review?status=pending|resolved ──► 按 chunk 聚合 + 三态过滤
POST /api/admin/feedback/dispositions {chunk_id, disposition, note?}
        └► feedback_dispositions 行（INSERT-only）+ 审计行（ActorType=admin）
GET /api/admin/feedback/metrics?dim=skill|base&days=N ──► 全票分母 + 日粒度序列
        └► AdminDashboard 第 6 tab「反馈」消费
```

### Recommended Project Structure
```
internal/store/sqlite/
├── feedback.go              # feedback + feedback_chunk_refs 读写（INSERT-only）
├── feedback_test.go         # TestFeedbackInsertOnly（source-grep + round-trip + 幂等）
├── tickets.go               # tickets 读写（INSERT-only）
├── tickets_test.go          # TestTicketsInsertOnly
├── review.go                # 待复核聚合查询 + dispositions 写入（或并入 feedback.go）
└── schema.go                # 三张新表 DDL + migrate() 尾部索引（既有文件追加）
internal/ticket/             # 新包：TicketService 接口 + provider 注册表
├── ticket.go                # Service/Provider 接口 + Ticket/DispatchResult 类型
├── webhook.go               # webhook provider（per-instance http.Client + 超时/重试）
├── webhook_test.go          # httptest.Server hermetic：2xx/重试后成功/耗尽失败/超时
└── registry.go              # NewProvider("none"|"webhook", cfg)
internal/server/
├── feedback.go              # POST /api/feedback + admin review/dispositions/metrics 端点
├── feedback_test.go
├── tickets.go               # POST /api/tickets + 异步外发 goroutine
├── tickets_test.go
└── server.go                # Deps.TicketService/ProviderName + 路由注册（既有文件）
web/src/
├── components/chat/parts/FeedbackRow.tsx   # 真实 API + 三态状态机
├── components/chat/MessageBubble.tsx       # 扫描 data-turn part → FeedbackRow props
├── components/admin/FeedbackPanel.tsx      # 第 6 tab：指标 + 复核视图
├── components/admin/AdminDashboard.tsx     # tab 列表 + adminSubroute union 扩展
├── lib/chat-restore.ts                     # SessionHistoryMessage + data-turn part 合成
├── types/wire-types.ts                     # SkillForgeChunk += data-turn（D15 配对）
└── __tests__/FeedbackRow.test.tsx          # 重写：fetch mock + 服务端驱动锁定态
```

### Pattern 1: turn id 暴露（chat 流 + history 双路径）
**What:** `session_turn.id` 是唯一被 D-29-01 锁定的消息身份。现状三处缺口：`AppendTurn` 丢弃 id；`getSessionSQL` 不 SELECT id；`historyEntry` 无该字段。
**When to use:** 本 phase 的地基任务，必须先于一切 feedback 端点。
**Example（关键改动点，全部经代码核实）:**
```go
// internal/store/store.go — 接口签名变更（同一 commit 内完成 impl + fakes）
// 旧: AppendTurn(ctx context.Context, sessionID string, user, assistant SessionMessage) error
// 新（建议）:
AppendTurn(ctx context.Context, sessionID string, user, assistant SessionMessage) (int64, error)
// 返回 assistant 行 id；ON CONFLICT 全冲突时返回 0（见 Pitfall 2）

// internal/store/sqlite/session.go — appendTurnHalf 捕获 LastInsertId
res, err := tx.ExecContext(ctx, appendTurnSQL, sessionID, role, m.Content, ts.UTC())
// RowsAffected==0（conflict DO NOTHING）→ 返回 0；否则 res.LastInsertId()

// internal/server/handlers_session_history.go — historyEntry 增列（additive）
TurnID       int64  `json:"turn_id,omitempty"`       // 仅 assistant 行非零
VotedVerdict string `json:"voted_verdict,omitempty"` // helpful|unhelpful|outdated，未投缺省
// getSessionSQL: SELECT id, role, content, ts …（SessionMessage 增 ID int64 字段）
```
SSE 发射窗口已核实：pipeline.go:528 `writeAuditAndTurn` 在 :572 text-end / :580 finish **之前**执行，agent 路径（chat.go:483→480-482）同样可以先发 data-turn 再收尾——两条生产路径都存在「turn 写完、finish 未发」的合法窗口。 [VERIFIED: internal/server/pipeline.go:519-583, chat.go:449-483]

### Pattern 2: `data-turn` wire 帧（ValidTypes + TS union 配对）
**What:** 新增 kebab-case chunk type，载荷 `{turnId:number}`，复用 `UIMessageChunk.Data json.RawMessage` 通道。
**When to use:** live 流路径；FE 把它作为 message part 消费。
**Example:**
```go
// internal/wire/chunk.go — ValidTypes += "data-turn": true（计数注释 23→24）
// 新 TurnPayload struct（镜像 DecisionPayload 的载体方式：Data json.RawMessage）
type TurnPayload struct {
    TurnID int64 `json:"turnId"`
}
```
```tsx
// web/src/types/wire-types.ts — SkillForgeChunk union 追加（D15 配对提交，同一 commit）
| { type: 'data-turn'; id?: string; data: { turnId: number; votedVerdict?: 'helpful'|'unhelpful'|'outdated' } }
// MessageBubble.tsx — 复用 verdictStatus 的部件扫描模式（:280-288 先例）
const turnPart = message.parts.find((p) => p.type === 'data-turn') as {data?: {turnId?: number; votedVerdict?: Vote}} | undefined;
<FeedbackRow turnId={turnPart?.data?.turnId ?? null} votedVerdict={turnPart?.data?.votedVerdict ?? null} />
// chat-restore.ts — historyToUIMessages 为 assistant 行合成同一 part（恢复路径零特判）
```
FE 侧 `message.id`（客户端生成/`${sessionId}-${ts}-${role}`）不再作为投票标识，只保留 React key 用途。 [VERIFIED: chat-restore.ts:113-114, MessageBubble.tsx:305]

### Pattern 3: feedback/tickets 表 DDL（schema.go 惯例）
**What:** 三张 additive 新表，全部走既有惯例：schemaSQL 内 `CREATE TABLE IF NOT EXISTS`，索引挂 `migrate()` 尾部独立 const（D14-E1 契约——本 phase 无 ALTER 依赖，但惯例统一）。
**Example:**
```sql
CREATE TABLE IF NOT EXISTS feedback (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    session_id  TEXT    NOT NULL,
    turn_id     INTEGER NOT NULL,   -- session_turn.id（D-29-01）
    verdict     TEXT    NOT NULL CHECK (verdict IN ('helpful','unhelpful','outdated')),
    comment     TEXT    NOT NULL DEFAULT '',   -- D-29-06 可选说明
    skill_id    TEXT    NOT NULL DEFAULT '',   -- 服务端从 audit 行复制
    chunk_ids   TEXT    NOT NULL DEFAULT '',   -- CSV 原样快照（审计可读）
    UNIQUE(session_id, turn_id)             -- D-29-02 一次一票
);
CREATE TABLE IF NOT EXISTS feedback_chunk_refs (   -- 聚合归一化（FBK-02/03 地基）
    feedback_id INTEGER NOT NULL,
    chunk_id    INTEGER NOT NULL,
    kb_slug     TEXT    NOT NULL DEFAULT '',  -- retriever.GetChunk().Tier（Phase 26 后=slug）
    UNIQUE(feedback_id, chunk_id)
);
CREATE TABLE IF NOT EXISTS feedback_dispositions ( -- INSERT-only，多次处置合法（时间戳定终态）
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    chunk_id    INTEGER NOT NULL,
    disposition TEXT    NOT NULL CHECK (disposition IN ('updated','false_positive','ignored')),
    note        TEXT    NOT NULL DEFAULT '',
    actor       TEXT    NOT NULL DEFAULT 'admin'
);
CREATE TABLE IF NOT EXISTS tickets (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    ts             DATETIME NOT NULL,
    session_id     TEXT    NOT NULL,
    turn_id        INTEGER NOT NULL,
    description    TEXT    NOT NULL,
    chunk_ids      TEXT    NOT NULL DEFAULT '',   -- CSV（同 audit.chunk_ids 先例）
    skill_id       TEXT    NOT NULL DEFAULT '',
    provider       TEXT    NOT NULL DEFAULT 'none',
    dispatch_status TEXT   NOT NULL DEFAULT 'none' CHECK (dispatch_status IN ('none','queued')),
    UNIQUE(session_id, turn_id)                -- D-29-14 一消息一单
);
-- 索引：idx_feedback_session(session_id) / idx_feedback_ts(ts) / idx_feedback_skill(skill_id)
--       idx_refs_chunk(chunk_id) / idx_dispositions_chunk(chunk_id) / idx_tickets_session(session_id)
-- 全部以独立 const 挂 migrate() 尾部（kpi_calls/reports/knowledge_bases 同款惯例）
```
注意 CHECK 约束是 closed vocabulary 的存储层兜底（knowledge_bases role CHECK 先例 [VERIFIED: schema.go:364-372]）。`ON CONFLICT ... DO NOTHING` 不含被 source-grep 锁禁止的动词；**禁用 DO UPDATE 变体**（含 "UPDATE " 子串，会撞 TestXxxInsertOnly 锁）。

### Pattern 4: 服务端 chunk/skill 归因（audit 邻近回溯）
**What:** D-29-05 要求被引 chunk 来自服务端。审计行与 turn 行之间**没有任何外键**（writeAuditAndTurn 先写 audit 再写两条 turn，id 不互通）——需要一个归因查询。
**推荐（邻近回溯）:** audit 行的 `Timestamp` 在 turn 的 ts 之前零点几毫秒写入，且 chat 路径每响应恰一行：
```sql
SELECT id, skill_id, chunk_ids, refusal_reason FROM audit
WHERE session_id = ? AND ts <= ?      -- ? = 该 assistant turn 的 ts
ORDER BY ts DESC, id DESC LIMIT 1
```
**备选（精确内容匹配）:** verdict 路径 `audit.verdict_json == session_turn.content`（同一字符串两次落库），refusal 路径 `audit.refusal_reason == content`；并发同会话两轮产生相同 verdict JSON 时用 ts 邻近消歧。推荐邻近回溯为主、内容匹配为测试对照（可写一个 hermetic 测试证明两法在同 fixture 上一致）。拒绝轮（refusal）的 audit 行 `SkillID` 照常携带 skill（只有 prompt-hash 入参为空），所以**拒绝轮投票同样可归因 per-Skill**；其 `chunk_ids` 为空 → refs 表无行 → per-KB 指标不含它（诚实缺口，非错误）。 [VERIFIED: chat.go:512-563（写入顺序与字段）, handlers_session_history.go:163-177]

### Pattern 5: 复核队列 = 派生 pending 视图 + INSERT-only dispositions
**What:** 不建「待复核条目」表——pending 状态由数据派生，避免第二写路径：
```sql
-- pending：某 chunk 有 outdated 票，且其后无同 chunk 处置行（最新处置赢）
SELECT r.chunk_id,
       MAX(r.kb_slug)                                   AS kb_slug,   -- 聚合展示用
       COUNT(*)                                         AS report_count,
       MAX(f.ts)                                        AS last_reported_at,
       GROUP_CONCAT(f.comment, char(10))                AS comments
FROM feedback f
JOIN feedback_chunk_refs r ON r.feedback_id = f.id
WHERE f.verdict = 'outdated'
  AND NOT EXISTS (SELECT 1 FROM feedback_dispositions d
                  WHERE d.chunk_id = r.chunk_id AND d.ts >= f.ts)
GROUP BY r.chunk_id
ORDER BY report_count DESC, last_reported_at DESC
```
处置端点 `POST /api/admin/feedback/dispositions` 写一行 dispositions + 一行审计（`putKBAdminAuditRow` 同款：`PutAudit{UserID:"admin", Action:"action_feedback_disposition", ActorType:"admin", FrozenArgs: snapshot}`）[VERIFIED: kb_admin.go:317-348]。**一处处置关闭该 chunk 全部条目**由 NOT EXISTS 时间比较天然满足（D-29-08）。chunk 预览不在列表响应里——FE 展开时走既有 `GET /api/knowledge/chunk/:id`，零新读面。
**结构红线（FBK-05）:** feedback/review 代码文件 source-grep 零变更动词、零 `Indexer`/`Retriever.Insert` 引用（`TestAdminKPIs/retrieval_only` 双 grep 先例 [VERIFIED: STATE D16-08-E]）。

### Pattern 6: 指标 SQL（全票分母 + 日粒度）
**Example:**
```sql
-- per-Skill（D-29-10 全票分母，Go 侧算比率避免 SQL 除零/精度坑）
SELECT skill_id,
       SUM(verdict='helpful') AS helpful, SUM(verdict='unhelpful') AS unhelpful,
       SUM(verdict='outdated') AS outdated
FROM feedback WHERE ts >= ? GROUP BY skill_id;
-- per-KB（经 refs 归因；一票涉多 KB 时每个 KB 各计一次——见 Open Questions 2）
SELECT r.kb_slug, SUM(f.verdict='helpful'), …
FROM feedback f JOIN feedback_chunk_refs r ON r.feedback_id=f.id
WHERE f.ts >= ? GROUP BY r.kb_slug;
-- 日趋势（D-29-11；周=Go 侧把日桶合并）
SELECT date(ts) AS day, verdict, COUNT(*) FROM feedback
WHERE ts >= ? GROUP BY day, verdict ORDER BY day;
```
端点 `GET /api/admin/feedback/metrics?dim=skill|base&days=N`（admin 门先行）。票数 0 → 返回空/`null` 帮助率，绝不返回 0% 或 NaN。FE 挂 AdminDashboard 第 6 tab：`adminSubroute` union（app-store.ts:457）加 `'feedback'` + tab 按钮 + ViewBody 分支（AdminDashboard.tsx:49/113-147/160-169 同款三处改动）。

### Pattern 7: FE FeedbackRow 真实化（服务端驱动锁定态）
**Example 状态机:**
```
props: turnId:number|null, votedVerdict:Vote|null（服务端真相，来自 data-turn part）
state: localVote = votedVerdict（缓存）；submitting；error
idle → click → POST /api/feedback
  ├─ 201/200 {status:"recorded"}      → localVote=verdict，toast「已记录」
  ├─ 200 {status:"already_recorded"}  → localVote=返回 verdict，toast「已记录过：X」
  ├─ 网络断/5xx                        → 不锁定（D-29-04），toast 显示真实错误，可重试
turnId==null（mock 流/极端冲突）→ 按钮禁用 + title 诚实说明「该消息无服务端轮次标识」
```
工单表单（D-29-13）：`创建工单` 点击 → 行内展开 textarea（预填 = 该消息 chatText 截断，可改）+ 引用展示（session 短 id / turn id / chunk chips）→ 提交 → `POST /api/tickets` → 三态诚实文案由 `dispatch_status` 驱动：`none`→「已受理（编号 TKT-x）：未配置外发 provider，工单已留档」；`queued`→「已受理（编号 TKT-x）：正在排队外发」；`already_exists`→「已建单：TKT-x」（D-29-14）。`sf-toast` 监听器按 wizard-success 先例挂 App.tsx（:301-309）。

### Pattern 8: TicketService 接口 + provider 注册表 + best-effort 外发
**Example:**
```go
// internal/ticket/ticket.go — 接口形状参照 IndexerBackend 的窄接口纪律
type Provider interface {
    Dispatch(ctx context.Context, t Ticket) (DispatchResult, error) // 终态才返回
}
type DispatchResult struct {
    ExternalRef string // webhook 可选回执；空 = 无
    Attempts    int
}
// registry.go
func NewProvider(name string, cfg Config) (Provider, error)
//   "none"    → noneProvider{}（零外发）
//   "webhook" → webhookProvider{url, client *http.Client, timeout, retries}
//   未知值    → 返回 error（main.go log.Fatalf——受控配置坏值不得静默降级，D16-08-D 姿态）
// webhook.go — BGE 先例：per-instance http.Client + 有界重试 + 1s 固定退避
```
```go
// cmd/server/main.go — env 接线（envOr/parseIntEnvOrDefault 先例 main.go:156-192）
providerName := envOr("SKILLFORGE_TICKET_PROVIDER", "none")
webhookURL   := envOr("SKILLFORGE_TICKET_WEBHOOK_URL", "")
timeoutMS    := parseIntEnvOrDefault("SKILLFORGE_TICKET_WEBHOOK_TIMEOUT_MS", 5000)
retries      := parseIntEnvOrDefault("SKILLFORGE_TICKET_WEBHOOK_RETRIES", 2)
// server.go Deps 增 TicketService ticket.Provider + TicketProviderName string
// tickets.go handler：落库 → 201 → 若 queued: go func(){ background ctx + 总预算; Dispatch; PutAudit } }()
//   goroutine 内 defer recover()（尽力而为的兜底）；审计行：
//   action_ticket_dispatch_succeeded | action_ticket_dispatch_failed（FrozenArgs={ticket_id,provider,attempts,err}）
```
**非阻塞保证：** 外发完全在 ticket POST 的新 goroutine 里，chat 与 feedback 路径零接触（TKT-04 结构性成立）。hermetic 测试用 httptest.Server 慢端点断言「handler 响应先于外发完成」。 [VERIFIED: IndexerBackend internal/retriever/backend.go; BGE client internal/embedder/bge/bge.go:90-130]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSE 幀白名单与发射 | 自写 SSE writer / 跳过校验 | `wire.EmitChunk` + ValidTypes 注册 | T-1-11 预写门防 snake_case 静默丢弃 [VERIFIED: internal/wire/sse.go:80-93] |
| 幂等去重 | SELECT-then-INSERT 手写锁 | UNIQUE + `ON CONFLICT DO NOTHING` + RowsAffected | DB 层并发安全；HITL pending 唯一索引同款 [VERIFIED: schema.go:450-453] |
| 出站 HTTP + 重试 | 新传输层 / fetch 循环 | BGE embedder 的 client+timeout+retries 形态 | 已在 intranet 环境验证过的姿势 [VERIFIED: bge.go:111-130] |
| admin 鉴权 | 新中间件/新 token 逻辑 | inline `isAdminRequest` 首行 + 403 先于 503 | 常量时间比较已实现 [VERIFIED: audit.go:114-129, kb_admin.go:139-146] |
| INSERT-only 审计行 | 新审计表/新写法 | `putKBAdminAuditRow` 形态复用 | FrozenArgs + ActorType=admin 已被 /api/admin/audit 消费 [VERIFIED: kb_admin.go:330-348] |
| chunk→KB 映射 | 自建 chunk 元数据表 | `retriever.GetChunk().Tier`（= 注册表 slug） | Phase 26 后 Tier 即 slug，fail-closed 已在索引/检索侧 [VERIFIED: retriever.go:109-116, graph.go:78-90] |
| FE part 消费 | 新 context/store 通道 | MessageBubble 部件扫描（verdictStatus 模式） | 已被 data-policy/data-decision 验证 [VERIFIED: MessageBubble.tsx:280-288] |

**Key insight:** 本 phase 的每个「新能力」都是既有先例的第二次应用；planner 的任务是把先例点名到具体 task，而不是发明新形态。

## Common Pitfalls

### Pitfall 1: Store 接口签名变更的编译涟漪（最高优先级）
**What goes wrong:** `AppendTurn` 加返回值会瞬间打破 `var _ store.Store` 编译门——SQLiteStore + internal/server 至少 5 个手写 fake（recordingAuditStore / fakeAdminSchedulesStore / spyAdminSchedulesStore / fakeAdminAuditStore / spyAdminAuditStore + upload_test mockStore）。fake 是逐方法实现（非 embed），不会自动继承新方法。
**Why it happens:** fake 显式实现全部接口方法；中间态 commit 无法编译。
**How to avoid:** 接口签名 + sqlite impl + `writeAuditAndTurn` 调用点 + 全部 fake 修复放**同一个原子 commit**（D16-03 SUMMARY 教训：接口与实现分 Task/commit 落地曾直接撞碎中间态 verify）。plan 需把 fake 清单写死进 files_modified。
**Warning signs:** `go build ./...` 在 store 包之外的 server/agent 包报「does not implement store.Store」。

### Pitfall 2: ON CONFLICT DO NOTHING 下的 LastInsertId 语义
**What goes wrong:** 冲突时 sqlite 的 `last_insert_rowid()` 返回**连接上最近一次成功 INSERT** 的 rowid（可能是同事务的 user 行，甚至连接池里上一请求的行）——拿它当 turn id 是静默错配。
**How to avoid:** 每个 half 的 INSERT 检查 `RowsAffected`：0 → 返回 0；writeAuditAndTurn 拿到 0 时不发 data-turn（FE 端按钮禁用降级，honest UI）。store 层加一个 hermetic 重放测试锁该语义。
**Warning signs:** 极端场景下投票落到错误 turn（幂等键校验会挡住跨 session，但同 session 错位不会报错）。

### Pitfall 3: data-turn 发射时序（finish 之后 = 永久丢失）
**What goes wrong:** useChat 在 finish 前不注册消息；finish 后发的帧进不来或迟到。若有人在 agent 路径把 data-turn 挪到 finish 后，live 流的 FeedbackRow 永远拿不到 turnId——功能静默死亡（无报错）。
**How to avoid:** 发射点固定在 writeAuditAndTurn 之后、text-end/finish 之前（两条路径都有该窗口）；hermetic 流测试断言帧序 `…data-turn < finish`。同时**必须**在 ValidTypes 注册——EmitChunk 对未注册 type 直接 error（仅 WARN 日志），漏注册同样是静默死亡。
**Warning signs:** FE 测试全绿（mock 流无 turnId 也会渲染禁用态）但 live 流点不动投票。

### Pitfall 4: audit 邻近回溯的错配窗口
**What goes wrong:** 同会话两个 chat 请求并发（或 NTP 回拨）时，`ts <= turn.ts ORDER BY ts DESC LIMIT 1` 可能取到相邻轮的 audit 行 → skill_id/chunk_ids 错归因。
**How to avoid:** FE 对同一会话的 chat 天然串行（单 Chat 实例 await sendMessage）；仍要 (a) 归因失败（查无 audit 行）时落 `skill_id=''`、`chunk_ids=''` 的票，绝不 fail 整个投票；(b) 用内容匹配（Pattern 4 备选）写一个对照测试钉住主查询语义。
**Warning signs:** 指标里 skill 维度出现空桶比例异常。

### Pitfall 5: source-grep 锁误伤自己的 DDL
**What goes wrong:** `TestXxxInsertOnly` 族 grep 文件内 `"UPDATE "`/`"DELETE "` 字符串；写成 `ON CONFLICT … DO UPDATE` 或注释里出现 "UPDATED" 都可能撞锁/绕锁。
**How to avoid:** 只用 DO NOTHING；新表测试文件从 kpi_calls_test.go:140-162 复制双段结构（source-grep + round-trip + 幂等重放断言「返回既有行而非新行」）。
**Warning signs:** insert-only 测试红但行为测试绿。

### Pitfall 6: 工单外发 goroutine 的 ctx 与 panic
**What goes wrong:** 用 handler 的 request ctx 跑外发 → handler 返回即取消，重试永远死在第一次 sleep；无 recover 的 panic 直接打崩 server。
**How to avoid:** goroutine 内 `context.WithTimeout(context.Background(), 总预算)`；`defer recover()` + 审计行兜底记录 failed。总预算/超时/重试全部 env 驱动（Phase 30 零硬编码抽查会扫新魔数）。
**Warning signs:** webhook 测试只在「慢端点 + handler 已返回」场景下间歇失败。

### Pitfall 7: FE 两处 message-id 语义混用
**What goes wrong:** 既有 FeedbackRow 测试与 MessageBubble 传 `message.id`（客户端 id / `${sessionId}-${ts}-${role}` 合成 id）；若投票 payload 继续用它，服务端无法校验（D-29-01 明确否决 FE uuid）。
**How to avoid:** FeedbackRow props 从 `messageId:string` 改为 `turnId:number|null`；MessageBubble 扫 data-turn part；chat-restore 合成同名 part——三条产线（live/history/mock）一个消费点。旧测试文件整文件重写而非增量补丁。
**Warning signs:** POST body 里出现非数字 turn 标识。

### Pitfall 8: admin 门顺序与 503 信息泄漏
**What goes wrong:** 新 admin 端点若先查依赖再验 token，403/503 顺序颠倒会泄漏「该面是否接线」。
**How to avoid:** 每个 admin handler 首行 `isAdminRequest`（kbAdminHandler :139-146 原样照抄）；hermetic 测试断言无 token → 403（即便依赖 nil）。
**Warning signs:** checker 的 gate-order 断言失败。

## Code Examples

### 幂等投票写（store 层）
```go
// Source: internal/store/sqlite/session.go:73-77（ON CONFLICT 形态）+ kpi_calls.go（InsertOnly 族）
const insertFeedbackSQL = `
INSERT INTO feedback (ts, session_id, turn_id, verdict, comment, skill_id, chunk_ids)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id, turn_id) DO NOTHING`
// RowsAffected==0 → QueryFeedbackByTurn(sessionID, turnID) 读回既有行 → handler 返回
// {status:"already_recorded", verdict: existing.Verdict}
```

### 发射 data-turn（server 层）
```go
// Source: internal/server/pipeline.go:519-582 的发射窗口 + chat.go:605-611 emitDecision 载体方式
func emitTurnID(c *app.RequestContext, turnID int64) error {
    if turnID <= 0 { return nil } // 冲突/降级：不发，FE 走禁用态
    payload, err := json.Marshal(wire.TurnPayload{TurnID: turnID})
    if err != nil { return err }
    return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-turn", Data: payload})
}
// pipeline.go: 在 writeAuditAndTurn(...) 之后、emitCounterEvidence 之前或之后均可，必须在 text-end/finish 之前
```

### webhook provider（best-effort 外发）
```go
// Source: internal/embedder/bge/bge.go:111-130（重试骨架）
func (w *webhookProvider) Dispatch(ctx context.Context, t ticket.Ticket) (ticket.DispatchResult, error) {
    body, _ := json.Marshal(map[string]any{
        "ticketNumber": t.Number, "sessionId": t.SessionID, "turnId": t.TurnID,
        "description": t.Description, "chunkIds": t.ChunkIDs, "skillId": t.SkillID,
    })
    var lastErr error
    for attempt := 0; attempt < w.retries+1; attempt++ {
        if attempt > 0 { select { case <-ctx.Done(): return res, ctx.Err(); case <-time.After(1 * time.Second): } }
        req, _ := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        if w.secret != "" { req.Header.Set("X-SkillForge-Ticket-Secret", w.secret) }
        resp, err := w.client.Do(req)   // client 带 per-instance Timeout
        if err == nil && resp.StatusCode/100 == 2 { return ticket.DispatchResult{Attempts: attempt + 1}, nil }
        lastErr = …
    }
    return ticket.DispatchResult{Attempts: w.retries + 1}, lastErr
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| TierEnum 4 层字面量 | KB slug 即 `retriever` Tier 字段 | Phase 26（2026-09-07） | chunk→KB 归因直接读 Tier，无需 tier→目录换算 |
| audit 行外无每轮 chunk 记录 | `audit.chunk_ids` CSV 每轮已写 | chat 路径既有 | 反馈归因零新写路径，只复制 [VERIFIED: chat.go:519-528] |
| history 只回显 role/content/verdict/ts | + turn_id / voted_verdict | 本 phase | additive JSON 字段，FE mirror snake_case（chat-restore 头注纪律） |
| ValidTypes 23 类 | 24 类（+data-turn） | 本 phase | D15 配对提交：Go ValidTypes 与 TS union 同 commit |

**Deprecated/outdated:** 无相关弃用；`message.id` 作为投票标识被 D-29-01 显式否决。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | audit 邻近回溯能把 turn 正确归因到 audit 行（同会话 chat 串行 + audit.ts ≤ turn.ts 恒成立） | Pattern 4 / Pitfall 4 | skill/chunk 错归因 → 指标失真；缓解 = 内容匹配对照测试 + 归因失败降级落空桶 |
| A2 | `ai` v7 data-* part 的缓冲/注册行为与 v5 data-decision 先例一致（part 在 finish 前落地即可被 useChat 收进 message.parts） | Pattern 2 | data-turn 收不到 → live 投票禁用（honest 降级，不产生错数据）；live 流冒烟即可证实 |
| A3 | webhook「成功」= HTTP 2xx；payload 格式由本项目自定义（范围内无真实下游系统） | Pattern 8 | 无——CONTEXT 已把真实工单系统划出 scope（discretion 授权） |
| A4 | `LastInsertId` + RowsAffected 守卫在 mattn/go-sqlite3 下语义如描述 | Pitfall 2 | 需要 store 层 hermetic 重放测试兜底（列为必写测试） |
| A5 | dispositions 允许多行、pending 取「最新处置赢」；处置后新增 outdated 报告会重新打开条目 | Pattern 5 | D-29-09 未规定「处置后再报告」语义——见 Open Questions 1，plan 前确认 |
| A6 | 一票涉多 KB 时每个 KB 各计一票（票数在 KB 维度可重复计数，在 Skill/总量维度严格一次） | Pattern 6 | 口径歧义 → 指标被质疑；见 Open Questions 2 |
| A7 | 拒绝轮（refusal）可投票、其 chunk_ids 为空 → 只进 per-Skill 不进 per-KB | Pattern 4/6 | 若用户期望拒绝轮不可投，FeedbackRow 渲染条件需加 verdict 判断——plan 应确认（默认可投：拒答质量也是信号） |

## Open Questions

1. **处置后再报告的语义**（A5）
   - What we know: D-29-09 锁定三处置均终态；未规定处置之后同一 chunk 收到**新的** outdated 报告时是否重开条目。
   - What's unclear: 用户意图——「已更新」后新报告大概率是新一批内容又过时（应重开）；「误报/忽略」后新报告（也可能重开）。
   - Recommendation: 采用「最新处置时间戳赢」（Pattern 5 SQL 天然支持：新报告 ts > 处置 ts → 重新 pending），并在 plan 中标注为可一行反转的口径常量；discuss-phase 未覆盖，建议 planner 默认重开并在 SUMMARY 里显式记录。
2. **多 KB 一票的指标口径**（A6）
   - What we know: 一个回答可能引用两个 KB 的 chunk；帮助率的 per-KB 分母语义未定义。
   - What's unclear: 重复计数是否可接受。
   - Recommendation: 接受重复计数（每 KB 视角独立统计「含该 KB 引用的回答的得票」），在反馈 tab 加一行口径说明文案；总量/per-Skill 维度严格一次。
3. **FeedbackRow 是否在拒绝轮渲染投票**
   - What we know: MessageBubble 对所有 assistant 消息渲染 FeedbackRow（含拒绝轮）。
   - Recommendation: 保持可投（拒绝质量是合法信号），无 chunk 引用时 refs 无行、UI 引用区显示「无引用」。若 planner 认为需收窄，改动点只在 MessageBubble 渲染条件。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go + mattn/go-sqlite3（cgo，store/sqlite 新表测试） | store 层测试 | ✓（WSL 路径，Windows 原生 gcc 有 emutls 限制 D15-01-1） | go 1.25.0 / driver v1.14.50 | sqlite 测试在 WSL2 跑（internal/data 同款先例） |
| WSL2 Go 工具链 | cgo 测试 + cmd/server 构建 | ✓（env-wsl-backend-runbook 既有 runbook） | — | — |
| Node + vitest + tsc | FE 测试 | ✓（Phase 28 收尾 535 FE tests 绿） | web/package.json | — |
| httptest.Server（stdlib） | webhook provider hermetic 测试 | ✓ | go stdlib | — |
| 真实工单系统 / 外网 webhook | TKT 验收 | ✗（scope 外） | — | provider=none + httptest 假端点即满足全部 SC |

**Missing dependencies with no fallback:** none。
**Missing dependencies with fallback:** 全部（见上表）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (BE) | go test + testify v1.11.1（go 1.25.0） |
| Framework (FE) | vitest（`web`，`npm run test`）+ `tsc --noEmit` |
| Config file | 既有（无新增框架配置） |
| Quick run command | `go test ./internal/server/ -run 'TestFeedback|TestTickets' -count=1`；`cd web && npx vitest run FeedbackRow` |
| Full suite command | `go test ./... -count=1`（cgo 包在 WSL：`wsl -e bash -lc "go test ./internal/store/sqlite/ ./internal/ticket/ -count=1"`）+ `cd web && npm run test && npx tsc --noEmit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FBK-01 | 投票落库 + 幂等重放返回已记录 + session/turn 绑定 + 枚举校验 | unit(hermetic) | `go test ./internal/store/sqlite/ -run TestFeedbackInsertOnly -count=1` + `go test ./internal/server/ -run TestFeedbackHandler -count=1` | ❌ 新建（Wave 内任务） |
| FBK-01 地基 | AppendTurn 返回 assistant turn id + conflict→0 | unit | `go test ./internal/store/sqlite/ -run TestAppendTurn -count=1` | ❌ 新建 |
| FBK-01 地基 | data-turn 帧序在 finish 前 + ValidTypes 注册 | unit(hermetic stream) | `go test ./internal/server/ -run 'TestVerdictPipeline|TestChatHandlerSSE' -count=1`（断言新帧） | ❌ 扩展既有 |
| FBK-02 | pending 按 chunk 聚合 + 处置关闭 + 处置历史筛选 + 审计行 | unit | `go test ./internal/store/sqlite/ -run TestReviewQueue -count=1` + `go test ./internal/server/ -run TestFeedbackReview -count=1` | ❌ 新建 |
| FBK-02 门 | admin 403 先于 503；无 token 处置被拒 | unit | `go test ./internal/server/ -run TestFeedbackDispositionAdminGate -count=1` | ❌ 新建 |
| FBK-03 | 全票分母帮助率 + per-Skill/per-KB + 日粒度 trend | unit | `go test ./internal/store/sqlite/ -run TestFeedbackMetrics -count=1` | ❌ 新建 |
| FBK-04 | FE 三态 + 服务端驱动锁定 + 失败不锁定 + toast 真实文案 | unit(vitest) | `cd web && npx vitest run FeedbackRow` | ❌ 重写既有 FeedbackRow.test.tsx |
| FBK-04 | history 恢复带 turn_id/voted_verdict | unit | `go test ./internal/server/ -run TestSessionHistory -count=1`（扩展）+ `cd web && npx vitest run chat-restore` | ❌ 扩展既有 |
| FBK-05 | 反馈/复核代码零 KB/retriever 变更动词（结构红线） | structural(grep) | `go test ./internal/server/ -run TestFeedbackNoTrustMutation -count=1` + verify-phase-29 stage | ❌ 新建 |
| TKT-01 | 注册表 none/webhook/未知值 fatal；webhook 2xx/重试后成功/耗尽失败/超时 | unit | `go test ./internal/ticket/ -count=1` | ❌ 新建 |
| TKT-02 | 工单 INSERT-only + 幂等 + dispatch 审计行 | unit | `go test ./internal/store/sqlite/ -run TestTicketsInsertOnly -count=1` + `go test ./internal/server/ -run TestTicketsHandler -count=1` | ❌ 新建 |
| TKT-03 | FE 三态诚实文案（编号/排队中/未配置）+ 展开式表单 | unit(vitest) | `cd web && npx vitest run FeedbackRow`（含工单表单用例） | ❌ 新建 |
| TKT-04 | handler 先于外发返回；外发失败不影响 chat/feedback 全套既有测试 | integration(hermetic) | `go test ./internal/server/ -run TestTicketDispatchNonBlocking -count=1` + 全量回归 | ❌ 新建 |

### Sampling Rate
- **Per task commit:** 对应包的 `-run` 快测 + `cd web && npx vitest run <touched>` + `npx tsc --noEmit`
- **Per wave merge:** `go test ./... -count=1`（WSL 含 cgo 包）+ `cd web && npm run test`
- **Phase gate:** `scripts/verify-phase-29.sh`（结构锁：FBK-05 source-grep / ValidTypes-TS 配对存在 / DDL 只用 DO NOTHING）+ 全量 BE/FE 绿；建议按 26-10 惯例补 `make phase-29` 目标

### Wave 0 Gaps
- None — 测试基建（go test/testify、vitest、hermetic Hertz httptest 模式、fake store 族）全部就绪；上表 ❌ 均为随 task 新建/重写的测试文件，非前置缺口。

## Security Domain

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no（沿用部署形态：chat/feedback 公开、admin 面 token） | — |
| V3 Session Management | partial | session_id 走既有 `sessionIDRegex`（`^[a-zA-Z0-9-]{8,128}$`）校验后再进任何查询 [VERIFIED: handlers_session_history.go:43,115-121] |
| V4 Access Control | yes | admin 端点首行 `isAdminRequest`（constant-time），403 先于 503 [VERIFIED: audit.go:114-129] |
| V5 Input Validation | yes | verdict/disposition 服务端 closed 枚举 + DB CHECK 兜底；turn_id 严格 int64>0 且必须存在、role=assistant；comment/description 长度上限 + 既有 NonImportMaxBodyBytes 体量门 |
| V6 Cryptography | no | 不引入新密码学；token 比较沿用 subtle.ConstantTimeCompare |
| V7 Authz Logging / V14 Config | yes | 处置与外发结果全部 INSERT-only 审计行；provider/重试/超时全 env 驱动（零硬编码，HYG 纪律） |

### Known Threat Patterns for (Go + Hertz + sqlite + React)
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 伪造投票/垃圾工单（公开写端点） | Spoofing | 与 /api/chat 同信任形态（本地部署）；服务端 session/turn 存在性绑定 + UNIQUE 幂等去重 + description/comment 长度上限；不做验证码（超出部署形态） |
| SQL 注入（新表查询/聚合） | Tampering | 全部 `?` 参数化（T-3-SQLI 姿态）；表名/列名零拼接 |
| 反馈越权改信任（FBK-05） | Elevation/Tampering | 结构锁：feedback/review 文件零 retriever/KB 写动词 + verify 阶段 grep 门；处置只产生状态行与审计行 |
| webhook SSRF/密钥泄漏 | Information Disclosure | URL 仅来自 operator env（非用户输入）；payload 只 POST JSON body，用户描述永不进 URL/header；可选 `X-SkillForge-Ticket-Secret` env 注入；per-instance client（不污染 DefaultClient，BGE 先例） |
| admin 面信息泄漏（503 探测） | Information Disclosure | 403 先于 503 的固定检查顺序 + hermetic 断言 |
| 跨会话投票错绑 | Repudiation | turn 查询强制 `WHERE session_id=? AND id=?` 双键；反馈行冗余存 session_id 供审计对账 |

## Sources

### Primary (HIGH confidence — 全部为本仓库直接检查)
- `internal/server/pipeline.go:480-584` — writeAuditAndTurn 与 text-end/finish 的发射顺序（data-turn 窗口）
- `internal/server/chat.go:512-563` — audit 行 + turn 对写入顺序、chunk_ids CSV 来源、SkillID 恒填
- `internal/store/sqlite/session.go` + `schema.go` — AppendTurn/GetSession 现状、DDL/迁移/索引惯例、CHECK 约束先例
- `internal/store/store.go:453-607` — Store 接口面、SessionMessage/AuditRow 结构
- `internal/server/kb_admin.go:136-348` + `audit.go:114-129` — admin 门、403/503 顺序、INSERT-only 审计行范式
- `internal/server/handlers_session_history.go` + `web/src/lib/chat-restore.ts` — history wire 契约与 FE mirror
- `internal/wire/{sse,chunk}.go` + `web/src/types/wire-types.ts` — ValidTypes 白名单、Data 通道、D15 配对纪律
- `internal/retriever/{retriever,backend}.go` + `internal/knowledge/graph.go` — GetChunk/Tier=slug、IndexerBackend 窄接口范式
- `internal/embedder/bge/bge.go:60-130` — 出站 HTTP per-instance client + 超时 + 重试先例
- `internal/server/admin_audit_test.go / kb_admin_test.go / upload_test.go` + `internal/store/sqlite/kpi_calls_test.go:140-162` — fake store 涟漪面 + InsertOnly 测试族
- `web/src/components/{chat/MessageBubble.tsx,chat/parts/FeedbackRow.tsx,admin/AdminDashboard.tsx}` + `store/app-store.ts:457` + `App.tsx:297-309` — FE 挂载点/部件扫描/tab 架构/toast 先例
- `cmd/server/main.go:132-235` + `scripts/verify-phase-28.sh` + `Makefile` — envOr 接线、verify 脚本与 make 目标惯例

### Secondary (MEDIUM confidence)
- `.planning/phases/16-p10-2/16-03-SUMMARY.md` — 接口签名与实现必须同 commit 的实证教训
- `.planning/STATE.md`（D14-C1/D15-01-1/D16-08-D/D16-08-E 条目）— WSL cgo 测试纪律、受控配置坏值 fatal、结构锁先例

### Tertiary (LOW confidence)
- A2（`ai` v7 data-* part 行为）——由仓库内 data-decision live 先例背书，未在本 session 实机复验

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 零新依赖，全部版本读自仓库清单
- Architecture（turn id / 表设计 / provider / 指标）: HIGH — 每个形态都点名了既有先例 file:line
- Pitfalls: HIGH — 涟漪面（fake 清单）、LastInsertId 语义、帧序、grep 锁误伤均为代码级核实；A1/A4/A5/A6 标记为需 plan/test 兜底的假设

**Research date:** 2026-09-08
**Valid until:** 2026-10-08（仓库不变式稳定；若 Phase 26-28 代码在此期间变动需复核 file:line 锚点）
