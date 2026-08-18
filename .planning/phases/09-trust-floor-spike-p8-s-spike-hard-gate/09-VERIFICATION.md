---
phase: 09-trust-floor-spike-p8-s-spike-hard-gate
verified: 2026-08-18T05:30:24Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
deferred:
  - truth: "历史 verdict JSON 经 conclusionText/refusalText 确定性渲染为纯文本（SC-4 第二子句的实现与测试）"
    addressed_in: "Phase 10"
    evidence: "Phase 10 SC-4/MEM-04 逐字包含该条款：「历史 verdict JSON 经 conclusionText/refusalText 确定性渲染为纯文本（测试断言投影输出零 chunk 标记）（MEM-04）」；Phase 9 SC-4 自身措辞「供 Phase 10 实现」。规则文本已在 ROADMAP SC-4 与 09-CONTEXT.md:28 成文；projection.go 仅实现 B1/B2/B3（按 D9-B4 spike 范围裁定）。"
  - truth: "恶意注入用例被拒答或标注「未验证用户陈述」（SC-3 软子句）"
    addressed_in: "Phase 10"
    evidence: "实测 refusal_rate=0（恶意组被作答而非拒答）——该失败即 NO-GO 裁决的核心发现（Pitfall 1 H2），由用户签字的逃逸方案覆盖（STATE.md D9-V2：拒答判定不依赖历史上下文 / 历史不携带拒答语义 / query-only 拒答分类器候选），Phase 10 前置强化后重跑硬门验证。"
---

# Phase 9: 历史注入 trust-floor spike 硬门（P8.S）Verification Report

**Phase Goal:** 用 golden-set A/B 实验证明：把投影后的会话历史注入 prompt 不会静默漂移已校准的 trust floor（拒答率、faithfulness、引用精度）。这是硬门——不过门，Phase 10 记忆不得发布。本 phase 是 spike，产出是报告 + 消毒规则，不是产品代码。
**Verified:** 2026-08-18T05:30:24Z
**Status:** passed
**Re-verification:** No — initial verification

## Verdict Logic (read first)

The experiment ran and returned a decisive **NO-GO** (refusal 95%→0%, faithfulness -12.6%~-22.4%, context_recall -63.0%~-87.2%). The phase goal's literal wording ("证明…不会漂移") is the **gate criterion**, not the phase deliverable. Per the phase's own contract, the deliverable is: 报告 + 消毒规则 + 有记录的明确结论（SC-2: "go/no-go 结论明确且有记录"）+ 逃逸方案. GEVL-08's own wording explicitly permits the NO-GO path: "不过门则记忆不得发布，产出消毒规则或逃逸方案". A recorded, human-signed NO-GO is a **successful hard-gate execution** — the gate did exactly what it existed to do: it stopped Phase 10 memory from shipping on a drifted trust floor. All verification below is goal-backward against that contract.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC-1: Spike 报告落盘——3 组场景 A/B 对比，四指标漂移逐项可见 | ✓ VERIFIED | `reports/` on disk: 3 JSONL streams `wc -l` = 10+15+10 = 35 rows (case_ids verified `hist-adv-001..010` / `hist-fup-001..015` / long signals, all `with_history:true`); `summary.md` 55 lines with 5-metric drift table (baseline + 3 category Δ + threshold, all four gate metrics per-category); `summary.json` machine-readable; headline numbers fully transcribed into in-git 09-06-SUMMARY.md (durable record — T-9-RESULT-LOSS mitigation). Baseline arm reads frozen `eval/report.json` per Plan 09-05 planner ruling (no baseline re-run by design). |
| 2 | SC-2: go/no-go 结论明确且有记录；不过门则逃逸方案选型及理由 | ✓ VERIFIED | `reports/summary.json` `decision:"NO-GO"` (parsed); exit 1 recorded; `## Verdict` + reasons + filled decision table in summary.md; human signature 2026-08-18「接受 NO-GO 带逃逸方案」recorded in 09-06-SUMMARY.md + STATE.md D9-V1/V2/V3 + ROADMAP Progress. Escape plan with reasons: D9-V2 (3 hardening obligations + 3 candidate escapes + mandatory re-gate <5%/<10pp before memory ships). |
| 3 | SC-3: 零例让历史中的伪文本穿透成 citation | ✓ VERIFIED | `grep -c '"injection_penetrated":true' reports/*.jsonl` → **0 matches** across all 35 rows; summary.json per-category `penetration_count: 0` ×3. Code-read independence check (runner.go:402-413): Step 7 scans citation `ChunkID` for the fake-ref string — orthogonal to CR-02's phantom-ID grounding defect; the fake refs were scrubbed from user history by D9-B2 (model never saw them), so non-penetration is genuine. Soft sub-clause (被拒答或标注) measured-FAIL → folded into the NO-GO record (see Deferred). |
| 4 | SC-4: 消毒规则作为 spike 主产物成文 | ✓ VERIFIED | `internal/history/projection.go` (78 lines) implements B1 (role whitelist + field trim), B2 (user-side `[CHUNK/CH-/id:]` scrub → `[REF-N]`), B3 (last-3-pairs hard limit + orphan-assistant drop), D9-B1/B2/B3 pinned in godoc; `projection_test_fixtures.go` (150 lines) exports 7 named fixtures (3 pos + 4 neg). Verdict-JSON-rendering clause documented (ROADMAP/CONTEXT:28), implementation explicitly Phase 10 MEM-04 (see Deferred). WR-04 fixture defect noted under Anti-Patterns. |
| 5 | 硬约束：冻结基线与产品代码零污染 | ✓ VERIFIED | `git status --porcelain eval/frozen/ eval/sample.jsonl cmd/eval/ cmd/server/ internal/server/ go.mod go.sum` → empty. Phase-9 commit list scoped to `cmd/spike-history/`, `internal/history/`, `eval/golden/`, `internal/eval/golden_test.go`, `.gitignore` (+reports/ line 25), docs. |
| 6 | golden set 75 行口径 + 契约门升级（加法式） | ✓ VERIFIED | `wc -l eval/golden/compliance.jsonl` = 75; 10 adversarial + 15 followup; subclasses 5/5/5 (anaphora/multi_hop_chain/post_refusal); `TestGoldenCategoryCoverage` asserts `require.Len(t, cases, 75)` + preserved 20/20/10 baseline slices; `TestGoldenHistoryInjectionRows` locks anchor self-consistency + baseline query byte-equality; both pass. |
| 7 | 全量回归绿 | ✓ VERIFIED | `go test ./...` exit 0 (verified in this verification run); `go test ./cmd/spike-history -v` → 10 hermetic cases PASS (TestComputeVerdict 6 gate scenarios incl. injection-penetration hard gate, TestFormatDelta 2, TestBaselineRefusalAccuracy, TestRunSummarizeHermetic). |

**Score:** 7/7 truths verified

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | 历史 verdict JSON 经 `conclusionText`/`refusalText` 确定性渲染为纯文本（SC-4 第二子句实现 + 测试断言） | Phase 10 | Phase 10 SC-4/MEM-04 carries the clause verbatim: 「历史 verdict JSON 经 conclusionText/refusalText 确定性渲染为纯文本（测试断言投影输出零 chunk 标记）（MEM-04）」. Architecturally necessary: production stores raw `verdictJSON` as assistant session content (`internal/server/chat.go:521-524`), so Phase 10's projection must handle it. Phase 9 scope per SC-4's own tail: 「供 Phase 10 实现」. |
| 2 | 恶意注入用例被拒答或标注「未验证用户陈述」 | Phase 10 | Measured refusal_rate=0 — this IS the NO-GO core finding (Pitfall 1 H2). Covered by user-signed escape plan (STATE.md D9-V2: 拒答判定不依赖历史上下文；历史不携带拒答语义；query-only 拒答分类器候选) + mandatory re-gate before memory ships. |

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | ----------- | ------ | ------- |
| `eval/golden/compliance.jsonl` | 75 行（50 baseline 纯追加 + 10 恶意 + 15 善意 5/5/5） | ✓ VERIFIED | Counts + subclass distribution verified; contract tests lock baseline immutability |
| `internal/eval/golden_test.go` | GEVL-05 升级 75 行口径 + spike schema 门 | ✓ VERIFIED | 360 lines; both tests present and green |
| `internal/history/projection.go` | Project() B1/B2/B3 + D9 godoc | ✓ VERIFIED | 78 lines; consumed by runner.go:236 |
| `internal/history/projection_test_fixtures.go` | ≥6 exported fixtures | ✓ VERIFIED (defect) | 7 fixtures (3 pos + 4 neg); WR-04: `neg_history_truncated_to_three_pairs` Expected (4 msgs) contradicts Project() output (6 msgs incl. "old two" pair) — see Anti-Patterns |
| `cmd/spike-history/{main,runner,spike_row}.go` | spike-only 二进制 | ✓ VERIFIED | 415+527+235 lines; compiles, dry-run hermetic smoke passes |
| `cmd/spike-history/long.go` | 长会话合成器 | ✓ VERIFIED | 352 lines; seeded deterministic; long.jsonl 10 rows category `history_injection_long` |
| `cmd/spike-history/{summary.go,summary_test.go}` | 聚合 + 裁决 + hermetic 测试 | ✓ VERIFIED | 860+489 lines; 5 functions + 10 test cases green |
| `reports/` 7 files | live spike 产物 | ✓ VERIFIED | On disk (git-ignored per planner ruling 3); headline numbers transcribed to in-git SUMMARY |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| runner.go::RunEntry | internal/history.Project | with_history=true 投影注入 | ✓ WIRED | runner.go:236 `history.Project(sessionMsgs)`; projected messages flow into `buildUserMessageBody` (runner.go:283, 432-441) |
| runner.go::RunEntry | internal/eval 四指标 | Faithfulness/AnswerRelevancy/ContextPrecision/ContextRecall | ✓ WIRED | runner.go:370-385; judge-computed values present in live JSONL |
| runner.go::RunEntry | internal/verdict.GroundingCheck | grounding 白名单校验 | ✓ WIRED (defective) | runner.go:342 — wired but CR-02 makes `grounding_pass` structurally false in live mode (phantom chunk labels); diagnostic column only, not a gate metric |
| long.go::loadLongHistoryRows | loadSpikeGolden + eval.LoadGolden + seeded rand | signal + filler 合成 | ✓ WIRED | Verified in long.jsonl output (10 rows, deterministic signal mix) |
| summary.go | JSONL 流 + eval.LoadReport/LoadLock | 聚合 + baseline 引用 | ✓ WIRED | summary.md Reproducibility table carries model_ver=glm-5.2 + prompt_hash=58ffea08… from lock |
| golden spike rows | baseline rows | baseline_id 逐字回指 | ✓ WIRED | JSONL `baseline_id` fields + TestGoldenHistoryInjectionRows machine assertion |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| reports/*.jsonl (35 rows) | Metrics / refused / cost_usd | Real GLM-5.2 generation + Milvus tier-filtered retrieval + BGE-M3 embed + GLM judge | Yes — non-degenerate floats (e.g. faith 0.714-1.0, arel 0.79-0.89), real costs ($0.0481 cumulative), RFC3339Nano timestamps | ✓ FLOWING |
| reports/summary.json | decision / categories | computeVerdict over live JSONL + frozen baseline | Yes — per-category means + penetration counts parsed and consistent with JSONL | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Full test suite | `go test ./...` | exit 0, zero FAIL | ✓ PASS |
| Hermetic gate tests | `go test ./cmd/spike-history -v -count=1` | 10 cases PASS incl. 6 TestComputeVerdict gate scenarios | ✓ PASS |
| Dry-run smoke (isolated temp out-dir) | `go run ./cmd/spike-history --dry-run --out=/tmp/spike-verify --limit=2` | 2 rows, valid JSONL schema, exit 0 | ✓ PASS |
| Machine verdict readable | `python json.load(reports/summary.json)` | `decision: NO-GO`, cost 0.048139 | ✓ PASS |
| SC-3 penetration zero | `grep -c '"injection_penetrated":true' reports/*.jsonl` | 0 matches | ✓ PASS |
| Hard-constraint isolation | `git status --porcelain <protected paths>` | empty | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED — no `scripts/*/tests/probe-*.sh` convention exists in this project; the phase's runnable checks are the spike binary + test suite, both executed above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| GEVL-08 | 09-01..09-06 (all declare `requirements: [GEVL-08]`) | 历史注入回归硬门——golden set A/B，四指标漂移 <5% 且 refusal delta <10%；不过门则记忆不得发布，产出消毒规则或逃逸方案 | ✓ SATISFIED | Requirement's own wording permits the NO-GO path; both 消毒规则 (projection.go + 7 fixtures) and 逃逸方案 (D9-V2, user-signed) delivered; A/B ran live with recorded per-metric drift. REQUIREMENTS.md marks GEVL-08 Complete; traceability table consistent. |

Orphaned requirements: none — ROADMAP maps exactly GEVL-08 to Phase 9; all 6 plans claim it.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| cmd/spike-history/main.go | 120 | CR-01: default `--category=adversarial,followup` exact-matches **baseline** rows, not spike categories — silent wrong-corpus default run | ⚠️ Warning | Empirically CONFIRMED in this verification (isolated dry-run ran `adv-001/002`, wrote `adversarial.jsonl`). Did NOT corrupt the live run (its outputs carry `hist-*` spike rows with history fields — verified). Must fix before Phase 10 re-gate: a default re-run would burn live spend measuring nothing. |
| cmd/spike-history/runner.go | 445 | CR-02: live-mode chunk labels use dry-run base 9991..9995 — `grounding_pass` structurally false in all live rows | ⚠️ Warning | Confirmed in code + data (25 short live rows all `grounding_pass:false`). Weakens the grounding diagnostic in the report only; the NO-GO verdict rests on refusal collapse + faithfulness/context_recall (independent of citation labels); SC-3 penetration path independent (verified). Must fix before re-gate. |
| cmd/spike-history/summary.go | 781-853 | CR-03: model/prompt pin match claimed in header but never enforced (no AssertPin; SpikeResult carries no run identity) | ⚠️ Warning | Reproducibility table is echo-only; a different-model re-run would compare against the frozen baseline unchecked. Note: this "SC-4" label is the code file's own header numbering, not ROADMAP SC-4. |
| cmd/spike-history/runner.go | 121-142 | CR-04: `--cost-limit=0` disable semantics contradict (`Exceeded()` true at 0); final log overstates written count | ⚠️ Warning | Did not affect live run (explicit 5.00 limits); latent operator hazard. |
| internal/history/projection_test_fixtures.go | 112-131 | WR-04: `neg_history_truncated_to_three_pairs` Expected (4 msgs) contradicts Project() (6 msgs) | ⚠️ Warning | Verified by direct reading: after B1+B3 the output starts with the "old two" pair. Phase 10's first fixture-driven `_test.go` will fail on this row — loud, not silent, but the executable handoff contract ships one wrong datum. Fix the Expected, not Project(). |
| cmd/spike-history/long.go | 54 | `TODO(phase-10)` — B3 → summarization middleware | ℹ️ Info | Referenced formal follow-up (phase-10); passes debt-marker gate |
| .planning/ROADMAP.md | 316 | Phase 10 detail "Depends on: Phase 9（spike 硬门必须 go…）" not updated to reflect accept-NO-GO-with-escape | ℹ️ Info | Top-line ROADMAP entries (Phase 9 line + Progress table) DO record the acceptance; Phase 10's literal dependency string is stale — planner should read 09-06-SUMMARY escape section per the completion note |

Debt-marker gate: zero `TBD`/`FIXME`/`XXX` in all phase-modified source files (grep verified).

**Review disposition:** 09-REVIEW.md (committed `b1be190`, status issues_found: 4 Critical / 11 Warning / 10 Info) was independently spot-verified — CR-01 and CR-02 confirmed accurate (code + empirical + data); WR-04 confirmed by direct fixture reading. None of the 25 findings falsify a roadmap SC: the corrupted signals (grounding/citation columns) are diagnostics, while the verdict-driving signals (refusal — direct behavioral flag; faithfulness/context_recall — judge-computed from real retrieved contexts) are unaffected. All 4 criticals are mandatory pre-re-gate fixes tracked by the committed review + D9-V2's re-gate obligation.

### MVP Mode Note (formal discrepancy, non-blocking)

ROADMAP Phase 9 carries `Mode: mvp` but the goal is not in canonical User Story format (it is an experiment/hard-gate statement). Per `references/verify-mvp-mode.md` the user-flow framing does not fire; strictly, the user should run `/gsd mvp-phase 9` to reformat. Verification proceeded against the four objective Success Criteria (the roadmap contract) because: (a) this is a completed spike phase with `UI hint: no` — a User Flow Coverage table would be vacuous; (b) each plan (09-01..09-06) restates its goal in user-story form. Surfaced for the user to fix or accept.

### Human Verification Required

None remain. The phase's single human decision point — the blocking go/no-go checkpoint — was resolved during execution and is recorded with rationale (user signature 2026-08-18「接受 NO-GO 带逃逸方案」, STATE.md D9-V1/V2/V3, 09-06-SUMMARY Human-Verify Feedback section). All remaining claims in this report were verified programmatically against on-disk artifacts, code, git state, and test execution.

### Gaps Summary

No must-have truth failed; no artifact is missing or stub; no key link is unwired; live data flowed end-to-end. The phase delivered exactly what a spike hard-gate promises: a decisive, recorded, human-signed NO-GO with attributed root cause (refusal-behavior drift — Pitfall 1 H2, evidence chain uncontaminated per SC-3 zero penetration), shipped disinfection rules (B1/B2/B3 + 7 fixtures), and a selected escape plan with a mandatory re-gate before Phase 10 memory ships.

Open warnings requiring action before the Phase 10 re-gate (all tracked in committed 09-REVIEW.md): CR-01 wrong default corpus, CR-02 phantom chunk labels (grounding signal), CR-03 unenforced pin match, CR-04 cost-guard semantics, WR-04 fixture contradiction. Two scope clauses are explicitly deferred to Phase 10 (verdict-JSON rendering → MEM-04; refuse-or-annotate behavior → D9-V2 hardening).

---

_Verified: 2026-08-18T05:30:24Z_
_Verifier: Claude (gsd-verifier)_
