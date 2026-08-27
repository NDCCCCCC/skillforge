---
status: human_needed
phase: 17
phase_name: KPI Dashboard Skill (P10.3)
verified: 2026-08-27T23:30:00Z
score: 9/9 must-have groups verified
overrides_applied: 1
overrides:
  - must_have: "DASH-03 每个点独立数据引用可审计 — sparkline per-point hover popup interaction"
    reason: "DataReferenceChip.tsx:41-45 onPointHover is a disclosed no-op callback. Point-level popup copy exists and is tested (DataReferencePopup '点 N / total', D17-B4), but circle-hover -> popup state plumbing (hovered-index state lifted into chip/MessageBubble) is not wired. Single-value hover tooltip works fully via CSS group-hover. Registered as follow-up plan responsibility per 17-09 gap-closure brief; does not block phase 17."
    accepted_by: "orchestrator (17-09 gap-closure brief)"
    accepted_at: "2026-08-27T23:30:00Z"
re_verification:
  previous_status: gaps_found
  previous_score: 7/9
  gaps_closed:
    - "DASH-03: Sparkline 从未渲染 — DataReferenceChip now imports and renders real Sparkline SVG in the hasSparkline branch (8740805)"
    - "DASH-01: ValidateAgainstKPI boot 未接线 — cmd/server/main.go:388 calls loader.ValidateAgainstKPI(kpiRegistry), fail-closed (815c928)"
    - "Probe defect: verify-phase-17.sh Stage 05 wrong vitest path — src/ prefix + exit code no longer swallowed (10ef061)"
  gaps_remaining: []
  regressions: []
gaps: []
human_verification:
  - test: "SC-1 端到端聊天数字问答（单值路径）：启动 server + 前端，聊天输入「上月贷款余额是多少」"
    expected: "答案含数值 + KPI 定义 + 计算时间窗 + as_of；数字带虚线下划线，hover 显示 5 字段 popup（含 来源：data:aggregated_only）"
    why_human: "需要活体 LLM + server 运行时；UI hover 呈现无法程序断言"
  - test: "修复后视觉走查（DASH-03 闭合后）：多周期 KPI 回答中查看 sparkline 渲染（N>1 references）"
    expected: "svg 趋势线内嵌消息，逐点 circle 可见；单值路径 hover tooltip 不回归。注意：逐点 hover popup 属已知 stub（override），不作为本项通过条件"
    why_human: "视觉布局 + CSS 渲染；8740805 改变了用户可见 UI，只有测试看过，人眼未看过"
  - test: "Eval 运行 + 冻结锁签署（operator 步骤）：go run ./cmd/eval --frozen=kpi --dry-run 填充 prompt_hash / lock_sha256，然后跑 live eval 50 例"
    expected: "kpi_call_accuracy >= 0.9（lock thresholds），metrics 填充，operator_signed_at 签署"
    why_human: "依赖内网 GLM 端点可用性（env-intranet-service-flakiness 先例）；签署是 operator 职责"
  - test: "WSL leg（Stage 06 补腿）：MSYS_NO_PATHCONV=1 wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/kpi/wsl/... -count=1'"
    expected: "PASS（含 TestKPILoadFailClosedLayer2WSL/ValidRealCorpus 断言 7 KPIs + ValidateAgainstKPI 路由名）"
    why_human: "Windows MSYS2 gcc 无法链 duckdb_static（D15-01-1 既定先例）；17-09-SUMMARY 声称已过但验证器无法在本 host 复跑"
---

# Phase 17: KPI Dashboard Skill (P10.3) Verification Report

**Phase Goal:** 业务人员在聊天里问「上月贷款余额多少」得到带完整出处的数字——数值 + KPI 定义 + 计算时间窗 + as_of + 数据引用（hover 显示通道名）；sparkline 趋势内嵌，下钻永不触达原始行。KPI eval 双指标（调用准确率/引用一致性）+ 再识别断言进冻结集。
**Verified:** 2026-08-27T23:30:00Z (re-verification after gap closure 17-09)
**Status:** human_needed (all 9 must-have groups verified; 4 human UAT items pending)
**Re-verification:** Yes — after gap closure (17-09: commits 8740805, 815c928, 10ef061)

## Verification Summary (Re-verification, 2026-08-27)

All 2 blocker gaps and the probe defect from the initial verification are CLOSED. Score: 9/9 must-have groups verified. Status is `human_needed` only because 4 items require human/live verification (visual walkthrough of the newly rendered sparkline, live E2E chat, operator eval signing, WSL leg) — automated checks are fully green.

## Re-verification (2026-08-27) — Gap Closure Evidence

### Gap 1 (DASH-03): Sparkline 从未渲染 — CLOSED

Fix commit `8740805` (`fix(17): integrate Sparkline into DataReferenceChip — close DASH-03 gap`, in git log; touches DataReferenceChip.tsx + DataReferenceChip.test.tsx).

| Check | Evidence | Result |
| ----- | -------- | ------ |
| Import | `web/src/components/chat/parts/DataReferenceChip.tsx:17` — `import { Sparkline } from './Sparkline';` | PASS |
| hasSparkline branch renders real component | `:25` `const hasSparkline = refs.length > 1;`; `:27-48` returns `<Sparkline points={refs.map((r, i) => ({ index: i, period: r.period, value: Number(r.value) }))} ...>` — fed by real `references[]` wire data, data-testid `kpi-sparkline-ref` | PASS |
| Stale placeholder + TODO(17-01) removed | grep for `TODO|FIXME|XXX|TBD|PLACEHOLDER` across DataReferenceChip.tsx, main.go, verify-phase-17.sh, loader_kpi_test.go — 0 matches; the `[N 点趋势]` placeholder span is gone | PASS |
| New integration test | `TestSparklineIntegrated: N>1 references render a real SVG sparkline with one circle per point` in DataReferenceChip.test.tsx — PASS (verbose vitest output) | PASS |
| Sparkline no longer orphaned | Production import exists (DataReferenceChip.tsx:17); Sparkline.test.tsx 4/4 pass | PASS |
| Known stub (disclosed, non-blocking) | `:41-45` `onPointHover={(i) => { ... }}` empty callback body — point-level popup copy exists + tested (DataReferencePopup `点 N / total`), but circle-hover -> popup state plumbing not wired. Disclosed in 17-09-SUMMARY "Known Stubs"; accepted as override, follow-up plan responsibility | OVERRIDE |

### Gap 2 (DASH-01): ValidateAgainstKPI boot 未接线 — CLOSED

Fix commit `815c928` (`fix(17): wire ValidateAgainstKPI at server boot — close DASH-01 gap`, in git log; touches cmd/server/main.go, internal/skill/loader_kpi_test.go, internal/kpi/validate_text.go, kpis/testdata.yaml, internal/kpi/wsl/load_wsl_test.go).

| Check | Evidence | Result |
| ----- | -------- | ------ |
| Boot call exists | `cmd/server/main.go:388` — `if err := loader.ValidateAgainstKPI(kpiRegistry); err != nil {` | PASS |
| Fail-closed posture | `:389` `log.Fatalf("[FATAL] cmd/server: skill/KPI routing validation failed: ...")`; positioned after `kpi.LoadKPIs` (`:368`) and loader population; comment block `:378-387` cites 6c-4 / DASH-01 / D16-08-D; success log `:391` | PASS |
| Test coverage of the method | `internal/skill/loader_kpi_test.go:111` `func TestValidateAgainstKPI` — happy path over real examples/skills x real kpis/ corpus (`:124` NoError), empty registry -> `unknown KPI` error carrying skill name (`:130`), tier-only skill skipped (`:141-142`) | PASS |
| Type-check | `go vet ./cmd/server` exit 0 (independent re-run; Windows link blocked by pre-existing D15-01-1, unchanged status quo) | PASS |
| Collateral repairs (17-09-SUMMARY deviations 1-4) | kpis/testdata.yaml rewritten to Layer-1 dialect (`ds_testdata.data` staging alias, count_column, period params); validate_text.go Rule 7 gains `ParamTypeArray` case + 6 validator tests; storeDatasetMeta.Periods adapter implemented; WSL corpus test count updated to 7 | VERIFIED via go test PASS |

### Probe defect: verify-phase-17.sh Stage 05 — FIXED

Fix commit `10ef061` (`fix(17): repair verify-phase-17.sh Stage 05 vitest path`, in git log).

| Check | Evidence | Result |
| ----- | -------- | ------ |
| src/ prefix | `scripts/verify-phase-17.sh:143-144` — file check and `npx vitest run src/components/chat/parts/Sparkline.test.tsx src/components/chat/parts/DataReferenceChip.test.tsx src/components/chat/parts/DrillChip.test.tsx` | PASS |
| Exit code no longer swallowed | No `\|\| true` on the vitest line; `set -e` active; vitest failure now fails the gate | PASS |
| Stage 05 really executes inside the gate | Fresh gate run: `[STAGE-05-PASS] TypeScript + vitest (frontend components)` with real vitest Duration output (1.93s) and `vitest components passed` — no more "No vitest component tests found — skipping" | PASS |

### Final Test Results (re-run by verifier, this host)

| Suite | Command | Result |
| ----- | ------- | ------ |
| Go packages | `go test ./internal/wire/ ./internal/eval/ ./internal/skill/ ./internal/kpi/ -count=1` | all `ok` (wire 1.5s, eval 1.4s, skill 7.8s, kpi 0.6s) |
| Frontend components | `cd web && npx vitest run src/components/chat/parts/ --reporter=verbose` | 4 files, 16/16 tests pass (incl. TestSparklineIntegrated, TestSparklinePureSVG/HoverPoint/Autoscale/NoDeps, DrillChip 3/3, DataReferencePopup 5-field + point-position) |
| cmd/server type-check | `go vet ./cmd/server` | exit 0 |
| Full hermetic gate | `bash scripts/verify-phase-17.sh` | exit 0; banner `PHASE 17 HERMETIC GATE: PASS`; 10 stages PASS + Stage 06 BLOCKED-WITH-NOTES (D15-01-1 WSL precedent, unchanged); Stage 05 now runs the 3 declared test files for real |
| Debt markers | grep TODO/FIXME/XXX/TBD/PLACEHOLDER across the 4 modified files | 0 matches |

### Known Stubs (registered follow-up responsibility, non-blocking)

| Stub | Location | Disposition |
| ---- | -------- | ----------- |
| Sparkline `onPointHover` empty callback — circle-hover does not yet open the point-level DataReferencePopup (hover state not lifted into chip/MessageBubble; single-value CSS group-hover tooltip unaffected) | `web/src/components/chat/parts/DataReferenceChip.tsx:41-45` | Registered as follow-up plan responsibility per 17-09 gap-closure brief; recorded as formal override in frontmatter; does not block phase 17 |

### Carried-forward warnings (from initial verification, unchanged)

| Item | Location | Status |
| ---- | -------- | ------ |
| DrillChip still orphaned in production (imported only by its own test); internal wiring (pendingDraft store -> ChatInput prefill) correct + tested | `web/src/components/chat/parts/DrillChip.tsx` | Warning — not in gap-closure scope (17-09 brief); drill-down UX render site remains a follow-up responsibility alongside the point-hover stub |

---

# Initial Verification Report (2026-08-27T22:40Z) — historical record

**Status at the time:** gaps_found — 7/9 must-have groups verified; 2 blocker gaps (DASH-03 sparkline orphaned, DASH-01 ValidateAgainstKPI never called) + 1 probe defect (Stage 05 path bug). All closed above; the sections below are the initial-round evidence, retained for audit.

## Requirements Coverage (initial round; re-verified statuses in brackets)

| Requirement | Status (initial) | Evidence | Re-verification |
|-------------|------------------|----------|-----------------|
| DASH-01 Skill 编排 + 路由 | PARTIAL — 2/3 (ValidateAgainstKPI orphaned) | SKILL.md 7-field schema + YAML routing table (loans/customers) | CLOSED — boot call verified, tested |
| DASH-02 每个数字带 data-reference | VERIFIED (single-value) | DataReferencePopup 5 fields; chip CSS hover; MessageBubble kpi_reference branch | Unchanged (point-hover sub-item = override) |
| DASH-03 Sparkline 渲染 | FAILED (wiring) | Component substantive; wire contract verified; chip branch was placeholder | CLOSED — real render verified + integrated test |
| DASH-04 下钻 UX | VERIFIED (internal wiring) / unreachable | pendingDraft store + DrillChip + ChatInput prefill, tests pass | Unchanged (DrillChip render site still open — carried warning) |
| DASH-05 多 KPI 组合编排 | VERIFIED (declaration level) | SKILL.md steps + routing + 10 golden multi_kpi cases | Unchanged; live E2E -> human item 1 |
| GEVL-09 KPI 调用准确率 50 例 | VERIFIED | eval/golden/kpi.jsonl 50 cases, 5 categories; JudgeKPIReferenceConsistency | Unchanged; gate Stage 10 PASS (50 cases) |
| GEVL-10 引用一致性 | VERIFIED | 3-step judge: as_of token, channel, number-vs-snapshot 0.5% tolerance | Unchanged; go test ./internal/eval/ ok |
| REID 冻结子集 | VERIFIED | reid.jsonl 10 cases (5 join + 5 differential), should_refuse + min_suppression_count 5 | Unchanged; gate Stage 11 PASS |
| 5+1 冻结锁字段 | VERIFIED | kpi.lock.json incl. kpi_registry_sha256; thresholds 0.9 / 0.005; operator signing pending | Unchanged; gate Stage 09 PASS; signing -> human item 3 |

## Anti-Patterns Found (final state after gap closure)

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `web/src/components/chat/parts/DataReferenceChip.tsx` | 41-45 | `onPointHover` no-op callback (disclosed known stub) | Resolved-as-override | Point hover popup deferred to follow-up plan; rendering itself works |
| `web/src/components/chat/parts/DrillChip.tsx` | — | Still no production render site | Warning (carried) | Drill UX unreachable end-to-end; internal wiring correct + tested |
| `internal/wire/chunk.go` | 257 | "deep.TODO" | Info | eino ADK type reference, not a debt marker |

Resolved in gap closure: stale TODO(17-01) placeholder (8740805), Sparkline orphan (8740805), ValidateAgainstKPI orphan (815c928), Stage 05 path bug + swallowed exit code (10ef061).

## Probe Execution

| Probe | Command | Result | Status |
| ----- | ------- | ------ | ------ |
| `scripts/verify-phase-17.sh` | `bash scripts/verify-phase-17.sh` (re-run) | exit 0; Stage 01-05, 07-11 PASS; Stage 06 BLOCKED-WITH-NOTES (D15-01-1 WSL precedent); Stage 05 runs vitest for real; banner `PHASE 17 HERMETIC GATE: PASS` | PASS |

## Human Verification Required

See frontmatter `human_verification` (4 items): (1) SC-1 live E2E single-value chat Q&A with hover popup; (2) post-fix visual walkthrough of the newly rendered sparkline (N>1); (3) operator eval run + frozen-lock signing; (4) WSL leg for internal/kpi/wsl (D15-01-1).

---

_Verified: 2026-08-27T23:30:00Z (re-verification) — initial round 2026-08-27T22:40:00Z_
_Verifier: Claude (gsd-verifier)_
