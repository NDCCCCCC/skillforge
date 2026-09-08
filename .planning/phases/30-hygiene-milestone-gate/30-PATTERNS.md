# Phase 30: 工程卫生收尾 (Hygiene & Milestone Gate) - Pattern Map

**Mapped:** 2026-09-08
**Files analyzed:** 22 (9 modified Go/TS files + 7 new scripts/docs + 6 reference files)
**Analogs found:** 18 / 22

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/pipeline.go` (CR-01) | service | request-response | `internal/server/pipeline.go:556` (self) | exact (same file) |
| `internal/server/chat.go` (CR-01) | controller | request-response | `internal/server/chat.go:317` (self) | exact (same file) |
| `internal/server/feedback.go` (WR-01/04/05) | controller | CRUD | `internal/server/feedback.go:207-249` (self) | exact (same file) |
| `internal/store/sqlite/feedback.go` (WR-01) | model | CRUD | `internal/store/sqlite/feedback.go:129-140` (self) | exact (same file) |
| `internal/server/tickets.go` (WR-02/05) | controller | CRUD | `internal/server/tickets.go:325-385` (self) | exact (same file) |
| `internal/store/sqlite/tickets.go` (WR-02) | model | CRUD | `internal/store/sqlite/tickets.go:47-102` (self) | exact (same file) |
| `internal/ticket/webhook.go` (WR-03) | service | request-response | `internal/ticket/webhook.go:93-96` (self) | exact (self) |
| `web/src/components/chat/parts/FeedbackRow.tsx` (WR-06) | component | request-response | `web/src/components/chat/parts/FeedbackRow.tsx:170` (self) | exact (same file) |
| `web/src/store/app-store.ts` (WR-05 client) | store | request-response | `web/src/store/app-store.ts:566-567` (self) | exact (same file) |
| `scripts/verify-phase-30.sh` | script | batch | `scripts/verify-phase-29.sh` | exact |
| `Makefile` (add phase-30) | config | build | `Makefile:111-114` (phase-29 target) | exact |
| `docs/threat-model/STRIDE-30.md` | doc | documentation | `docs/threat-model/STRIDE-29.md` | exact |
| `docs/threat-model/STRIDE-28.md` | doc | file-move | `docs/threat-model/STRIDE-29.md` | exact |
| `.planning/phases/30-.../30-ACCEPTANCE.md` | doc | compilation | `.planning/phases/22-eval-ci/22-CONTEXT.md` (frozen set schema) | partial |
| `REQUIREMENTS.md` (traceability) | doc | documentation | `REQUIREMENTS.md` existing rows | exact |
| `.env.example` | config | documentation | `cmd/server/main.go:156-192` (SKILLFORGE_* wiring) | role-match |
| `internal/server/feedback_test.go` (regression locks) | test | unit | `internal/server/feedback_test.go` | exact |
| `internal/server/tickets_test.go` (regression locks) | test | unit | `internal/server/tickets_test.go` | exact |
| `internal/ticket/webhook_test.go` (regression locks) | test | unit | `internal/ticket/webhook_test.go` | exact |
| `web/src/components/chat/parts/FeedbackRow.test.tsx` (update) | test | unit | `web/src/__tests__/FeedbackRow.test.tsx` | exact |
| `cmd/server/main.go` (SKILLFORGE_ADMIN_TOKEN wiring) | config | request-response | `cmd/server/main.go:156-192` (self) | exact |

## Pattern Assignments

### `internal/server/pipeline.go` (CR-01 fix — SSE emit failure path)

**File to modify:** `internal/server/pipeline.go:556-559`

**Current pattern (must change):**
```go
// Line 556 — writes audit+turn, then returns emitErr
turnID, _ := s.writeAuditAndTurn(ctx, sessionID, skillID, query, v, skillID, "", meta)
if emitErr != nil {
    return fmt.Errorf("verdict pipeline: emit verdict stream: %w", emitErr)
}
```

**Fix pattern (from 29-REVIEW.md CR-01):**
```go
// CR-01 fix: return persisted flag so caller knows whether to write fallback
turnID, persisted := s.writeAuditAndTurn(ctx, sessionID, skillID, query, v, skillID, "", meta)
if emitErr != nil {
    return &pipelineEmitError{err: emitErr, persisted: persisted}
}
```

---

### `internal/server/chat.go` (CR-01 fix — error branch guard)

**File to modify:** `internal/server/chat.go:317`

**Current pattern (line 317):**
```go
turnID, _ := s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query, verdict.Verdict{}, "", "internal_error", nil)
```

**Fix pattern (from 29-REVIEW.md CR-01):**
```go
// CR-01 fix: persist only if the pipeline did not (persisted flag from pipelineEmitError)
if !persisted {
    turnID, _ = s.writeAuditAndTurn(ctx, sessionID, req.Skill, req.Query,
        verdict.Verdict{}, "", "internal_error", nil)
}
```

**Wait pattern for pipeline error (line 288-328):**
```go
// Line 288-289 — pipeline invocation with error capture
if err := s.VerdictPipeline(ctx, c, req.Query, manifest, sessionID, req.Skill); err != nil {
    // Lines 289-327 — error branch that writes second audit+turn (CR-01 issue)
```

---

### `internal/server/feedback.go` (WR-01 backfill + WR-04 attribution + WR-05 rate-limit)

**File to modify:** `internal/server/feedback.go:207-249` (WR-01), `262-279` (WR-04), route handlers (WR-05)

**WR-01 fix — backfill on replay (lines 231-239):**
```go
if !inserted {
    // WR-01: Replay: refs may be missing from a previously failed write;
    // UNIQUE(feedback_id, chunk_id) makes the backfill a no-op when they exist.
    _ = s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs)
    c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", Verdict: saved.Verdict})
    return
}
// Normalized per-chunk refs (WR-01 — moved inside inserted=true branch for clarity)
if err := s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs); err != nil {
    // existing error handling
}
```

**WR-04 fix — attribution Action filter (from RESEARCH.md Pattern 5):**
```go
// feedbackAttribution function (lines 262-279)
// Add: AND action = '' AND chunk_ids != ''
// Add: ORDER BY ts DESC, id DESC (tie-break)
NearestVerdictAuditBySession(ctx, sessionID, before time.Time)
  -> SELECT ... WHERE session_id=? AND ts < ? AND action = '' AND chunk_ids != ''
     ORDER BY ts DESC, id DESC LIMIT 1
```

**WR-05 fix — rate limit (new, inject per-session limiter):**
```go
// At route handler top (WR-05):
// import "golang.org/x/time/rate"
// Per-session limiter map (global or on Server struct)
if !s.rateLimiter.Allow(sessionID) {
    c.JSON(http.StatusTooManyRequests, errorResponse{Error: "rate limit exceeded"})
    return
}
```

---

### `internal/store/sqlite/feedback.go` (WR-01 ON CONFLICT DO NOTHING)

**File to modify:** `internal/store/sqlite/feedback.go:129-140`

**WR-01 fix — PutFeedbackChunkRefs with idempotent backfill:**
```go
// Line 129-140 — existing PutFeedbackChunkRefs
// Add ON CONFLICT DO NOTHING to the INSERT statement:
// INSERT INTO feedback_chunk_refs(feedback_id, chunk_id, kb_slug, seq)
// VALUES (?, ?, ?, ?)
// ON CONFLICT(feedback_id, chunk_id) DO NOTHING
```

---

### `internal/server/tickets.go` (WR-02 sweep/retry + WR-05 rate-limit)

**File to modify:** `internal/server/tickets.go:325-385` (WR-02), route handlers (WR-05)

**WR-02 fix — startup sweep pattern (from RESEARCH.md Pattern 3):**
```go
// On server boot / periodic ticker:
// Re-read all tickets WHERE dispatch_status='queued' AND created_at < now - threshold
// Re-dispatch through provider (INSERT-only: emit action_ticket_dispatch_started audit row)
for _, ticket := range s.Store.QueuedTickets(ctx, threshold) {
    s.dispatchForTicket(ctx, ticket.ID)
}
```

**WR-05 fix — rate limit at route handler:**
```go
// Per POST /api/tickets handler top (mirrors feedback rate limit):
if !s.rateLimiter.Allow(sessionID) {
    c.JSON(http.StatusTooManyRequests, errorResponse{Error: "rate limit exceeded"})
    return
}
```

---

### `internal/store/sqlite/tickets.go` (WR-02 — QueuedTickets query)

**File to modify:** `internal/store/sqlite/tickets.go:47-102`

**WR-02 fix — add QueuedTickets method:**
```go
// New method for WR-02 sweep:
// SELECT id, ... FROM tickets WHERE dispatch_status='queued' AND created_at < ?
// INSERT-only: no status update ever
func (db *DB) QueuedTickets(ctx context.Context, olderThan time.Time) ([]*TicketRow, error)
```

---

### `internal/ticket/webhook.go` (WR-03 CheckRedirect)

**File to modify:** `internal/ticket/webhook.go:93-96` (client construction)

**WR-03 fix — CheckRedirect policy (from 29-REVIEW.md WR-03):**
```go
// Line 93-96 — http.Client construction
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

---

### `web/src/components/chat/parts/FeedbackRow.tsx` (WR-06 useEffect)

**File to modify:** `web/src/components/chat/parts/FeedbackRow.tsx:170`

**WR-06 fix — useEffect for late-arriving votedVerdict (from 29-REVIEW.md WR-06):**
```tsx
// Line 170 — current pattern (seed only at mount):
const [locked, setLocked] = useState<Vote | null>(votedVerdict);

// WR-06 fix — useEffect to capture late-arriving server truth:
const [localLock, setLocalLock] = useState<Vote | null>(null);
useEffect(() => { setLocked(votedVerdict); }, [votedVerdict]);
const locked = localLock ?? votedVerdict;
```

---

### `web/src/store/app-store.ts` (WR-05 — local-session- prefix rejection hint)

**File to modify:** `web/src/store/app-store.ts:566-567`

**WR-05 client pattern:**
```typescript
// Lines 566-567 — session ID generation
// Server-side must reject 'local-session-' prefix — client generates UUIDs
// for new sessions; server-bound sessions use server-minted 32-hex IDs
```

---

### `scripts/verify-phase-30.sh` (HYG-02 magic-number gate — new file)

**Analog:** `scripts/verify-phase-29.sh` (stage structure), `scripts/verify-phase-26.sh` (run_stage pattern)

**Stage structure (from 30-CONTEXT.md D-30-04):**
```
Stage 1: TODO/FIXME zero (26-29 production files)
Stage 2: HYG-02 config-driven check — verify SKILLFORGE_* via envOr (no hardcoded literals)
         - Graph limits: SKILLFORGE_GRAPH_MAX_NODES/EDGES/NEIGHBOR_DEPTH_LIMIT/PATH_HOPS_LIMIT
         - Ticket params: SKILLFORGE_TICKET_PROVIDER/WEBHOOK_URL/WEBHOOK_RETRIES/WEBHOOK_TIMEOUT_MS/DISPATCH_BUDGET_MS/WEBHOOK_SECRET
         - Other: SKILLFORGE_UPLOAD_MAX_BYTES, SKILLFORGE_KB_SEED, SKILLFORGE_MILVUS_RECREATE
         - Parser format whitelist (kb registry), semantic roles (kb registry)
Stage 3: HYG-02 diff scan — scan phases 26-29 diff for new hardcoded literals
         (test fixture whitelist explicit maintenance)
Stage 4: UAT clearance checkpoints (operator tasks, autonomous=false)
Stage 5: HYG-03 evidence compilation summary (referencing prior verify scripts)
```

**run_stage pattern from verify-phase-26.sh (lines 68-87):**
```bash
run_stage() {
  local name="$1"
  shift
  stage_num=$((stage_num + 1))
  local nn
  nn=$(printf "%02d" "$stage_num")
  echo "=== Stage ${nn}: ${name} ==="
  local log_file="/tmp/phase-30-stage-${nn}.log"
  local rc=0
  "$@" >"${log_file}" 2>&1 || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    echo "  [STAGE-${nn}-PASS] ${name}"
    tail -3 "${log_file}" | sed 's/^/    /'
    return 0
  fi
  echo "  [STAGE-${nn}-FAIL] ${name} (exit=${rc}) — full log at ${log_file}"
  tail -30 "${log_file}" | sed 's/^/    /'
  echo "PHASE 30 STRUCTURAL GATES: FAIL — see stage ${nn} output" >&2
  exit 1
}
```

---

### `Makefile` (add phase-30 target)

**Analog:** `Makefile:111-114` (phase-29 target)

**Pattern to copy:**
```makefile
# phase-30 is the Phase 30 hygiene gate (HYG-02 + HYG-03).
# Invokes scripts/verify-phase-30.sh; exits 0 with the PASS banner on
# full pass, 1 with [STAGE-NN-FAIL] otherwise.
phase-30:
	@echo "Running Phase 30 verification..."
	@bash scripts/verify-phase-30.sh
	@echo "Phase 30 verification passed"
```

---

### `docs/threat-model/STRIDE-30.md` (new file)

**Analog:** `docs/threat-model/STRIDE-29.md` (format, structure, threat register table)

**STRIDE-30 format (from 30-CONTEXT.md D-30-01 + 30-RESEARCH.md STRIDE-30 Threat Register):**
```markdown
# Phase 30 — Threat Model (STRIDE-30)

**Phase:** 30 — 工程卫生收尾 (Hygiene & Milestone Gate)
**Gate:** `bash scripts/verify-phase-30.sh`

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-30-01 | Tampering | CR-01: SSE emit failure double-write | mitigate | Persisted flag prevents second audit/turn write |
| T-30-02 | Tampering | WR-01: feedback refs backfill gap | mitigate | ON CONFLICT DO NOTHING idempotent backfill |
| T-30-03 | Denial of Service | WR-02: queued ticket restart loss | mitigate | Startup sweep re-dispatch |
| T-30-04 | Information Disclosure | WR-03: webhook redirect secret leak | mitigate | CheckRedirect policy denies cross-host redirects |
| T-30-05 | Tampering | WR-04: attribution misbinding | mitigate | Action whitelist + ts+id tie-break |
| T-30-06 | Denial of Service | WR-05: anonymous webhook relay abuse | mitigate | Per-session rate limit + session/turn ownership |
| T-30-07 | Tampering | WR-06: late votedVerdict | mitigate | useEffect syncs server truth to lock |
```

---

### `docs/threat-model/STRIDE-28.md` (relocate)

**Source:** `.planning/phases/28-dynamic-knowledge-graph/STRIDE-28.md`
**Dest:** `docs/threat-model/STRIDE-28.md`

**Pattern:** Simple file move (preserves history)

---

### `30-ACCEPTANCE.md` (evidence compilation)

**Analog:** `.planning/phases/22-eval-ci/22-CONTEXT.md` (frozen set schema), 29-REVIEW.md (format)

**Structure (from 30-CONTEXT.md D-30-05, D-30-06):**
```markdown
# Phase 30: Acceptance Evidence

## HYG-03 Evidence Compilation (phases 26-29)

| Phase | Unit Tests | Hermetic | WSL Build | Verify Gate | FE vitest/tsc |
|-------|------------|----------|-----------|-------------|---------------|
| 26    | [count/pass]| [result] | [result]  | [result]    | [result]      |
| 27    | ...        | ...      | ...       | ...         | ...           |
| 28    | ...        | ...      | ...       | ...         | ...           |
| 29    | ...        | ...      | ...       | ...         | ...           |

## Trust Semantics Regression

### Hermetic Layer
- KNOW-02 fail-closed
- D-09 refusal construction
- Citation whitelist structural gate
- Frozen golden set self-consistency

### Live Layer
- cmd/eval --frozen --gate-check
- faithfulness >= 0.9, refusal_accuracy >= 0.95
- Pre-flight: Milvus/BGE/GLM probe
- STOP-on-disconnect discipline

## UAT Clearance (D-30-07)
- [x] 27-UAT: [operator sign-off]
- [x] 28-UAT: [operator sign-off]
- [x] 29-UAT: [operator sign-off]
- [x] 29-HUMAN-UAT: [operator sign-off]

## CR-01 + WR-01..06 Regression Locks
| ID   | Fix | Test | Status |
|------|-----|------|--------|
| CR-01| SSE emit persisted flag | TestCR01Fix | [result] |
| WR-01| ON CONFLICT DO NOTHING backfill | TestFeedbackRefs | [result] |
| WR-02| Startup sweep | TestQueuedTicketSweep | [result] |
| WR-03| CheckRedirect | TestCheckRedirect | [result] |
| WR-04| Action filter + tie-break | TestAttribution | [result] |
| WR-05| Rate limit + ownership | TestRateLimit | [result] |
| WR-06| useEffect for votedVerdict | FeedbackRow.test.tsx | [result] |
```

---

### `REQUIREMENTS.md` (traceability backfill)

**Analog:** `REQUIREMENTS.md` existing traceability table rows

**Pattern to copy:** Row format with ID, Description, Phase, Status columns
- Add PLAT-01/02/03/06/07/08, SCALE-01/02/03, DOMAIN-01/02/03 (12 IDs total)

---

### `.env.example` (add SKILLFORGE_ADMIN_TOKEN)

**Analog:** `cmd/server/main.go:156-192` (SKILLFORGE_* wiring pattern)

**Pattern to add:**
```bash
# Admin gate token — fail-closed semantics: if not set, admin endpoints return 403
SKILLFORGE_ADMIN_TOKEN=
```

---

### `cmd/server/main.go` (SKILLFORGE_ADMIN_TOKEN wiring — if not already present)

**Analog:** `cmd/server/main.go:156-192` (existing SKILLFORGE_* envOr pattern)

**Pattern to add:**
```go
// adminToken := os.Getenv("SKILLFORGE_ADMIN_TOKEN") // or envOr for default
adminToken := parseEnvOrDefault("SKILLFORGE_ADMIN_TOKEN", "")
// Wire to Server struct for isAdminRequest check
```

---

## Shared Patterns

### CR-01 Fix (Persisted Flag) — Apply to: pipeline.go + chat.go
**Source:** 29-REVIEW.md CR-01 fix suggestion

```go
// pipeline.go:556
turnID, persisted := s.writeAuditAndTurn(...)
if emitErr != nil {
    return &pipelineEmitError{err: emitErr, persisted: persisted}
}

// chat.go:317
if !persisted {
    turnID, _ = s.writeAuditAndTurn(...)
}
```

### WR-01 Backfill (ON CONFLICT DO NOTHING) — Apply to: feedback.go + sqlite/feedback.go
**Source:** 29-REVIEW.md WR-01 fix + internal/store/sqlite/feedback.go:129-140

```go
// Feedback replay path (feedback.go:231-235)
if !inserted {
    _ = s.Store.PutFeedbackChunkRefs(ctx, saved.ID, refs) // idempotent
    c.JSON(http.StatusOK, feedbackResponse{Status: "already_recorded", ...})
    return
}
```

### WR-02 Sweep (INSERT-only recovery) — Apply to: tickets.go + sqlite/tickets.go
**Source:** RESEARCH.md Pattern 3

```go
// On startup: re-read queued rows older than threshold, re-dispatch
// INSERT-only: emit action_ticket_dispatch_started audit row per attempt
```

### WR-03 CheckRedirect — Apply to: webhook.go
**Source:** 29-REVIEW.md WR-03 fix suggestion

```go
CheckRedirect: func(req *http.Request, via []*http.Request) error {
    if len(via) >= 3 { return fmt.Errorf("too many redirects") }
    if req.URL.Host != via[0].URL.Host { return fmt.Errorf("refusing cross-host redirect") }
    return nil
}
```

### WR-04 Attribution Action Filter — Apply to: feedback.go + sqlite/audit.go
**Source:** RESEARCH.md Pattern 5

```go
// NearestVerdictAuditBySession with:
// AND action = '' AND chunk_ids != '' ORDER BY ts DESC, id DESC LIMIT 1
```

### WR-05 Rate Limit — Apply to: feedback.go + tickets.go
**Source:** RESEARCH.md Pattern 6

```go
// golang.org/x/time/rate per-session limiter
// Reject 'local-session-' prefix server-side
```

### WR-06 useEffect — Apply to: FeedbackRow.tsx
**Source:** 29-REVIEW.md WR-06 fix suggestion

```tsx
useEffect(() => { setLocked(votedVerdict); }, [votedVerdict]);
const locked = localLock ?? votedVerdict;
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `scripts/verify-phase-30.sh` (Stage 2-3 magic-number gate) | script | batch | Novel HYG-02 requirement; closest analog is verify-phase-29.sh (4-stage) but magic-number diff scan has no exact prior art |
| `30-ACCEPTANCE.md` (evidence compilation) | doc | compilation | Novel HYG-03 requirement; partial analog in 22-CONTEXT.md (frozen set schema) |

---

## Metadata

**Analog search scope:**
- `internal/server/*.go` (28 files) — all modified files read
- `internal/store/sqlite/*.go` (28 files) — modified store files read
- `internal/ticket/*.go` (5 files) — webhook.go read
- `web/src/components/chat/parts/*.tsx` (24 files) — FeedbackRow.tsx read
- `scripts/verify-phase-*.sh` (13 files) — verify-phase-26/27/28/29.sh read
- `Makefile` — phase-29 target read
- `docs/threat-model/STRIDE-29.md` — format reference read

**Pattern extraction date:** 2026-09-08
