# Changelog

All notable changes to SkillForge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Versioning note (2026-11-09).** The milestone numbers below follow
> `.planning/ROADMAP.md`. The repository's actual git tags are `v1.0`, `v1.4`,
> `v1.5`, `v1.5-archive` and `v1.6` — **v1.2, v1.3 and v1.7 were never
> tagged**. The current development version is **v1.7**.

## [Unreleased] — 2026-11-09 · 信任边界兑现 + 评测门禁真正可跑

这一批的主题不是加功能，是**让已经宣称的机制在代码里真的成立**，并修掉两处
让门禁无法运行的根本阻塞。

### Added

- **最小身份层**（`internal/server/identity.go`）：每浏览器一个 owner token
  （`X-Owner-Token`），会话归属存于 `session_meta.owner_token`。会话列表按
  归属过滤（过滤写在 SQL 里）。审批裁决要求 admin token 或归属匹配；无归属
  的历史会话 fail-closed 要求 admin。
- **`cmd/eval` 支持 `RETRIEVER_BACKEND=embedded`**。此前它只认 `mongo` /
  `milvus`，遇到 `embedded` 会打一句 WARN 然后**回落到 milvus** —— 于是在
  任何没有 Milvus 的机器上（本机和绝大多数想跑一遍门禁的开发者），
  **评测门禁根本跑不起来**。这是冻结锁长期无法重冻的根本原因。
  语料向量仍由外部 BGE-M3 计算，embedded 只把**向量存储**放进进程内，
  不违反「向量化不可降级」原则。
- **CI workflow**（`.github/workflows/`）：`ci.yml`（build / vet / `go test
  -race` / 覆盖率 / 前端 tsc+vitest+build / `verify-phase-41.sh`）、
  `eval-gate.yml`（两级：无密钥的刚性 `lock-integrity` + 需 secrets 的
  `live-eval`）、`release.yml`。此前 README 引用的三个文件**全部不存在**。
- **前端 ESLint**（`web/eslint.config.js` + `lint`/`typecheck`/`verify`
  scripts）。31 处历史基线违规分类归档于 `web/eslint-baseline.md`。

### Changed

- **三份 system prompt 副本收敛为一份**（`internal/agent/prompts.go`）。
  生产 chat 路径（`VerdictPipeline`）此前**完全不经过 platform floor** ——
  `internal/server/` 全目录 `ComposePlatformPrompt` 零调用。现在三条 chat
  路径共用同一份角色模板，全部经 floor 包装。
- **SKILL.md 的 `steps` / `guardrails` 不再是死字段**：渲染进 system prompt
  的「Skill 契约」段，夹在 floor header 与 tail 之间。`sanitizeSkillText`
  整行丢弃 Skill 正文中伪造平台段落的行。
- **审计行的 `prompt_hash` 现在哈希实际发送的那份 prompt**（此前哈希的是
  一份从未发送的渲染结果）。
- **`GET /api/audit` 改为 admin-only**。原 `redact()` 只在超过保留期（默认
  90 天）时脱敏，保留期内的 query 原文与 verdict 对匿名调用方原样返回。
- **`GET /api/knowledge/doc` 与 `/api/knowledge/chunk/:id` 增加 tier 校验**。
  chunk id 是递增整数可枚举，此前权威库原文可被任意匿名客户端整篇取走。
- 热重载现在真的生效（registry 收到真实 embedder）；语义路由
  （`Deps.Router`）已装配 —— 此前两者都是死路径。
- `EMBEDDER_BACKEND=local` 从生产 env 路径移除。向量化只剩外部 BGE-M3，
  端点不可达即失败。
- **`data.lock.json` / `deep.lock.json` / `fanout.lock.json` /
  `hitl.lock.json` / `kpi.lock.json` / `platform-floor.lock.json` 的核查结果**：
  只有 `compliance.lock.json` 是真正的评测集 pin。其余 4 个无任何代码或测试
  引用，其中 `deep` 的 `prompt_hash` 是全零、`kpi` 的是字面占位符文本。

### Fixed

- **前端从不消费 `data-decision`，路由兜底提示无处落地。** grep `web/src` 里
  `ambiguous` / `data-decision` / `skillScores` / `SkillPicker` **全部零命中** ——
  后端 `chat.go` 注释反复写「the FE renders the SkillPicker with the SkillScores」，
  但前端从未实现。后果：后端判 ambiguous 时用户看到纯空白，连「请选择 Skill」都没有。
  本次补上：`App.tsx` 新增 `routeNotice` state + 扫描 `data-decision` 的 effect，
  `ChatColumn.tsx` 在 `PlanTracePart` 之后渲染提示条 ——「已自动选择 Skill X
  （最高匹配度 N%），可能不是你要的知识域」。仅在 `ambiguous && selected !== ""`
  时出现（后端放弃时没有答案可误解，不该再叠提示）。`tsc --noEmit` 通过。
- **拒答建议文案把内部 tier 标识符拼进了用户可见的中文句子。**
  实测：「…可尝试调整问题，例如：「**authority** 公司年会在哪个酒店办？预算多少？」。」
  根因 `internal/server/pipeline.go:1083` 用 `keyword = tiers[0]`（raw slug），
  而同一文件里就有 `tierDisplayName(reg, tier)`（slug → registry DisplayName，
  26-06 约定里 CN 标签的唯一归属）没被用上。改为 `tierDisplayName(reg, tiers[0])`
  → 「权威 公司年会在哪个酒店办？」。nil registry 时原样退回 slug，nil-reg
  降级断言不变。顺带暴露**双副本漂移**：`internal/agent/trust_floor.go:291` 有同名
  函数，优先 DisplayName 且放**后缀**；pipeline 这份优先 raw slug 且放**前缀**。
- **SHACL 启动报 7 条 error 并把 7 个 SubCategory 全部隔离。**
  7 条是**同一个根因**：`emitter.go:264` 生成 `sub_*` 的 `belongsToTier` 时写成
  `skf:cat_%s`（指向 KBCategory），而 262 行给 `cat_*` 写的是 `skf:%s`（指向 Tier，
  正确）—— 264 行抄错了，违反 shapes.ttl 的 `sh:class` 约束。
  只改代码不够，数据侧要走完这条链才闭环：`emitter.UpgradeIfNeeded` 是**版本驱动**
  的（v7 不变），不会重生成已有 TTL；而 `onstore.Restore` 会把 TTL 的 triple 用
  `InsertUnique` **合并**进快照 —— 快照里修好的 `#sql-guides` 与 TTL 里的
  `#cat_sql-guides` 成了两条不同 triple（989 + 7 = 996）。**更关键**：
  `main.go:1068` 启动末尾 `WriteSnapshot` 会把这 996 条写回快照，固化脏数据。
  最终修复：改 `data/ontology/skillforge_tbox.ttl` 的 7 行（`skf:cat_` → `skf:`），
  并从 `data/ontology/skillforge_kg.json` 删除那 7 条残留错误 triple（996 → 989）。
  **实测**：`{"health":"ok","error_count":0,"warning_count":0,"quarantined":null,"triple_count":989}`；
  端到端回归无变化（带 skill 105 字/1 引用；不带 skill 105 字/2 引用；域外 27 字拒答）。
- **`/api/chat` 在用户显式指定 Skill 时仍被语义路由的模糊门禁否决，导致返回空答案。**
  实测：问「出差住宿费超标 200 元，能批吗？」（compliance 的 description 里就写着
  这个触发短语）时，`top1=compliance 0.5226`、`top2=external-lookup 0.4488`，
  `gap=0.0738` 只比 `AmbiguityMargin=0.08` 小 **0.0062**，于是被判为 ambiguous →
  `Selected=""` → 不跑 VerdictPipeline → SSE 只有 `data-decision` + `finish-step`，
  **answer_len=0、sources=0**。即使请求体里明确写了 `"skill":"compliance"` 也绕不过，
  因为 `chat.go:275` 无条件调用 `routeThenRun`。
  根因是数据流上丢失了「用户是否显式指定」这一信息：`decodeChatRequest` 在填默认值
  **之后**才返回，下游无法区分「用户说了 compliance」和「用户没说、默认 compliance」。
  修法：`decodeChatRequest` 多返回 `explicitSkill`（在填默认值**之前**采样），
  `chatHandler` 的路由条件加 `&& !explicitSkill`。门禁的用途是「用户没说时避免猜错」，
  不是「用户说了还要再赌一次」。
  **实测效果**：同一句话从 `events=2 / answer_len=0` 变成 `events=16 / answer_len=105`，
  返回 `结论：compliant（置信度 98%）。依据 权威 tier 条款：「员工出差住宿费超过标准
  额度时，200 元以内的超标部分可由部门负责人审批…」`，引用
  `authority/travel_expense.md · chunk 13`（tier=authority，附审计 turnId）。
  语料外问题仍正确拒答（`未检索到可回答该问题的授权知识源`，sources=0）。
  **仍未解决**：不带 `skill` 的直接提问（前端默认行为）依然走路由门禁，仍返回空 ——
  需要「ambiguous 时也执行 top1 并标注」的改动，待定。
- **语义路由把两种性质不同的「模糊」混为一谈，导致不带 `skill` 的直接提问拿不到任何答案。**
  上一项修了「显式指定被否决」，但前端默认不带 `skill`，所以仍走路由门禁：
  `top1=compliance 0.5226 / top2=external-lookup 0.4488 / gap=0.0738 < 0.08` →
  `Ambiguous=true` → `Selected=""` → 不跑 VerdictPipeline → `answer_len=0`。
  原实现把 (a)「没有任何候选相关」（top1 < `AmbiguityLowFloor`=0.4）和
  (b)「有多个相关候选但咬得近」（gap < `AmbiguityMargin`=0.08）当成一回事。
  前者猜一个**有害**（会把无关 Skill 的拒答复述出来，掩盖「这是域外问题」）；
  后者 top1 本身是相关的，**兜底执行它显然优于让用户面对一行空白再点一次**。
  修法：**用 `Selected` 是否为空区分两档，不改 wire 格式**——
  (a) 保持 `Selected=""` + `Ambiguous=true`（前端渲染 SkillPicker）；
  (b) 设 `Selected=top1` 且 `Ambiguous` 仍为 `true`（前端出答案 + 提示
  「已自动选择 X，可能不是你要的知识域」）。`chat.go` 的短路条件相应改成
  `result.Ambiguous && result.Selected == ""`。
  **实测（同一问题，三种调用方式）**：
  · 带 `skill` → `answer_len=105`，引用 `travel_expense.md · chunk 13`，confidence 0.98
  · **不带 `skill` → `answer_len=105`，引用 2 条**（新增
    `expense_reimbursement_scope.md · chunk 6`），`route: selected=compliance ambiguous=true`
  · 不带 `skill` 问域外问题 → `answer_len=27`「该问题超出当前 Skill 的处理范围」，
    `sources=0`，未编造
  `internal/skill/router_test.go::TestRouter_Ambiguous_Top1Top2Close` 的断言同步更新
  （它原本钉死 `Selected==""`，即被改掉的旧语义）。`go test ./internal/skill/` 通过。
- **`TestNoSQLGenerationSurface` 在 HEAD 上就是红的**（引入于 `3a99605`，
  与本批无关）。它不是放宽，而是把表达收紧到意图：`stubchat` 是只读 SQL
  解释器，零执行入口，因此加入扫描跳过表，**同时新增**
  `TestStubChatHasNoSQLExecutionCapability` 直接断言该包永远不得获得 SQL
  执行能力 —— 比被替代的关键字扫描更严格。
- **`verify-phase-41.sh` 的 (b) 段此前直接 `exit 1`**（它跑
  `internal/server/platform/verify_main.go`，该文件从未进入 git），导致其后
  所有 tier 隔离断言都是死代码。已改为跑真实测试并加 `^--- PASS` 防假绿灯；
  (e) 段从整文件 grep 改为函数体级断言。
- **主索引器补上 chunker**：此前超过 `maxDocBytes` 的权威政策文档被**静默
  跳过**，检索永远找不到且系统不报错。现在递归切分（分隔符阶梯与
  `ecl/chunk.go` 一致，预算 3500 runes），`maxDocBytes` 降级为纯告警阈值。
- **MCP 的 `skf_neighbors` 绕过了分层过滤**（裸用 `kg.Query`，与
  `main.go` 的公开声明矛盾）→ 改走 `QueryScoped`。`resolveSkillTiers`
  改为三态归一且永不返回非 nil 空切片。新增 14 个离线测试（该目录此前零测试）。
- **`docker-compose.yml` 补 `SKILLFORGE_MCP_SERVICE_SKILL`**。backend 启动时
  对它做 `log.Fatalf` 校验，而 compose 从未透传 —— README 的
  `docker compose up` 头号路径必然崩溃循环。
- **测试临时根移出源码树**（`internal/server/import_api_test.go`）。此前建在
  包目录内且 `os.RemoveAll` 错误被丢弃，Windows 上每跑一次就沉淀几百 MB，
  实测累积 72 个目录 / 1.68 GB。删除失败现在会告警而不是假装成功。
- 前端 10 个文件的编码损坏（`tsc` 报大量 "Unterminated string literal"）
  已恢复并重做必要部分。

### Notes

- **`TestShippedFrozenLockIsInSync` 曾红，现已转绿** —— 通过真的重跑评测
  并更新 `compliance.lock.json` 的 `prompt_hash`
  （`5b12acd9…` → `e0081be1…`），不是改断言。
- **实测评测耗时**：50 例全量 `-frozen compliance` 每例 2–4 分半（agent +
  judge 各一次 GLM 调用），**全量需要 1.5–3 小时**。因此 `eval-gate.yml`
  的 `live-eval` 不应按「每个改 `skills/**` 的 PR 都跑全量」设计 —— 必须用
  子集（`-cases`）或只在 release 前跑全量。
- **重冻后的对抗子集复测**（20 例，`prompt_hash=e0081be1…`）：faithfulness
  **0.8764 → 0.9602**，`context_precision` **0.9808 → 0.9808**（检索未动，
  符合预期）。17/20 例满分，3 例非满分均可归因到召回。**本批 prompt 改动
  未降质。** 50 例全量在新 prompt 下尚未重跑。
- **SC-6「数值完全一致」实测不成立。** `internal/eval/frozen.go` 声称同一
  冻结集重跑返回 numerically identical 的分数，并设了
  `LiveReproducibilityTolerance = 1e-3`。实测：同一 `prompt_hash` /
  `model_ver` / `seed` / case，两次运行 `adv-001` 与 `adv-002` 的
  faithfulness 分别是 `0.500` 和 `1.000` —— 单次噪声 ±0.5，是声明容差的
  500 倍。**在此之前，用单次 `--frozen` 分数当晋级门禁不严谨**；需要
  judge 侧确定性化，或改为对多次重复取均值并写清置信区间。
- `LOCAL_BASE_URL` 会覆盖 `GLM_BASE_URL`。若本地 LLM 端点不可达，需清掉
  `LOCAL_*` 才会回落到云端 GLM。



**Open-Source Readiness** — Phases 47-51, all Complete
(47: 2026-10-19 · 48: 2026-10-21 · 49: 2026-10-22 · 50 + 51: 2026-10-24).
The tag has **not** been cut — the latest tagged release is still `v1.6`.

- **Phase 47 — 清债与可复现基线**: v1.6 leftover debt closed or explicitly
  dispositioned. Edge-direction determinism (root cause: `map` iteration order,
  not float precision), rollback governance-cache refresh (audit WARNING-1), and
  a disposition record for 19 leftover items.
- **Phase 48 — CI 矩阵**: Go / FE / Docker / verify gates on GitHub Actions;
  the public four-green run (first all-green `39380373930`).
- **Phase 49 — Hermetic Demo**: embedded vector store + stub LLM path — clone →
  running with no internal-network Milvus / BGE-M3 and no API key.
- **Phase 50 — 开源门面** and **Phase 51 — 发布工程**: README facade,
  architecture map, comparison tables, community files; goreleaser plus a
  tag-triggered GitHub Release.
- **Phase 52 — LLM Provider 配置面板** (landed after the five v1.7 phases): an
  admin panel for vendor → plan → key, with a hot-reload chat model that takes
  effect without a restart.
- **Docs (2026-11-09)**: the outward narrative switched from "Skill-driven" to
  "a verifiable enterprise agent runtime — gates, not scores". The three moats
  are renamed approve-and-remember / **provable evidence** / **provable
  execution + provable evolution**; the README gained a "What is not on the
  production path yet" section, and the eval table was corrected against
  `eval/report.json`.

## [v1.6.0] — 2026-10-08

**SPARQL Schema Detection + Entity SHACL Shapes** — Phases 45-46.

- **D-SPARQL-05 (β semantics)**: subject-based schema detection —
  `schemaSubjects` ∩ a 13-predicate allowlist, derived at query time with zero
  persisted state. A Skill's `hasReadAccess` becomes visible in the admin
  baseline, and D-TIER-03 fail-closed is preserved.
- **Entity SHACL shapes**: entity-class ABox constraints (`entities.ttl`),
  severity moved out of hardcoded engine defaults into the shapes data,
  boot / rollback shape paths made env-configurable, violations routed into the
  existing quarantine flow, governance rendered in three views.
- 7 plans green; `verify-phase-45.sh` and `verify-phase-46.sh` both exit 0;
  milestone audit 6/6 requirements + 11/11 wiring.

## [v1.5.0] — 2026-10-02

**Outstanding Closure** — Phases 42-44 (closure only, no new features).

- The Phase 42 operator runbook was executed 2026-09-30 / 10-01: 8 PASS +
  L8 PARTIAL + L4 / L7 env-SKIP; live 抽取 F1 improved 0.39 → 0.87.
- Phase 44: SPARQL test fixes 10/10 green, D-TIER-03 restored.

## [v1.4.0] — 2026-09-23

**Ontology Layer** — Phases 38-41.

- **Phase 38 — Foundation**: TBox + memory-map KG + auto schema lift
  (`internal/ontology/{emitter,store,induction}` + 8 admin routes + 本体 tab).
- **Phase 39 — ECL pipeline**: section-aware chunker + LLM entity extraction with
  ABox dual-write to KG and Milvus; golden F1 tables 0.9846 / related 1.0.
- **Phase 40 — Data governance**: SHACL + provenance + TBox versioning.
  Violations never block boot (quarantine flow); tier isolation D-TIER-01/03/06/07.
- **Phase 41 — Platform layer**: MCP stdio server (5 tools) + `POST /api/sparql`
  (skill-scoped tier isolation) + hybrid retrieval (KG 1-2 hop → Milvus → rerank).

## [v1.3.0] — 2026-09-09

**Code Quality** — Phases 31-37 (23/26 requirements closed; trust semantics preserved).

- Phase 31: the FE hostname whitelist becomes runtime-configurable, and
  `evalStatus` no longer sticks on loading.
- Phase 32: BE error matching switched to typed sentinels, and the O(n²) string
  concat in the hot path replaced by `strings.Join`.
- Phases 33-35: SRP splits (handlers ≤ 200 lines), HITL handler template
  extraction, HTTP-layer deduplication.
- Phases 36-37: FE deduplication and silent audit-write failures fixed; store
  parameterization, constraints and concurrency guarantees.

## [v1.2.0] — 2026-09-08

**Knowledge Ops** — Phases 26-30.

- Phase 26: knowledge-base registry foundation — knowledge bases become runtime
  registry entries carrying one of four semantic roles, and the role (not the
  KB's own switch) decides what is citable.
- Phase 27: document upload pipeline — admin-side upload with a format and
  magic-byte allowlist, in-memory parse → chunk → embed → insert, plus an audit row.
- Phase 28: dynamic knowledge graph — N-dynamic nodes (one per active KB),
  deterministic relation edges, neighbour and path APIs.
- Phases 29-30: feedback loop + ticket interface, then the milestone hygiene gate.

## [v1.1.1] — 2026-09-07

v1.1.1 closes the Phase 21 integration gap flagged by
`.planning/reports/MILESTONE_SUMMARY-v1.1.md` (Gap 1, HIGH): the External
Authorized Channel shipped in v1.1.0 as a fully tested library + HTTP
surface, but nothing could reach it in a running server. No breaking
changes.

### Added

- **`needs_external_query` agent tool** (`internal/agent/tools/external_query_tool.go`)
  — the only agent-visible entry into the channel: pass-through NL2SQL
  preview → pending approval → masked rows after approval, with
  structured refusals (never Go errors). Registration is fail-closed: a
  Skill must declare `reads: [data:external:authorized]` AND the server
  must have wired the manager, otherwise `BuildRunner` refuses to build.
- **`external_views/` controlled-file view registry** +
  `externalauth.LoadViewsDir` (DATA-10) — the only source of view
  registrations at runtime; fail-closed on malformed files, empty
  directory is the single legal degraded state. Ships a reference view
  `external_views/v_user_consumption.yaml`.
- **`eval/frozen/external-auth.jsonl`** — 10-case golden set covering
  DATA-04/05/06/10/11/13/14, exactly 3 refusal-required. Deliberately
  unpinned (no `.lock.json`) until an authorized live replay mints one.
- Published example Skill `examples/skills/external-lookup` (approve-
  before-query flow, 4 prohibitions).

### Fixed

- **External Authorized Channel is reachable** — `externalauth.Manager`
  is now constructed in `cmd/server` (assembly section 6c-5) behind the
  repo's declared `EXTERNAL_AUTH_ENABLED=1` opt-in gate; the six
  `/api/external-auth/*` routes no longer answer a permanent `503` in a
  wired deployment. With the gate unset the behavior is unchanged
  (fail-closed `503`).
- Sensitive channel surfaces (`pending` / `approve` / `reject` /
  `execute` / `reconcile`) now require `X-Admin-Token` (constant-time
  compare); the admin check runs before the nil-manager `503` so a `403`
  never leaks channel state. `POST /api/external-auth/request` remains
  intentionally agent-facing and un-gated in v1.1.1 (documented in the
  README channel section; hardening queued for v1.2).
- `reads: [data:external:authorized]` now validates (DATA-07 second
  half) — the two data-channel constants are the complete allowlist and
  unknown `data:*` values still fail closed.

### Docs

- README: architecture paragraph + diagram labels now name the same
  three moats as the moats section (approve-and-remember /
  trust-contract kernel / CI-integrated trust gate); new **External
  Authorized Channel** section with the honest ✅/⚠️/❌ boundary table.
- `.planning/REQUIREMENTS.md`: DATA-04/05/06/07/10/11/13/14 rows
  reconciled with the shipped code.
- `.planning/phases/21-external-auth-channel/21-SUMMARY.md`: honest
  post-hoc phase record backfilled (a955450 + this cleanup).

## [v1.1.0] — 2026-09-06

v1.1 is a **trust-architecture maturation** release. No breaking changes —
all additions are backward-compatible wire shapes (additive fields,
omitempty) and additive store tables (DDL migrations auto-apply).

### Added — Trust Contract Hardening (Phase 24, audit chain)

- **PROV-O export endpoint** `GET /api/session/:id/audit/prov` — INSERT-only
  audit rows are now mapped to W3C PROV-JSON (Entity / Activity / Agent
  + wasGeneratedBy / used / wasDerivedFrom). Zero LLM, deterministic,
  passes through the existing retention redaction. Read by Semantica
  users via standard PROV tooling.
- **Decision-chain endpoint** `GET /api/session/:id/decision-chain?limit&at` —
  every audit row for the session, classified into a chronological
  Link list (verdict / pending_approval / approved / rejected /
  rule_created / rule_revoked). FE AuditTimelineTab renders this as a
  vertical timeline.
- **Time-travel state endpoint** `GET /api/session/:id/state?at=RFC3339` —
  reconstructs the per-session T-point snapshot (visible turns +
  effective approval-rules). Revoked rules are rebuilt from the
  `action_rule_revoked` row's `frozen_args` column.
- **New package `internal/auditchain`** — pure Go reconstruction
  helpers (6 unit tests covering verdict / approval / create / revoke
  / skill-scope / session-scope / empty-session / ordering invariant).
- **New package `internal/auditprov`** — PROV-JSON encoder.

### Added — External Authorized Channel (Phase 21)

New `internal/externalauth` package + 4 HTTP endpoints — implements
**DATA-04/05/06/10/11/13/14** from `REQUIREMENTS.md`:

| Need | Implementation |
|---|---|
| DATA-04 TTL token | `externalauth.TTLToken` + `Manager.ExecuteWithToken` + 5-min default lifetime + `token.Valid()` guard |
| DATA-05 二级脱敏 | `MaskSpec` (account last-N / phone / email / generic) + `View.Mask` + `ApplyMask` |
| DATA-06 双重审计 | `Source.AppendExternalAudit` + `Manager.Reconcile` + `query_id` pair matching |
| DATA-10 预编译视图 | `ViewRegistry` whitelist + `ViewNamePattern` + `ErrViewNotAllowed` |
| DATA-11 NL2SQL 预览 | `internal/nl2sql.Generate` + `RejectNonSelect` AST gate (rejects INSERT/UPDATE/DELETE/DDL/CALL) |
| DATA-13 持续挂起 | `pending` state is permanent without explicit Approve / Reject; **no auto-reject, no auto-timeout** |
| DATA-14 audit row schema | `external_auth_audit` 9-field schema with `MaskApplied` column |

Endpoints:
- `POST /api/external-auth/request` — create approval
- `GET /api/external-auth/pending` — data-officer dashboard
- `POST /api/external-auth/approve` / `/reject` — explicit human resolution
- `POST /api/external-auth/execute` — agent's post-approval query
- `GET /api/external-auth/audit/reconcile` — dual-audit pair mismatch

7-step end-to-end release-gate test (`TestUAT_FullChannelFlow`)
covers all 7 DATA invariants. UAT doc:
`.planning/uat/phase21-external-auth-2026-09-06.md`.

### Added — CI-Integrated Trust Gate (Phase 22)

- `.github/workflows/eval-gate.yml` — every PR + main push runs
  `cmd/eval --frozen compliance --gate-check` (50-case frozen replay
  against `examples/skills/<skill>/eval-gate.json` thresholds). Fail-closed
  exit code; no auto-merge.
- `eval/frozen/compliance.jsonl` — 50 cases now carry `source` annotation
  (spike-anchor / closed-loop-2026-09-06 / negative-test) and
  `corpus_file` attribution back to `knowledge/authority/*.md`. The
  3 spike-anchor cases (adv-001/002/003) had GBK mojibake from a
  historical terminal encoding bug; rewritten as UTF-8 verbatim copies.
- **README §3 "评测定位声明"** — SkillForge does NOT ship an independent
  evaluation platform. `internal/eval` is internal infrastructure that
  backs the Skill promotion gate. If you want a standalone RAG
  evaluation suite, use [Ragas](https://docs.ragas.io/) directly.

### Added — Eval Trust-Loop Closure (Phase 25)

- 13 ground-truth entries added to `knowledge/authority/` (4 rewrites +
  9 new topical docs: annual_leave carryover / overtime_pay / sick_leave /
  maternity_leave / probation / supplier_management / travel_meals /
  travel_flight_class / travel_personal_delay / expense_documents).
  All 30 should-answer questions now ≥ 50% shingle coverage.
- `cmd/eval`:
  - `--cases <id,id>` — diagnostic subset mode
  - `--dump <path>` — per-case raw verdict dump (query / answer /
    refusal / chunk_scores) for offline harness-vs-model dispute triage
- `CaseResult.RefusalReason` additive + `Report.AgentErrors` counter —
  harness faults (agent_error) are excluded from means and counted
  separately. No more 2/50 infrastructure fault masquerading as
  refusal collapse.

### Added — Local Model Baseline

- New env triple `LOCAL_BASE_URL` + `LOCAL_API_KEY` + `LOCAL_MODEL` —
  switch the chat model + judge to a self-hosted OpenAI-compatible
  endpoint. Precedence `LOCAL_*` > `GLM_*`; if neither is set, the
  default Coding Plan endpoint is used.
- `internal/llm.ExtraFieldsModel` decorator — `LOCAL_LLM_THINK=false`
  injects ollama's `think:false` body field on every Generate /
  Stream call. Reaches the ReAct agent loop (flowagent has no chat-
  model-option passthrough in eino v0.9.14).
- Budget tunables: `HISTORY_LAYER_TIMEOUT` (rewrite + classifier
  per-call), `VERDICT_STREAM_TIMEOUT` (verdict stream), `EVAL_CASE_TIMEOUT`
  (cmd/eval per case). Cloud-tuned defaults preserved.

### Changed — README 差异化叙事重排 (Phase 23)

- README §2 "Why SkillForge — The Three Moats" rewritten from
  tier-filter / Skill-as-Platform / Go-eval to **approve-and-remember
  primitive / trust-contract kernel / CI-Integrated Trust Gate**.
- New §"SkillForge vs WeKnora vs Semantica — three lanes" comparison
  table positioning SkillForge as the execution-time trust boundary
  alongside (not against) the other two.
- New opening tagline + Skill de-confusion note — distinguishes
  SkillForge's `SKILL.md` (domain adaptation contract) from WeKnora's
  Skill (sandboxed skill plugin) and Anthropic Agent Skills (MCP tool).
- "Go 企业 RAG 平台" / "4 信任指标" / "审计链" → demoted to **support
  capabilities**, not headline differentiation.

### Migration notes

No data migration required. New env vars (all optional, defaults
preserve v1.0 behavior):
```
LOCAL_BASE_URL=http://10.x.x.x:8888/v1   # optional
LOCAL_API_KEY=...                         # optional
LOCAL_MODEL=Qwen3.8-27B                    # optional
LOCAL_LLM_THINK=false                      # optional (ollama dialect)
HISTORY_LAYER_TIMEOUT=2s                   # optional
VERDICT_STREAM_TIMEOUT=60s                 # optional
EVAL_CASE_TIMEOUT=90s                     # optional
EXTERNAL_AUTH_ENABLED=1                    # optional (default: disabled)
```

Schema migrations are auto-applied at server start (additive ALTER
TABLE only).

### Dependencies

- `cloudwego/eino@v0.9.14` (locked 2026-08-17) — unchanged
- `cloudwego/eino-ext` (latest) — unchanged
- Go 1.25 — unchanged

---

## [v1.0.0] — 2026-08-30

Initial public release. v1.0 ships:

- 4-tier knowledge isolation (权威 / 历史 / 个人 / 草稿)
- Skill-as-platform (SKILL.md hot-reload, intent routing)
- 3 example Skills (simple-qa, calculator, compliance)
- 4 RAG-trust metrics (Ragas v0.1.21 verbatim port)
- HITL approve-and-remember (Phase 11)
- Admin console (Phase 20)
- KPI pipeline (Phase 17)
- Static report portal (Phase 19)
- 439 vitest + Go test suites green at v1.0 UAT (`.planning/uat/v1.0-uat-2026-09-06.md`)
