# Phase 11: 写工具 + 审批门（P8.2）- Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in 11-CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-20
**Phase:** 11 — 写工具 + 审批门（P8.2）
**Areas discussed:** Runner 迁移边界 / writes: 强制层 / 跨刷新恢复形态 (RF-3) / approve-and-remember 形态 (HITL-08)
**Discussion mode:** Default (4 single-question turns per area, total 16 decisions locked)
**gsd-sdk status:** bypass (commit + state ops done manually)

---

## Area 1: Runner 迁移边界

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q1 | A | 双轨型（最小变更） | |
| Q1 | **B** | **全 Runner 化（最大统一性）** | ✓ |
| Q1 | C | 混合型 | |
| Q1 | D | 延迟 Runner 一 phase | |

**User's choice:** B — 全 Runner 化
**Notes:** History injection（Phase 10 D10-B1 拼 pipeline messages）天然契合 Runner.Run(ctx, []schema.Message)；trust 楼梯拆 callbacks/tools 干净。User 选最大统一性，承担 SC-2/3/4/5/6 + Phase 10 spike hard-gate 重测成本。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q2 | A | Tool 形态（三 Tool 串联） | |
| Q2 | B | 纯 Callback 路径 | |
| Q2 | C | Hybrid：short-circuit = Callback，grounding = Tool | |
| Q2 | **D** | **信任楼梯三层（确认）** | ✓ |

**User's choice:** D — 信任楼梯三层（确认 via Context7 research）
**Notes:** Q2 第一次问时未含 Context7 实证依据，user 推 back "这个问题是替换 runner 引起的吗？请联网搜索"。联网 + Context7 quick_start/chapter_07_interrupt_resume 实证：`tool.StatefulInterrupt` 只能在 Tool 函数内发起（架构性边界）。LOCK：HITL approval = Tool+StatefulInterrupt；trust floor = BeforeAgentCallback；D10-A1 = BeforeModelCallback；D2-01 grounding = AfterModelCallback。callback 不是可选形态，是 eino 推荐 pattern。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q3 | A | 全部下沉，按职能拆到各新文件 | ✓ |
| Q3 | B | 留 pipeline.go 作 Runner sub-graph 组装点 | |
| Q3 | C | 保留 VerdictPipeline 为 Tool | |

**User's choice:** A — 全部下沉，按职能拆到各新文件
**Notes:** 5 文件归宿 — internal/agent/prompts.go, runner_sse.go (含 eventToUIMessageChunk), tools/retrieve.go, callbacks/grounding.go, callbacks/classifier.go。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q4 | A | 事件 adapter + ctx 共享 | ✓ |
| Q4 | B | Runner.Cancel 主 path | |
| Q4 | C | 放弃 chatHandler sync | |

**User's choice:** A — 事件 adapter + ctx 共享
**Notes:** Runner.AsyncIterator → `internal/agent/runner_sse.go` adapter → 现有 chunked SSE writer；Runner 与 SSE writer 共享 chatHandler ctx，cancel = ctx.Done() 同时断两边。TestCancelPropagationMidTool <200ms 锁零冲击；D-04 transport-swap + D15 wire-type compile-time alarm 不破。

---

## Area 2: writes: 代码级强制层级（HITL-05）

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q1 | A | 仅加载期校验 | |
| Q1 | **B** | **仅运行时动态 whitelist** | ✓ |
| Q1 | C | 加载 + 运行两层（Defense-in-Depth） | |
| Q1 | D | 三层全上 | |

**User's choice:** B — 仅运行时动态 whitelist
**Notes:** Agent instance per-session 单例；每轮 Construction 按 active_skill.writes ⊆ AllowedWriteTargets 拼 Tool Registry；HITL-05 "agent 无法绕过" 是结构性能力不是加载期字符串校验。Rejects C/D：HITL-04 frozen snapshot 路径已有冗余 defense，无需叠加 enforcement layer。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q2 | A | 共享 ApprovalDecision + target 字段 | ✓ |
| Q2 | B | 每 target 专属类型 | |
| Q2 | C | interface 适配 | |

**User's choice:** A — 共享 ApprovalDecision + target 字段
**Notes:** ApprovalDecision { Decision, Reason, Target, FrozenArgs json.RawMessage }；所有 HITL Tool file 同 schema；Phase 12 plan approval 加 Tool 不需新 schema type。FrozenArgs RawMessage 统一存储，tool-specific grounding 在 Handler 内做。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q3 | A | 编译期允许表 + 路由 map（推荐） | ✓ |
| Q3 | B | 包 init() 注册 | |
| Q3 | C | Skill manifest 动态读取 | |

**User's choice:** A — 编译期允许表 + 路由 map
**Notes:** `internal/agent/hitl/registry.go` 单文件 source of truth：AllowedWriteTargets + hitlHandlers map；三场共享（Agent Construction whitelist / approve-and-remember rule matching / Phase 12 plan approval add-point）。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q4 | v1 | per-Skill Agent instance | (REJECTED after user pushback) |
| Q4 | v2 | per-Skill Activation 新 Agent instance | |
| Q4 | **v3** | **不允许切换，取消才能切**（推荐） | ✓ |

**User's choice:** v3 — 不允许切换，取消才能切
**Notes:** Q4 第一次问时用"per-Skill Agent instance 生命周期"抽象表述，user 推 back "我没懂，在 claudecode 中同一个回话使用不同 skill 很常见，这里为什么会有问题"。Reframing 后 user 选 v3：Agent per-session 单例 + per-turn Tool whitelist 重拼；pending HITL 时 Skill picker 禁用切换。Claude Code 类比简化路径（无 Skill state machine）。

---

## Area 3: 跨刷新审批态恢复形态（RF-3 主轴）

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q1 | A | Dual 端点 | ✓ |
| Q1 | B | 合成 RESUME user message | |
| Q1 | C | SSE sticky session | |

**User's choice:** A — Dual 端点（chat history + approval）
**Notes:** 走 RF-3 旗标首选项。chat history 走 /api/session/:id/history（Phase 10 已锁）+ 新建 GET /api/session/:id/approval + 新建 POST /api/session/:id/approval/:approvalId。ApprovalCard state 派生自 store（D14 不破），不污染 SSE channel。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q2 | A | Snapshot 双表落 | ✓ |
| Q2 | B | audit 是唯一 source | |
| Q2 | C | 不要过期，prompt_hash 不检查 | |

**User's choice:** A — Snapshot 双表落
**Notes:** checkpoint 表 + audit.action_pending_approval 行均带 skill_version + prompt_hash 两列（D8-03 已预留 3 列基础）；resume 时双表 snapshot vs current Skill manifest + RenderSystemPrompt().Hash() 比对，不匹配 DELETE checkpoint + UI expired state。Defense-in-depth at storage layer 而非 runtime interceptor。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q3 | A | Interrupt 发起时同步写（推荐） | ✓ |
| Q3 | B | checkpoint 后异步写 | |
| Q3 | C | GET 时 lazy 写 | |

**User's choice:** A — Interrupt 发起时同步写（fail-closed）
**Notes:** tool.StatefulInterrupt 返回前同步写 audit action_pending_approval 行；失败则 Tool 拒绝执行。D8-06 SC-5 stub Hermetic 实证这条路径；HITL-04 "写路径 fail-closed" 不变明示。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q4 | A | approval_id == tool_call_id（推荐） | ✓ |
| Q4 | B | approval_id 独立 + crossref | |
| Q4 | C | approval_id = hash(args + skill + ts) | |

**User's choice:** A — approval_id == tool_call_id
**Notes:** tool_call_id = audit PK = POST 路由 PK = ApprovalCard state.key。跨设备 race = 同 ID 并发由 audit PK UNIQUE 约束天然兜底；HITL-04 "tool_call_id 幂等键" 一句话贯穿三层。

---

## Area 4: approve-and-remember 规则形态（HITL-08）

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q1 | A | target + hash(frozenArgs) + skill_id（推荐） | ✓ |
| Q1 | B | 仅 target name | |
| Q1 | C | target + JQ / JSON Path 表达式 | |

**User's choice:** A — target + hash(frozenArgs) + skill_id
**Notes:** HITL-07 edit-before-approve 改 frozen_args → rule key 与 frozen snapshot 一一对应；hash 撞概率 0；与 HITL-04 frozen snapshot 不变路径兼容。Rejects B：违反 HITL-02 "审批绑定结构化 tool call (name+args)"。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q2 | A | 独立 approval_rules 表（推荐） | ✓ |
| Q2 | B | 复用 audit.action_approved 行 | |
| Q2 | C | session_turn 派生 | |

**User's choice:** A — 独立 approval_rules 表
**Notes:** 新表 PK = (target, args_hash, skill_id, scope_session_id)；含 scope / created_at / expires_at? 列；T-3-AUDIT INSERT-only 不破（不污染 audit 表为 rule store）。Agent Construction 前查 rule 表表决 whitelist 中 target 的 auto-pass 子集。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q3 | A | session / skill 二元（推荐） | ✓ |
| Q3 | B | Skill-only 默认 | |
| Q3 | C | 三 scope 含 global | |
| Q3 | D | Skill manifest 决定默认 scope | |

**User's choice:** A — session / skill 二元
**Notes:** UI radio button 让用户选 scope；session 规则 PK 中 scope_session_id = current session_id；skill 规则 scope_session_id = "" + skill_id 共享跨 session。Rejects C: global 违反 HITL-05 Skill 限 writes 不变；Rejects D: manifest 字段扩张归 Phase 12。

| Q | Option | Description | Selected |
|---|--------|-------------|----------|
| Q4 | A | 左 sidebar Rule panel + audit rule_revoked（推荐） | ✓ |
| Q4 | B | Skill 面板内 | |
| Q4 | C | Session 内派生 | |
| Q4 | D | panel + Skill reload auto-expire（推荐+） | |

**User's choice:** A — 左 sidebar Rule panel + audit rule_revoked
**Notes:** 复用 Phase 5 5-07 LeftSidebar 4 section 架构 + 5-07 `item.manifest.description` BLOCKING CONSTRAINT 同样的 block const pattern；list 列 (target/scope/created_at/last_used_at)；每行 X 图标 DELETE row + INSERT audit `action_rule_revoked` 行。T-3-AUDIT INSERT-only 不破。Rejects D：v0.8 不强制 Skill reload auto-clean。

---

## Claude's Discretion

- **`memory:facts` 与 `review_report` 两个 write target 的具体 payload schema 字段**（HITL-06 — executor 实施时定）
- **prompt_hash 具体 hash 算法**（默认 sha256(RenderSystemPrompt())；若调研发现 skill hot reload 有 fingerprint 字段可直接复用亦可）
- **approval_rules 表 expires_at 列是否真带自动清理**（Area 4 Q4 locked path 上的优化项，不强制 v0.8 落地）
- **Audit `action_rule_revoked` 行的 audit schema 形态**（列集合 executor 实施时定）

---

## Deferred Ideas

详见 `11-CONTEXT.md` §Deferred Ideas。摘要：
- Phase 12 PLAN-03 计划确认门复用 HITL Tool 审批模式（scaffold only）
- Phase 13 FAN-04 子 agent per-sub-agent grounding（复用 AfterModelCallback 模式）
- frozen eval baseline v2 重锁（v0.8 milestone 收尾统一做）
- Skill hot reload 时 approval_rules auto-clean（优化项）
- Runner 化 SC-2/3/4/5/6 + Phase 10 spike hard-gate 重测作为 Phase 11 acceptance gate
- Phase 7 遗留（07-02 T3 / 07-05 T2/T3 / 07-06）用户决策推迟到 v0.8 后

---

## Resolutions after user clarification

| Round | Issue | Resolution |
|-------|-------|-----------|
| Area 1 Q2 | User requested web search before answering | Researched Context7 eino Runner API + tool.StatefulInterrupt architecture constraint; reformulated as "信任楼梯三层" |
| Area 2 Q4 | User pushed back on Per-Skill Agent lifecycle framing | Reformulated: Agent per-session single instance + per-turn Tool whitelist recompute; Skill switch blocked during pending HITL |

---

*Audit trail captured: 2026-08-20 /gsd:discuss-phase 11 — Default mode, 4 areas completed, 16 decisions locked.*
