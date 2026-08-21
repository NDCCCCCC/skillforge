# Plan 13-04 — Hermetic Gate + Frozen Eval v2 + STRIDE-13 + WalkDir Extension — SUMMARY

**Phase**: 13 (P8.4 — 子 agent 扇出 — tier 继承)
**Plan**: 13-04 (Wave 3)
**Status**: ✅ COMPLETE — 9/9 hermetic gate stages PASS
**Date**: 2026-08-21
**Operator**: Phase 13 acceptance batch commit pending; live UAT deferred to Plan 13-05

---

## Goal

Land the Phase 13 hermetic acceptance gate that byte-identity-verifies
all FAN-01..04 + P13-L1..L9 invariants without operator involvement.
The operator-runs-live gate (Plan 13-05) measures actual cost /
faithfulness / tier-closure regression rate against
`eval/frozen/fanout.lock.json` and signs off the v0.8 milestone.

This plan achieves the goal by delivering 4 artifacts:

1. **`scripts/verify-phase-13.sh`** — 9-stage hermetic gate (the
   only place FAN-01..04 + P13-L1..L9 are validated byte-identity).
2. **`eval/frozen/fanout.lock.json`** — Phase 13 v0.8 milestone
   re-baseline lock file (placeholder; operator fills at 13-05).
3. **`docs/threat-model/STRIDE-13.md`** — 8-threat register
   T-P13-1..T-P13-8 mapped to named test gates.
4. **`integration/contract/no_legacy_interrupt_test.go` WalkDir
   extension** — `phase11GoScanRoots` extended from 6 (Phase 12)
   to 8 (Phase 13) entries, covering the new fan-out files
   (`internal/store/sqlite` + `internal/wire`).

---

## Tasks Completed (4/4)

### Task 1: `scripts/verify-phase-13.sh` — 9-stage hermetic gate

Created `scripts/verify-phase-13.sh` (mode 0755). Mirrors the
`scripts/verify-phase-12.sh` pattern (run_stage helper + log capture
to `/tmp/phase-13-stage-NN.log` + `[STAGE-NN-PASS]/[STAGE-NN-FAIL]`
prefixes + banner "PHASE 13 HERMETIC GATE: PASS").

**Stages**:

| Stage | Description | Verification |
|-------|-------------|--------------|
| 1 | `go build ./... && go vet ./...` | Foundation safety net (catches type drift before any test runs) |
| 2 | Hermetic Go test suite (FAN-01..04) | `go test ./internal/runtime/... ./internal/skill/... ./internal/agent/... ./internal/agent/callbacks/... ./internal/agent/hitl/... ./internal/store/... ./internal/wire/... ./internal/verdict/... ./internal/server/... ./cmd/spike-fanout/... -count=1 -timeout=300s` — `-race` omitted per Phase 12 precedent (cgo + gcc requirement) |
| 3 | WalkDir legacy interrupt guard (D8-04 extended; 8 roots) | `go test ./integration/contract/... -tags=contract -run 'TestNoLegacyInterruptAPI\|TestNoLegacyInterruptAPIPhase11'` |
| 4 | `adk.NewParallelAgent` ban source-grep (P13-L1 sealed sentinel) | `grep -rn 'adk\.NewParallelAgent(' internal/runtime/` + sealed sentinel present check |
| 5 | D-04 transport-swap byte-identity with `data-decision fanout_*` enum extended | `go test ./internal/wire/...` — byte-identity contract preserved through D16 overload |
| 6 | D15 wire-type compile alarm (npx tsc --noEmit) | `cd web && npx tsc --noEmit` — TS+Go lockstep on `data-decision.decision` enum |
| 7 | Frozen eval byte-identity (compliance.lock.json + fanout.lock.json sha256) | `sha256sum` both lock files; compare against STATE.md recorded values |
| 8 | Sub-agent budget half-of-parent (P13-L3 source-grep; MaxIteration=5 + TokenBudget=20000) | `grep` literals in `internal/runtime/fanout_runtime.go` |
| 9 | SharedChunkAccumulator LRU cap (P13-L9; Pitfall 6) | `go test ./internal/runtime/... -run 'TestSharedChunkAccumulator*'` |

**Output (executed 2026-08-21T01:42:38Z)**:

```
===================================================================
PHASE 13 HERMETIC GATE: PASS
===================================================================
All 9 stages passed. Phase 13 fan-out fan-in architecture
(FAN-01..04) hermetically verified:
  - Tier-closure structural inheritance (P13-L2)
  - Deterministic 10× byte-identity merge (P13-L4)
  - Conservative partial-failure merge (P13-L5)
  - Per-sub-agent grounding via ApplyTrustStage (P13-L9)
  - D16 data-decision overload (P13-L7) — fanout_* enum
  - D15 TS lockstep + D-04 transport-swap byte-identity
  - Frozen eval byte-identity (compliance.lock.json UNCHANGED)
  - adk.NewParallelAgent ban (P13-L1 sealed sentinel)
```

**Adjusted during execution**:
- **Stage 2 —race removed**: Phase 12 `verify-phase-12.sh` already
  documents why -race is omitted (cgo + gcc requirement; Windows
  hosts without gcc cannot run it). Phase 13 follows the same
  precedent. Operator-runs-live with -race documented separately
  in the gate header.
- **Stage 4 grep regex broadened**: The original grep pattern
  `禁止 adk.NewParallelAgent` did not match the actual sentinel
  text in `internal/runtime/orchestrator.go` (which uses "DO NOT
  introduce \`adk.NewParallelAgent\`" in English). Broadened to
  `grep -qE '禁止 adk\.NewParallelAgent|DO NOT introduce \`adk\.NewParallelAgent'`
  to match both Chinese + English sealed-sentinel variants.

### Task 2: `eval/frozen/fanout.lock.json` — Phase 13 v0.8 milestone re-baseline placeholder

Created `eval/frozen/fanout.lock.json` with:
- `lock_id: "fanout-2026-08-21"`
- `model_version: "v0.8"` (Phase 13 is a v0.8 milestone rebaseline)
- `phase: 13`
- `eval_set: "eval/golden/fanout.jsonl"`
- `eval_set_sha256: "<computed at 13-05 acceptance gate operator sign-off>"`
  (placeholder — operator fills at 13-05)
- `case_count: 20`
- `category_counts: { multi_hop: 8, tier_mismatch_rejection: 6, partial_failure_conservative_merge: 4, single_vertex_fallback: 2 }`
- `thresholds: { fanout_dispatched_count_floor: 20, conservative_merge_rate_max: 0.0, tier_closure_regression_rate_max: 0.0, min_confidence_aggregation_min: 0.0, tier_mismatch_rejection_rate_min: 1.0 }`
- `agent_archetype: "orchestrator"`
- `sub_agent_budget_frozen: true`
- `sub_agent_max_iteration: 5`
- `sub_agent_token_budget: 20000`
- `shared_chunk_accumulator_max: 20`
- `phase_8_pin_unchanged: "5cf700f3..."` (T-8-12 regression lock)
- `metrics.*`: all null (operator fills at 13-05)
- `operator_signed_at`, `operator_name`, `live_fanout_md_sha256`,
  `lock_sha256`: all null (operator fills at 13-05)
- `_operator_action_required`: protocol note for Plan 13-05

**T-8-12 invariant**: `compliance.lock.json` (Phase 8 pin) is
UNCHANGED. A silent edit to either lock file (without re-running the
spike and operator sign-off) surfaces as a sha256 drift → the
acceptance gate exits 1.

**Updated `eval/frozen/README.md`**:
- Added `fanout.lock.json` row to Files table (Phase 13 / 13-04 /
  13-05; 20-case `eval/golden/fanout.jsonl` with sub-agent budget
  + accumulator cap recorded).
- Added `## fanout.lock.json (Phase 13 v0.8 milestone re-baseline)`
  section with full schema + What is locked list (5 behaviors:
  tier-closure + 10× byte-identity merge + conservative partial-
  failure + per-sub-agent grounding + D16 overload) + byte-identity
  verification command + reproduce command (Plan 13-05).
- Updated "Byte-Identity Verification" section to document the 4-
  lock verification (compliance + hitl + fanout + deep).

**Updated `Makefile`**:
- Added `.PHONY: verify-phase-13` (alongside existing
  `verify-phase-11` + `verify-phase-12`).
- Added `verify-phase-13` target: `bash scripts/verify-phase-13.sh`.

### Task 3: `docs/threat-model/STRIDE-13.md` — 8-threat register

Created `docs/threat-model/STRIDE-13.md` (Phase 13 introduction;
mirrors STRIDE-12 structure):

| Section | Content |
|---------|---------|
| §1 Scope & Trust Boundaries | 6 new boundaries (dispatchAgent factory → FanOutOrchestrator + BuildSubAgent; ApplyTrustStage → verdict primitives; runSubAgent per-goroutine errgroup bounded; mergeSubAgentResults pure function; audit data-decision D16 overload; QueryFanoutByRunPath helper); 2 inherited (Phase 11 chatHandler, Phase 12 wire-type discrimination) |
| §2 STRIDE Register | 8 threats T-P13-1..T-P13-8 mapped to named test gates (all Critical/High) |
| §3 Critical Threats Full Descriptions | T-P13-1 + T-P13-2 + T-P13-3 + T-P13-4 with stakes / component / mitigation / verification |
| §4 High Threats Paragraph Summaries | T-P13-5 + T-P13-6 + T-P13-7 + T-P13-8 paragraph summaries |
| §5 Trust Boundaries — Detail | New + inherited boundaries; fan-out specific |
| §6 Cross-References | D11-A2 + D11-B1 + D11-B3 + D11-C3 (Phase 11 preserved) + P12-L1 + P12-L2 + P12-L6 + P12-L9 (Phase 12 preserved) + P13-L1..L9 (new Phase 13 locks) |
| §7 Security Domain Controls (ASVS V2..V8) | Inherited V2..V10 controls; Phase 13 adds V3 (subSessionID), V4 (tier-closure + empty Writes), V5 (two-layer tier-filter validation), V7 (defer recover + conservative merge), V10 (sealed sentinel + budget cap) |
| §8 Acceptance Criteria | 8 threats each with named test + Stage gate; single failure reopens Critical threat to production |
| §9 Open Risks (Deferred to Phase 14 / v1.1) | Cross-Skill plan replay with fan-out; sub-agent write approval rules; async sub-agent approval; quorum / delegation; cross-tenant fan-out |
| §10 Change Log | 2026-08-21 initial STRIDE register; reviewed quarterly by Internal Audit Specialist |

**Threat summary**:

| Threat | Category | Component | Mitigation | Test |
|--------|----------|-----------|------------|------|
| T-P13-1 | EoP | BuildSubAgent tier intersection mismatch | Two-layer defense (load-time intersection + Tool validator) | TestSubAgentTierInheritance + TestSubAgentCannotExpandTierBoundary |
| T-P13-2 | EoP | HITL whitelist leak | `subManifest.Writes = []string{}` AT CONSTRUCTION | TestSubAgentIsReadOnly + TestSubAgentToolWhitelistExcludesHITL |
| T-P13-3 | DoS | Sub-agent fault propagation | `defer recover()` + `context.WithTimeout(30s)` + `errgroup.SetLimit(4)` | TestFanOutOrchestratorFaultIsolation + TestFanOutOrchestratorSubAgentTimeout |
| T-P13-4 | Repudiation | Merge function non-determinism | PURE function + `sort.SliceStable` + 10× byte-identity | TestMergeSubAgentResultsByteIdentity (10×) + TestMergeSubAgentOrderIndependent |
| T-P13-5 | DoS | Classifier cost explosion | Hardcoded `MaxIteration=5` + `TokenBudget=20000` + pre-fanout budget check | TestSubAgentBudgetHalfOfParent + Stage 8 source-grep |
| T-P13-6 | DoS | Shared accumulator growth | LRU cap 20 + dedup + oldest-first | TestSharedChunkAccumulatorLRUCap + Dedup + HitsOrder |
| T-P13-7 | Tampering | Sub-agent manifest wiring typo | SubAgentConfig type-safe struct (ParentManifest vs SubSkillManifest fields distinct) | TestSubAgentRetrieveToolClosesOverSubManifest |
| T-P13-8 | Tampering | `adk.NewParallelAgent` accidental use | Sealed sentinel + source-grep | Stage 4 |

### Task 4: STATE.md closeout + WalkDir extension + `runner_setup_test.go` (UNCHANGED)

**WalkDir extension (`integration/contract/no_legacy_interrupt_test.go`)**:

`phase11GoScanRoots` extended from 6 (Phase 12 union) to 8 (Phase 13 union):
- Phase 12 union (6 roots):
  1. `internal/agent`
  2. `internal/agent/callbacks`
  3. `internal/agent/tools`
  4. `internal/agent/hitl`
  5. `internal/server`
  6. `internal/runtime`
- **Phase 13 union additions (2 roots)**:
  7. `internal/store/sqlite` — covers `internal/store/sqlite/audit.go`
     + `internal/store/sqlite/fanout_events.go` (P13-L6 audit RunPath/
     Transfer columns via ADD COLUMN DDL; sub-agent audit writer
     uses Phase 11 audit_writer.go pattern — sync INSERT, no interrupts
     since FanOutOrchestrator is fire-and-forget).
  8. `internal/wire` — covers `internal/wire/chunk.go` (D16 overload
     extends data-decision discriminator with `fanout_dispatched |
     fanout_completed | fanout_failed`; wire package MUST stay
     interrupt-free — server-emit-only SSE invariant).

`phase11LegacyRunnerQueryCarveOut` UNCHANGED (pre-Phase-11 legacy
seam in `internal/server/pipeline.go:438`; tracked for Phase 12+
migration; not in Phase 13 scope).

`phase11AllowedStatefulInterruptFiles` UNCHANGED (HITL Tool files
only; Phase 13 sub-agents are structural read-only via P13-L2 +
P13-L4 — they cannot reach HITL Tools).

**`internal/agent/runner_setup_test.go` (UNTOUCHED)**:

A second copy of `TestNoLegacyInterruptAPI` exists here with 5
roots (includes `internal/runtime` per 13-01). Per plan instruction
"modify in place whichever file contains the canonical WalkDir",
the integration/contract version was modified since that is the
Phase 12 pattern. The runner_setup_test.go copy is preserved as-is
for backward compatibility (the 5 roots are a superset of the
Phase 8 ban list coverage; Phase 13 extensions are tested via the
integration/contract version which scans the broader Phase 13 union).

**STATE.md closeout block**:

Appended Phase 13 / Plan 13-04 closeout block to
`.planning/STATE.md` (lines 354-434):
- 9/9 gate stages PASS attested
- 4 FAN requirement IDs each with verifying test + Stage gate
- 9 P13-L1..L9 lock attestations each with mitigation + Stage gate
- 8 STRIDE-13 threats attested (T-P13-1..T-P13-8)
- 8 atomic commit files listed
- sha256 values recorded for: `compliance.lock.json` (Phase 8 pin
  UNCHANGED) + `fanout.lock.json` (Phase 13 v0.8 placeholder) +
  STRIDE-13.md + verify-phase-13.sh + no_legacy_interrupt_test.go
  + Makefile
- Sign-off line: `Operator signature: ___   Date: 2026-08-21`

---

## Files Modified (8 total)

| File | Change | sha256 |
|------|--------|--------|
| `scripts/verify-phase-13.sh` | NEW (9-stage gate) | `eefc4df0b1cf3d88f6743e56c89b25cda1bf763bb5d6dfa71d3282713da5e8cb` |
| `docs/threat-model/STRIDE-13.md` | NEW (8-threat register) | `db0b3dc92e5bfc174239dd1fb556cd953e60b08d263117fd30bbb8bb8d0fd548` |
| `eval/frozen/fanout.lock.json` | NEW (Phase 13 v0.8 placeholder) | `2fca4ee25c11f57a17e3628bdd759d6618d9508eebb12b135f5294630da1b4ee` |
| `eval/frozen/README.md` | MODIFIED (added fanout.lock.json row + 4-lock verification + §fanout.lock.json section) | — |
| `integration/contract/no_legacy_interrupt_test.go` | MODIFIED (`phase11GoScanRoots` extended 6→8 roots) | `b9b0677ea8ca4ad2145eaabcade20a5361ae77856a92951527259a1422a55dc8` |
| `Makefile` | MODIFIED (added `verify-phase-12` + `verify-phase-13` targets) | `9f6babe29dc744775336b47d2d371e1a33229d86172a906593947f1a2e4e6678` |
| `.planning/STATE.md` | MODIFIED (Phase 13 closeout block appended, lines 354-434) | — |
| `.planning/phases/13-p8-4-sub-agent-fan-out/13-04-SUMMARY.md` | NEW (this file) | — |

**Files preserved (UNTOUCHED)**:
- `internal/agent/runner_setup_test.go` — second WalkDir copy with
  5 roots (Phase 8 union); integration/contract version is the
  canonical Phase 12 pattern.
- `eval/frozen/compliance.lock.json` — Phase 8 pin (T-8-12 regression
  lock; sha256 `5cf700f3...` UNCHANGED).

---

## Success Criteria

| Criterion | Status |
|-----------|--------|
| All 9 hermetic gate stages PASS | ✅ PASS |
| `compliance.lock.json` (Phase 8 pin) UNCHANGED (T-8-12) | ✅ PASS — sha256 `5cf700f3...` byte-identical |
| `fanout.lock.json` placeholder created + byte-identity verifiable | ✅ PASS — sha256 `2fca4ee25c...` recorded |
| WalkDir 8 roots cover Phase 13 fan-out files | ✅ PASS — `internal/store/sqlite` + `internal/wire` added |
| STRIDE-13 has 8 Critical/High threats mapped to named tests | ✅ PASS — T-P13-1..T-P13-8 each with named test |
| Makefile has `verify-phase-13` target | ✅ PASS |
| STATE.md closeout block appended with sha256 values | ✅ PASS |
| All 8 deliverables documented + sha256 recorded | ✅ PASS |
| 13-04 SUMMARY.md written | ✅ PASS (this file) |
| 9-stage gate logs to `/tmp/phase-13-stage-NN.log` | ✅ PASS |
| `-race` documented as omitted (Phase 12 precedent; cgo env constraint) | ✅ PASS |
| Sealed sentinel regex broadened to match English + Chinese variants | ✅ DONE |
| Atomic commit pending (operator-driven) | ⏳ PENDING — see "Next action" below |

---

## Deviations

1. **Stage 2 `-race` removed** — `go test -race` requires cgo + gcc;
   Windows hosts without gcc surface "cgo: C compiler gcc not found"
   and fail every package. Phase 12 already documented this; Phase 13
   follows the same precedent. Operator-runs-live with -race is
   documented separately in the gate header (Linux/macOS operators
   with gcc installed can run `-race` for the same suite).

2. **Stage 4 sealed-sentinel regex broadened** — The Chinese literal
   `禁止 adk.NewParallelAgent` did not match the actual sentinel text
   in `internal/runtime/orchestrator.go` (which uses the English
   variant `DO NOT introduce \`adk.NewParallelAgent\``). Broadened
   regex to `grep -qE '禁止 adk\.NewParallelAgent|DO NOT introduce
   \`adk\.NewParallelAgent'` to match both variants.

3. **WalkDir extension interpreted** — Plan §13-04 Task 4 specified
   adding 4 specific file paths (`internal/runtime/orchestrator.go`
   etc.) but the WalkDir test scans directory roots, not file paths.
   Interpreted sensibly by adding `internal/store/sqlite` +
   `internal/wire` (covers 6→8 roots; covers the relevant files:
   `fanout_events.go` + `audit.go` in sqlite/ and `chunk.go` in wire/).

4. **Two WalkDir copies preserved** — `TestNoLegacyInterruptAPI`
   exists in both `integration/contract/no_legacy_interrupt_test.go`
   AND `internal/agent/runner_setup_test.go` (the latter with 5
   roots; Phase 8 union only). Per plan instruction "modify in
   place whichever file contains the canonical WalkDir", the
   integration/contract version was modified; runner_setup_test.go
   preserved (its 5 roots are a Phase 8 superset; the Phase 13
   extensions are tested via the integration/contract version).

---

## Next Action

**Operator manually commits the 13-04 batch atomically**:

```bash
cd /d/code/ClaudeCode/ai-agent
git add scripts/verify-phase-13.sh \
        docs/threat-model/STRIDE-13.md \
        eval/frozen/fanout.lock.json \
        eval/frozen/README.md \
        integration/contract/no_legacy_interrupt_test.go \
        Makefile \
        .planning/STATE.md \
        .planning/phases/13-p8-4-sub-agent-fan-out/13-04-SUMMARY.md
git commit -m "docs(phase-13): 13-04 hermetic gate + STRIDE-13 + WalkDir 8 roots + fanout.lock.json

Plan 13-04 (Wave 3) — Phase 13 acceptance gate + threat model + frozen
eval placeholder. 9-stage hermetic gate (scripts/verify-phase-13.sh)
verifies all FAN-01..04 + P13-L1..L9 invariants byte-identity. STRIDE-13
threat model (docs/threat-model/STRIDE-13.md) registers 8 Critical/High
threats T-P13-1..T-P13-8. fanout.lock.json (eval/frozen/) is the Phase 13
v0.8 milestone re-baseline placeholder — operator populates metrics +
actual sha256 values at 13-05 live UAT. WalkDir phase11GoScanRoots
extended from 6 (Phase 12) to 8 (Phase 13) entries — covers
internal/store/sqlite (audit + fanout_events) + internal/wire (chunk.go
D16 overload).

Signed-off: 2026-08-21
Co-Authored-By: Claude <noreply@anthropic.com>"

git push origin master
```

**Then Plan 13-05 live UAT** (operator-gated; v0.8 milestone close):

```bash
GLM_API_KEY=$GLM_API_KEY \
  MILVUS_ADDRESS=$MILVUS_ADDRESS \
  BGE_M3_URL=$BGE_M3_URL \
  go run ./cmd/spike-fanout --live \
    --golden=eval/golden/fanout.jsonl \
    --sub-skill-config=examples/skills/compliance-orchestrator/SKILL.md \
    --cost-limit=2.00 \
    --seed=42 \
    --summarize

# Operator populates metrics.* + actual eval_set_sha256 +
# operator_signed_at + operator_name + live_fanout_md_sha256 in
# fanout.lock.json, computes lock_sha256, re-runs verify-phase-13.sh
# Stage 7 to verify, commits lock.
```

Phase 13 v0.8 milestone re-baseline closes at 13-05 operator sign-off.