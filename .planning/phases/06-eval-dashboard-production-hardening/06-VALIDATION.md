---
phase: 06
slug: eval-dashboard-production-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-13
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Derived from `06-RESEARCH.md` § Validation Architecture. Every PLAN.md task SHOULD reference an automated verify from the map below; the planner ensures no task lacks a verify path.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (BE)** | Go `testing` + `stretchr/testify` v1.11.1 (existing) |
| **Framework (FE)** | Vitest 2.x + @testing-library/react 16 (existing — `web/package.json`) |
| **Config file (BE)** | none (standard `go test`) |
| **Config file (FE)** | `web/vitest.config.ts` (existing) |
| **Quick run command** | `go test ./internal/... ./cmd/... && cd web && npx tsc --noEmit && npx vitest run` |
| **Full suite command** | `go test -race ./... && cd web && npx tsc --noEmit && npx vitest run && npx vite build` |
| **Estimated runtime** | quick ~30s · full ~1–3m |

*Phase 6 adds ZERO new Go dependencies. The only FE dependency decision (recharts / router) is gated by a `checkpoint:human-verify` before install (slopcheck was unavailable during research).*

---

## Sampling Rate

- **After every task commit:** Run the quick command (hermetic unit tests — includes the SC-6 strict `1e-6` layer, redaction/audit/gate unit tests, FE component tests; EXCLUDES the 50×5 concurrency test and the `liveeval`-tagged jitter test).
- **After every plan wave:** Run the full suite with `-race` (INCLUDES `TestSessionIsolation50x5` — the `-race` flag is mandatory for SC-4). Excludes `-tags liveeval`.
- **Before `/gsd:verify-work`:** Full suite green + the two phase-gate checks below (SC-2 sacrificial PR, SC-6 live jitter).
- **Max feedback latency:** ~30s per task commit · ~3m per wave merge.

**Phase-gate-only checks (not per-commit/per-wave — quota or runtime reasons):**
1. **SC-2 (CI gate wiring):** open a sacrificial PR that drops a Skill's faithfulness below threshold → confirm the workflow fails + the `gh pr comment` names the Skill + threshold. The *gate logic* is unit-tested per-commit as a pure function (`gateDecision`); the *Actions wiring* can only be verified by running a workflow.
2. **SC-6 (live jitter layer):** `go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility` once with `GLM_API_KEY` set (or `cmd/eval --frozen` twice + diff). Guarded by `//go:build liveeval` so it never runs in the default suite.

---

## Per-Requirement / Per-SC Verification Map

> Task IDs (`06-XX-YY`) are assigned by the planner. Plan/Wave columns fill in once PLAN.md files exist; the Test Type + Automated Command are fixed by this contract.

| Req / SC | Behavior | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|----------|----------|------------|-----------------|-----------|-------------------|-------------|--------|
| GEVL-06 / SC-1 | Dashboard renders Skills table (per-Skill faith/cp/ar/refusal_acc + ❌/✅ badge) from `/api/eval/reports` | — | N/A | FE unit | `cd web && npx vitest run src/components/eval/SkillsTable.test.tsx` | ❌ W0 | ⬜ pending |
| GEVL-06 / SC-1 | Trend panel renders line + dashed frozen baseline + >5% drift flag | — | N/A | FE unit | `cd web && npx vitest run src/components/eval/TrendPanel.test.tsx` | ❌ W0 | ⬜ pending |
| GEVL-06 / SC-1 | `/api/eval/reports` serves INDEX.json; `/api/eval/reports/:ts` serves one report | T-6-PATH | `:ts` regex-validated `^[0-9T:-]+Z$` before fs read | BE unit | `go test ./internal/server/ -run TestEvalReports` | ❌ W0 | ⬜ pending |
| GEVL-07 / SC-2 | `gateDecision(report, thresholds)` fails when ANY Skill faithfulness < threshold; names the Skill | — | N/A | BE unit (pure fn) | `go test ./internal/eval/ -run TestGateDecision` | ❌ W0 | ⬜ pending |
| GEVL-07 / SC-2 | PR comment markdown table on failure; report.json+calibration.json uploaded | T-6-SECRET | `GLM_API_KEY` never in logs/artifacts | manual / CI-only | sacrificial PR (phase gate) | manual | ⬜ pending |
| SC-3 | `/api/audit?user=&skill=&since=&until=&refusal_reason=&limit=&offset=` maps to QueryAudit + paginates | T-3-SQLI | params bound via parameterized AuditFilter; limit/offset integer-clamped | BE unit | `go test ./internal/server/ -run TestAuditHandler` | ❌ W0 | ⬜ pending |
| SC-3 | Audit table renders rows + filter bar | — | N/A | FE unit | `cd web && npx vitest run src/components/eval/AuditTable.test.tsx` | ❌ W0 | ⬜ pending |
| SC-3 | SQLi regression still holds (DROP TABLE in params survives) | T-3-SQLI | reuse `store.QueryAudit`; never build SQL in handler | BE regression | `go test ./internal/store/sqlite/ -run TestSQLi` | ✅ (Phase 3) | ⬜ verify-green |
| SC-4 | 50 sessions × 5 turns: each GetSession(sessN) has exactly 10 msgs; no cross-session query leakage | T-3-ISO | D18 `WHERE session_id=?` partition | BE integration (`-race`) | `go test -race ./internal/server/ -run TestSessionIsolation50x5 -count=1` | ❌ W0 (scales existing) | ⬜ pending |
| SC-5 | Rows past retention → Query + VerdictJSON redacted (non-admin); raw (admin token) | T-6-IDOR | read-time projection only; INSERT-only invariant preserved | BE unit | `go test ./internal/server/ -run TestRetentionRedact` | ❌ W0 | ⬜ pending |
| SC-5 | `SKILLFORGE_RETENTION_DAYS` override + unset `SKILLFORGE_ADMIN_TOKEN` → all non-admin | T-6-TOKEN | constant-time token compare | BE unit | `go test ./internal/server/ -run TestAdminToken` | ❌ W0 | ⬜ pending |
| SC-6 | StubJudge strict layer: `abs(run1-run2) < 1e-6` (strengthened from 1e-9) | — | N/A | BE unit (hermetic) | `go test ./internal/eval/ -run TestRunnerReproducibility` | ✅ (Phase 3 — MODIFY) | ⬜ pending |
| SC-6 | Frozen lock `AssertPin` detects model_ver / prompt_hash drift | — | N/A | BE unit | `go test ./internal/eval/ -run TestAssertPin` | ❌ W0 | ⬜ pending |
| SC-6 | Live jitter layer: `abs(run1-run2) < 1e-3` (real judge) | — | N/A | BE integration (`liveeval`) | `go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility` (phase gate) | ❌ W0 | ⬜ pending |
| D6-14 | Cross-eval `--judge-models a,b` writes 2 reports; >5% delta flagged | — | N/A | BE unit (diff fn) + manual | `go test ./internal/eval/ -run TestCrossEvalDiff` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/src/components/eval/SkillsTable.test.tsx` — GEVL-06/SC-1 (table render + badge)
- [ ] `web/src/components/eval/TrendPanel.test.tsx` — GEVL-06/SC-1 + SC-6 (baseline overlay + drift flag)
- [ ] `web/src/components/eval/AuditTable.test.tsx` — SC-3 (filter + redaction badge)
- [ ] `internal/server/eval_test.go` — `/api/eval/reports` + `/api/eval/frozen` handlers (+ `:ts` path-traversal reject)
- [ ] `internal/server/audit_test.go` — `/api/audit` handler + retention projection + admin token
- [ ] `internal/eval/frozen_test.go` — `LoadLock` + `AssertPin` + `WriteArchive`/INDEX.json
- [ ] `internal/eval/gate_test.go` — `gateDecision` pure function
- [ ] `internal/server/chat_session_test.go::TestSessionIsolation50x5` — SC-4 (scales existing test)
- [ ] `//go:build liveeval` guard on the new live-reproducibility test — Pitfall 3 counter-measure
- [ ] FE test fixtures: `__fixtures__/index.json` + `__fixtures__/report.json` for dashboard tests

*No framework install needed — Go testing + Vitest are both present. The recharts/router FE dependency install (if chosen) is gated by `checkpoint:human-verify`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CI gate fails a real PR + posts comment naming Skill + threshold | GEVL-07 / SC-2 | GitHub Actions runtime — a workflow can only be truly tested by running it; gate *logic* is unit-tested, *wiring* is manual | Open a sacrificial PR that drops compliance faithfulness below 0.9; confirm the workflow fails + `gh pr comment` markdown table appears; confirm `report.json`/`calibration.json` are workflow artifacts |
| Dashboard renders real trend data + redaction badges | GEVL-06 / SC-3 / SC-5 | End-to-end visual UAT against a locally-archived run | Run `cmd/eval --archive` once locally → open `/eval` → confirm trend line + ❌/✅ badges; open `/eval/audit` → confirm `[REDACTED 90d+]` badges on old rows; set `SKILLFORGE_ADMIN_TOKEN` → confirm raw rows appear |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING (❌ W0) references above
- [ ] No watch-mode flags (`vitest run`, not `vitest`)
- [ ] Feedback latency < 30s (quick) / < 3m (full)
- [ ] `nyquist_compliant: true` set in frontmatter (planner/executor flips after every task has a verify)

**Approval:** pending
