---
phase: 49-hermetic-demo
disposition_round: 1
dispositioned: 2026-10-22T15:55:00Z
source_review: 49-REVIEW.md
total: 8
fixed: 3
deferred: 1
open_accepted: 4
---

# Phase 49: Review Disposition Ledger (Round 1)

处置原则：warning 必须在本 phase 内修复或显式说明为何不修；info 可 `fixed` / `deferred` / `open (accepted with rationale)`。每行的 `Source` 写明证据位置，`Fix` 写明落地提交或测试名。

## Warnings

- **R-49-01** — status: **fixed**
  Finding: `internal/webui/static.go` 的 dist 检查在 `/api` 闸之前，缺 dist 时未注册 `/api/*` 拿 503 而非 404（来源：独立 verifier W-1）。
  Fix: `/api` 闸前移（提交 `216620b`）；新增 `TestStaticHandlerMissingDistStill404sUnregisteredAPIPaths`；门禁第 3 次连跑 ALL PASSED。
  Source: `internal/webui/static.go`（Handler 首段）、`internal/webui/static_test.go`、`49-03-SUMMARY.md` §Post-verification fix。

- **R-49-02** — status: **fixed**
  Finding: 49-02 truth ⑧ 的 Go 侧 omitempty 机械判据未落地（来源：独立 verifier W-2）。
  Fix: `TestModeOmitEmptyKeepsRealModelWireUnchanged`（零值 Verdict marshal 无 `"mode"` 键；`Mode=ModeStub` marshal 出 `"mode":"stub"`）。
  Source: `internal/verdict/verdict_test.go`。

- **R-49-03** — status: **fixed**
  Finding: 门禁无 README 内容断言，quickstart 可以无声丢掉 stub 记载（来源：独立 verifier W-3；49-04 truth ⑩ 声称了它）。
  Fix: P4 新增 `P4-README-LABEL` 检查块（四条 grep：`export RETRIEVER_BACKEND=embedded` / Ollama 默认端点 / `"mode":"stub"` / `SKILLFORGE_MCP_SERVICE_SKILL`）+ 变异演练 **D10**（删块 ⇒ `--only P4` 必红）；`ok_partition "P4"` 扩为 4 token。
  Source: `verify-phase-49.sh` partition_P4 与 D10 注册行。

## Info

- **R-49-04** — status: **open (accepted with rationale)**
  Finding: truth ⑨ 字面「git status 为空」vs 实现的运行前后差集。
  Rationale: 实现**更强**（开发机既存脏文件不产生假红，门禁自留产物必现于差集）；方向正确，措辞对齐记入 `49-VERIFICATION.md` W-4，不改计划原文。
  Source: `verify-phase-49.sh` partition_P9 第 (4) 项。

- **R-49-05** — status: **deferred**（→ Phase 50）
  Finding: ROADMAP SC1/SC2 与 DEMO-01 措辞和 D-01/D-02 字面冲突（来源：独立 verifier W-6）。
  Action taken: ROADMAP Phase 49 段新增 **Scope Note**（记录实施口径），不改验收标准原文。
  Deferred to: Phase 50（其目标即「README 与仓库实况零失真」，顺带修订 SC1/SC2/DEMO-01 措辞）。
  Source: `.planning/ROADMAP.md` Phase 49 §Scope Note。

- **R-49-06** — status: **open (accepted with rationale)**
  Finding: 门禁 live 腿使用测试专用 embedding 替身（来源：独立 verifier W-7）。
  Rationale: 49-04-PLAN flagged_assumption 已锁定边界（只 bind 127.0.0.1、key 只进子进程、生产零改动、打印 `using test-only embedding double`）；独立 verifier 用真实模型端点在 fresh clone 上补齐真链路证据（其 §4），风险面已收敛。
  Source: `verify-phase-49.sh` live 驱动段；`49-04-PLAN.md` flagged_assumptions。

- **R-49-07** — status: **open (accepted with rationale)**
  Finding: fresh clone 未 `npm run build` 时 P6 变红而非跳过（来源：独立 verifier W-8）。
  Rationale: fail-closed 优于静默假绿；CI 化时需先 build 前端（已记入验证报告 W-8 建议）。
  Source: `verify-phase-49.sh:547/:564`。

- **R-49-08** — status: **open (accepted with rationale)**
  Finding: D-08 前端落点与决策字面不同（`isStubVerdict` + VerdictCard 徽标，而非 MessageBubble 的 isVerdict 扩展）（来源：独立 verifier W-5）。
  Rationale: 49-02-PLAN 已登记为 flagged_assumption（"MessageBubble 的消费留给后续 slice"）；D-08 的目标（用户可见 stub 标注）已达成且有组件测试兜底（`VerdictCard.test.tsx` 20 条）。
  Source: `web/src/types/verdict.ts`、`web/src/components/center/VerdictCard.tsx`、`web/src/__tests__/VerdictCard.test.tsx`。

## Count reconciliation

`total: 8 = fixed 3 (R-49-01/02/03) + deferred 1 (R-49-05) + open-accepted 4 (R-49-04/06/07/08)`。无 silent drop。
