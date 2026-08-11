---
phase: 03-agentic-retrieval-trust-evaluation
plan: 02
subsystem: store
tags: [store, sqlite, audit, session-memory, gevl-03, agnt-05, sc-7, sc-9, tdd, d11]
requires:
  - "Task 1 checkpoint approval: modernc.org/sqlite@v1.56.0 (legitimacy audit APPROVED by orchestrator)"
  - "Phase 1 go.mod at Go 1.25.0 (orchestrator prep commit 5db74da)"
provides:
  - "internal/store.Store interface + AuditRow + SessionMessage + AuditFilter types (the D11 store abstraction)"
  - "internal/store/sqlite.SQLiteStore — zero-ops default impl (pure-Go modernc.org/sqlite, no cgo)"
  - "Compile-time D11 conformance check: var _ store.Store = (*SQLiteStore)(nil)"
  - "audit table schema (15 cols, 4 indexes) + session_turn schema (UNIQUE session_id+role+ts)"
  - "DB() accessor for operator-side SC-7 SQL forensics (chunk_ids LIKE queries)"
affects:
  - "Plan 03-05 (server integration): chat handler wires Store.PutAudit + GetSession + AppendTurn"
  - "Plan 03-06 (eval sidecar): reads audit rows via the same AuditRow schema for JSONL"
  - "Future PG backend: drops in alongside internal/store/sqlite without caller changes"
tech-stack:
  added:
    - "modernc.org/sqlite@v1.56.0 (pure-Go SQLite, BSD-3-Clause, no cgo, driver name 'sqlite')"
  patterns:
    - "TDD RED/GREEN per task (3 commits: 1 RED + 1 GREEN + 1 verify/harden)"
    - "D11 interface-first (mirrors internal/retriever): interface in parent package, impl in subpackage, compile-time check"
    - "INSERT + SELECT only — no UPDATE path anywhere (T-3-AUDIT anti-tampering)"
    - "Dynamic WHERE clause with parameterized ? placeholders (T-3-SQLI); strings.Join only over hardcoded literals, NEVER user input"
    - "Mutex-guarded closed flag → post-Close error contract on all 5 methods"
    - "UNIQUE(session_id, role, ts) + ON CONFLICT DO NOTHING → AppendTurn idempotency"
key-files:
  created:
    - internal/store/store.go
    - internal/store/sqlite/sqlite.go
    - internal/store/sqlite/schema.go
    - internal/store/sqlite/audit.go
    - internal/store/sqlite/session.go
    - internal/store/store_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - "AuditFilter gained a SessionID field (not in the plan's <interfaces> block) so Plan 03-05/03-06 can scope a query to one conversation's full audit trail — addition-only, zero cost, matches the plan's key_links 'GetSession|AppendTurn' / 'AuditRow' consumer pattern"
  - "SQLiteStore exposes a DB() accessor (documented operator-side-only) so the SC-7 regression test can issue a raw `SELECT id FROM audit WHERE chunk_ids LIKE '%c4%'` and PROVE chunk_ids is a first-class queryable column, not a JSON_EXTRACT on VerdictJSON — this is the literal SC-7 acceptance criterion"
  - "WAL journal_mode tolerated as no-op on :memory: (modernc.org/sqlite rejects WAL on in-memory DBs); file-backed DSNs get real WAL for sub-ms GEVL-03 audit writes"
  - "MaxOpenConns defaults to 1 for :memory: (each conn otherwise gets its own private DB) and 4 for file DSNs (SQLite's file lock serializes writes; 4 readers is conservative)"
  - "T2 acceptance criterion 'no UPDATE in any SQL string' (grep returns 0) is honored literally: doc comments reworded to avoid the literal UPDATE token so the grep stays clean and the anti-tampering contract is machine-verifiable"
  - "Task 2 RED wrote ALL 6 named tests at full spec in one shot (the plan's 7 behaviors and Task 3's 6 named tests overlap completely); Task 3 then hardened TestHealthAndClose to lock the post-Close error contract on all 5 Store methods, not just 2"
metrics:
  duration: 18m
  completed: 2026-08-12
  tasks: 3
  files_created: 6
  files_modified: 2
  commits: 3
---

# Phase 03 Plan 02: Store interface + SQLite audit/session backend Summary

GEVL-03 / SC-7 (audit row SQL-queryable) + AGNT-05 / SC-9 (per-session memory with cross-session isolation) substrate unlocked: the `internal/store.Store` interface and a zero-ops pure-Go `SQLiteStore` default (modernc.org/sqlite@v1.56.0, no cgo) ship with the D11 interface-first shape (compile-time `var _ store.Store = (*SQLiteStore)(nil)` check mirroring retriever.Retriever + mongo). The audit table is INSERT+SELECT only (T-3-AUDIT anti-tampering); `QueryAudit` builds its WHERE dynamically with 100% `?` parameterized placeholders so `'; DROP TABLE audit; --` in any filter field is treated as a literal string (T-3-SQLI regression-locked); `GetSession`'s `WHERE session_id = ?` is the SQL-layer SC-9 isolation boundary; `AppendTurn` is idempotent via `UNIQUE(session_id, role, ts) + ON CONFLICT DO NOTHING`. Plan 03-05 (server) and Plan 03-06 (eval sidecar) are unblocked.

## Tasks Completed

### Task 1 — Verify modernc.org/sqlite package legitimacy (checkpoint)

**Outcome:** APPROVED by orchestrator (`approved: modernc.org/sqlite@v1.56.0`).

All 4 legitimacy criteria verified (import path exact, source gitlab.com/cznic/sqlite live, BSD-3-Clause license, CGo-free pure-Go with init() only calling sql.Register — no network). Authenticity anchored to `GOSUMDB=sum.golang.org`. No code change in this task (the install happens in Task 2). Orchestrator prep already committed as `5db74da` (Go 1.25.0 toolchain bump + `go get -u ./...`).

### Task 2 — Add modernc.org/sqlite dep + Store interface + SQLite backend (TDD)

**TDD cycle:**

| Phase | Commit | Subject |
|-------|--------|---------|
| RED   | `76787c2` | `test(03-02): add failing tests for Store interface + SQLite backend` |
| GREEN | `3e24bda` | `feat(03-02): implement Store interface + SQLite backend` |

**Dep install:** `go get modernc.org/sqlite@v1.56.0` → `go mod tidy` → `go list -m modernc.org/sqlite` returns `v1.56.0` (now a direct dependency). Blank import `_ "modernc.org/sqlite"` registers the driver under name `"sqlite"`.

**`internal/store/store.go` (interface, mirrors retriever.go):**

- Package doc states the D11 mirror: interface-only here, impl in subpackage, future backends alongside.
- `Store` interface godoc: "Implementations MUST be safe for concurrent use across goroutines (SC-9 -race test). All methods are ctx-aware for AGNT-04 cancel propagation."
- Exported types: `AuditRow` (15 fields, 4 nullable-metric `*float64`), `SessionMessage` (Role/Content/Ts), `AuditFilter` (UserID/SkillID/RefusalReason/SessionID/Since/Until/Limit — **SessionID added beyond the plan's interfaces block**, see Decisions).
- `PutAudit` godoc carries the GEVL-03 contract: "the row MUST exist before the response ends — synchronous write, SQLite WAL keeps this sub-ms".

**`internal/store/sqlite/sqlite.go` (impl entry, mirrors mongo.go):**

- `Config{DSN string; MaxOpenConns int}` with `withDefaults()` (DSN → `:memory:`, MaxOpenConns → 1 for `:memory:`, 4 for file — see Decisions).
- `SQLiteStore struct{cfg Config; db *sql.DB; closed bool; mu sync.Mutex}`.
- `New(ctx, cfg)`: `sql.Open("sqlite", cfg.DSN)` → `SetMaxOpenConns` → PRAGMA journal_mode=WAL (tolerated as no-op on :memory:) → PRAGMA foreign_keys=ON → `migrate()`.
- `var _ store.Store = (*SQLiteStore)(nil)` compile-time check at package level — the D11 contract.
- `Health(ctx)`: `SELECT 1` via `QueryRowContext().Scan()`; `isClosed()` guard returns `sql.ErrConnDone`.
- `Close()`: mutex-guarded `closed` flag (idempotent — second Close is a no-op) + `db.Close()`.
- `DB() *sql.DB` accessor (operator-side-only, documented) for SC-7 SQL forensics.

**`internal/store/sqlite/schema.go`:**

- `const schemaSQL` with two `CREATE TABLE IF NOT EXISTS` statements:
  - `audit` (15 cols, `id INTEGER PRIMARY KEY AUTOINCREMENT`, NOT NULL on session_id/skill_id/query/prompt_hash/model_version/verdict_json/ts, `refusal_reason`/`chunk_ids` with `DEFAULT ''`, 4 nullable REAL metrics) + 4 indexes (`session_id`, `skill_id`, `refusal_reason`, `ts`).
  - `session_turn` with `CHECK(role IN ('user','assistant'))` + `UNIQUE(session_id, role, ts)` + index on session_id.
- `migrate(ctx)` runs schemaSQL via `ExecContext`; idempotent.

**`internal/store/sqlite/audit.go`:**

- `PutAudit(ctx, row)`: single INSERT with 14 `?` placeholders → `res.LastInsertId()`.
- `QueryAudit(ctx, filter)`: dynamic WHERE built from non-empty filter fields; EVERY clause is a hardcoded parameterized literal (`"user_id = ?"`, etc.) joined via `strings.Join` — **the joined slice contains ONLY hardcoded literals, never user input** (T-3-SQLI). `Limit` defaults to 100 when ≤ 0. Nullable metrics scanned via `sql.NullFloat64` → `*float64`.
- Helpers `toNullFloat64` / `fromNullFloat64` for the metric round-trip.

**`internal/store/sqlite/session.go`:**

- `GetSession(ctx, sessionID)`: `WHERE session_id = ?` — the SC-9 isolation enforcement. Returns empty (non-nil) slice for unknown session_id.
- `AppendTurn(ctx, sessionID, user, assistant)`: `db.BeginTx` wraps both inserts; `ON CONFLICT(session_id, role, ts) DO NOTHING` is the idempotency mechanism. Empty session_id short-circuits with `errEmptySessionID`.

**Anti-tampering contract:** `grep -cE "\bUPDATE\b" internal/store/sqlite/*.go` returns 0 (the schema has INSERT + SELECT paths only; the one-shot eval-sidecar metrics backfill is the only sanctioned exception and lives in Plan 03-06, not here).

**Tests (`internal/store/store_test.go`, external package `store_test`, written in RED):** all 6 named tests from 03-VALIDATION.md at full spec:
- `TestAuditRowSQLQuery` (SC-7): 3 distinct rows; UserID / SkillID+time-window / RefusalReason filters; field round-trip; raw `SELECT id FROM audit WHERE chunk_ids LIKE '%c4%'` via `s.DB()` proves chunk_ids is a first-class column.
- `TestAuditRowSQLInjection` (T-3-SQLI): `'; DROP TABLE audit; --` injected into UserID, SkillID, AND RefusalReason; no error; 0 rows each time; **follow-up `QueryAudit({})` returns the original row** — the audit table survived (the regression-lock).
- `TestSessionIsolationConcurrent` (SC-9): two goroutines with `sync.WaitGroup` interleave 5 AppendTurns each for `sess-A` / `sess-B`; concurrent GetSession reads; each session sees exactly `2*turnsPerSession` of its own messages, ZERO leaks from the other; no -race warnings.
- `TestAppendTurnIdempotent`: replayed `AppendTurn(sess, user, asst)` with identical timestamps; `GetSession` returns exactly 2 rows, not 4.
- `TestHealthAndClose`: Health nil fresh; non-nil after Close; **all 5 methods** return errors after Close (Health/PutAudit/QueryAudit/GetSession/AppendTurn — locked by Task 3 hardening).
- `TestPromptHashFormat`: 64-char hex SHA-256 round-trips byte-identically; store treats it as an opaque string (the SHA-256 canon is the caller's responsibility).

### Task 3 — Verify named-test suite under -race + harden post-Close contract

Commit `0810a7b`.

- All 6 named tests verified green under `-race -v -timeout 60s` (per-test timings captured: 0.01–0.03s each; full suite ~3.4s under -race).
- **Hardened `TestHealthAndClose`** to lock the post-Close error contract on ALL five Store methods. The plan's done-criteria said "every method returns a meaningful error" — Task 2's test only checked Health + PutAudit; Task 3 added the QueryAudit / GetSession / AppendTurn assertions so the `isClosed()` guard is regression-locked for the full surface (defense-in-depth: a future refactor that drops the guard from one method now fails the test, not a production incident).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Health() *sql.Row vs error confusion**

- **Found during:** Task 2 GREEN build
- **Issue:** First implementation used `if err := s.db.QueryRowContext(ctx, "SELECT 1"); err != nil` — but `QueryRowContext` returns `*sql.Row`, not an error. Compiler caught it.
- **Fix:** Changed to `.Scan(&one)` and added a probe-value assertion (`one != 1` → error).
- **Files modified:** `internal/store/sqlite/sqlite.go`
- **Commit:** `3e24bda`

**2. [Rule 2 - Completeness] Added SessionID to AuditFilter**

- **Found during:** Task 2 GREEN (writing TestAuditRowSQLQuery's full-round-trip assertion)
- **Issue:** The plan's `<interfaces>` block omitted `SessionID` from `AuditFilter`, but the plan's own key_links (`internal/server/chat.go → Store.GetSession`) and the VALIDATION.md SC-7 narrative ("queryable via SQL filter") both imply per-conversation audit scoping. Without it, an operator cannot ask "show me the full audit trail of this one conversation" — a basic GEVL-03 forensics use case.
- **Fix:** Added `SessionID string` to `AuditFilter` (addition-only, zero cost). `QueryAudit` builds the parameterized clause the same way as the other filters. TestAuditRowSQLQuery's round-trip subtest uses it.
- **Files modified:** `internal/store/store.go`, `internal/store/sqlite/audit.go`
- **Commit:** `3e24bda`

**3. [Rule 2 - Completeness] Added DB() accessor for SC-7 forensics**

- **Found during:** Task 2 RED (writing TestAuditRowSQLQuery)
- **Issue:** Behavior #7 requires proving `SELECT id FROM audit WHERE chunk_ids LIKE '%c2%'` returns the seeded row — i.e. SC-7's "queryable via SQL as a first-class column" is verified by issuing a RAW query the store does NOT wrap. Without an accessor, the test cannot issue raw SQL.
- **Fix:** Added `func (s *SQLiteStore) DB() *sql.DB` with a godoc that mandates operator-side-only use (production callers go through Store methods).
- **Files modified:** `internal/store/sqlite/sqlite.go`
- **Commit:** `3e24bda`

**4. [Rule 2 - Documentation] Reworded doc comments to keep the UPDATE-grep machine-verifiable**

- **Found during:** Task 2 GREEN acceptance-criteria check
- **Issue:** The plan's acceptance criterion `grep -c "UPDATE" internal/store/sqlite/*.go` must return 0. Two doc comments ("no UPDATE statement", "NO UPDATE path") tripped the grep even though no UPDATE SQL exists.
- **Fix:** Reworded to "no row-modification statement" / "NO row-modification path". The grep is now clean and the anti-tampering contract is machine-verifiable as the plan intended.
- **Files modified:** `internal/store/sqlite/audit.go`, `internal/store/sqlite/schema.go`
- **Commit:** `3e24bda`

## TDD Gate Compliance

| Gate | Commit | Status |
|------|--------|--------|
| Task 2 RED   | `76787c2` (`test(03-02): add failing tests for Store interface + SQLite backend`) | ✅ |
| Task 2 GREEN | `3e24bda` (`feat(03-02): implement Store interface + SQLite backend`) | ✅ |
| Task 3 verify/harden | `0810a7b` (`test(03-02): verify SC-7/SC-9/SQLi suite under -race, harden post-Close contract`) | ✅ |

RED failed as expected (compile failure — `store` / `sqlite` packages did not exist). GREEN flipped all 6 named tests to pass under -race. Task 3 added the post-Close hardening assertions and re-verified the full suite under -race.

## Threat Model

| Threat | Disposition | Mitigation Landing |
|--------|-------------|--------------------|
| T-3-SQLI (Tampering) | mitigate | Task 2 + Task 3: every `QueryAudit` filter field is a `?` placeholder; `strings.Join` runs ONLY over hardcoded literals (never user input); `TestAuditRowSQLInjection` injects `'; DROP TABLE audit; --` into UserID, SkillID, RefusalReason and asserts the table survives + a follow-up `QueryAudit({})` returns the seeded row. `grep -cE "fmt\.Sprintf.*(WHERE|SELECT|INSERT)" internal/store/sqlite/*.go` returns 0. |
| T-3-AUDIT (Repudiation) | mitigate | Task 2: schema is INSERT + SELECT only — `grep -cE "\bUPDATE\b" internal/store/sqlite/*.go` returns 0. The eval-sidecar metrics backfill (the one sanctioned exception) is owned by Plan 03-06, not this package. |
| T-3-HASH (Repudiation) | mitigate | Task 2: `prompt_hash` is a hex string column; the SHA-256 canon (collision-resistant to 2^128) is the caller's responsibility; input = rendered system prompt + skill_id + model_version so cross-skill reuse does not collide. `TestPromptHashFormat` locks the 64-char-hex round-trip. |
| T-3-SESSION (Info Disclosure) | mitigate | Task 2 + Task 3: `GetSession` query is `WHERE session_id = ?` (SQL-layer enforcement); `TestSessionIsolationConcurrent` under `-race` proves two concurrent same-user sessions with different session_id see ONLY their own history. |
| T-3-SC (supply chain) | mitigate | Task 1: blocking `checkpoint:human-verify` BEFORE install — orchestrator APPROVED modernc.org/sqlite@v1.56.0 against all 4 legitimacy criteria; GOSUMDB=sum.golang.org verifies the module hash on `go get`. |

## Success Criteria

- ✅ **GEVL-03 / SC-7:** every audit row is queryable via SQL filter; `TestAuditRowSQLQuery` exercises UserID / SkillID+time-window / RefusalReason / SessionID filters + field round-trip; the raw `chunk_ids LIKE` query proves first-class-column queryability (NOT JSON_EXTRACT).
- ✅ **AGNT-05 / SC-9 (store layer):** two concurrent same-user sessions are isolated at the SQL `WHERE session_id = ?` clause; `TestSessionIsolationConcurrent` under `-race` confirms ZERO cross-session leakage.
- ✅ **T-3-SQLI regression:** `'; DROP TABLE audit; --` injected into 3 filter fields; the audit table survives (follow-up `QueryAudit({})` returns the seeded row).
- ✅ **D11 reviewer narrative:** interface-first (`internal/store.Store` in parent package) + SQLite default (`internal/store/sqlite`) + compile-time check (`var _ store.Store = (*SQLiteStore)(nil)`) — production-swappable for Postgres without changing any caller.
- ✅ **Plan 03-05 unblocked:** chat handler can wire `Store.PutAudit` + `GetSession` + `AppendTurn`.
- ✅ **Plan 03-06 unblocked:** eval sidecar reads audit rows via the same `AuditRow` schema (the 4 nullable metric `*float64` pointers are the backfill target).

## Known Stubs

None. The store is fully wired: all 6 methods implemented against a real SQLite backend; no placeholder data, no TODOs, no hardcoded mock returns. Every AuditRow field round-trips through PutAudit/QueryAudit. The 4 Ragas metric pointers are intentionally NULL at insert time (the eval sidecar backfills them in Plan 03-06 — this is the designed contract, not a stub).

## Self-Check: PASSED

- All 6 created source files present (`internal/store/store.go`, `internal/store/sqlite/{sqlite,schema,audit,session}.go`, `internal/store/store_test.go`).
- All 3 task commits present (`76787c2`, `3e24bda`, `0810a7b`).
- All acceptance-criteria greps return the expected count:
  - `go list -m modernc.org/sqlite` → `v1.56.0`
  - `var _ store.Store = (*SQLiteStore)(nil)` → 1
  - `func (s *SQLiteStore) PutAudit` → 1
  - `func (s *SQLiteStore) QueryAudit` → 1
  - `func (s *SQLiteStore) GetSession` → 1
  - `func (s *SQLiteStore) AppendTurn` → 1
  - `CREATE TABLE.*audit|CREATE TABLE.*session_turn` → 2
  - `fmt\.Sprintf.*(WHERE|SELECT|INSERT)` in `internal/store/sqlite/*.go` → 0
  - `\bUPDATE\b` in `internal/store/sqlite/*.go` → 0
  - `func TestAuditRowSQLInjection` → 1
  - `func TestSessionIsolationConcurrent` → 1
- `go build ./...` exits 0.
- `go test ./internal/store/ -count=1 -race -timeout 60s` exits 0 (~3.4s).
