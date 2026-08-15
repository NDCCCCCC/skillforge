---
gsd_state_version: 1.0
milestone: v0.7.7
milestone_name: milestone
current_phase: 7
current_plan: 4
status: executing
last_updated: "2026-08-14T13:05:10.354Z"
progress:
  total_phases: 9
  completed_phases: 6
  total_plans: 35
  completed_plans: 36
  percent: 67
---

# State — SkillForge

**Last updated:** 2026-08-14 (Completed quick task 260814-wre: 后端代码整理 — .gitattributes EOL 修复（gofmt -l 119→0）、cmd/spiked 迁至 scripts/spiked、根目录调试产物清理、go mod tidy。前置：**Phase 6 正式拆分**：code-side 范围（6a）标记完成 — 3/3 active plans + 11/12 must-haves（唯一 fail = 06-03 Actions 管道，移出范围）；06-03 CI wiring + 4 项 live human UAT 后置到 **Backlog Phase 999.2（= 6b）**。ROADMAP/PROJECT 已同步。修复 ROADMAP 预存损坏：Phase 4 段落的 plan list 曾错贴 06 内容，已还原 04-01..04-04。`06-VERIFICATION.md` 保持 `human_needed` 不变（反映真实状态），999.2 启动时转为其输入。)

**Prior:** 2026-08-13 (Plan 07-05 IN PROGRESS — Task 1 DONE: `scripts/eval_snapshot/{main.go,main_test.go}` shipped via TDD RED `abf2f4d` → GREEN `4873213`; `deriveRefusalAccuracy` pure fn is the ONE derivation point for DIST-09 refusal_accuracy [count(Cases cat=refusal-required && Refused) / count(Cases cat=refusal-required)]; 6/6 hermetic unit tests PASS, `go vet` clean, end-to-end smoke render matches RESEARCH Example 6 format; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. Tasks 2 & 3 are `checkpoint:human-verify gate=blocking` and BLOCKING — Task 2 needs live Milvus+BGE-M3+GLM stack to run `cmd/eval --frozen=compliance` and paste `scripts/eval_snapshot` stdout into README Eval Results [still PLACEHOLDER]; Task 3 needs screen capture to record `docs/demo.gif` [cited verdict + structured refusal, <5MB]. Plan 07-02 Task 3 docker-build human-verify gate ALSO still BLOCKING — Docker not in PATH. Phase 7 has 2 concurrent blocking checkpoints: 07-02 Task 3 + 07-05 Tasks 2/3.)
**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 7
**Current Plan:** 4
**Previous phase:** 04 (streaming-trust-ui) — COMPLETE 2026-08-11 (UAT 12/12, SECURITY verified / 0 open threats)
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** Plan 05-01 (Phase 5 skill-platform-&-full-knowledge-boundaries) is READY TO PLAN. Alternative: Phase 6 CI gate wiring (eval/report.json → GEVL-07 Skill promotion gate). The D2-15 threshold-swap procedure (manual ops review of `eval/calibration.json` `recommended_threshold` before editing `internal/server/pipeline.go:43`) is documented in `eval/README.md` + pipeline godoc.

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Go-native trust evaluator (internal/eval; faithfulness ports Ragas v0.1.21 prompt, Apache-2.0) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4. D6 reversed 2026-08-12: Python/Ragas sidecar eliminated in favor of single-language Go stack.

**License:** Apache-2.0.

**Current focus:** Phase 7 — open-source-release
**See:** `.planning/PROJECT.md` (updated 2026-08-11 after Phase 4)

---

## Current Position

Phase: 7 (open-source-release) — EXECUTING
Plan: 5 of 6 — **IN PROGRESS (1/3 tasks; Task 1 DONE via TDD; Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING — need live Milvus+BGE-M3+GLM stack + screen capture)**
**Phase:** 5 (skill-platform-&-full-knowledge-boundaries) — READY TO PLAN
**Plan:** not started
**Status:** Executing Phase 7 — Plan 07-05 awaiting human-verify of live-stack eval run + GIF capture (Plan 07-02 Task 3 docker-build also still BLOCKING)
**Just completed:** Plan 03-06 — Go-native trust evaluator (`internal/eval` + `cmd/eval`). D6 reversed 2026-08-12: Python/Ragas sidecar eliminated in favor of single-language Go stack. 4 RAG-trust metrics (faithfulness ports Ragas v0.1.21 prompts VERBATIM, Apache-2.0 attributed in `eval/NOTICE`; other 3 use Go-authored judge prompts). 50-case synthetic golden set (20 adversarial + 20 refusal-required + 10 multi-hop + 5 spike-anchors, GEVL-05). D2-15 calibration vehicle (`Calibrate()` → `eval/calibration.json` `recommended_threshold`; manual ops swap of literal 0.3 in `internal/server/pipeline.go:43`). Additive accessors `agent.LastRetrieved()` + `store.UpdateAuditMetrics()` (COALESCE partial backfill). SC-6 reproducibility gate via StubJudge + deterministic embedder; 34 eval tests pass. Pitfall 8 counter-measures in place (deterministic grounding floor, human spot-check, JUDGE_MODEL slot). 5 commits: `52dab7e` (D6 reversal docs) + `d4f96fc` (Task 1) + `a77be40` (Task 2) + `f7979af` (Task 3 A+B) + `247dfd8` (Task 3 C-H). GEVL-04 + GEVL-05 marked complete.

**Phase 4 shipped (trust visualization front-end):**

- Three-panel React 19 UI (资料库 / 审查结论 / 对话) — `web/src/components/layout/{ThreePanel,LeftSidebar,RightChat}.tsx`
- VerdictCard (D-06) + RefusalCard (D-07) + ReasoningPart (D-08) + ChunkSourceView (D-01/FEUI-03) + SkillPicker (FEUI-04) + DataSubprogressPart (FEUI-06)
- Wire-type compile-time drift alarm + MockChatTransport (D-04 swap invariant) + T-1-08 source-URL sanitization gate
- 83/83 unit tests green; UAT 12/12 (chrome-devtools MCP + unit evidence); 1 gap fixed (commit `e757d6e`)
- Requirements covered: FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08

**Deferred to Phase 5 (from 04-03 Known Stubs + 260811-uaf):**

- LeftSidebar real multi-session + file tree (current = mock skeleton)
- ChatInput `@`-mention fuzzy search (current = drag-drop + 10MB validation only)
- Skill manager / settings UI (topbar chevron is a no-op stub)
- Bidirectional tab nav + UI-SPEC §169 chunk empty-state

**Open dependency note:** Phase 3 (Agentic Retrieval & Trust Eval) was skipped under MVP vertical-slice reordering (Phase 2 → Phase 4). Its scope (Self-RAG self-verification, multi-hop, Ragas golden-set gates, audit row) is still open and may need to be sequenced before or alongside Phase 5.

---

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

---

## Accumulated Context

### Roadmap Evolution

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

- Phase 1: Eino APIs in depth, Hertz-to-AI-SDK v5 transport, Milvus filter expression syntax
- Phase 3: Chinese eval dataset sourcing, BGE-M3 reranker choice (self-host via TEI vs API), Ragas judge calibration
- Phase 5: hot reload for in-flight conversations, cross-tier conflict policy, historical tier storage, multi-agent value proof
- Phase 6: audit privacy (retention / redaction / access control), reproducibility, CI evaluation cost

---

## Blockers

(none)

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

- Phases completed: 3 (Phase 1 spike; Phase 2 compliance vertical slice; Phase 4 streaming trust UI) — Phase 3 deferred under MVP reordering, now in progress
- Requirements covered: 24 / 55 (Phase 2: 14; Phase 4: +FEUI-01..06, SYSK-03, KNOW-05, CMPL-05, T-1-08; Phase 3 03-01: +ARAG-01 substrate, +KNOW-01 substrate)
- Frontend tests: 84/84 green (14 files); UAT 12/12; `cd web && npx tsc --noEmit` exit 0 (debt-free since 04-04)
- Plans run: 15 (4 Phase 1 + 4 Phase 2 incl. MONGO + 4 Phase 4 incl. 04-04 gap-closure + 3 Phase 3 incl. 03-06 Go-native evaluator)
- Last plan: 03-06 (Phase 3 Go-native trust evaluator) — commits 52dab7e + d4f96fc + a77be40 + f7979af + 247dfd8. **Phase 3 COMPLETE (6/6 plans)**

---

## Session Continuity

**Resume point (Phase 7):** Plan 07-05 (eval snapshot + demo GIF) is IN PROGRESS — Task 1/3 DONE via TDD, 2 commits (`abf2f4d` RED test + `4873213` GREEN impl). `scripts/eval_snapshot/{main.go,main_test.go}` shipped: standalone Go program reads `eval/report.json` + `eval/frozen/compliance.lock.json` via `eval.LoadReport`/`eval.LoadLock`, **derives** `refusal_accuracy` from `Report.Cases` via the pure fn `deriveRefusalAccuracy` (count(cat=="refusal-required" && Refused) / count(cat=="refusal-required") — RESEARCH Q1 load-bearing, NOT an evaluator field), renders the README markdown snapshot block (date + lock metadata + Overall metrics + derived % + case breakdown + reproduce cmd). 6/6 hermetic unit tests PASS (all-refused 1.0 / half-refused 0.5 / zero-refusal-required graceful 0,N/A / category-filter invariant / renderSnapshot metadata / N/A cell); `go vet` clean; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. **Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING** — Task 2 needs live Milvus+BGE-M3+GLM stack (`docker compose up` + `ollama pull bge-m3` + populate `GLM_API_KEY`+`BGE_M3_URL` in `.env`) to run `go run ./cmd/eval --frozen=compliance` → `eval/report.json` → `go run ./scripts/eval_snapshot` → paste stdout into README `## Eval Results` (still PLACEHOLDER); Task 3 needs screen capture (ScreenToGif/OBS) to record `docs/demo.gif` (cited verdict + structured refusal flows, <5MB, embed already in README hero). Plan 07-02 (containerization) Task 3 docker-build ALSO still IN PROGRESS — Docker not in PATH. Plans 07-01 (release-meta) + 07-03 (docs) + 07-04 (examples+eval sample) COMPLETE. Next: human resolves 07-05 Task 2/3 checkpoints (which unblocks 07-06 publication governance — must NOT push to public remote until README eval numbers + GIF are in tree). See `.planning/phases/07-open-source-release/07-05-SUMMARY.md`.

**Resume point (legacy):** Plan 03-06 (Go-native trust evaluator) is COMPLETE — 3/3 tasks, 5 commits (`52dab7e` D6 reversal docs + `d4f96fc` Task 1 + `a77be40` Task 2 + `f7979af` Task 3 A+B + `247dfd8` Task 3 C-H). D6 reversed 2026-08-12: evaluator is now Go-native (internal/eval + cmd/eval); faithfulness ports Ragas v0.1.21 prompts VERBATIM (Apache-2.0, eval/NOTICE); other 3 metrics use Go-authored judge prompts. 50-case synthetic golden set shipped (GEVL-05: 20/20/10 + 5 spike-anchors). D2-15 calibration vehicle emits `recommended_threshold` to `eval/calibration.json` — manual ops swap of the literal 0.3 in `internal/server/pipeline.go:43`. **Phase 3 COMPLETE (6/6 plans)** — GEVL-04 + GEVL-05 marked complete. Next: Plan 05-01 (Phase 5 skill-platform) or Phase 6 CI gate wiring (eval/report.json → GEVL-07 promotion gate). **Phase 4 TS-drift debt CLOSED 2026-08-12 via Plan 04-04** — `cd web && npx tsc --noEmit` exits 0 (21 → 0 errors), vitest 84/84, build 411.12 KB / 120.47 KB gzip. See `.planning/phases/04-streaming-trust-ui/04-04-SUMMARY.md`.

**Debug fixes — 2026-08-13 (Phase 5 UAT blockers, /gsd-debug):**

- **#9 (blocker) wizard commit registration** — FIXED, commit `af1ca9a`. `wizardCommitHandler` rescans `Loader` + `Registered` flag; FE gates toast on `registered`; `Loader.cached` race-guarded. Resolved session: `.planning/debug/resolved/wizard-commit-registration.md`.
- **OOS-1 (blocker) GLM verdict fence-parse** — FIXED, commit `d62fe98`. New shared `internal/llm.ExtractJSON`; all 4 verdict parse sites re-routed. Resolved session: `.planning/debug/resolved/glm-verdict-fence-parse.md`.
- **eval/judge.go OOS-1 follow-up** — FIXED, commit `d1af0f6` (quick task 260813-1i6). `glmJudge.generateJSON` now delegates to `llm.ExtractJSON`; private `stripJSONDelim` removed; hermetic judge-path regression test added.
- **Remaining Phase 5 work (1 open UAT issue #2):** see `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-REMAINING-WORK.md`. §1 (eval follow-up) DONE; §2 (#7 hot-reload, polling-B) DONE 2026-08-13 (3d77870 + 3265178 BE + 09ae711 FE); §3 (#4 graph chunk full-text) DONE 2026-08-13 (8939a74 BE + aec582e FE + c646382 closeout — quick task 260813-dno; GET /api/knowledge/chunk/:id by-id content fetch, retriever→knowledge→server 3-layer mirroring graphHandler); **§4 (#2 left-sidebar IA) DONE 2026-08-13** (Plan 05-07, `gap_closure:true` `closes_uat:#2` — 7 commits `7cc252e`+`58e1bb5`+`277d7f3`+`b050967`+`9dbbe4d`+`0ac41d8`+`f6859d0`, merge `2873776`; 4 LOCKED decisions D5-IA-01/03/04/06 implemented; BLOCKING CONSTRAINT `item.manifest.*` honored; 209/209 vitest).
- Phase 5 UAT **11/11 pass — all Phase 5 work complete**. SYSK-04 gap closed by 05-07. Awaiting Phase-5 verification + phase completion.

**Hot files this session:**

- `.planning/PROJECT.md` — core value + decisions
- `.planning/REQUIREMENTS.md` — 55 v1 requirements + traceability
- `.planning/research/SUMMARY.md` — stack locked, pitfalls, recommended 7-phase order
- `.planning/ROADMAP.md` — phase structure (this file's sibling)
- `.planning/STATE.md` — this file

---

*State initialized 2026-08-10 during roadmap creation. Last transition: Phase 4 → Phase 3 (resumed) on 2026-08-11. Plan 03-01 complete 2026-08-11. Plan 03-02 complete 2026-08-12. Plan 04-04 (Phase 4 gap-closure, wire-contract TS realignment) complete 2026-08-12 — `cd web && npx tsc --noEmit` now exits 0; Phase 4 debt-free. Plan 03-06 (Go-native trust evaluator, D6 reversed) complete 2026-08-12 — Phase 3 COMPLETE (6/6 plans); GEVL-04 + GEVL-05 marked complete.*
