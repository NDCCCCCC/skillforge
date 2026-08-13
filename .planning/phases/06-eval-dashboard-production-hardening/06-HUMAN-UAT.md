---
status: partial
phase: 06-eval-dashboard-production-hardening
source: [06-VERIFICATION.md]
started: 2026-08-13T17:35:00Z
updated: 2026-08-13T17:35:00Z
---

# Phase 6 — Human UAT

## Current Test

[awaiting human testing]

## Tests

### 1. Merge eval-gate.yml from worktree branch, then open a sacrificial PR that drops compliance faithfulness below 0.9

**expected:** Workflow triggers (pull_request, path-filtered), gate step exits non-zero, gh pr comment posts the FormatGateMarkdown table naming skill=compliance + threshold=0.9 + ❌, report.json/calibration.json appear as workflow artifacts

**why_human:** VALIDATION.md classifies SC-2 wiring as "manual / CI-only" — GitHub Actions runtime is not exercisable from the repo; the gate LOGIC (gateDecision / FormatGateMarkdown / runGateCheck exit code) is fully unit-tested on master, only the Actions plumbing remains

**notes:** The workflow file exists on `worktree-agent-a1bfd67bedf21ad40` branch at commit `fc26d12` (138 lines, contains all required strings: `pull_request:`, `GLM_API_KEY`, `--frozen=compliance`, `--gate-check`, `gh pr comment`). Currently NOT merged to master — user explicitly skipped 06-03 ("先跳过 CI"). Requires `GLM_API_KEY` secret registered in repo Settings → Secrets and variables → Actions.

**result:** [pending]

---

### 2. Run cmd/eval --archive locally, open /eval, then /eval/audit

**expected:** SkillsTable shows one compliance row with 5 metrics + ✅ badge; TrendPanel renders the recharts line + gray dashed frozen-baseline ReferenceLine; /eval/audit shows filter bar; rows older than 90d show '[REDACTED (90d+)]' badge with hover detail

**why_human:** VALIDATION.md manual-only row — end-to-end visual UAT against a locally archived run; FE unit tests (14 green) cover component render against fixtures but not live backend integration

**how:** `go run ./cmd/eval --frozen=compliance --archive` then `go run ./cmd/server` (or existing server command), open `http://localhost:8888/eval` and `/eval/audit`

**result:** [pending]

---

### 3. Set SKILLFORGE_ADMIN_TOKEN and re-fetch /api/audit; then unset it and re-fetch

**expected:** With valid X-Admin-Token header: raw query/verdict_json on old rows; without: '[REDACTED:90d+ ...]' sentinel

**why_human:** Unit tests cover the pure redact() fn and the constant-time compare, but the live env-var interaction with a running server is a runtime smoke check

**how:** `export SKILLFORGE_ADMIN_TOKEN=test-token && curl -H "X-Admin-Token: test-token" "http://localhost:8888/api/audit?limit=5"` then unset and re-curl without header; compare the redacted vs raw rows

**result:** [pending]

---

### 4. go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility with GLM_API_KEY set

**expected:** PASS (or SKIP if key unset) — abs(run1-run2) < 1e-3 on all 4 metrics across the 5-case cap

**why_human:** Phase-gate check requires a real GLM API key (quota cost); the //go:build liveeval guard is verified on master, the live run itself is the SC-6 jitter layer

**how:** `export GLM_API_KEY=<your-key> && go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility -count=1 -v`

**result:** [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps

### GAP-01: `.github/workflows/eval-gate.yml` not on master (SC-2 partial)

**status:** failed → deferred per user decision ("先跳过 CI")
**source:** VERIFICATION.md Truth #12
**detail:** The CI workflow file exists on `worktree-agent-a1bfd67bedf21ad40` branch at commit `fc26d12` but is NOT merged to master. The gate LOGIC (gateDecision / FormatGateMarkdown / runGateCheck exit code) is fully unit-tested on master. The Actions plumbing requires:
1. Merging the worktree branch (or recreating the file on master)
2. Registering `GLM_API_KEY` secret in GitHub repo settings
3. Opening a sacrificial PR to verify end-to-end

**disposition decision needed:** Confirm whether to merge the worktree-branch workflow now (closing GEVL-07) or defer to Phase 7 (DIST-*). If deferring, add a note to `.planning/ROADMAP.md` Phase 7 or create a TODO for Phase 7 pickup.
