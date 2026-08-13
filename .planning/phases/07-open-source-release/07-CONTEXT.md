# Phase 7: Open-Source Release - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

把已完成的 SkillForge（Go+Eino 后端 + React 前端 + Go-native 评估体系）打包成一个**陌生人可以 `git clone && docker compose up` 跑起来、能看到已发布评估指标**的开源发布物 —— 这是个人技术品牌的最终载体。覆盖 DIST-01..10 共 10 条需求、8 条成功标准。

本阶段**不新增产品功能**，全部是打包/文档/发布工程：README + LICENSE + CONTRIBUTING + docker-compose + Dockerfile + examples/ + eval 结果发布 + 仓库公开治理。Skill 运行时、评估器、前端 UI 均为已交付资产，本阶段把它们"上架"。

**两条成功标准的表述需在计划阶段锁定修订**（见决策 D7-03、D7-06）：SC-1 的"含 BGE-M3"、SC-5 的"live demo site"。

</domain>

<decisions>
## Implementation Decisions

### Docker 打包与模型服务（SC-1, DIST-04）— D7-01..03
- **D7-01: docker-compose 起 Milvus standalone 全套（etcd + minio + milvus）。** 与生产路径（`RETRIEVER_BACKEND=milvus`）100% 一致，最忠实展示 tier-filter 护城河（Phase 1 SC-2 验证的核心机制）。全历史重写后仓库即为 `skillforge`，compose 是发布物的第一入口。陌生人笔记本资源开销大可接受——README 明确标注最低资源要求。**拒绝：** 默认走 mongo 后端（默认路径没展示 Milvus 生产形态，弱化护城河叙事）。
- **D7-02: BGE-M3 embedding 不内置进 compose，指向外部服务。** `.env.example` 的 `BGE_M3_URL` 指向用户自备的 embedding 端点；README 提供一行 `ollama pull bge-m3` 的本地启动指引作为最低门槛。理由：embedding 服务不是 SkillForge 的核心价值，且模型服务容器化（镜像体积、GPU/CPU 兼容）会拖慢"一键起栈"体验。**拒绝：** compose 内置 Ollama 容器自动拉模型（与用户选择"要求外部服务"冲突）。
- **D7-03: SC-1 修订为「compose 起 backend + frontend + Milvus + 可插拔 embedding 端点」。** 原文"含 BGE-M3"与 D7-02 的外部服务选择冲突，故修订表述：发布物不强制内置模型服务，但必须含 Milvus + backend + frontend 三件套，并通过 `.env` 把 embedding 端点声明为必填变量（缺值则启动报错，呼应 DIST-03）。这是对 ROADMAP 成功标准的合理裁剪——计划阶段需同步更新 ROADMAP SC-1 与 PROJECT.md。
- **D7-04: 后端/前端镜像纯源码构建（多阶段 Dockerfile）。** `git clone` 后 `docker compose up` 在容器内完成 Go build + Vite build，无需任何 registry 账号、代码即真相。首次构建 3-5 分钟可接受（README 提示）。**拒绝：** GHCR 预构建镜像（引入 registry 发布流程，违背"源码即真相"的极简发布理念）；双路径并行（维护成本翻倍）。

### Live demo 站点形态（SC-5, DIST-05）— D7-05..06
- **D7-05: 不部署在线 demo 服务，用录制 GIF 代替。** 理由：真实 LLM demo 需把 GLM API key 暴露在公网服务上（成本 + 滥用风险），对个人品牌项目不划算。GIF 在 README 顶部走完一次完整问答流程（含引用点击、裁决卡、拒答卡），足以传达核心价值。**拒绝：** 真实 demo + 硬限流（key 消耗与运维负担）；检索真实 + 响应预录制（仍需在线服务）。
- **D7-06: SC-5 / DIST-05 修订为「录制演示 GIF」。** 原文"live demo site deployed"与 D7-05 冲突，修订为录制的高质量演示 GIF（或短视频），放置在 README hero 区。计划阶段需同步更新 ROADMAP SC-5 与 REQUIREMENTS DIST-05。**不设内置 demo 模式**——想用真系统必须配 GLM key + BGE-M3，发布物保持精简（用户明确拒绝无 key 体验路径）。

### 开源仓库发布形态 — D7-07..09
- **D7-07: 现有仓库转 public（不另开干净仓库）。** 保留完整开发历史——GSD 工作流痕迹本身是"agent 工程方法论"的展示，与个人品牌叙事一致。公开仓库名定为 **`skillforge`**（当前目录名 `ai-agent` 过于通用）。计划阶段需在发布前确认 GitHub 仓库重命名/路径。
- **D7-08: `.planning/` 目录从公开历史中剔除。** 用 `git filter-repo` 剥离整个 `.planning/`（ROADMAP/CONTEXT/SUMMARY/STATE 等 GSD 规划痕迹）——公开仓库只含产品代码（`cmd/`、`internal/`、`web/`、`skills/`、`knowledge/`、`eval/`、`scripts/`）+ 官方文档（README/LICENSE/CONTRIBUTING）。理由：`.planning/` 含个人规划细节与执行过程，不属于发布物；剔除让公开仓库聚焦产品。
- **D7-09: 全历史重写清理内网信息。** `git filter-repo` 同时完成两件事：(a) 剥离 `.planning/`；(b) 扫描全历史替换内网 IP/域名为占位符（`10.62.10.34`、`milvus.internal.company.com`、`bge-m3.internal.company.com` 等 → 通用占位符如 `milvus:19530` / `localhost`）。接受代价：所有 commit hash 改变、需强制推送重建远端。计划阶段需把"全历史敏感信息扫描"作为发布前的硬性 gate（扫描 IP/域名/key 模式，出报告后人工确认清理完整）。

### examples/ 与 eval 结果发布（DIST-08/09/10, SC-6/7/8）— D7-10..12
- **D7-10: examples/ 的三个 Skill 模板（compliance + simple QA + calculator）只需通过 schema 校验，不强制全部可运行。** DIST-08 字面只要求"each passing the schema validator"。compliance 是唯一可运行模板（现有 `skills/compliance/SKILL.md`）；simple QA + calculator 作为展示 Skill 格式多样性的范例文件，README 注明"calculator 演示 `reads`/`writes` 声明，纯工具执行运行时后续版本支持"。理由：calculator 是纯工具型 Skill，当前平台运行时是 RAG 管线，强行支持纯工具执行会超出 Phase 7 范围（属新能力）。**拒绝：** 全部可运行（超范围）；换成另一个 RAG 类模板（多样性展示变弱）。
- **D7-11: DIST-09 eval 结果用手工快照 + 日期标注写入 README。** 本阶段手工跑一次 frozen eval（`cmd/eval --frozen`），把 per-Skill 指标表（faithfulness % / context_precision % / refusal_accuracy %）+ 运行日期 + lock 文件版本写进 README「Eval Results」节。DIST-09 的"latest CI run"修订为"latest published run (date)"。理由：06-03 CI gate 工作流已推迟，把 CI 自动化拉回 Phase 7 会范围膨胀；手工快照足以满足"published eval results"的品牌叙事。**拒绝：** 补齐 CI 自动更新（拉回已推迟的 06-03，范围膨胀）；静态一次性表格无日期（不可信）。
- **D7-12: DIST-10 eval 样本集用完整 50 例金标集（`eval/golden/compliance.jsonl`）。** 直接发布现有的 50 例完整集作为 `eval/sample.jsonl`，与运行时一致、内容完整。理由：50 例已是"sample"规模（非全量生产集），完整发布让陌生人能复现评估；金标集内容为合成测试数据，无敏感信息顾虑。

### Claude's Discretion
- **README 结构与章节顺序：** researcher/planner 决定。品牌目标是"陌生人 fork-run-benchmark"，README 应 hero 区放 value prop + demo GIF + quick start，架构图与 contributing 放后半。SC-2 列出的必含元素（badges、value prop、demo GIF、quick start、architecture diagram、contributing、license）须全部覆盖。
- **CONTRIBUTING.md 内容深度：** DIST-07 要求含"how to write a Skill"、"add a tool"、"add a knowledge tier"三节。具体写作深度与代码示例由 researcher 调研现有 Skill/tool/knowledge 机制后由 planner 决定。
- **GIF 录制内容脚本：** 录哪些场景（带引用裁决 / 结构化拒答 / counter-evidence / redline 等）由 planner 结合 Phase 4/5 已交付的 UI 能力决定。时长建议 30-60s。
- **docker-compose 服务编排细节：** healthcheck、依赖顺序（etcd→minio→milvus→backend→frontend）、数据卷、端口映射由 planner 设计；`.env.example` 的内网占位需替换为 compose 友好的默认值（如 `milvus:19530`）。
- **架构图（SC-2 architecture diagram）形态：** Mermaid 源码内嵌 README vs PNG。倾向 Mermaid（可维护、GitHub 原生渲染），planner 确认。
- **license 文件：** Apache-2.0 全文 + 每个 Go 包的版权头策略——planner 决定是否加 header（`eval/NOTICE` 已有 Ragas 归因先例可参照）。
- **examples/ 校验器：** 复用 Phase 2 的 7-field validator（SYSK-08 temperature 已含）校验三个模板；planner 决定是否暴露一个 `cmd/skillforge validate <path>` 子命令。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 项目意图与品牌定位
- `.planning/PROJECT.md` §What This Is / §Core Value / §Requirements — 开源目标"打磨文档 + 易上手 + 可衡量效果（个人品牌）"、Apache-2.0 决策、demo Skill 定位
- `.planning/REQUIREMENTS.md` §Open-Source Distribution (DIST) — DIST-01..10 完整需求定义（本阶段唯一需求来源）
- `.planning/ROADMAP.md` §Phase 7: Open-Source Release — Goal / Mode(mvp) / Depends(Phase 6) / 8 条 Success Criteria / Pitfalls(P-OSS-1..6)

### 已交付资产（发布物的来源）
- `.env.example` — 现有环境变量清单（含内网占位，D7-09 需清理）；SC-3 只剩"缺值启动报错"增强
- `eval/golden/compliance.jsonl` — 50 例金标集，D7-12 的发布来源
- `eval/frozen/compliance.jsonl` + `eval/frozen/compliance.lock.json` — D7-11 手工快照的运行对象（`cmd/eval --frozen`）
- `eval/README.md` + `eval/NOTICE` — 评估方法论 + Apache-2.0 归因（Ragas faithfulness prompt）先例
- `skills/compliance/SKILL.md` — 唯一可运行 Skill 模板，examples/ 的参照基准
- `knowledge/authority/*.md` — 7 篇预置合规语料，DIST-06 sample docs 的来源
- `.planning/phases/06-eval-dashboard-production-hardening/06-CONTEXT.md` D6-09..12 — 06-03 CI gate（eval-gate.yml）的推迟决策，影响 D7-11 的 CI 自动化判断

### 先前阶段决策（约束本阶段）
- `.planning/phases/01-integration-spike-trust-contract/01-CONTEXT.md` — 内网约束（Milvus + BGE-M3 走内网，GLM 走公网）、docker-compose 部署路径推迟到本阶段的确认
- `.planning/phases/02-compliance-vertical-slice/02-CONTEXT.md` — 双 retriever 后端抽象（`RETRIEVER_BACKEND=milvus|mongo`）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`.env.example`**：已存在且质量高（每变量带注释、含 backend 选择说明）。SC-3/DIST-03 几乎免费——只需把内网占位替换为 compose 友好默认值 + 确认缺值启动报错路径。
- **`eval/golden/compliance.jsonl`（50 例）**：D7-12 直接发布为 `eval/sample.jsonl`，零额外构造。
- **`eval/frozen/` + `cmd/eval --frozen`**：D7-11 手工快照的现成运行路径，无需新工具。
- **`skills/compliance/SKILL.md`**：examples/ 的 compliance 模板直接复用或软链；simple QA + calculator 是新建但格式可照抄。
- **`knowledge/authority/`（7 篇）**：DIST-06 sample docs 来源已就绪。
- **`internal/eval/` 校验逻辑 + Phase 2 的 7-field validator**：D7-10 的 examples 校验可复用。

### Established Patterns
- **Apache-2.0 归因先例**：`eval/NOTICE` 已为 Ragas faithfulness prompt 做了归因——LICENSE 文件 + 包头策略可参照此模式。
- **内网/公网分离约束**：`RETRIEVER_BACKEND`、`BGE_M3_URL`、`GLM_API_KEY` 三个外部依赖点已在 `.env.example` 模式化，compose 只需把它们参数化。

### Integration Points
- **docker-compose.yml / Dockerfile / README / LICENSE / CONTRIBUTING / examples/**：全部是仓库根的新建文件，无与现有代码的冲突点。
- **`git filter-repo` 历史重写（D7-08/09）**：是发布前对仓库元数据的操作，不触碰产品代码内容，但会重写所有 commit hash——必须在所有文档/GIF 产出完成后、转 public 前执行。

</code_context>

<specifics>
## Specific Ideas

- **仓库名 `skillforge`**：用户明确选定，当前目录 `ai-agent` 不作公开名。发布时 GitHub 仓库路径应为 `ninedrunk/skillforge`（待确认用户名）。
- **发布物极简理念**：用户两次倾向"精简"——拒绝内置 demo 模式（D7-05/06）、拒绝 GHCR 预构建（D7-04）。贯穿原则：**代码即真相，外部依赖（LLM key、embedding 服务）由用户自备**。
- **护城河叙事优先级**：Milvus standalone 全套（D7-01）的资源开销用户愿意承受，因为 tier-filter 是核心展示点——README/架构图应突出真源分离 + tier-filter + 信任评估这三个已验证的护城河。

</specifics>

<deferred>
## Deferred Ideas

- **CI 自动更新 README eval 指标（06-03 eval-gate.yml 合并）**：属已推迟的 Phase 6 工作。Phase 7 用手工快照（D7-11）绕过；真正的 CI 自动化在后续把 `eval-gate.yml`（commit `fc26d12`，未合并 worktree 分支）合并 + 牺牲性 PR 时处理。
- **纯工具型 Skill 运行时支持（让 calculator 模板真正可运行）**：calculator 作为 examples/ 模板只需校验通过（D7-10）；让平台支持纯工具执行是新能力，属后续版本。
- **在线 live demo 服务**：用户明确拒绝部署（D7-05）。若未来想提供在线体验，需重新评估 key 成本/滥用防护（限流、配额、可观测）。

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md`（todo.match-phase score 0.6，跨阶段反复关键词误匹配）—— 跨 Phase 2-6 已多次评审：SYSK-08（temperature 必填、`WithTemperature`）在 Phase 2 的 7-field validator 已实现，frozen 集的 `prompt_hash` 已捕获渲染 temperature。非 Phase 7 工作。

</deferred>

---

*Phase: 7-open-source-release*
*Context gathered: 2026-08-13*
