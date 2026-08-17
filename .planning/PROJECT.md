# SkillForge

## What This Is

SkillForge 是一个面向企业的、可交互的 Web 应用，展示大模型 agent 工程能力。技术栈为 **Go + Eino（字节 CloudWeGo 开源的 LLM 框架）+ Eino ADK 多 agent 编排**（后端），**React 19 + Vite + TypeScript**（前端）。核心创新是"**Skill 驱动**"：企业通过编写或更新 Skill 文件（`SKILL.md`）让同一个 agent 内核适配不同业务需求，无需改代码。内置"企业合规/制度审查"Skill 作为样板间，开源发布，用于建个人技术品牌。

## Core Value

**同一个 agent 内核，通过可插拔的 Skill 文件被任意企业快速定制，无需改代码** —— 这是从"应用"到"平台"的护城河，也是"通过更新 Skill 让项目更理解企业使用需求"这一设计哲学的直接落地。

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

- ✅ **Phase 1: Integration Spike & Trust Contract** (2026-08-11) — 验证了 Eino v0.7.7 + Hertz SSE + Vercel AI SDK + Milvus tier-filter 的 wire 契约。5 个成功标准中，SC-1（SSE envelope）/ SC-4（citation 渲染）/ SC-5（version-pinned contract test <10s）全自动化绿；SC-2（Milvus tier-filter）+ SC-3（ChatModelAgent + 取消 <200ms）代码路径完成，live verify 延后给用户（内网 Milvus + GLM key 需手动跑）。spike 产物：`cmd/spiked/` 多子命令二进制（wire/milvus/sse/e2e）+ `internal/spike/`（uimsg/hertz_sse/milvus_client/verdict）+ `web/` React stub + `integration/contract/` drift-detection 套件。**关键约束确认**：内网 API 策略（Milvus + BGE-M3 走内网，GLM 走公网），Phase 7 才补 docker-compose 部署路径；`@ai-sdk/react` 实际是 v4.x（STACK.md 的 "AI SDK v5" 是产品代号 UIMessageChunk，非 npm 版本）。

- ✅ **Phase 2: Compliance Vertical Slice** (2026-08-11) — **"Skill 即平台"论点证明**。一个 Skill（合规）/ 一个权威层 / 一个端点（`POST /api/chat`），对任何查询给出**带引用裁决 OR 结构化拒答，绝不臆造**。6/6 SC 对真实 MongoDB + Ollama-BGE-M3 + GLM-5.2 全绿（人工 checkpoint approved）。**D2-01 护城河实测有效**：SC-3 裁决 confidence 0.95 未被降到 0.89，证明 Go grounding checker 成功验证了 citation span 逐字出现在 chunk 原文（P-COMPL-1 头号风险落地）。产物：`internal/{verdict,wire,retriever/{milvus,mongo},embedder/bge,skill,knowledge,server}` + `cmd/server` Hertz 服务 + `skills/compliance/SKILL.md` + `knowledge/authority/` 7 篇语料 + `scripts/live-verify.sh`。**多后端能力兑现**：`retriever.Retriever` + `embedder.Embedder` 接口抽象（11 commits），MongoDB 后端实战落地（5 commits，因用户 Milvus 环境损坏而启用）。活体验证发现并修复 5 个真实 bug（bge model 字段 / bge 超时 / unanswered→refusal 映射 / mongo seed 去重 / grounding key 不匹配导致护城河静默失效）。**Phase 3 待办**：embedding 区分度低（Ollama bge-m3 所有 doc 0.29-0.40）、LLM temp=0 非确定性——均靠 Phase 3 Ragas eval 驱动校准。

- ✅ **Phase 4: Streaming Trust UI** (2026-08-11) — **信任可视化前端落地**。React 19 + Vite + TS + Vercel AI SDK（`ai@7`）三面板（资料库 / 审查结论 / 对话）聊天 UI，把 Phase 2 后端的"带引用裁决 + 结构化拒答"翻译成可审计的用户体验：可见推理链（D-08 高度受限、可折叠，DOM 序在答案之前）、可点击引用（D-01 → ChunkSourceView 展示 TierBadge + 版本 + 生效日期 + 逐字原文）、结构化 VerdictCard（D-06 状态色边框 + 置信度条 + 需复核 badge）/ RefusalCard（D-07 超范围 🚫 vs 不知道 ❓ 强区分）、子进度指示器（FEUI-06 五步文案）、Skill 选择器（FEUI-04 切换 Skill 并联动 CenterPanel）。**UAT 12/12 全绿**（chrome-devtools MCP 自动化 + 单元测试证据），83/83 单元测试绿，1 个 gap（CenterPanel 标签指示器未受控于 `centerView.kind`）已修复（commit `e757d6e` + 回归测试）。**关键工程决策固化**：D-04 transport-swap 不变量（Mock ↔ Default 字节级一致，`transport-swap.test.tsx` 把关生产切换）、T-1-08 source-URL 白名单消毒门、Pitfall 7 状态边界（Zustand store 文件级禁止 `messages` 字段，`useChat` 独占消息流）、wire-type 编译期漂移报警（`AssertAssignable<SkillForgeChunk, UIMessageChunk>`）。覆盖需求：FEUI-01..06、SYSK-03、KNOW-05、CMPL-05、T-1-08。**已知的 Phase 5 延期项**：LeftSidebar 真实多会话/文件树、ChatInput `@` 模糊搜索、Skill 管理器 UI、双向 tab 导航 + UI-SPEC §169 chunk 空状态。

- ✅ **Phase 6: Eval Dashboard & Production Hardening — Code-Side (6a)** (2026-08-13；2026-08-14 正式拆分) — **可衡量效果护城河收尾（代码侧）**。把 Phase-3 已交付的 Go-native 信任评估器（`internal/eval` + `cmd/eval`）从批量 CLI 升级为 **gate 逻辑 + 可视化仪表板 + 生产级隐私姿态**。**2026-08-14 拆分决策**：本 phase 范围收窄为 code-side（ROADMAP 已同步），原 06-03（CI workflow wiring）+ 4 项 live human UAT 正式后置到 **Backlog Phase 999.2（= Phase 6b）**——需要 GitHub Actions 运行时 + GLM key，v0.7.7 里程碑内不承诺。代码侧 3/3 计划交付，**全部 6 个 SC 在单元/集成级别已验证**（11/12 must-haves，唯一 fail 即 06-03 的端到端 Actions 管道，已按拆分决策移出本 phase 范围）：
  - **SC-1 / GEVL-06 — 仪表板**：`/eval` 独立全屏路由（admin 模式）+ SkillsTable（per-Skill 5 个指标 + ✅/❌ 徽章）+ TrendPanel（recharts 折线 + 灰色虚线冻结基线 + >5% 漂移标记）+ CrossEvalTab（自我评估偏差检测）+ AuditTable（`/eval/audit` 子路由）。14 个 FE 测试绿色；D14 无 `messages` 不变量保留；D-04/D12 transport-swap 不变量隔离（`/eval` 使用自己的传输，聊天 SSE 未触动）。
  - **SC-2 / GEVL-07 — 网关逻辑**：`internal/eval.gateDecision` 纯函数 + `FormatGateMarkdown` + `runGateCheck`（cmd/eval 的唯一非零退出路径）+ `skills/compliance/eval-gate.json`（Q2 解决路径，`{"faithfulness_threshold": 0.9}`）。CI 工作流 `.github/workflows/eval-gate.yml` 存在于未合并 worktree 分支（commit `fc26d12`，138 行）——按用户决策推迟。
  - **SC-3 — 审计查询 UI + 行级脱敏**：`/api/audit` 7 参数 1:1 映射到 `store.QueryAudit(AuditFilter)`（100% 参数化，T-3-SQLI 回归绿色）；AuditTable 过滤栏 + `[REDACTED (90d+)]` 徽章 + 悬停详情。
  - **SC-4 — 50×5 会话隔离**：`TestSessionIsolation50x5` 在 `-race` 下运行，50 会话 × 5 轮，每次恰好 10 条消息；2.398s 内绿色；D18 `WHERE session_id=?` 分区保留。
  - **SC-5 — 保留/脱敏**：`redact()` 在 `QueryAudit` 和 `c.JSON` 之间运行（读取时投影，保留 T-3-AUDIT INSERT-only）；`SKILLFORGE_RETENTION_DAYS` env（默认 90）；`X-Admin-Token` 通过 `crypto/subtle.ConstantTimeCompare`（ASVS V6.2.1）门控原始行；unset admin token → 所有非管理员。
  - **SC-6 — 可复现性**：`computePromptHash` 3 参数（含渲染提示体）通过 `internal/reprohash` 叶子包（Q1 解决）；50 个冻结集 + `compliance.lock.json`（5 字段 pin：model_ver / prompt_hash / seed / eval_set_version / skill_name）；`TestShippedFrozenLockIsInSync` 证明字节身份；两层容差：StubJudge `1e-6`（严格）+ `//go:build liveeval` `1e-3`（抖动容忍）。

  **关键工程决策固化**：
  - **Q1 prompt_hash 加强**：hash 现在覆盖渲染提示体（之前仅 `skillName+modelVersion`），使 SC-6 冻结 pin 有意义。`internal/reprohash` 打破 server↔eval 导入循环；长度前缀字段防碰撞。
  - **Q2 eval-gate.json 路径**：`skills/<skill>/eval-gate.json`（非 `.claude/skills/`）——与产品 Skill 共置（D6-10 意图），匹配 D6-09 path-filter `skills/*/SKILL.md`。
  - **读取时脱敏**：保留 T-3-AUDIT INSERT-only 不变量（无 UPDATE/DELETE 存储路径）；脱敏是查询后投影，非写时修改。
  - **T-6-PATH Windows 安全**：`:ts` 正则从 `^[0-9T:-]+Z$` 收紧到 `^[0-9T-]+Z$`（冒号在 Windows 文件名中非法）。
  - **零新增 Go 依赖**（除 recharts@3.10.1 + human-verify 检查点）——符合 RESEARCH.md 要求。

  **4 个人工 UAT 项目 + CI wiring 已后置到 Backlog Phase 999.2**（原 `06-HUMAN-UAT.md`，2026-08-14 拆分时移入 999.2 范围）：
  1. 合并 eval-gate.yml 从 worktree 分支 + 牺牲性 PR（SC-2 端到端）
  2. 本地运行 `cmd/eval --frozen --archive` → 打开 `/eval` + `/eval/audit` 验证基线覆盖 + 脱敏徽章
  3. `SKILLFORGE_ADMIN_TOKEN` 切换（脱敏 vs 原始行）
  4. `go test -tags liveeval ./internal/eval/` 带 `GLM_API_KEY`（SC-6 抖动层）

  **产物**：`internal/reprohash/` + `internal/eval/{frozen,archive,gate,crosseval}.go` + `internal/server/{audit,eval}.go` + `internal/store/sqlite/audit.go`（OFFSET 支持）+ `cmd/eval` 标志矩阵 + `web/src/components/eval/{EvalDashboard,SkillsTable,TrendPanel,CrossEvalTab,AuditTable}.tsx` + `web/src/store/app-store.ts` eval slice + `web/src/App.tsx` 路由分支 + `web/src/components/layout/LeftSidebar.tsx` 导航条目 + `web/src/hooks/useEvalReports.ts` + `web/src/types/eval.ts` + `eval/frozen/compliance.jsonl` + `eval/frozen/compliance.lock.json` + `skills/compliance/eval-gate.json`。

### Active

<!-- Current scope. Building toward these. -->

- [ ] **Skill 系统**：`SKILL.md` 6 字段规范（`description` / `agent` / `reads` / `writes` / `steps` / `guardrails`）、Skill 热加载与管理、意图路由（基于 description 语义匹配）
- [ ] **Agent 内核（Eino + Eino ADK）**：多 agent 编排、工具调用、流式响应（参照 denova 的 Eino + ADK 落地）
- [ ] **真源分离知识层**：权威 / 历史 / 个人分层；Skill 通过 `reads` 字段声明知识边界（denova 真源分离机制的结构化升级）
- [ ] **Agentic RAG**：自主决策（检索/拒答/追问/多跳）、引用溯源到具体段落、来源可信度标注
- [ ] **护栏 & Eval**：拒答机制（不确定不瞎编）、引用溯源可视化、RAGAS 等可衡量评估管线
- [ ] **内置样板 Skill：企业合规/制度审查**：仅读权威制度文档、结构化判断（合规/不合规/需补充）+ 引用条款、强护栏（绝不臆测）
- [ ] **Skill 向导**（对标 denova 的 `skills-creator`）：企业用自然语言描述需求，agent 自动生成一个合规的 `SKILL.md`
- [ ] **可交互 Web 前端**：问答界面、Skill 管理/浏览、引用溯源可视化、推理链展示
- [ ] **开源可发布**：Apache-2.0 license、README + 文档、Demo 站点、本地启动 + Docker 一键跑

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- **多租户 SaaS**（计费/订阅/用户管理）—— v1 聚焦 agent 能力本身，不做商业化平台层
- **桌面级桌面应用**（如 denova 的本地二进制 + Web 混合）—— v1 聚焦 Web 应用形态
- **完整私有化部署运维**（K8s / Helm / 企业 SSO）—— v1 给本地启动 + Docker，运维套件后续
- **实时多人协作编辑 Skill** —— v1 假设单人 / 小团队维护 Skill，协作后续
- **自定义 LLM 训练 / 微调** —— 只做 agent 编排与工程，不做模型训练
- **移动端原生 App** —— Web + PWA 足够，不做原生

## Context

[Background information that informs implementation:]

- **参照项目 denova（alfredxw/denova）**：Skills / SubAgents 机制、真源分离（历史事实 / 当前状态 / 稳定设定 / 未来意图分层）、有界工具结果 + 可检索回合历史、本地 Git 版本管理 + Agent 可撤销修改，是 SkillForge 的核心参照系。用户已 fork denova 源码，可直接对照学 Eino 怎么落地 Skill 与真源分离。

- **2026 趋势背景**：
  - 全球 agent 竞争从"模型能力"转向"**控制与编排**"（Blue Prism）；多 Agent 协作成主流架构（麦肯锡）；Gartner 预测 40% 企业应用内嵌专用 agent（2026）
  - **企业 RAG 知识库**是国内 AI 落地最热场景，演进方向是 **Agentic RAG**（从"人找知识"到"知识主动服务人"），核心难点：检索精度、权限治理、**幻觉压制**、多 agent 编排
  - **面试官 2026 看重的项目类型**：#1 RAG 问答、#2 Tool-calling Agent、#3 多 Agent 系统；最热技能 = AI agent 架构 + RAG 工程 + LLM 评估 + 防幻觉 guardrails
  - **反例警示**：**Height.app**（最激进的 AI-native PM 工具）已于 **2025.9 关闭**，证明纯"AI 套壳工具"赛道已死，要做"平台"而非"功能"

- **用户动机**：展示大模型 agent 开发能力，建个人开源品牌。

## Constraints

- **Tech stack**：Go + Eino + Eino ADK + Hertz（后端，用户明确选择，对标 denova 路线）；React 19 + Vite + TypeScript + Vercel AI SDK + shadcn/Tailwind（前端）
- **License**：Apache-2.0（仿 denova，开源友好）
- **Deployment**：v1 本地启动 + Docker Compose；聚焦 Web 应用形态，非桌面级
- **Open source goal**：打磨文档 + 易上手 + 可衡量效果（用于个人品牌，而非纯面试展示）
- **Referential constraint**：denova 源码可作为实现参照（用户已 fork），但**不复制 denova 的内容生成方向**（小说 / RPG），仅借鉴 agent 工程架构（Skills / 真源分离 / Eino ADK）

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 选 **Go + Eino**（而非 Python / 全 TS） | 用户对 Go 熟悉；denova fork 提供成熟参照；想突出 agent 工程深度而非 Web 迭代速度 | — Pending |
| 选 **Skill 驱动**而非固定功能 agent | 让项目从"应用"升级为"平台"；企业通过更新 Skill 适配需求，无需改代码；这是 denova 的核心机制迁移 | — Pending |
| 内置 demo Skill = **企业合规/制度审查** | 痛点尖锐（合规不容许臆测）、真源分离天然契合（制度是权威真源）、拒答机制能出彩 | — Pending |
| 受众 = **开源建个人品牌** | 打磨重点：文档 + 易上手 + 可衡量结果（区别于纯面试项目） | — Pending |
| Skill 规范 **6 字段（显式 reads / writes）** | 把 denova 隐含在 body 步骤里的"读什么/写什么"结构化显式化 → 更规范、更安全、更易被 Skill 向导自动生成 | — Pending |
| **D-04 transport-swap 不变量**（Phase 4） | Mock 与 Default transport 必须从同一 fixture 产出字节级一致的 UIMessageChunk 序列；`transport-swap.test.tsx` 把关，避免生产切换到真实 Hertz 端点时 UI 漂移 | ✅ Validated — Phase 4 |
| **T-1-08 source-URL 白名单消毒门**（Phase 4） | `source-url` href 是 XSS 头号入口；在 SourceUrlPart 边界强制协议+主机白名单，拒绝即渲染 blocked 态，`<a>` 恒带 `rel=noopener noreferrer` | ✅ Validated — Phase 4（7 断言回归门） |
| **状态边界：Zustand 不存 `messages`**（Pitfall 7，Phase 4） | `useChat` 独占消息流；`app-store.ts` 文件级注释禁止 `messages` 字段，杜绝双真值源 | ✅ Validated — Phase 4 |
| **wire-type 编译期漂移报警**（Phase 4） | TS `AssertAssignable<SkillForgeChunk, UIMessageChunk>` 让 SDK 上游改名任何 discriminator 时编译失败——编译器即漂移报警 | ✅ Validated — Phase 4 |
| **D6 Reversed — Go-native trust evaluator**（Phase 3 / Plan 03-06） | **2026-08-12 锁定决策反转**（原 D6："Ragas as Python sidecar for eval"）。改为单语言 Go 栈：`internal/eval` 包 + `cmd/eval` 二进制，4 个指标（faithfulness / answer_relevancy / context_precision / context_recall）全部通过 GLM-as-judge + 现有 BGE-M3 embedder 计算。**只有 faithfulness 逐字移植 Ragas v0.1.21 的 claim-extraction + entailment prompts**（Apache-2.0，`eval/NOTICE` 署名），保证 headline 数字行业可比（"Ragas 口径"）；其余 3 个指标用简单 Go-authored yes/no judge prompts。理由：研究自评（03-RESEARCH.md line 132）估算"全量移植 faithfulness 约 2000 行"，但成本集中在 prompt——只有 claim extraction 真正校准敏感（处理否定 / 模态 / 时序 / 多句答案）。决策：只移植 faithfulness prompt；其余 3 个指标 Go 自建；栈保持单语言。**消除 T-3-SC 供应链 checkpoint**（无 PyPI 审计）、砍掉 ragas/langchain-openai 依赖树、规避 ragas 0.1.21 ↔ Python 3.13 安装摩擦。原始 Python 草案保留在 git 历史。 | ✅ Validated — Phase 3 / Plan 03-06 |

## Current Milestone: v0.8 Harness — 可信×行动融合

**Goal:** 把 SkillForge 从「可信问答内核」升级为「可信行动 harness」——每一步行动可审计、可拒答、可回放。

**Target features:**
- 前置：✅ 全量依赖升级（Phase 8 完成 2026-08-18：eino v0.9.14 + eino-ext openai v0.1.13 联动，summarization 中间件与 ADK HITL 解锁；hertz/milvus/sqlite pin 有意保持稳定，前端依赖升级仍挂 `todos/pending/upgrade-deps-latest.md`）
- 前置：spike 验证历史注入对 trust floor 拒答率的影响（golden set 对比 baseline 四指标漂移）——Phase 9 硬门
- 多轮对话记忆：sqlite session history 喂进 agent（修复 chat.go 单轮限制）+ 摘要压缩
- 写操作 + 审批门：Eino ADK Interrupt/Resume HITL + 审计行，「可信×行动」首次正面验证
- 规划层：DeepAgent（`adk/prebuilt/deep` + write_todos），规划轨迹可回放
- 子 agent 扇出：multi-hop 并行化，子 agent 继承 tier 知识边界

**Key context:** 开源发布（Phase 07-06）用户决定延后至本里程碑之后（2026-08-17）；可信税原则——每个 harness 维度都嫁接可信机制（审批门 / 审计行 / tier 继承 / 可回放）。定位与 Eino 能力事实清单见 `.planning/notes/harness-roadmap-decision.md`。

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-18 — Phase 8 complete (eino v0.9.14 + eino-ext v0.1.13 lockstep, sqlite CheckPointStore 骨架, live 三门 human-verify approved; v0.8 frozen eval 基线 faithfulness 0.972 / refusal_accuracy 95%). Prior: 2026-08-17 — Milestone v0.8 (Harness — 可信×行动融合) started; open-source release (07-06) deferred to after this milestone by user decision.*