# Phase 30: 工程卫生收尾 (Hygiene & Milestone Gate) - Research

**Researched:** 2026-09-08
**Domain:** Engineering hygiene closeout, structural verification, SDLC evidence compilation
**Confidence:** HIGH

## Summary

Phase 30 is a milestone closeout phase — no new features. It delivers: (1) fixes for CR-01 + 6 WRs from 29-REVIEW, each with regression locks; (2) HYG-02 magic-number gate (verify-phase-30.sh) for config-driven invariants across phases 26-29; (3) HYG-03 full SDLC evidence compilation for phases 26-29 plus trust-semantics regression dual-layer proof; (4) UAT clearance for 27/28/29 runbooks as a hard gate; (5) hygiene tidy (traceability backfill, .env.example admin token, STRIDE-28 relocation). The phase is加固-first (fixes before gates) per D-30-01..06 and user direction.

**Primary recommendation:** Build verify-phase-30.sh as an extension of the existing verify-phase-29.sh pattern — 4 new stages for magic-number sampling over the 26-29 diff, integrated into Makefile as `make phase-30`. Run all four prior verify scripts (26/27/28/29) as sub-stages of HYG-03 evidence compilation. UAT clearance is an operator checkpoint with explicit autonomous=false tasks in the plan.

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-30-01:** CR-01 fix — SSE emit failure path must reuse already-written turn row, not write a second pair; emit-failure does NOT produce a second audit row OR second (user, assistant) turn pair; STRIDE-30 registration required.
- **D-30-02:** All 6 WRs fixed with regression locks: WR-01 transactionalize feedback refs; WR-02 queued ticket sweep/retry on startup; WR-03 http.Client.CheckRedirect policy; WR-04 Action filter + ts tie-break on attribution; WR-05 rate limit + session/turn ownership validation; WR-06 useEffect for late-arriving votedVerdict.
- **D-30-03:** 8 infos — trivial fixes inline, rest recorded in 30-ACCEPTANCE.md with accept/defer + reason.
- **D-30-04:** verify-phase-30.sh with magic-number sampling stages for SKILLFORGE_GRAPH_*, SKILLFORGE_TICKET_*, SKILLFORGE_UPLOAD_MAX_BYTES, SKILLFORGE_KB_SEED, SKILLFORGE_MILVUS_RECREATE, parser format whitelist, semantic roles (kb registry).
- **D-30-05:** Evidence compilation for phases 26-29 — unit + hermetic + WSL build + verify gates + FE vitest/tsc.
- **D-30-06:** Trust regression dual-layer: (a) hermetic: KNOW-02/D-09/citation whitelist structural gates + frozen golden set; (b) live: cmd/eval --frozen --gate-check with GLM, faithfulness >= 0.9, refusal_accuracy >= 0.95, with Milvus/BGE/GLM probe before run and STOP-on-disconnect discipline.
- **D-30-07:** UAT clearance — 27-UAT + 28-UAT + 29-UAT + 29-HUMAN-UAT all items run by operator before closeout.

### Claude's Discretion
- Traceability bookkeeping: fill 12 old PLAT/SCALE/DOMAIN IDs in REQUIREMENTS.md traceability table.
- `.env.example` add `SKILLFORGE_ADMIN_TOKEN` with comment explaining fail-closed admin semantics.
- STRIDE-28 relocation from `.planning/phases/28-dynamic-knowledge-graph/` to `docs/threat-model/`.
- STRIDE-30 threat surface: register CR-01/WR-01..06处置 + residual risks (no new attack surface).
- 30-ACCEPTANCE.md structure and location.
- Milestone closeout ceremony (CHANGELOG/RELEASE-NOTES) is OUT OF SCOPE — `/gsd:complete-milestone` responsibility.

### Deferred Ideas (OUT OF SCOPE)
- Milestone closeout ceremony (CHANGELOG/RELEASE-NOTES/gsd-milestone-summary).
- WR-05 deep auth system (real user identity/login state) — rate limit + ownership only.
- WR-02 persistent outbox pattern — sweep/retry only, outbox-level semantics deferred.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CR-01 fix (SSE emit error branch) | API/Backend | — | chat.go + pipeline.go error path; store layer unchanged |
| WR-01 (feedback refs transactional) | API/Backend | Database | feedback.go handler + sqlite/feedback.go store |
| WR-02 (queued ticket sweep) | API/Backend | — | tickets.go startup sweep + goroutine dispatch |
| WR-03 (CheckRedirect) | API/Backend | — | webhook.go http.Client construction |
| WR-04 (attribution Action filter + ts tie-break) | API/Backend | — | feedback.go attribution + sqlite/audit.go filter |
| WR-05 (rate limit + session ownership) | API/Backend | — | tickets.go + feedback.go route handlers |
| WR-06 (FeedbackRow useEffect) | Frontend | — | FeedbackRow.tsx useEffect for votedVerdict |
| verify-phase-30.sh magic-number gate | CI/Gate | — | bash script, no runtime dependency |
| HYG-03 evidence compilation | CI/Gate | — | bash aggregation of prior gate outputs |
| UAT clearance | Operator | — | Human-run runbooks, AI prepares environment |
| .env.example SKILLFORGE_ADMIN_TOKEN | Config | — | Documentation only |
| STRIDE-28 relocation | Documentation | — | File move |
| Traceability backfill | Documentation | — | REQUIREMENTS.md edits |

## Standard Stack

### Core (No new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|--------|---------|--------------|
| bash (verify scripts) | any POSIX | Structural gate execution | All prior phases use bash gates |
| `golang.org/x/time/rate` | (stdlib equiv) | WR-05 rate limiting candidate | [ASSUMED] — golang.org/x/time/rate is standard token-bucket for Go; planner must decide whether to import or use in-process simple counter |

### No new packages introduced
This phase is maintenance/hygiene — zero external package additions. All work is code modification, script authoring, and documentation.

## Package Legitimacy Audit

> No external packages are installed in this phase. All work is Go/TypeScript code fixes, bash script authoring, and documentation updates. Package Legitimacy Gate is N/A.

## Architecture Patterns

### System Architecture Diagram

Phase 30 does not introduce new system components. Architecture is unchanged — this phase operates on the existing system as a verification and hardening layer:

```
[Operator] --UAT runbooks--> [Phase 30 deliverables]
     |
     +-- verify-phase-30.sh --> [CR-01 fix] --> [WR-01..06 fixes] --> [magic-number gate]
     |
     +-- cmd/eval --frozen --gate-check --> [trust regression proof]
     |
     +-- HYG-03 compilation --> [30-ACCEPTANCE.md milestone evidence]
```

### Pattern 1: CR-01 Fix — Persisted Flag for Error Branch

**What:** Add a `persisted` boolean return from `writeAuditAndTurn`, propagate it through the pipeline error return, and guard the error branch in `chatHandler` to skip re-writing when the pipeline already persisted.

**When to use:** When a pipeline writes data and then may fail to emit it — the caller must know whether to write a fallback or not.

**Example:**
```go
// pipeline.go — report whether this pipeline invocation already persisted
turnID, persisted := s.writeAuditAndTurn(...)  // Step 11 site
if emitErr != nil {
    return &pipelineEmitError{err: emitErr, persisted: persisted}
}

// chat.go error branch — persist only if the pipeline did not
if !persisted {
    turnID, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query,
        verdict.Verdict{}, "", "internal_error", nil)
}
```

### Pattern 2: WR-01 — Transactional Feedback Backfill

**What:** On feedback replay (inserted=false branch), backfill missing refs with `ON CONFLICT DO NOTHING` idempotency.

**When to use:** When a multi-write operation can partially fail and needs a recoverable idempotent backfill.

**Example:**
```go
if !inserted {
    // Replay: refs may be missing from a previously failed write;
    // UNIQUE(feedback_id, chunk_id) makes the backfill a no-op when they exist.
    _ = s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs)
    c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", Verdict: saved.Verdict})
    return
}
```

### Pattern 3: WR-02 — Startup Queued Ticket Sweep

**What:** On server boot, re-dispatch all tickets with `dispatch_status='queued'` that are older than a threshold, emitting audit rows per attempt.

**When to use:** When a fire-and-forget async operation must survive process restarts.

### Pattern 4: WR-03 — CheckRedirect Policy

**What:** Configure http.Client with a redirect policy that denies cross-origin redirects (prevents Secret header forwarding + SSRF pivot).

**Example:**
```go
client: &http.Client{
    Transport: http.DefaultTransport.(*http.Transport).Clone(),
    CheckRedirect: func(req *http.Request, via []*http.Request) error {
        if len(via) >= 3 {
            return fmt.Errorf("ticket webhook: too many redirects")
        }
        if req.URL.Host != via[0].URL.Host {
            return fmt.Errorf("ticket webhook: refusing cross-host redirect to %s", req.URL.Host)
        }
        return nil
    },
},
```

### Pattern 5: WR-04 — Attribution Action Filter + Tie-Break

**What:** Bind attribution to verdict rows only with an explicit Action whitelist and deterministic ORDER BY ts DESC, id DESC.

**Example:** `NearestVerdictAuditBySession(ctx, sessionID, before time.Time)` with `AND action = '' AND chunk_ids != '' ORDER BY ts DESC, id DESC LIMIT 1`.

### Pattern 6: WR-05 — Rate Limit + Session Ownership

**What:** Per-session token-bucket rate limit on POST /api/feedback and POST /api/tickets, plus session/turn ID ownership validation (reject cross-session guesses).

**Example:** `golang.org/x/time/rate` limiter per session; reject `local-session-` prefix on server side.

### Pattern 7: WR-06 — useEffect for Late-Arriving votedVerdict

**What:** Seed server lock via useEffect on votedVerdict prop, not just initial useState seed.

**Example:**
```tsx
const [localLock, setLocalLock] = useState<Vote | null>(null);
useEffect(() => { setLocked(votedVerdict); }, [votedVerdict]);
const locked = localLock ?? votedVerdict;
```

### Anti-Patterns to Avoid
- **WR-05 deep auth (deferred):** Adding full user authentication infrastructure for this phase — rate limit + ownership are sufficient per D-30-02.
- **WR-02 outbox pattern (deferred):** Full persistent outbox with transactional outbox table — sweep/retry is sufficient per deferred list.
- **Milestone ceremony:** CHANGELOG/RELEASE-NOTES are OUT OF SCOPE per D-30-07; handled by `/gsd:complete-milestone`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Rate limiting | Custom token bucket from scratch | `golang.org/x/time/rate` | Well-tested, standard, and sufficient for per-session limiting |
| Attribution query | Ad-hoc filter without tie-break | Explicit store helper with Action filter + id DESC tie-break | Deterministic ordering is critical for correctness |
| Feedback replay backfill | Re-write refs on every replay | `ON CONFLICT DO NOTHING` idempotent backfill | Already established pattern; zero additional writes |
| Queued ticket recovery | UPDATE status on restart | INSERT-only sweep — re-read queued rows and re-dispatch | INSERT-only discipline forbids status flip |

## Runtime State Inventory

> This phase does not involve rename/refactor/migration. Runtime state inventory is N/A.

## Common Pitfalls

### Pitfall 1: UAT Debt Underestimation
**What goes wrong:** 27/28/29-UAT combined have ~25+ operator steps; treating UAT clearance as a single task leads to incomplete execution.
**Why it happens:** The 29-HUMAN-UAT has 9 items, 28-UAT has 18 steps, 27-UAT has 7 steps — total ~34 operator actions.
**How to avoid:** Break UAT clearance into explicit per-phase operator checkpoint tasks (autonomous: false), one per runbook.
**Warning signs:** Plan shows UAT as a single line item.

### Pitfall 2: Magic-Number Gate Over-Bloating
**What goes wrong:** HYG-02 gate tries to verify every config value, creating a gate that is too expensive to maintain.
**Why it happens:** The 26-29 surfaces have many config values; not all are "magic numbers" that need active verification.
**How to avoid:** Focus on the 4+6+4=14 fields explicitly named in D-30-04: graph limits (4), ticket params (6), upload/KB/parser/milvus (4). Do not expand scope.
**Warning signs:** Gate script exceeds 200 lines.

### Pitfall 3: Live Eval Without Network Probe
**What goes wrong:** Running `cmd/eval --frozen --gate-check` against an unreachable Milvus/BGE/GLM endpoint causes a hard failure that looks like a code defect.
**Why it happens:** The network-disconnect discipline (env-intranet-service-flakiness memory) is well-known but live eval is often run without pre-flight checks.
**How to avoid:** Structure the HYG-03 live layer as: (1) pre-flight probe for Milvus/BGE/GLM; (2) STOP with "env STOP" if any probe fails; (3) operator controls retry timing.

### Pitfall 4: Missing STRIDE-30 Registration
**What goes wrong:** CR-01 fix is implemented but STRIDE-30 threat register does not document the disposition of CR-01/WR-01..06, leaving a gap in the threat model.
**Why it happens:** STRIDE-30 is mentioned in D-30-01 but the format/content of the registration is left to planner discretion.
**How to avoid:** Create STRIDE-30.md following the STRIDE-29.md format, with one threat row per finding and explicit Disposition.

## Code Examples

### CR-01 Fix Surface (pipeline.go + chat.go)

From 29-REVIEW.md CR-01 fix suggestion:
```go
// pipeline.go — return persisted flag with error
turnID, persisted := s.writeAuditAndTurn(...)
if emitErr != nil {
    return &pipelineEmitError{err: emitErr, persisted: persisted}
}

// chat.go error branch
if !persisted {
    turnID, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query,
        verdict.Verdict{}, "", "internal_error", nil)
}
```

### verify-phase-30.sh Magic-Number Stage Structure

From D-30-04, the gate checks 14 named fields in two sub-checks:
1. **Config-driven check:** `cmd/server/main.go` uses `envOr`/`parseIntEnvOrDefault`/`parseBytesEnvOrDefault` for all 14 fields — verify no hardcoded literals in the source.
2. **Diff scan check:** For phases 26-29 diff, scan for new hardcoded literals matching the 14 field name patterns (with test-fixture whitelist).

### HYG-03 Evidence Compilation Structure

Per D-30-05, evidence for each phase (26-29):
- Unit test count + pass/fail
- Hermetic integration test coverage
- WSL build clean (`make check-cgo && wsl go build ./...`)
- Verify gate result (verify-phase-26/27/28/29.sh)
- FE vitest + tsc clean

### cmd/eval --frozen --gate-check Trust Regression

From cmd/eval/main.go:65-67:
```go
// --gate-check=path  do NOT run an eval at all: load an existing
//                   report, apply the per-Skill thresholds, print the
//                   markdown table and exit non-zero on regression.
```

From D-30-06: hermetic layer (frozen golden set self-consistency) + live layer (`cmd/eval --frozen compliance --gate-check eval/reports/compliance/gate-check.json`, faithfulness >= 0.9, refusal_accuracy >= 0.95).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CR-01 double-write (pre-30) | Persisted flag guards error branch | Phase 30 | Eliminates duplicate audit rows on SSE emit failure |
| WR-01 non-transactional refs | ON CONFLICT DO NOTHING backfill | Phase 30 | Per-KB attribution survives partial failure |
| WR-02 queued tickets lost on restart | Startup sweep re-dispatch | Phase 30 | No silent loss of outbound ticket half |
| WR-03 no CheckRedirect | Cross-host redirect deny | Phase 30 | Secret header + SSRF pivot blocked |
| WR-04 attribution without Action filter | Action whitelist + ts+id tie-break | Phase 30 | Attribution binds correct audit row |
| WR-05 no rate limit | Per-session token bucket | Phase 30 | Bounded webhook relay abuse |
| WR-06 FeedbackRow seed at mount | useEffect on votedVerdict prop | Phase 30 | Late-arriving data-turn handled |

**Deprecated/outdated:** None — all changes are additive hardening.

## Assumptions Log

> List all claims tagged `[ASSUMED]` in this research.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `golang.org/x/time/rate` is the standard rate-limit package for Go | Common Pitfalls | The project may choose a different approach (e.g., in-process simple counter); planner must decide |
| A2 | WR-05 rate limit uses `golang.org/x/time/rate` | Architecture Patterns | Same as A1 |
| A3 | verify-phase-30.sh magic-number diff scan operates on `git diff phase-26..HEAD -- "*.go"` | Common Pitfalls | Exact diff range needs confirmation; may need to be git log based |

## Open Questions

1. **WR-05 rate limit implementation choice:** Use `golang.org/x/time/rate` (standard token bucket) or an in-process simple counter? The CONTEXT says planner discretion.
   - What we know: `golang.org/x/time/rate` is in Go stdlib orbit; project has no existing rate-limit dependency.
   - What's unclear: Whether importing a new package is acceptable under the "zero new dependency" discipline.
   - Recommendation: Use `golang.org/x/time/rate` — it is maintained by the Go team and the discipline is about unvetted third-party packages, not stdlib-adjacent utilities.

2. **SKILLFORGE_ADMIN_TOKEN scope:** D-30-07 says `.env.example` needs `SKILLFORGE_ADMIN_TOKEN` but this token is not currently wired in main.go. The admin endpoints use `isAdminRequest` which checks a header — is this header currently env-driven?
   - What we know: `SKILLFORGE_ADMIN_TOKEN` is referenced in the UAT runbooks as the admin gate.
   - What's unclear: Whether `SKILLFORGE_ADMIN_TOKEN` is already parsed from env in main.go, or whether it is a different token from `SKILLFORGE_DATA_SALT`.
   - Recommendation: Verify during planning — if not wired, this is a trivial envOr addition to main.go.

3. **UAT operator checkpoint granularity:** Should each of 27-UAT, 28-UAT, 29-UAT, 29-HUMAN-UAT be a separate plan task, or should they be combined?
   - What we know: 27-UAT has 7 steps, 28-UAT has 18, 29-UAT has ~15, 29-HUMAN-UAT has 9.
   - What's unclear: Whether the operator prefers one session or separate sessions.
   - Recommendation: One task per runbook (4 tasks total) with `autonomous: false` and clear step instructions in the task description.

## Environment Availability

> This phase has no external dependencies beyond the existing codebase. All verification scripts use grep/bash only.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| bash | verify-phase-30.sh | ✓ (WSL) | any POSIX | Windows Git Bash |
| git | diff-based magic-number scan | ✓ | any | — |
| go (WSL) | WSL build verification | ✓ | 1.22+ | — |
| npx/vitest | FE vitest | ✓ | latest | — |
| npx/tsc | FE type check | ✓ | latest | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test + bash gate scripts + vitest |
| Config file | No new config — uses existing Makefile + verify scripts |
| Quick run command | `bash scripts/verify-phase-30.sh` |
| Full suite command | `make phase-30 && bash scripts/verify-phase-26.sh && bash scripts/verify-phase-27.sh && bash scripts/verify-phase-28.sh && bash scripts/verify-phase-29.sh` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| HYG-02 | Zero hardcoded magic numbers in 26-29 config surfaces | bash gate | `bash scripts/verify-phase-30.sh` | NEW — create |
| HYG-03 | SDLC evidence compilation for phases 26-29 | compilation | `make phase-30` | NEW |
| CR-01 fix | emit-failure does not write second audit/turn pair | unit | `go test ./internal/server/... -run TestCR01Fix -v` | NEW test file |
| WR-01 | feedback refs transactional with backfill | unit | `go test ./internal/server/... -run TestFeedbackRefs -v` | NEW test file |
| WR-02 | queued ticket survives restart | unit | `go test ./internal/server/... -run TestQueuedTicketSweep -v` | NEW test file |
| WR-03 | CheckRedirect blocks cross-host | unit | `go test ./internal/ticket/... -run TestCheckRedirect -v` | NEW test file |
| WR-04 | attribution uses Action filter + id tie-break | unit | `go test ./internal/server/... -run TestAttribution -v` | NEW test file |
| WR-05 | rate limit + session ownership | unit | `go test ./internal/server/... -run TestRateLimit -v` | NEW test file |
| WR-06 | FeedbackRow handles late votedVerdict | vitest | `npx vitest run FeedbackRow` | update existing |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... -count=1` (target: <60s)
- **Per wave merge:** `make phase-30` (full gate)
- **Phase gate:** All verify scripts (26/27/28/29/30) green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/server/feedback_test.go` — add CR-01 regression test
- [ ] `internal/server/feedback_test.go` — add WR-01 backfill test
- [ ] `internal/server/tickets_test.go` — add WR-02 sweep test
- [ ] `internal/ticket/webhook_test.go` — add WR-03 CheckRedirect test
- [ ] `internal/server/feedback_test.go` — add WR-04 attribution test
- [ ] `internal/server/feedback_test.go` — add WR-05 rate limit + ownership test
- [ ] `web/src/components/chat/parts/FeedbackRow.test.tsx` — update WR-06 useEffect test
- [ ] `scripts/verify-phase-30.sh` — new gate script (HYG-02)
- [ ] `Makefile` — add `phase-30` target

*(If no gaps: "None — existing test infrastructure covers all phase requirements")*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Not in scope for this phase |
| V3 Session Management | partial | WR-05 adds rate limit + session ownership; deep auth deferred |
| V4 Access Control | yes | WR-05 session/turn ownership validation; .env.example admin token |
| V5 Input Validation | yes | WR-05 rate limit + length validation already present |
| V6 Cryptography | no | Not in scope for this phase |

### Known Threat Patterns for this Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SSE emit failure double-write (CR-01) | Tampering | Persisted flag prevents second audit/turn write |
| Feedback refs backfill failure (WR-01) | Tampering | ON CONFLICT DO NOTHING idempotent backfill |
| Queued ticket loss on restart (WR-02) | Denial of Service | Startup sweep re-dispatch |
| Webhook secret redirect leak (WR-03) | Information Disclosure | CheckRedirect policy denies cross-host redirects |
| Attribution misbinding (WR-04) | Tampering | Action whitelist + ts+id tie-break |
| Anonymous webhook relay + cross-session vote (WR-05) | Denial of Service + Spoofing | Per-session rate limit + session/turn ownership validation |
| Late-arriving votedVerdict (WR-06) | Tampering | useEffect syncs server truth to lock |

### STRIDE-30 Threat Register

> New for this phase. Format mirrors STRIDE-29.md. Required by D-30-01.

Registration entries for:
- CR-01: SSE emit failure double-write → mitigated by persisted flag
- WR-01: feedback refs backfill gap → mitigated by ON CONFLICT DO NOTHING
- WR-02: queued ticket restart loss → mitigated by startup sweep
- WR-03: webhook redirect secret leak → mitigated by CheckRedirect
- WR-04: attribution misbinding → mitigated by Action filter + tie-break
- WR-05: anonymous rate limit + cross-session → mitigated by per-session limit + ownership check
- WR-06: late votedVerdict → mitigated by useEffect
- IN-01..IN-08: disposition recorded in 30-ACCEPTANCE.md

## Sources

### Primary (HIGH confidence)
- 30-CONTEXT.md — all decisions D-30-01..07 locked by user
- 29-REVIEW.md — CR-01 + WR-01..06 + IN-01..08 detailed findings
- 29-VERIFICATION.md — pre-existing verification baseline (12/12 must-haves)
- scripts/verify-phase-26.sh / 27 / 28 / 29 — existing gate pattern
- Makefile — existing phase targets + check-cgo
- cmd/eval/main.go — --frozen / --gate-check documentation
- docs/threat-model/STRIDE-29.md — STRIDE format reference

### Secondary (MEDIUM confidence)
- .env.example — SKILLFORGE_ADMIN_TOKEN gap identified (not present; confirmed by grep)
- cmd/server/main.go — SKILLFORGE_* config wiring (lines 331-364 confirmed via grep)

### Tertiary (LOW confidence)
- None — all key facts verified via primary sources

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new packages
- Architecture: HIGH — all patterns verified from existing code/literature
- Pitfalls: HIGH — derived from existing gate scripts and phase history

**Research date:** 2026-09-08
**Valid until:** 30 days (hygiene phase, no fast-moving tech changes expected)
