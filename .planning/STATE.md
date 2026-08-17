---
gsd_state_version: 1.0
milestone: v0.8
milestone_name: — Harness（可信×行动融合）
current_phase: 08
current_plan: 4
status: executing
last_updated: "2026-08-17T16:06:01.564Z"
last_activity: 2026-08-17
progress:
  total_phases: 13
  completed_phases: 5
  total_plans: 40
  completed_plans: 39
  percent: 38
---

# State — SkillForge

**Last updated:** 2026-08-17 (Milestone v0.8 roadmap created: phases 8-13 appended to ROADMAP.md per research SUMMARY.md strict dependency chain 8→9→10→11→12→13; 25/25 v0.8 requirements mapped in REQUIREMENTS.md traceability; v0.7.7 phases 1-7 + backlog 999.1/999.2 unchanged. Earlier same day: completed quick task 260817-i36: 聊天回答区四项 UI 修复 —「已给出」改为结论角标、引用 chips 去重并可点击跳转、同文档 chip 加 chunk N 区分、conclusionText 标点规范化；4 任务 10 commits，go test ok + vitest 280/280 PASS。同日早前完成 260817-hck VerdictCard 答案去重。)

**Prior:** 2026-08-13 (Plan 07-05 IN PROGRESS — Task 1 DONE: `scripts/eval_snapshot/{main.go,main_test.go}` shipped via TDD RED `abf2f4d` → GREEN `4873213`; `deriveRefusalAccuracy` pure fn is the ONE derivation point for DIST-09 refusal_accuracy [count(Cases cat=refusal-required && Refused) / count(Cases cat=refusal-required)]; 6/6 hermetic unit tests PASS, `go vet` clean, end-to-end smoke render matches RESEARCH Example 6 format; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. Tasks 2 & 3 are `checkpoint:human-verify gate=blocking` and BLOCKING — Task 2 needs live Milvus+BGE-M3+GLM stack to run `cmd/eval --frozen=compliance` and paste `scripts/eval_snapshot` stdout into README Eval Results [still PLACEHOLDER]; Task 3 needs screen capture to record `docs/demo.gif` [cited verdict + structured refusal, <5MB]. Plan 07-02 Task 3 docker-build human-verify gate ALSO still BLOCKING — Docker not in PATH. Phase 7 has 2 concurrent blocking checkpoints: 07-02 Task 3 + 07-05 Tasks 2/3.)
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 08
**Current Plan:** 4
**Previous phase:** 7 (open-source-release) — 07-06 deferred to after v0.8 milestone by user decision 2026-08-17
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** `/gsd:plan-phase 8` (全量依赖升级 P8.0 — eino v0.7.7 → v0.9.13 + sqlite CheckPointStore; RF-2 compile-check spike on CheckPointStore interface shape). Phase 9 (历史注入 spike 硬门) is the HARD GATE for Phase 10 memory.

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Go-native trust evaluator (internal/eval; faithfulness ports Ragas v0.1.21 prompt, Apache-2.0) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4. D6 reversed 2026-08-12: Python/Ragas sidecar eliminated in favor of single-language Go stack. **v0.8 Phase 8 upgrades eino v0.7.7 → v0.9.13** (summarization middleware + mature ADK HITL; NOT v0.10.0-alpha.x per maintainer warning).

**License:** Apache-2.0.

**Current focus:** Phase 08 — p8-0-eino-v0-7-7-v0-9-13
**See:** `.planning/PROJECT.md` (updated 2026-08-17 — v0.8 milestone section)

---

## Current Position

Phase: 08 (p8-0-eino-v0-7-7-v0-9-13) — EXECUTING
Plan: 4 of 5
Status: Ready to execute
Last activity: 2026-08-17 -- Plan 08-03 complete: SC-3 legacy interrupt 禁令门（test-as-gate `fea6114`——TestNoLegacyInterruptAPI 全仓 WalkDir 扫描三标识符，负向证明探针注入 FAIL→删除 PASS）+ 版本叙事对齐（`996c33a`——REQUIREMENTS/ROADMAP/CLAUDE.md 定点改 v0.9.14 + schema.RegisterName 修正，CLAUDE.md 历史出处/STATE.md 叙事句未动）

**Milestone v0.8 phase map (from ROADMAP.md):**

| Phase | Slug | Requirements | Notes |
|-------|------|--------------|-------|
| 8 | 全量依赖升级 (P8.0) | PLAT-04, PLAT-05 | eino → v0.9.13; sqlite CheckPointStore; legacy interrupt lint ban |
| 9 | 历史注入 spike 硬门 (P8.S) | GEVL-08 | SPIKE / HARD GATE — memory must not ship unless gate passes (<5% drift, <10% refusal delta) |
| 10 | 多轮对话记忆 (P8.1) | MEM-01..05 | gated by Phase 9; UI hint yes |
| 11 | 写工具 + 审批门 (P8.2) | HITL-01..08 | FE+BE 同 phase 不可拆; UI hint yes |
| 12 | 规划层 (P8.3) | PLAN-01..05 | DeepAgent + 预算门 + 可回放; UI hint yes |
| 13 | 子 agent 扇出 (P8.4) | FAN-01..04 | RF-4 highest-uncertainty spike opens phase |

**贯穿性不变量:** citation 白名单 = 本轮检索结果，代码强制 — 历史/摘要/子 agent 输出/resume 注入文本永远不能成为证据。

## Decisions (from PROJECT.md + SUMMARY.md)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Go + Eino (not Python / full TS) | User fluency in Go; denova fork as mature reference; surfaces agent engineering depth over web iteration speed |
| D2 | Skill-driven, not fixed-function agent | Promotes app → platform; updates adapt enterprise needs with no code change |
| D3 | Built-in demo Skill = 企业合规 / 制度审查 | Sharp pain (no tolerable hallucination); natural fit for true-source separation; refusal mechanism shines |
| D4 | Audience = OSS personal brand | Polish docs + easy start + measurable outcomes (not just interview-project) |
| D5 | Skill schema = 6 fields with explicit `reads` / `writes` | Makes denova's implicit boundaries auditable + wizard-generatable |
| D6 | Go-native trust evaluator (`internal/eval`) | **REVERSED 2026-08-12** (was: "Ragas as Python sidecar for eval"). Single-language stack preserved; faithfulness ports the Ragas v0.1.21 claim-extraction + entailment prompts VERBATIM (Apache-2.0, attributed in `eval/NOTICE`) so the headline number is industry-comparable ("Ragas 口径"); the other 3 metrics (answer_relevancy / context_precision / context_recall) use simple Go-authored yes/no judge prompts. Reversal rationale: 03-RESEARCH.md own build-vs-buy verdict estimated ~2000 LOC to match faithfulness alone for a FULL reimplementation, but that cost concentrates in the prompts — only faithfulness's claim extraction is genuinely calibration-sensitive (handles negation / modality / temporal claims / multi-sentence answers). Decision: port ONLY the Ragas faithfulness prompt; author Go judge prompts for the other 3; keep the stack single-language. Eliminates T-3-SC supply-chain checkpoint (no PyPI audit), drops ragas/langchain-openai dependency tree, avoids ragas 0.1.21 ↔ Python 3.13 install friction. |
| D7 | Milvus tier as mandatory metadata field | No collection-per-tier (couples ops); tier-filter expr before similarity is the hard barrier |
| D8 | Citation = chunk ID + span | Free-text citations are the #1 hallucination vector (P-COMPL-1) |
| D9 | Refusal-by-construction on missing tier-match | Default-deny; never improvise (GEVL-02, KNOW-04) |
| D10 | Self-RAG / CRAG self-verification | Re-read & verify claims before final answer (ARAG-03) |
| D11 | MongoDB 4.4+ as alternate retriever backend (`RETRIEVER_BACKEND=mongo`) | Internal Milvus is broken (image configured for external etcd). Tier filter via `{tier:{$in}}` query BEFORE in-Go cosine; D-09 holds structurally on both backends. Proves the `retriever.Retriever` abstraction is real. |
| D12 | D-04 transport-swap invariant (Phase 4) | Mock ↔ Default transport must produce byte-identical UIMessageChunk sequences from the same fixture; `transport-swap.test.tsx` gates the production swap to the real Hertz endpoint, preventing UI drift. |
| D13 | T-1-08 source-URL allowlist sanitization gate (Phase 4) | `source-url` href is the primary XSS vector; protocol+hostname allowlist at the SourceUrlPart boundary, rejected URLs render a blocked state, `<a>` always carries `rel=noopener noreferrer`. 7-assertion regression gate. |
| D14 | State boundary: Zustand store bans `messages` (Pitfall 7, Phase 4) | `useChat` owns the message stream; `app-store.ts` file-level comment forbids a `messages` field, eliminating the dual-source-of-truth class of bugs. |
| D15 | Wire-type compile-time drift alarm (Phase 4) | TS `AssertAssignable<SkillForgeChunk, UIMessageChunk>` fails the build if the SDK upstream renames any chunk discriminator — the compiler IS the drift alarm (cheaper than runtime contract tests alone). |
| D16 | `data-decision` SSE chunk discriminator (Phase 3 / Plan 03-01) | Registers the ARAG-01 / SC-1 observability substrate — a per-hop wire-level slot for agent autonomous decisions (retrieve/answer/refuse/forced_stop). Mirrored Go + TS in one commit per the D15 two-PR rule; emitted by Plan 03-03's Eino callbacks (`newDecisionCallback`), parsed by the Phase 4 React UI's `SkillForgeChunk` union. |
| D17 | TierEnum reverse-lookup helpers (Phase 3 / Plan 03-01) | `skill.TierEnumByValue` + `skill.TierEnumValues` expose the 4-tier enum for backend-agnostic validation. KNOW-02 enforcement at `internal/knowledge.Index` (Plan 03-04) calls `TierEnumByValue(doc.Tier)` — fail-closed default-deny on any tier outside {权威, 历史, 个人, 草稿}. |
| D18 | Store interface-first + SQLite default (Phase 3 / Plan 03-02) | `internal/store.Store` + `internal/store/sqlite.SQLiteStore` mirror the D11 retriever pattern (interface in parent pkg, impl in subpkg, compile-time `var _ store.Store = (*SQLiteStore)(nil)`). modernc.org/sqlite@v1.56.0 = pure-Go (no cgo) zero-ops default; PG-ready drop-in without caller changes. audit table is INSERT+SELECT only (T-3-AUDIT); `QueryAudit` is 100% `?`-parameterized (T-3-SQLI); `GetSession WHERE session_id=?` is the SC-9 isolation boundary; `AppendTurn` idempotent via `UNIQUE(session_id,role,ts)+ON CONFLICT DO NOTHING`. Plan 03-05 (server wiring) + Plan 03-06 (eval sidecar AuditRow reads) unblocked. |
| D19 | Go-native trust evaluator (Phase 3 / Plan 03-06) | `internal/eval` library + `cmd/eval` CLI ship 4 RAG-trust metrics (faithfulness / answer_relevancy / context_precision / context_recall). Faithfulness ports Ragas v0.1.21 claim-extraction + entailment prompts VERBATIM into Go string constants (Apache-2.0, attributed in `eval/NOTICE` — the headline number stays industry-comparable, "Ragas 口径"); the other 3 metrics use simple Go-authored yes/no judge prompts. 50-case synthetic golden set (20 adversarial + 20 refusal-required + 10 multi-hop, 5 spike-anchors from cmd/spiked/e2e.go:76-82). D2-15 calibration vehicle: `Calibrate()` emits `recommended_threshold` to `eval/calibration.json` — manual ops swap of the literal 0.3 in `internal/server/pipeline.go:43` (trust-boundary human sign-off). Additive accessors: `agent.LastRetrieved()` + `store.UpdateAuditMetrics()` (COALESCE partial backfill). Pitfall 8 counter-measures: deterministic Go grounding floor (D3-01), human spot-check, `JUDGE_MODEL` config slot (D3-07) for Phase 6 cross-eval. |
| D8-01 | 升级目标 = eino v0.9.14 + eino-ext openai v0.1.13（Phase 8 / Plan 08-01） | 用户 2026-08-17 决策：v0.9.14 覆盖研究期 v0.9.13（v0.9.13→v0.9.14 仅 2 项变更，#1157 恰好修复 Phase 10 要用的 summarization middleware ctx bug）；v0.10.0-alpha.x maintainer 警告 breaking，明确不用。TestContractVersionPin 锁新 pin；hertz v0.10.6 / milvus v2.4.2 / sqlite v1.56.0 不动。8 处 godoc file:line 已按 v0.9.14 module cache 重核（react.go:139、tool_node.go:186、template.go:525-528/596-600、invokable_func.go:139-143、WithChatModelOptions 迁至 adk/chatmodel.go:97 等）。 |
| D8-02 | 升级 = 行为同构（hermetic 面实证，Phase 8 / Plan 08-01） | eino v0.9.14 下 hermetic 四道门（go build+vet / go test ./... 全量 / contract -tags=contract 全绿含 VersionPin 新断言与 RunsUnder10s 81.8ms / TestCancelPropagationMidTool 50.6ms <200ms）零修复提交通过——RF-2「编译同构」扩展为「行为同构」（hermetic 面）。live 面（真 GLM cancel / Milvus tier-filter / 冻结 eval 重跑）归 08-05 checkpoint:human-verify，尚未重锁。 |
| D8-03 | sqlite CheckPointStore 骨架形态（Phase 8 / Plan 08-02） | Get/Set 挂 SQLiteStore（非独立类型，PATTERNS 形态 A），`var _ compose.CheckPointStore = (*SQLiteStore)(nil)` 一个断言同时锁 compose 与 adk 两个 alias 消费面；checkpoint 表 DDL 本 phase 即带 session_id/skill_id/expires_at 预留列（Phase 11 生命周期免 ALTER），本 phase 不写不滤 expires_at；不实现 Delete（CheckPointDeleter 留 Phase 11）；store 层 payload 纯透传不解析（resume-data 校验归 Phase 11 HITL-04）。六面测试（round-trip 含非 UTF8 / Close+reopen 等值读回 / upsert / unknown-id 三态 / DROP TABLE 存活 / closed 守卫）锁定 SC-4 store 层语义。 |
| D8-04 | SC-3 禁令门形态（Phase 8 / Plan 08-03） | 独立 TestNoLegacyInterruptAPI（不并入 RunsUnder10s 聚合，contract_test.go 零改动）；WalkDir 从 findModuleRoot 扫全仓 .go（含 _test.go），排除 .claude/web/.git/node_modules/.planning/screenshots + 门自身文件；禁令清单三标识符（NewInterruptAndRerunErr / compose.InterruptAndRerun / deprecatedInterruptAndRerun）；WrapInterruptAndRerunIfNeeded 不禁（官方迁移辅助）；负向证明（探针注入 FAIL 点名 file:line→删除恢复 PASS）入 08-03-SUMMARY。 |
| D8-05 | 版本叙事对齐口径（Phase 8 / Plan 08-03） | REQUIREMENTS/ROADMAP/CLAUDE.md 定点改 v0.9.14 + 双处用户决策注记（2026-08-17 覆盖研究期 v0.9.13 / 而非研究原锁）；PLAT-05/SC-4 注册 API 修正为 schema.RegisterName[T](name)（RESEARCH Pitfall 1）；CLAUDE.md:203/:227 历史研究出处、STATE.md、phase 目录名一律不动（身份锚点）。 |

---

## Accumulated Context

### Roadmap Evolution

- 2026-08-17: Milestone v0.8 roadmap appended — phases 8-13 (P8.0 dep-upgrade → P8.S spike hard gate → P8.1 memory → P8.2 write-tools+approval → P8.3 planning → P8.4 fan-out), strict dependency chain per research SUMMARY.md; 25/25 v0.8 requirements mapped; v0.7.7 phases 1-7 + backlog 999.1/999.2 untouched.
- Phase 6 edited: edited fields: title,goal,sc-2,plan-list (split into code-side 6a complete + 06-03 deferred to new Phase 999.2 backlog; also repaired pre-existing corruption: Phase 4 section carried a stray copy of the 06 plan list — restored real 04-01..04-04 list)

**Open questions (from research SUMMARY.md):**

1. Whether `name` is part of the 6-field schema or an external manifest identity
2. Public / legally-safe CN compliance dataset acquisition strategy
3. Exact citation-entailment verification method
4. Retrieval / refusal / confidence / HITL threshold values (empirical calibration required)
5. Metadata filter vs separate Milvus collections for tier isolation trade-off
6. Policy version conflict semantics (when 权威 and 历史 disagree)
7. Audit retention + redaction + access control requirements
8. Eval set composition: synthetic vs real-user sampling for compliance domain

**Research flags (phase-specific research needed):**

- Phase 8: RF-2 — exact CheckPointStore interface shape post-upgrade (cheap 30-min compile-check spike)
- Phase 10: RF-6 — summarization middleware token-counter calibration vs GLM usage logs; tokenizer configurability unverified
- Phase 11: RF-3 — ai@7 custom transport resume shape (dedicated endpoint + transport branch vs synthetic user message)
- Phase 12: RF-5 — DeepAgent final-message verdict-parse rate on GLM-5.2 (fallback: structured-output final step)
- Phase 13: RF-4 (highest uncertainty) — ADK ctx-value propagation into sub-agent tool closures; fallback = per-Run agent construction with accumulator baked into closures

**v0.7.7 legacy flags (closed milestones, kept for reference):**

- Phase 1: Eino APIs in depth, Hertz-to-AI-SDK v5 transport, Milvus filter expression syntax
- Phase 3: Chinese eval dataset sourcing, BGE-M3 reranker choice (self-host via TEI vs API), Ragas judge calibration
- Phase 5: hot reload for in-flight conversations, cross-tier conflict policy, historical tier storage, multi-agent value proof
- Phase 6: audit privacy (retention / redaction / access control), reproducibility, CI evaluation cost

---

## Blockers

(none for v0.8 planning; Phase 7 carries 2 human-verify checkpoints — 07-02 T3 docker-build, 07-05 T2/T3 live stack + GIF — deferred to after the v0.8 milestone by user decision 2026-08-17)

### Quick Tasks Completed

| # | Description | Date | Commit | Status | Directory |
|---|-------------|------|--------|--------|-----------|
| 260811-uaf | Fix CenterPanel tab indicator sync with centerView.kind (04-UAT #8) | 2026-08-11 | e757d6e |  | [260811-uaf-fix-centerpanel-tab-indicator-sync-with-](./quick/260811-uaf-fix-centerpanel-tab-indicator-sync-with-/) |
| 260812-cng | Finalize v0.7.7 dep-upgrade tail (drop scripts/test, reconcile STATE.md drift, track milestone audit) | 2026-08-12 | 6bbfa61 | [260812-cng-go-scripts-test-state-md-frontmatter-hea](./quick/260812-cng-go-scripts-test-state-md-frontmatter-hea/) |
| 260813-1i6 | Consolidate eval/judge.go JSON parse onto shared llm.ExtractJSON (OOS-1 follow-up) | 2026-08-12 | d1af0f6 | [260813-1i6-eval-judge-go-llm-extractjson-oos-1](./quick/260813-1i6-eval-judge-go-llm-extractjson-oos-1/) |
| 260813-1y0 | UAT #7 hot-reload toast bridge (polling-B): JS-safe registry version counter + failed map (BE) → GET /api/skills wrapper + useSkillReload polling-only + drop dead SSE (FE) | 2026-08-13 | 3d77870 + 3265178 + 09ae711 | [260813-1y0-hot-reload-toast-bridge-polling-b](./quick/260813-1y0-hot-reload-toast-bridge-polling-b/) |
| 260813-dno | UAT #4 graph chunk full-text: GET /api/knowledge/chunk/:id (retriever.GetChunk → knowledge.GetChunk → chunkHandler) + chunk-lookup async fetch on fixture-miss (FE) | 2026-08-13 | 8939a74 + aec582e + c646382 | [260813-dno-uat-4-graph-chunk-full-text-get-api-know](./quick/260813-dno-uat-4-graph-chunk-full-text-get-api-know/) |
| 260814-wre | 后端代码整理：.gitattributes + EOL renormalize + gofmt 全树、cmd/spiked → scripts/spiked、根目录调试产物清理 + gitignore 补漏、go mod tidy | 2026-08-14 | 4f4e631 + 1f1edd6 + c4b34a8 + b1ded0f | [260814-wre-gitattributes-cmd-spiked-go-mod-tidy](./quick/260814-wre-gitattributes-cmd-spiked-go-mod-tidy/) |
| 260814-ofn | 修正过时注释：indexer.go 3处 + grounding.go/grounding_test.go 2处 "markdown splitter deferred to Phase 3 (CMPL-06)" → 真实状态（分块器从未被任何 phase 认领；CMPL-06 = Phase 5 反证高亮，05-06 已完成） | 2026-08-14 | 70cd1e5 + 01fd467 | [260814-ofn-fix-stale-splitter-comments](./quick/260814-ofn-fix-stale-splitter-comments/) |
| 260814-tsr | 文档叙事层去合规化：README/CLAUDE.md/CONTRIBUTING.md 不再以 compliance 为主线，Quick Start 改用 simple-qa 通用文档问答演示，突出平台全面性/通用性（代码与 examples/ 不动） | 2026-08-14 | f30d7d1 + 2c3f7f8 | [260814-tsr-remove-compliance-wording](./quick/260814-tsr-remove-compliance-wording/) |
| 260814-vga | 代码层去合规化：web/ 前端 UI 文案泛化为通用文档问答（标题/tab/空状态/建议问题/占位符）+ skills/ 默认 Skill 从 compliance 换为 simple-qa（examples 模板保留，eval-gate.json 随迁 examples）| 2026-08-14 | e42220e + 4bda3ce + 80b6c2d |  | [260814-vga-ui-skill-simple-qa](./quick/260814-vga-ui-skill-simple-qa/) |
| 260814-w4n | 修复 Skill 热重载目录级缺口：fsnotify 监听 skills/ 目录 create/remove，新 Skill 目录自动发现 + 已删 Skill 自动移除（含 O(n) 存在性扫描兜底交织突发），启动时补 Add 直属子目录修复 Linux 非递归缺口 | 2026-08-14 | 3a46a24 + c3ccf55 | Verified* | [260814-w4n-skill-fsnotify-skills-create-remove-skil](./quick/260814-w4n-skill-fsnotify-skills-create-remove-skil/) |
| 260815-0xb | ChatInput 下拉框改 Claude Code 式斜杠命令菜单（"/" 触发 + 过滤 + 键盘导航）+ VerdictPipeline 逐阶段 data-subprogress 流式输出（wire 白名单 19→20，打通既有 FE DataSubprogressPart 通道） | 2026-08-14 | 6939050 + 6366eff + d55c852 + 498d051 |  | [260815-0xb-skill-skill-chat](./quick/260815-0xb-skill-skill-chat/) |
| 260815-cos | C 方案 chat 流式（LLM ChatModel.Stream 草稿 + text-delta-draft 20→22 + DraftStreamChip "🟡 待校验 → ✅ 已验证" + grounding 失败 RefusalCard 替换）+ CitationCluster 角标 [N] + hover 弹层（150ms debounce + 复用 fetchChunkContent 拉全文）+ VerdictCard 置信度 UI 隐藏（保留 hidden testid）+ CounterEvidenceCard 仅冲突渲染 | 2026-08-15 | c348294 + 46b9fad + f02476d | Verified | [260815-cos-chat-c-llm-streaming-grounding-hover-ui](./quick/260815-cos-chat-c-llm-streaming-grounding-hover-ui/) |
| 260815-gcw | Chat 全链路诊断日志补全：chat.go 7 个埋点（C-1 godoc DEBUG 注释 + C-2..C-7 chatHandler 入口/Agent/Registry/Loader/VerdictPipeline/runAgentPath 路由点）+ pipeline.go 7 个埋点（D-1..D-7 VerdictPipeline/streamAndAssemble/Stream/Recv/draft-end），chat.go +38 / pipeline.go +52 行，零控制流/签名/SSE 输出字节变更。等待用户重启后端 + curl 复现 | 2026-08-15 | aec5392 |  | [260815-gcw-chat-streamandassemble-chathandler-strea](./quick/260815-gcw-chat-streamandassemble-chathandler-strea/) |
| 260815-grw | 修 streamAndAssemble 单条 System message 被 GLM Coding Plan 400 拒绝：拆 systemPromptTemplate 为 systemInstructionsTemplate + userQueryTemplate，buildPrompt 返回 (system, user) 双字符串，streamAndAssemble 传 {System, User} 双 message 给 ChatModel.Stream。chat.go / Runner.Query fallback 不动（adk 内部已用双 message）。RED test 0dee051 → GREEN impl 7f9e641。等用户重启 + curl 验证 SSE 流 | 2026-08-15 | 0dee051 + 7f9e641 |  | [260815-grw-streamandassemble-llm-system-message-glm](./quick/260815-grw-streamandassemble-llm-system-message-glm/) |
| 260815-h0j | BGE-M3 embedder TLS unknown certificate 修复：bge.go 加 Config.InsecureSkipVerify bool + BGE.client *http.Client + 自定义 Transport.TLSClientConfig；main.go 读 BGE_M3_TLS_SKIP_VERIFY env（默认 false），开启时打印 [WARN]。RED d564e8e → GREEN 47b45d8 → wiring 896768b。支持内网自签名 HTTPS 部署 | 2026-08-15 | d564e8e + 47b45d8 + 896768b | Verified | [260815-h0j-bge-m3-embedder-tls-unknown-certificate-](./quick/260815-h0j-bge-m3-embedder-tls-unknown-certificate-/) |
| 260815-j4y | 真后端 transport 拦截 draft-* chunks 触发 AI SDK parser abort：写 RealChatTransport wrap DefaultChatTransport，在 SSE→UIMessageChunk 层过滤 text-delta-draft/text-delta-draft-end 走 onDraftUpdate，其他 chunks 原样透传；createTransport('prod') 改为返回 RealChatTransport（带 onDraftUpdate 选项）。RED af9bb59 → GREEN be7bd62；vitest 244/244 PASS（含 4 个新 RealChatTransport 测试 + transport-swap 不变绿）。 | 2026-08-15 | af9bb59 + be7bd62 |  | [260815-j4y-transport-text-delta-draft-ai-sdk-parser](./quick/260815-j4y-transport-text-delta-draft-ai-sdk-parser/) |
| 260815-jpu | chunk_id 三层防御修复 LLM 把 [CHUNK 7] 字面抄到 verdict JSON 触发后端 ParseInt 400：Layer 1+2 (BE pipeline.go buildPrompt [CHUNK N]→[ref-N] + systemInstructionsTemplate schema 加 digits-only 提示)、Layer 3 (BE knowledge.go parseLenientChunkID 宽容解析 + WARN 日志)、Layer 4 (FE chunk-lookup.ts normalizeChunkID 入口归一化)。8eab17c + e41a810 + 977333f；go test + vitest 262/262 PASS | 2026-08-15 | 8eab17c + e41a810 + 977333f |  | [260815-jpu-llm-chunk-id-chunk-7-parseint-400-1-buil](./quick/260815-jpu-llm-chunk-id-chunk-7-parseint-400-1-buil/) |
| 260815-k8d | 修 DraftStreamChip "✅ 已验证" 永久挂在最底部：draftStatus=verified 后没有 reset 路径回 idle。RealChatTransport + MockChatTransport 在 text-end（最终结论文本完成）分支 emitDraft('idle','')，RightChat 通过 draftStatus !== 'idle' 卸载 chip。状态机闭环 pending→verified→idle。46d2593 + df9e1b9；vitest 263/263 PASS | 2026-08-15 | 46d2593 + df9e1b9 |  | [260815-k8d-draftstreamchip-draftstatus-verified-idl](./quick/260815-k8d-draftstreamchip-draftstatus-verified-idl/) |
| 260817-hck | VerdictCard 答案去重：verdict.answer 与聊天流结论正文重复时隐藏卡片内 verdict-answer 区块（chatText 可选 prop，verbatim-substring 判定），卡片只保留状态徽章 + 引用计数 + CitationCluster；MessageBubble → DataPolicyPart → VerdictCard 逐级透传；CenterPanel 独立用法不受影响。vitest 272/272 PASS | 2026-08-17 | e41a2cd + ec0b1b2 + edfcd72 + 0da6e17 |  | [260817-hck-verdictcard-dedupe-answer-vs-chat-conclu](./quick/260817-hck-verdictcard-dedupe-answer-vs-chat-conclu/) |
| 260817-i36 | 聊天回答区四项 UI 修复（每项一原子提交）：①「已给出」StatusBadge 从 VerdictCard 移除，改为结论正文角标小徽章（MessageBubble 从 data-policy 提取 status → TextPart verdictStatus prop）；②引用去重——source-document chips 改 button 可点击跳转（setCenterView chunk-source），VerdictCard 在聊天上下文（chatText 存在）隐藏 CitationCluster/计数，DataPolicyPart 对 ok 事实型且非 review_required 时 return null，CenterPanel 完整卡片保留；③chip 区分——BE titleFor 产出「path · chunk N」+ FE 去冗余 doc: 前缀；④conclusionText 标点——剥 Answer 结尾句号 + 统一 。。→。。go test ok；vitest 280/280 PASS；tsc 维持既有 1 错无新增 | 2026-08-17 | 135b655 + a2732ce + 5c08229 + 561c162 + b7de1d3 + ed43c0e + b0a9e7c + 14158d8 + b1c70aa + 5b7a734 |  | [260817-i36-chat-answer-area-four-ui-fixes-verdict-b](./quick/260817-i36-chat-answer-area-four-ui-fixes-verdict-b/) |

*260814-w4n：代码层 5/5 must-haves 全部验证通过（5 个真实 fsnotify 测试 Windows 本机 PASS，无 skip）；唯一待办为完整 HTTP E2E——本机无 Milvus/Docker 无法起独立实例，待用户重启其 dev 服务器时由 chrome-devtools 复验关闭。

---

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| 04 | 1 | 75m | 4 tasks / 38 files — Wave 1 foundation (wire types + mock transport + parts) |
| 04 | 2 | 50m | 3 tasks — Wave 2: VerdictCard + RefusalCard + ReasoningPart + CenterPanel + locked-file reconciliation (24 new / 8 mod) |
| 04 | 3 | 50m | 3 tasks — Wave 3: ThreePanel + SkillPicker + ChunkSourceView wire + transport-swap integration test (17 new / 8 mod) |
| 02 | 3 | — | verdict pipeline + cmd/server |
| 02 | 2 | — | skill package + corpus |
| 02 | 1 | — | grounding + verdict schema |
| 02 | 04-MONGO | 13m | 5 commits — MongoDB retriever backend (RETRIEVER_BACKEND=mongo). See 02-04-MONGO-SUMMARY.md |
| 04 (quick) | 260811-uaf | ~20m | CenterPanel tab-indicator ↔ centerView.kind sync — closed UAT #8 (83/83 green). See quick/260811-uaf-*-SUMMARY.md |
| 03 | 01 | 7m | 2 TDD tasks / 4 commits / 5 files (2 created + 3 modified) — `data-decision` SSE chunk discriminator (Go `wire.ValidTypes` 16→17 + exported `DecisionPayload` + TS `SkillForgeChunk` variant) + `skill.TierEnumByValue`/`TierEnumValues` reverse-lookup helpers (additions only). ARAG-01/SC-1/KNOW-01 substrates unlocked. See 03-01-SUMMARY.md |
| 03 | 02 | 18m | 3 TDD tasks / 3 commits / 8 files (6 created + 2 modified) — `internal/store.Store` interface + `SQLiteStore` default impl (modernc.org/sqlite@v1.56.0 pure-Go, no cgo). D11 mirror of retriever + mongo. audit INSERT+SELECT only (T-3-AUDIT); `QueryAudit` 100% `?`-parameterized (T-3-SQLI regression: DROP TABLE survives in 3 filter fields); SC-9 isolation at `WHERE session_id=?`; `AppendTurn` idempotent via UNIQUE+ON CONFLICT. GEVL-03/SC-7 + AGNT-05/SC-9 unlocked at store layer. See 03-02-SUMMARY.md |
| 04 | 04 | 12m | 3 tasks / 3 commits / 6 modified — wire-contract TS realignment (gap closure, `requirements: []`). `cd web && npx tsc --noEmit` 21 → 0 errors; vitest 84/84; build 411.12 KB / 120.47 KB gzip (04-03 band). D15 alarm STRENGTHENED (`finishReason?: string` → `FinishReason`); Pitfall 1/3 guards preserved via string-cast; ToolPart toolName structural-type read; transport-swap `FixtureDrivenTransport` zero-arg contract. Closes Phase 4 TS-drift debt. See 04-04-SUMMARY.md |
| 03 | 06 | 19m | 3 tasks / 5 commits / 22 files (15 created + 7 modified) — Go-native trust evaluator (D6 reversed): `internal/eval` library + `cmd/eval` CLI. 4 RAG-trust metrics (faithfulness ports Ragas v0.1.21 prompts VERBATIM, Apache-2.0 attributed in `eval/NOTICE`; other 3 use Go-authored judge prompts). 50-case synthetic golden set (20/20/10, 5 spike-anchors, GEVL-05). D2-15 calibration vehicle emits `recommended_threshold` to `eval/calibration.json` (manual ops swap of literal 0.3). Additive accessors `agent.LastRetrieved()` + `store.UpdateAuditMetrics()` (COALESCE partial backfill). SC-6 reproducibility gate via StubJudge + deterministic embedder. Pitfall 8 counter-measures in place. See 03-06-SUMMARY.md |
| 05 | 07 | ~30m | 3 TDD tasks / 7 commits / 13 files (10 web/ + SUMMARY + UAT + REMAINING-WORK) — **left-sidebar IA gap-closure (UAT #2)**, the LAST open Phase-5 item. `gap_closure:true` + `closes_uat:#2`. Store contract: `SkillSummary` + `SessionMeta` + `sessions[]/activeSessionId/createSession/setActiveSession/deleteSession` (D14 — no `messages` field); `skills[]` + `setSkills`. Live data path: `useSkillReload.tick()` normalizes `GET /api/skills` reading **`item.manifest.description`** (BLOCKING CONSTRAINT, not `item.description`). `App.tsx` passes `id: activeSessionId` into `useChat` (Vercel AI SDK v5 in-memory per-id chatStore). `ChatInput` becomes a live Skill popover (reads `store.skills`); `SkillManagerMenu` collapses to wizard-only (`skill-manager-new-skill`); `RightChat` header becomes read-only badge (`right-chat-badge`). `LeftSidebar` rewrites to 4 stacked sections: `新建会话` + `会话` + `文件`(SOURCES preserved) + `技能` with status badges (已加载/加载失败(旧版可用)). Dead `/api/skills/reload` 404 gone (D5-IA-05); `__fixtures__/skills.json` retired from all 3 Skill surfaces. Merge `2873776`: +984/-228 across 14 files. `npx tsc --noEmit` 0; `vitest run` **209/209** (baseline 196 + 13 new/updated across 31 files); `vite build` 466.50 KB / 135.02 KB gzip. 14 grep gates pass. See 05-07-SUMMARY.md |
| 07 | 01 | 3m | 3 tasks / 3 commits / 6 files — **Phase 7 release-meta foundations**. LICENSE (verbatim Apache-2.0 SPDX, closes DIST-02) + `.dockerignore` (build-context boundary = backend source + skills/ + knowledge/ + web/ only; T-7-OSS-CONTAINER) + `.gitignore` skillforge.db* (T-7-OSS-SECRET) + `.env.example` internal-IP scrub (milvus.internal→milvus:19530, bge-m3.internal→localhost:11434, 10.62.10.34→localhost; T-7-OSS-ENV zero matches at HEAD) + two-layer fail-fast contract doc (envRequired + `${VAR:?}`) + `ollama pull bge-m3` one-liner (D7-02). ROADMAP SC-1/SC-5 + REQ DIST-04/DIST-05 revised per D7-03/D7-06 (pluggable embedding endpoint / recorded demo GIF). DIST-02/DIST-03 marked complete. History scrub (T-7-OSS-INFO) deferred to Plan 07-06. See 07-01-SUMMARY.md |
| 07 | 02 | 5m | 2/3 tasks / 2 commits / 3 files — **Phase 7 containerization artifacts** (PARTIAL — Task 3 human-verify gate BLOCKING). `docker-compose.yml` (5 services: etcd+minio+milvus+backend+frontend; Milvus v2.4.17 pinned to SDK v2.4.2 RocksMQ; `${BGE_M3_URL:?}` + `${GLM_API_KEY:?}` two-layer fail-fast; healthcheck-gated depends_on; named volumes *-data; no embedding container D7-02) + `Dockerfile.backend` (golang:1.25-alpine → alpine:3.20; CGO_ENABLED=0 static binary; ships skills/+knowledge/) + `Dockerfile.frontend` (node:22-alpine → nginx:1.27-alpine; `/api` proxy with `proxy_buffering off` for SSE + SPA fallback). All 7 base images pinned (zero :latest). Commits `798248d` + `3d9f590`. DIST-04 PENDING Task 3 (Docker not in PATH — human must install Docker Desktop + run compose config + both builds). See 07-02-SUMMARY.md |
| 07 | 03 | 3m | 2 tasks / 2 commits / 2 files — **Phase 7 docs (README + CONTRIBUTING)**. `README.md` (all 7 DIST-01 elements: badges via shields.io + value prop + demo GIF embed `docs/demo.gif` + Quick Start within 2 screens with ollama one-liner D7-02 + Mermaid architecture diagram GitHub-native with 3 moats labeled + Eval Results PLACEHOLDER for Plan 07-05 + contributing link + Apache-2.0 license section) + `CONTRIBUTING.md` (3 named sections DIST-07: Write a Skill / Add a Tool / Add a Knowledge Tier, each with fenced code examples referencing real identifiers internal/skill.Validate + validAgentTypes + TierEnum + ResolveTiers + retrieve_tool.go; fork-PR flow + gofmt + Apache-2.0 SPDX header precedent). Zero internal IPs/domains at HEAD (T-7-OSS-INFO). Commits `d35e7a6` + `4daade7`. DIST-01 + DIST-07 marked complete. Eval Results section is PLACEHOLDER — Plan 07-05 fills real numbers. See 07-03-SUMMARY.md |
| 07 | 04 | 4m | 2 TDD tasks / 3 commits / 12 files (11 created + 1 test) — **Phase 7 examples/ + eval sample (DIST-06 + DIST-08 + DIST-10)**. `examples/skills/compliance/SKILL.md` (verbatim copy of `skills/compliance/SKILL.md`, the canonical runnable 7-field Skill) + `examples/skills/simple-qa/SKILL.md` (minimal RAG template: ChatModelAgent, reads `[authority/**]`, writes `[]`, temp 0.3, 3 prohibitions) + `examples/skills/calculator/SKILL.md` (tool-type template: ReActAgent, reads `[draft/**]` PLACEHOLDER tier per D7-10 + RESEARCH Q2 so P-SKILL-3 default-deny invariant holds without changing the validator, writes `[skill_state]`, temp 0.0; body note flags validate-only status) + `examples/data/*.md` (7 verbatim copies of `knowledge/authority/*.md`: annual_leave / anti_corruption_clause / business_gift_prohibition / contract_approval / expense_reimbursement_scope / non_compete / travel_expense) + `internal/skill/examples_validate_test.go` (`TestExamplesValidate` drives `skill.NewLoader("../../examples/skills").Scan()` + asserts 3 manifests; CONTRIBUTING [Plan 07-03] points contributors at this test per CONTEXT Q4 — no `cmd/skillforge validate` subcommand) + `eval/sample.jsonl` (byte-identical copy of `eval/golden/compliance.jsonl`, 50 cases = 20 adversarial + 20 refusal-required + 10 multi-hop, D7-12). `internal/skill.Validate` UNCHANGED. Commits `9f2b734` (RED test) + `1eddc53` (GREEN examples/) + `51b042f` (sample.jsonl). DIST-06 + DIST-08 + DIST-10 marked complete. See 07-04-SUMMARY.md |
| 07 | 05 | 2m | **1/3 tasks** (autonomous:false — checkpointed) / 2 commits / 2 files — **Phase 7 eval snapshot script (DIST-09 + DIST-05 — DIST-09 PENDING live-stack Task 2; DIST-05 PENDING screen-capture Task 3)**. `scripts/eval_snapshot/main.go` + `main_test.go`: standalone Go program reads `eval/report.json` + `eval/frozen/compliance.lock.json` via `eval.LoadReport`/`eval.LoadLoad` (read-only — `cmd/eval` + `internal/eval` UNCHANGED), **derives** `refusal_accuracy` from `Report.Cases` via pure fn `deriveRefusalAccuracy` (count(cat=refusal-required && Refused) / count(cat=refusal-required) — RESEARCH Q1 load-bearing, NOT an evaluator field), renders README markdown snapshot (date + lock metadata + Overall metrics + derived % + case breakdown + reproduce cmd). 6/6 hermetic unit tests PASS (all-refused 1.0 / half-refused 0.5 / zero-refusal-required graceful 0,N/A / category-filter invariant / renderSnapshot metadata / N/A cell); `go vet` clean; stdlib-only imports. Commits `abf2f4d` (RED test) + `4873213` (GREEN impl). **Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING** — Task 2: live stack (`docker compose up` + `ollama pull bge-m3` + `.env` keys) → `go run ./cmd/eval --frozen=compliance` → paste `scripts/eval_snapshot` stdout into README `## Eval Results` (still PLACEHOLDER); Task 3: record `docs/demo.gif` (cited verdict + structured refusal, <5MB). README PLACEHOLDER + docs/demo.gif are intentional stubs pending checkpoints. See 07-05-SUMMARY.md |
| 08 | 01 | 6m | 2 tasks / 1 commit / 8 files — **eino v0.7.7→v0.9.14 + eino-ext openai v0.1.5→v0.1.13 升级原子提交**（`36845c0`，零功能代码：go.mod/go.sum 双 bump + TestContractVersionPin 改写 + 8 处 godoc file:line 按 v0.9.14 module cache 重核）。indirect 联动仅 acl/openai v0.1.17 + go-openai v0.1.2（tidy 自动）；hertz/milvus/sqlite 逐字未动。hermetic 四道门全绿零修复提交：build+vet / `go test ./...` 全量 / contract `-tags=contract`（VersionPin 新断言 PASS + RunsUnder10s 81.8ms）/ TestCancelPropagationMidTool 50.6ms（<200ms 未放松）。提交卫生：恰好 8 规定路径，scripts/test 等 untracked 未入提交。Task 2 验证性任务零提交（无回归→无归因 fix）。See 08-01-SUMMARY.md |
| 08 | 02 | 6m | 2 TDD tasks / 2 commits / 3 files (2 created + 1 modified) — **sqlite CheckPointStore 骨架**（RED `9a557b1` 六面测试先行 → GREEN `e6a580a` schema DDL 追加 + checkpoint.go）。checkpoint 表随既有 migrate() 幂等建出（同库 additive，audit/session_turn 零触碰；session_id/skill_id/expires_at Phase 11 预留列）；SQLiteStore.Get/Set 满足 compose.CheckPointStore（编译期断言，== adk alias）；六面全绿：round-trip（含非 UTF8 BLOB 透传）/ Close+reopen 等值读回（SC-4「进程重启」store 层证明，Pitfall 4 反面）/ upsert 覆盖 / unknown-id (nil,false,nil) 三态 / DROP TABLE 注入存活（T-8-04）/ closed 守卫 sql.ErrConnDone。`go test ./internal/store/...` 全绿零回归 + `go build ./... && go vet ./...` exit 0。一处 Rule 1 注释准确性修正（audit 注释「anywhere in this package」→「for audit rows」，DDL 未动）。See 08-02-SUMMARY.md |

- Phases completed: 3 (Phase 1 spike; Phase 2 compliance vertical slice; Phase 4 streaming trust UI) — Phase 3 deferred under MVP reordering, now in progress
- Requirements covered: 24 / 55 (Phase 2: 14; Phase 4: +FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08; Phase 3 03-01: +ARAG-01 substrate, +KNOW-01 substrate)
- Frontend tests: 84/84 green (14 files); UAT 12/12; `cd web && npx tsc --noEmit` exit 0 (debt-free since 04-04)
- Plans run: 15 (4 Phase 1 + 4 Phase 2 incl. MONGO + 4 Phase 4 incl. 04-04 gap-closure + 3 Phase 3 incl. 03-06 Go-native evaluator)
- Last plan: 03-06 (Phase 3 Go-native trust evaluator) — commits 52dab7e + d4f96fc + a77be40 + f7979af + 247dfd8. **Phase 3 COMPLETE (6/6 plans)**

---
| Phase 08 P03 | 6m | 2 tasks | 4 files |

## Session Continuity

**Resume point (v0.8):** Milestone v0.8 roadmap created 2026-08-17 — ROADMAP.md carries phases 8-13 (strict chain 8→9→10→11→12→13 per research SUMMARY.md §Implications for Roadmap); REQUIREMENTS.md traceability maps all 25 v0.8 requirements (PLAT-04/05, GEVL-08, MEM-01..05, HITL-01..08, PLAN-01..05, FAN-01..04); Phase 9 is the HARD GATE for Phase 10. **Plan 08-01 COMPLETE 2026-08-17 (commit `36845c0`)**: eino v0.9.14 + ext openai v0.1.13 落库，hermetic 四道门全绿零修复提交（TestContractVersionPin 已断言新 pin）。**Plan 08-02 COMPLETE 2026-08-17 (commits `9a557b1` RED + `e6a580a` GREEN)**: sqlite CheckPointStore 骨架就位——SQLiteStore.Get/Set 满足 compose.CheckPointStore（编译期断言），checkpoint 表同库 additive 幂等（Phase 11 预留列齐备），SC-4 store 层语义（含 Close+reopen 等值读回）六面测试锁定，internal/store 全树零回归。**Plan 08-03 COMPLETE 2026-08-17 (commits `fea6114` 禁令门 + `996c33a` 文档对齐)**: SC-3 legacy interrupt 禁令门 TestNoLegacyInterruptAPI 落地（全仓扫描 + 负向证明），REQUIREMENTS/ROADMAP/CLAUDE.md 版本叙事对齐 v0.9.14（PLAT-05/SC-4 → schema.RegisterName）。Next: Plan 08-04 (Wave 3: SC-5 真实中断往返——消费 08-02 的 store，经 adk.Runner CheckPointStore 字段 + schema.RegisterName 注册) 然后 08-05 (Wave 4: live 重锁 human-verify 三门)。Phase 7 leftovers (07-02 T3, 07-05 T2/T3, 07-06) are deferred to after this milestone by user decision 2026-08-17 — do not lose them.

**Resume point (Phase 7):** Plan 07-05 (eval snapshot + demo GIF) is IN PROGRESS — Task 1/3 DONE via TDD, 2 commits (`abf2f4d` RED test + `4873213` GREEN impl). `scripts/eval_snapshot/{main.go,main_test.go}` shipped: standalone Go program reads `eval/report.json` + `eval/frozen/compliance.lock.json` via `eval.LoadReport`/`eval.LoadLock`, **derives** `refusal_accuracy` from `Report.Cases` via the pure fn `deriveRefusalAccuracy` (count(cat=="refusal-required" && Refused) / count(cat=="refusal-required") — RESEARCH Q1 load-bearing, NOT an evaluator field), renders the README markdown snapshot block (date + lock metadata + Overall metrics + derived % + case breakdown + reproduce cmd). 6/6 hermetic unit tests PASS (all-refused 1.0 / half-refused 0.5 / zero-refusal-required graceful 0,N/A / category-filter invariant / renderSnapshot metadata / N/A cell); `go vet` clean; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. **Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING** — Task 2 needs live Milvus+BGE-M3+GLM stack (`docker compose up` + `ollama pull bge-m3` + populate `GLM_API_KEY`+`BGE_M3_URL` in `.env`) to run `go run ./cmd/eval --frozen=compliance` → `eval/report.json` → `go run ./scripts/eval_snapshot` → paste stdout into README `## Eval Results` (still PLACEHOLDER); Task 3 needs screen capture (ScreenToGif/OBS) to record `docs/demo.gif` (cited verdict + structured refusal flows, <5MB, embed already in README hero). Plan 07-02 (containerization) Task 3 docker-build ALSO still IN PROGRESS — Docker not in PATH. Plans 07-01 (release-meta) + 07-03 (docs) + 07-04 (examples+eval sample) COMPLETE. Next: human resolves 07-05 Task 2/3 checkpoints (which unblocks 07-06 publication governance — must NOT push to public remote until README eval numbers + GIF are in tree). See `.planning/phases/07-open-source-release/07-05-SUMMARY.md`.

**Resume point (legacy):** Plan 03-06 (Go-native trust evaluator) is COMPLETE — 3/3 tasks, 5 commits (`52dab7e` D6 reversal docs + `d4f96fc` Task 1 + `a77be40` Task 2 + `f7979af` Task 3 A+B + `247dfd8` Task 3 C-H). D6 reversed 2026-08-12: evaluator is now Go-native (internal/eval + cmd/eval); faithfulness ports Ragas v0.1.21 prompts VERBATIM (Apache-2.0, eval/NOTICE); other 3 metrics use Go-authored judge prompts. 50-case synthetic golden set shipped (GEVL-05: 20/20/10 + 5 spike-anchors). D2-15 calibration vehicle emits `recommended_threshold` to `eval/calibration.json` — manual ops swap of the literal 0.3 in `internal/server/pipeline.go:43`. **Phase 3 COMPLETE (6/6 plans)** — GEVL-04 + GEVL-05 marked complete. Next: Plan 05-01 (Phase 5 skill-platform) or Phase 6 CI gate wiring (eval/report.json → GEVL-07 promotion gate). **Phase 4 TS-drift debt CLOSED 2026-08-12 via Plan 04-04** — `cd web && npx tsc --noEmit` exits 0 (21 → 0 errors), vitest 84/84, build 411.12 KB / 120.47 KB gzip. See `.planning/phases/04-streaming-trust-ui/04-04-SUMMARY.md`.

**Debug fixes — 2026-08-13 (Phase 5 UAT blockers, /gsd-debug):**

- **#9 (blocker) wizard commit registration** — FIXED, commit `af1ca9a`. `wizardCommitHandler` rescans `Loader` + `Registered` flag; FE gates toast on `registered`; `Loader.cached` race-guarded. Resolved session: `.planning/debug/resolved/wizard-commit-registration.md`.
- **OOS-1 (blocker) GLM verdict fence-parse** — FIXED, commit `d62fe98`. New shared `internal/llm.ExtractJSON`; all 4 verdict parse sites re-routed. Resolved session: `.planning/debug/resolved/glm-verdict-fence-parse.md`.
- **eval/judge.go OOS-1 follow-up** — FIXED, commit `d1af0f6` (quick task 260813-1i6). `glmJudge.generateJSON` now delegates to `llm.ExtractJSON`; private `stripJSONDelim` removed; hermetic judge-path regression test added.
- **Remaining Phase 5 work (1 open UAT issue #2):** see `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md`. §1 (eval follow-up) DONE; §2 (#7 hot-reload, polling-B) DONE 2026-08-13 (3d77870 + 3265178 BE + 09ae711 FE); §3 (#4 graph chunk full-text) DONE 2026-08-13 (8939a74 BE + aec582e FE + c646382 closeout — quick task 260813-dno; GET /api/knowledge/chunk/:id by-id content fetch, retriever→knowledge→server 3-layer mirroring graphHandler); **§4 (#2 left-sidebar IA) DONE 2026-08-13** (Plan 05-07, `gap_closure:true` `closes_uat:#2` — 7 commits `7cc252e`+`58e1bb5`+`277d7f3`+`b050967`+`9dbbe4d`+`0ac41d8`+`f6859d0`, merge `2873776`; 4 LOCKED decisions D5-IA-01/03/04/06 implemented; BLOCKING CONSTRAINT `item.manifest.*` honored; 209/209 vitest).
- Phase 5 UAT **11/11 pass — all Phase 5 work complete**. SYSK-04 gap closed by 05-07. Awaiting Phase-5 verification + phase completion.

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions + v0.8 milestone section
- `.planning/REQUIREMENTS.md` — 55 v1 + 25 v0.8 requirements + traceability (updated 2026-08-17)
- `.planning/research/SUMMARY.md` — v0.8 stack/pitfalls/6-phase order (P8.0 → P8.S → P8.1 → P8.2 → P8.3 → P8.4)
- `.planning/ROADMAP.md` — phase structure (v0.8 phases 8-13 appended 2026-08-17)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation. Last transition: Milestone v0.8 roadmap created 2026-08-17 — phases 8-13 appended (25/25 v0.8 requirements mapped; Phase 9 = spike hard gate for Phase 10); Phase 7 leftovers deferred to after v0.8 by user decision. Prior: Phase 4 → Phase 3 (resumed) on 2026-08-11. Plan 03-01 complete 2026-08-11. Plan 03-02 complete 2026-08-12. Plan 04-04 (Phase 4 gap-closure, wire-contract TS realignment) complete 2026-08-12 — `cd web && npx tsc --noEmit` now exits 0; Phase 4 debt-free. Plan 03-06 (Go-native trust evaluator, D6 reversed) complete 2026-08-12 — Phase 3 COMPLETE (6/6 plans); GEVL-04 + GEVL-05 marked complete.*
