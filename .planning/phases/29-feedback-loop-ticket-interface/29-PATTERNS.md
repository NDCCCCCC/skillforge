# Phase 29: 反馈闭环 + 工单预留 - Pattern Map

**Mapped:** 2026-09-08
**Files analyzed:** 33 new/modified (18 Go + 12 TS/TSX + 2 build/script)
**Analogs found:** 33 / 33 (every new file has an in-repo analog; 0 no-analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/store/sqlite/schema.go` (mod) | model/migration | batch | same file `kpi_calls`/`reports`/`knowledge_bases` blocks (schema.go:285-372 + 491-553 + 738-744) | exact |
| `internal/store/sqlite/feedback.go` (new) | model | CRUD + batch-aggregate | `internal/store/sqlite/kpi_calls.go` | exact |
| `internal/store/sqlite/feedback_test.go` (new) | test | — | `internal/store/sqlite/kpi_calls_test.go` | exact |
| `internal/store/sqlite/tickets.go` (new) | model | CRUD (idempotent insert) | `internal/store/sqlite/kpi_calls.go` + `session.go` ON CONFLICT | exact |
| `internal/store/sqlite/tickets_test.go` (new) | test | — | `internal/store/sqlite/kpi_calls_test.go` | exact |
| `internal/store/sqlite/session.go` (mod) | model | CRUD | same file `appendTurnHalf` (session.go:86-131) | exact |
| `internal/store/store.go` (mod) | model/interface | — | same file Store interface (store.go:579-620) | exact |
| `internal/ticket/ticket.go` (new) | service/interface | event-driven | `internal/retriever/backend.go` `IndexerBackend` | exact |
| `internal/ticket/registry.go` (new) | service/factory | — | `cmd/server/main.go` backend switch (milvus/mongo) + `bge.New` | role-match |
| `internal/ticket/webhook.go` (new) | service | request-response (outbound) | `internal/embedder/bge/bge.go` | exact |
| `internal/ticket/webhook_test.go` (new) | test | — | `internal/embedder/bge` tests + `internal/server` httptest hermetic pattern | role-match |
| `internal/wire/chunk.go` (mod) | utility/config | transform | same file ValidTypes map (chunk.go:428-456) | exact |
| `internal/server/feedback.go` (new) | controller | request-response | `internal/server/kb_admin.go` | exact |
| `internal/server/feedback_test.go` (new) | test | — | `internal/server/kb_admin_test.go` + `admin_audit_test.go` | exact |
| `internal/server/tickets.go` (new) | controller | request-response + async fan-out | `internal/server/kb_admin.go` (write+audit) + `chat.go` best-effort posture | role-match |
| `internal/server/tickets_test.go` (new) | test | — | `internal/server/kb_admin_test.go` | role-match |
| `internal/server/chat.go` (mod) | controller | streaming | same file `writeAuditAndTurn` (chat.go:512-563) + `emitDecision` (chat.go:605-611) | exact |
| `internal/server/pipeline.go` (mod) | controller | streaming | same file Step 11-12 emission window (pipeline.go:519-583) | exact |
| `internal/server/handlers_session_history.go` (mod) | controller | request-response | same file `historyEntry` (handlers_session_history.go:45-67) | exact |
| `internal/server/server.go` (mod) | route/config | — | same file `Deps` (server.go:204+) + route block (server.go:483-611) | exact |
| `cmd/server/main.go` (mod) | config | — | same file envOr helpers (main.go:146-192) | exact |
| `internal/server/{admin_audit,admin_schedules,upload}_test.go` (mod — fake ripple: 5 explicit AppendTurn fakes) | test | — | same files (AppendTurn fakes at admin_audit_test.go:76,220; admin_schedules_test.go:84,226; upload_test.go:74)。**kb_admin_test.go NOT modified**: recordingAuditStore embeds store.Store → inherits the new signature, zero edits (checker warning-5) | exact |
| `web/src/components/chat/parts/FeedbackRow.tsx` (rewrite) | component | event → request-response | same file (current placeholder, 85 lines) | exact |
| `web/src/__tests__/FeedbackRow.test.tsx` (rewrite) | test | — | same file | exact |
| `web/src/components/chat/MessageBubble.tsx` (mod) | component | transform (part scan) | same file verdictStatus scan (MessageBubble.tsx:272-305) | exact |
| `web/src/lib/chat-restore.ts` (mod) | utility | transform | same file (snake_case mirror discipline, header comment lines 9-25) | exact |
| `web/src/types/wire-types.ts` (mod) | config/types | — | same file union + `_driftCheck` (wire-types.ts:380-475) | exact |
| `web/src/components/layout/SfToast.tsx` (new) | component | event-driven | `web/src/App.tsx` wizard-success listener (:301-309) + toast markup (:378-401) | exact |
| `web/src/components/admin/AdminDashboard.tsx` (mod) | component | — | same file tab architecture (AdminDashboard.tsx:49,108-170) | exact |
| `web/src/components/admin/FeedbackMetricsPanel.tsx` (new) | component | request-response | `web/src/components/admin/KnowledgeBaseList.tsx` | exact |
| `web/src/components/admin/ReviewQueuePanel.tsx` (new) | component | request-response (mutation) | `web/src/components/admin/KnowledgeBaseList.tsx` (mutation path :119-160) | exact |
| `web/src/store/app-store.ts` (mod) | store | — | same file `adminSubroute` (app-store.ts:457, :521, :854) | exact |
| `scripts/verify-phase-29.sh` + `Makefile` (new/mod) | config/script | batch | `scripts/verify-phase-28.sh` + Makefile `phase-28` target (Makefile:98-103) | exact |

## Pattern Assignments

### `internal/store/sqlite/schema.go` (migration — additive DDL)

**Analog:** same file — the kpi_calls (Phase 16) and knowledge_bases (Phase 26) blocks are the two freshest "brand-new table, additive" precedents.

**New-table DDL pattern** (schema.go:285-296, kpi_calls; CHECK-constraint precedent schema.go:364-372 knowledge_bases role):
```sql
CREATE TABLE IF NOT EXISTS kpi_calls (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ts              TEXT    NOT NULL,
    session_id      TEXT    NOT NULL,
    ...
);
```
```sql
CREATE TABLE IF NOT EXISTS knowledge_bases (
    slug              TEXT PRIMARY KEY,
    role              TEXT NOT NULL CHECK (role IN ('authority','historical','personal','draft')),
    ...
);
```

**Index-at-migrate()-tail pattern** (schema.go:491-503 consts + 704-712 execution; rationale comment schema.go:480-486):
```go
const (
	createKPICallsFuncIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_kpi_calls_func ON kpi_calls(func)
`
	createKPICallsSessionIDIdxSQL = `
CREATE INDEX IF NOT EXISTS idx_kpi_calls_session_id ON kpi_calls(session_id)
`
)
```
```go
// migrate() tail — schema.go:738-744
if _, err := s.db.ExecContext(ctx, idxKnowledgeBasesRole); err != nil {
    return fmtError("migrate.knowledge_bases.role_idx", err)
}
```
Key convention (schema.go:540-553 comment): the new table is additive (no ALTER dependency) but indexes STILL go in `migrate()` tail consts, never inlined in schemaSQL — "so the D14-E1 ordering contract has exactly one shape to remember." feedback/tickets DDL goes in `schemaSQL` const (schema.go:50-373); all 6 new indexes become tail consts + tail Exec blocks.

**Idempotency-via-UNIQUE DDL precedent** (schema.go:104-111):
```sql
CREATE TABLE IF NOT EXISTS session_turn (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT    NOT NULL,
    role        TEXT    NOT NULL CHECK(role IN ('user','assistant')),
    content     TEXT    NOT NULL,
    ts          DATETIME NOT NULL,
    UNIQUE(session_id, role, ts)
);
```
feedback and tickets copy this: `UNIQUE(session_id, turn_id)`.

**CRITICAL for the InsertOnly grep lock (Pitfall 5):** use only `ON CONFLICT ... DO NOTHING`. `DO UPDATE` inserts the literal `"UPDATE "` into the file and trips `TestXxxInsertOnly`'s source-grep.

---

### `internal/store/sqlite/feedback.go` + `tickets.go` (model, INSERT-only CRUD)

**Analog:** `internal/store/sqlite/kpi_calls.go` — whole-file mirror target (file header states it is itself a "wholesale mirror of import_batches.go").

**Write-path pattern** (kpi_calls.go:38-65):
```go
const putKPICallSQL = `
INSERT INTO kpi_calls (
    ts, session_id, skill_id, func, args_json,
    as_of_period, k_min_applied, result_snapshot, signature
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`

func (s *SQLiteStore) PutKPICall(ctx context.Context, row store.KPICallRow) error {
	_, err := s.db.ExecContext(ctx, putKPICallSQL,
		row.TS.UTC().Format(time.RFC3339Nano),
		...
	)
	if err != nil {
		return fmtError("PutKPICall", err)
	}
	return nil
}
```
feedback variant adds the idempotent clause + RowsAffected guard — **ON CONFLICT shape precedent** (session.go:73-77):
```go
const appendTurnSQL = `
INSERT INTO session_turn (session_id, role, content, ts)
VALUES (?, ?, ?, ?)
ON CONFLICT(session_id, role, ts) DO NOTHING
`
```

**Filter-list read pattern** (kpi_calls.go:75-124): `listKPICallsBaseSQL` const + hardcoded clause consts (`kpiCallsClauseFunc = "func = ?"`, kpi_calls.go:84-87) + `strings.Join(clauses, " AND ")` + limit appended as arg. Copy verbatim for feedback list/metrics queries — "the column names are code, never caller input" (kpi_calls.go:71-74).

**Aggregate-read pattern** (kpi_calls.go:174-177) — the model for metrics + review-queue GROUP BY:
```go
const latestKPICallByFuncSQL = `
SELECT func, ts FROM kpi_calls
WHERE id IN (SELECT MAX(id) FROM kpi_calls GROUP BY func)
`
```

**Audit-nearby-attribution query (D-29-05)** — composed from `QueryAudit` filter fields (store.go:533-565: `SessionID`, `Until`, `Limit`) — no new SQL needed for the lookup itself:
```go
auditRows, err := s.Store.QueryAudit(ctx, store.AuditFilter{
    SessionID: sessionID,
    Until:     turnTs,
    Limit:     1,
})
```

---

### `internal/store/sqlite/kpi_calls_test.go` (test — InsertOnly family)

**Analog:** whole file. Copy the two-segment structure for `TestFeedbackInsertOnly` / `TestTicketsInsertOnly`.

**Segment 1: source-grep structural lock** (kpi_calls_test.go:155-162):
```go
src, err := os.ReadFile("kpi_calls.go")
require.NoError(t, err, "kpi_calls.go must be readable from the package dir")
source := string(src)
assert.NotContains(t, source, "UPDATE ",
    "kpi_calls.go must contain no row-mutation verb (T-16-03-01)")
assert.NotContains(t, source, "DELETE ",
    "kpi_calls.go must contain no row-removal verb (T-16-03-01)")
```

**Segment 2: round-trip + idempotent-replay assertions** (kpi_calls_test.go:164-250): `New(ctx, Config{DSN: ":memory:"})`, seeded rows, `assertRowsEqual` helper (kpi_calls_test.go:182-195), newest-first ordering assert. Add the idempotent-replay segment: second `Put` of the same (session_id, turn_id) must return the EXISTING row (read back + compare), never a new row (CONTEXT D-29-02).

**DDL-lock test pattern** (kpi_calls_test.go:42-45 + 73-107): `expectedKPICallColumns` slice vs `PRAGMA table_info` ordering, plus per-index existence check via `sqlite_master`:
```go
var idxCount int
require.NoError(t, s.db.QueryRowContext(ctx,
    `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&idxCount))
assert.Equal(t, 1, idxCount, "%s must exist on a fresh DB", idx)
```

---

### `internal/store/sqlite/session.go` + `internal/store/store.go` (AppendTurn signature change — Pitfall 1 + 2)

**Analog:** same file. Current signature (store.go:604-607):
```go
// AppendTurn atomically appends a (user, assistant) turn pair for
// sessionID. Idempotent on (sessionID, user.Ts, assistant.Ts) via
// a UNIQUE constraint + ON CONFLICT DO NOTHING.
AppendTurn(ctx context.Context, sessionID string, user, assistant SessionMessage) error
```
New: `(int64, error)` returning the assistant row id. `SessionMessage` gains `ID int64` (store.go:455-461) so `GetSession` can carry `session_turn.id` into history.

**Transaction + half-insert pattern to modify** (session.go:94-131):
```go
tx, err := s.db.BeginTx(ctx, nil)
...
if err := appendTurnHalf(ctx, tx, sessionID, user); err != nil { ... }
if err := appendTurnHalf(ctx, tx, sessionID, assistant); err != nil { ... }
```
```go
func appendTurnHalf(ctx context.Context, tx *sql.Tx, sessionID string, m store.SessionMessage) error {
	...
	if _, err := tx.ExecContext(ctx, appendTurnSQL, sessionID, role, m.Content, ts.UTC()); err != nil {
```
Change: capture `res` from `ExecContext`; `rows, _ := res.RowsAffected(); if rows == 0 { return 0 }` (Pitfall 2 — `LastInsertId()` after a conflict DO NOTHING returns the CONNECTION's last successful insert rowid, a silent mis-attribution); else `res.LastInsertId()`.

**getSessionSQL must gain the id column** (session.go:33-38):
```go
const getSessionSQL = `
SELECT role, content, ts
FROM session_turn
WHERE session_id = ?
ORDER BY ts ASC
`
```
→ `SELECT id, role, content, ts ...` + Scan into `&m.ID` (session.go:57-62).

**Fake-store ripple (MUST be in the same commit as the signature change)** — exactly **5** explicit AppendTurn impls (per-method impls, none embed, so none inherit the new method):
- `internal/server/admin_audit_test.go:76` (`fakeAdminAuditStore.AppendTurn`), `:220` (`spyAdminAuditStore.AppendTurn`) — both files assert `var _ store.Store` at `:200` / `:344`
- `internal/server/admin_schedules_test.go:84`, `:226` (+ `var _` at `:199` / `:341`)
- `internal/server/upload_test.go:74` (`mockStore`)
- Correction (checker warning-5): `internal/server/kb_admin_test.go:114` (`recordingAuditStore`) **embeds `store.Store`** and contains zero AppendTurn references — it satisfies the new signature through the embedded interface and needs NO edit. Explicit-fake count is 5, not 6.
- Plus `writeAuditAndTurn` call sites — 12 total, grep-verified: chat.go:317/:449/:461/:483, pipeline.go:226/:282/:307/:347/:515/:528, runner_setup.go:215/:232 — and `internal/store/store_test.go:245` `TestAppendTurnIdempotent`.

---

### `internal/ticket/*` (service, provider registry + outbound webhook)

**Interface analog:** `internal/retriever/backend.go:23-36` — the narrow-interface + not-wired-constant discipline:
```go
// IndexerBackend is the narrow insert-only interface for the document upload pipeline.
// It is deliberately minimal: only Insert is needed to append new chunks.
type IndexerBackend interface {
	Insert(ctx context.Context, docs []Document) error
}

// IndexerBackendNotWiredError is the 503 body when Deps.IndexerBackend is nil.
const IndexerBackendNotWiredError = "indexer backend not wired (server started without a retriever backend)"
```
`internal/ticket/ticket.go` mirrors this shape: `Provider` interface with one `Dispatch(ctx, Ticket) (DispatchResult, error)` method + a `notWiredError`-style constant for the 503 body.

**Registry analog:** `cmd/server/main.go` backend switch (main.go:269-271 area + milvus/mongo imports) — config-string → concrete impl, unknown value = fatal. Per D16-08-D (CONTEXT canonical_refs), an unknown `SKILLFORGE_TICKET_PROVIDER` must `log.Fatalf`, never silently degrade to `none`.

**Webhook provider analog:** `internal/embedder/bge/bge.go` — per-instance client + bounded retry loop.

Per-instance client + transport isolation (bge.go:74-98):
```go
type BGE struct {
	cfg    Config
	client *http.Client
}
var _ embedder.Embedder = (*BGE)(nil)

func New(cfg Config) *BGE {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
	}
	return &BGE{cfg: cfg, client: &http.Client{Transport: transport}}
}
```

Bounded retry with ctx-aware 1s backoff (bge.go:120-136):
```go
var lastErr error
for attempt := 0; attempt < embedRetries+1; attempt++ {
	if attempt > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	vec, err := embedOnce(ctx, b.client, b.cfg.URL, ...)
	if err == nil {
		return vec, nil
	}
	lastErr = err
}
return nil, fmt.Errorf("BGE.Embed failed after %d attempts: %w", embedRetries+1, lastErr)
```

Per-attempt timeout + request build (bge.go:152-159):
```go
reqCtx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
...
req.Header.Set("Content-Type", "application/json")
```
2xx = success (CONTEXT discretion); payload only in the JSON body, secret only in a header (Security Domain: no user text in URL).

---

### `internal/wire/chunk.go` (mod — data-turn registration)

**Analog:** same file ValidTypes (chunk.go:428-456). Registration + the count-comment discipline:
```go
// Count: 23 types — text(5: ...) + reasoning(3) + tool-input(3) + tool-output(1)
// + source(2) + data-policy(1) + data-decision(1) + data-redline(1)
// + data-subprogress(1) + data-plan-trace(1) + finish(3: ...) + error(1).
var ValidTypes = map[string]bool{
	...
	"data-plan-trace":       true,
	"finish": true,
	...
}
```
Add `"data-turn": true` + bump the count comment to 24. **EmitChunk rejects unregistered types with only a WARN** (chunk.go:458-464 `IsValidType` + RESEARCH T-1-11) — forgetting registration is silent feature death.

**Payload struct analog:** `TurnPayload` mirrors how `DecisionPayload` rides `UIMessageChunk.Data json.RawMessage` — the emit-site pattern is `chat.go:605-611`:
```go
func emitDecision(c *app.RequestContext, p wire.DecisionPayload) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal decision: %w", err)
	}
	return wire.EmitChunk(c, wire.UIMessageChunk{Type: "data-decision", Data: payload})
}
```
Copy as `emitTurnID` (guard `turnID <= 0` → return nil, the honest-degradation path).

---

### `internal/server/pipeline.go` + `chat.go` (mod — emission window)

**Analog:** same file. The verified window (pipeline.go:519-583):
```go
// Step 11: SSE emit
emitErr := emitVerdictStream(c, v, chunks, s.KBRegistry)
s.writeAuditAndTurn(ctx, sessionID, skillID, query, v, skillID, "", meta)   // :528
if emitErr != nil { return ... }
// Step 12: counter-evidence + redline (both best-effort)                // :554-559
// Terminal emits:
if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "text-end", ID: "msg-1"}); err != nil { ... }  // :572
if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish-step"}); ...                          // :576
if err := wire.EmitChunk(c, wire.UIMessageChunk{Type: "finish"}); ...                                // :580
```
data-turn goes between `writeAuditAndTurn` (:528) and `text-end` (:572). `writeAuditAndTurn` (chat.go:512-563) is the function to modify so it returns the assistant turn id; the agent path's mirrored call sites are chat.go:483 and :515. Note the pipeline's terminal-emit error posture (pipeline.go:568-571: "Terminal emits are best-effort... must NOT propagate") — data-turn emission failure should log WARN + continue, never 500 after the audit row landed.

---

### `internal/server/kb_admin.go` (analog for feedback.go + tickets.go handlers)

**Analog:** whole file. Three load-bearing patterns.

**Admin gate first, 403 before 503** (kb_admin.go:136-146 — identical gate also at :167-176 and :241-248):
```go
func (s *Deps) kbAdminHandler(ctx context.Context, c *app.RequestContext) {
	// CHECK ORDER: admin check FIRST — a 403 must not leak whether the
	// KB registry is wired.
	if !isAdminRequest(c) {
		c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
		return
	}
	if s.KBRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: kbRegistryNotWiredError})
		return
	}
```
`isAdminRequest` itself (audit.go:114-129): `os.Getenv("SKILLFORGE_ADMIN_TOKEN")` + `X-Admin-Token` header walk + `subtle.ConstantTimeCompare(...) == 1`. Every new admin handler (review / dispositions / metrics) copies the gate VERBATIM as its first statement; hermetic tests assert 403 even with nil deps (Pitfall 8).

**Public endpoint nil-dependency 503** (kb_admin.go:296-300, `knowledgeBasesHandler`) — the model for `POST /api/tickets` when no TicketProvider is wired (provider=none still stores; nil Deps field → 503 per the Deps godoc convention, server.go:212-244).

**Body decode + closed-vocabulary validation + INSERT-only audit row** (kb_admin.go:179-198, 330-348):
```go
var body kbCreateBody
if err := c.Bind(&body); err != nil {
	c.JSON(http.StatusBadRequest, graphResponseError{Error: "invalid body: " + err.Error()})
	return
}
if !kb.IsRole(body.Role) {
	c.JSON(http.StatusBadRequest, graphResponseError{
		Error: fmt.Sprintf("role %q must be one of %v ...", body.Role, kb.Roles()),
	})
	return
}
```
```go
func (s *Deps) putKBAdminAuditRow(ctx context.Context, action string, entry kb.Entry) {
	if s.Store == nil {
		return
	}
	payload, err := json.Marshal(entry)
	...
	if _, err := s.Store.PutAudit(ctx, store.AuditRow{
		Timestamp:  time.Now().UTC(),
		UserID:     "admin",
		Action:     action,
		ActorType:  "admin",
		FrozenArgs: payload,
	}); err != nil {
		log.Printf("[WARN] server: kb audit row write ... (best-effort, mutation not rolled back)")
	}
}
```
Dispositions + ticket-dispatch audit rows copy this helper's shape (`action_feedback_disposition`, `action_ticket_dispatch_succeeded|failed`; FrozenArgs = JSON snapshot; ActorType="admin" so `/api/admin/audit` surfaces them). Best-effort, log-not-500.

**Async-dispatch skeleton for tickets.go:** the handler returns 201 immediately; the goroutine uses `context.WithTimeout(context.Background(), budget)` (NOT the request ctx — Pitfall 6) + `defer recover()` + audit row on failure. Response-envelope analog: `graphResponseError{Error: ...}` is the shared error shape (kb_admin.go:140).

**Input validation for feedback/tickets:** session ids through the existing `sessionIDRegex` (handlers_session_history.go:43, :115-121 — validate BEFORE any SQL); verdict/disposition against closed Go enums mirrored by DB CHECK constraints; turn_id strictly int64 > 0 + existence + `role='assistant'` + session binding (`WHERE session_id=? AND id=?`).

---

### `internal/server/handlers_session_history.go` (mod — turn_id + voted_verdict)

**Analog:** same file. Extend the wire struct (handlers_session_history.go:55-60):
```go
type historyEntry struct {
	Role    string         `json:"role"`
	Content string         `json:"content"`
	Verdict *verdict.Verdict `json:"verdict,omitempty"`
	Ts      int64          `json:"ts"`
}
```
Add `TurnID int64 \`json:"turn_id,omitempty"\`` + `VotedVerdict string \`json:"voted_verdict,omitempty"\``. `renderRow` (handlers_session_history.go:207-305) is the per-row construction site — all four assistant branches need the new fields populated. `voted_verdict` comes from one feedback lookup for the session (JOIN in the store read or a batch map), NOT a per-row query.

The FE mirror MUST stay snake_case — chat-restore.ts header discipline (chat-restore.ts:9-13): "The JSON tag casing is snake_case (matching the Go server) — DO NOT camelCase."

---

### `internal/server/server.go` + `cmd/server/main.go` (wiring)

**Deps field convention** (server.go:212-214 is the canonical nil-contract godoc):
```go
// Store persists audit rows + session turns (Plan 03-02). nil =
// degraded mode (responses still stream, nothing persisted).
Store store.Store
```
Add `TicketProvider ticket.Provider` + `TicketProviderName string` (+ feedback store access rides the existing `Store`) with the same nil-semantics godoc.

**Route registration** (server.go:483-527 — the flat `h.POST/h.GET` block; comment-block precedent at :509-519):
```go
h.GET("/api/admin/kb", deps.kbAdminHandler)
h.POST("/api/admin/kb", deps.kbCreateHandler)
h.POST("/api/admin/kb/:slug/deactivate", deps.kbDeactivateHandler)
```
Register `POST /api/feedback`, `POST /api/tickets`, `GET /api/admin/feedback/review`, `POST /api/admin/feedback/dispositions`, `GET /api/admin/feedback/metrics` in the same block with the same comment-block style.

**env wiring analog** (main.go:156-161, 179-192):
```go
func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func parseIntEnvOrDefault(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("[WARN] cmd/server: %s=%q invalid (expected positive integer); using default %d", name, v, def)
		return def
	}
	return n
}
```
New knobs (all zero-hardcode, Phase-30-greppable): `SKILLFORGE_TICKET_PROVIDER` / `_WEBHOOK_URL` / `_WEBHOOK_TIMEOUT_MS` / `_WEBHOOK_RETRIES`. Bad provider NAME is fatal (registry error → `log.Fatalf`); bad numeric values WARN + default (parseIntEnvOrDefault posture).

---

### `web/src/types/wire-types.ts` (mod — D15 paired union member)

**Analog:** same file. The union entry shape (wire-types.ts:423, 441):
```ts
| { type: 'data-plan-trace'; id?: string; data: PlanTracePayload }
...
| { type: 'data-redline'; id?: string; data: RedlinePayload }
```
Add the payload interface (mirroring `PlanTracePayload`, wire-types.ts:370-374 — camelCase, 1:1 with the Go struct) and the union member:
```ts
export interface TurnPayload { turnId: number; }
| { type: 'data-turn'; id?: string; data: TurnPayload }
```
The compile-time drift alarm needs zero changes (wire-types.ts:455-456):
```ts
type AssertAssignable<A, B> = A extends B ? (B extends A ? true : never) : never;
type _driftCheck = AssertAssignable<SkillForgeChunk, UIMessageChunk>;
```
D15 discipline: Go `ValidTypes["data-turn"]` + this TS member land in the SAME commit (wire-types.ts:4-8 header + chunk.go:424-427).

---

### `web/src/components/chat/MessageBubble.tsx` + `chat-restore.ts` (mod — part scan + restore synthesis)

**Part-scan analog** (MessageBubble.tsx:272-288) — the exact pattern FeedbackRow props extraction copies:
```tsx
const chatText = message.parts
  .filter((p) => p.type === 'text')
  .map((p) => (p as { text: string }).text)
  .join('');
const policyData = (
  message.parts.find((p) => p.type === 'data-policy') as
    | { data?: unknown }
    | undefined
)?.data;
```
Mount point (MessageBubble.tsx:305):
```tsx
<FeedbackRow messageId={message.id} />
```
becomes `turnId`/`votedVerdict` props pulled from a `data-turn` part scan; `chatText` (:272-275) is also the prefill source for the ticket description (≤200 chars).

**Restore synthesis analog** (chat-restore.ts:109-143) — assistant rows already synthesize a `data-policy` part; add the same for `data-turn`:
```tsx
if (m.verdict) {
  return {
    id,
    role: 'assistant' as const,
    parts: [
      { type: 'text', text: m.content },
      { type: 'data-policy', data: m.verdict },
    ],
  };
}
```
So live-stream and history-restore converge on ONE FE consumption point. Extend `SessionHistoryMessage` (chat-restore.ts:32-39) with `turn_id?: number; voted_verdict?: 'helpful'|'unhelpful'|'outdated'` (snake_case per the header rule). `message.id` (`${sessionId}-${m.ts}-${m.role}`, chat-restore.ts:114) stays React-key-only — never a vote identifier (Pitfall 7).

---

### `web/src/components/chat/parts/FeedbackRow.tsx` (rewrite — component, request-response)

**Analog:** the current placeholder file itself (85 lines). Keep the skeleton, replace the action layer.

**Keep** — the single-select lock structure and testid suite (FeedbackRow.tsx:23-27, 56-71):
```tsx
const VOTE_META: Array<{ key: Vote; testid: string; label: string; Icon: typeof ThumbsUp }> = [
  { key: 'helpful', testid: 'feedback-helpful', label: '有帮助', Icon: ThumbsUp },
  { key: 'unhelpful', testid: 'feedback-unhelpful', label: '没帮助', Icon: ThumbsDown },
  { key: 'outdated', testid: 'feedback-outdated', label: '报告知识过时', Icon: FileWarning },
];
...
<div className="feedback-row" data-testid="feedback-row" data-message-id={messageId} data-voted={vote ?? ''}>
```
```tsx
<button ... onClick={() => cast(key)} disabled={vote !== null && vote !== key}
  aria-pressed={vote === key}
  className={cn('feedback-row-btn', vote === key && 'feedback-row-btn-active')}>
```

**Replace** — the toast placeholder (FeedbackRow.tsx:36-38) becomes a real POST:
```tsx
function fireToast(kind: Vote | 'ticket') {
  window.dispatchEvent(new CustomEvent('sf-toast', { detail: { kind, message: TOAST_TEXT[kind] } }));
}
```
Props change (FeedbackRow.tsx:40-45): `messageId: string` → `turnId: number | null, votedVerdict: Vote | null` (server truth). New state machine (UI-SPEC §Component-1): idle → submitting → recorded / already_recorded / failure-unlocked; `turnId == null` → disabled + `title="该消息缺少服务端轮次标识，暂时无法反馈"`. Fetch style: plain `fetch('/api/feedback', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(...)})` — same raw-fetch posture as chat-restore.ts:70-80 (public endpoint, no admin token).

---

### `web/src/components/layout/SfToast.tsx` (new — event-driven viewport)

**Analog:** `web/src/App.tsx` — BOTH halves live here today.

Listener precedent (App.tsx:301-309):
```tsx
useEffect(() => {
  function onSuccess(e: Event) {
    const detail = (e as CustomEvent<{ skillName: string; path: string }>).detail;
    if (!detail) return;
    setLastCreatedSkill({ name: detail.skillName, path: detail.path, ts: Date.now() });
  }
  window.addEventListener('wizard-success', onSuccess);
  return () => window.removeEventListener('wizard-success', onSuccess);
}, [setLastCreatedSkill]);
```
Markup precedent (App.tsx:378-401) — SfToast mirrors this exactly:
```tsx
<div data-testid="wizard-success-toast" role="status" aria-live="polite"
  className="fixed bottom-4 right-4 z-40 rounded-md border border-status-compliant/40 bg-surface-2 px-4 py-3 shadow-lg">
  <div className="flex items-center gap-2 text-xs font-semibold text-status-compliant">
    <span>Skill 已创建：{lastCreatedSkill.name}。已注册并热加载。</span>
  </div>
  ...
  <button type="button" data-testid="wizard-success-toast-close" ...
    className="absolute right-1 top-1 inline-flex h-6 w-6 items-center justify-center rounded-md text-text-muted hover:bg-surface-3"
    aria-label="关闭">×</button>
</div>
```
Existing dispatcher contract to stay compatible with (FeedbackRow.tsx:37; SessionDrawer.tsx:170 dispatches `kind:'error'`): `CustomEvent('sf-toast', { detail: { kind, message } })`. Component owns listener + viewport and is mounted in all three App branches + once inside AdminDashboard's root (because `/admin` early-returns at App.tsx:343-345, an App-level listener alone would not cover admin toasts).

---

### `web/src/components/admin/AdminDashboard.tsx` + `app-store.ts` (mod — tab wiring)

**Analog:** same file. Three coordinated edits, all mirrored from the existing five tabs.

1. Union (app-store.ts:457 and its setter signature :521):
```ts
adminSubroute: 'batches' | 'bases' | 'kpis' | 'skills' | 'schedules';
...
setAdminSubroute(sub: 'batches' | 'bases' | 'kpis' | 'skills' | 'schedules'): void;
```
→ append `| 'feedback' | 'review'` in BOTH places (the setter at :854 is a pass-through, no change).

2. `handleSubroute` param type (AdminDashboard.tsx:49) widens to match; URL-sync useEffect (:58-63) stays (`'batches'` default, store-only subroute).

3. Tab + ViewBody branches (AdminDashboard.tsx:141-147, 169):
```tsx
<SubrouteTab active={adminSubroute === 'schedules'} testId="admin-tab-schedules"
  icon={<Clock className="h-3.5 w-3.5" />} label="定时任务"
  onClick={() => handleSubroute('schedules')} />
...
{adminSubroute === 'schedules' && <ScheduledTaskList />}
```
Append 反馈 (`admin-tab-feedback`, 6th) and 复核 (`admin-tab-review`, 7th). Import icons from lucide-react (import line AdminDashboard.tsx:21).

---

### `web/src/components/admin/FeedbackMetricsPanel.tsx` + `ReviewQueuePanel.tsx` (new — admin request-response)

**Analog:** `web/src/components/admin/KnowledgeBaseList.tsx` — whole-component shape.

Token'd read (KnowledgeBaseList.tsx:87-117):
```tsx
const loadBases = useCallback(() => {
  const token = localStorage.getItem('sf_admin_token') ?? '';
  return fetch('/api/admin/kb', { headers: { 'X-Admin-Token': token } })
    .then((resp) => {
      if (!resp.ok) { setError(true); setLoading(false); return null; }
      return resp.json() as Promise<KBAdminResponse>;
    })
    .then((d) => { if (d) { setData(d); setError(false); } setLoading(false); })
    .catch(() => { setError(true); setLoading(false); });
}, []);

useEffect(() => { void loadBases(); }, [loadBases]);
```
Mutation + server-truth refetch + verbatim-error honesty (KnowledgeBaseList.tsx:134-160):
```tsx
setBusy(true);
const token = localStorage.getItem('sf_admin_token') ?? '';
fetch('/api/admin/kb', {
  method: 'POST',
  headers: { 'X-Admin-Token': token, 'Content-Type': 'application/json' },
  body: JSON.stringify({ slug, display_name: displayName, role, description }),
})
  .then(async (resp) => {
    if (resp.ok) { ...; await loadBases(); // 201 → refresh the list from the server truth
      return; }
    const body = (await resp.json().catch(() => ({}))) as { error?: string };
    if (resp.status === 409) { setActionError('slug 已存在'); }
    else if (resp.status === 403) { setActionError('创建失败：缺少或错误的 X-Admin-Token'); }
    ...
```
Dispositions copy this exactly (busy per-row, 201 → refetch pending list, error rendered verbatim). Loading/empty primitives (KnowledgeBaseList.tsx:54-71): `EmptyState` (py-12) and `LoadingSkeleton` (3 surface rows) are copy-paste starting points.

Wire-shape header comment precedent (KnowledgeBaseList.tsx:15-19) — each new panel opens with the frozen wire contract:
```tsx
// Wire shapes (26-05, frozen):
//   GET  /api/admin/kb                  → { signature, bases: KBEntry[] }
//   POST /api/admin/kb                  body {slug,display_name,role,description}
//                                       → 201 | 400 {error} | 403 | 409 | 503
```

**Trend chart:** reuse `Sparkline` as-is (Sparkline.tsx:16-19) — `points: SparklinePoint[]` (`{index, period, value}`), `onPointHover` required; series color via the existing `var(--color-sparkline, var(--color-primary))` default or per-row inline `stroke` (UI-SPEC §Color: compliant/non-compliant/needs-info tokens). No new chart dependency (D17-B1). 0-vote cells render `—` (server returns null ratios; FE never computes 0%).

---

### `scripts/verify-phase-29.sh` + `Makefile` (structural hygiene gate)

**Analog:** `scripts/verify-phase-28.sh` (whole file) + Makefile `phase-28` target (Makefile:98-103).

Script shape (verify-phase-28.sh:12-31, 70):
```bash
set -uo pipefail

phase_28_files=(
  "internal/knowledge/graph_edges.go"
  ...
)

echo "=== Stage 1: TODO/FIXME clear ==="
TODO_COUNT=$(grep -rE "TODO|FIXME" --include="*.go" "${phase_28_files[@]}" 2>/dev/null | grep -v "_test.go" | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
  echo "FAIL: $TODO_COUNT TODO/FIXME in Phase 28 production files:"
  ...
  exit 1
fi
echo "PASS: no TODO/FIXME in Phase 28 production files"
...
echo ""
echo "=== Phase 28 verify PASS ==="
```
Stage-2-style targeted single-file grep (verify-phase-28.sh:43-49):
```bash
EDGES_CITABLE=$(grep -cE "kb\.Citable" internal/knowledge/graph_edges.go 2>/dev/null)
if [ "$EDGES_CITABLE" -gt 0 ]; then
  echo "FAIL: ..."
  exit 1
fi
```
Phase 29 stages (per RESEARCH §Validation Architecture): (1) TODO/FIXME clear over phase files, (2) FBK-05 source-grep — `internal/server/feedback.go` / `internal/store/sqlite/feedback.go` contain zero retriever/KB mutation verbs (`UPDATE `, `DELETE `, `Insert(`, index/milvus imports), (3) ValidTypes↔TS pairing — `data-turn` present in BOTH `internal/wire/chunk.go` and `web/src/types/wire-types.ts`, (4) feedback/tickets DDL uses `DO NOTHING` and never `DO UPDATE`.

Makefile target (Makefile:98-103):
```make
phase-28:
	@echo "Running Phase 28 verification..."
	@bash scripts/verify-phase-28.sh
	@echo "Phase 28 verification passed"
```
Copy as `phase-29`; add `phase-29` to the `.PHONY` list (Makefile:9).

## Shared Patterns

### INSERT-only + source-grep structural lock
**Source:** `internal/store/sqlite/kpi_calls.go` + `kpi_calls_test.go:152-162`
**Apply to:** `feedback.go`, `tickets.go` (store layer) + their test files
```go
assert.NotContains(t, source, "UPDATE ", "...")
assert.NotContains(t, source, "DELETE ", "...")
```
Only `ON CONFLICT(...) DO NOTHING`; `DO UPDATE` variants and even prose "UPDATED" in comments risk tripping the lock (RESEARCH Pitfall 5). Dispatch results are recorded as NEW audit rows, never ticket-row UPDATEs.

### Admin gate (403 before 503) + INSERT-only audit row
**Source:** `internal/server/kb_admin.go:139-146` (gate), `audit.go:114-129` (isAdminRequest), `kb_admin.go:330-348` (audit row)
**Apply to:** review / dispositions / metrics handlers, ticket-dispatch goroutine
```go
if !isAdminRequest(c) {
    c.JSON(http.StatusForbidden, graphResponseError{Error: "admin token required"})
    return
}
if s.Xxx == nil { c.JSON(http.StatusServiceUnavailable, ...); return }
```
```go
s.Store.PutAudit(ctx, store.AuditRow{Timestamp: time.Now().UTC(), UserID: "admin",
    Action: action, ActorType: "admin", FrozenArgs: payload})
```
Best-effort (log WARN, never roll back the mutation); `ActorType:"admin"` makes rows surface in `/api/admin/audit`.

### Idempotent write = UNIQUE constraint + ON CONFLICT DO NOTHING + RowsAffected guard
**Source:** `internal/store/sqlite/session.go:73-77` (clause) + RESEARCH Pitfall 2
**Apply to:** feedback votes, tickets, AppendTurn turn-id capture
`RowsAffected == 0` → read back the existing row → return `{status:"already_recorded"|"already_exists"}` with the stored verdict/ticket number. Never trust `LastInsertId()` after a conflict.

### Best-effort outbound HTTP with per-instance client + bounded retry
**Source:** `internal/embedder/bge/bge.go:90-136`
**Apply to:** `internal/ticket/webhook.go` + the dispatch goroutine in `internal/server/tickets.go`
Per-instance `*http.Client` (never DefaultClient); per-attempt `context.WithTimeout`; `retries+1` attempts with ctx-aware 1s backoff; goroutine uses `context.Background()` + total budget + `defer recover()` (RESEARCH Pitfall 6).

### Wire-type D15 paired commit
**Source:** `internal/wire/chunk.go:424-456` + `web/src/types/wire-types.ts:4-8, 380-456`
**Apply to:** `data-turn` (Go ValidTypes entry + count comment 23→24) and the TS union member land in ONE commit; `_driftCheck` compile assertion is the guard.

### FE part scanning (one consumption point for live + restored messages)
**Source:** `web/src/components/chat/MessageBubble.tsx:272-288` (verdictStatus) + `web/src/lib/chat-restore.ts:127-136` (data-policy synthesis)
**Apply to:** data-turn consumption by FeedbackRow; chat-restore synthesizes the same part so restore needs no special case. Snake_case wire mirror discipline (chat-restore.ts:9-13).

### Admin FE panel: raw fetch + localStorage token + server-truth refetch + verbatim errors
**Source:** `web/src/components/admin/KnowledgeBaseList.tsx:87-160`
**Apply to:** `FeedbackMetricsPanel.tsx`, `ReviewQueuePanel.tsx`
`localStorage.getItem('sf_admin_token')`, `X-Admin-Token` header, `.then/.catch` chain with `loading/error/data` trio, post-mutation `await loadX()` refetch, backend `error` strings rendered verbatim (KnowledgeBaseList honesty rule, :21-27).

### Component-local UI state, no new zustand fields
**Source:** `web/src/components/admin/KnowledgeBaseList.tsx:74-85` (all `useState`)
**Apply to:** both new admin panels (UI-SPEC §Interaction Behaviors: "component-local useState, no zustand fields"); the ONLY store change is the `adminSubroute` union widening.

### Window CustomEvent toast channel
**Source:** `web/src/App.tsx:301-309` (listener), `App.tsx:378-401` (viewport markup), `FeedbackRow.tsx:37` + `web/src/components/layout/SessionDrawer.tsx:170` (dispatchers)
**Apply to:** `SfToast.tsx` — component owns listener + viewport; lenient kind mapping (`error`→error, `success`/`compliant`→success, else info).

### Verify script + make target
**Source:** `scripts/verify-phase-28.sh`, `Makefile:98-103`
**Apply to:** `scripts/verify-phase-29.sh` + `phase-29` target; `set -uo pipefail`, per-stage PASS/FAIL echo, `exit 1` on first failure.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | — | — | Every planned file copies an existing in-repo precedent. `internal/ticket/registry.go` is the weakest match (no existing provider-registry-by-config-string in one place; the milvus/mongo backend switch lives inline in `cmd/server/main.go`) — planner should lift that switch into `internal/ticket.NewProvider` while keeping the fatal-on-unknown posture. |

## Metadata

**Analog search scope:** `internal/store/sqlite/`, `internal/server/`, `internal/wire/`, `internal/retriever/`, `internal/embedder/bge/`, `internal/knowledge/`, `cmd/server/`, `web/src/components/{chat,chat/parts,admin,layout}/`, `web/src/lib/`, `web/src/types/`, `web/src/store/`, `scripts/`, `Makefile`
**Files read in full or in relevant part:** 22 (schema.go, kpi_calls.go, kpi_calls_test.go, session.go, store.go ×3 ranges, kb_admin.go, handlers_session_history.go, chat.go, pipeline.go, wire/chunk.go, retriever/backend.go, bge/bge.go, server.go ×2 ranges, main.go, audit.go, verify-phase-28.sh, FeedbackRow.tsx, chat-restore.ts, wire-types.ts, MessageBubble.tsx, AdminDashboard.tsx, KnowledgeBaseList.tsx, Sparkline.tsx, App.tsx)
**Pattern extraction date:** 2026-09-08
**Codebase HEAD:** 6852e70 (matches RESEARCH.md's anchor commit; file:line anchors verified against working tree)
