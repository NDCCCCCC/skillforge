---
phase: 06-eval-dashboard-production-hardening
verified: 2026-08-13T17:35:00Z
status: human_needed
score: 11/12 must-haves verified
overrides_applied: 0
re_verification: false
human_verification:
  - test: "Merge eval-gate.yml from worktree-agent-a1bfd67bedf21ad40 to master, then open a sacrificial PR that drops compliance faithfulness below 0.9"
    expected: "Workflow triggers (pull_request, path-filtered), gate step exits non-zero, gh pr comment posts the FormatGateMarkdown table naming skill=compliance + threshold=0.9 + ❌, report.json/calibration.json appear as workflow artifacts"
    why_human: "VALIDATION.md classifies SC-2 wiring as 'manual / CI-only' — GitHub Actions runtime is not exercisable from the repo; the gate LOGIC (gateDecision / FormatGateMarkdown / runGateCheck exit code) is fully unit-tested on master, only the Actions plumbing remains"
  - test: "Run cmd/eval --archive locally, open /eval, then /eval/audit"
    expected: "SkillsTable shows one compliance row with 5 metrics + ✅ badge; TrendPanel renders the recharts line + gray dashed frozen-baseline ReferenceLine; /eval/audit shows filter bar; rows older than 90d show '[REDACTED (90d+)]' badge with hover detail"
    why_human: "VALIDATION.md manual-only row — end-to-end visual UAT against a locally archived run; FE unit tests (14 green) cover component render against fixtures but not live backend integration"
  - test: "Set SKILLFORGE_ADMIN_TOKEN and re-fetch /api/audit; then unset it and re-fetch"
    expected: "With valid X-Admin-Token header: raw query/verdict_json on old rows; without: '[REDACTED:90d+ ...]' sentinel"
    why_human: "Unit tests cover the pure redact() fn and the constant-time compare, but the live env-var interaction with a running server is a runtime smoke check"
  - test: "go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility with GLM_API_KEY set"
    expected: "PASS (or SKIP if key unset) — abs(run1-run2) < 1e-3 on all 4 metrics across the 5-case cap"
    why_human: "Phase-gate check requires a real GLM API key (quota cost); the //go:build liveeval guard is verified on master, the live run itself is the SC-6 jitter layer"
audit_acknowledged:
  milestone: v1.7
  at: 2026-10-24
  status: human_needed
---

# Phase 6: Eval Dashboard & Production Hardening Verification Report

**Phase Goal:** Turn the eval pipeline into a CI gate, surface per-Skill metrics on a dashboard, and stand up the production-grade observability + privacy posture — audit retention, redaction, session isolation.
**Verified:** 2026-08-13T17:35:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth   | Status     | Evidence       |
| --- | ------- | ---------- | -------------- |
| 1   | SC-1 / GEVL-06 — Dashboard renders per-Skill metrics + trend + frozen baseline | ✓ VERIFIED | `web/src/components/eval/SkillsTable.tsx` renders 5 metrics + ✅/❌ badge; `TrendPanel.tsx` renders recharts LineChart + gray dashed ReferenceLine + `data-testid="trend-drift-flag"` on >5% drift; `EvalDashboard.tsx` polls `/api/eval/reports` + `/api/eval/frozen` every 30s via `useEvalReports`; 14 FE tests green |
| 2   | SC-2 / GEVL-07 — CI gate blocks promotion on faithfulness < threshold | ⚠️ PARTIAL (logic ✓, workflow wiring deferred) | `internal/eval/gate.go::gateDecision` pure fn + `FormatGateMarkdown` produces the Skill+threshold table; `cmd/eval/main.go::runGateCheck` is the ONLY non-zero exit path; `skills/compliance/eval-gate.json` ships `{"faithfulness_threshold": 0.9}`; `.github/workflows/eval-gate.yml` exists ONLY on unmerged branch `worktree-agent-a1bfd67bedf21ad40` (commit `fc26d12`), NOT on master — user explicitly skipped 06-03 ("先跳过 CI") |
| 3   | SC-3 — Queryable audit UI with row-level redaction | ✓ VERIFIED | `internal/server/audit.go::auditHandler` maps user/skill/since/until/refusal_reason/limit/offset 1:1 to `store.AuditFilter` (lines 248-256); `web/src/components/eval/AuditTable.tsx` renders 7-input filter bar + `RedactedBadge` with `data-testid="audit-row-redacted"` + `title={value}` hover detail (D6-07); TestAuditHandler_* green |
| 4   | SC-4 — Session isolation proven by 50×5 concurrency | ✓ VERIFIED | `internal/server/chat_session_test.go::TestSessionIsolation50x5` (line 363) runs 50 sessions × 5 turns under `-race`; asserts `GetSession(sessN)` returns exactly 10 messages + zero cross-session query-token leakage; passes in 2.398s under `-race` |
| 5   | SC-5 — Retention/redaction policy enforced at read-time | ✓ VERIFIED | `redact()` (audit.go:148-165) is invoked between `QueryAudit` and `c.JSON` (line 273) — never in the store; `auditRetention()` reads `SKILLFORGE_RETENTION_DAYS` (default 90); `isAdminRequest` uses `subtle.ConstantTimeCompare` (line 128); unset `SKILLFORGE_ADMIN_TOKEN` → all non-admin (D6-08); T-3-AUDIT INSERT-only preserved (only pre-existing `UpdateAuditMetrics` UPDATE remains) |
| 6   | SC-6 — Eval runs reproducible (Q1 + Q2 landed) | ✓ VERIFIED | `internal/reprohash/reprohash.go::PromptHash(skillName, modelVersion, renderedPromptBody)` — 3-arg with rendered body (Q1 resolved); `eval/frozen/compliance.jsonl` has 50 lines; `compliance.lock.json` pins all 5 fields (model_ver, prompt_hash, seed, eval_set_version, skill_name); `TestShippedFrozenLockIsInSync` proves byte-identity with the live render path; `eval_live_test.go` has `//go:build liveeval` at top of separate file; `TestRunnerReproducibility` uses `StrictReproducibilityTolerance = 1e-6` (frozen.go:43) |
| 7   | D6-02 — WriteArchive + INDEX.json trend history | ✓ VERIFIED | `internal/eval/archive.go::WriteArchive` writes `<ts>.json` + appends IndexRun via tmp+rename atomic write; `IndexRun` carries all 10 fields including derived `RefusalAccuracy` from `PerCategory["refusal-required"]["context_recall"]`; `TestWriteArchive_*` green |
| 8   | D6-14 — crossEvalDiff flags >5% delta | ✓ VERIFIED | `internal/eval/crosseval.go::crossEvalDiff` with `crossEvalEpsilon=1e-9` boundary guard (0.92 vs 0.87 exact-5% case does NOT trip); `TestCrossEvalDiff_*` green |
| 9   | BE API surface — /api/eval/reports, /api/eval/reports/:ts, /api/eval/frozen | ✓ VERIFIED | `internal/server/eval.go` defines `EvalReporter` interface + `FilesystemEvalReporter` impl; routes registered in server.go:219-222; `cmd/server/main.go:265` wires `NewFilesystemEvalReporter(evalReportsDir, evalFrozenDir)` into Deps; missing INDEX.json → empty 200 (not 500); TestEvalReports_* + TestEvalFrozen_* green |
| 10  | T-6-PATH — :ts regex-validated before fs read | ✓ VERIFIED | `validTimestamp = regexp.MustCompile("^[0-9T-]+Z$")` (eval.go:91); tightening from plan's `^[0-9T:-]+Z$` (colons excluded because the actual layout `2006-01-02T15-04-05Z` uses hyphens — colons are illegal in Windows filenames); source-check test `TestEvalReports_TsRegexSourceCheck` locks the pattern; defense-in-depth absolute-path escape check in `FilesystemEvalReporter.Report` (lines 274-288) |
| 11  | D14 — eval slice has NO messages field | ✓ VERIFIED | `web/src/store/app-store.ts` eval slice (lines 164-171) declares route / evalSubroute / indexReports / auditRows / auditFilter / frozenBaseline only; explicit `INVARIANT (D14): NO messages field` comment at line 164; D-04/D12 transport-swap preserved — chat `useChat` SSE untouched, /eval route uses plain JSON fetch |
| 12  | GEVL-07 end-to-end — CI workflow file present on master + secrets configured | ✗ FAILED (deferred per user) | `.github/workflows/eval-gate.yml` does NOT exist on master (`ls .github/workflows/` → No such directory); exists only on unmerged worktree branch `worktree-agent-a1bfd67bedf21ad40` at commit `fc26d12`; ROADMAP.md 06-03 row remains unchecked `[ ]`; user explicitly said "先跳过 CI" so this is a documented skip, not a slip — but SC-2's "PR comment" behavior is unverifiable without GitHub Actions runtime |

**Score:** 11/12 truths verified (the 1 failure is the user-skip; gate logic is fully landed and tested)

### Required Artifacts

| Artifact | Expected    | Status | Details |
| -------- | ----------- | ------ | ------- |
| `internal/reprohash/reprohash.go` | Canonical 3-arg PromptHash | ✓ VERIFIED | Leaf package breaks server↔eval import cycle; length-prefixed fields prevent collision |
| `internal/eval/frozen.go` | LockFile + LoadLock + AssertPin + PromptHashFor + ActiveModelVersion | ✓ VERIFIED | AssertPin checks model_ver → skill_name → prompt_hash in most-actionable-first order with named errors |
| `internal/eval/archive.go` | WriteArchive + LoadIndex + IndexRun + EvalIndex + RefusalAccuracy | ✓ VERIFIED | Atomic tmp+rename INDEX.json append; corruption stops (does not silently reinit) |
| `internal/eval/gate.go` | gateDecision (pure) + GateDecision + FailedSkills + LoadThresholds + FormatGateMarkdown + LoadReport | ✓ VERIFIED | Pure fn tested; exported seam for cmd/eval; markdown table names Skill+threshold+✅/❌ |
| `internal/eval/crosseval.go` | crossEvalDiff (pure) + CrossEvalDiff + CrossEvalDelta | ✓ VERIFIED | >5% flag with 1e-9 epsilon boundary guard |
| `internal/eval/eval_live_test.go` | //go:build liveeval guard on live reproducibility test | ✓ VERIFIED | Tag at file top (separate file per Go build constraint rules); GLM_API_KEY skip-guard; 5-case cap |
| `internal/server/chat.go` | computePromptHash(skillName, modelVersion, renderedPromptBody) | ✓ VERIFIED | 3-arg signature at line 439; delegates to reprohash.PromptHash; call site at line 343 passes `s.renderedPromptBody(skillNameForHash)` |
| `internal/server/audit.go` | auditHandler + redact + isAdminRequest | ✓ VERIFIED | 503 on nil Store; 1:1 param map; negative-limit 400; oversize-limit clamp to 1000; read-time redaction between QueryAudit and c.JSON |
| `internal/server/eval.go` | EvalReporter + 3 handlers + FilesystemEvalReporter | ✓ VERIFIED | Regex gate before fs read; absolute-path escape defense in depth; empty-200 on missing INDEX.json |
| `internal/store/store.go` | AuditFilter.Offset field | ✓ VERIFIED | Line 113 `Offset int` with T-3-SQLI comment |
| `internal/store/sqlite/audit.go` | OFFSET ? placeholder | ✓ VERIFIED | Lines 137-139 append ` OFFSET ?` with `args = append(args, filter.Offset)` — never fmt.Sprintf |
| `cmd/server/main.go` | EvalReporter wiring | ✓ VERIFIED | Line 265 `EvalReporter: server.NewFilesystemEvalReporter(evalReportsDir, evalFrozenDir)`; EVAL_REPORTS_DIR + EVAL_FROZEN_DIR env vars (lines 128-129) |
| `cmd/eval/main.go` | --archive / --frozen / --judge-models / --gate-check / --skills-dir flags | ✓ VERIFIED | Lines 216-220; validateFlagMatrix rejects --golden+--frozen together; runGateCheck is the ONLY non-zero exit (line 226) |
| `eval/frozen/compliance.jsonl` | 50-case pinned set | ✓ VERIFIED | 50 lines |
| `eval/frozen/compliance.lock.json` | 5-field pin sidecar | ✓ VERIFIED | model_ver=glm-5.2, prompt_hash=bd59cb…, seed=42, eval_set_version=compliance-2026-08-13-v1, skill_name=compliance |
| `skills/compliance/eval-gate.json` | Q2-resolved threshold file | ✓ VERIFIED | `{"faithfulness_threshold": 0.9}` at the Q2-resolved path (NOT .claude/skills/) |
| `web/src/components/eval/{EvalDashboard,SkillsTable,TrendPanel,CrossEvalTab,AuditTable}.tsx` | 5 dashboard components | ✓ VERIFIED | All present + substantive + wired |
| `web/src/components/eval/__fixtures__/{index,report}.json` | Test fixtures | ✓ VERIFIED | Present |
| `web/src/hooks/useEvalReports.ts` | 30s polling hook | ✓ VERIFIED | Present, consumed by EvalDashboard |
| `web/src/types/eval.ts` | TS mirror of EvalIndex/IndexRun/FrozenSetInfo/AuditRow/AuditFilter | ✓ VERIFIED | Present |
| `web/src/store/app-store.ts` | Eval slice, NO messages field | ✓ VERIFIED | D14 invariant preserved |
| `web/src/App.tsx` | Route branch + popstate sync | ✓ VERIFIED | `route === 'eval'` → EvalDashboard; popstate listener syncs /, /eval, /eval/audit |
| `web/src/components/layout/LeftSidebar.tsx` | "Eval Dashboard" nav entry | ✓ VERIFIED | BarChart3 icon + setRoute('eval') + pushState('/eval') |
| `.github/workflows/eval-gate.yml` | CI workflow on master | ✗ MISSING (on master) | Present on `worktree-agent-a1bfd67bedf21ad40` at commit `fc26d12` (10 grep matches for required strings); NOT merged to master — user-skip |

### Key Link Verification

| From | To  | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `internal/eval/frozen.go::AssertPin` | `internal/server/chat.go::computePromptHash` | Both delegate to `reprohash.PromptHash` | ✓ WIRED | TestShippedFrozenLockIsInSync proves byte-identity against live render path |
| `cmd/eval --frozen=compliance` | `internal/eval/frozen.go::AssertPin` | Flag triggers pin assertion before metric run | ✓ WIRED | cmd/eval/main.go frozenSetPath/frozenLockPath helpers + AssertPin call |
| `cmd/eval --archive` | `internal/eval/archive.go::WriteArchive` | Flag triggers report copy + INDEX.json append | ✓ WIRED | cmd/eval/main.go --archive flag at line 216 |
| `cmd/eval --gate-check` | `internal/eval/gate.go::GateDecision + FormatGateMarkdown` | runGateCheck at line 182-208 | ✓ WIRED | Exits non-zero ONLY in this mode; default eval keeps exit-0 contract |
| `/api/audit` handler | `store.QueryAudit` | AuditFilter 1:1 map | ✓ WIRED | audit.go:248-258 |
| `store.QueryAudit` result | `redact()` | Read-time projection before c.JSON | ✓ WIRED | audit.go:273 — comment "T-3-AUDIT invariant: redaction runs in the HTTP layer, NEVER in the store" |
| `Deps.EvalReporter` | `cmd/server/main.go` | NewFilesystemEvalReporter wiring | ✓ WIRED | main.go:265 |
| `/api/eval/reports/:ts` | `EvalReporter.Report(ts)` | validTimestamp regex before fs access | ✓ WIRED | eval.go:182-188 |
| `App.tsx` | `EvalDashboard.tsx` | `route === 'eval'` branch | ✓ WIRED | App.tsx:165-168 |
| `SkillsTable / AuditTable / TrendPanel` | Zustand store eval slice | useAppStore selectors | ✓ WIRED | All 3 components consume indexReports / auditRows / frozenBaseline from store |
| `EvalDashboard` | `/api/eval/reports` + `/api/eval/frozen` + `/api/audit` | useEvalReports 30s poll + AuditTable fetch | ✓ WIRED | Plain JSON fetch, no SSE |
| `.github/workflows/eval-gate.yml` | `cmd/eval --frozen=compliance --gate-check` | Workflow invokes frozen + gate-check | ⚠️ NOT_WIRED on master | Workflow file exists on worktree branch only (user-skip); the CLI commands it would call are on master and tested |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `SkillsTable` | `indexReports` | `useEvalReports` → `fetch('/api/eval/reports')` → `FilesystemEvalReporter.Index()` → `eval.LoadIndex("eval/reports/INDEX.json")` | Yes — reads real archived runs | ✓ FLOWING (empty-state handled when INDEX.json absent) |
| `TrendPanel` | `indexReports` + `frozenBaseline.baseline_faithfulness` | Same as above + `useEvalFrozen` → `/api/eval/frozen` | Yes — reads real frozen lock metadata | ✓ FLOWING |
| `AuditTable` | `auditRows` | `fetchAudit` → `fetch('/api/audit?...')` → `auditHandler` → `store.QueryAudit` + `redact()` | Yes — reads real SQLite audit rows | ✓ FLOWING |
| `TrendPanel` baseline ReferenceLine | `frozenBaseline` | `useEvalFrozen` → `/api/eval/frozen` → `FilesystemEvalReporter.FrozenSets()` scans `eval/frozen/*.lock.json` | Yes — real lock files | ✓ FLOWING |
| Gate decision | `report.Overall["faithfulness"]` | `runGateCheck` → `eval.LoadReport(reportPath)` → `gateDecision` | Yes — reads real report.json from disk | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Eval package gate tests (AssertPin / LoadLock / WriteArchive / GateDecision / CrossEvalDiff / TestRunnerReproducibility / TestShippedFrozenLockIsInSync) | `go test ./internal/eval/ -run "TestAssertPin\|TestLoadLock\|TestComputePromptHash\|TestWriteArchive\|TestLoadIndex\|TestGateDecision\|TestCrossEvalDiff\|TestRunnerReproducibility\|TestShippedFrozenLock" -count=1` | `ok ... 0.288s` | ✓ PASS |
| Server audit/eval handler tests | `go test ./internal/server/ -run "TestAuditHandler\|TestRetentionRedact\|TestAdminToken\|TestEvalReports\|TestEvalFrozen\|TestComputePromptHash" -count=1` | `ok ... 0.254s` | ✓ PASS |
| SC-4 session isolation under race | `go test -race ./internal/server/ -run TestSessionIsolation50x5 -count=1 -timeout 90s` | `ok ... 2.398s` | ✓ PASS |
| cmd/eval tests + builds | `go build ./cmd/eval ./cmd/server && go test ./cmd/eval/ -count=1` | `ok ... 0.259s` | ✓ PASS |
| go vet across all packages | `go vet ./...` | clean | ✓ PASS |
| Shipped lock regression | `go test ./internal/eval/ -run TestShippedFrozenLockIsInSync -v` | `PASS` | ✓ PASS |
| Store tests (incl. SQLi regression) | `go test ./internal/store/... -count=1` | `ok ... 1.000s` | ✓ PASS |
| FE TypeScript check | `cd web && npx tsc --noEmit` | clean | ✓ PASS |
| FE eval component tests | `cd web && npx vitest run src/components/eval/ --reporter=dot` | 3 files / 14 tests green | ✓ PASS |
| liveeval guard: default suite does not burn quota | `go test ./internal/eval/ -run TestRunnerLiveReproducibility` (no tag) | (test not compiled — `//go:build liveeval` at file top) | ✓ PASS (guard verified by source inspection) |

### Probe Execution

No `scripts/**/tests/probe-*.sh` files in this repo; no probe declarations in PLAN/SUMMARY. Skipped — N/A for this phase.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| GEVL-06 | 06-02 (BE) + 06-04 (FE) | Eval dashboard surfaces per-Skill metrics (faithfulness / citation precision / refusal accuracy) with trend over time | ✓ SATISFIED | SkillsTable + TrendPanel + CrossEvalTab render 5 metrics; `/api/eval/reports` + `/api/eval/frozen` serve real archived data; 14 FE tests green |
| GEVL-07 | 06-01 (gate logic) + 06-03 (CI wiring — SKIPPED) | CI gating on faithfulness < 0.9 fails Skill promotion | ⚠️ PARTIAL | Gate LOGIC fully landed: `gateDecision` pure fn + `FormatGateMarkdown` + `cmd/eval --gate-check` non-zero exit + `skills/compliance/eval-gate.json` threshold; CI WIRING deferred (user-skip): `.github/workflows/eval-gate.yml` on `worktree-agent-a1bfd67bedf21ad40` only, NOT on master; sacrificial-PR verification pending per VALIDATION.md |

No orphaned requirements — both Phase 6 IDs are accounted for.

### Cross-Cutting Invariants (PATTERNS.md 1-10)

| # | Invariant | Status | Evidence |
|---|-----------|--------|----------|
| 1 | T-3-AUDIT INSERT-only — no new UPDATE/DELETE in store | ✓ | Grep of `internal/store/sqlite/*.go` shows only pre-existing `UpdateAuditMetrics` UPDATE (Phase 3, sanctioned); redaction is read-time in handler |
| 2 | T-3-SQLI 100% parameterized | ✓ | `OFFSET ?` uses args slice (sqlite/audit.go:137-139); DROP TABLE regression in `internal/store/store_test.go` green |
| 3 | D14 no-`messages` on eval slice | ✓ | app-store.ts eval slice declares route/evalSubroute/indexReports/auditRows/auditFilter/frozenBaseline only |
| 4 | D-04/D12 transport-swap (chat SSE untouched) | ✓ | Chat `useChat` SSE in App.tsx chat branch unchanged; /eval route uses plain JSON fetch |
| 5 | ASVS V6.2.1 constant-time token compare | ✓ | audit.go:128 `subtle.ConstantTimeCompare` (not ==) |
| 6 | ASVS V5 path-traversal regex on `:ts` | ✓ | eval.go:91 `validTimestamp = ^[0-9T-]+Z$` BEFORE any fs read; defense-in-depth absolute-path escape in FilesystemEvalReporter.Report |
| 7 | D6-09 CI secret hygiene (`${{ secrets.GLM_API_KEY }}`, pull_request only) | ⚠️ DEFERRED | Workflow on unmerged branch only; will be verified when 06-03 is resumed |
| 8 | computePromptHash ↔ .lock.json byte-identity (Q1 BLOCKING) | ✓ | TestShippedFrozenLockIsInSync passes — regression test proves the shipped lock matches the live render path |
| 9 | eval-gate.json path at `skills/<skill>/eval-gate.json` (Q2 BLOCKING) | ✓ | `skills/compliance/eval-gate.json` exists (NOT `.claude/skills/`) |
| 10 | cmd/eval does NOT exit non-zero on low faithfulness | ✓ | `runGateCheck` is the ONLY non-zero exit path (cmd/eval/main.go:226); default eval mode keeps exit-0 contract |

**Q1 + Q2 (the two BLOCKING research questions):** both landed as designed.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | — | — | Grep for TBD/FIXME/XXX/PLACEHOLDER/coming-soon across phase-modified files returned only matches in planning docs, spike fixtures, sketch HTML, and pre-existing comments — no debt markers in production code |

### Human Verification Required

1. **CI gate end-to-end (SC-2 / GEVL-07 closure)** — merge `.github/workflows/eval-gate.yml` from `worktree-agent-a1bfd67bedf21ad40` (commit `fc26d12`) to master, configure `GLM_API_KEY` GitHub Actions secret, then open a sacrificial PR that drops compliance faithfulness below 0.9. Expected: workflow triggers on pull_request (path-filtered), `--gate-check` exits non-zero, `gh pr comment` posts the FormatGateMarkdown table naming skill=compliance + threshold=0.9 + ❌, report.json + calibration.json appear as workflow artifacts. **Why human:** GitHub Actions runtime cannot be exercised from the repo; VALIDATION.md explicitly classifies this as "manual / CI-only".

2. **Live dashboard UAT (SC-1 + SC-3 + SC-5 visual)** — run `cmd/eval --frozen=compliance --archive` once locally, start `cmd/server`, open `/eval` (expect SkillsTable + TrendPanel + frozen baseline ReferenceLine), then `/eval/audit` (expect filter bar + [REDACTED (Nd+)] badges on old rows). Set `SKILLFORGE_ADMIN_TOKEN` + re-fetch with `X-Admin-Token` header to confirm raw rows. **Why human:** FE unit tests cover component render against fixtures but not the live backend integration.

3. **Live reproducibility jitter layer (SC-6 live)** — `go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility` with `GLM_API_KEY` set. Expected: PASS, abs(run1-run2) < 1e-3 across 4 metrics × 5 capped cases. **Why human:** requires a real GLM API key + quota; the `//go:build liveeval` guard is verified on master, only the live run remains.

4. **Decision: 06-03 disposition** — the user explicitly said "先跳过 CI" during execution. Confirm whether (a) the worktree-branch workflow file should be merged to master now (closing GEVL-07 end-to-end), or (b) GEVL-07 should be tracked as a follow-up task in Phase 7 / backlog. The gate LOGIC is fully landed and unit-tested either way.

### Gaps Summary

**One truth partially met, by user choice:** SC-2 / GEVL-07 end-to-end wiring is incomplete on master. The gate LOGIC (the part that must be correct — `gateDecision` pure fn, `FormatGateMarkdown` table, `runGateCheck` exit code, `skills/compliance/eval-gate.json` threshold, flag matrix, frozen pin machinery) is fully landed, unit-tested, and regression-locked. The CI WIRING (the workflow file + GitHub secrets + sacrificial PR) was explicitly deferred by the user ("先跳过 CI") and lives on the unmerged worktree branch `worktree-agent-a1bfd67bedf21ad40`. This is not a slip — it is a documented user decision — but it does mean SC-2 cannot be marked VERIFIED from the master branch alone.

**All other 5 SCs are fully verified** with behavioral spot-checks green:
- SC-1 dashboard (14 FE tests + 3 backend handler test suites)
- SC-3 audit UI (1:1 param map + redaction + filter bar)
- SC-4 session isolation (50×5 under `-race` in 2.398s)
- SC-5 retention/redaction (read-time projection + constant-time admin token + env override)
- SC-6 reproducibility (Q1 strengthened hash + Q2 eval-gate.json path + 1e-6 strict + 1e-3 live guard + shipped-lock regression test)

**All 10 cross-cutting invariants hold** on master (invariant #7 CI secret hygiene is deferred along with the workflow file).

**Zero anti-patterns, zero new Go dependencies, zero broken tests.**

---
_Verified: 2026-08-13T17:35:00Z_
_Verifier: Claude (gsd-verifier)_
