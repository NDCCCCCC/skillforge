---
phase: 30
slug: hygiene-milestone-gate
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-08
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Draft created at plan-phase (before planner defines tasks) — per-task map below is a skeleton to be filled/rewritten once plans land (prior-phase convention: 27/28/29 rewrote this file post-planning).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (cgo in WSL) + vitest (FE) + bash gate (verify-phase-30.sh) + `cmd/eval --frozen --gate-check` (live trust regression) |
| **Config file** | `go.mod` / `web/package.json` / `Makefile` |
| **Quick run command** | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ ./internal/store/... ./internal/ticket/ -count=1"` + `cd web && npx vitest run` |
| **Full suite command** | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && make check-cgo && go test ./... -count=1"` + `cd web && npm run test && npx tsc --noEmit` |
| **Estimated runtime** | ~120-180 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick run command (scoped to touched packages)
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite + verify-phase-30.sh 4 stages all green
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (pending planner) | — | — | HYG-02/03 | — | (refinement fixups: CR-01 emit-failure no double-write + 6 WR regression locks) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run 'TestEmitFailure\|TestFeedbackRefs\|TestTicketSweep\|TestWebhookNoRedirect\|TestAttributionFilter\|TestPublicEndpointThrottle' -count=1"` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | HYG-02/03 | — | magic-number gate (envOr/registry-driven + 26-29 diff scan) | shell gate | `bash scripts/verify-phase-30.sh` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | HYG-03 | — | full SDLC gate compilation + trust regression dual layer | shell + live | `bash scripts/verify-phase-30.sh && wsl -e bash -lc ".../cmd/eval --frozen --gate-check"` | ❌ W0 | ⬜ pending |
| (pending planner) | — | — | HYG-03 | — | UAT clearance (4 operator runbooks) | manual + record | (operator-action, documented) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] All Go test files for CR-01/WR-01..06 fixes created in-task
- [ ] verify-phase-30.sh stages 4 (TODO / structure / config-driven / diff literal scan)
- [ ] Makefile phase-30 target + STRIDE-30.md + 30-ACCEPTANCE.md

*Existing infrastructure covers framework needs — no new framework install.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 27/28/29 三份 runbook 全项 operator 亲自跑 | HYG-03 | Operator 需真实浏览器/外发端点 | Per-runbook steps in `.planning/phases/{27,28,29}-*/*.md` |
| live eval frozen replay | HYG-03 | 需要 GLM live + 内网服务探测 | pre-flight probe;断线记环境 STOP 不刷分 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending