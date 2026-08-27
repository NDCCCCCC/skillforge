---
phase: 17
slug: p10-3
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-27
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Go test framework** | `testing` + `testify/assert` (Phase 16 precedent) |
| **TS test framework** | `vitest` + `@testing-library/react` (Phase 4 init) |
| **Verify script** | `scripts/verify-phase-17.sh` (NEW — 11-stage gate mirroring `verify-phase-16.sh`) |
| **Makefile target** | `verify-phase-17` (NEW — mirrors `verify-phase-16`) |
| **Quick run (Go hermetic)** | `go test ./internal/kpi/ ./internal/agent/tools/ ./internal/skill/ ./internal/wire/ ./internal/store/sqlite/ ./internal/eval/ -count=1` |
| **Full suite (Go)** | `go test .\... -count=1` (Windows: WSL retry for `./internal/kpi/wsl/`) |
| **Quick run (TS)** | `cd web && npx vitest run components/chat/parts/Sparkline.test.tsx components/chat/parts/DataReferenceChip.test.tsx` |
| **Full suite (TS)** | `cd web && npx vitest run && npx tsc --noEmit` |
| **Estimated runtime** | ~3–5 min (Go hermetic <60s, WSL live 2–3 min) |

---

## Sampling Rate

- **After every task commit (Hermetic tier):** `go test ./internal/kpi/ ./internal/agent/tools/ ./internal/skill/ ./internal/wire/ ./internal/store/sqlite/ ./internal/eval/ -count=1` + TS quick run
- **After every plan wave:** `bash scripts/verify-phase-17.sh` (11-stage gate — mirror verify-phase-16.sh Stages 1-11)
- **Before `/gsd:verify-work 17`:** Full suite must be green
- **Max feedback latency:** ~5 minutes (WSL live eval operator-gated; GLM-only probe required)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | DASH-01 | T17-V9 | SSE chunk wire discipline | unit | `go test ./internal/wire/ -run TestDecisionDataRefWireReferences -count=1` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | DASH-03 | T17-V9 | Sparkline no XSS, pure SVG | structural | `grep -rE "recharts\|react-sparklines\|chart.js" web/src/components/chat/Sparkline.tsx` → 0 | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | DASH-03 | — | sparkline hover point → popup renders | unit | `npx vitest run components/chat/parts/Sparkline.test.tsx -t "hover" --run` | ❌ W0 | ⬜ pending |
| 17-01-04 | 01 | 1 | DASH-03 | T17-V5 | data-reference part references[] array wire | unit | `go test ./internal/wire/ -run TestDecisionDataRefWire -count=1` | ✅ extend existing | ⬜ pending |
| 17-02-01 | 02 | 1 | DASH-02 | T17-V9 | Each number has channel name on hover | unit | `go test ./internal/agent/tools/ -run TestDataRefHasChannelName -count=1` | ❌ W0 | ⬜ pending |
| 17-02-02 | 02 | 1 | DASH-02 | — | TS popup renders channel name | unit | `npx vitest run components/chat/parts/DataReferenceChip.test.tsx -t "channel name" --run` | ❌ W0 | ⬜ pending |
| 17-03-01 | 03 | 1 | DASH-05 | T17-V11 | Multi-KPI answer each number independent | unit | `npx vitest run components/chat/MessageBubble.test.tsx -t "multi KPI" --run` | ❌ W0 | ⬜ pending |
| 17-03-02 | 03 | 1 | DASH-05 | T17-V4 | Skill steps routing → KPI tool call | unit | `go test ./internal/skill/ -run TestKPISkillStepsRoutingTable -count=1` | ❌ W0 | ⬜ pending |
| 17-04-01 | 04 | 2 | DASH-04 | — | Drill chip → prefills chat input | unit | `npx vitest run components/chat/parts/DrillChip.test.tsx -t "prefill" --run` | ❌ W0 | ⬜ pending |
| 17-04-02 | 04 | 2 | DASH-04 | — | ChatInput receives pendingDraft | unit | `npx vitest run components/chat/ChatInput.test.tsx -t "pendingDraft" --run` | ❌ W0 | ⬜ pending |
| 17-04-03 | 04 | 2 | DASH-04 | — | Drill reinvokes kpi_call tool | integration | `go test ./internal/agent/tools/ -run TestDrillChipReinvokesMultiPeriod -count=1` | ❌ W0 | ⬜ pending |
| 17-05-01 | 05 | 2 | GEVL-09 | T17-V5 | 50 golden cases pass 3-step judge | integration (eval CLI) | `go run ./cmd/eval --frozen=kpi --archive` (operator-gated; hermetic for CI) | ❌ W0 | ⬜ pending |
| 17-05-02 | 05 | 2 | GEVL-10 | T17-V5 | as_of + channel name in every answer | integration | same as GEVL-09 | ❌ W0 | ⬜ pending |
| 17-05-03 | 05 | 2 | GEVL-09/10 | T17-V8 | frozen lock byte-identity | structural | `git diff --quiet eval/frozen/kpi.lock.json && git diff --quiet eval/frozen/reid.jsonl` | ❌ W0 | ⬜ pending |
| 17-06-01 | 06 | 3 | DASH-01 | T17-V9 | E2E numeric answer end-to-end | integration (WSL) | `go test ./internal/kpi/wsl/multi_period_wsl_test.go -run TestEndToEndNumericAnswer -count=1` | ❌ W0 | ⬜ pending |
| 17-06-02 | 06 | 3 | DASH-04 | T17-V11 | Drill-down never reaches raw rows | structural | `go test ./internal/agent/ -run TestDrillDownNeverReachesRaw -count=1` | ❌ W0 | ⬜ pending |
| 17-06-03 | 06 | 3 | Pitfall 11 | T17-V8 | Join reidentification → suppressed | integration (eval) | `go run ./cmd/eval --frozen=kpi --subset=reid --attack=join` | ❌ W0 | ⬜ pending |
| 17-06-04 | 06 | 3 | Pitfall 22 | T17-V8 | KPI differential → suppressed/min count | integration (eval) | `go run ./cmd/eval --frozen=kpi --subset=reid --attack=differential` | ❌ W0 | ⬜ pending |
| 17-07-01 | 07 | 3 | DASH-03 | — | sparkline autoscale | unit | `npx vitest run components/chat/parts/Sparkline.test.tsx -t "autoscale" --run` | ❌ W0 | ⬜ pending |
| 17-07-02 | 07 | 3 | DASH-03 | T17-V9 | TS wire types lockstep | structural | `cd web && npx tsc --noEmit` | ❌ W0 | ⬜ pending |
| 17-07-03 | 07 | 3 | GEVL-09 | T17-V3 | kpi.lock byte-identity against source | structural | `grep kpi_registry_sha256 eval/frozen/kpi.lock.json` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

### Go stubs (33 files)
- [ ] `internal/kpi/multi_period.go` — multi-period resolve + sort + assembly
- [ ] `internal/kpi/multi_period_test.go` — hermetic unit tests
- [ ] `internal/kpi/wsl/multi_period_wsl_test.go` — live engine integration for multi-period
- [ ] `internal/skill/loader_kpi_test.go` — TestKPISkillStepsRoutingTable (fenced YAML parse + KPI name cross-check)
- [ ] `internal/agent/tools/kpi_tool_test.go` — TestDataReferenceArrayPart + TestDrillChipReinvokesMultiPeriod
- [ ] `internal/wire/chunk_test.go` — TestDecisionDataRefWireReferences extension
- [ ] `internal/store/sqlite/kpi_calls_test.go` — TestMultiPeriodKPIPeriodsArray extension
- [ ] `internal/eval/kpi_metrics.go` — TestThreeStepValueMatch + TestJudgeKPIReferenceConsistency
- [ ] `internal/eval/kpi_metrics_test.go` — programmatic 3-step match unit tests
- [ ] `internal/agent/tools/refusal_test.go` — TestRefusalTaxonomy extension for `periods` shape
- [ ] `internal/agent/sql_surface_test.go` — TestNoSQLGenerationSurface kpi-dashboard root extension
- [ ] `internal/agent/channel_guard_test.go` — TestReadsChannelGuardXOR + TestSingletonRegistry
- [ ] `internal/store/sqlite/kpi_calls_test.go` — TestKPICallsInsertOnly + TestKPICallsAuditRowHasSessionID

### TS stubs (10 files)
- [ ] `web/src/components/chat/parts/Sparkline.tsx` — pure SVG (zero chart library deps)
- [ ] `web/src/components/chat/parts/Sparkline.test.tsx` — TestSparklinePureSVG / HoverPoint / Autoscale / NoDeps
- [ ] `web/src/components/chat/parts/DataReferenceChip.tsx` — inline number hover wrapper
- [ ] `web/src/components/chat/parts/DataReferenceChip.test.tsx` — TestChannelName / TestPopupRenders
- [ ] `web/src/components/chat/parts/DataReferencePopup.tsx` — shared popup (single value + sparkline point)
- [ ] `web/src/components/chat/parts/DrillChip.tsx` — Badge-based chip
- [ ] `web/src/components/chat/parts/DrillChip.test.tsx` — TestPrefill / TestBadgeRenders
- [ ] `web/src/components/chat/MessageBubble.tsx` — data-decision kpi_reference branch
- [ ] `web/src/components/chat/ChatInput.tsx` — useEffect reader for pendingDraft
- [ ] `web/src/store/app-store.ts` — `pendingDraft` field + setter

### Eval data files (5 files)
- [ ] `eval/golden/kpi.jsonl` — 50 cases (query → expected_kpi + expected_value + allowed_expressions + expected_period)
- [ ] `eval/frozen/kpi.lock.json` — 5+1 pinned fields (D17-D4; kpi_registry_sha256)
- [ ] `eval/frozen/reid.jsonl` — Pitfall 11/22 attack cases (D17-D3)

### Config/infra (3 files)
- [ ] `web/src/types/wire-types.ts` — `DataReference.references?: PointReference[]` + TS AssertAssignable
- [ ] `scripts/verify-phase-17.sh` — 11-stage hermetic gate (mirror verify-phase-16.sh)
- [ ] `Makefile` — `verify-phase-17` target

### Data fixtures (2 files)
- [ ] `data/staging/testdata.duckdb` — synthetic 10K-row fixture (pre-build + commit per Phase 16 precedent)
- [ ] `kpis/testdata.yaml` — mini KPI registry (3–5 KPI definitions)

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | n/a (chat auth unchanged from Phase 4) |
| V3 Session Management | yes | `session_id` stamped on `kpi_calls` audit row |
| V4 Access Control | yes | `data:aggregated_only` reads channel + singleton registry (D16-C1/C4) |
| V5 Input Validation | yes | Programmatic 3-step numeric match in GEVL-09 (no LLM judge) |
| V6 Cryptography | no | n/a |
| V7 Error Handling | yes | Refusal taxonomy (no_matching_kpi / invalid_params / dataset_not_available / result_too_large) |
| V8 Data Protection | yes | INSERT-only audit + kpi_calls signature (KPI-02) + reid.jsonl frozen assertions |
| V9 Communication | yes | SSE chunk wire discipline (D16-B3 + D17-B2 — additive field) |
| V11 Business Logic | yes | D14 platform-floor composition re-asserted at chat + deep path for kpi-dashboard |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Synthetic testdata leak via eval prompt | Info Disclosure | Golden set queries never reference real data (D17-D1) |
| KPI join reidentification (Pitfall 11) | Info Disclosure | k_min row suppression + reid.jsonl join-attack assertions (D16-D1/D2) |
| KPI differential attack (Pitfall 22) | Info Disclosure | enum/pattern constraint on every param (D16-A3) + reid.jsonl diff assertions |
| LLM hallucinating a number | Tampering | GEVL-10 programmatic judge + as_of + channel name token presence |
| LLM fabricating KPI name | Tampering | Tool catalog Enum locks LLM to `reg.Names()` (D16-B1) |
| LLM attempting SELECT raw | Tampering | Platform floor "永不编写/执行/建议 SQL" (D14-A1) + WalkDir gate |
| Routing table poisoned | Tampering | TestKPISkillStepsRoutingTable cross-validates KPI names at boot |
| Sparkline XSS via period string | Tampering | Periods constrained to `^\d{4}-(0[1-9]|1[0-2])$` (loader static check) |
| Chip prefill injection | Tampering | `pendingDraft` plain text; ChatInput renders via safe textarea |

### ASVS Verification by Phase 17 Tests

| ASVS | Verification |
|------|-------------|
| V3 (session) | `TestKPICallsAuditRowHasSessionID` (extend existing Phase 16) |
| V4 (access control) | `TestReadsChannelGuardXOR` + `TestSingletonRegistry` |
| V5 (input validation) | `TestGEVL09GoldenSet50` + `TestThreeStepValueMatch` |
| V7 (error handling) | `TestRefusalTaxonomy` (extend Phase 16 for `periods` shape) |
| V8 (data protection) | `TestKPICallsInsertOnly` + `TestReidFrozenSubset` |
| V9 (communication) | `TestDecisionDataRefWire` (References extension) + `TestWireTypeCompileAlarm` |
| V11 (business logic) | `TestPlatformFloorPrependBothPaths` (existing) — covers kpi-dashboard chat path |

---

## Validation Sign-Off

- [ ] All tasks have automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all ❌ references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5 min
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
