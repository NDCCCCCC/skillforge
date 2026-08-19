# Phase 11: 写工具 + 审批门（P8.2）- Context

**Gathered:** 2026-08-20
**Status:** Ready for planning
**Source:** `/gsd:discuss-phase 11` — Default mode (4 questions per area, 4 areas, 16 locked decisions)
**Requirements:** HITL-01..08 (8 v0.8 requirements) — 5 Success Criteria (ROADMAP §Phase 11)

<domain>
## Phase Boundary

agent 调用写工具前必须经人在环审批——流内审批卡、跨刷新/重启存活的审批态、INSERT-only 三行审计生命周期、"批的 A 不会执行成 B"。Runner 迁移（Interrupt/Resume 新语义）落在简单 ReAct archetype 上，**全套既有回归套件做安全网**。**FE 与 BE HITL 同 phase 交付，不可拆。**

**前置条件（Phase 10 落地）：**
- D10-A1 query-only 拒答分类器（`internal/server/classifier/types.go` RefusalClassifierInput 结构已锁 type-isolated）
- D10-B1 `internal/history/projection.go` 历史投影（已扩展为 verdict 渲染 + 中性占位 + 预算窗口）
- D10-C1/C2 query rewrite 前置 + 先 rewrite 后分类的链路已锁
- `data-decision` SSE chunk discriminator（D16）已承载 MEM-* 决策

**本 phase 交付：**
- 全 Runner 化：`internal/server/chat.go` 改写为 `internal/server/runner_setup.go`，驱动 `adk.NewRunner().Run(ctx, messages)` 一条线；旧 `internal/server/pipeline.go` 下沉为 5 文件（prompts / runner_sse adapter / tools/retrieve / callbacks/grounding / callbacks/classifier）
- 信任楼梯三层：trust floor short-circuit (BeforeAgentCallback) + D10-A1 classifier (BeforeModelCallback) + D2-01 grounding (AfterModelCallback)；HITL approval Tool+StatefulInterrupt
- `writes:` frontmatter 代码级强制：每轮按 `session.active_skill.writes` ⊆ `AllowedWriteTargets` 动态拼 Tool Registry；LLM 看不到未授权 target 即 prompt-inject 也调不到
- HITL Tool 共享 `ApprovalDecision { Decision, Reason, Target, FrozenArgs }` schema；`internal/agent/hitl/registry.go` 单文件 source of truth
- 跨刷新 Dual 端点：`/api/session/:id/approval` (GET pending) + `/api/session/:id/approval/:approvalId` (POST approve/deny)；D-04 transport-swap + D15 wire-type 不动
- 过期 checkpoint：snapshot 双表落（`checkpoint` 表 + `audit.action_pending_approval` 行均带 `skill_version` + `prompt_hash`）；不匹配时 DELETE checkpoint + UI expired state
- `action_pending_approval` 行在 `tool.StatefulInterrupt` 返回前同步写（fail-closed），与 D8-06 SC-5 stub Hermetic 实证路径一致
- `approval_id == tool_call_id`（HITL-04 幂等键 single identity）
- HITL-04 审计三行生命周期：`action_pending_approval` → `action_approved` / `action_rejected`
- approve-and-remember（HITL-08）：独立 `approval_rules` 表，PK = (target, args_hash, skill_id, scope_session_id)；session/skill 二元 scope；左 sidebar Rule panel UI；DELETE row 同时 INSERT audit `action_rule_revoked`

**不交付（deferred）：**
- Phase 12 PLAN-03 计划确认门（复用 HITL Tool 审批模式 + 同 ApprovalDecision schema + 新建 plan approve Tool 文件，本 phase 仅留 scaffold 不实现）
- Phase 13 FAN-04 子 agent per-sub-agent grounding（复用 AfterModelCallback grounding）
- frozen eval baseline v2 重锁（v0.8 milestone 收尾统一做，Phase 11-13 行为变更全部落地后）
- Runner 化 SC-2/3/4/5/6 + Phase 10 spike hard-gate 重测作为 Phase 11 acceptance gate（非 new feature 但必须重测）
- Skill reload 时 approval_rules 自动清理（优化项，不在 v0.8 必交付）
- Phase 7 遗留（07-02 T3 / 07-05 T2/T3 / 07-06）用户决策推迟到 v0.8 后，本 phase 不触碰

</domain>

<decisions>
## Implementation Decisions

### A. Runner 化边界（Area 1）

- **D11-A1: 全 Runner 化生产路径。** `chatHandler` 改写为 `adk.NewRunner().Run(ctx, []schema.Message{...history, query})` 一条线；VerdictPipeline 退役（拆 5 文件）。拒绝：双轨（变更面小但中断语义撕裂）、延迟到 Phase 12（block 后续 phases）。理由：history injection（D10-B1 拼 pipeline messages）天然契合 Runner.Run；HITL-04 中断点不限于写工具（任何 retrieval/finalize 节点默认 write-only interrupt）。
- **D11-A2: 信任楼梯三层（基于 eino v0.9.14 架构）。** 因 `tool.StatefulInterrupt` 只能在 Tool 函数内发起（Context7 quick_start/chapter_07 实证），**HITL approval = Tool+StatefulInterrupt（架构性边界，无其他选项）**；trust floor short-circuit = BeforeAgentCallback；D10-A1 query-only classifier = BeforeModelCallback；D2-01 grounding = AfterModelCallback。拒绝：所有 trust 升 Tool（多 2-3 次 LLM 调用 / 拒答 case 成本 + 模型调谁调谁不可控）、全 Callback（HITL 无法真正中断）。
- **D11-A3: pipeline.go 全部下沉为 5 文件。** `internal/agent/prompts.go`（system 模板 + userQueryTemplate）、`internal/agent/runner_sse.go`（eventToUIMessageChunk adapter）、`internal/agent/tools/retrieve.go`（retrieve Tool 实现）、`internal/agent/callbacks/grounding.go`（D2-01 AfterModelCallback）、`internal/agent/callbacks/classifier.go`（D10-A1 BeforeModelCallback）；原 `pipeline.go` 删除或退化为 alias。拒绝：sub-graph 包装（双层抽象）、保 VerdictPipeline 为 Tool（违反信任楼梯分层）。
- **D11-A4: SSE adapter + ctx 共享。** 新建 `runner_sse.go` adapter 函数 `eventToUIMessageChunk(e *AgentEvent) UIMessageChunk`，把 Runner AsyncIterator 事件逐个翻译到现有 chunked SSE writer；chatHandler 起的同一 ctx 传给 Runner.Run 与 SSE writer，cancel = ctx.Done() 同时断两边。`TestCancelPropagationMidTool` <200ms 锁原口径不动（Phase 8 升级为 Runner 测同一路径）；D-04 transport-swap 测试不变（D15 wire-type compile-time alarm 不破）。拒绝：Runner.Cancel() 主 path（多一层 HTTP 走 <200ms 锁难保）、放弃 chatHandler sync（破坏 D-04）。

### B. writes: 代码级强制层级（Area 2）

- **D11-B1: 仅运行时动态拼 Tool whitelist。** Skill 加载期不做 enforce（manifest validator 允许任何已知 target）；每轮 Agent Construction 按 `session.active_skill.writes` ⊆ `AllowedWriteTargets` 拼 Tool Registry，呈现给 LLM；未授权 target 不出现在 Tool 列表中 → prompt-inject 调不到。拒绝：仅加载期校验（agent 可看未授权 tool，prompt-inject 可调）、三层全上（与 HITL-04 frozen snapshot 路径冗余抵消）。理由：HITL-05 「agent 无法绕过」是结构性能力，不是加载期字符串校验。
- **D11-B2: 共享 `ApprovalDecision` schema type。** `ApprovalDecision { Decision enum, Reason string, Target string, FrozenArgs json.RawMessage }`；所有 HITL Tool file（hitl_memory_facts.go / hitl_review_report.go）使用同一 schema；schema.RegisterName[ApprovalDecision] + storedArgs 携带 tool_call_id。Phase 12 计划确认门加 plan approve 不需新 Schema 类型。拒绝：per-target 专属类型（每 target 加 schema + Runner 多 Schema 路由）、interface 适配（eino v0.9.14 反序列化 interface 要 JSON witness type 本质仍是 RawMessage）。
- **D11-B3: 编译期允许表 + 路由 map 单文件。** `internal/agent/hitl/registry.go`：`var AllowedWriteTargets = []string{"memory:facts", "review_report"}` + `var hitlHandlers = map[string]HITLHandler{...}`；三处共享 source of truth — (a) Agent Construction 拼 Tool Registry、(b) approve-and-remember rule 匹配走同一允许表、(c) Phase 12 plan approve 加 target 时一处改动全场同步。拒绝：包 init() 注册（global init() 生态差调试）、Skill manifest dynamic discovery（v0.8 不要求 hot config）。
- **D11-B4: per-Session Agent 单例 + per-turn Tool whitelist 重拼；pending HITL 时 Skill picker 禁用切换。** Agent instance per-session 单例持续存活（不重建，与 D14 useChat 会话连续性零冲突）；Skill 切换 = 下轮 Tool whitelist 重拼（纯函数式计算 `AllowedWriteTargets ∩ currentSkill.writes`）；pending HITL 时 Skill picker 禁用 + 提示「有审批进行中，请先处理后再切换」。拒绝：每 Skill 一个 instance（states 撕裂 + session_turn 不连续）、Union 跨 Skill 双 Tool 集（破坏 HITL-05 不变量）。

### C. 跨刷新审批态恢复（Area 3）

- **D11-C1: Dual 端点 — chat history + approval。** chat history 走 `/api/session/:id/history`（Phase 10 已锁）+ **新建** `GET /api/session/:id/approval` 返回待决 approval 列表（按 active_session_id bound）+ **新建** `POST /api/session/:id/approval/:approvalId`（body: `{decision, reason?}`）返 200 后端驱动 Runner.ResumeWithParams。FE ApprovalCard 状态派生自 store（D14 不破）；D-04 transport-swap + D15 wire-type alarm 测试不变。拒绝：合成 user message RESUME（FE chat 看到「system-resume」奇怪 message）、SSE sticky session（Vercel AI SDK v5 不支 Edge 复接）。
- **D11-C2: Snapshot 双表落。** `checkpoint` 表加 `skill_version` + `prompt_hash` 列（D8-03 已预留 `session_id/skill_id/expires_at`）；`audit.action_pending_approval` 行同样 snapshot 两字段。Resume 时比对 `checkpoint.skill_version == current Skill.manifest.version` AND `checkpoint.prompt_hash == now-active Skill.RenderSystemPrompt().Hash()`，不匹配 → DELETE checkpoint row + UI "expired" state + audit chain 完整保留。拒绝：仅 audit 是 source（checkpoint 不带版本则 resume 路径多走 audit query 重）、不要过期（违背 HITL-03 明示 "过期 checkpoint 有显式 UI 态"）。
- **D11-C3: `action_pending_approval` 行在 `tool.StatefulInterrupt` 返回前同步写（fail-closed）。** Tool.StatefulInterrupt 调用返回前同步写 audit.action_pending_approval 行（含 frozen_args + skill_version + prompt_hash snapshot）；失败则 Tool 返回 error + 拒绝执行（符合 HITL-04 "写路径 fail-closed"）；之后 Runner emit interrupt event + CheckPointStore.Set。理由：D8-06 SC-5 stub Hermetic 实证这条路径；HITL-04 不变明示。拒绝：checkpoint 后异步写（race window）、GET approval 时 lazy 写（违背 fail-closed）。
- **D11-C4: `approval_id == tool_call_id`。** HITL Tool StatefulInterrupt storedArgs.tool_call_id = audit.action_pending_approval.PK = POST approve/deny 路由 primary key；audit 表 PK UNIQUE 约束天然兜底跨设备并发；同一身份贯穿 ApprovalCard / audit / resume 三层。拒绝：approval_id 独立 + tool_call_id 跨表 crossref（双 ID 对账复杂）、approval_id = hash(args+skill+ts)（hash collision 测试成本 + HITL-04 abstractness 部分冲突）。

### D. approve-and-remember（HITL-08 — Area 4）

- **D11-D1: Rule 匹配 = target + sha256(frozenArgs) + skill_id。** Approve 后记录 ice: (target, args_hash, skill_id, scope)；HITL Tool 调用三字段比对匹配 → auto-pass；不匹配走正常 interrupt 流程。理由：HITL-07 edit-before-approve 已改 frozen_args → rule key 与 frozen snapshot 一一对应；hash 撞 0；与 HITL-04 frozen snapshot 不变路径兼容。拒绝：仅 target name（违反 HITL-02 "审批绑定结构化 tool call (name+args)"）、target + JQ / JSON Path 表达式（UI 复杂度推 Phase 12）。
- **D11-D2: 独立 `approval_rules` 表。** 新建 `internal/store/sqlite/approval_rules.go`；PK = (target, args_hash, skill_id, scope_session_id)，列含 scope / created_at / expires_at?。Agent Construction 前查 rule 表决 whitelist 中 target 的 auto-pass 子集；可撤销 = DELETE WHERE pks = X + INSERT audit action_rule_revoked；可查看 = SELECT WHERE scope in (current_session | current_skill)。拒绝：复用 audit.action_approved（DELETE = UPDATE 违背 T-3-AUDIT INSERT-only）、session_turn 派生（session-scope 跨 session 不可查）。
- **D11-D3: rule scope = session / skill 二元。** Approve 时 UI radio button 让用户选 scope；session 规则 PK 中 scope_session_id = current session_id；skill 规则 scope_session_id = "" 但 skill_id 一致即跨 session 共享。拒绝：Skill-only 默认（用户跨 session 不可控；HITL-08 明示两个 scope）、三 scope 含 global（违反 HITL-05 Skill 限 writes）、Skill manifest 决定默认 scope（manifest 字段扩张 Phase 12 做）。
- **D11-D4: 左 sidebar Rule panel + audit rule_revoked。** 新建 left sidebar Approval Rules section（复用 Phase 5 5-07 LeftSidebar 架构 + 5-07 SKILL load `item.manifest.description` BLOCKING CONSTRAINT 同样的 block const pattern）；list 列展示 (target / scope / created_at / last_used_at)；每行 X 图标 DELETE row + INSERT audit `action_rule_revoked` (INSERT-only)；audit chain 完整保留。SELECT 过滤 scope = current_session 或 current_skill 自动。理由：HITL-08 第二句明示 "可查看可撤销"；T-3-AUDIT INSERT-only 不破；与 Phase 5 LeftSidebar 架构一致。拒绝：Skill 面板（session-scope 不可见）、session 内派生（scope 跨 session 不可查）。

### E. Claude's Discretion

- **`memory:facts` 与 `review_report` 两个 write target 的具体 payload schema 字段**（HITL-06）：planner 与 executor 在实施时定；仅锁 schema 名与允许 target 白名单
- **prompt_hash 具体 hash 算法**：当前建议 sha256(RenderSystemPrompt())；若调研发现 skill hot reload 时有 fingerprint 字段可直接复用亦可
- **approval_rules 表 expires_at 列是否真带自动清理**：Area 4 Q4 locked path 是 "panel + Skill reload auto-expire" 优化项；不强制本 phase 落地，留 Claude's Discretion
- **Audit `action_rule_revoked` 行的 audit schema 形态**：当前建议同 action_approved schema 增 fields 记录 rule PK 范围；具体 column 集合由 executor 实施定

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 不变量 + 决策锚（in-scope 不可破）
- `.planning/PROJECT.md` §Core Value — 贯穿性不变量 "citation 白名单 = 本轮检索，代码强制"
- `.planning/REQUIREMENTS.md` §Write Tools + Approval Gate (HITL-01..08) — 8 条需求原文（含 3 行生命周期 / 跨刷新 / writes: 强制 / edit-before-approve / approve-and-remember）
- `.planning/ROADMAP.md` §Phase 11 — Goal / Mode / Depends on Phase 10 / 5 SC / Pitfalls addressed（3 审批门破坏 / 4 持久 checkpoint / 6 版本绑定 / 10 FE 状态机）/ Research flags RF-3
- `.planning/STATE.md` — Phase 10 closed-blocker-notes (用户 2026-08-20 签字接受)

### Phase 9 + Phase 10 落地基础（直接前置）
- `.planning/phases/10-p8-1/10-CONTEXT.md` — D10-A1 query-only 隔离 / D10-B1 history injection 在 pipeline / D10-B3 token 预算 / D10-C1-C3 query rewrite 链路
- `.planning/phases/10-p8-1/10-RESEARCH.md` — Phase 10 plan + Validation Architecture
- `.planning/phases/09-trust-floor-spike-p8-s-spike-hard-gate/09-CONTEXT.md` — D9-B1/B2/B3 投影契约 / D9-V1/V2 信任地板逃逸方案

### 已交付核心资产（直接扩展）
- `internal/server/pipeline.go` — 现有 VerdictPipeline 形态（本 phase **全部下沉为 5 文件**，原文件删除）
- `internal/server/chat.go` — 现有 chatHandler 形态（本 phase 改写为 runner_setup.go 调度）
- `internal/agent/` — Phase 8 Runner 落地骨架（`internal/agent/{runner_setup,prompts,runner_sse,callbacks,tools,hitl}/`）
- `internal/store/sqlite/checkpoint.go` — D8-03 骨架（本 phase 实施：增 skill_version/prompt_hash 列 + Delete 方法 + expires_at 过滤）
- `internal/store/sqlite/schema.go` — D8-03 已预留 session_id/skill_id/expires_at 三列
- `internal/store/sqlite/session.go` — GetSession/AppendTurn（phase 10 已用）
- `internal/verdict/{verdict,citation,grounding}.go` — D2-01 grounding moat（AfterModelCallback 复用 Normalize 规范化）
- `internal/server/classifier/types.go` — D10-A1 RefusalClassifierInput type-isolated（BeforeModelCallback 复用 prompt）
- `internal/audit/` — T-3-AUDIT INSERT+SELECT only（本 phase 增 action_rule_revoked 行 type）
- `internal/skill/loader.go` — Skill Loader（manifest.writes 字段时间合法校验；仅 allow 已知 target 不 enforce）

### eino v0.9.14 关键 API（Context7 实证）
- `@$HOME/.claude/get-shit-done/references/cloudwego_io_eino/04-adk-runner.md` — `Runner.ResumeWithParams(ctx, cpID, &ResumeParams{Targets: resumeData})`
- `chapter_07_interrupt_resume` — `tool.StatefulInterrupt(ctx, &ApprovalInfo{...}, storedArgs)` + `tool.GetResumeContext[T](ctx)` + `tool.GetInterruptState[T](ctx)`
- `callbacks/doc.go` — Callbacks 横切（BeforeAgent / BeforeModel / AfterModel）作 trust 楼梯实现

### FE 资产（D14 + D-04 transport-swap 不动约束）
- `web/src/components/` VerdictCard / CitationCluster / DataPolicyPart / DataDecisionPart（Phase 4/5 落地）
- `web/src/stores/app-store.ts` — D14 禁 messages 字段约束（本 phase Approval Rules panel 不进 store.messages）
- `web/src/components/LeftSidebar.tsx` — Phase 5 5-07 4 section 架构（左 sidebar 新增 Approval Rules section）
- `web/src/lib/real-chat-transport.ts` — RealChatTransport（real-chat path SSE → UIMessageChunk 解析层；approval GET/POST 走 axios 单独 channel）

### frozen 评估（Phase 11 重测基线）
- `eval/frozen/compliance.lock.json` — v0.8 baseline pin (sha256 50bb1688, D8-07)
- `eval/golden/compliance.jsonl` — 75 例（Phase 9 + 10 扩）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/store/sqlite/SQLiteStore`** — D18/D8-03 已锁接口 + schema + 六面测试；`Get/Set` 满足 `compose.CheckPointStore`（编译期断言 `var _ compose.CheckPointStore = (*SQLiteStore)(nil)`）。本 phase 在此之上：`Delete(id)` 方法（D8-06 stub 路径预留）+ `WHERE skill_version=? AND prompt_hash=?` 过期过滤 + 列 DDL 增量迁移。
- **`internal/verdict/grounding.go`** — D2-01 grounding moat 完整；`Normalize()` 全角→半角 + whitespace collapse 已是统一规范化层；AfterModelCallback 直接复用 `Verdict.GroundingChecker.Check()`。
- **`internal/server/classifier/types.go:RefusalClassifierInput`** — D10-A1 type-isolated struct（仅 query + chunks，无 History 字段）；BeforeModelCallback 复用 prompt + 结构化 JSON 输出。
- **`internal/agent/`** — Phase 8 落地骨架包含 Runner + Agent factory；本 phase 直接 extend 而非新建。
- **`internal/store/sqlite/audit.go`** — T-3-AUDIT INSERT+SELECT only；本 phase 扩 action_rule_revoked 行 type。
- **`internal/store/sqlite/session.go:GetSession/AppendTurn`** — session_turn DDL 已含 raw verdictJSON；本 phase scope session_id-scope rule。
- **`web/src/components/LeftSidebar.tsx`** — Phase 5 5-07 4 section IA；本 phase 新增第 5 section "Approval Rules"。
- **Web 路由 constants** — `/api/session/:id/history`（Phase 10）模式可镜像出 `/api/session/:id/approval` 与 `/api/session/:id/approval/:approvalId`。

### Established Patterns

- **D11 interface-first (mirroring D11/D18)** — `internal/agent/hitl/registry.go` 路径与 `internal/retriever/`、`internal/store/` 同模式；多场 source of truth 是同一文件 `AllowedWriteTargets + hitlHandlers`。
- **D8-06 SC-5 stub pattern** — RegisterName[ApprovalDecision] 双注册（Go `schema.RegisterName[T]` + test grep 双重强制）+ Round-trip + 两进程 stub 剧本；本 phase 沿用同一 pattern 增 Delete + 过期过滤测试。
- **D8-04 禁令门** — `TestNoLegacyInterruptAPI` WalkDir 全仓扫描；本 phase 同样 WalkDir 扫禁用形态（`Runner.Query` 与 `Runner.Resume` 的 deprecated 形态）。
- **T-3-AUDIT INSERT-only + parameterized** — `QueryAudit` 100% `?`-parameterized；本 phase 增列仅走 `ALTER TABLE audit ADD COLUMN ...`（DDL 幂等迁移），audit 表 SELECT ONLY for read-by-id。
- **D14 Zustand 禁 messages** — ApprovalCard 与 ApprovalRules panel 状态全部派生自 store（独立 slice `approval`），不进 messages；useChat 不破。
- **D-04 transport-swap byte-identical** — mock ↔ Default 产出 byte-identical UIMessageChunk；ApprovalCard 不从 SSE 通道流式（仅 GET 后渲染），不破 transport-swap。
- **D15 wire-type compile-time alarm** — `AssertAssignable<SkillForgeChunk, UIMessageChunk>` 仅扩 data-decision decision 类型字符串，不扩 wire union；Phase 11 ApprovalCard 不引入新 SSE chunk 类型。

### Integration Points

- **`internal/server/chat.go`** → 改写为 `internal/server/runner_setup.go`：chatHandler 调用 `buildRunner(skillManifest)` → `runner.Run(ctx, messages)` → 回调 `runner_sse.go` adapter 推 SSE。
- **audit 表 DDL** → 增量 ALTER ADD COLUMN skill_version + prompt_hash 字段；action_pending_approval 行加 (skill_version, prompt_hash, frozen_args, tool_call_id)；action_rule_revoked 同 schema。
- **checkpoint 表 DDL** → 增量 ALTER ADD COLUMN skill_version + prompt_hash 字段；本 phase 实施 `Delete(id)`。
- **`internal/skill/loader.go`** → manifest validator 增 allowed_writes 校验（D11-B3 共享 AllowedWriteTargets 列表）；不 enforce，仅 allow known target。
- **`internal/store/sqlite/approval_rules.go`** → 新建；`Create(pks, scope)` + `Query(scope)` + `Delete(pks)` + `Revoke(pks)` (revoke 额外写 audit row)。
- **`web/src/components/LeftSidebar.tsx`** → 新增第 5 section `ApprovalRules`；load via `GET /api/skills/me/approval-rules?scope=session|skill`。
- **`web/src/components/ApprovalCard.tsx`** → 新建（独立组件，不进 message stream）；基于 `GET /api/session/:id/approval` 状态渲染；click approve/deny 调 `POST /api/session/:id/approval/:approvalId`。

</code_context>

<specifics>
## Specific Ideas

- **Runner 化路径已 D10-B1 预言** — Phase 10 把 history injection 落在 VerdictPipeline 时就把投影输出做成 `[]schema.Message` 形态，预设 Runner.Run 形态契合；本 phase 兑现预期不破既有 Phase 10 investment。
- **Trust 楼梯的 callback-tool 二分是 eino 架构性边界** — interrupt 仅 Tool function 内可发起（Context7 实证），trust floor/grounding 走 callback 是 eino 推荐 pattern；不接受折中。
- **`approval_id == tool_call_id` 一句话 identity 是 HITL-04 的关键** — 跨表 PK UNIQUE 约束天然兜底 race，policy 简单到一行代码可表达；不是工程妥协是设计意图。
- **写 audit 行必须 fail-closed** — HITL-04 与 D8-06 SC-5 stub 双重明示；audit 写入失败则 Tool 拒绝执行；这是 Phase 11 收尾时的 acceptance test（不是优化项）。
- **Skill 切换的简单实现** — Agent instance per-session 单例 + per-turn Tool whitelist 重拼（纯函数）；pending HITL 时禁用切换（用户清晰反馈）；这是 Claude Code 类比的最简实现路径，没有引入复杂的 "Skill transition state machine"。
- **Ragas/frozen eval baseline v1 保档不动** — 本 phase 行为变更（Runner 化）by Phase 11 SC acceptance 重测合规后，frozen v2 重锁归 v0.8 milestone 收尾统一做。
- **History injection 跨刷新 resume 时复用 MEM-03 白名单** — Phase 10 已声明此不变量（CONTEXT §Deferred Ideas），Phase 11 落地时 verification：Runner.messages 中 history 段仍走 D10-B1 projection（role filter + scrubChunkRefs user 侧 + budget window）；不重新开辟路径。

## Specific test lock requirements

- **TestNoLegacyInterruptAPI (D8-04) 继承**：WalkDir 扫 Phase 11 新增的 `internal/agent/{callbacks,tools,hitl}/` 全树仍 0 命中
- **TestInterruptResumeRoundTrip (D8-06 SC-5) 升级为 Runner path**：同 DSN 重开 + 新 Agent/Runner + interrupt → checkpoint existed=true → DELETE on expire paths 全部覆盖
- **TestApprovalRulesScope**：session vs skill 二元；同一 args_hash 在另一 session 不命中 session-scope rule
- **TestApprovalRulesRevoke**：DELETE approval_rules row + INSERT audit action_rule_revoked 同事务内
- **TestExpiredCheckpointUIState**：skill_version mismatch → GET pending approval 返回 UI state "expired" + approval_id 不可 approve
- **TestFailClosedAudit**：audit INSERT 失败模拟 → Tool 返回 error + Runner 不走 checkpoint Set（forced_stop 路径）
- **TestCancelPropagationMidToolRunner**：<200ms 锁原口径不动（共享 ctx）
- **TestTransportSwapWithApprovalCard**：D-04 byte-identical + ApprovalCard 新 GET/POST 不污染 transport-swap 单元

</specifics>

<deferred>
## Deferred Ideas

- **Phase 12 PLAN-03 计划确认门** — 复用 HITL Tool 审批模式 + 同 ApprovalDecision schema + 新建 `hitl_plan_approve.go` Tool 文件 + registry 加一行；本 phase 仅在 scaffold 留 phase 12 add-point 文件注释，不实施 plan approval 端到端
- **Phase 13 FAN-04 子 agent per-sub-agent grounding** — AfterModelCallback grounding 模式复用 + ChunkAccumulator per-Run baked into closure
- **frozen eval baseline v2 重锁** — v0.8 milestone 收尾统一做（Phase 11-13 全部 behavior 变更落地后一次性 rebaseline）
- **Skill hot reload 时 approval_rules 自动清理** — Area 4 Q4 locked path 上的优化项；v0.8 不强制
- **memory:facts 与 review_report 两个 write target 的具体 payload schema 字段** — planner + executor 实施时定
- **RAGAS judge/eval sidecar 升级** — Phase 11 不引入（Go-native 不变 D6）
- **Runner 化 SC-2/3/4/5/6 + Phase 10 spike hard-gate 重测** — 作为 Phase 11 acceptance gate（非 new feature），最终 plan 在 verify 阶段统一跑
- **Phase 7 遗留（07-02 T3 / 07-05 T2/T3 / 07-06）** — 用户决策推迟到 v0.8 后，本 phase 不触碰
- **Plan 03-06 frozen CI gate 上 HITL 行为变更豁免理由** — Phase 11 验收完成 + frozen v2 重锁时一并处理
- **approval_rules 表 + Skill manifest 的双向联动 UI**（Skill 级 rule 数量显示在 Skill picker hover） — UX 优化项，未在 v0.8 必交付

</deferred>

---

*Phase: 11-p8-2-write-tools-approval-gate*
*Context gathered: 2026-08-20 via /gsd:discuss-phase 11 (Default mode, 4 areas × 4 questions = 16 decisions)*

<!-- gsd-sdk bypass: this CONTEXT.md was written manually in place of `gsd-sdk query commit`. The README in .planning/PROJECT.md gsd-sdk 章节 notes Phase 11 was the first phase ran with gsd-sdk PATH bypass. -->
