# Requirements: SkillForge

**Defined:** 2026-08-10
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Skill System (SYSK)

- [ ] **SYSK-01**: User can create a Skill from a declarative `SKILL.md` file in a Skill directory
- [ ] **SYSK-02**: System parses and validates `SKILL.md` against the **7-field** schema (`description` / `agent` / `reads` / `writes` / `steps` / `guardrails` / **`temperature`**) — see `.planning/SKILL_SCHEMA.md`
- [ ] **SYSK-03**: User can browse and select installed Skills via the UI
- [x] **SYSK-04**: System hot-reloads Skills on file change (fsnotify)
- [x] **SYSK-05**: System routes user queries to the right Skill via intent matching on `description`
- [x] **SYSK-06**: User can describe a Skill need in natural language and the **Skill Wizard** auto-generates a compliant `SKILL.md`
- [ ] **SYSK-07**: System enforces `reads` field as a hard filter at retrieval time (retriever cannot query outside declared tiers)
- [ ] **SYSK-08**: System enforces `temperature` field as **required** (no default) and passes it to LLM calls via Eino `GenerateOption(temperature)` — supports per-Skill determinism level (compliance Skills use 0.0, creative Skills use 0.7+)

### Agent Core (AGNT)

- [ ] **AGNT-01**: System orchestrates multi-agent workflows via Eino ADK (`ChatModelAgent` / `SequentialAgent` / `ParallelAgent`)
- [ ] **AGNT-02**: Agent can invoke external tools (function calling)
- [ ] **AGNT-03**: System streams agent events (text / reasoning / tool-call / source) to client in real-time via Hertz SSE
- [ ] **AGNT-04**: User can cancel an in-flight agent run
- [x] **AGNT-05**: System maintains per-session conversation memory (with no cross-session bleed)
- [x] **AGNT-06**: User can inspect live agent state (turns / tools / retrievals) via the UI

### Knowledge Layer (KNOW)

- [x] **KNOW-01**: System supports 4-tier knowledge: 权威 / 历史 / 个人 / 草稿 (each chunk tagged with `tier`)
- [ ] **KNOW-02**: System indexes documents into Milvus with mandatory `tier` metadata (insert fails if tier missing)
- [ ] **KNOW-03**: Retrieval filters by `tier` BEFORE similarity search (per Skill's `reads` declaration)
- [ ] **KNOW-04**: System defaults to refusing when no tier-matched docs found (refusal-by-construction)
- [ ] **KNOW-05**: UI displays citation tier alongside each citation (`[权威]` vs `[历史]` etc.)
- [x] **KNOW-06**: User can browse the knowledge graph (tier + doc + chunk relationships) in the UI

### Agentic RAG (ARAG)

- [x] **ARAG-01**: Agent autonomously decides to retrieve, ask clarifying, refuse, or answer
- [ ] **ARAG-02**: System supports multi-hop retrieval (agent retrieves additional chunks based on partial answer)
- [ ] **ARAG-03**: System supports Self-RAG / CRAG self-verification — agent re-reads retrieved docs and verifies each claim before final answer
- [ ] **ARAG-04**: Every cited claim is a chunk ID + span (not free text); unverifiable claims are stripped or refused
- [ ] **ARAG-05**: Refusal includes structured reason `{reason, can_user_rephrase, suggested_alternative_query}`

### Guardrails & Eval (GEVL)

- [ ] **GEVL-01**: System refuses with structured reason when retrieval score < threshold
- [ ] **GEVL-02**: System refuses when no tier-matched docs exist (refusal-by-construction; no improvising)
- [x] **GEVL-03**: Every query logs audit record: `{ts, user, query, skill, chunk_ids[], model_ver, prompt_hash, answer, refusal_reason}`
- [x] **GEVL-04**: System integrates Ragas eval (faithfulness, context precision, answer relevance) as a sidecar Python service
- [x] **GEVL-05**: System maintains per-Skill eval dataset (≥50 cases) with adversarial / refusal-required / multi-hop queries
- [ ] **GEVL-06**: Eval dashboard surfaces per-Skill metrics (faithfulness / citation precision / refusal accuracy) with trend over time
- [ ] **GEVL-07**: CI gating on faithfulness < 0.9 fails Skill promotion

### Frontend (FEUI)

- [x] **FEUI-01**: User can chat with the agent and see streaming responses (React 19 + Vercel AI SDK v5)
- [x] **FEUI-02**: Reasoning chain is collapsible and visible by default (typed message parts: reasoning / tool-invocation / text / source)
- [ ] **FEUI-03**: Citations are clickable links opening side panel with retrieved chunk verbatim + tier + version + effective date
- [ ] **FEUI-04**: User can switch between installed Skills via a skill picker
- [ ] **FEUI-05**: Refusal UX distinguishes "out-of-scope" from "I don't know" with structured reason + suggested rephrasing
- [x] **FEUI-06**: Loading state shows sub-progress during tool calls ("Searching 权威 tier... Found 12 candidates... Re-ranking...")

### Open-Source Distribution (DIST)

- [x] **DIST-01**: README with badges, value prop, demo GIF, quick start, architecture diagram, contributing, license
- [x] **DIST-02**: Apache-2.0 license file present from day 1
- [x] **DIST-03**: `.env.example` with every required variable
- [ ] **DIST-04**: `docker-compose up` brings up full stack (Go backend + Milvus + frontend + pluggable embedding endpoint)
- [ ] **DIST-05**: Recorded demo GIF in README hero (cited verdict + structured refusal flow) — D7-06 reversed the live-demo decision
- [x] **DIST-06**: Pre-bundled sample compliance docs (works without API key for retrieval)
- [x] **DIST-07**: `CONTRIBUTING.md` with "how to write a Skill", "add a tool", "add a knowledge tier"
- [x] **DIST-08**: `examples/` folder with 2-3 Skill templates (compliance, simple QA, calculator)
- [ ] **DIST-09**: Published eval results in README (faithfulness %, citation precision %, refusal accuracy %)
- [x] **DIST-10**: Eval set sample included in repo

### Compliance Demo Skill (CMPL)

- [ ] **CMPL-01**: System includes a built-in `企业合规/制度审查` Skill as the showcase
- [ ] **CMPL-02**: Compliance Skill outputs structured verdict `{status: 合规 | 不合规 | 需补充, confidence, citations, review_required}`
- [ ] **CMPL-03**: Compliance Skill requires human review (`review_required: true`) when confidence < 0.9 OR claim is novel
- [ ] **CMPL-04**: Compliance Skill reads ONLY 权威 tier (hard filter via `reads`; no 其他 tier bleed)
- [ ] **CMPL-05**: UI displays compliance verdict with `needs human review` badge when triggered
- [x] **CMPL-06**: Counter-evidence highlighting — surface chunks that contradict the agent's verdict
- [x] **CMPL-07**: Redline suggestion — propose clause-level rewrites when policy gap detected
- [x] **CMPL-08**: Policy version tracking (chunk metadata `effective_from` / `effective_to`; superseded versions filtered out)

### Financial Data Layer (DATA)

> Added 2026-08-10 via /gsd:explore (finance compliance data architecture). Map to Phase 5+ (architecture integration TBD).

- [ ] **DATA-01**: System provides a data import pipeline for data personnel to ingest CSV / SQL dump / API data with tier + schema metadata
- [ ] **DATA-02**: System enforces mandatory desensitization at import time (L1 PII hashing + L2 bucketing) — raw L1/L2/L3 data does NOT persist in SkillForge for aggregation-tier Skills
- [ ] **DATA-03**: System exposes aggregation API (predefined KPI functions) — agent invokes KPI functions, never SELECTs raw data
- [ ] **DATA-04**: System supports external authorized channel for individual-identification Skills — raw data stays in user's enterprise data warehouse, agent calls via read-only DB user / proxy with TTL token
- [ ] **DATA-05**: External authorized channel enforces field-level second-layer desensitization (e.g., display only last-4 of account number) plus pre-authorization by data personnel
- [ ] **DATA-06**: System maintains dual audit log: internal (SkillForge query log) + external (data warehouse query log), reconcilable by query ID + timestamp
- [ ] **DATA-07**: Skill's `reads` field supports two new value patterns: `data:aggregated_only` and `data:external:authorized`
- [ ] **DATA-08**: System rejects any agent query that violates the Skill's data channel (e.g., aggregation skill trying to call external channel) — guardrail at platform layer
- [ ] **DATA-09**: Aggregation channel: agent can ONLY invoke predefined KPI functions / views — no SQL, no new KPI definition (NL2SQL safety layer 1)
- [ ] **DATA-10**: Individual channel: agent queries go through predefined views (not raw tables) — DB user restricted (read-only, limited schema, field-level view masking) (NL2SQL safety layer 2)
- [ ] **DATA-11**: Agent must generate SQL preview (NL2SQL output) BEFORE entering approval flow for individual-identification Skills (NL2SQL safety layer 3 — data officer audits the actual query intent)
- [ ] **DATA-12**: Rejection must include structured reason `{reason_code, human_explanation}` (compliance audit requirement)
- [ ] **DATA-13**: Approval timeout = 持续挂起 (no auto-reject, no auto-timeout) — each query must be explicitly approved or rejected by data officer
- [ ] **DATA-14**: Every approval creates audit row: `{query_id, user_id, business_reason, sql_preview, approve_or_reject_result, data_officer_id, ttl_token_id, timestamp, chunk_ids_returned}`

## v0.8 Requirements (Harness Milestone — 可信×行动融合)

> Added 2026-08-17 via /gsd:new-milestone. Research: `.planning/research/SUMMARY.md`. 贯穿性不变量：**citation 白名单 = 本轮检索结果，代码强制**——历史/摘要/子 agent 输出/resume 注入的文本永远不能成为证据。

### Foundation (PLAT / GEVL)

- [x] **PLAT-04**: eino v0.7.7 → v0.9.14 升级（用户决策 2026-08-17：v0.9.14 覆盖研究期 v0.9.13；不用 v0.10.0-alpha.x）+ eino-ext 逐组件联动升级；回归门 = `integration/contract` 漂移套件 + 冻结 eval 基线全绿 + Phase-1 live verify 重新锁定（cancel <200ms、Milvus tier-filter）；legacy interrupt API lint 禁用
- [x] **PLAT-05**: sqlite `CheckPointStore` 持久化实现（复用 `internal/store/sqlite`；所有自定义状态类型 `schema.RegisterName[T](name)`（其内部完成 gob.RegisterName 双注册——RESEARCH Pitfall 1 修正）；真实中断运行往返测试）
- [x] **GEVL-08**: 历史注入回归硬门——golden set A/B（`history=nil` vs 投影历史 × 善意追问/恶意注入/旧拒答上下文），四指标漂移 <5% 且 refusal delta <10%；不过门则记忆不得发布，产出消毒规则或逃逸方案（last-3-turns / prompt 附录 / 仅用户轮记忆）

### Multi-turn Memory (MEM)

- [ ] **MEM-01**: 用户可进行追问指代（「那第二条呢」「依据是什么」），agent 结合会话历史正确解析并作答
- [ ] **MEM-02**: 会话历史跨页面刷新存活并喂给 agent（server 层 `internal/history` 投影 sqlite session → `[]schema.Message`）
- [ ] **MEM-03**: citation 白名单代码强制——citation 只能引用**本轮** Run 检索到的 chunk；历史/摘要/resume 注入文本物理上不能成为证据
- [ ] **MEM-04**: 历史投影消毒——投影时剥离 `[CHUNK id]` 标记，verdict JSON 经 `conclusionText`/`refusalText` 确定性渲染为纯文本
- [ ] **MEM-05**: 长会话 token 预算 + 摘要压缩（先哑预算，summarization 中间件在 Runner 迁移后启用；project→then→summarize 顺序）

### Write Tools + Approval Gate (HITL)

- [ ] **HITL-01**: agent 调用写工具前中断（Eino Interrupt），流内呈现内联审批卡（what / tool / 完整参数快照 / why），SSE 流干净关闭
- [ ] **HITL-02**: 用户可批准一次 / 拒绝（拒绝必填理由）；审批绑定结构化工具调用（name+args），永不绑定原始字符串
- [ ] **HITL-03**: 审批态跨刷新/重启存活（sqlite checkpoint + GET resume 端点恢复）；checkpoint 绑定 skill 版本 + prompt_hash，过期 checkpoint 有显式 UI 态
- [ ] **HITL-04**: 每个审批决策产生 INSERT-only 审计三行生命周期（`action_pending_approval` → `action_approved` / `action_rejected`）；写路径审计 **fail-closed**（与读路径 swallow 相反的显式 Key Decision）；interrupt 时冻结参数快照进 pending 行（批的 A 不会执行成 B）；tool_call_id 幂等键
- [ ] **HITL-05**: `writes:` frontmatter 代码级强制——agent 无法调用其 Skill 未声明的写工具
- [ ] **HITL-06**: 首批两个写工具落地：memory:facts 写入 + 审查报告生成
- [ ] **HITL-07**: 用户可在批准前修改工具参数（edit-before-approve，LangGraph edit 决策）
- [ ] **HITL-08**: 用户可「批准并记住」（approve-and-remember），规则按会话/Skill 级作用域存储，可查看可撤销

### Planning Layer (PLAN)

- [ ] **PLAN-01**: SKILL.md `agent: deep` 启用 DeepAgent archetype（`adk/prebuilt/deep` + write_todos），经 `internal/runtime` 工厂按 `Manifest.Agent` 分发；所有 archetype 统一经 `ApplyTrustStage`（floor→grounding→critique）出口
- [ ] **PLAN-02**: 用户可看到实时计划清单流式更新（todo pending/in_progress/completed），计划随会话持久化
- [ ] **PLAN-03**: 计划确认门——DeepAgent 执行前用户批准计划（复用 HITL 审批机制，Gemini 模式）
- [ ] **PLAN-04**: 三层预算门——MaxIteration ≤10 + run 级 token 预算 + 回路检测；触发即保守收尾（forced_stop），预算事件进审计
- [ ] **PLAN-05**: 可回放规划轨迹——write_todos 审计行可完整重建某次运行的计划演变，UI 可回放

### Sub-agent Fan-out (FAN)

- [ ] **FAN-01**: orchestrator 可将多跳检索扇出为并行子 agent（一层为限，禁止递归扇出），子 agent 返回压缩结果、故障隔离
- [ ] **FAN-02**: tier 边界构造期闭包继承——子 agent 的 retrieve 工具闭包自父 manifest 的 `ResolvedTiers()`，结构性无法扩权；交集为空 = 加载期校验错误
- [ ] **FAN-03**: 合并确定性（key 排序纯函数，契约测试 10× 一致）+ 保守部分失败合并（任一子 agent 拒答 → 合并结论最多「需补充」，绝不肯定）
- [ ] **FAN-04**: per-sub-agent 合并前 grounding（复用 D2-01 逐字引用验证护城河）；扇出事件（RunPath/Transfer）写入审计

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Platform Capabilities

- **PLAT-01**: Skill marketplace with version pinning and dependency resolution
- **PLAT-02**: Multi-user real-time collaborative Skill editing
- **PLAT-03**: Cross-Skill chaining (compose Skill A → Skill B pipelines)
- **PLAT-06**: Cross-session semantic long-term memory（需先做隐私设计；v0.8 只做会话内记忆）
- **PLAT-07**: Async / webhook 审批委托（需身份 + 投递基础设施；v0.8 只做会话内同步审批）
- **PLAT-08**: 递归子 agent / 多会话任务板（需成本与审计模型；v0.8 子 agent 限一层）

### Scale & Production

- **SCALE-01**: Multi-tenant deployment with per-tenant Skill libraries
- **SCALE-02**: K8s / Helm chart for enterprise SSO deployment
- **SCALE-03**: Cost / token dashboard with per-Skill usage analytics

### Domain Skills (sample, not exhaustive)

- **DOMAIN-01**: Codebase comprehension Skill (reads source tree + ADRs + commit history)
- **DOMAIN-02**: Decision-retrospective Skill (reads meeting notes + decision logs)
- **DOMAIN-03**: Industry compliance packs (financial / medical / manufacturing)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-tenant SaaS (billing / subscription / user mgmt) | v1 聚焦 agent 能力本身，不做商业化平台层 |
| Desktop-grade desktop app (denova-style binary) | v1 聚焦 Web 应用形态 |
| Full private-deployment ops (K8s / Helm / enterprise SSO) | v1 给本地启动 + Docker，运维套件后续 |
| Real-time multi-user Skill editing | v1 假设单人 / 小团队维护 Skill |
| Bypass / YOLO 审批模式（自动批准所有写操作） | 与合规定位根本冲突，永不发布（FEATURES.md anti-feature 结论，2026-08-17） |
| Custom LLM training / fine-tuning | 只做 agent 编排与工程，不做模型训练 |
| Native mobile app | Web + PWA 足够，不做原生 |
| Self-hosted LLM training infrastructure | Out of core value; rely on OpenAI-compatible APIs |
| Height.app-style "AI wrapper" pattern | Closed 2025.9; prove proprietary value via Skill schema + eval |

## Traceability

Which phases cover which requirements.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SYSK-01 | Phase 2 | Pending |
| SYSK-02 | Phase 2 | Pending |
| SYSK-03 | Phase 4 | Pending |
| SYSK-04 | Phase 5 | Complete |
| SYSK-05 | Phase 5 | Complete |
| SYSK-06 | Phase 5 | Complete |
| SYSK-07 | Phase 2 | Pending |
| SYSK-08 | Phase 2 | Pending |
| AGNT-01 | Phase 3 | Pending |
| AGNT-02 | Phase 3 | Pending |
| AGNT-03 | Phase 2 | Pending |
| AGNT-04 | Phase 3 | Pending |
| AGNT-05 | Phase 3 | Complete |
| AGNT-06 | Phase 5 | Complete |
| KNOW-01 | Phase 3 | Complete |
| KNOW-02 | Phase 3 | Pending |
| KNOW-03 | Phase 3 | Pending |
| KNOW-04 | Phase 3 | Pending |
| KNOW-05 | Phase 4 | Pending |
| KNOW-06 | Phase 5 | Complete |
| ARAG-01 | Phase 3 | Complete |
| ARAG-02 | Phase 3 | Pending |
| ARAG-03 | Phase 3 | Pending |
| ARAG-04 | Phase 2 | Pending |
| ARAG-05 | Phase 2 | Pending |
| GEVL-01 | Phase 2 | Pending |
| GEVL-02 | Phase 2 | Pending |
| GEVL-03 | Phase 3 | Complete |
| GEVL-04 | Phase 3 | Complete |
| GEVL-05 | Phase 3 | Complete |
| GEVL-06 | Phase 6 | Pending |
| GEVL-07 | Phase 6 | Pending |
| FEUI-01 | Phase 4 | Complete |
| FEUI-02 | Phase 4 | Complete |
| FEUI-03 | Phase 4 | Pending |
| FEUI-04 | Phase 4 | Pending |
| FEUI-05 | Phase 4 | Pending |
| FEUI-06 | Phase 4 | Complete |
| DIST-01 | Phase 7 | Complete |
| DIST-02 | Phase 7 | Complete |
| DIST-03 | Phase 7 | Complete |
| DIST-04 | Phase 7 | Pending |
| DIST-05 | Phase 7 | Pending |
| DIST-06 | Phase 7 | Complete |
| DIST-07 | Phase 7 | Complete |
| DIST-08 | Phase 7 | Complete |
| DIST-09 | Phase 7 | Pending |
| DIST-10 | Phase 7 | Complete |
| CMPL-01 | Phase 2 | Pending |
| CMPL-02 | Phase 2 | Pending |
| CMPL-03 | Phase 2 | Pending |
| CMPL-04 | Phase 2 | Pending |
| CMPL-05 | Phase 2 | Pending |
| CMPL-06 | Phase 5 | Complete |
| CMPL-07 | Phase 5 | Complete |
| CMPL-08 | Phase 5 | Complete |
| DATA-01 | Phase 5 | Pending |
| DATA-02 | Phase 5 | Pending |
| DATA-03 | Phase 5 | Pending |
| DATA-04 | Phase 5 | Pending |
| DATA-05 | Phase 5 | Pending |
| DATA-06 | Phase 5 | Pending |
| DATA-07 | Phase 5 | Pending |
| DATA-08 | Phase 5 | Pending |
| DATA-09 | Phase 5 | Pending |
| DATA-10 | Phase 5 | Pending |
| DATA-11 | Phase 5 | Pending |
| DATA-12 | Phase 5 | Pending |
| DATA-13 | Phase 5 | Pending |
| DATA-14 | Phase 5 | Pending |
| PLAT-04 | Phase 8 | Complete |
| PLAT-05 | Phase 8 | Complete |
| GEVL-08 | Phase 9 | Complete |
| MEM-01 | Phase 10 | In Progress (10-04 Task 1 shipped; live gate pending operator stack) |
| MEM-02 | Phase 10 | In Progress (10-04 Task 1 shipped; live gate pending operator stack) |
| MEM-03 | Phase 10 | In Progress (10-04 Task 1 shipped; live gate pending operator stack) |
| MEM-04 | Phase 10 | In Progress (10-04 Task 1 shipped; live gate pending operator stack) |
| MEM-05 | Phase 10 | In Progress (10-04 Task 1 shipped; live gate pending operator stack) |
| HITL-01 | Phase 11 | Pending |
| HITL-02 | Phase 11 | Pending |
| HITL-03 | Phase 11 | Pending |
| HITL-04 | Phase 11 | Pending |
| HITL-05 | Phase 11 | Pending |
| HITL-06 | Phase 11 | Pending |
| HITL-07 | Phase 11 | Pending |
| HITL-08 | Phase 11 | Pending |
| PLAN-01 | Phase 12 | Pending |
| PLAN-02 | Phase 12 | Pending |
| PLAN-03 | Phase 12 | Pending |
| PLAN-04 | Phase 12 | Pending |
| PLAN-05 | Phase 12 | Pending |
| FAN-01 | Phase 13 | Pending |
| FAN-02 | Phase 13 | Pending |
| FAN-03 | Phase 13 | Pending |
| FAN-04 | Phase 13 | Pending |

**Coverage:**

- v1 requirements: 70 total (was 55; +8 DATA +1 SYSK-08 +6 DATA-09..14 added via /gsd:explore 2026-08-10/11)
- v1 mapped to phases: 70
- v0.8 requirements: 25 total (added 2026-08-17 via /gsd:new-milestone)
- v0.8 mapped to phases: 25 (phases 8-13)
- Unmapped: 0 ✓

**Phase distribution:**

| Phase | Count | Requirements |
|-------|-------|--------------|
| Phase 1 (Spike) | 0 | (none — integration validation only) |
| Phase 2 (Compliance Vertical Slice) | 13 | SYSK-01, SYSK-02, SYSK-07, AGNT-03, CMPL-01..05, ARAG-04, ARAG-05, GEVL-01, GEVL-02 |
| Phase 3 (Agentic Retrieval & Trust Evaluation) | 14 | AGNT-01, AGNT-02, AGNT-04, AGNT-05, ARAG-01..03, KNOW-01..04, GEVL-03, GEVL-04, GEVL-05 |
| Phase 4 (Streaming Trust UI) | 8 | SYSK-03, KNOW-05, FEUI-01..06 |
| Phase 5 (Skill Platform & Full Knowledge Boundaries) | 8 | SYSK-04, SYSK-05, SYSK-06, AGNT-06, KNOW-06, CMPL-06, CMPL-07, CMPL-08 |
| Phase 6 (Eval Dashboard & Production Hardening) | 2 | GEVL-06, GEVL-07 |
| Phase 7 (Open-Source Release) | 10 | DIST-01..10 |
| **v1 Total** | **55** | |
| Phase 8 (全量依赖升级 P8.0) | 2 | PLAT-04, PLAT-05 |
| Phase 9 (历史注入 spike 硬门 P8.S) | 1 | GEVL-08 |
| Phase 10 (多轮对话记忆 P8.1) | 5 | MEM-01..05 |
| Phase 11 (写工具 + 审批门 P8.2) | 8 | HITL-01..08 |
| Phase 12 (规划层 P8.3) | 5 | PLAN-01..05 |
| Phase 13 (子 agent 扇出 P8.4) | 4 | FAN-01..04 |
| **v0.8 Total** | **25** | |

---
*Requirements defined: 2026-08-10*
*Last updated: 2026-08-17 — v0.8 milestone requirements (25) mapped to phases 8-13; traceability + distribution updated*
