# Phase 30: 工程卫生收尾 (Hygiene & Milestone Gate) - Context

**Gathered:** 2026-09-08
**Status:** Ready for planning

<domain>
## Phase Boundary

v1.2 里程碑收尾——不交付新功能，交付「可证」与「清债」：

1. **加固清债**：29-REVIEW 的 CR-01（critical）+ 全部 6 个 warning 修复，各配回归测试锁；STRIDE-30 登记处置
2. **HYG-02**：零硬编码魔数抽查常驻结构门（verify-phase-30.sh）——26-29 新增配置面逐一验证 env/注册表可配 + diff 新增字面量扫描
3. **HYG-03**：phases 26-29 全 SDLC 门证据汇编（单元 + hermetic + WSL 构建 + verify 门 + FE vitest/tsc）+ 信任语义回归双层证明（hermetic + live）
4. **UAT 全部清偿**：27/28/29 三份 runbook 全部项在关账前由 operator 亲自跑完（关账硬门）
5. **顺手卫生**：REQUIREMENTS traceability 补 12 个 PLAT/SCALE/DOMAIN 旧 ID、`.env.example` 补 `SKILLFORGE_ADMIN_TOKEN`、STRIDE-28 归位 `docs/threat-model/`

Requirements: HYG-02, HYG-03（HYG-01 已于 Phase 26 关闭）。

</domain>

<decisions>
## Implementation Decisions

### 加固范围（29-REVIEW 清债）
- **D-30-01:** CR-01 纳入 30 修复——SSE emit 失败路径的错误分支复用已落库 turn 而非重写（消除双 audit/turn 对 + 可投票重复），配回归测试锁（emit-failure 不产生第二 audit/turn 对），STRIDE-30 登记处置；30 验证门证明其关闭。
- **D-30-02:** 6 个 warning 全部修复，各配回归锁：WR-01 投票行与 refs 写入事务化（失败可重试回填，不再永久脱离复核/指标）；WR-02 queued 行 sweep/retry 机制（进程重启后不再 silent loss）；WR-03 webhook http.Client 增加 CheckRedirect 策略（杜绝 X-SkillForge-Ticket-Secret 跨域重定向泄漏 + SSRF pivot）；WR-04 归因邻近回溯加 Action filter（拒收 ChunkIDs="" 的 fanout 行胜出）+ 同 ts tie-break；WR-05 公共反馈/工单端点节流（配置驱动限额）+ session/turn 归属校验（拒绝跨会话猜测写入）；WR-06 FE FeedbackRow 对迟到 data-turn.votedVerdict 的采纳（响应晚于 mount 时仍锁定）。结构性改动（事务化、sweep）都在范围内。
- **D-30-03:** 剩余 8 个 info 逐条裁定：trivial 顺手修，其余记录进 30 验收文档（accept/defer 带理由）。

### HYG-02 零硬编码魔数抽查
- **D-30-04:** 常驻结构门——`scripts/verify-phase-30.sh` 纳入魔数抽查 stages + Makefile `phase-30` target，永久把守。关键字段清单：`SKILLFORGE_GRAPH_MAX_NODES/EDGES/NEIGHBOR_DEPTH_LIMIT/PATH_HOPS_LIMIT`（4）、`SKILLFORGE_TICKET_PROVIDER/WEBHOOK_URL/WEBHOOK_RETRIES/WEBHOOK_TIMEOUT_MS/DISPATCH_BUDGET_MS/WEBHOOK_SECRET`（6）、`SKILLFORGE_UPLOAD_MAX_BYTES`、`SKILLFORGE_KB_SEED`、`SKILLFORGE_MILVUS_RECREATE`、parser 格式白名单（注册表驱动）、语义角色定义（kb registry）。抽查 = 逐一验证配置可驱动（默认值来自 envOr/注册表而非字面量散布）+ 26-29 diff 新增硬编码字面量扫描（测试 fixture 豁免清单显式维护）。

### HYG-03 全 SDLC 门 + 信任语义回归
- **D-30-05:** 证据汇编——phases 26-29 每 phase 出示：单元 + hermetic 集成测试证据、WSL 构建验证（`make check-cgo` 前置）、受影响 verify 门（verify-phase-26/27/28/29.sh）全绿、FE vitest + tsc 干净记录。汇编产物 = 里程碑级验收文档（30 验收报告），逐 phase 逐门打表。
- **D-30-06:** 信任语义回归双层证明——(a) hermetic 层：KNOW-02 fail-closed / D-09 拒答构造 / citation 白名单的结构门 + 冻结 golden set schema/lock 自洽性重跑（零外部依赖）；(b) live 层：`cmd/eval --frozen --gate-check` 真跑一轮（GLM live，faithfulness ≥ 0.9 / refusal_accuracy ≥ 0.95），遵守既有断线纪律：跑前探测内网服务，断线记环境 STOP 不刷分、重跑窗口由 operator 掌握。两层证据都进验收记录。

### UAT 清偿（关账硬门）
- **D-30-07:** 全部清偿——27-UAT + 28-UAT + 29-UAT/29-HUMAN-UAT 三份 runbook 全部项（含 Phase 27→28→29 联合验收与 FBK-05 实机确认）在里程碑关账前由 operator 亲自跑完并记录结果。30 的 plan 把清偿落成显式 operator checkpoint 任务；跑出的 issue 走 gap-closure。

### Claude's Discretion
- traceability 簿记：REQUIREMENTS.md traceability 表补 PLAT-01/02/03/06/07/08、SCALE-01/02/03、DOMAIN-01/02/03 共 12 个旧 ID（phase.complete warning 的机械修复）。
- `.env.example` 补 `SKILLFORGE_ADMIN_TOKEN`（含注释说明 admin 门 fail-closed 语义）。
- STRIDE-28 从 `.planning/phases/28-dynamic-knowledge-graph/` 归位 `docs/threat-model/`（与 6/7 先例对齐）。
- STRIDE-30 的威胁面：本 phase 无新攻击面（加固型 phase），登记 CR-01/WR-01..06 的处置与残余风险即可。
- 30 验收文档的结构与存放位置（建议 `.planning/phases/30-hygiene-milestone-gate/30-ACCEPTANCE.md`，随 plan 定）。
- 里程碑关账仪式（CHANGELOG/RELEASE-NOTES/milestone-summary）不在本 phase scope——`/gsd:complete-milestone` 职责。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope 与需求
- `.planning/ROADMAP.md` §Phase 30 — Goal / Success Criteria（里程碑收尾必须在全部功能 phase 之后）
- `.planning/REQUIREMENTS.md` §工程卫生 (HYG) — HYG-02/03 定义；§v1.2 Out of Scope

### 加固输入（修复对象）
- `.planning/phases/29-feedback-loop-ticket-interface/29-REVIEW.md` — CR-01 + WR-01..06 逐条详情（file:line + 修复方向已给）；8 infos
- `.planning/phases/29-feedback-loop-ticket-interface/29-VERIFICATION.md` — CR-01 判定（继承形态 + 可投票放大面）与 12/12 must-haves 基线

### 纪律与既有门（汇编对象）
- `scripts/verify-phase-26.sh` / `27` / `28` / `29` — 四个 per-phase 门的 stage 结构（30 门沿用 + 扩展魔数抽查）
- `Makefile` — `phase-27/28/29` targets + `check-cgo`（WSL 构建前置）
- `.planning/phases/27-document-upload-pipeline/27-UAT.md`、`.planning/phases/28-dynamic-knowledge-graph/28-UAT.md`、`.planning/phases/29-feedback-loop-ticket-interface/29-UAT.md` + `29-HUMAN-UAT.md` — 待清偿 runbook（D-30-07）
- `docs/threat-model/STRIDE-26.md` / `STRIDE-29.md`（+ 27/28）— 威胁模型族；STRIDE-30 沿用格式
- `.planning/phases/22-eval-ci/22-CONTEXT.md` — 冻结重放纪律（CI 阈值与本地 gate 一致、不接 nightly 自动 replay）
- 记忆纪律（session context）：内网服务（Milvus/BGE/GLM）间歇断——live 前探测，断线记环境 STOP 不刷分

### 代码事实（26-29 配置面，魔数抽查对象）
- `cmd/server/main.go:156-192` envOr/parseIntEnvOrDefault helpers + 全部 SKILLFORGE_* 接线
- `internal/kb`（语义角色/slug 注册表）、`internal/document`（parser 白名单注册表）、`knowledge_bases.yaml`（受控 seed）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `scripts/verify-phase-2[6-9].sh` — stage 结构、负向自证模式（29 Stage 2a 注入即失败）、TODO 清零 regex（26 的 `TODO\(|TODO:` 形态防 eino deep.TODO 误报）
- `cmd/eval --frozen --gate-check` — live 冻结重放与阈值门（faithfulness/refusal_accuracy）
- `TestPlatformFloorGoldenSchema/FrozenSet/LockSelfConsistent`（14-04）— hermetic 信任地板门样例
- `TestFeedbackNoTrustMutation`（29-05）— source-grep 结构锁范式，30 的魔数抽查/加固回归锁照此
- 29 的 5 个 admin/audit test fakes（store.Store nil-embed 模式）— 加固修复的测试基建
- `golang.org/x/time/rate` 或令牌桶——WR-05 节流实现候选（若引依赖需权衡零新依赖纪律，planner 裁量）

### Established Patterns
- per-plan 2-3 tasks + wave 并行 + verify 脚本收尾（26-29 全程惯例）
- STRIDE-xx 每威胁 Disposition 行 + operator runbook（27-09/28-07/29-09 收尾 plan 惯例）
- INSERT-only / additive migration / admin 403-before-503 / 零硬编码 envOr——加固修复不得放松任何一条
- WSL Go 构建 + Windows FE vitest/tsc 双环境验证纪律

### Integration Points
- `internal/server/chat.go:288-328` + `pipeline.go:543-559` — CR-01 修复点（错误分支）
- `internal/server/feedback.go` + `internal/store/sqlite/feedback.go` — WR-01/04/05
- `internal/server/tickets.go` — WR-02/05
- `internal/ticket/webhook.go` — WR-03（CheckRedirect）
- `web/src/components/chat/parts/FeedbackRow.tsx` — WR-06
- `cmd/server/main.go` — 任何新配置面的 envOr 接线（节流限额等）

</code_context>

<specifics>
## Specific Ideas

- 用户对审查清债采取最严口径（全部修）——30 的性质从「纯收尾门」扩展为「加固 + 收尾门」，planner 按此排 wave：加固修复先行（先于门与证据汇编，否则门扫到已知洞）。
- UAT 全部清偿是 operator 与 AI 的协作项：plan 落成显式 checkpoint 任务（autonomous: false），AI 备好环境与步骤，operator 执行并记录。
- live eval 断线不刷分纪律（env-intranet-service-flakiness）：探测先行，断线 = STOP 记环境，不得用降配重试掩盖。

</specifics>

<deferred>
## Deferred Ideas

- 里程碑关账仪式（CHANGELOG / RELEASE-NOTES / `/gsd:complete-milestone` / `gsd-milestone-summary`）——本 phase 只出验收证据，关账仪式是独立命令职责。
- WR-05 的深度鉴权体系（真实用户身份/登录态）——30 只做节流 + 归属校验；完整鉴权是未来里程碑级能力。
- WR-02 的持久化 outbox 模式（跨重启强投递保证）——30 实现 sweep/retry 机制，outbox 级语义留待真实工单系统对接里程碑。

</deferred>

---

*Phase: 30-hygiene-milestone-gate*
*Context gathered: 2026-09-08*
