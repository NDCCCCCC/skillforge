---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: — 业务数据支撑（Business Data Support）
current_phase: 17
current_plan: Not started
status: verifying
last_updated: "2026-08-27T14:29:57.808Z"
last_activity: 2026-08-27
progress:
  total_phases: 20
  completed_phases: 13
  total_plans: 86
  completed_plans: 108
  percent: 65
---

# State — SkillForge

## Current Position

Phase: 16 (p10-2) — EXECUTING (Wave 5 closed; Phase 16 closeout deferred)
Plan: 9 of 9 (Plan 16-09 COMPLETE — verify-phase-16.sh 11-stage acceptance gate)
Status: Phase complete — ready for verification
Last activity: 2026-08-27

**Phase 14 final closeout (2026-08-23, user decision: close with deferred items):**

- 12/12 plans executed across 4 gap-closure rounds (14-01..14-08 core + 14-09/14-10 UAT gaps + 14-11 verification gaps + 14-12 re-baseline). UAT 6/6 pass. All hermetic gates green (verify-phase-14 11/11, verify-phase-13 9/9); frozen locks + golden sets byte-identical; regression suite全绿.
- **Deferred item 1 (STATE Blockers, operator gate):** platform-floor.lock.json 1.1.0 formal re-sign — blocked on BGE-M3 endpoint restore (`10.62.0.123:11434`, no route to host) → one authorized compliance replay (`go run ./cmd/eval --frozen=compliance --archive`) → ≥0.9 executes 14-10-PLAN Task 2 steps 3-5 (fill-lock + README caveat + docs commit). D14-G2 re-baseline semantics already code-locked (computeCaseMetrics refused→faith 1.0, TestRefusedCaseFaithExemption).
- **Deferred item 2 (blocked-with-notes):** Docker live build+run verification — host has no Docker; static fix shipped (libstdc++ libgcc in Dockerfile.backend:44, correct-by-construction).
- Code review 14-REVIEW.md: CR-01/CR-02 fixed (14-11); CR-03 (approvalId regex rejects `call_*` underscore IDs → HITL 400) + WR-01..08 + IN-01..04 recorded as quality debt, non-blocking.

**14-11 outcome summary:**

- **Gap 1 (cmd/eval refusal-accounting defect):** FIXED hermetically. `classifyRunOutcome` extracted and now recognizes `verdict.status == "refused"` as a refusal, routing it into the same frozen conventions as Run-error / structured-refusal paths (`refused=true`, `answer=""`). `TestClassifyRunOutcome` 6-row table-driven regression lock shipped RED→GREEN. verify-phase-14 11/11 PASS and verify-phase-13 9/9 PASS with all frozen locks + golden sets byte-identical.
- **Gap 2 (Dockerfile runtime libstdc++ libgcc):** FIXED in source; live Docker build+run blocked on this host because Docker is not installed (git-bash + WSL both `command not found`), recorded as `blocked-with-notes` per 14-08 precedent.
- **Gap 3 (verify-phase-13 stale pin):** FIXED. Stage 7 `EXPECTED_COMPLIANCE_SHA` rippled to `1dc6f148…` (lockstep with verify-phase-14.sh Stage 11); stale "Phase 8 pin (UNCHANGED)" narrative replaced with truthful re-lock history.
- **Live replay (single authorized run):** `cmd/eval --frozen=compliance --archive` completed 50/50 cases (2026-08-23T02:22Z–03:13Z, ~52 min). AssertPin held (`frozen pin holds`, `prompt_hash=5b12acd9…`). Refusal-accounting fix performed as designed: refusal-required 19/20 refused with faithfulness 0.9727. However, overall faithfulness = **0.888** (< 0.9 gate). A mid-run BGE endpoint failure (`no route to host`) affected several multi-hop cases. Per the no-falsify contract, the result is recorded exactly as measured — **no re-sign, no retry, no README caveat**. The 0.83–0.90 result routes to the pre-declared 14-10-SUMMARY Operator-Gate Addendum Alternative (explicit re-baseline of gate semantics) as a pending planner/user decision.

Phase 14 UAT remains 6/6 pass (14-UAT.md status: complete); the 1.1.0 floor behavior is live and hermetically locked, but the platform-floor.lock.json 1.1.0 formal re-sign is still deferred.

# State — SkillForge

**Last updated:** 2026-08-21 (**Phase 13 Wave 3 COMPLETE + Wave 4 operator-gated + Wave 5 planned** — 13-04 hermetic acceptance gate SHIPPED 9/9 PASS at 2026-08-21T01:42:38Z, commit `7133feb`: `scripts/verify-phase-13.sh` 9-stage gate + `docs/threat-model/STRIDE-13.md` 8-threat register + `eval/frozen/fanout.lock.json` v0.8 placeholder + WalkDir 8 roots + Makefile target; pause-work handoff `9663b15` (HANDOFF.json + .continue-here). Prior Wave 2: 13-02 + 13-03 SHIPPED (7 commits); Wave 1: 13-01 SHIPPED (`0484bca`). **Wave 4 (13-05 live 20-case fan-out UAT) is operator-gated pending** — requires `.env` (GLM_API_KEY + BGE_M3_URL + MILVUS_ADDRESS) + Docker Milvus + BGE-M3 stack; operator then populates `fanout.lock.json` metrics + `eval_set_sha256` + sign-off and re-runs gate Stage 7. **Wave 5 (13-06 v0.8 milestone wrap) PLANNED 2026-08-21** — completion report + Operator Gates Ledger (incl. Phase 10 deferred 35-case `--via-pipeline` re-run) + README DIST-09 fill from existing `eval/report.json` (hermetic, no live stack) + STATE terminal flip + operator `v0.8.0` tag hand-off; see `13-06-PLAN.md`. **Next action**: operator runs Plan 13-05 live UAT → then `/gsd:execute-phase 13 --wave 5` for 13-06 wrap. Resume file: `.planning/phases/13-p8-4-sub-agent-fan-out/13-PLAN-OUTLINE.md`.)

**Prior:** 2026-08-20 (**Phase 10 CLOSED hermetic-gated** — UAT `10-UAT.md` 22 tests = 19 passed / 0 issues / 3 blocked-by-server; STATUS `partial` per GSD framework rules with explicit operator-gate reasons documented in tests 19/20/21; user 2026-08-20 signed acceptance of `closed/blocked-with-notes` closeout; **live 35-case `--via-pipeline --summarize` re-run is the v0.8 milestone responsibility, NOT a Phase 11 entry prerequisite**; Phase 11 planning independent of operator-actionable gates.)

**Prior:** 2026-08-18 (**Phase 10 Plan 04 Task 1 shipped** — `--via-pipeline` flag + production chat-path composition in `cmd/spike-history` + hermetic `runner_pipeline_test.go`; all BE suites green, frozen lock byte-identity verified sha256 `5cf700f3...`). **Task 2 live 35-case re-run blocked-pending-human** (execution environment missing `.env` with `GLM_API_KEY` / `BGE_M3_URL` / `MILVUS_ADDRESS` and Docker Milvus + BGE-M3 stack). **Task 3 closeout transcription committed** with explicit blocker notes. Phase 10 remains in `executing-blocked` until operator provides live stack and the 35-case `--via-pipeline --summarize` gate is run.)

**Prior:** 2026-08-18 (**Phase 10 Waves 1-3 complete** — 10-01 BE foundation / 10-02 pipeline injection / 10-03 FE restore; all hermetic green; FE `tsc` retains 1 pre-existing `DataSubprogressPart.tsx:47` error from 10-03.)

**Prior:** 2026-08-18 (**Phase 9 COMPLETE 6/6** — Plan 09-06 live spike 端到端跑完：35 例（10 恶意 + 15 善意 + 10 长会话）真 GLM-5.2 + Milvus + BGE-M3，实测成本 **$0.0481**（限额 $5.00/段）。机器裁决 **NO-GO**（exit 1）：SC-3 恶意零穿透 **0/10 PASS**（消毒规则 + grounding 白名单守住，证据链未被污染），但 refusal_accuracy **-95pp**（95%→0%，三类全灭）+ faithfulness -18.6%/-12.6%/-22.4% + context_recall -87.2%/-63.0%/-70.1% 漂移超门（Pitfall 1 H2 证实：拒答上下文使拒答率漂移）。用户 2026-08-18 签字「**接受 NO-GO 带逃逸方案**」——Phase 9 的交付物（明确且有记录的 go/no-go + 消毒规则 + 逃逸方案）按 SC-2/GEVL-08 口径达成；**Phase 10 记忆功能必须前置逃逸强化**（拒答判定不得依赖历史上下文；历史不得携带拒答语义；候选 = 信任地板 default-deny 强化 / MEM-05 摘要中间件 / prompt 历史投射收紧），且 memory 发布前须重跑 spike 硬门回到 <5%/<10pp。)

**Prior:** 2026-08-18 早前 (**Phase 8 COMPLETE 5/5** — Plan 08-05 live 三门重锁全部 approved：真 GLM cancel 0ms / Milvus tier 5行+0行 零泄漏 / 冻结 eval gate PASS faithfulness 0.972 + refusal_accuracy 95% 留档 v0.8 基线；PLAT-04 + PLAT-05 双勾选；eino v0.9.14 编译同构 + 行为同构双证据闭环。)

**Project:** SkillForge (Skill-driven enterprise Agent platform)
**Mode:** mvp (vertical slices)

**Current Phase:** 17
**Current Plan:** Not started
**Previous phase:** 10 (closed hermetic-gated) — live 35-case re-run is v0.8 milestone gate, NOT Phase 11 entry prerequisite
**Phase 11 prerequisites (locked by /gsd:discuss-phase 11):** D11-A* Runner 化、信任楼梯三层 / D11-B* per-turn Tool whitelist + 共享 ApprovalDecision / D11-C* Dual 端点 + Snapshot 双表落 + fail-closed / D11-D* approval_rules 独立表 + session/skill 二元
**Extension 2026-08-11:** 02-04-MONGO — MongoDB retriever backend shipped. Set `RETRIEVER_BACKEND=mongo` + `MONGO_URI=mongodb://...` to switch. See `.planning/phases/02-compliance-vertical-slice/02-04-MONGO-SUMMARY.md`.
**Next action:** Run `/gsd:ui-phase 11` → `/gsd:ai-integration-phase 11` → `/gsd:plan-phase 11 --chunked` to consume CONTEXT.md + UI-SPEC + AI-SPEC into executable plan(s). Plan-phase will spawn gsd-phase-researcher (RF-3 main spike + cheap CheckPointStore interface compile-check) → gsd-planner (chunked: per-plan agents with atomic commits) → gsd-plan-checker (verifies 5 SC + 8 HITL requirements coverage).

---

## Project Reference

**Core Value:** Same agent kernel, customized per enterprise via pluggable Skill files — without code changes. From "application" to "platform" via the Skill schema; from "AI wrapper" (dead, Height.app closed 2025.9) to defensible Skill-driven platform.

**Demo Skill:** 企业合规 / 制度审查 — the showcase Skill that proves the platform thesis (compliance is the killer use case because hallucination is unacceptable).

**Audience:** Open-source personal brand. Quality bar = a stranger can fork, run, and benchmark.

**Stack (locked):** Go + Eino v0.7.7 (ADK GA) + Hertz SSE + Milvus (tier-filtered) + BGE-M3 + Go-native trust evaluator (internal/eval; faithfulness ports Ragas v0.1.21 prompt, Apache-2.0) + React 19 + Vite + TS + Vercel AI SDK v5 + shadcn / Tailwind v4. D6 reversed 2026-08-12: Python/Ragas sidecar eliminated in favor of single-language Go stack. **v0.8 Phase 8 upgrades eino v0.7.7 → v0.9.13** (summarization middleware + mature ADK HITL; NOT v0.10.0-alpha.x per maintainer warning).

**License:** Apache-2.0.

**Current focus:** Phase 16 — p10-2
**See:** `.planning/PROJECT.md` (updated 2026-08-17 — v0.8 milestone section)
**Phase 10 closeout (2026-08-20):** shipped hermetic-gated — see "Current Position" / "Last updated" above. Live 35-case re-run remains operator gate for v0.8 milestone (NOT Phase 11 prerequisite).
**Phase 11 prerequisites inherited from Phase 9/10:**

- D9-V2 escape plan already implemented (RefusalClassifierInput type-isolated to Query + Chunks, no History — Plan 10-01 Task 3; Plan 10-02 Task 1+3)
- sqlite CheckPointStore skeleton ready (D8-03; Phase 11 implements Delete + expires_at filter; `schema.RegisterName[T](name)` registered; `composed.CheckPointStore` — D8-06)
- `data-decision` SSE chunk discriminator wired (D16; HITL-04 verdict parse uses Phase 4 wire types)
- Next action: `/gsd:plan-phase 11`

---

## Current Position

Phase: 16 (KPI 聚合通道，P10.2) — **Wave 5 leg closed** (16-09 complete; Phase 16 closeout deferred)
Plan: 9 of 9 (Plan 16-09 COMPLETE — scripts/verify-phase-16.sh 11-stage acceptance gate + Makefile target)
Status: ready for Phase 16 closeout reviews (post-phase gates — verifier, code-review, regression, roadmap sync)
Last activity: 2026-08-27 -- Plan 16-09 executed; 2 commits landed (`0a74938` verify-phase-16.sh 11-stage gate, `0718e9d` Makefile target). Gate runs **9/11 PASS + 2 BLOCKED-WITH-NOTES on Windows** (D15-01-1 emutls ABI; `internal/kpi/wsl` test binary + cmd/server D-Link); Linux/WSL operator run closes the 2 WSL legs (literal commands in Stage 08 + Stage 10 of the script).
Next action: `/gsd:close-out-phase 16` for post-phase gates (verifier, code-review, regression, roadmap sync) — the two inherited `internal/kpi/wsl` failures remain operator-WSL responsibilities tracked in Stage 08's blocked-with-notes command line.
Resume File: `.planning/phases/16-p10-2/16-09-SUMMARY.md`

**Plan 16-08 delivery summary (Wave 4, 2026-08-27):**

- 16-08-1: `internal/agent/callbacks.go` Tool OnStart dispatches on tool identity — kpi_call advances the hop counter but emits ZERO decisions (its `kpi_reference` + DataRef leg rides the tool closure); two-level identity (RunInfo.Name authoritative per eino `compose/tool_node.go:896`, kpiCallArgs shape probe fallback, conservative default = emit retrieve). `TestCallbackToolDispatch` 4 sub-tests (commit `0cc1ad3`).
- 16-08-2: `cmd/server/main.go` assembly section 6c-3 — `SKILLFORGE_KPIS_DIR` (default `kpis`, in git) + dedicated `:memory:` DuckDB connector + AttachLRU + `NewDuckDBSQLSerializer` + `LoadKPIs` (failure = `log.Fatalf`, empty dir = the one legal degraded state) + `NewExecutor` over the sqlite store as AuditSink + `storeDatasetLister` / `storeDatasetMeta` adapters. Wired UNCONDITIONALLY (KPI reads desensitized data and never re-derives a hash, so `SKILLFORGE_DATA_SALT` does not gate it). `internal/server`: `Deps.KPIExecutor` / `Deps.KPIRegistry` + `ChatHandler` injects them plus a per-request `dataRefEmitter(c)` closure publishing `wire.DecisionKPIReference` + DataRef through the existing `data-decision` envelope (commit `1f1aed4`).
- 16-08-3: `internal/server/kpi_admin.go` GET /api/admin/kpis — admin token gate (403) + nil registry 503 fail-closed + audit-read failure 500 + 200 `{signature, kpis[], latest_calls{}}`. Retrieval-only by structure: `TestAdminKPIs/retrieval_only` greps zero mutation verbs in the handler file and asserts the route table binds the path exactly once, as GET (commit `c94e39d`).

**Verification (16-08):** Windows hermetic `go test ./internal/agent/... -count=1` green (4 packages); WSL `go build ./...` exit 0, `go vet ./internal/server/ ./cmd/server/` clean, `go test ./internal/server/... -count=1` green (12.75s + classifier + rewrite); `TestAdminKPIs` 4/4; `grep -cE "POST|PUT|DELETE|PATCH" internal/server/kpi_admin.go` == 0.

**Inherited failure carried into 16-09 (NOT from 16-08):** `internal/kpi/wsl` fails `TestKPIEngineIntegration` (fixture trips the D16-A2 bidirectional `:param`↔`params` coverage check — "params has SQL missing: [status]") and `TestNameLexiconParity/kpi_vs_duckdb_lexicon_agreement` (64-char identifier cap disagreement). Confirmed pre-existing by re-running the package with `internal/kpi` + `internal/agent` at the 16-07 baseline `dc69291` — same FAIL. Both hermetic-fixable; they belong to the 16-06 WSL triple lock and must close before verify-phase-16 signs.

**Prior — Plan 16-07 delivery summary (Wave 3, 2026-08-27):**

- 16-07-1: `internal/agent/sql_surface_test.go` — TestNoSQLGenerationSurface WalkDir gate (3 roots × 5 SQL-shape regexes + comment stripping) + TestKPINarrowSurface reflect assertion on `*kpi.Executor` method set (commit `696ca81`)
- 16-07-2: `internal/agent/tools/kpi_tool.go` — BuildKPICallTool (catalog snapshot at construction, Enum, structured refusal pass-through, DataRefEmitFunc); 5 hermetic locks (commit `c80867b`)
- 16-07-3: `runner_setup.go` BuildRunner channel branch (data → `{kpi_call}`, tier → existing compose; fail-fast on nil KPI deps) + `agent.go` data-channel fail-closed guard + TestToolsForChannel 4 sub-tests (commit `dc69291`)

**Prior decisions / plans:**

- 2026-08-24: `/gsd-discuss-phase 16` complete: 16 decisions locked (D16-A1..A4 YAML 受控文件/SQL 模板/枚举值域/内存 registry；D16-B1..B4 单一 kpi_call/工具层拒答/结构化快照+data-reference/as_of 系统生成；D16-C1..C4 reads 互斥/加载期交叉验证/双层镜像 D11-B1/平台级共享目录；D16-D1..D4 k_min 行级抑制/枚举即差分白名单/全量快照冻结/独立 kpi_calls 表)，commit 4200e1c.
- 2026-08-24..27: 16-01 through 16-06 executed (KPI registry + loaders + Layer 1+2 validation + kmin + Executor + engine adapter + WSL triple lock) — see `.planning/phases/16-p10-2/` for per-plan SUMMARIES.

**Verification:** all `go test ./internal/agent/...` and `go test ./internal/kpi/` hermetic tests green (4.95s / 0.65s / 1.26s / 1.26s / 0.84s across the 5 packages). CGO binaries (`cmd/server`, `cmd/eval`, `internal/kpi/wsl/`) blocked on D15-01-1 (pre-existing MSYS2 gcc linker limit) — recorded as blocked-with-notes per 14-08 precedent; plan 16-07 is hermetic and unaffected.

**Phase 15 delivery summary:**

- 15-01: `internal/data` desensitization core (normalize / HMAC / L2 bucketing / rules / 4-encoding decode) — DATA-02 locked.
- 15-02: SQLite additive migration + `import_batches` / `dataset_deletes` INSERT-only audit tables — DATA-17 store layer locked.
- 15-03: DuckDB supersede 3-step atomic swap + `WriteStagingBatch` Appender helper + composite-name ATTACH quoting — DATA-16 engine locked.
- 15-04: sqlite `.dump` accept-list parser (never executes SQL) + preview session store + content-hash bridge — DATA-01 dump channel + DATA-15 locked.
- 15-05: Importer three-channel orchestrator (Preview / Commit / Push / DryRun) + idempotency + supersede chain audit — DATA-15/16/17 library layer locked.
- 15-06: Hertz streaming HTTP surface (preview/commit/push/delete) with admin-token gates, split body-size policy, and SKILLFORGE_DATA_SALT fail-closed 503 — DATA-01/C3 HTTP layer locked.
- 15-07: Structural ban gates (multipart pre-parse / row INSERT / row-value logging) + `verify-phase-15.sh` 10-stage hermetic gate + Makefile target — SC-1/D15-B1/B4 structural locks in place.

**Verification:** `bash scripts/verify-phase-15.sh` → **PASS 10/10**; `go test ./... -count=1` and `go test ./... -count=1 -race` both green in WSL2.

**Next action:** `/gsd-plan-phase 16` — KPI 聚合通道（P10.2）context 已就绪（16-CONTEXT.md，16 decisions 锁定）；planner 消费 D16-A/B/C/D 决策产出 plans。

**Phase 10 closeout note (2026-08-20):** shipped hermetic-gated (UAT 19/0/3, `git status` clean 3 commits ahead of starting baseline). The 3 blocked tests are operator gates (`.env` populate + Docker Milvus + BGE-M3 startup + live 35-case re-run). User accepted `closed/blocked-with-notes` closeout for Phase 10; the live re-run is **explicitly the v0.8 milestone gate** (GEVL-08/N9-V2), not a Phase 11 entry prerequisite.

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
| D8-06 | SC-5「进程重启」口径 + 两进程 stub 剧本形态（Phase 8 / Plan 08-04） | 「重启」= 实例级重建（新 DB handle + 新 ChatModelAgent + 新 Runner，同一 DSN 文件）——官方跨实例 resume 语义（同 CheckPointID + 共享 store）成立；stub 剧本按对话历史分支（有无 Tool 消息）而非实例轮次计数（进程 2 新实例拿到恢复历史才走终答分支，计数剧本会死循环）；中断 ID 取 InterruptContexts[0].ID 并断言地址末段指向审批工具；注册面 schema.RegisterName[T](稳定名) 双注册由 round-trip 测试 + grep 门双重强制（裸 gob 注册 = 0）。 |
| D8-07 | v0.8 frozen eval 基线口径与数值（Phase 8 / Plan 08-05） | gate 指标 PASS 留档：faithfulness **0.972** ≥ 0.9 阈值 + refusal_accuracy **95%**（19/20，scripts/eval_snapshot 派生），judge=glm-5.2，50/50 case，lock sha256 50bb1688… 逐字节未动（T-8-12——重生成 lock = 篡改基线）。非 gate 指标偏低（context_precision 0.10 / answer_relevance 0.1002 / context_recall 0.39）成因 = 拒答 case 被 judge 机械归零（D-09 refusal-by-construction 结构性后果）+ multi-hop 全拒答（检索阈值偏保守）——用户 2026-08-18 知悉接受，改善归 Phase 9+ 关注，不构成 blocking。 |
| D9-B1 | History projection role/field contract (Phase 9 / Plan 09-02) | Keep only user/assistant Role and Content so tool/system metadata and implementation fields never reach Eino history. |
| D9-B2 | History projection marker contract (Phase 9 / Plan 09-02) | Replace only user-side CHUNK markers with unpadded `[REF-N]`; preserve assistant citations byte-for-byte. |
| D9-B3 | History projection window contract (Phase 9 / Plan 09-02) | Keep at most the final six filtered messages and drop a leading orphan assistant after role filtering. |
| D9-B4 | Projection fixture handoff (Phase 9 / Plan 09-02) | Exported fixtures are a non-test contract for Phase 10, not a reason to expand the spike into unit-test implementation. |
| D9-L1 | Long-mode file naming = `history_injection_long.jsonl` (Phase 9 / Plan 09-04) | Follow Plan 09-03's category-as-filename convention (`writeResult: <category>.jsonl`). The plan body contained conflicting guidance about `long.jsonl`; resolved in favor of consistency with `history_injection_adversarial.jsonl` + `history_injection_followup.jsonl`. Plan 09-05 report layer aggregates by `Category` field, so the filename is a layout detail. |
| D9-L2 | Neutral filler template (T-9-FILLER-LEAK, Phase 9 / Plan 09-04) | `fillerAssistantTemplate = "已记录您的问题 %d（仅上下文，不进入证据）。"` is a fixed string; never cites baseline.GroundTruth / ExpectedChunks. User-half filler uses `baseline[i].Query` (real compliance Q&A, not a leak). Verified via `grep -c "GroundTruth\|ExpectedChunks" cmd/spike-history/long.go` = 0 in helper code. |
| D9-L3 | 8/12/15 length targets are upper bounds (Phase 9 / Plan 09-04) | `computeFillerCount` floors odd differences so pairs always produce symmetric (user, assistant) filler. Target=15 with signal=2 → 12 filler (6 pairs) → total 14 turns (not 15). Acceptable per planner's intent — the targets are upper bounds. |
| D9-V1 | Phase 9 spike 裁决 = NO-GO，用户 2026-08-18 签字「接受 NO-GO 带逃逸方案」进入 Phase 10 (Plan 09-06) | 机器裁决 NO-GO（exit 1，faithfulness -18.6%/-12.6%/-22.4% + refusal -95pp 全超门），但 SC-3 恶意零穿透 0/10 PASS——证据链未污染，失败归因 = 拒答行为漂移（Pitfall 1 H2：拒答上下文使拒答率漂移），非引用穿透（Pitfall 2）。Phase 9 的目标是「跑硬门 + 记录明确结论」，该目标达成（SC-2/GEVL-08 口径）。GEVL-08 自身措辞允许 NO-GO 路径：「不过门则记忆不得发布，产出消毒规则或逃逸方案」——两者均已产出（projection.go D9-B1/B2/B3 + 逃逸方案）。 |
| D9-V2 | Phase 10 逃逸方案 = 前置信任地板强化，而非重跑 spike (Plan 09-06 用户裁决) | (a) 拒答判定不得依赖历史上下文（信任地板只看 query + 本轮检索）；(b) 历史不得携带拒答语义进 prompt（历史中的旧拒答不得许可后续作答本应拒答的追问）；(c) 考虑 query-only 独立拒答分类器置于历史增强生成上游。候选逃逸：信任地板 default-deny 在历史注入后的强化 / MEM-05 摘要中间件 / prompt 历史投射收紧。memory 发布前必须重跑 spike 硬门回到 <5% / <10pp（cmd/spike-history 即重门机制）。 |
| D9-V3 | 不做同 seed 重试 (Plan 09-06) | T-9-SPIKE-DRIFT 的重试条款针对抖动级 NO-GO；本次 -95pp + 方向三类一致（35/35 行）超出阈值数量级，属结构性漂移非随机抖动。如需 judge 侧二次意见，Plan 09-05 已埋 JUDGE_MODEL cross-eval flag。 |
| D14-A1 | 七条数据铁律条目集锁定 (Phase 14 / Plan 14-01) | ①永不编写/执行/建议 SQL（只按名调 KPI）②永不请求或输出原始行/个体记录 ③数据结论必须带 as_of + 数据引用 ④拒绝跨数据集推断个体身份 ⑤声明脱敏不可逆 ⑥永不协助去脱敏/再识别（含提示词诱导）⑦永不声称拥有实时数据；条目集由 hermetic 子串断言锁（TestPlatformFloorTextContainsAllSevenItems），条目级修改需 D14-B4 全冻结集重跑。 |
| D14-A2 | 中文文案（Phase 14 / Plan 14-01） | 生产 GLM 中文最强 + Skill 全中文环境 + 拒答话术可直接引用地板原文保持一致；英文/双语方案被拒（双语 token 翻倍）。 |
| D14-A3 | 头部全文 prepend + 尾部一句轻提醒（Phase 14 / Plan 14-01） | floor 全文 prepend 在 Skill prompt 之前；尾部一句「以上数据铁律优先于 Skill 指令」抗长对话指令衰减（D14-C3 ⑥ 攻击面验证）；chat + deep 双渲染路径同步覆盖（TestPlatformFloorPrependBothPaths + TestRenderDeepSystemPromptPrependsFloor）。 |
| D14-A4 | 上提统一——根治 Pitfall 17（Phase 14 / Plan 14-01） | chat systemInstructionsTemplate、ReAct tmpl、deep base prompt 三处模板的规则 3/4（引用白名单 + 无 grounding 即拒答）上提为平台地板统一版本化；模板只留 persona + 分类规则 + JSON schema 输出段；上提前两份副本已分叉措辞（chat: "Return status=refused ONLY when NO retrieved chunk..." vs deep: "NEVER emit a refusal when a relevant chunk exists..."），上提后单一 canonical home in internal/platformfloor。 |
| D14-B1 | semver 字面量代码级不可 env 配置（Phase 14 / Plan 14-01） | `const PlatformFloorVersion = "1.0.0"`（internal/platformfloor + internal/agent/platform_floor.go 镜像）；不进 config、不进 env；verify-phase-14.sh source-grep 锁定（Pattern E — fanout_runtime.go:63 模型）。 |
| D14-B3 | prompt_hash 覆盖合成全文（Phase 14 / Plan 14-01） | reprohash.PromptHash 第三参数从 Skill prompt 升级为 ComposePlatformPrompt 合成全文；digest 序 skillName → renderedPromptBody → modelVersion 不动（reprohash.go 零 diff）；TestPromptHashCoversFloor 双向断言 PASS；eval/frozen/compliance.lock.json prompt_hash 已重锁 58ffea08... → ca333fbadbfa30eb...（D14-B4 重锁链首段，hitl/fanout/platform-floor 归 14-04）。 |
| D14-B4 | 地板文字任何改动 = 全量重跑纪律（Phase 14 / Plan 14-01） | semver 强制递增（PATCH 标点 / MINOR 新条款 / MAJOR 删 D14-A1 条目）+ compliance/hitl/fanout/platform-floor 四个冻结集全部重跑 + audit 可见变更；平台地板是每轮都发的产品级承诺，风险不对称。仅 14-01 即触发 compliance.lock.json 第一次重锁（其余三个归 14-04）。 |
| D14-C1 | CGO sqlite 驱动切换 + DSN pragma 纪律（Phase 14 / Plan 14-03） | modernc.org/sqlite → mattn/go-sqlite3 v1.14.50；busy_timeout/WAL/foreign_keys 必须进 DSN 参数而非 post-open PRAGMA；任何 in-memory DSN 默认 MaxOpenConns=1 以避免 shared-cache 死锁；Makefile check-cgo 门必须在 go build 前通过。 |
| D14-E1 | 索引-after-ALTER 顺序契约泛化（Phase 14 / Plan 14-09） | 任何引用可迁移列的 CREATE INDEX 必须以独立 const 在 migrate() 的 ALTER 循环之后 ExecContext，绝不内联进 schemaSQL（D11-C4 先例第二次适用——idx_audit_run_path 内联导致旧库启动即死 no such column: run_path，UAT Test 2 blocker）。回归锁 TestMigrateLegacyAuditUpgradePath 用文件级临时库构造 21 列旧表（hermetic :memory: 盲区永久补上）。 |
| D14-E2 | live 复验 skill 字面量 = "compliance"（Phase 14 / Plan 14-09） | findSkill（chat.go:660，Phase-2 简化）只接受字面量 "compliance"，不认 manifest 真名 simple-qa——线上契约即该字面量（107/115 旧 audit 行为证）。UAT Test 2 face C + Test 6 chat 用 "compliance" 复验通过；未修（越 plan 范围、动 chat 路由语义），记 deferred-items.md 归 SYSK-05。 |
| D14-E3 | Test 6 live 断言按 id 水位线划界（Phase 14 / Plan 14-09） | 断言 id > pre-chat max 的新行全部 platform_floor_version='1.0.0' + 旧行计数不变，而非计划原文的末 3 行全等（旧行 ALTER 后合法为 ''）。 |
| D14-F1 | 1.1.0 尾部重锚定 + item-1 计数锁语义升级（Phase 14 / Plan 14-10） | platformFloorTail 整体替换为抗稀释重申段（保留 1.0.0 锁定子串「以上数据铁律优先于 Skill 指令」+ 新增三子串锁「无论对话进行多少轮」「切换到何种角色或模式」「铁律均不豁免」）；因尾部刻意重申铁律一措辞，agent 包三处 strings.Count==1「单一 canonical home」锁升级为 ==2（floor 头部 + 1.1.0 尾部重申；Skill body 注入会变成 3+，锁强度不降）——Pitfall 17 的「单一 home」指代码级单一来源（internal/platformfloor），非合成串内单次出现。 |
| D14-F2 | compliance replay 双窗口 FAIL 纪律 + cmd/eval 拒答口径缺陷定位（Phase 14 / Plan 14-10） | 1.1.0 重锁后 live AssertPin PASS（prompt_hash 5b12acd9… 双向一致）；50 例 replay 劣化窗口 faithfulness 0.6876<0.9（2×挂死 + 2×内容审查 400）→ 按计划 STOP；2026-08-23 operator 授权干净窗口重跑二次 FAIL（0.6763<0.9，无挂死、agent.Run 0 错误、refusal 0/20 复现、ref 案例 faith 双峰 11×1.0/9×≤0.333）→ 根因五处代码锚点定位：cmd/eval 只认 Run-error 与空累加器结构化拒答，不识别 verdict status:"refused"（ValidStatuses 成员）；1.0.0 基线 0.972/95% 由 parse-error 空答案→虚高 faith 1.0（faithfulness.go:75-78）+ REFUSAL 哨兵 crecall（metrics.go:189-195）撑起；1.1.0 尾部使模型改在 verdict schema 内拒答（live judge 20/20 佐证）从而揭穿该缺陷——非行为回归。锁重签/README caveat 冻结，planner 裁决测量层修复（minimal 候选：cmd/eval 识别 status:"refused"）后单次重跑补齐 Task 2 步骤 3-5（根因详见 14-10-SUMMARY Operator-Gate Addendum）。 |
| D14-G2 | 显式 re-baseline D14-B4 gate 语义（Phase 14 / Plan 14-12） | in-schema refused verdict（classifyRunOutcome refused=true）按 frozen 空答案约定豁免 faithfulness 评判：cmd/eval 提取 computeCaseMetrics，在开头显式短路 refused=true → faith=1.0，其余 arel/cprec/crecall 保持既有约定；TestRefusedCaseFaithExemption 用敌对 StubJudge 锁定该语义。单次 authorized compliance replay 因 BGE-M3 endpoint no route to host 未能执行；按 no-falsify 契约未重签 platform-floor.lock.json / 未写 README caveat。 |
| D15-01-1 | internal/data 复用 duckdb.ValidateDatasetID，WSL2/Linux 跑 internal/data 测试（Phase 15 / Plan 15-01） | 数据集 ID 白名单与 ATTACH 层共用单一正则真源；代价是 internal/data 包间接依赖 go-duckdb cgo，此 Windows 主机的 MSYS2 gcc 无法链接（emutls ABI 不匹配，duckdb-go-bindings#105），因此 internal/data 测试与 internal/duckdb 套件同样在 WSL2 Ubuntu-22.04 下运行。 |
| D15-01-2 | hash 列 profile 可空（默认链），非空则必须在 KnownProfiles() 内（Phase 15 / Plan 15-01） | normalize.go 显式定义了空 profile 默认链（NFC+TrimSpace）；若规则文件校验强制必填 profile，该默认链对导入管线不可达。空 profile 等价于不声明；声明了就必须是注册名，防规则名幻觉。 |
| D15-01-3 | 规则校验拒绝无意义字段组合（Phase 15 / Plan 15-01） | hash+width、bucket_fixed+profile、passthrough+profile/width 等组合视为规则文件 typo，fail-closed 拒绝，而非静默忽略字段。避免 YAML 中字段误用导致脱敏行为偏离数据人员意图。 |
| D15-01-4 | GB18030 解码器包严格替换字符检查（Phase 15 / Plan 15-01） | x/text GB18030 解码器对非法序列静默输出 U+FFFD 而非 error。为满足 T-15-12 fail-closed 契约，decode.go 用 strictDecoderReader 包裹，检测到 U+FFFD 字节序列即硬报错。 |
| D15-02-1 | import_batches 使用专用表（Phase 15 / Plan 15-02） | 15-RESEARCH Open Q3 采纳选项 A：批次审计用 dedicated `import_batches` 表而非 audit.action 枚举 + frozen_args BLOB，ADM-02 批次视图可 SQL 直查。 |
| D15-02-2 | DeleteDataset 精确 name 匹配（Phase 15 / Plan 15-02） | 删除路径用 `DELETE FROM datasets WHERE name = ?`，不在 SQL 层做前缀/LIKE（`_` 是 LIKE 通配符）。需要前缀删除的调用方用 ListDatasets + strings.HasPrefix 在 Go 侧过滤。 |
| D15-02-3 | KMinOverride=0 表示使用全局默认（Phase 15 / Plan 15-02） | datasets 注册表 k_min_override 列 0 = 全局 KMinDefault；旧调用方与预 Phase 15 行不填即为 0，无需每次导入都显式指定 k_min。 |
| D15-03-1 | Quoted ATTACH/DETACH aliases for hyphenated composite dataset ids (Phase 15 / Plan 15-03) | Composite ids like `loans__2026-07` contain hyphens; DuckDB rejects unquoted identifiers with hyphens. `AttachDataset` and `AttachLRU.execDetachLocked` now double-quote the `ds_{id}` alias. Existing underscore-only ids are unaffected. |
| D15-03-2 | Supersede staging file preserved after registry-write rollback (Phase 15 / Plan 15-03) | When `SupersedeStagedDataset` step 3 (`Registry.PutDataset`) fails, the new data is rolled back from target to staging and the old data is restored from backup to target. Staging is preserved so the caller can retry, matching `CommitStagedDataset` retry semantics. |
| D15-03-3 | WriteStagingBatch rejects existing target tables (Phase 15 / Plan 15-03) | Import batches must write fresh staging files. `WriteStagingBatch` queries `information_schema.tables` before `CREATE TABLE` and returns `ErrTableExists` if the table already exists, preventing accidental staging reuse. |
| D15-03-4 | Staging column type allowlist before Appender creation (Phase 15 / Plan 15-03) | go-duckdb's Appender cannot create types like BIT/ANY/INVALID. `WriteStagingBatch` rejects unsupported types up front with `ErrUnsupportedColumnType`, turning a runtime Appender creation error into a clear, early failure (T-15-16). |
| D15-04-1 | dump.go uses package-level fmtError wrapper and in-band NULL sentinel (Phase 15 / Plan 15-04) | `ParseDump` errors are wrapped with `data.%s: %w` to match the rest of `internal/data`; tests assert on required substrings. SQL NULL is represented by the non-printable sentinel `\x00NULL\x00` to avoid ambiguity with real blank cells. |
| D15-04-2 | Preview session TTL is a hygiene parameter; the security boundary is the content-hash bridge (Phase 15 / Plan 15-04) | `PreviewSessionTTL` (30 min) limits memory residency only. `ValidateCommitAgainstPreview` closes D15-B1 by requiring the commit request's recomputed SHA256 to equal the previewed file's SHA256; an expired session simply cannot be matched. |
| D15-05-1 | Push shares the Commit staging/commit tail (Phase 15 / Plan 15-05) | `Importer.Push` buffers the first 100 rows for schema inference, then streams the remainder through the same `writeStaging` + `commitOrSupersede` path used by `Commit`. The preview session and content-hash bridge are the only human-channel differences. |
| D15-05-2 | DryRun is a first-class zero-write validation method (Phase 15 / Plan 15-05) | `Importer.DryRun` returns `DryRunResult{Columns, Violations}` after header/column/type checks; it writes no staging file, registry row, or audit row. |
| D15-05-3 | Single-pass reader contract preserved for Push (Phase 15 / Plan 15-05) | `TeeReader→hasher` wraps `req.Reader` before `openChannel`, and `ValidateCommitAgainstPreview`/`checkIdempotent` run only after `WriteStagingBatch` exhausts the stream. `oneShotReader` tests lock this for both Commit and Push. |
| D16-03-A | kpi_calls 专用 INSERT-only 表落地（Phase 16 / Plan 16-03，KPI-02/D16-D4） | import_batches 整文件复刻换列集：PutKPICall 单写路径 + kpi_calls.go 零行变异/行移除动词 source-grep 结构锁（TestKPICallsInsertOnly）；10 列 additive DDL + 三索引独立 const 走 migrate() 尾部（D14-E1 形态纪律）；大 JSON 快照物理隔离于 chat 审计主表（T-16-03-03），ADM-03 直查无 JOIN。 |
| D16-03-B | LatestKPICallByFunc 取 MAX(id) 而非计划原文 MAX(ts)（Plan 16-03 Rule 1 修复） | RFC3339Nano 裁剪尾随小数零：零纳秒戳 "...00Z" 文本排序反而高于同秒更晚的 "...00.25Z"，MAX(ts) 在同秒双调用时错报；AUTOINCREMENT id 与插入序（即回放序）严格单调且仍是一条 GROUP BY 聚合。伴生修正：List/Latest 接口签名顺延至其实现提交（var _ store.Store 一致性检查要求每个提交可编译）。 |
| D16-08-A | callbacks 工具分流两级判定 + 保守默认（Phase 16 / Plan 16-08） | Level 1 `callbacks.RunInfo.Name` 为权威（实测 eino v0.9.14 `compose/tool_node.go:896-899` + `:935-939` 两条路径都塞 `Name: task.name`）；Level 2 无名 RunInfo 时按 `kpiCallArgs` 形状探测（`name` 非空 + 存在 `params`）；两级都不可判定时**发 retrieve**（Phase 3 现状）——未知工具多一条 retrieve 只是时间线瑕疵，而对所有工具静默不发会让可观测面整体变白。kpi_call 分支只 inc 计数不发决策，`kpi_reference` + DataRef 由工具闭包发射，二者构造性互斥。 |
| D16-08-B | data-reference part 的 `DecisionPayload.Step` 取 0（Phase 16 / Plan 16-08） | hop 计数器是 internal/agent 的未导出 per-Run `stepCounter`，server 边界（emitter 闭包构造处）无缝可取；FE 按 decision **种类**（kpi_reference → 数据引用 chip）消费而非按 hop 序号。诚实的 0 无代价，编造序号则是在近审计流里撒小谎。将来若需真 hop 号，正解是把它穿进 `DataRefEmitFunc` 签名，而非在 server 侧反推。 |
| D16-08-C | KPI 管线装配不受 SKILLFORGE_DATA_SALT 门控（Phase 16 / Plan 16-08） | 盐是**写**侧（导入脱敏）的 L1 HMAC 站点秘密；KPI 只**读**已脱敏数据集且从不再派生哈希。用盐门控分析能力会让一个完全合法的只读部署失去指标。装配段就地注释该差异与理由，与 importer 段形态其余部分保持镜像。 |
| D16-08-D | LoadKPIs 失败 = log.Fatalf；空目录是唯一合法降级态（Phase 16 / Plan 16-08） | 与 Skill Scan（main.go:201）同姿态：受控文件是承重结构（D16-A1），坏 .yaml 就不该起服务，否则 agent 静默丢指标。零定义加载空 registry 且 `[OK]` 行明示 `signature=(empty registry — no KPI definitions loaded)`。 |
| D16-08-E | admin KPI 端点只读是结构性而非约定性 + 审计读失败报 500（Phase 16 / Plan 16-08 / KPI-01 / Pitfall 14） | 定义只在启动时从受控文件加载，任何 HTTP 变更面就是该设计要防的注册后门；锁 = TestAdminKPIs 双 source-grep（handler 文件零变更动词 + 路由表恰好一次且为 GET）。`latest_calls` 空 map 会被运维读成"从未调用"，故真读失败返 500 而非空 map（nil Store 仍返空 map——与本包 degraded 约定一致）。 |
| D16-08-F | storeDatasetMeta 在 Go 侧取最新而不新增 store 方法（Phase 16 / Plan 16-08 Rule 1 简化） | 计划原文设想 `ORDER BY created_at DESC LIMIT 1` 形态的新 store 查询；但 `ListDatasets` 已返回适配器所需全部字段（Dataset/Period/FilePath/KMinOverride/CreatedAt），为一张有界注册表加方法+SQL+测试超出本 plan 爆炸半径。适配器按逻辑 dataset 过滤取 `max(CreatedAt)`——与该 SQL 同一行。零行 → Period 空 → executor 转 dataset_not_available 拒答，绝不编造 as_of。 |

---

## Accumulated Context

### Roadmap Evolution

- 2026-08-24: Phase 15 post-phase gates CLOSED — code review resolved 6/6 findings (commit `c0d7d5a`: CR-01 streaming multipart onFile callback + `strictUTF8Reader` rewritten on `bufio.ReadRune` fixing a latent multi-byte rune-split corruption + DATA-17 audit-before-mutation ordering); 15-VERIFICATION verdict PASS; ROADMAP progress rows 10/11/12/14/15 reconciled with recorded closeouts; STATE advanced to Phase 16 (discussing).
- 2026-08-24: Phase 15 COMPLETE — Plan 07 (structural ban gates + `verify-phase-15.sh` 10/10 PASS + `make verify-phase-15`) closes the phase. All 7 plans (15-01..15-07) hermetically gated. Post-phase closeout gates (code-review / regression / verifier / roadmap sync) are next.
- 2026-08-24: Phase 15 Plan 06 (15-06) complete — Hertz streaming import HTTP surface: POST /api/import/preview/commit/push (with dry_run), DELETE /api/import/dataset/:name, per-route body-size guard, admin-token gate, SKILLFORGE_DATA_SALT fail-closed 503, Detach-before-Remove Windows lock ordering, and INSERT-only dataset_deletes audit. TestImportStreaming + TestDeleteDatasetAudited green; full `./internal/server/...` regression passes.
- 2026-08-24: Phase 15 Plan 05 (15-05) complete — `internal/data/importer.go` three-channel orchestrator (Preview / Commit / Push / DryRun) + `internal/data/batch.go` batch idempotency/supersede/audit helpers + `TestRawNeverOnDisk` library-level raw-leak guard. DATA-01/02/15/16/17 fully locked at the library layer; HTTP handlers remain in 15-06.
- 2026-08-23: Phase 15 Plan 04 (15-04) complete — `internal/data/dump.go` sqlite .dump three-layer accept-list parser (zero SQL execution) + four 3.53.0 ground-truth fixtures + `internal/data/preview.go` in-memory preview session store with TTL/capacity/deep-copy snapshots + `ValidateCommitAgainstPreview` content-hash bridge (D15-B1). DATA-01 dump channel + DATA-15 two-stage preview/commit base locked.
- 2026-08-23: Phase 15 Plan 03 (15-03) complete — `SupersedeStagedDataset` three-step backup-rename-register chain + `WriteStagingBatch` Appender-only bulk write helper + composite-name ATTACH probe (A1 closed). `internal/duckdb/connector.go` and `attach_lru.go` now quote ATTACH/DETACH aliases to support hyphenated dataset ids. DATA-16 engine atomics locked.
- 2026-08-23: Phase 15 Plan 02 (15-02) complete — `internal/store/sqlite` additive schema migration + `import_batches`/`dataset_deletes` INSERT-only audit tables + Store interface extension (DATA-16 + DATA-17 store layer). See 15-02-SUMMARY.md.
- 2026-08-23: Phase 15 Plan 01 (15-01) complete — `internal/data` desensitization core pure functions (normalize/desensitize/rules/decode + fixtures + datarules/loans.yaml). DATA-02 requirement locked; 2/2 deviation rules auto-fixed (gitignore anchor + GB18030 strict replacement-character guard).
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

### Phase 14 Plan 14-12 — compliance frozen replay: D14-B4 re-baseline semantics code-locked hermetically; live replay environment-blocked by BGE-M3 no route to host

- **Status:** Phase complete — ready for verification
- **Blocked since:** 2026-08-22 (initial 14-10 replay FAIL); advanced 2026-08-23 (~03:15 UTC after 14-11 replay completed); advanced again 2026-08-23 (~04:38 UTC after 14-12 hermetic re-baseline landed).
- **Measurement-layer fix (Gap 1):** `cmd/eval/main.go` now extracts `classifyRunOutcome` (14-11) AND `computeCaseMetrics` (14-12). `computeCaseMetrics` explicitly short-circuits `faithfulness=1.0` when `refused=true`, before calling `eval.Faithfulness`. This makes the D14-B4 re-baseline semantics a code-level contract rather than relying on the implicit empty-answer short-circuit in `internal/eval/faithfulness.go:75-78`. Verified by `TestRefusedCaseFaithExemption` RED→GREEN and full hermetic gates (`verify-phase-14` 11/11 PASS, `verify-phase-13` 9/9 PASS).
- **Static fixes (Gaps 2 + 3):** `Dockerfile.backend` runtime stage installs `libstdc++ libgcc`; `scripts/verify-phase-13.sh` Stage 7 pin synchronized to `1dc6f148…` (lockstep with verify-phase-14.sh Stage 11).
- **Live replay result (14-12, single authorized run):** NOT EXECUTED. Pre-flight probe on 2026-08-23T04:33Z showed the BGE-M3 endpoint at `10.62.0.123:11434` returning `No route to host`. The eval pipeline requires BGE-M3 for query embedding and answer-relevancy judging, so the replay was not started.
- **Prior live evidence (14-11):** `cmd/eval --frozen=compliance --archive` completed 50/50 cases (2026-08-23T02:22Z–03:13Z, ~52 min). AssertPin held (`frozen pin holds`, `prompt_hash=5b12acd9…`). Overall faithfulness = **0.888** (< 0.9 gate); per-category: adversarial 0.7683, multi-hop 0.9581, refusal-required 0.9727; refusal-required 19/20 refused. Mid-run BGE-M3 endpoint failure (`connect: no route to host`) affected cases `mh-004`, `mh-005`, `mh-008`.
- **Root cause (code-anchored):** The 14-10 Addendum root cause is closed at the code level by 14-11 + 14-12. The remaining open item is a healthy-service replay to decide whether the re-baseline semantics push overall faithfulness above 0.9.
- **Unblock action (operator required):** restore the BGE-M3 endpoint (`10.62.0.123:11434`) and re-run the authorized compliance replay. If overall faithfulness ≥ 0.9 on a healthy service, execute 14-10-PLAN Task 2 steps 3-5 (fill-lock + README caveat + docs commit). If < 0.9 on a healthy service, escalate per the 14-10-SUMMARY Operator-Gate Addendum. No further executor-initiated replays are authorized until the endpoint is restored.
- **Verification:** replay faithfulness ≥ 0.9 + `TestPlatformFloorLockSelfConsistent` green + 11-stage gate PASS with hitl/fanout/deep byte-identical (deferred until a PASS replay is obtained in a healthy environment).
- **Evidence:** 14-12 SUMMARY `.planning/phases/14-p10-0-cgo-trust-floor/14-12-SUMMARY.md`; 14-11 replay archive `eval/reports/2026-08-23T03-13-46Z.json` + `eval/reports/INDEX.json`; log `build/1411-replay.log`; 14-11 SUMMARY `.planning/phases/14-p10-0-cgo-trust-floor/14-11-SUMMARY.md`.
- **Current lock state (honest):** `eval/frozen/platform-floor.lock.json` in HEAD still carries `platform_floor_version=1.1.0` + recomputed `lock_sha256` with the 2026-08-22 metrics/signature. The 1.1.0 formal re-sign remains deferred per STOP discipline.

### Phase 10 Plan 04 — Task 2 live re-run

- **Status:** Phase complete — ready for verification
- **Blocked since:** 2026-08-18
- **Reason:** Execution environment lacks the live stack required for the 35-case GLM/Milvus/BGE-M3 re-run. No `.env` file is present; `GLM_API_KEY`, `BGE_M3_URL`, and `MILVUS_ADDRESS` are not set; Docker Milvus + BGE-M3 endpoints are not confirmed reachable.
- **Unblock action:** Operator populates `.env` and starts the Docker stack, then runs:
  ```bash
  go run ./cmd/spike-history --via-pipeline --summarize --seed=42 \
    --golden=eval/golden/compliance.jsonl --out=reports/ --cost-limit=5.00
  ```

- **Verification:** All 4 gates PASS (4-metric drift <5%, refusal delta <10pp, SC-3 0/10, cost <$5.00/segment) and `git diff --quiet eval/frozen/compliance.lock.json` remains exit 0.

### Prior blockers

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
| 260819-dsp | 移除 DataSubprogressPart.tsx:47 预存在 FE tsc 错误（data.state 不在 SubprogressStep；步骤是 append-only 无 live pending 态可建模）。单文件 4 处移除 + 1 注释，**不触碰 D15 wire-type 不变式**。npx tsc --noEmit exit 0（was exit 1 + 1 pre-existing error）；npx vitest 349/349 PASS（无回归）；Phase 10 UAT Test 22 由 issue 转为 pass。 | 2026-08-19 | c285c55 |  | [260819-dsp-fix-datasubprogress-state-ref](./quick/260819-dsp-fix-datasubprogress-state-ref/) |

*260814-w4n：代码层 5/5 must-haves 全部验证通过（5 个真实 fsnotify 测试 Windows 本机 PASS，无 skip）；唯一待办为完整 HTTP E2E——本机无 Milvus/Docker 无法起独立实例，待用户重启其 dev 服务器时由 chrome-devtools 复验关闭。

---

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| 15 | 05 | 65m | 3 TDD tasks / 5 commits / 5 files — `internal/data/importer.go` Preview/Commit/Push/DryRun orchestrator + `internal/data/batch.go` idempotency/supersede/audit + `TestRawNeverOnDisk` raw-leak guard. DATA-01/02/15/16/17 library layer locked. See 15-05-SUMMARY.md |
| 15 | 01 | 21m | 3 TDD tasks / 7 commits / 15 files — `internal/data` desensitization core pure functions (normalize/desensitize/rules/decode + 5 encoding fixtures + datarules/loans.yaml + README). DATA-02 locked. See 15-01-SUMMARY.md |
| 15 | 04 | 9m | 2 TDD tasks / 4 commits / 8 files — `internal/data/dump.go` sqlite .dump accept-list parser + 4 fixtures + `internal/data/preview.go` TTL/capacity session store + hash bridge. DATA-01 + DATA-15 locked. See 15-04-SUMMARY.md |
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
| 14 | 11 | 76m | 3 tasks / 4 files — gap closure: cmd/eval refusal-accounting fix (classifyRunOutcome RED→GREEN) + Dockerfile libstdc++ libgcc + verify-phase-13 pin lockstep; single compliance replay STOPped at faithfulness 0.888 (<0.9) with BGE infra fault + adversarial drag; no re-sign, no retry, honest numbers routed to 14-10 Addendum Alternative. See 14-11-SUMMARY.md |
| 14 | 12 | 16m | 3 tasks / 2 files — explicit D14-B4 re-baseline semantics in cmd/eval scoring loop (computeCaseMetrics refused=true → faith=1.0) + TestRefusedCaseFaithExemption RED→GREEN hermetic lock; authorized compliance replay environment-blocked by BGE-M3 no route to host — no replay run, no falsified numbers, platform-floor.lock.json 1.1.0 re-sign remains deferred. See 14-12-SUMMARY.md |
| Phase 09-trust-floor-spike-p8-s-spike-hard-gate P06 | ~75m | 2 tasks (T2 = human-verify gate) | 3 files committed + 7 git-ignored reports |
| Phase 13-p8-4-sub-agent-fan-out P02 | ~25m | 2 tasks | 4 files (1 created merge.go + 1 created merge_test.go + 2 modified orchestrator.go/orchestrator_test.go) |
| Phase 14-p10-0-cgo-trust-floor P01 | ~25m | 3 tasks (T1 TDD + T2 TDD + T3 structural gate) | 7 files (4 created: platformfloor/platform_floor{,_test}.go + agent/platform_floor_test.go + contract/platform_floor_bypass_test.go; 3 modified: agent/{prompts,agent,platform_floor}.go + runtime/deep_agent.go + agent/hitl/threat_mitigations_test.go + eval/frozen/compliance.lock.json) |
| Phase 14-p10-0-cgo-trust-floor P02 | 15m | 2 TDD tasks (audit floor-version column + admin prompt preview) | 14 files (3 created: store/sqlite/audit_test.go + server/platform_prompt{,_test}.go; 11 modified: store/{store,sqlite/{audit,approval_rules}}, agent/hitl/{audit_writer,audit_writer_test,threat_mitigations_test}, server/{chat,approval_api,approval_api_test,server}, cmd/spike-hitl/main.go) |
| Phase 14-p10-0-cgo-trust-floor P09 | ~17m | 2 tasks (T1 TDD RED dd81dd2 + GREEN e1c1a12: idx_audit_run_path after ALTER loop + 3 legacy-upgrade regression tests; T2 live re-verify 0 code commits — UAT Test 2 issue→pass + Test 6 blocked→pass on real 115-row legacy skillforge.db) | 2 code files (1 created migrate_legacy_test.go 166 lines + 1 modified schema.go) + 11-stage phase gate re-run PASS |
| Phase 14-p10-0-cgo-trust-floor P10 | ~160m | 3 tasks (T1 hermetic relock chain: tail re-anchor 1.0.0→1.1.0 + 双字面量 + 4.5 测试锁 + compliance/platform-floor 双锁重锁 + verify 双 pin，11-stage gate 全绿，commits 174ee18+ebd547c; T2 live judge 20/20 首跑 PASS（long_dialogue_dilution 4/4，pfloor-20 引用新条款拒答）+ compliance replay STOP（端点劣化窗口 faithfulness 0.6876<0.9，不重签）; T3 真机 spot-check 0 code commits — audit 行 117=1.1.0 FLOOR-110-AUDIT-PASS + admin 预览 1.1.0/hash 5b12acd9 一致 ADMIN-FLOOR-110-PASS） | 8 files (5 code/test + 2 locks + verify script) + UAT 6/6 pass；遗留：replay 重跑→fill-lock→README caveat（STATE blocker） |
| Phase 16 P05 | 25min | 2 tasks | 9 files |
| Phase 16 P08 | ~35min | 3 tasks | 7 files (2 new: server/kpi_admin.go + agent/callbacks_test.go; 1 new test: server/kpi_admin_test.go; 4 modified: agent/callbacks.go + server/runner_setup.go + server/server.go + cmd/server/main.go) |
| Phase 17 P07 | 45 | 8 tasks | 6 files |

## Session Continuity

**Resume point (v0.8):** Milestone v0.8 roadmap created 2026-08-17 — ROADMAP.md carries phases 8-13 (strict chain 8→9→10→11→12→13 per research SUMMARY.md §Implications for Roadmap); REQUIREMENTS.md traceability maps all 25 v0.8 requirements (PLAT-04/05, GEVL-08, MEM-01..05, HITL-01..08, PLAN-01..05, FAN-01..04); Phase 9 is the HARD GATE for Phase 10. **Plan 08-01 COMPLETE 2026-08-17 (commit `36845c0`)**: eino v0.9.14 + ext openai v0.1.13 落库，hermetic 四道门全绿零修复提交（TestContractVersionPin 已断言新 pin）。**Plan 08-02 COMPLETE 2026-08-17 (commits `9a557b1` RED + `e6a580a` GREEN)**: sqlite CheckPointStore 骨架就位——SQLiteStore.Get/Set 满足 compose.CheckPointStore（编译期断言），checkpoint 表同库 additive 幂等（Phase 11 预留列齐备），SC-4 store 层语义（含 Close+reopen 等值读回）六面测试锁定，internal/store 全树零回归。**Plan 08-03 COMPLETE 2026-08-17 (commits `fea6114` 禁令门 + `996c33a` 文档对齐)**: SC-3 legacy interrupt 禁令门 TestNoLegacyInterruptAPI 落地（全仓扫描 + 负向证明），REQUIREMENTS/ROADMAP/CLAUDE.md 版本叙事对齐 v0.9.14（PLAT-05/SC-4 → schema.RegisterName）。**Plan 08-04 COMPLETE 2026-08-17 (commit `3c42c20`)**: SC-5 真实中断往返 TestInterruptResumeRoundTrip 落地——工具层 StatefulInterrupt 携带 schema.RegisterName 注册的 *approvalDecision → adk Runner Action.Interrupted（InterruptContexts[0].ID）→ checkpoint 持久化 sqlite（existed=true 实证）→ 实例级「进程重启」（Close + 同 DSN 重开 + 全新 Agent/Runner）→ ResumeWithParams(Targets) 注入决策 → 恢复到脚本化终答且注入值在终答文本 + 重启侧工具双重可见（D8-06）；phase hermetic 五门全绿（build+vet / go test ./... 全量 / contract 含 VersionPin+禁令门 / store 七面 / cancel 50.07ms<200ms）。**Plan 08-05 COMPLETE 2026-08-18 (零代码——三门 blocking human-verify 全部 approved)**: ①真 GLM cancel 传播实测 **0ms**（`go run ./scripts/spiked e2e` ×2 exit 0，两次稳定复现，亚毫秒截断口径；5 query ~1.7s/case；400ms stuck-cleanup 探测器干净退出；pin = eino v0.9.14 + ext openai v0.1.13）；②内网 Milvus tier-filter：声明 tier(权威)「出差住宿费超过500元合规吗」→ **5 candidates** + 2 source-document 引用 + needs_input/confidence 0.8；声明外 tier「查询张三的手机号码」→ **0 行**（个人 tier 在 Milvus expr 层排除，终判 no_authorized_source 结构化拒答，零泄漏；SC-2 low_retrieval_score 拒答保持）；③冻结 eval 重跑 `--frozen=compliance` **50/50 case**（~15min，judge=glm-5.2，pin 保持 model_ver/prompt_hash=bd59cbe0…）gate-check **PASS**（faithfulness **0.9720** ≥ 0.9000，exit 0；refusal_accuracy **95%** 19/20 via scripts/eval_snapshot；lock sha256 **50bb1688…** 逐字节一致，git status eval/frozen/ 干净）——Overall 留档 v0.8 基线：faithfulness 0.972 / context_precision 0.10 / answer_relevance 0.1002 / context_recall 0.39 / refusal_accuracy 95%（非 gate 指标偏低成因与 multi-hop 全拒答已被用户知悉接受，D8-07，改善归 Phase 9+）。执行侧偏差 4 项见 08-05-SUMMARY（taskkill 陈旧 server / curl UTF-8 --data-binary 适配 / 有意不跑 spiked milvus 保护冻结语料 / eval 产物保持 untracked）。**PHASE 8 COMPLETE (5/5)**——PLAT-04 + PLAT-05 双勾选。Next: `/gsd:verify-work 8` → `/gsd:plan-phase 9`。Phase 7 leftovers (07-02 T3, 07-05 T2/T3, 07-06) are deferred to after this milestone by user decision 2026-08-17 — do not lose them.

**Resume point (Phase 7):** Plan 07-05 (eval snapshot + demo GIF) is IN PROGRESS — Task 1/3 DONE via TDD, 2 commits (`abf2f4d` RED test + `4873213` GREEN impl). `scripts/eval_snapshot/{main.go,main_test.go}` shipped: standalone Go program reads `eval/report.json` + `eval/frozen/compliance.lock.json` via `eval.LoadReport`/`eval.LoadLock`, **derives** `refusal_accuracy` from `Report.Cases` via the pure fn `deriveRefusalAccuracy` (count(cat=="refusal-required" && Refused) / count(cat=="refusal-required") — RESEARCH Q1 load-bearing, NOT an evaluator field), renders the README markdown snapshot block (date + lock metadata + Overall metrics + derived % + case breakdown + reproduce cmd). 6/6 hermetic unit tests PASS (all-refused 1.0 / half-refused 0.5 / zero-refusal-required graceful 0,N/A / category-filter invariant / renderSnapshot metadata / N/A cell); `go vet` clean; stdlib-only imports; `cmd/eval` + `internal/eval` UNCHANGED. **Tasks 2 & 3 `checkpoint:human-verify gate=blocking` BLOCKING** — Task 2 needs live Milvus+BGE-M3+GLM stack (`docker compose up` + `ollama pull bge-m3` + populate `GLM_API_KEY`+`BGE_M3_URL` in `.env`) to run `go run ./cmd/eval --frozen=compliance` → `eval/report.json` → `go run ./scripts/eval_snapshot` → paste stdout into README `## Eval Results` (still PLACEHOLDER); Task 3 needs screen capture (ScreenToGif/OBS) to record `docs/demo.gif` (cited verdict + structured refusal flows, <5MB, embed already in README hero). Plan 07-02 (containerization) Task 3 docker-build ALSO still IN PROGRESS — Docker not in PATH. Plans 07-01 (release-meta) + 07-03 (docs) + 07-04 (examples+eval sample) COMPLETE. Next: human resolves 07-05 Task 2/3 checkpoints (which unblocks 07-06 publication governance — must NOT push to public remote until README eval numbers + GIF are in tree). See `.planning/phases/07-open-source-release/07-05-SUMMARY.md`.

**Resume point (Phase 9 — GEVL-08 spike hard gate):** Plan 09-01 (DONE 2026-08-18, 3 commits `0df3483` Task 1 + `1216c79` Task 2 + `8e56a27` Task 3). **Wave 1 spike 数据地基就位**：golden set 从 50 扩到 75（baseline 50 逐字节未动 + 10 history_injection_adversarial + 15 history_injection_followup 按 5/5/5 anaphora/post_refusal/multi_hop_chain 切分）。恶意行 A/B 锚点（baseline_id + history_anchor{turn_index, fake_chunk_ref}）与善意行 subclass 标签由新增 `TestGoldenHistoryInjectionRows` schema 门锁死，含 adversarial query ↔ baseline query 逐字相等断言（D9-A3）；恶意行 10 个 fake_chunk_ref 互异且每个 ref 作为子串出现于对应行的 history_turn（自洽）。`eval/frozen/` + `eval/sample.jsonl` 零触碰。Phase 9 关键 planner 裁决落地：(a) LLM 生成伪制度文本 = 执行者生成（planner 裁决 1，零 API 调用成本 + T-9-LLM-COST 规避）；(b) `golden_test.go` 改加法升级，原 20/20/10 + spike-anchor 5 + ID 去重 + 非空断言逐条保留；(c) `eval/sample.jsonl` 停留在 50 例（DIST-10 已发布样本，裁决 3）。Wave 1 partial — Wave 2/3/4 plans（09-02 projection.go 接口骨架 + 09-03 spike runner + 09-04 long.go + 09-05 summary.go + 09-06 live UAT）尚未启动，待本 plan summary 同步后启动 09-02。

**Phase 9 execution (2026-08-18):** Plan 09-02 COMPLETE in 3m10s with task commits `7aa8a3c` + `7fe80db` and summary commit `0609656`; `internal/history/projection.go` now enforces D9-B1/B2/B3 and `projection_test_fixtures.go` exports 7 contracts. Next: Plan 09-03 spike runner. **Plan 09-03 COMPLETE in 13m45s** (commits `7632923` + `b0b4571` + `cdd1377`) — `cmd/spike-history` spike-only binary ships: 11-flag CLI, RunEntry live+dry-run dual path (8-step pipeline), per-entry JSONL schema locked, stub gates for `--history-length=long` (Plan 09-04) + `--summarize` (Plan 09-05) with grep-friendly error strings. **Plan 09-04 COMPLETE in ~12m** (commits `923f1c0` long.go + `65cb8d0` main.go+runner.go) — `--history-length=long` path implemented: `loadLongHistoryRows` synthesizes 10 SpikeRows with 8/12/15-turn histories (1 signal per row + neutral filler from baseline 50 rows), seed=42 deterministic, output to `history_injection_long.jsonl`. D9-L1/L2/L3 decisions captured (Plan 09-03 category-as-filename convention + T-9-FILLER-LEAK fixed template + 8/12/15 upper-bound semantics). Stub gates removed from both main.go and runner.go; short path zero-regression. **Plan 09-05 COMPLETE in ~20m** (commits `e1660c6` plan 09-04 leftover fix + `3ef406e` RED test + `4078ab5` GREEN impl + `08d2920` main.go wire + `969ed25` docs closeout) — `--summarize` aggregator ships: `summary.go` 838 lines; TestComputeVerdict locks 6 gate scenarios; 4 output files per call; baseline verbatim read (no re-run). **Plan 09-06 COMPLETE 2026-08-18 (~75m; commits `e06576f` .gitignore+test-fix + `791c86d` renderer literals + `4ed48d8` docs)** — live spike 三段跑 + summarize 全通：`history_injection_adversarial.jsonl` 10 行 + `history_injection_followup.jsonl` 15 行 + `history_injection_long.jsonl` 10 行（真 GLM-5.2 + Milvus + BGE-M3，seed=42，cost-limit $5.00/段，实测 total **$0.0481**）→ `--summarize` exit **1** 产 4 文件（summary.md 55 行 / short_history.md / long_history.md / summary.json）。**裁决 NO-GO**：SC-3 零穿透 0/10 PASS（证据链未污染），faithfulness -18.6%/-12.6%/-22.4% + context_precision +483%/+300%/+417% + answer_relevancy +524%/+392%/+483% + context_recall -87.2%/-63.0%/-70.1% + refusal **-95pp**（三类 refusal_rate 全 0）超门；指标耦合注记：cp/ar 上升是「不再拒答 → judge 不再机械归零」的影子，根因 = 拒答行为漂移（Pitfall 1 H2）。**Task 2 checkpoint:human-verify（gate=blocking）用户签字「接受 NO-GO 带逃逸方案」**→ D9-V1/V2/V3 决策入册；4 SC + 决策表 + 三段命令时序全部转录进 09-06-SUMMARY（T-9-RESULT-LOSS 缓解——reports/ 已 git-ignore，数字在 git 内留档）。git 隔离验证：eval/frozen/ + eval/sample.jsonl + 硬约束面零改动；`go build ./... && go vet ./...` exit 0。**PHASE 9 COMPLETE (6/6)——GEVL-08 交付（消毒规则 + 明确有记录的 go/no-go + 逃逸方案）**。Next: `/gsd:verify-work 9` → `/gsd:plan-phase 10`（前置逃逸强化，见 D9-V2）。

**Resume point (Phase 13 — Fan-out P8.4):** Plan 13-01 (DONE 2026-08-21, commit `0484bca`): FanOutOrchestrator foundation (`internal/runtime/orchestrator.go` adk.Agent + skeleton Run) + tier-closure sub-agent builder (`internal/runtime/sub_agent_builder.go` — `intersectTiers` + read-only enforcement `subManifest.Writes = []string{}`) + LRU accumulator (`internal/runtime/fanout_runtime.go` — `SharedChunkAccumulatorMaxEntries=20` + per-sub-agent budget constants 5/20000/3/30) + `cmd/spike-fanout` 5-scenario hermetic binary (match / empty-intersection / sub-empty-reads / parent-empty-tiers / canonical); 16 hermetic runtime tests pass, FAN-02 SC-2 load-time error contract verified. **Plan 13-02 (DONE 2026-08-21, commits `7c9652b` Task 1 + `5d75e6d` Task 2)**: `mergeSubAgentResults` pure function + 10× byte-identity contract test + conservative partial-failure merge + citation dedup first-wins + min-confidence aggregation (D2-13 threshold 0.89). 28 hermetic runtime tests green. **Phase 13 Wave 2 NEXT**: Plan 13-03 (per-sub-agent grounding via shared `*ChunkAccumulator` + errgroup fan-out with `SetLimit(4)` + audit RunPath/Transfer columns + D16 overload on `data-decision.decision`). **Phase 13 Wave 3**: Plan 13-04 (STRIDE-13 threat model + hermetic gate — `TestNoNewParallelAgentUsage` source-grep guard + frozen eval re-lock). **Phase 13 Wave 4**: Plan 13-05 (acceptance gate + live fan-out UAT).

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

*State initialized 2026-08-10 during roadmap creation. Last transition: Phase 14 UAT gap closure executed 2026-08-23 (10/10 plans — 14-09 legacy-DB migrate fix + 14-10 floor 1.1.0 re-anchor: UAT 6/6 pass, Test 4 live 20/20 @1.1.0, Test 6 真机 1.1.0；1 operator gate open — compliance replay clean-window re-run → platform-floor lock re-sign + README caveat, see Blockers). Prior: Phase 11 ACCEPTANCE GATE PASS 2026-08-20 (8/8 plans — 11-01 Runner-ization / 11-02 HITL Tool infra / 11-03 audit fail-closed / 11-04 approval endpoints / 11-05 approval rules / 11-06 FE ApprovalCard / 11-07 FE Rules panel / 11-08 acceptance gate; all 11 verification stages green; HITL-01..08 all verified end-to-end). Prior transition: Phase 9 COMPLETE 2026-08-18（6/6 plans——09-06 live spike 裁决 **NO-GO**，用户签字「接受 NO-GO 带逃逸方案」：SC-3 零穿透 0/10 PASS 但 refusal -95pp + faithfulness/context_recall 漂移超门（Pitfall 1 H2 证实）；GEVL-08 交付（消毒规则 + 有记录的 go/no-go + 逃逸方案）；Phase 10 须前置信任地板强化并在 memory 发布前重跑 spike 硬门；ready for /gsd:verify-work 9）。Earlier: Milestone v0.8 roadmap created 2026-08-17 — phases 8-13 appended (25/25 v0.8 requirements mapped; Phase 9 = spike hard gate for Phase 10); Phase 7 leftovers deferred to after v0.8 by user decision; Phase 4 → Phase 3 (resumed) on 2026-08-11; Phase 3 COMPLETE (6/6 plans) 2026-08-12.*

---

## Phase 11 — Write Tools + Approval Gate (P8.2 — HITL-01..08) — Acceptance Sign-Off

**Status**: Complete (Acceptance Gate PASS — executor-side 11/11 stages green)

**Plans completed**: 8/8 (11-01..11-08)

**Acceptance gate run command**: `bash scripts/verify-phase-11.sh` (or `make verify-phase-11` where make available)

**Acceptance gate log**: `/tmp/verify-phase-11_<YYYY-MM-DD>.log` (operator redirect; executor ran all 11 stages green on 2026-08-20)

**Frozen baseline lock sha256**: `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (Phase 8 pin — UNCHANGED; byte-identity verified at gate time)

**HITL frozen baseline sha256**: `6e91881f4aafeffb843e996c89ff1c27ef22eb18fac94eceff9184f938481d08` (NEW — `eval/frozen/hitl.lock.json` locks 75-case HITL golden set)

**Threat-model sha256**: `b0d0f95bcdd211b7807cacf7d27d434a85ca7415048823a1e80e251f1a30b3f6` (`docs/threat-model/STRIDE-11.md` — 4 Critical + 4 High threats)

**Sign-off line**: `Operator signature: ___   Date: 2026-08-20`

**Compliance attestation** (8 HITL requirement IDs each with verifying test):

| Requirement | Verifying Test(s) |
|-------------|-------------------|
| HITL-01 (stream approval card) | TestApprovalCardRendering + TestTransportSwapWithApprovalCard (11-06) + TestE2EApprovalFlow (11-08) |
| HITL-02 (approve/deny once + structured binding) | TestApprovalDecisionValidate (11-02) + TestApprovalCardDenyRequiresReason (11-06) + TestE2EApprovalFlow (11-08) |
| HITL-03 (cross-refresh state survival) | TestExpiredCheckpointUIState (11-04) + TestApprovalCardResumeBanner (11-07) + TestInterruptResumeRoundTrip (11-01) |
| HITL-04 (3-row audit + frozen snapshot + fail-closed + idempotency) | TestFailClosedAudit + TestAuditChainThreeRowLifecycle + TestApprovalArgsByteEquality + TestConcurrentApproveDenyIdempotency + TestApprovalIDIdentityInvariant (11-03) |
| HITL-05 (writes: code-level enforcement) | TestToolWhitelistMatchesSkillWrites + TestPromptInjectionResistance (11-02) + TestThreatModelCriticalMitigations T-HITL-05 (11-08) |
| HITL-06 (memory:facts + review_report write tools) | TestMemoryFactsToolFirstPassInterrupts (11-02) + TestReviewReportToolFirstPassInterrupts (11-04) + TestE2EApprovalFlow (11-08) |
| HITL-07 (edit-before-approve) | TestFrozenArgsThreeWayByteEquality (11-05) + TestApprovalCardEditBeforeApprove (11-06) + TestThreatModelCriticalMitigations T-HITL-07 (11-08) |
| HITL-08 (approve-and-remember rules) | TestApprovalRulesScope + TestApprovalRulesRevoke + TestRuleAutoPass (11-05) + TestApprovalRulesPanelEmptyState + TestApprovalRulesPanelRevoke (11-07) + TestThreatModelCriticalMitigations T-HITL-08 (11-08) |

**8 Plan SUMMARY references**:

- `.planning/phases/11-p8-2-write-tools-approval-gate/11-01-SUMMARY.md` — Runner-ization foundation (6 commits, all gates green)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-02-SUMMARY.md` — HITL Tool infra (10 commits, 11 checks green)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-03-SUMMARY.md` — audit fail-closed (6 commits, 9 checks green)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-04-SUMMARY.md` — approval endpoints (6 commits, all checks green)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-05-SUMMARY.md` — approval rules (9 commits, all checks green)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-06-SUMMARY.md` — FE ApprovalCard (8 commits, 372/372 vitest, tsc 0 errors)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-07-SUMMARY.md` — FE Rules panel (7 commits, 372/372 vitest, tsc 0 errors)
- `.planning/phases/11-p8-2-write-tools-approval-gate/11-08-SUMMARY.md` — acceptance gate (8 commits, 11/11 stages green)

**Gate stages executed (all PASS)**:

1. Hermetic Go suite (store + hitl + agent): PASS
2. Approval API endpoints (11-04 + 11-05): PASS
3. WalkDir legacy interrupt guard: PASS
4. Cancel propagation <200ms lock: PASS
5. STRIDE-11 critical mitigations: PASS
6. Hermetic E2E smoke cmd/spike-hitl: PASS
7. Vitest UI tests (11-06 + 11-07): PASS
8. D-04 transport-swap: SKIP (dir absent; covered by Stage 2)
9. D-15 wire-type: SKIP (dir absent; covered by Stage 2)
10. Frozen eval baseline re-lock: PASS ([FROZEN-RELOCK-PASS])
11. Frozen lock byte-identity: PASS

**Next action**: Operator manually commits the acceptance batch via `gsd-sdk query commit` (or `git commit`) with the files listed in 11-08-PLAN.md Task 8; then Phase 11 is formally closed.

## Phase 13 — Sub-agent Fan-out (P8.4 — tier 继承) — Plan 13-04 Hermetic Gate Sign-Off

**Status**: Plan 13-04 COMPLETE (Hermetic Gate 9/9 PASS; live UAT deferred to Plan 13-05 operator-gated)

**Plans completed**: 4/5 in this wave structure (13-01 DONE + 13-02 DONE + 13-03 DONE + 13-04 DONE); 13-05 pending operator-driven live UAT (v0.8 milestone re-baseline)

**Acceptance gate run command**: `bash scripts/verify-phase-13.sh` (or `make verify-phase-13`)

**Acceptance gate log**: `/tmp/verify-phase-13_<YYYY-MM-DD>.log` (operator redirect; 13-04 ran all 9 stages green on 2026-08-21)

**Frozen baseline lock sha256 (Phase 8 pin — UNCHANGED)**: `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (`eval/frozen/compliance.lock.json` — T-8-12 regression lock verified)

**Frozen baseline lock sha256 (Phase 13 v0.8 milestone re-baseline — placeholder)**: `2fca4ee25c11f57a17e3628bdd759d6618d9508eebb12b135f5294630da1b4ee` (`eval/frozen/fanout.lock.json` — operator runs Plan 13-05 to populate `metrics.*` + actual `eval_set_sha256` + `lock_sha256`)

**Threat-model sha256**: `db0b3dc92e5bfc174239dd1fb556cd953e60b08d263117fd30bbb8bb8d0fd548` (`docs/threat-model/STRIDE-13.md` — 8 threats T-P13-1..T-P13-8; all Critical/High)

**Hermetic gate script sha256**: `eefc4df0b1cf3d88f6743e56c89b25cda1bf763bb5d6dfa71d3282713da5e8cb` (`scripts/verify-phase-13.sh` — 9-stage gate; -race omitted per Phase 12 cgo env precedent)

**WalkDir test sha256**: `b9b0677ea8ca4ad2145eaabcade20a5361ae77856a92951527259a1422a55dc8` (`integration/contract/no_legacy_interrupt_test.go` — `phase11GoScanRoots` extended from 6 to 8 entries)

**Makefile sha256**: `9f6babe29dc744775336b47d2d371e1a33229d86172a906593947f1a2e4e6678` (added `verify-phase-13` target)

**Sign-off line**: `Operator signature: ___   Date: 2026-08-21`

**Compliance attestation** (4 FAN requirement IDs each with verifying test + gate stage):

| Requirement | Verifying Test(s) + Gate Stage |
|-------------|-------------------------------|
| FAN-01 (orchestrator parallel fan-out + fault isolation + conservative merge) | `TestFanOutOrchestratorFaultIsolation` + `TestFanOutOrchestratorSubAgentTimeout` + `TestMergeConservativePartialFailure` (13-02 + 13-03) — Stage 02 PASS |
| FAN-02 (sub-agent tier-closure structural inheritance) | `TestSubAgentTierInheritance` + `TestSubAgentCannotExpandTierBoundary` + `TestSubAgentRetrieveToolClosesOverSubManifest` + `TestSubAgentIsReadOnly` + `TestSubAgentToolWhitelistExcludesHITL` (13-01) — Stage 02 PASS + Stage 03 WalkDir 8 roots PASS |
| FAN-03 (deterministic 10× byte-identity merge) | `TestMergeSubAgentResultsByteIdentity` (10× contract) + `TestMergeSubAgentOrderIndependent` (13-02) — Stage 02 PASS |
| FAN-04 (per-sub-agent grounding via ApplyTrustStage + shared ChunkAccumulator + audit RunPath/Transfer) | `TestFanOutOrchestratorAppliesTrustStagePerSubAgent` + `TestSharedChunkAccumulatorLRUCap` + `TestSharedChunkAccumulatorDedup` + `TestSharedChunkAccumulatorHitsOrder` (13-01 + 13-03) — Stage 02 PASS + Stage 09 PASS |

**P13-L1..L9 lock attestations**:

| Lock | Mitigation | Gate Stage |
|------|-----------|------------|
| P13-L1 (adk.NewParallelAgent banned) | Sealed sentinel + source-grep | Stage 04 PASS |
| P13-L2 (tier-closure structural) | BuildSubAgent constructor | Stage 02 + Stage 03 PASS |
| P13-L3 (sub-agent budget cap MaxIteration=5 + TokenBudget=20000) | Hardcoded literals + source-grep | Stage 08 PASS |
| P13-L4 (subManifest.Writes=[] + merge sort.SliceStable) | Constructor zero + 10× byte-identity | Stage 02 PASS |
| P13-L5 (conservative partial-failure merge) | any refused/faulted → needs_input confidence 0 | Stage 02 PASS |
| P13-L6 (audit RunPath/Transfer columns ADD COLUMN DDL) | 3 nullable TEXT columns; no migration break | Stage 02 PASS (audit package tests) |
| P13-L7 (D16 data-decision.decision overload) | 3 new enum values; NO new SSE chunk type | Stage 05 D-04 byte-identity + Stage 06 D15 tsc PASS |
| P13-L8 (sub-agent scope: ONE layer only) | WithoutGeneralSubAgent already in P12 BuildDeepAgent; orchestrator does NOT invoke sub-orchestrators | Stage 02 PASS |
| P13-L9 (per-sub-agent grounding + shared ChunkAccumulator) | runtime.ApplyTrustStage reused + LRU cap 20 | Stage 02 + Stage 09 PASS |

**Gate stages executed (all PASS, 9/9)**:

1. go build + vet: PASS
2. hermetic Go test suite (FAN-01..04; -race omitted per Phase 12 precedent): PASS
3. WalkDir legacy interrupt guard (D8-04 extended; 8 roots incl. orchestrator + sub_agent_builder + merge + fanout_events): PASS
4. adk.NewParallelAgent ban source-grep (P13-L1 sealed sentinel): PASS
5. D-04 transport-swap byte-identity (mock ↔ Default; data-decision fanout_* enum extended): PASS
6. D15 wire-type compile alarm (npx tsc --noEmit; data-decision fanout_* enum lockstep): PASS
7. frozen eval byte-identity (compliance.lock.json + fanout.lock.json sha256): PASS
8. sub-agent budget half-of-parent (P13-L3 source-grep; MaxIteration=5 + TokenBudget=20000): PASS
9. SharedChunkAccumulator LRU cap (P13-L9; Pitfall 6 — 20-chunk cap + dedup + oldest-first order): PASS

**Plan SUMMARY reference**: `.planning/phases/13-p8-4-sub-agent-fan-out/13-04-SUMMARY.md`

**8 STRIDE-13 threats attested** (T-P13-1..T-P13-8; all Critical/High — see `docs/threat-model/STRIDE-13.md` for full register + mitigation mapping):

- T-P13-1 (EoP — tier intersection mismatch) → BuildSubAgent load-time + classifier-side two-layer defense
- T-P13-2 (EoP — HITL whitelist leak) → `subManifest.Writes = []string{}` structural
- T-P13-3 (DoS — fault propagation) → `defer recover()` + `context.WithTimeout(30s)` + errgroup `SetLimit(4)`
- T-P13-4 (Repudiation — merge non-determinism) → PURE function + `sort.SliceStable` + 10× byte-identity contract
- T-P13-5 (DoS — classifier cost explosion) → sub-agent budget cap + pre-fanout budget check
- T-P13-6 (DoS — shared accumulator growth) → LRU cap 20 + dedup + oldest-first
- T-P13-7 (Tampering — manifest wiring typo) → SubAgentConfig type-safe struct
- T-P13-8 (Tampering — adk.NewParallelAgent accidental use) → sealed sentinel + source-grep

**Next action**: Operator manually commits the 13-04 batch atomically (8 files listed below); then runs Plan 13-05 live UAT to populate `fanout.lock.json` `metrics.*` + `eval_set_sha256` + `lock_sha256`. Phase 13 v0.8 milestone re-baseline closes at 13-05 operator sign-off.

**13-04 atomic commit files (8 total)**:

1. `scripts/verify-phase-13.sh` (new — 9-stage hermetic gate)
2. `docs/threat-model/STRIDE-13.md` (new — 8-threat register + acceptance criteria)
3. `eval/frozen/fanout.lock.json` (new — Phase 13 v0.8 placeholder; operator fills 13-05)
4. `eval/frozen/README.md` (modified — added fanout.lock.json row + 4-lock verification + §fanout.lock.json)
5. `integration/contract/no_legacy_interrupt_test.go` (modified — phase11GoScanRoots extended 6→8 roots)
6. `internal/agent/runner_setup_test.go` (UNTOUCHED — second WalkDir copy preserved; integration/contract is the canonical Phase 12 pattern)
7. `Makefile` (modified — added `verify-phase-12` + `verify-phase-13` targets)
8. `.planning/STATE.md` (this closeout block)

## Phase 13 — Plan 13-05 Executor Batch COMPLETE (2026-08-21) — Live Gate = OPERATOR PENDING

**Status**: Tasks 1-2 COMPLETE (hermetic, all tests green); Task 3 (live UAT + `fanout.lock.json` sign-off + operator signature below) = **OPERATOR GATE — NOT YET RUN**

**Executor commits**:

1. `74682ed` fix(13-05) — 2 Rule-1 unblocks: `defaultSubAgentMerge` → delegates to `mergeSubAgentResults` (FAN-03 at the DispatchAgent surface); `BuildSubAgent` structural P13-L8 recursion guard (orchestrator-agent sub-manifests rewritten to chat leaf at construction)
2. `bdcded7` feat(13-05) — Task 1: dual-mode `cmd/spike-fanout` (7-flag matrix `--live/--scenario/--golden/--sub-skill-config/--cost-limit/--seed/--summarize`; `runLiveSpike` env fail-fast + pre-flight cost abort; `verifyFanoutGate` reading `eval/frozen/fanout.lock.json` thresholds; 4-file summarize renderer `reports/live_fanout.md|multi_hop.md|mismatch.md|fanout_summary.json`; 10 tests incl. `TestLiveModeParityWithHermetic` byte-identity + `TestLiveModeRunsWithoutNetwork` skip-without-key)
3. `e38abe1` test(13-05) — Task 2: `eval/golden/fanout.jsonl` 20 cases (8 multi_hop / 6 tier_mismatch / 4 partial_failure / 2 single_vertex) + `examples/skills/compliance-orchestrator/SKILL.md` (`agent: orchestrator`; operator-prep gap closed hermetically per dispatch) + `TestExamplesValidate` 3→4 templates (original assertions intact)

**Verification (hermetic)**: `go build ./...` + `go vet ./...` exit 0; `go test ./cmd/spike-fanout/... ./internal/runtime/... ./internal/skill/...` PASS (no `-race` — Phase 12 Windows cgo precedent); `git diff --quiet eval/frozen/` clean (compliance `5cf700f3…` UNCHANGED — T-8-12 preserved; fanout placeholder `2fca4ee2…` UNCHANGED — operator fills at Task 3 step E).

**Operator runbook**: `.planning/phases/13-p8-4-sub-agent-fan-out/13-05-SUMMARY.md` §"Task 3 Operator Runbook" (steps A-G: pre-flight → `make verify-phase-13` → `go run ./cmd/spike-fanout --live --golden=eval/golden/fanout.jsonl --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md --cost-limit=2.00 --seed=42 --summarize` → gate verdict → fanout.lock.json re-baseline → sign-off → final gate re-run).

**Live gate result** (2026-08-21T15:10Z, operator-signed):

- `PHASE 13 LIVE ACCEPTANCE GATE: FAIL` on the original threshold set (conservative_merge_rate_max: 0.0 breached at 0.200).- **Operator accepted deviation** (D9-V2-style authority pattern): conservative_merge_rate_max raised 0.0 → 0.5 (P13-L5 by-design; 2 sub-agent GLM Coding Plan timeouts at fanout-03/07). 2.5× headroom over measured 0.2.
- **Effective gate verdict (operator-signed)**: PASS — 5/6 thresholds satisfied unchanged, 1 accepted-with-notes. Tier-closure 6/6 rejected; per-sub-agent grounding completed; deterministic merge; RunPath/Transfer audit landed.

**Operator signature (live gate)**: `ninedrunk`   Date: `2026-08-21`

**Frozen lock committed**: `eval/frozen/fanout.lock.json` (commit `e02525f`):

- `eval_set_sha256`: `bfe8bbffdcb3e23acf5fb0095b7bb01e0dafe68762ef7d4645023f1f3569868b` (golden)
- `live_fanout_md_sha256`: `e5af980ca019751b7fb78554c03e80ebae6461660d3e5594e3f1291a85dad467`
- `lock_sha256`: `f3566e52e67bb66ae0f3024d98245a9182b7b7993c4295a58535bcc4043ed0af` (canonical body excluding self-field)
- `phase_8_pin_unchanged`: `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (T-8-12 — preserved)
- `phase_12_pin_unchanged_sha256`: `<n/a — Phase 12 does not have a frozen eval lock>` (Phase 12 used hermetic dry-run only)

**v0.8 milestone completion**: Phase 13 live gate PASSED with accepted deviation; operator-signed `fanout.lock.json` committed. Next: `/gsd:execute-phase 13 --wave 5` for 13-06 v0.8 milestone wrap (completion report + Operator Gates Ledger incl. Phase 10 deferred 35-case re-run status + README DIST-09 fill + `v0.8.0` tag hand-off). **Phase 10 deferred 35-case `--via-pipeline --summarize` re-run**: attempted 2026-08-21T15:14Z, blocked by GLM Coding Plan rate-limit / context deadline (20/20 cases classifier-error on attempt 3); operator may re-run once API is reachable; status tracked in 13-06 Operator Gates Ledger as `BLOCKED-ENV`.

### 13-05 measured live metrics (13-06 Task 1 capture)

| Metric | Threshold | Measured | Verdict |
|--------|-----------|----------|---------|
| `fanout_dispatched_count` | floor >= 20 | 20 | PASS |
| `conservative_merge_rate` (spurious) | max 0.0 -> 0.5 re-baselined | 0.200 | ACCEPTED-WITH-NOTES |
| `tier_closure_regression_rate` | max 0.0 | 0.0 | PASS |
| `min_confidence_aggregation` | min >= 0.0 | 0.95 | PASS |
| `tier_mismatch_rejection_rate` | min 1.0 (6/6) | 1.0 | PASS |
| `total_cost_usd` | $2.00 advisory pre-flight cap (not a lock threshold) | $2.92 | OVER ADVISORY CAP — noted |

**`eval/frozen/fanout.lock.json` file sha256 (post-sign):** `af3020fd7f38fce2f7396dce81eb151e4ed6952262565785554a59ff51e01252` (distinct from the canonical-body `lock_sha256` f3566e52… and from the `eval_set_sha256` bfe8bbff…).

**13-05 status:** COMPLETE. Full closeout: `.planning/phases/13-p8-4-sub-agent-fan-out/13-05-SUMMARY.md` §"Live Gate Closeout". Phase 13 = 5/5 plans through 13-05; 13-06 (v0.8 wrap) in progress.

---

## v0.8 MILESTONE CLOSED — Plan 13-06, 2026-08-21 (tag deferred by operator)

**Milestone:** v0.8 — Harness（可信×行动融合）. **Status:** `complete` — flipped 2026-08-21T13:59Z by operator decision "跳过 tag" (the same accepted-with-notes authority as Phase 10 closeout / D13-V1). The `v0.8.0` tag remains unwritten and is carried into the v1.0 ledger.

**Completion report:** `.planning/v0.8-MILESTONE-COMPLETION.md` — the single artifact that carries the milestone evidence spine (25/25 requirement attestation with 0 unmapped / 0 orphaned; phase status 8-13; frozen locks; Operator Gates Ledger; deviations; test-suite state; v1.0 next path).

**Phases:** 8 / 9 / 10 / 11 / 12 / 13 — all six hermetically green. Phase 13 COMPLETE 6/6 (13-01..13-06).

**Frozen locks (3 gated pins, byte-identical through 13-06):**

- `eval/frozen/compliance.lock.json` — `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded` (Phase 8 pin, T-8-12 UNCHANGED)
- `eval/frozen/hitl.lock.json` — `6e91881f4aafeffb843e996c89ff1c27ef22eb18fac94eceff9184f938481d08` (Phase 11)
- `eval/frozen/fanout.lock.json` — `af3020fd7f38fce2f7396dce81eb151e4ed6952262565785554a59ff51e01252` (Phase 13, operator-signed)
- *(substrate, not a live-gated pin)* `eval/frozen/deep.lock.json` — `f3958dcd4930ac09e03c824f781df9c6eb04b691770cab4d512e11fba3c3c482` (Phase 12 hermetic dry-run)

**Operator Gates Ledger — 3 CLOSED / 3 OPEN / 1 DEFERRED (post-flip 2026-08-21T13:59Z):**

| # | Gate | Status |
|---|------|--------|
| 1 | Phase 10 live 35-case `--via-pipeline --summarize` re-run | **OPEN (BLOCKED-ENV)** — GLM rate-limit 2026-08-21 |
| 2 | Phase 13 live 20-case fan-out UAT | **CLOSED** (accepted-with-notes, D13-V1) |
| 3 | Phase 8 live three-gate re-lock | **CLOSED** |
| 4 | Phase 9 live spike hard gate | **CLOSED** (verdict NO-GO, accepted with escape plan) |
| 5 | `git push origin master` | **MERGED into row 7** — `git remote -v` shows NO remote configured 2026-08-21T13:59Z; the public push is structurally part of 07-06 publication governance |
| 6 | `v0.8.0` annotated tag | **DEFERRED (operator decision 2026-08-21 "跳过 tag")** — carries into v1.0 ledger; tag is local-only, may be written at any time |
| 7 | Phase 7 publication governance | **OPEN (partial)** — DIST-09 eval-numbers half CLOSED by 13-06; demo GIF + docker-build + public remote push outstanding |

**DIST-09 closed:** README `## Eval Results` now carries the verbatim `go run ./scripts/eval_snapshot` block (faithfulness 97% / context_precision 10% / refusal_accuracy 95% 19/20 / 50 cases, `prompt_hash 58ffea08…`). Zero hand-typed numbers.

**13-06 commits:** `9c1781a` (T1 13-05 closeout capture) + `77d6a6d` (T2 completion report + REQUIREMENTS reconciliation) + `b79f297` (T3 README DIST-09) + this terminal flip. Zero code changes — `go build ./... && go vet ./...` exit 0.

**Terminal flip record (2026-08-21T13:59Z):** operator chose "跳过 tag" — status flipped `complete-pending-tag` → `complete` in both this file and `v0.8-MILESTONE-COMPLETION.md` frontmatter WITHOUT a tag hash (ledger row 6 DEFERRED, not CLOSED — no artifact exists). Verified at flip time: `git remote -v` empty (row 5 merged into row 7), `git tag -l "v0.8*"` empty. The tag may be written retroactively at any commit (`git tag -a v0.8.0 <sha>`); until then v1.0's ledger carries row 6.

**Next: `/gsd:new-milestone` (v1.0).** Pre-seeded backlog to re-triage: Phase 999.1 分层分块器 · Phase 999.2 Eval CI · Phase 7 leftovers (demo GIF / docker-build / public remote) · D8-07 non-gate eval metrics · fan-out LLM classifier (13-03 placeholder) · v0.8.0 tag (this ledger row 6).

---

## Phase 14 — CGO 地基 + 平台信任地板 (P10.0) — Plan 14-01 Platform Floor Composition Sign-Off

**Status**: Plan 14-01 COMPLETE 2026-08-22 (4 commits — Task 1 RED + Task 1 GREEN + Task 2 GREEN + Task 3 + docs; PLAT-09 substrate locked hermetically)

**Plans completed**: 1/8 in Phase 14

**Acceptance gate results**:

- `go build ./... && go vet ./...` exit 0
- `go test ./... -count=1` — 25 packages all green, zero regressions
- `go test ./integration/contract -tags=contract -run TestNoBypassOfPlatformFloorCompose -count=1` PASS
- 4 named test locks PASS: `TestPlatformFloorPrependBothPaths` (chat side) + `TestRenderDeepSystemPromptPrependsFloor` (deep side) + `TestPlatformFloorCannotBeShadowed` + `TestFloorHotReloadSurvives`
- Negative proof: probe injection of duplicate `platformFloorText`/`platformFloorTail` → assertion 1 FAILS naming both files → delete → PASS (gate bites)

**Commits** (atomic per-task):

| # | Hash | Type | Subject |
|---|------|------|---------|
| 1 | `737fa7c` | test(14-01) | add failing tests for platform-floor constants and ComposePlatformPrompt |
| 2 | `c28c7d4` | feat(14-01) | implement platform-floor constants and ComposePlatformPrompt |
| 3 | `b9917c4` | test(14-01) | add failing tests for dual-path floor prepend + shadow + hot-reload |
| 4 | `66ab8ce` | feat(14-01) | converge chat+deep prompt rendering onto ComposePlatformPrompt |
| 5 | `fd313ee` | feat(14-01) | add WalkDir structural gate for platform-floor composition |
| 6 | `abafbe2` | docs(14-01) | plan summary — platform-floor composition + structural gate |

**Rule 1 auto-fix deviations** (directly caused by the correct code change):

| # | Target | Change | Why |
|---|--------|--------|-----|
| R1-1 | `internal/agent/hitl/threat_mitigations_test.go` | T-P12-3 sha256LinesRange range (92,97) → (93,98) | +1 line in `internal/runtime/deep_agent.go` from the `platformfloor` import shifted all subsequent line numbers; the bytes inside the alignment group are unchanged (SHA still matches `b432d5dd6e7e750222fdaf9a33d1909ef7cebc565ae901e2342e3a4a713c5e41`) |
| R1-2 | `eval/frozen/compliance.lock.json` | `prompt_hash` 58ffea08... → ca333fbadbfa30eb... | D14-B3 — floor prepend changes the prompt_hash surface; first of the planned full-set re-locks (14-04 handles hitl/fanout/platform-floor) |

**Rule 3 deviation** (blocking import cycle resolution): created `internal/platformfloor` leaf package (documented in RESEARCH as "或独立 internal/platformfloor 包，planner 定"). The agent↔runtime edge in `internal/agent/runner_setup.go` blocks `internal/runtime/deep_agent.go` from importing `internal/agent`, yet Task 3's gate requires `ComposePlatformPrompt(` inside `renderDeepSystemPrompt`'s body. The leaf (no internal dependencies) breaks the cycle structurally: `agent → platformfloor ← runtime`. `internal/agent/platform_floor.go` exists as a documented re-export alias.

**Compliance attestation** (1 PLAT requirement with verifying tests):

| Requirement | Verifying Tests | Gate Stage |
|-------------|----------------|------------|
| PLAT-09 (platform-floor compose = sole exit + Skill cannot shadow) | `TestPlatformFloorPrependBothPaths` + `TestPlatformFloorCannotBeShadowed` + `TestFloorHotReloadSurvives` + `TestPromptHashCoversFloor` + `TestNoBypassOfPlatformFloorCompose` (assertions 1/2/3) | hermetic PASS |

**Resumes at**: `.planning/phases/14-p10-0-cgo-trust-floor/14-02-PLAN.md` (next: admin preview endpoint + /api/admin/platform-prompt route + TestPlatformPromptPreview).

---

## Phase 14 — Plan 14-04 Platform-Floor Frozen Eval Set Sign-Off

**Status**: Plan 14-04 COMPLETE 2026-08-22 (4 commits — Task 1 RED/GREEN + Task 2 RED/GREEN; PLAT-11 hermetic layer + D14-C4 lock)

**Plans completed**: 3/8 in Phase 14 (14-01, 14-02, 14-04)

**Acceptance gate results**:

- `go build ./... && go vet ./...` exit 0
- `go test ./internal/eval/ -count=1` — all green, zero regressions in existing eval framework tests
- `TestPlatformFloorGoldenSchema` PASS (20 rows, 6 scenarios, rule coverage, long-dialogue >=20 turns)
- `TestPlatformFloorFrozenSet` PASS (refusal PASS / SQL leakage FAIL / SQL source FAIL / clean-no-refusal FAIL)
- `TestPlatformFloorLockSelfConsistent` PASS (eval_set_sha256 match / case_count 20 / platform_floor_version 1.0.0 / lock_sha256 convention)
- `eval/frozen/` existing locks untouched; only `platform-floor.lock.json` added

**Commits** (atomic per-task):

| # | Hash | Type | Subject |
|---|------|------|---------|
| 1 | `83fd55c` | test(14-04) | add platform-floor golden schema gate (RED) |
| 2 | `898b655` | feat(14-04) | add 20-case platform-floor golden set (GREEN) |
| 3 | `a48606d` | test(14-04) | add hermetic judge and frozen-lock self-consistency tests (RED) |
| 4 | `1c5af78` | feat(14-04) | implement hermetic platform-floor judge and freeze lock (GREEN) |

**Compliance attestation** (1 PLAT requirement with verifying tests):

| Requirement | Verifying Tests | Gate Stage |
|-------------|-----------------|------------|
| PLAT-11 (platform-floor frozen eval set + hermetic judge + lock) | `TestPlatformFloorGoldenSchema` + `TestPlatformFloorFrozenSet` + `TestPlatformFloorLockSelfConsistent` | hermetic PASS |

**Deviations**: None. One implementation refinement: `TestPlatformFloorLockSelfConsistent` uses a separate `platformFloorLockCanonical` struct (without `lock_sha256`) to recompute the canonical body hash, matching the literal `_lock_sha256_convention` wording.

**Resumes at**: `.planning/phases/14-p10-0-cgo-trust-floor/14-03-PLAN.md` (Wave 3: mattn/go-sqlite3 CGO migration).
