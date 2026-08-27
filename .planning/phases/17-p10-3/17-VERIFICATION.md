---
status: gaps_found
phase: 17
phase_name: KPI Dashboard Skill (P10.3)
verified: 2026-08-27T22:40:00Z
score: 7/9 must-have groups verified
re_verification: false
gaps:
  - truth: "DASH-03: sparkline 迷你趋势内嵌消息，每个点独立数据引用可审计（ROADMAP SC-2）"
    status: failed
    reason: "Sparkline.tsx exists and is substantive, but is ORPHANED — zero production imports. DataReferenceChip's sparkline branch (refs.length > 1) still renders a '[N 点趋势]' placeholder behind a stale 'TODO(17-01)' marker. Plans 17-01 and 17-02 ran in parallel and the promised 1-line integration swap (replace placeholder with <Sparkline/>) was never made after 17-01 landed. Users can never see a sparkline in chat."
    artifacts:
      - path: "web/src/components/chat/parts/DataReferenceChip.tsx"
        issue: "Lines 26-44: hasSparkline branch renders placeholder span + stale TODO(17-01); does not import or render Sparkline"
      - path: "web/src/components/chat/parts/Sparkline.tsx"
        issue: "ORPHANED — referenced only by its own test file; never imported by any production component"
    missing:
      - "Import Sparkline into DataReferenceChip and render it in the hasSparkline branch (replace placeholder)"
      - "Remove stale TODO(17-01) marker"
      - "Add/extend a test asserting Sparkline svg (data-testid=sparkline) renders when refs.length > 1"
      - "Fix scripts/verify-phase-17.sh Stage 05 path bug (see warning) so vitest actually runs in the gate"
  - truth: "DASH-01: ValidateAgainstKPI 在 loader.go 中存在并在 boot 时调用"
    status: failed
    reason: "ValidateAgainstKPI exists (internal/skill/loader.go:164) but is NEVER called anywhere — no call site in cmd/server/main.go (skills load at :205, kpi.LoadKPIs at :367, no ValidateAgainstKPI between/after them), and zero test coverage of the method itself (loader_kpi_test.go only tests parseRoutingTable). Repo-wide grep finds only the definition + its doc comment. The routing table is validated nowhere at runtime."
    artifacts:
      - path: "internal/skill/loader.go"
        issue: "ValidateAgainstKPI defined at :164, orphaned — no callers, no tests"
      - path: "cmd/server/main.go"
        issue: "Boot sequence loads skills (:205) and KPIs (:367) but never invokes loader.ValidateAgainstKPI(kpiRegistry)"
    missing:
      - "Call loader.ValidateAgainstKPI(kpiRegistry) in cmd/server boot after both LoadKPIs and Loader.Scan (D16-C2 hermetic pattern), failing fast on routing-table/registry drift"
      - "Add a test exercising ValidateAgainstKPI (happy path + unknown-KPI-name failure)"
---

# Phase 17: KPI Dashboard Skill (P10.3) Verification Report

**Phase Goal:** 业务人员在聊天里问「上月贷款余额多少」得到带完整出处的数字——数值 + KPI 定义 + 计算时间窗 + as_of + 数据引用（hover 显示通道名）；sparkline 趋势内嵌，下钻永不触达原始行。KPI eval 双指标（调用准确率/引用一致性）+ 再识别断言进冻结集。
**Verified:** 2026-08-27T22:40:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Verification Summary

7 of 9 must-have groups verified against actual code. Two wiring gaps block the phase goal:

1. **DASH-03 (BLOCKER):** Sparkline component exists, is pure SVG, and passes all 12 vitest tests — but nothing renders it. `DataReferenceChip.tsx` still carries a stale `TODO(17-01)` and renders a `[{N} 点趋势]` placeholder in the sparkline branch. The 17-01/17-02 wave ran in parallel and the planned integration swap never happened. ROADMAP SC-2 ("sparkline 迷你趋势内嵌消息") is not user-visible. Collateral: `DrillChip` is also orphaned, so the DASH-04 drill-down UX is unreachable end-to-end (its internal wiring — pendingDraft store → ChatInput prefill — is correct and tested).
2. **DASH-01 (BLOCKER):** `ValidateAgainstKPI` is defined in `loader.go` but never called — not at boot in `cmd/server/main.go`, not in any test. The must-have explicitly requires the boot-time call. The skill routing table is cross-checked against the KPI registry nowhere at runtime.

Everything else holds: single-value hover provenance popup (5 fields incl. `来源：data:aggregated_only`) is wired end-to-end from `kpi_tool.go` through wire types to MessageBubble; multi-period executor + `references[]` wire contract verified in Go and TS; GEVL-09/10 judge, 50-case golden set, 10-case REID frozen subset, and the 5+1 lock fields all verified; the phase's own 11-stage gate passes 10 PASS + 1 blocked-with-notes (documented D15-01-1 WSL precedent).

Note: ROADMAP marks Phase 17 `mode: mvp`, but the goal is not in strict "As a…, I want to…, so that…" User Story format and `gsd-sdk` is not installed on this host to run `user-story.validate`. Verification proceeded against the coordinator's must-have decomposition, which maps 1:1 onto the ROADMAP Success Criteria. User Flow Coverage under MVP narrowing: ask → KPI call → provenance hover = PASS (single value); sparkline view = FAIL; drill click = FAIL (unreachable).

## Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| DASH-01 Skill 编排 + 路由 | ⚠️ PARTIAL — 2/3 | `examples/skills/kpi-dashboard/SKILL.md:1-41` — 7-field schema (description/agent/reads/writes/steps/guardrails/temperature), `reads: [data:aggregated_only]` (:4-5); fenced YAML routing table grouped by business domain loans/customers (:10-22). **FAILED:** `ValidateAgainstKPI` (`internal/skill/loader.go:164`) exists but has zero call sites — not called at boot (`cmd/server/main.go:205` skills, `:367` LoadKPIs, no validate call), zero test coverage |
| DASH-02 每个数字带 data-reference | ✅ VERIFIED (single-value) | `DataReferencePopup.tsx:46-68` renders all 5 fields: value (:51), 周期 (:55), 数据截至/as_of (:58), `来源：data:aggregated_only` (:62), 函数 (:64). `DataReferenceChip.tsx:54-77` single-value CSS group-hover tooltip. `MessageBubble.tsx:108-109` `kpi_reference` branch renders chip. Sparkline-point hover path exists in Popup but is unreachable (see DASH-03 gap) |
| DASH-03 Sparkline 渲染 | ❌ FAILED (wiring) | `Sparkline.tsx` exists: pure SVG `viewBox="0 0 300 32"` (:50), `<polyline>` (:58) + per-point `<circle>` (:64-78), zero chart libs in file (gate Stage 07 PASS). `references[]` wire contract: `internal/wire/chunk.go:175,215` (Go struct + field), `web/src/types/wire-types.ts:153,187` (TS mirror), `internal/agent/tools/kpi_tool.go:275,333` (`buildPointReferences` populates from real snapshot rows). **BUT:** Sparkline is ORPHANED — `DataReferenceChip.tsx:27` stale `TODO(17-01)`, hasSparkline branch renders placeholder. ROADMAP SC-2 not user-visible |
| DASH-04 下钻 UX | ✅ VERIFIED (internal wiring) / ⚠️ unreachable | `app-store.ts:468,528,579,834` pendingDraft + setPendingDraft; `DrillChip.tsx:30-32` onClick → `setPendingDraft('请调 {kpiName} periods={periodsTemplate}')`; `ChatInput.tsx:131-140` useEffect prefill when empty, `:147-148` clear on send. All 3+3 tests pass. **WARNING:** DrillChip has no production render site (same root cause as DASH-03) — user cannot reach the chip |
| DASH-05 多 KPI 组合编排 | ✅ VERIFIED (declaration level) | SKILL.md steps `:24-28`: "多 KPI 答案：每个数字独立 hover；可下钻「看月度趋势」" combination template hint; routing table supports multi-KPI selection; `tier_limits` restricts to `kpi_call only`. Golden set has 10 multi_kpi_combination cases. End-to-end multi-call behavior needs human/live verification |
| GEVL-09 KPI 调用准确率 50 例 | ✅ VERIFIED | `eval/golden/kpi.jsonl` = 50 lines, 50 `expected_kpi` matches, 5 categories (20+10+10+5+5); `internal/eval/kpi_metrics.go:59` `JudgeKPIReferenceConsistency` programmatic 3-step; `go test ./internal/eval/` PASS (gate Stage 04) |
| GEVL-10 引用一致性 | ✅ VERIFIED | `kpi_metrics.go`: Step 1 as_of token (:60-62, tokens incl. as_of/数据截至/截至/截止 :116-117), Step 2 channel `data:aggregated_only` (:65-67), Step 3 number-vs-snapshot within 0.5% tolerance (:84-99) or AllowedExpressions |
| REID 冻结子集 | ✅ VERIFIED | `eval/frozen/reid.jsonl` = 10 cases: 5 `attack_type:"join"` (Pitfall 11, incl. k_min dual-period defense) + 5 `attack_type:"differential"` (Pitfall 22); outcomes carry `should_refuse` / `min_suppression_count: 5`. (17-07 SUMMARY claims 6+4; actual 5+5 — total 10 correct, distribution doc-inaccurate) |
| 5+1 冻结锁字段 | ✅ VERIFIED | `eval/frozen/kpi.lock.json`: model_ver, prompt_hash, seed=42, eval_set_version, skill_name, eval_set_sha256, **kpi_registry_sha256** (:2-9) + thresholds (accuracy_min 0.9, tolerance 0.005). Note: prompt_hash and lock_sha256 are explicit operator-fill placeholders (`<fill: … --dry-run …>`); `operator_signed_at: null`, metrics null — lock not yet signed |

## Key Files Verified

| File | Status |
|------|--------|
| `examples/skills/kpi-dashboard/SKILL.md` | EXISTS — substantive (7 fields, YAML routing, guardrails) |
| `web/src/components/chat/parts/Sparkline.tsx` | EXISTS — substantive, **ORPHANED** |
| `web/src/components/chat/parts/Sparkline.test.tsx` | EXISTS — 4 tests pass |
| `web/src/components/chat/parts/DataReferencePopup.tsx` | EXISTS — substantive, wired |
| `web/src/components/chat/parts/DataReferenceChip.tsx` | EXISTS — single-value path wired; **sparkline branch STUB (stale TODO)** |
| `web/src/components/chat/parts/DrillChip.tsx` | EXISTS — substantive, **ORPHANED** |
| `web/src/store/app-store.ts` | pendingDraft + setPendingDraft WIRED |
| `web/src/components/chat/ChatInput.tsx` | pendingDraft useEffect WIRED |
| `web/src/components/chat/MessageBubble.tsx` | kpi_reference branch WIRED (:108-109) |
| `internal/wire/chunk.go` | PointReference :175 + References :215 |
| `web/src/types/wire-types.ts` | PointReference :153 + references :187 (D15 lockstep) |
| `internal/skill/loader.go` | ValidateAgainstKPI :164 — **ORPHANED (no callers)** |
| `internal/skill/loader_kpi_test.go` | parseRoutingTable test (4 entries) — does NOT cover ValidateAgainstKPI |
| `internal/eval/kpi_metrics.go` | JudgeKPIReferenceConsistency 3-step |
| `internal/agent/tools/kpi_tool.go` | References: buildPointReferences :275 |
| `eval/golden/kpi.jsonl` | 50 cases |
| `eval/frozen/reid.jsonl` | 10 attack cases (5 join + 5 differential) |
| `eval/frozen/kpi.lock.json` | all required fields incl. kpi_registry_sha256 |
| `scripts/verify-phase-17.sh` | EXISTS — runs; **Stage 05 path bug** (see Anti-Patterns) |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Frontend component tests (gate skipped them — run manually) | `npx vitest run src/components/chat/parts/{Sparkline,DataReferenceChip,DrillChip}.test.tsx` | 3 files, 12/12 tests pass | ✓ PASS |
| Golden set size | `grep -c expected_kpi eval/golden/kpi.jsonl` | 50 | ✓ PASS |
| REID subset size | `grep -c attack_type eval/frozen/reid.jsonl` | 10 (5 join + 5 differential) | ✓ PASS |
| Sparkline zero-dep | grep recharts/react-sparklines/chart.js in Sparkline.tsx | 0 hits (recharts in package.json is pre-existing, Phase 06-04 TrendPanel — untouched this phase) | ✓ PASS |
| ValidateAgainstKPI called at boot | repo-wide grep | only definition + comment | ✗ FAIL |

## Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `scripts/verify-phase-17.sh` | `bash scripts/verify-phase-17.sh` | 10/11 stages PASS; Stage 06 BLOCKED-WITH-NOTES (kpi/wsl duckdb_static link under MSYS2 gcc, D15-01-1 precedent — Linux leg documented); gate banner `PHASE 17 HERMETIC GATE: PASS` | ✓ PASS (with Stage 05 caveat below) |

**Probe caveat:** gate Stage 05 printed "No vitest component tests found — skipping" yet reported PASS. The script (line ~143) checks `components/chat/parts/...` after `cd web`, but the real path is `src/components/...`. The vitest leg never runs inside the gate. Compensating control: I ran the three test files manually (12/12 pass above). The script path bug is listed as a WARNING anti-pattern — the probe overstates frontend coverage.

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `web/src/components/chat/parts/DataReferenceChip.tsx` | 27 | `TODO(17-01)` — stale marker; referenced plan is COMPLETE and did not include the integration; placeholder is live production behavior in the sparkline branch | 🛑 Blocker (root cause of DASH-03 gap) | Sparkline never renders; DrillChip unreachable |
| `web/src/components/chat/parts/Sparkline.tsx` | — | Orphaned component (no production importer) | 🛑 Blocker (wiring) | Exists but invisible to users |
| `web/src/components/chat/parts/DrillChip.tsx` | — | Orphaned component (no production importer) | ⚠️ Warning (internal wiring itself correct + tested) | Drill UX unreachable end-to-end |
| `internal/skill/loader.go` | 164 | Orphaned method — ValidateAgainstKPI never invoked, untested | 🛑 Blocker (DASH-01 must-have) | Routing table unvalidated at runtime |
| `scripts/verify-phase-17.sh` | ~143 | Stage 05 path bug (`components/` vs `src/components/`) + `|| true` on vitest | ⚠️ Warning (probe defect) | Gate reports frontend PASS without running frontend tests |
| `internal/wire/chunk.go` | 257 | "deep.TODO" | ℹ️ Info | eino ADK type reference, not a debt marker |

## Human Verification Required

### 1. SC-1 端到端聊天数字问答（单值路径）
**Test:** 启动 server + 前端，聊天输入「上月贷款余额是多少」
**Expected:** 答案含数值 + KPI 定义 + 计算时间窗 + as_of；数字带虚线下划线，hover 显示 5 字段 popup（含 `来源：data:aggregated_only`）
**Why human:** 需要活体 LLM + server 运行时；UI hover 呈现无法程序断言

### 2. Eval 运行 + 冻结锁签署（operator 步骤）
**Test:** `go run ./cmd/eval --frozen=kpi --dry-run` 填充 prompt_hash / lock_sha256；然后跑 live eval 50 例
**Expected:** kpi_call_accuracy ≥ 0.9（lock thresholds），metrics 填充，operator_signed_at 签署
**Why human:** 依赖内网 GLM 端点可用性（env-intranet-service-flakiness 先例）；签署是 operator 职责

### 3. WSL leg（Stage 06 补腿）
**Test:** `MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/kpi/wsl/... -count=1'`
**Expected:** PASS
**Why human:** Windows MSYS2 gcc 无法链 duckdb_static（D15-01-1 既定先例）

### 4. 修复后视觉走查（DASH-03/04 闭合后）
**Test:** 多周期 KPI 回答中查看 sparkline 渲染、逐点 hover popup、点击「看月度趋势」chip 后输入框预填
**Expected:** svg 趋势线内嵌消息；每点 hover 显示「（点 N / total）」popup；点击 chip 后 ChatInput 预填「请调 … periods=…」
**Why human:** 视觉布局 + CSS hover 交互；且当前代码该路径尚未接通，闭合后必须人验

## Gaps Summary

Two blocker gaps, one root cause each, neither deferred by later phases (Phase 18 report skill / Phase 19 portal / Phase 20 admin do not cover sparkline integration or boot-time routing validation):

1. **Sparkline/DrillChip integration (DASH-03, breaks SC-2).** 17-01 and 17-02 ran in parallel; both summaries documented the deferral ("1-line swap once 17-01 commits") but no later plan in the phase owned the swap, and it never happened. All pieces exist and pass isolated tests; the single missing edit is wiring `<Sparkline>` (and `DrillChip`) into `DataReferenceChip`'s hasSparkline branch.
2. **ValidateAgainstKPI boot call (DASH-01).** The method and its parsing helper were delivered with tests for the parser only; the boot wiring in `cmd/server/main.go` was in no plan's task list and is missing.

Minor: fix `scripts/verify-phase-17.sh` Stage 05 so the gate actually runs the frontend tests it claims to cover.

---

_Verified: 2026-08-27T22:40:00Z_
_Verifier: Claude (gsd-verifier)_
