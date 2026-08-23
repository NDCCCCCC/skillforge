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
- [x] **DIST-09**: Published eval results in README (faithfulness %, citation precision %, refusal accuracy %)
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

> Added 2026-08-10 via /gsd:explore (finance compliance data architecture). **v1.0 归并（2026-08-21）**: DATA-01/02/03/07/08/09/12 激活进 v1.0 milestone（映射由 v1.0 路线图定义）；DATA-04/05/06/10/11/13/14 延后 v1.1（外部授权通道全套，PROJECT.md v1.0 决策）。

- [x] **DATA-01**: System provides a data import pipeline for data personnel to ingest CSV (incl. GBK/GB18030 + BOM decode) / SQL dump / API data with tier + schema metadata — v1.0
- [x] **DATA-02**: System enforces mandatory desensitization at import time (L1 PII hashing with pre-hash normalization + L2 bucketing) — raw L1/L2/L3 data does NOT persist in SkillForge for aggregation-tier Skills — v1.0
- [ ] **DATA-03**: System exposes aggregation API (predefined KPI functions) — agent invokes KPI functions, never SELECTs raw data — v1.0
- [ ] **DATA-04**: System supports external authorized channel for individual-identification Skills — raw data stays in user's enterprise data warehouse, agent calls via read-only DB user / proxy with TTL token **→ v1.1**
- [ ] **DATA-05**: External authorized channel enforces field-level second-layer desensitization (e.g., display only last-4 of account number) plus pre-authorization by data personnel **→ v1.1**
- [ ] **DATA-06**: System maintains dual audit log: internal (SkillForge query log) + external (data warehouse query log), reconcilable by query ID + timestamp **→ v1.1**
- [ ] **DATA-07**: Skill's `reads` field supports two new value patterns: `data:aggregated_only` and `data:external:authorized` — v1.0 仅实现 `data:aggregated_only`（external 模式 → v1.1）
- [ ] **DATA-08**: System rejects any agent query that violates the Skill's data channel (e.g., aggregation skill trying to call external channel) — guardrail at platform layer — v1.0
- [ ] **DATA-09**: Aggregation channel: agent can ONLY invoke predefined KPI functions / views — no SQL, no new KPI definition (NL2SQL safety layer 1) — v1.0
- [ ] **DATA-10**: Individual channel: agent queries go through predefined views (not raw tables) — DB user restricted (read-only, limited schema, field-level view masking) (NL2SQL safety layer 2) **→ v1.1**
- [ ] **DATA-11**: Agent must generate SQL preview (NL2SQL output) BEFORE entering approval flow for individual-identification Skills (NL2SQL safety layer 3 — data officer audits the actual query intent) **→ v1.1**
- [ ] **DATA-12**: Rejection must include structured reason `{reason_code, human_explanation}` (compliance audit requirement; v1.0 具体化为 `no_matching_kpi` 等 reason code) — v1.0
- [ ] **DATA-13**: Approval timeout = 持续挂起 (no auto-reject, no auto-timeout) — each query must be explicitly approved or rejected by data officer **→ v1.1**
- [ ] **DATA-14**: Every approval creates audit row: `{query_id, user_id, business_reason, sql_preview, approve_or_reject_result, data_officer_id, ttl_token_id, timestamp, chunk_ids_returned}` **→ v1.1**

## v0.8 Requirements (Harness Milestone — 可信×行动融合)

> Added 2026-08-17 via /gsd:new-milestone. Research: `.planning/research/SUMMARY.md`. 贯穿性不变量：**citation 白名单 = 本轮检索结果，代码强制**——历史/摘要/子 agent 输出/resume 注入的文本永远不能成为证据。

### Foundation (PLAT / GEVL)

- [x] **PLAT-04**: eino v0.7.7 → v0.9.14 升级（用户决策 2026-08-17：v0.9.14 覆盖研究期 v0.9.13；不用 v0.10.0-alpha.x）+ eino-ext 逐组件联动升级；回归门 = `integration/contract` 漂移套件 + 冻结 eval 基线全绿 + Phase-1 live verify 重新锁定（cancel <200ms、Milvus tier-filter）；legacy interrupt API lint 禁用
- [x] **PLAT-05**: sqlite `CheckPointStore` 持久化实现（复用 `internal/store/sqlite`；所有自定义状态类型 `schema.RegisterName[T](name)`（其内部完成 gob.RegisterName 双注册——RESEARCH Pitfall 1 修正）；真实中断运行往返测试）
- [x] **GEVL-08**: 历史注入回归硬门——golden set A/B（`history=nil` vs 投影历史 × 善意追问/恶意注入/旧拒答上下文），四指标漂移 <5% 且 refusal delta <10%；不过门则记忆不得发布，产出消毒规则或逃逸方案（last-3-turns / prompt 附录 / 仅用户轮记忆）

### Multi-turn Memory (MEM)

- [x] **MEM-01**: 用户可进行追问指代（「那第二条呢」「依据是什么」），agent 结合会话历史正确解析并作答
- [x] **MEM-02**: 会话历史跨页面刷新存活并喂给 agent（server 层 `internal/history` 投影 sqlite session → `[]schema.Message`）
- [x] **MEM-03**: citation 白名单代码强制——citation 只能引用**本轮** Run 检索到的 chunk；历史/摘要/resume 注入文本物理上不能成为证据
- [x] **MEM-04**: 历史投影消毒——投影时剥离 `[CHUNK id]` 标记，verdict JSON 经 `conclusionText`/`refusalText` 确定性渲染为纯文本
- [x] **MEM-05**: 长会话 token 预算 + 摘要压缩（先哑预算，summarization 中间件在 Runner 迁移后启用；project→then→summarize 顺序）

### Write Tools + Approval Gate (HITL)

- [x] **HITL-01**: agent 调用写工具前中断（Eino Interrupt），流内呈现内联审批卡（what / tool / 完整参数快照 / why），SSE 流干净关闭
- [x] **HITL-02**: 用户可批准一次 / 拒绝（拒绝必填理由）；审批绑定结构化工具调用（name+args），永不绑定原始字符串
- [x] **HITL-03**: 审批态跨刷新/重启存活（sqlite checkpoint + GET resume 端点恢复）；checkpoint 绑定 skill 版本 + prompt_hash，过期 checkpoint 有显式 UI 态
- [x] **HITL-04**: 每个审批决策产生 INSERT-only 审计三行生命周期（`action_pending_approval` → `action_approved` / `action_rejected`）；写路径审计 **fail-closed**（与读路径 swallow 相反的显式 Key Decision）；interrupt 时冻结参数快照进 pending 行（批的 A 不会执行成 B）；tool_call_id 幂等键
- [x] **HITL-05**: `writes:` frontmatter 代码级强制——agent 无法调用其 Skill 未声明的写工具
- [x] **HITL-06**: 首批两个写工具落地：memory:facts 写入 + 审查报告生成
- [x] **HITL-07**: 用户可在批准前修改工具参数（edit-before-approve，LangGraph edit 决策）
- [x] **HITL-08**: 用户可「批准并记住」（approve-and-remember），规则按会话/Skill 级作用域存储，可查看可撤销

### Planning Layer (PLAN)

- [x] **PLAN-01**: SKILL.md `agent: deep` 启用 DeepAgent archetype（`adk/prebuilt/deep` + write_todos），经 `internal/runtime` 工厂按 `Manifest.Agent` 分发；所有 archetype 统一经 `ApplyTrustStage`（floor→grounding→critique）出口
- [x] **PLAN-02**: 用户可看到实时计划清单流式更新（todo pending/in_progress/completed），计划随会话持久化
- [x] **PLAN-03**: 计划确认门——DeepAgent 执行前用户批准计划（复用 HITL 审批机制，Gemini 模式）
- [x] **PLAN-04**: 三层预算门——MaxIteration ≤10 + run 级 token 预算 + 回路检测；触发即保守收尾（forced_stop），预算事件进审计
- [x] **PLAN-05**: 可回放规划轨迹——write_todos 审计行可完整重建某次运行的计划演变，UI 可回放

### Sub-agent Fan-out (FAN)

- [x] **FAN-01**: orchestrator 可将多跳检索扇出为并行子 agent（一层为限，禁止递归扇出），子 agent 返回压缩结果、故障隔离
- [x] **FAN-02**: tier 边界构造期闭包继承——子 agent 的 retrieve 工具闭包自父 manifest 的 `ResolvedTiers()`，结构性无法扩权；交集为空 = 加载期校验错误
- [x] **FAN-03**: 合并确定性（key 排序纯函数，契约测试 10× 一致）+ 保守部分失败合并（任一子 agent 拒答 → 合并结论最多「需补充」，绝不肯定）
- [x] **FAN-04**: per-sub-agent 合并前 grounding（复用 D2-01 逐字引用验证护城河）；扇出事件（RunPath/Transfer）写入审计

## v1.0 Requirements (业务数据支撑 Milestone — Business Data Support)

> Added 2026-08-21 via /gsd:new-milestone. Research: `.planning/research/{STACK,FEATURES,ARCHITECTURE,PITFALLS,SUMMARY}.md` + `STACK-VERIFICATION.md`（对抗式依赖审计）。
> 贯穿性不变量：**审计行 INSERT-only 永不删；清理只作用于脱敏数据集；KPI 快照冻结在审计行（数据清了不影响回放）；数据引用 = 函数名+参数+时间窗+结果快照**（区别于文档 chunk 引用）。
> 既有 DATA 需求归并：DATA-01/02/03/07(仅 aggregated_only)/08/09/12 激活（定义见上方 Financial Data Layer 节）；DATA-04/05/06/10/11/13/14 → v1.1。
> 栈决策（2026-08-21）：go-duckdb v2.4.3（CGO）+ sqlite 驱动 CGO 化（modernc → mattn v1.14.50）——见 STACK-VERIFICATION.md §1/§2.7。

### 数据导入与脱敏 (DATA — new items)

- [x] **DATA-15**: 数据人员可进行两阶段导入——预览（前 100 行 + 拟脱敏效果 + 拟 tier，可调整字段桶边界）→ 确认提交
- [x] **DATA-16**: 幂等重导入——内容哈希 + (dataset, period) 去重，同批刷新重跑安全
- [x] **DATA-17**: 每导入批次产生 INSERT-only 审计行证明 raw 已弃 `{batch_id, raw_size_bytes, desensitized_row_count, discard_ts, dataset}`
- [ ] **DATA-18**: 数据集 TTL 到期清理——先 DETACH 后删文件（Windows 文件锁）；只作用于脱敏数据集文件，审计行与 KPI 快照永不删

### KPI 注册表 (KPI)

- [ ] **KPI-01**: KPI 注册表由数据人员以受控文件/代码定义，运行时与 agent 不可增改；admin UI 只读展示（签名 + 最后调用时间）
- [ ] **KPI-02**: 每次 KPI 调用产生数据引用 `{func, args, as_of, result_snapshot}` 冻结进审计行——上游数据清理后仍可完整回放

### KPI 仪表盘 Skill (DASH)

- [ ] **DASH-01**: 用户可在聊天中提问数字问题（如「2026-Q2 逾期率是多少」），KPI Skill 经函数调用作答
- [ ] **DASH-02**: 每个数字带完整出处——数值 + KPI 定义 + 计算时间窗 + as_of 时间戳 + 数据引用（hover 显示通道名 `data:aggregated_only`）
- [ ] **DASH-03**: 答案内嵌迷你趋势图（sparkline），每个数据点继承同一引用契约、独立可审计
- [ ] **DASH-04**: 用户可点击数字下钻（「看月度趋势」）——下钻 = 更多 KPI 调用，永不进入原始行
- [ ] **DASH-05**: 多 KPI 组合答案（「收入 + 客数 + 同比」）由 Skill steps 编排，各自独立引用

### 业务月报 Skill (RPT)

- [ ] **RPT-01**: 用户可在聊天中触发月报生成（「出 2026-08 月报」），DeepAgent 计划清单先行可见
- [ ] **RPT-02**: 计划确认门复用（PLAN-03 模式）——执行前用户批准将调用的 KPI 集与报告结构
- [ ] **RPT-03**: 报告模板 = Skill 一部分（steps 声明 KPI 顺序 + 叙事槽位）；LLM 填槽不自由发挥
- [ ] **RPT-04**: 报告发布 `report:publish` 是写工具，走 HITL 审批门（HITL-01..08 模式），产出静态页
- [ ] **RPT-05**: 用户可带修改重生成（「重做，更突出 Q2 风险」）——新一轮 DeepAgent run

### 静态报告门户 + 定时更新 (PORT)

- [ ] **PORT-01**: `/reports` 门户列出全部已发布报告，按 Skill / 周期 / 运行筛选，显示最后成功时间
- [ ] **PORT-02**: cron 计划 = Skill manifest 字段（`report_schedule`），加载期校验语法；改计划 = 改 Skill（热重载生效）
- [ ] **PORT-03**: 每次定时重渲染产生审计行 + 全量 KPI 快照 `{run_id, skill, cron_expr, fired_at, kpi_snapshots[], output_url, status}`
- [ ] **PORT-04**: 版本留存——每次重渲染新增 run 目录永不覆盖，历史报告可追溯
- [ ] **PORT-05**: 用户/管理员可手动触发「立即重渲染」（复用 DeepAgent 运行路径）
- [ ] **PORT-06**: 漂移检测——本次与上次重渲染 KPI 差异超阈值（默认 ±5%，per-KPI 可配）时报告带 drift 徽标
- [ ] **PORT-07**: 并发运行守卫——同 (skill, period) 重渲染重叠时第二次跳过并记审计行

### 管理台 (ADM)

- [ ] **ADM-01**: `/admin` 路由复用 `/eval` 模式 + `X-Admin-Token` 常量时间门，不新增认证模型
- [ ] **ADM-02**: 导入批次列表视图（状态 + 预览行数 + 已弃 raw 体积）
- [ ] **ADM-03**: KPI 定义列表（函数签名 + 最后调用时间），UI 只读不可编辑
- [ ] **ADM-04**: Skill 管理视图（热重载状态 + 最后 eval 结果 + Skill 变更审计行）
- [ ] **ADM-05**: 定时任务列表（last-run 状态 + next-fire 时间）
- [ ] **ADM-06**: 全部 admin 动作产生审计行（`actor_type: admin`）

### 生产化 — Fan-out 分类器 (FAN)

- [ ] **FAN-05**: LLM fan-out 分类器替换 Phase 13 确定性占位——结构化输出 `{should_fan_out, num_subtasks, task_boundaries[]}`，决策进审计，分类器 Skill 感知（description + query）
- [ ] **FAN-06**: 冻结 eval 集断言 fan-out 率稳定性（`expected_fan_out` per case），防分类器漂移

### 生产化 — Eval 抬升 (GEVL)

- [ ] **GEVL-09**: 新指标「KPI 调用准确率」（agent 为 query 选对 KPI？）——50 例 KPI golden set；数值相等用程序判定（非 LLM judge）
- [ ] **GEVL-10**: faithfulness 扩展到数据引用——答案中的数字必须与所引 KPI 快照一致（引用出处完整性）
- [ ] **GEVL-11**: eval 门阻塞调度——KPI 指标回退的 Skill 被阻止 cron 重渲染（eval gate ↔ 调度器联动）

### 生产化 — 平台级信任地板 (PLAT)

- [x] **PLAT-09**: 平台级隐藏系统提示词——代码级固定、prepend 于 Skill prompt、Skill 不可见不可覆盖、热重载存活；数据维度铁律内建（永不写 SQL 只调注册 KPI 函数等）
- [x] **PLAT-10**: 审计行携带 platform_prompt 版本；admin 可预览 Skill + 平台合成后的有效 prompt
- [x] **PLAT-11**: `platform-floor.jsonl` 冻结集断言平台层不变量（诱导写 raw SQL → 拒答）

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
| NL2SQL 任何形态（含 LLM 生成 SQL + guard model） | DATA-09 铁律——一次误判即泄漏；v1.1 仅外部授权通道带 DATA-10..14 全套防护（v1.0 决策，FEATURES.md 反功能表） |
| 去哈希 / 再识别能力（解 L1 哈希） | 击穿脱敏护城河（GDPR/个保法/PCI-DSS 审计红旗）；需要原始数据时从源头重导 |
| LLM 自动分类列敏感级（L1/L2/L3） | 非确定性策略分类 = 审计灾难；导入时由数据人员静态声明 |
| 拖拽式 KPI 构建器 / 仪表盘布局编辑器 / 聊天内 Skill 编辑器 | KPI 定义与布局属代码纪律，走 PR + eval 门（Skill = 唯一定制面） |
| 报告 PDF 生成 | 引 headless browser/typst 依赖 + 排版漂移；Markdown + 图表满足 v1.0 |
| 邮件 / Slack / webhook 推送报告 | 交付基础设施是独立产品（PLAT-07 v2）；静态页 URL 替代 |
| 实时流式聚合（live tail） | 与 as_of 快照纪律 / 审计可复现性冲突 |
| 预测 / forecast KPI | ML 范围外（PROJECT.md 既有排除） |
| per-user 个性化报告 / 自定义仪表盘视图 | 配置漂移（"谁的报告是对的？"）；Skill 即定制 |
| 管理台 RBAC / 多用户管理 | 单 admin token 是 v1.0 姿态（SCALE-01 → v2） |

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
| DIST-09 | Phase 7 | Complete (Phase 7 leftover closed by 13-06 Task 3 via scripts/eval_snapshot) |
| DIST-10 | Phase 7 | Complete |
| CMPL-01 | Phase 2 | Pending |
| CMPL-02 | Phase 2 | Pending |
| CMPL-03 | Phase 2 | Pending |
| CMPL-04 | Phase 2 | Pending |
| CMPL-05 | Phase 2 | Pending |
| CMPL-06 | Phase 5 | Complete |
| CMPL-07 | Phase 5 | Complete |
| CMPL-08 | Phase 5 | Complete |
| DATA-01 | Phase 15 | Complete (15-04 SUMMARY; `internal/data/dump.go` accept-list parser + 3.53.0 fixtures; CSV/push channels remain in 15-05/15-06) |
| DATA-02 | Phase 15 | Complete |
| DATA-03 | Phase 16 | Pending |
| DATA-04 | — | Deferred v1.1（外部授权通道） |
| DATA-05 | — | Deferred v1.1（外部授权通道） |
| DATA-06 | — | Deferred v1.1（外部授权通道） |
| DATA-07 | Phase 16 | Pending（仅 aggregated_only 模式） |
| DATA-08 | Phase 16 | Pending |
| DATA-09 | Phase 16 | Pending |
| DATA-10 | — | Deferred v1.1（外部授权通道） |
| DATA-11 | — | Deferred v1.1（外部授权通道） |
| DATA-12 | Phase 16 | Pending |
| DATA-13 | — | Deferred v1.1（外部授权通道） |
| DATA-14 | — | Deferred v1.1（外部授权通道） |
| PLAT-04 | Phase 8 | Complete |
| PLAT-05 | Phase 8 | Complete |
| GEVL-08 | Phase 9 | Complete |
| MEM-01 | Phase 10 | Complete (Phase 10; live 35-case re-run = v0.8 ledger row 1, OPEN) |
| MEM-02 | Phase 10 | Complete (Phase 10; live 35-case re-run = v0.8 ledger row 1, OPEN) |
| MEM-03 | Phase 10 | Complete (Phase 10; live 35-case re-run = v0.8 ledger row 1, OPEN) |
| MEM-04 | Phase 10 | Complete (Phase 10; live 35-case re-run = v0.8 ledger row 1, OPEN) |
| MEM-05 | Phase 10 | Complete (Phase 10; live 35-case re-run = v0.8 ledger row 1, OPEN) |
| HITL-01 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-02 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-03 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-04 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-05 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-06 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-07 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| HITL-08 | Phase 11 | Complete (Phase 11 gate PASS 11/11, 2026-08-20) |
| PLAN-01 | Phase 12 | Complete (Phase 12 gate PASS 11/11) |
| PLAN-02 | Phase 12 | Complete (Phase 12 gate PASS 11/11) |
| PLAN-03 | Phase 12 | Complete (Phase 12 gate PASS 11/11) |
| PLAN-04 | Phase 12 | Complete (Phase 12 gate PASS 11/11) |
| PLAN-05 | Phase 12 | Complete (Phase 12 gate PASS 11/11) |
| FAN-01 | Phase 13 | Complete (Phase 13 gate PASS 9/9 hermetic + live UAT operator-signed 2026-08-21) |
| FAN-02 | Phase 13 | Complete (Phase 13 gate PASS 9/9 hermetic + live UAT operator-signed 2026-08-21) |
| FAN-03 | Phase 13 | Complete (Phase 13 gate PASS 9/9 hermetic + live UAT operator-signed 2026-08-21) |
| FAN-04 | Phase 13 | Complete (Phase 13 gate PASS 9/9 hermetic + live UAT operator-signed 2026-08-21) |
| PLAT-09 | Phase 14 | Complete (14-01 + 14-10 SUMMARY; single ComposePlatformPrompt exit + unshadowable + hot-reload survives + WalkDir no-bypass; 1.1.0 anti-dilution re-anchor live-verified 20/20) |
| PLAT-10 | Phase 14 | Complete (14-02 SUMMARY; audit platform_floor_version column + all 5 INSERT paths + admin preview endpoint) |
| PLAT-11 | Phase 14 | Complete (14-04 SUMMARY; 20-case frozen set + JudgeHermetic + platform-floor.lock.json) |
| DATA-15 | Phase 15 | Complete (15-04 SUMMARY; `internal/data/preview.go` in-memory preview session store + `ValidateCommitAgainstPreview` content-hash bridge; HTTP handlers remain in 15-06) |
| DATA-16 | Phase 15 | Complete (15-02 SUMMARY; LatestImportBatch query surface + import_batches (dataset,period) index) |
| DATA-17 | Phase 15 | Complete (15-02 SUMMARY; INSERT-only import_batches table with batch_id/raw_size_bytes/desensitized_row_count/discard_ts/dataset + TestBatchAuditRowShape) |
| KPI-01 | Phase 16 | Pending |
| KPI-02 | Phase 16 | Pending |
| DASH-01 | Phase 17 | Pending |
| DASH-02 | Phase 17 | Pending |
| DASH-03 | Phase 17 | Pending |
| DASH-04 | Phase 17 | Pending |
| DASH-05 | Phase 17 | Pending |
| GEVL-09 | Phase 17 | Pending |
| GEVL-10 | Phase 17 | Pending |
| RPT-01 | Phase 18 | Pending |
| RPT-02 | Phase 18 | Pending |
| RPT-03 | Phase 18 | Pending |
| RPT-04 | Phase 18 | Pending |
| RPT-05 | Phase 18 | Pending |
| FAN-05 | Phase 18 | Pending |
| FAN-06 | Phase 18 | Pending |
| PORT-01 | Phase 19 | Pending |
| PORT-02 | Phase 19 | Pending |
| PORT-03 | Phase 19 | Pending |
| PORT-04 | Phase 19 | Pending |
| PORT-05 | Phase 19 | Pending |
| PORT-06 | Phase 19 | Pending |
| PORT-07 | Phase 19 | Pending |
| DATA-18 | Phase 19 | Pending |
| GEVL-11 | Phase 19 | Pending |
| ADM-01 | Phase 20 | Pending |
| ADM-02 | Phase 20 | Pending |
| ADM-03 | Phase 20 | Pending |
| ADM-04 | Phase 20 | Pending |
| ADM-05 | Phase 20 | Pending |
| ADM-06 | Phase 20 | Pending |

**Coverage:**

- v1 requirements: 70 total (was 55; +8 DATA +1 SYSK-08 +6 DATA-09..14 added via /gsd:explore 2026-08-10/11)
- v1 mapped to phases: 70（其中 DATA 7 条 2026-08-21 起转入 v1.0 milestone 池；DATA 7 条延 v1.1）
- v0.8 requirements: 25 total (added 2026-08-17 via /gsd:new-milestone)
- v0.8 mapped to phases: 25 (phases 8-13)
- v1.0 requirements: 44 total (added 2026-08-21 via /gsd:new-milestone；7 条激活 DATA + 37 条新 ID：DATA-15..18 / KPI-01..02 / DASH-01..05 / RPT-01..05 / PORT-01..07 / ADM-01..06 / FAN-05..06 / GEVL-09..11 / PLAT-09..11)
- v1.0 mapped to phases: 44 (phases 14-20; inline roadmap 2026-08-21)
- Unmapped (within active milestones): 0 ✓

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
| Phase 14 (CGO 地基 + 平台信任地板 P10.0) | 3 | PLAT-09..11 |
| Phase 15 (数据导入管线 + 导入即脱敏 P10.1) | 5 | DATA-01, DATA-02, DATA-15..17 |
| Phase 16 (KPI 聚合通道 P10.2) | 7 | DATA-03, DATA-07..09, DATA-12, KPI-01, KPI-02 |
| Phase 17 (KPI 仪表盘 Skill P10.3) | 7 | DASH-01..05, GEVL-09, GEVL-10 |
| Phase 18 (业务月报 Skill P10.4) | 7 | RPT-01..05, FAN-05, FAN-06 |
| Phase 19 (静态报告门户 + 调度器 P10.5) | 9 | PORT-01..07, DATA-18, GEVL-11 |
| Phase 20 (管理台 P10.6) | 6 | ADM-01..06 |
| **v1.0 Total** | **44** | |

---
*Requirements defined: 2026-08-10*
*Last updated: 2026-08-21 — v1.0 milestone requirements (44) added + mapped to phases 14-20（inline roadmap）; DATA-01..14 归并（7 激活 / 7 延 v1.1）；Out of Scope 扩充（NL2SQL / 去哈希 / 拖拽构建器 / PDF / 推送 / RBAC 等）*
