---
phase: 30
slug: hygiene-milestone-gate
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-08
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Post-planning rewrite (2026-09-08): per-task map populated from 8 plans
> (30-01..30-08); Wave-0 items confirmed covered by 30-01..30-05 TDD tasks.

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
- **Before `/gsd:verify-work`:** Full suite + verify-phase-30.sh 5 stages all green
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| T-30-01-1 | 30-01 | 1 | HYG-02 | T-30-01 | CR-01: SSE emit failure — no double-write (persisted flag in pipeline.go + chat.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestCR01Fix -count=1"` | ✅ W0 | ⬜ pending |
| T-30-02-1 | 30-02 | 1 | HYG-02 | T-30-02 | WR-01: feedback refs backfill — ON CONFLICT DO NOTHING (feedback.go + sqlite/feedback.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestFeedbackRefs -count=1"` | ✅ W0 | ⬜ pending |
| T-30-02-2 | 30-02 | 1 | HYG-02 | T-30-05 | WR-04: attribution Action filter + ts+id tie-break (feedback.go + audit.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestAttribution -count=1"` | ✅ W0 | ⬜ pending |
| T-30-03-1 | 30-03 | 1 | HYG-02 | T-30-04 | WR-03: webhook CheckRedirect — no cross-host redirect (webhook.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/ticket/ -run TestCheckRedirect -count=1"` | ✅ W0 | ⬜ pending |
| T-30-04-1 | 30-04 | 1 | HYG-02 | T-30-03 | WR-02: queued ticket startup sweep — no silent loss on restart (tickets.go + sqlite/tickets.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestQueuedTicketSweep -count=1"` | ✅ W0 | ⬜ pending |
| T-30-04-2 | 30-04 | 1 | HYG-02 | T-30-06 | WR-05: per-session rate limit + session/turn ownership (tickets.go + feedback.go) | unit (Go) | `wsl -e bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./internal/server/ -run TestRateLimit -count=1"` | ✅ W0 | ⬜ pending |
| T-30-05-1 | 30-05 | 1 | HYG-02 | T-30-07 | WR-06: FeedbackRow useEffect — late-arriving votedVerdict is locked (FeedbackRow.tsx) | unit (FE) | `cd web && npx vitest run FeedbackRow --reporter=verbose` | ✅ W0 | ⬜ pending |
| T-30-06-1 | 30-06 | 2 | HYG-02 | — | verify-phase-30.sh 5-stage gate: TODO/FIXME zero + config-driven envOr check + 26-29 diff literal scan + UAT checkpoints + HYG-03 evidence | shell gate | `bash scripts/verify-phase-30.sh; echo "exit code: $?"` | ✅ W0 | ⬜ pending |
| T-30-06-2 | 30-06 | 2 | HYG-02 | — | Makefile phase-30 target invokes verify-phase-30.sh | shell | `make phase-30 2>&1 \| tail -10` | ✅ W0 | ⬜ pending |
| T-30-07-1 | 30-07 | 3 | HYG-02/HYG-03 | T-30-01..07 | STRIDE-30.md: all 7 threats registered (T-30-01..T-30-07) | docs | `grep -c "T-30-0" docs/threat-model/STRIDE-30.md` | ✅ W0 | ⬜ pending |
| T-30-07-2 | 30-07 | 3 | HYG-02/HYG-03 | — | STRIDE-28.md relocated: docs/threat-model/STRIDE-28.md exists; source removed | shell | `test -f docs/threat-model/STRIDE-28.md && test ! -f .planning/phases/28-dynamic-knowledge-graph/STRIDE-28.md && echo "ok"` | ✅ W0 | ⬜ pending |
| T-30-07-3 | 30-07 | 3 | HYG-02 | — | .env.example: SKILLFORGE_ADMIN_TOKEN documented + main.go envOr wiring | shell | `grep -c "SKILLFORGE_ADMIN_TOKEN" .env.example && grep -c "SKILLFORGE_ADMIN_TOKEN" cmd/server/main.go` | ✅ W0 | ⬜ pending |
| T-30-08-1 | 30-08 | 3 | HYG-02/HYG-03 | — | REQUIREMENTS.md: 12 old IDs backfilled (PLAT-01/02/03/06/07/08, SCALE-01/02/03, DOMAIN-01/02/03) | shell | `grep -cE "^\\| PLAT-0[1-368]\\| " REQUIREMENTS.md && grep -cE "^\\| SCALE-" REQUIREMENTS.md && grep -cE "^\\| DOMAIN-" REQUIREMENTS.md` | ✅ W0 | ⬜ pending |
| T-30-08-2 | 30-08 | 3 | HYG-03 | — | 30-ACCEPTANCE.md: HYG-03 evidence table + CR-01/WR-01..06 regression locks + UAT clearance + trust semantics regression | docs | `test -f .planning/phases/30-hygiene-milestone-gate/30-ACCEPTANCE.md && grep -c "WR-0" .planning/phases/30-hygiene-milestone-gate/30-ACCEPTANCE.md` | ✅ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

All Wave 0 test files confirmed created in-task by 30-01..30-05:

| Fix | Test File | Test Function |
|-----|-----------|--------------|
| CR-01 (SSE emit double-write) | `internal/server/chat_test.go` or `internal/server/pipeline_test.go` | `TestCR01Fix` |
| WR-01 (feedback refs backfill) | `internal/server/feedback_test.go` | `TestFeedbackRefs` |
| WR-02 (queued ticket sweep) | `internal/server/tickets_test.go` | `TestQueuedTicketSweep` |
| WR-03 (CheckRedirect) | `internal/ticket/webhook_test.go` | `TestCheckRedirect` |
| WR-04 (attribution Action filter) | `internal/server/feedback_test.go` | `TestAttribution` |
| WR-05 (rate limit + ownership) | `internal/server/feedback_test.go` | `TestRateLimit` |
| WR-06 (FeedbackRow votedVerdict) | `web/src/components/chat/parts/FeedbackRow.test.tsx` | `FeedbackRow.test.tsx` |

*Existing infrastructure covers framework needs — no new framework install.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| 27/28/29 三份 runbook 全项 operator 亲自跑 | HYG-03 | Operator 需真实浏览器/外发端点 | Per-runbook steps in `.planning/phases/{27,28,29}-*/*.md` |
| live eval frozen replay | HYG-03 | 需要 GLM live + 内网服务探测 | pre-flight probe; 断线记环境 STOP 不刷分 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all CR-01/WR-01..06 regression test files (confirmed in 30-01..30-05)
- [x] No watch-mode flags
- [x] Feedback latency < 180s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] `wave_0_complete: true` set in frontmatter

**Approval:** pending
