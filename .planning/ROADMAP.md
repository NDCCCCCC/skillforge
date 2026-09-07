# SkillForge — Roadmap

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes.
**Mode:** mvp (vertical slices, MVP-first delivery)
**Granularity:** standard
**Date:** 2026-08-10

---

## Milestones

- ✅ **v0.7.7** — 可信问答内核 (phases 1-7) — shipped 2026-08-13
- ✅ **v0.8** — Harness 可信×行动融合 (phases 8-13) — shipped 2026-08-21
- ✅ **v1.0** — 业务数据支撑 Business Data Support (phases 14-20) — shipped 2026-08-30
- ✅ **v1.1** — 信任架构成熟 + OSS 收官 (phases 21-25) — shipped 2026-09-06
- 🚧 **v1.2** — 知识库运营与反馈闭环 (phases 26-30) — planned 2026-09-07

## Phase Details

<details>
<summary>✅ v0.7.7 — 可信问答内核 (phases 1-7) — SHIPPED 2026-08-13</summary>

- [x] Phase 1: Integration Spike & Trust Contract — 4/4 plans (completed 2026-08-11)
- [x] Phase 2: Compliance Vertical Slice — 5/5 plans (completed 2026-08-11)
- [x] Phase 3: Agentic Retrieval & Trust Evaluation — 6/6 plans (completed 2026-08-12)
- [x] Phase 4: Streaming Trust UI — 4/4 plans (completed 2026-08-11)
- [x] Phase 5: Skill Platform & Full Knowledge Boundaries — 8/8 plans (completed 2026-08-12)
- [x] Phase 6: Eval Dashboard & Production Hardening (6a) — 3/3 plans (completed 2026-08-13)
- [ ] Phase 7: Open-Source Release — 5/6 plans (in progress — 07-02 T3, 07-05 T2/T3, 07-06 blocking)

</details>

<details>
<summary>✅ v0.8 — Harness 可信×行动融合 (phases 8-13) — SHIPPED 2026-08-21</summary>

- [x] Phase 8: 全量依赖升级 (P8.0) — 5/5 plans (completed 2026-08-18)
- [x] Phase 9: 历史注入 trust-floor spike (P8.S) — 6/6 plans (completed 2026-08-18, NO-GO + escape plan)
- [x] Phase 10: 多轮对话记忆 (P8.1) — 4/4 plans (completed 2026-08-20, closed/blocked-with-notes)
- [x] Phase 11: 写工具 + 审批门 (P8.2) — 8/8 plans (completed 2026-08-20)
- [x] Phase 12: 规划层 (P8.3) — 8/8 plans (completed 2026-08-20)
- [x] Phase 13: 子 agent 扇出 (P8.4) — 6/6 plans (completed 2026-08-21)

</details>

<details>
<summary>✅ v1.0 — 业务数据支撑 Business Data Support (phases 14-20) — SHIPPED 2026-08-30</summary>

- [x] Phase 14: CGO 地基 + 平台信任地板 (P10.0) — 12/12 plans (completed 2026-08-23)
- [x] Phase 15: 数据导入管线 + 导入即脱敏 (P10.1) — 7/7 plans (completed 2026-08-24)
- [x] Phase 16: KPI 聚合通道 (P10.2) — 9/9 plans (completed 2026-08-27)
- [x] Phase 17: KPI 仪表盘 Skill (P10.3) — 8/8 plans (completed 2026-08-27)
- [x] Phase 18: 业务月报 Skill (P10.4) — 5/5 plans (completed 2026-08-29)
- [x] Phase 19: 静态报告门户 + 调度器 (P10.5) — 5/5 plans (completed 2026-08-29)
- [x] Phase 20: 管理台 (P10.6) — 7/7 plans (completed 2026-08-30)

**Archived:** `.planning/milestones/v1.0-ROADMAP.md` + `.planning/milestones/v1.0-REQUIREMENTS.md`

</details>

### ✅ v1.1 — 信任架构成熟 + OSS 收官 (phases 21-25) — SHIPPED 2026-09-06

- [x] Phase 25: 评测信任环闭环 (Eval Trust-Loop Closure) — 2026-09-06 新增，本地模型基线回放暴露的 harness 遗留问题闭环；**建议最先执行**（golden 集是 Phase 22 Eval CI 的前置数据基础）：
  - [x] **Golden-corpus 同步**：frozen compliance 集 20 道 adversarial 题中 17 道（`source: None`，spike 后追加）+ 多数 mh 题的 ground_truth 在 knowledge/authority/ 无对应条款（4 字 shingle 覆盖 0-44%）。逐题处置：补写 authority 语料（首选，保留测试强度）或收缩 GT/降级为 refusal-required。验收 = 覆盖分析脚本全部应答题 ≥50%。
  - [x] **agent_error 与 refused 分离**：`classifyRunOutcome` 把 agent 运行错误（`exceeds max steps`、LLM 5xx）计为 refused，污染拒答类指标（全量跑 2/50）。CaseResult additive 增加 refusal_reason，Report 均值排除 agent_error 案例并单独计数。
  - [x] **本地模型基线归档**：已完成（.planning/uat/local-model-baselines-2026-09-06.md v2 修订）——修正后真实差距 = 语料无据时 48% 硬编率；UD-IQ1_M 对照 0/20 全编确认 1-bit 不可用。
  - [x] **闭环验收**：补语料 + 重索引后全量 replay 复跑，确认语料内应答题 faith、refusal-required 20/20、无 agent_error 混入。正式 pinned glm-5.2 复签（14-10 Task 2）仍等 GLM 配额，独立于本阶段。
- [x] Phase 21: External Authorized Channel (DATA-04/05/06/10/11/13/14)
- [x] Phase 22: Eval CI Wiring + Live UAT (Phase 6b completion) — 含 2026-09-05 竞品格局分析补充项：
  - [x] **Judge 人工校准集**：30-50 例人工标注对齐 + judge-vs-human agreement 报告（MLflow 2026 指南的标准 practice）。现有 `--judge-models` 跨评测只覆盖 judge 间分歧（Pitfall 8 自评偏差），未覆盖 judge-人工一致性——这是行业标准做法中缺口的一半。golden set 已有，成本低。
  - [x] **评测器定位声明**：`cmd/eval` 的 4 指标（faithfulness 等）与 Ragas/DeepEval/TruLens 完全同质（Ragas 口径已兼容），定位明确为 **Skill 晋级闭环的内部基础设施**，不作为独立卖点——README/文档措辞规避"评测器=核心能力"误读。
- [x] Phase 24: 审计链强化 (Audit Chain Hardening) — 2026-09-06 新增，对标 Semantica 补齐三弱势；**建议先于 Phase 23 执行**（产出直接把 Phase 23 README里"承认 PROV-O 是行业标准"的弱势措辞翻转为"支持 PROV-O 导出"的正面特性）：
  - [x] **PROV-O 标准导出**（补弱势①无标准导出）：新 `internal/auditprov` 包，把既有 INSERT-only 审计数据映射为 W3C PROV-JSON——prov:Entity = query/chunk/verdict/规则行，prov:Activity = run/retrieve/approve/revoke，prov:Agent = user/skill/model_version/platform_floor_version；关系 wasGeneratedBy / used / wasDerivedFrom。零 LLM 纯确定性（Semantica 同款哲学），经既有 retention 脱敏规则，导出端点 `GET /api/session/:id/audit/prov` + 行内 sha256 完整性清单。
  - [x] **决策因果链查询**（对标 trace_decision_chain，补弱势②扁平事件无因果链）：`GET /api/session/:id/decision-chain?root=<tool_call_id>`，沿既有 `tool_call_id==approval_id`（D11-C4）+ session_id + approval_rules 审计痕迹行走链：query → retrieve(chunk_ids) → verdict → approve/reject → rule 创建 → auto-pass(last_used) → revoke；FE AuditTimelineTab 增链式视图。
  - [x] **时间点状态重建**（对标 time-travel，补弱势③无历史回放）：`GET /api/session/:id/state?at=<RFC3339>` 从 INSERT-only 数据确定性重建 T 时刻的 turns + 审计事件 + 规则表状态——已撤销规则从 `action_rule_revoked` 的 frozen_args PK 复原（T-3-AUDIT"审计行即证据"设计的终极验证，纯只读实现）。
  - [x] （可选，需 planner 决策）审计行 prev-hash 哈希链 additive 列——防篡改可验证，超出 Semantica 对标范围，属增强项；如做须守 D14-E1 索引后置纪律。
  - 纪律：additive migration only（D14-E1）；INSERT-only 不变式 source-grep 锁不放松（T-3-AUDIT / D16-03-A 同款）；审计路径零 LLM；导出必须过既有脱敏。
- [x] Phase 23: Phase 7 completion (Open-Source Release) — 含 2026-09-05 竞品格局分析 + 2026-09-06 第二轮深度竞品验证（WeKnora/Semantica）补充项：
  - [x] **README 差异化叙事重排**（依据两轮联网验证的成熟度判定）：
    - 头条差异点（两轮验证仍空白/存活）：① **approve-and-remember 审批门一等公民原语**（`approval_rules` frozen-args-hash + session/skill 二元 scope + 撤销即审计——主流框架只做 pause-resume；Semantica 的 decision intelligence 是事后因果追溯，非执行前审批；WeKnora 无审批门）；② **信任契约内核**（tier 知识隔离 + refusal-by-construction + citation 白名单=本轮检索——WeKnora 走"查得准"混合检索无拒答构造；Semantica 管知识事实溯源不管执行边界）；③ **评测门禁**（frozen pin + CI gate + 跨评源——两个项目与业界均无）。
    - 降权/收窄的叙事：**审计链从头条降为支撑能力**（Semantica 以 W3C PROV-O 标准做溯源+决策因果图+冲突检测+时间旅行，2026-08 GitHub Trending #1；SkillForge 自研 audit 表无标准导出——差异收窄为"agent 写操作执行审计 vs 知识事实溯源"，措辞必须承认 PROV-O 是行业标准）；**"Go 企业 RAG 平台"不作为卖点**（WeKnora 21k+ star 腾讯背书 Go 单体后端+RBAC+审计已是成熟对标）；4 信任指标与 CI 门禁模式维持降权（Ragas 同款/行业最佳实践）。
    - 新增 Skill 术语去混淆段：WeKnora 的"Skill"=可插拔技能封装安装进 Docker/E2B/Cube 沙箱（Anthropic Agent Skills 路线）；SkillForge 的 Skill=**领域适配契约**（知识层边界 reads + 写目标 writes + 拒答护栏 + prompt 主体，同一内核不改代码换领域）——README 第一屏必须消除同名误读。
    - 竞品对照段扩为三分位：WeKnora 管"文档怎么查得准"（RAG+Wiki+多租户 RBAC）/ Semantica 管"决策为什么"（PROV-O 溯源+因果图+知识图谱）/ SkillForge 管"**什么不许答、什么必须批**"（执行时信任边界：tier 隔离、拒答构造、审批门、评测门禁）——三者互补而非同赛道对垒。

**Full phase details in archived milestones:**

- `.planning/milestones/v0.7.7-ROADMAP.md`
- `.planning/milestones/v0.8-ROADMAP.md`
- `.planning/milestones/v1.0-ROADMAP.md`

### 🚧 v1.2 — 知识库运营与反馈闭环 (phases 26-30) — PLANNED 2026-09-07

**Milestone Goal:** 把知识层从「4 个硬编码 tier 的启动期目录」升级为「运营型知识库平台」——动态知识库注册表、管理员上传常用格式文档入库、图谱反映真实知识结构、反馈按钮产生真实数据闭环、工单系统预留标准接口。贯穿性不变量：**信任语义不削弱**（KNOW-02 fail-closed / D-09 拒答构造 / citation 白名单）+ INSERT-only 审计纪律 + admin-token 门 + additive migration（D14-E1）+ 图谱边零 LLM 确定性 + 零硬编码（注册表/配置驱动）。

**Phases (execution order 26 → 27 → 28 → 29 → 30；28 与 29 可在 26 之后并行)：**

- [x] **Phase 26: 知识库注册表地基** - TierEnum 硬编码 4 值枚举 → 注册表驱动的动态知识库（admin 可建/停用，Skill reads 契约与检索层 fail-closed 校验）+ 里程碑卫生门（TODO 清零 + 结构门）（completed 2026-09-07 — 10/10 plans，verification 54/54 + 3/3 human items resolved，live UAT 5/5 PASS）
- [x] **Phase 27: 文档上传入库** - /admin 上传 md/pdf/docx/xlsx/pptx → 解析 → 分块 → 向量化 → 入指定知识库；原始文件即弃；入库即审计、即可见（completed 2026-09-07 — 7/7 plans，verify-phase-27.sh 3 stages / 6 sub-checks 全绿，17 upload tests PASS，operator runbook 27-UAT 待执行）
- [x] **Phase 28: 知识图谱动态化 + 关系层** - 图谱按注册表输出 N 知识库节点 + 确定性 chunk 关系边 + 邻居/路径查询 API + GraphCanvas 升级 + 性能护栏（completed 2026-09-07 — 7/7 plans，verify-phase-28.sh 3 stages / 6 sub-checks 全绿，23 BE tests + 535 FE tests PASS，operator runbook 28-UAT 待执行；mid-execution bug-fix 3766eb5 Neighbors degraded semantic frontier-cut）
  - [x] 28-01 — Edge struct + 4 edge-type constants + ComputeEdges pure function (GRF-02) — 2 commits: 5144fae + 858474c
  - [x] 28-02 — neighborsHandler + pathHandler + graphHandler degraded extension (GRF-03) — 3 commits: 61d6932 + 6b649f7 + 3a67f07 (plus bug-fix 3766eb5)
  - [x] 28-03 — sync.Map TTL cache + invalidation hooks (GRF-05) — 2 commits: 8b6aaa3 + d0369a5
  - [x] 28-04 — main.go wiring: SKILLFORGE_GRAPH_* env vars + SetGraphCacheInvalidateHook boot binding (GRF-05) — 3 commits: 6867599 + ef64ffa + fc8cc61
  - [x] 28-05 — FE GraphCanvas 边层 + useGraphNeighbors hook + NeighborExpandButton (GRF-04) — 4 commits: afc9377 + 028d3b9 + 2c14387 + 1767b1d
  - [x] 28-06 — InspectorRetrieval "常被一起引用" 面板 (GRF-04) — 2 commits: 41cebec + c654cd9
  - [x] 28-07 — 卫生门 (verify-phase-28.sh) + STRIDE-28 + 28-UAT.md（含 Phase 27→28 联合验收） — 4 commits: 0fceddc + c0906d1 + d8a881b + 82a7a1e
- [ ] **Phase 29: 反馈闭环 + 工单预留** - FeedbackRow 三按钮真实落库（一次一票）+ 过时复核队列 + 仪表盘指标 + TicketService 接口 / webhook provider / POST /api/tickets
- [ ] **Phase 30: 工程卫生收尾** - 零硬编码魔数抽查 + 全 SDLC 门里程碑级验收 + 信任语义回归证明

**Coverage:** 29/29 v1.2 requirements mapped — KBM-01..06 + HYG-01 → 26; DOC-01..06 → 27; GRF-01..05 → 28; FBK-01..05 + TKT-01..04 → 29; HYG-02..03 → 30

#### Phase 26: 知识库注册表地基 (KB Registry Foundation)

**Goal**: 管理员可以创建/停用知识库，Skill `reads` 契约与检索/索引两侧全部按注册表 fail-closed 校验，四语义角色语义重定义落档——知识层从「硬编码 4 tier 枚举」变为「注册表驱动的运营平台地基」，信任语义不削弱；同时建立里程碑卫生门（TODO 清零 + 结构门），后续所有 phase 继承该门。
**Depends on**: Nothing (v1.2 首个 phase；Phase 27/28/29 均依赖本 phase)
**Requirements**: KBM-01, KBM-02, KBM-03, KBM-04, KBM-05, KBM-06, HYG-01
**Success Criteria** (what must be TRUE):

  1. 管理员可经 token 门端点 + `/admin` 视图创建新知识库（slug 合法性校验）并停用既有知识库；停用后该库从检索与 Skill 校验中消失但数据不物理删除；创建/停用动作均产生 INSERT-only 审计行
  2. `reads` 声明未注册或已停用知识库的 Skill 在加载期校验失败（fail-closed）；旧 4-tier 字面量不再被接受——破坏性切换附带一次性迁移，既有 Skill 迁移后可正常加载
  3. 索引侧与检索侧对未注册知识库值均 fail-closed 拒绝；Milvus/Mongo 过滤表达式改为注册表驱动的知识库集合——KNOW-02 信任语义不削弱
  4. 四语义角色语义（authority = 唯一可引用证据源 / historical = 被取代版本留档不进默认检索 / personal = 用户私有默认排除 / draft = 未生效永不检索）写入平台地板文档与 README，且 citation 白名单代码强制仅限 authority 语义角色
  5. 启动期 `knowledge/<slug>/` 目录按注册表装载（未知目录 WARN + 跳过 + 审计，服务照常启动）；全仓非测试 Go 文件 TODO/FIXME 计数为 0 且有结构门把守回归（含 `cmd/spike-history/long.go:54` MEM-05 TODO 处置——核对 summarization 中间件真实落地状态后更新或移除）

**Plans**: 10 plans in 7 waves (planned 2026-09-07)

Plans:

- [x] 26-01-PLAN.md — internal/kb 注册表契约层（Role/CitableRole/slug/seed/原子快照/ResolveReads）+ knowledge_bases.yaml 受控 seed（KBM-01/03）
- [x] 26-02-PLAN.md — sqlite knowledge_bases 表 + kb.Store 实现 + kb.Open「结构归配置、开关归运营」合并语义（KBM-01/02）
- [x] 26-03-PLAN.md — knowledge/retriever 注册表化：共享 walker + 未知目录 WARN+审计 + validateTiers/graph Lookup + Milvus/Mongo slug 过滤 + cmd 调用点（KBM-05/06）
- [x] 26-04-PLAN.md — skill reads 契约破坏性切换：TierEnum 族删除 + Validate/ResolveTiers 注册表委托 + 全消费面迁移（runtime/data/loans.yaml/server/cmd）（KBM-01/04）
- [x] 26-05-PLAN.md — 服务端 KB 管理面：token 门创建/停用端点 + INSERT-only 审计 + 公开投影 + 反证/pipeline/knowledge.go 注册表化（KBM-02/03）
- [x] 26-06-PLAN.md — agent 信任面：ChunkAccumulator citability 门控（仅 authority 可引用）+ hitl Enum 注册表快照（KBM-03）
- [x] 26-07-PLAN.md — 评测数据迁移：fanout golden set CN→slug + fanout.lock.json 显式重签（单一连续任务）+ calculator/CONTRIBUTING 词汇（KBM-04）
- [x] 26-08-PLAN.md — FE：/admin Knowledge Base 管理视图 + TierBadge/图标签注册表化 + 类型放宽 + 硬编码 fallback 清除（KBM-02）
- [x] 26-09-PLAN.md — 装配收口：kb.Open 生产接线 + 审计钩子 + Milvus RECREATE re-seed 门 + WSL 全量门（KBM-01/02/05/06）
- [x] 26-10-PLAN.md — 收尾：KBM-03 四角色语义落档（README/kb godoc/STRIDE-26）+ verify-phase-26 三门（TODO 清零 RED→GREEN）+ live 迁移 operator checkpoint（KBM-03/HYG-01）

**UI hint**: yes
**Notes**: 用户已授权破坏性重构（不受向后兼容与既有设计约束）；受影响面 = `internal/skill`（TierEnum + validate.go allowlist）、`internal/knowledge/indexer.go` (KNOW-02)、`internal/retriever/{milvus,mongo}`、FE TierBadge / graph 节点；信任语义（KNOW-02 / D-09 / citation 白名单）只许迁移实现、不许削弱。

#### Phase 27: 文档上传入库 (Document Upload Pipeline)

**Goal**: 管理员可在 `/admin` 把常用格式文档上传进指定知识库，解析 → 分块 → 向量化 → 索引全链路 fail-closed，原始文件即弃，入库即审计、即可见（无需重启）。
**Depends on**: Phase 26（上传目标 = 注册表中的知识库）
**Requirements**: DOC-01, DOC-02, DOC-03, DOC-04, DOC-05, DOC-06
**Success Criteria** (what must be TRUE):

  1. 管理员可经 `/admin` 知识库管理视图上传 Markdown / PDF / Word(docx) / Excel(xlsx) / PowerPoint(pptx) 到指定知识库；解析器由注册表驱动（扩展名 + MIME 白名单，零硬编码）
  2. 上传文档按内容探测格式（不仅扩展名），走解析 → 既有分块 → 向量化 → 索引入指定知识库；解析失败 fail-closed，不产生半成品入库
  3. 上传处理完成后原始文件不落盘——持久化的只有解析文本与 chunk（延续 v1.0「不保存原始数据」哲学）
  4. 超限与重复上传按配置被拒绝或去重：单文件大小上限 / 并发上限 / 内容哈希去重策略（拒绝或替换为新版）全部配置驱动
  5. 每次上传产生 INSERT-only 审计行（操作者 / 知识库 / 文件名 / 大小 / hash / chunk 数 / 结果）；成功上传后文档立即出现在知识图谱与检索（无需重启），失败错误明确回报 UI

**Plans**: 7 plans in 4 waves
**UI hint**: yes
**Risks**: eino-ext document parsers 尚不在 go.mod 且内网网络间歇断——plan-phase 必须把「parser 依赖获取」当显式风险处理并预备纯 Go fallback 决策；上传管线不得继承 `knowledge.Index` 的 milvus-coupling（`cmd/server/main.go:730` 的启动期路径 Mongo 后端跑不了，新管线两条后端都要能达）。

Plans:

- [x] 27-01-PLAN.md — `internal/document` 解析层契约（ParserRegistry + 5 格式：md/pdf/docx/xlsx/pptx）+ CGO 探测 + magic-byte 验证
- [x] 27-02-PLAN.md — `internal/knowledge/upload_index.go` UploadIndex + GetMaxID 接口声明 + 8 个单元测试 + 验证 slug
- [x] 27-03-PLAN.md — `POST /api/admin/kb/:slug/upload` multipart streaming + admin-token 门 + 4-stage CHECK ORDER + magic byte + SHA-256 in-memory + INSERT-only audit
- [x] 27-04-PLAN.md — `internal/retriever/backend.go` IndexerBackend 接口 + Milvus/Mongo adapter + GetMaxID 在两个 client 的真实实现
- [x] 27-05-PLAN.md — FE `DocUpload` 面板（KB dropdown + 文件选择 + 状态机）+ AdminDashboard 接线 + 12 vitest
- [x] 27-06-PLAN.md — 卫生门：`scripts/verify-phase-27.sh`（3 stages / 6 sub-checks）+ Makefile phase-27 target + STRIDE-27.md + 27-UAT.md operator runbook
- [x] 27-07-PLAN.md — 退化模式（503/422/409/500）+ SHA-256 去重（Store.HasUploadBySHA256 on store.go + sqlite/audit.go）+ 5 退化测试

#### Phase 28: 知识图谱动态化 + 关系层 (Dynamic Knowledge Graph)

**Goal**: 知识图谱反映真实知识结构——按注册表输出 N 个知识库节点、chunk 间确定性关系边可查可展开、性能超限优雅降级，管理员与（未来的）agent 工具都能消费同一张图。
**Depends on**: Phase 26（动态 KB 节点来自注册表）；可与 Phase 27 并行执行（Phase 27 落地后图谱须包含上传文档——与 DOC-06 联合验收）
**Requirements**: GRF-01, GRF-02, GRF-03, GRF-04, GRF-05
**Success Criteria** (what must be TRUE):

  1. `GET /api/knowledge/graph` 按注册表输出 N 个知识库节点——创建/停用知识库后图谱结构随之变化，固定 4 tier 节点假设移除
  2. 图谱包含确定性（零 LLM、同输入可复算相同边集）chunk 关系边：同文档版本继承边 + 跨文档实体/关键词条款共现边
  3. 可经 API 查询给定 chunk 的关系邻居与受限深度路径（供 FE 与未来 agent 工具消费）
  4. FE GraphCanvas 渲染动态知识库节点与关系边，并支持邻居展开交互
  5. 节点/边上限、分页与懒加载策略全部配置驱动；超限返回骨架图并明示降级

**Plans**: 7 plans in 4 waves

Plans:

- [x] 28-01-PLAN.md — Edge struct + 4 edge-type constants + ComputeEdges pure function (GRF-02) — 5144fae + 858474c
- [x] 28-02-PLAN.md — neighborsHandler + pathHandler + graphHandler degraded extension (GRF-03) — 61d6932 + 6b649f7 + 3a67f07 + bug-fix 3766eb5
- [x] 28-03-PLAN.md — sync.Map TTL cache + invalidation hooks (GRF-05) — 8b6aaa3 + d0369a5
- [x] 28-04-PLAN.md — main.go wiring: 4 SKILLFORGE_GRAPH_* env vars + SetGraphCacheInvalidateHook boot binding (GRF-05) — 6867599 + ef64ffa + fc8cc61
- [x] 28-05-PLAN.md — FE GraphCanvas 边层 + useGraphNeighbors hook + NeighborExpandButton (GRF-04) — afc9377 + 028d3b9 + 2c14387 + 1767b1d
- [x] 28-06-PLAN.md — InspectorRetrieval "常被一起引用" 面板 (GRF-04) — 41cebec + c654cd9
- [x] 28-07-PLAN.md — 卫生门 (verify-phase-28.sh 3 stages / 6 sub-checks) + STRIDE-28 + 28-UAT.md（含 Phase 27→28 联合验收） — 0fceddc + c0906d1 + d8a881b + 82a7a1e

**UI hint**: yes
**Notes**: 图谱边保持零 LLM 确定性（可 hash 复现）——LLM 实体关系抽取明确 Out of Scope（未来候选，需评测门）。

#### Phase 29: 反馈闭环 + 工单预留 (Feedback Loop + Ticket Interface)

**Goal**: FeedbackRow 的四个按钮全部产生真实结果——三按钮投票落库幂等、过时报告进人工复核队列、指标进 admin 仪表盘、创建工单接真实端点；工单外发尽力而为不阻塞主链路；反馈绝不自动改信任。
**Depends on**: Phase 26（FBK-03 per-知识库指标需注册表命名）；可与 Phase 27/28 并行执行（session/message/chunk ids 已存在）
**Requirements**: FBK-01, FBK-02, FBK-03, FBK-04, FBK-05, TKT-01, TKT-02, TKT-03, TKT-04
**Success Criteria** (what must be TRUE):

  1. 用户点击有帮助 / 没帮助 / 报告知识过时后，投票写入 INSERT-only 反馈表（session/message/verdict/被引 chunk ids）且一次一票幂等（重复提交返回已记录状态）；FE 锁定态由服务端驱动、toast 反映真实结果——占位 toast 移除
  2. outdated 反馈生成复核队列条目，管理员在 `/admin` 复核视图看到待复核 chunk 并按已更新 / 误报 / 忽略处置，处置动作进审计；反馈绝不自动降权或删除 chunk——知识库变更只能由人工复核处置产生
  3. admin 仪表盘展示 per-Skill / per-知识库帮助率与过时报告数趋势
  4. FeedbackRow 创建工单按钮调用真实 `POST /api/tickets`：工单请求 INSERT-only 落库（session/message 引用、描述、被引 chunk ids），UI 诚实展示受理结果（受理编号 / 排队中 / 未配置 provider）
  5. TicketService Go 接口 + 配置驱动 provider 注册表（`none` / `webhook`）落地；provider 外发失败不影响聊天与反馈主流程（尽力而为 + 配置驱动重试），外发结果（成功/失败/none）进审计

**Plans**: 9 plans in 6 waves (planned 2026-09-08)

Plans:
**Wave 1**

- [ ] 29-01-PLAN.md — turn id 地基：AppendTurn → (int64, error) 原子涟漪（接口 + sqlite RowsAffected 守卫 + writeAuditAndTurn 返回值 + 12 调用点 + 5 fake）+ turn-id 语义测试（FBK-01/04 地基，D-29-01）
- [ ] 29-03-PLAN.md — internal/ticket 包：Provider 窄接口 + none/webhook 配置注册表 + webhook 出站（per-instance client + 有界重试 + body-only payload）（TKT-01）

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 29-02-PLAN.md — 反馈/复核/工单存储层：4 张 additive 新表（feedback / feedback_chunk_refs / feedback_dispositions / tickets）+ 13 个 Store 方法 + InsertOnly 结构锁 + pending 派生视图（处置后重开口径）+ 全票分母指标 SQL（FBK-01/02/03 + TKT-02 存储面，D-29-02/08/09/10/14 + Open Q1/Q2 resolved）

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 29-04-PLAN.md — data-turn wire 帧：ValidTypes + TS union D15 配对 + emitTurnID + 11 发射点（pipeline.go 6 / chat.go 3 / runner_setup.go 2；含 5 条拒绝路径的先落库重排，Open Q3 resolved 拒绝轮可投）+ history 内嵌 turn_id/voted_verdict（FBK-01/04，D-29-03）
- [ ] 29-05-PLAN.md — HTTP 面：POST /api/feedback（幂等投票 + audit 邻近归因服务端真相）+ admin 复核/处置/指标三端点（403 先于 503 + closed_count + 全票分母 null 语义）+ FBK-05 source-grep 结构锁（FBK-01/02/03/05，D-29-05/07）

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 29-06-PLAN.md — POST /api/tickets：幂等落库 + TKT-{id} 编号派生 + 异步外发 goroutine（后台 ctx + recover + 总预算）+ dispatch 结果审计行 + main.go 5 个 SKILLFORGE_TICKET_* env 装配（TKT-02/04）
- [ ] 29-07-PLAN.md — FE 接真：FeedbackRow 重写（服务端驱动锁定态 + outdated 说明框 + 工单 4 态诚实文案）+ MessageBubble data-turn 扫描 + SfToast 全局视口（chat/eval 分支）（FBK-04 + TKT-03，D-29-04/06/13，UI-SPEC 文案逐字）

**Wave 5** *(blocked on Wave 4 completion)*

- [ ] 29-08-PLAN.md — FE admin：「反馈」第 6 tab（口径注 + 5 汇总卡 0 票「—」+ 日/周趋势 + per-KB 脚注 + 待复核 cross-link）+「复核」第 7 tab（按 chunk 聚合 + 三处置 + 处置历史 + FBK-05 信任横幅）（FBK-02/03 + TKT-03，D-29-10/11/12）

**Wave 6** *(blocked on Wave 5 completion)*

- [ ] 29-09-PLAN.md — 收尾卫生门：verify-phase-29.sh 4 stages（TODO 清零 / FBK-05 结构锁 / D15 配对 / DO NOTHING 纪律，含负向自证）+ make phase-29 + STRIDE-29.md + 29-UAT.md（含 Phase 27→29 联合验收）（FBK-05 门 + 全 REQ 收口）

**UI hint**: yes
**Notes**: 真实工单系统对接（Jira/ServiceNow/自建）明确 Out of Scope——本里程碑只预留接口 + webhook provider；起点代码 = `web/src/components/chat/parts/FeedbackRow.tsx`（现为 toast 占位）+ `internal/server/` admin-token 门模式。

#### Phase 30: 工程卫生收尾 (Hygiene & Milestone Gate)

**Goal**: 里程碑级工程卫生达标——本里程碑新增的全部配置面零硬编码、全 SDLC 门在 phases 26-29 上可证、信任语义经注册表重构后由既有基线证明未削弱。
**Depends on**: Phase 26, Phase 27, Phase 28, Phase 29（里程碑收尾必须在全部功能 phase 之后）
**Requirements**: HYG-02, HYG-03
**Success Criteria** (what must be TRUE):

  1. 本里程碑新增的格式白名单 / 大小限额 / 图深限制 / 语义角色定义等关键字段魔数抽查通过——全部注册表或配置驱动，无新增硬编码字面量
  2. Phase 26-29 每个均可出示：单元测试 + hermetic 集成测试证据、WSL 构建验证（`make check-cgo` 前置）通过、受影响 verify 门全绿
  3. 触及 FE 的 phase（26-29 均触）均有 vitest + tsc 干净记录
  4. 信任语义回归证明：KNOW-02 fail-closed / D-09 拒答构造 / citation 白名单在注册表重构后由既有冻结 eval 基线与结构门证明未削弱

**Plans**: 7 plans in 4 waves

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Integration Spike & Trust Contract | 4/4 | Complete | 2026-08-11 |
| 2. Compliance Vertical Slice | 5/5 | Complete | 2026-08-11 |
| 3. Agentic Retrieval & Trust Evaluation | 6/6 | Complete | 2026-08-12 |
| 4. Streaming Trust UI | 4/4 | Complete | 2026-08-11 |
| 5. Skill Platform & Full Knowledge Boundaries | 8/8 | Complete | 2026-08-12 |
| 6. Eval Dashboard (6a) | 3/3 | Complete | 2026-08-13 |
| 7. Open-Source Release | 6/6 | Complete | 2026-09-06 |
| 8. 全量依赖升级 (P8.0) | 5/5 | Complete | 2026-08-18 |
| 9. 历史注入 spike (P8.S) | 6/6 | Complete | 2026-08-18 |
| 10. 多轮对话记忆 (P8.1) | 4/4 | Closed/blocked | 2026-08-20 |
| 11. 写工具 + 审批门 (P8.2) | 8/8 | Complete | 2026-08-20 |
| 12. 规划层 (P8.3) | 8/8 | Complete | 2026-08-20 |
| 13. 子 agent 扇出 (P8.4) | 6/6 | Complete | 2026-08-21 |
| 14. CGO + 平台信任地板 (P10.0) | 12/12 | Complete | 2026-08-23 |
| 15. 数据导入管线 (P10.1) | 7/7 | Complete | 2026-08-24 |
| 16. KPI 聚合通道 (P10.2) | 9/9 | Complete | 2026-08-27 |
| 17. KPI 仪表盘 Skill (P10.3) | 8/8 | Complete | 2026-08-27 |
| 18. 业务月报 Skill (P10.4) | 5/5 | Complete | 2026-08-29 |
| 19. 静态报告门户 (P10.5) | 5/5 | Complete | 2026-08-29 |
| 20. 管理台 (P10.6) | 7/7 | Complete | 2026-08-30 |
| 21. External Authorized Channel (DATA-04..14) | 6/6 | Complete | 2026-09-06 |
| 22. Eval CI Wiring + Live UAT | 4/4 | Complete | 2026-09-06 |
| 23. Open-Source Release (Phase 7 completion) | 4/4 | Complete | 2026-09-06 |
| 24. 审计链强化 (PROV-O + decision-chain + time-travel) | 4/4 | Complete | 2026-09-06 |
| 25. 评测信任环闭环 (eval trust-loop closure) | 4/4 | Complete | 2026-09-06 |
| 26. 知识库注册表地基 (KBM + HYG-01) | 10/10 | Complete | 2026-09-07 |
| 27. 文档上传入库 (DOC) | 7/7 | Complete (hermetic; 27-UAT operator runbook pending) | 2026-09-07 |
| 28. 知识图谱动态化 + 关系层 (GRF) | 7/7 | Complete (hermetic; 28-UAT operator runbook pending) | 2026-09-07 |
| 29. 反馈闭环 + 工单预留 (FBK + TKT) | 0/9 | Not started | - |
| 30. 工程卫生收尾 (HYG) | 0/TBD | Not started | - |

---

## Backlog

### Phase 999.1: 分层分块器决策 — hierarchical markdown splitter (BACKLOG)

**Goal:** Decide the fate of the markdown hierarchical splitter that was silently dropped from scope. Two options: (a) formally record one-doc-one-chunk + maxDocBytes(4000) guardrail as the permanent design (fine if knowledge corpus stays short-clause .md), or (b) implement eino-ext markdown header splitter to support long policy docs / PDFs. Origin: stale comments in internal/knowledge/indexer.go claimed "deferred to Phase 3 (CMPL-06)" — corrected in quick task 260814-ofn; no phase ever owned this work.
**Requirements:** TBD
**Plans:** 8/8 plans complete
