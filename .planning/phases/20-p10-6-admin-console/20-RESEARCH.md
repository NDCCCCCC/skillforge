# Phase 20: Admin Console (P10.6) - Research

**Researched:** 2026-08-29
**Domain:** Go backend admin API (`/api/admin/*`) + React admin dashboard (`/admin`) with four tab views
**Confidence:** HIGH

## Summary

Phase 20 implements the admin console (`/admin`) that aggregates the full data chain into a read-only four-view dashboard. The backend reuses the existing `X-Admin-Token` gate pattern with `crypto/subtle.ConstantTimeCompare` (no new auth). Four new `GET /api/admin/*` endpoints serve each view: import batches, KPI definitions, Skill management, and scheduled tasks. The frontend reuses the `EvalDashboard` shell pattern with tab-based navigation (`route === 'admin'` in `App.tsx`). All admin mutations are INSERT-only audit rows tagged `actor_type: admin`. Phase 6's redaction mechanism (`redact()` in `audit.go`) is reused directly for ADM-06. The 4th frozen lock (`data.lock.json`) follows the established `*.lock.json` structure. Milestone wrap is a pure-text summary in `20-SUMMARY.md`.

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Layout:** Tab switching — horizontal Tab bar four-view, reuse `/eval` dashboard visual style, lowest retrofit cost
- **Milestone wrap:** Plain text memo written to `20-SUMMARY.md` — no extra files, just a summary paragraph + operator gates ledger in the phase SUMMARY
- **Skill eval result interaction:** Hover tooltip — hover to view overview, does not occupy list row height
- **Audit monitoring:** Capacity progress bars — per-table count/threshold for each table (`audit` / `report_runs` / `kpi_calls`), turns red badge on breach

### Out of Scope (Deferred Ideas)
- Authentication model (reused from `/eval` X-Admin-Token)
- Redaction mechanism (reused from Phase 6)
- Editing of KPI definitions or schedules (read-only UI, files are sole truth)
- Audit archival/retention policy beyond growth monitoring
- 4th frozen lock (`data.lock.json`) is a documentation + lock file decision, no new code

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| X-Admin-Token gate | Backend (`audit.go`) | — | `isAdminRequest()` is the single gate; reused by all admin endpoints |
| Four view data serving | Backend (new `GET /api/admin/*` handlers) | — | Backend owns all data retrieval; FE is display-only |
| Tab navigation / routing | Frontend (`App.tsx`) | Frontend (`app-store.ts`) | `route === 'admin'` branch; `adminSubroute` state in Zustand |
| Audit row insertion | Backend (admin handlers) | Store (`sqlite`) | All admin mutations INSERT-only audit rows; store persists |
| Redaction | Backend (`audit.go::redact`) | — | Read-time projection between `QueryAudit` and `c.JSON`; reused by admin audit view |
| Capacity monitoring | Frontend (UI bars) | Backend (count queries) | Per-table threshold queries; UI renders badges |
| 4th frozen lock | Disk (`.lock.json`) | — | Documentation artifact; no runtime code |

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADM-01 | `/admin` route + X-Admin-Token constant-time gate (reuses `/eval` mode, no new auth model) | `isAdminRequest()` in `audit.go:114` uses `crypto/subtle.ConstantTimeCompare` against `SKILLFORGE_ADMIN_TOKEN` env; existing pattern verified |
| ADM-02 | Import batch list view (status + preview row count + discarded raw volume) | `store.ListImportBatches` + `store.ImportBatchRow` already exist; `ImportBatchRow` has `RawSizeBytes` + `DesensitizedRowCount` + `DiscardTS` |
| ADM-03 | KPI definition list (function signature + last call time), UI read-only | `GET /api/admin/kpis` already implemented in `kpi_admin.go`; returns `signature`, `kpis[]`, `latest_calls` |
| ADM-04 | Skill management view (hot-reload status + last eval result + skill change audit rows) | `GET /api/skills` returns `skillItem{version, status}`; eval results from `eval/frozen/*.lock.json`; audit rows for skill changes |
| ADM-05 | Scheduled task list (last-run status + next-fire time) | `store.ListReportRuns` + `scheduler.Scheduler`; `ReportRunRow` has `Status`, `FiredAt`, `CronExpr` |
| ADM-06 | All admin actions produce audit rows (`actor_type: admin`) | New column `actor_type TEXT DEFAULT 'user'` on audit table; INSERT-only with admin tag |

## Standard Stack

No new external dependencies. All libraries are already in use.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/subtle` | stdlib | Constant-time token comparison | ASVS V6.2.1; already used in `audit.go:128` |
| `Tabs` component | existing | Tab navigation primitive | `web/src/components/ui/tabs.tsx` — state-managed, `role=tab`/`aria-selected` semantics |
| `EvalDashboard` shell | existing | Admin route pattern | `route === 'admin'` branch mirrors `route === 'eval'` pattern in `App.tsx:340` |

### No New Packages
All work reuses existing patterns. No new npm packages or Go modules needed.

## Architecture Patterns

### System Architecture

```
Browser                      Go Backend                        SQLite Store
  |                               |                                   |
  |-- GET /api/admin/* ---------->|-- isAdminRequest() gate -------->|
  |   (X-Admin-Token header)     |   subtle.ConstantTimeCompare    |
  |                               |                                   |
  |<-- 200 JSON ------------------|<-- QueryAudit / ListXxx -------->|
  |                               |                                   |
  |-- GET /admin ---------------->|-- React SPA route branch         |
  |   (React Router)              |   route === 'admin'               |
  |                               |                                   |
  |<-- Tab UI (4 views) --------<|-- Four view components           |
  |   ImportBatches              |   (read-only, no mutations)       |
  |   KPIDefinitions             |                                   |
  |   SkillManagement           |   All admin mutations INSERT        |
  |   ScheduledTasks            |   audit rows with actor_type:admin |
```

### Backend: Admin Endpoints Pattern

Each admin endpoint follows the same shape as `kpi_admin.go`:

1. **Gate:** `if !isAdminRequest(c) { 403 }`
2. **Data retrieval:** Call store method or in-memory registry
3. **Error handling:** Structured error responses with appropriate status codes
4. **Response:** JSON envelope with typed fields

New endpoints to implement:
- `GET /api/admin/batches` — `store.ListImportBatches`, returns `[]ImportBatchRow`
- `GET /api/admin/kpis` — **already exists** in `kpi_admin.go`
- `GET /api/admin/skills` — extend existing `GET /api/skills` with eval status overlay
- `GET /api/admin/schedules` — `store.ListReportRuns`, returns `[]ReportRunRow`

### Frontend: Admin Route Pattern

```typescript
// App.tsx — mirrors the eval route pattern
if (route === 'admin') {
  return <AdminDashboard />;
}

// AdminDashboard.tsx — mirrors EvalDashboard.tsx shell
function AdminDashboard() {
  const [subroute, setSubroute] = useState<'batches'|'kpis'|'skills'|'schedules'>('batches');
  return (
    <div>
      <header>Admin Console</header>
      <nav>
        <SubrouteTab active={subroute === 'batches'} onClick={() => setSubroute('batches')} />
        <SubrouteTab active={subroute === 'kpis'} onClick={() => setSubroute('kpis')} />
        <SubrouteTab active={subroute === 'skills'} onClick={() => setSubroute('skills')} />
        <SubrouteTab active={subroute === 'schedules'} onClick={() => setSubroute('schedules')} />
      </nav>
      <main>
        {subroute === 'batches' && <ImportBatchList />}
        {subroute === 'kpis' && <KPIDefinitionList />}
        {subroute === 'skills' && <SkillManagementList />}
        {subroute === 'schedules' && <ScheduledTaskList />}
      </main>
    </div>
  );
}
```

### Capacity Bar UI Pattern

```typescript
// CapacityBar.tsx — reused pattern for audit growth monitoring
interface CapacityBarProps {
  tableName: string;
  currentCount: number;
  threshold: number;  // e.g., 10000 for audit rows
}

function CapacityBar({ tableName, currentCount, threshold }: CapacityBarProps) {
  const pct = Math.min((currentCount / threshold) * 100, 100);
  const isBreach = currentCount >= threshold;
  return (
    <div className="flex items-center gap-2">
      <span className="text-xs">{tableName}</span>
      <div className="flex-1 h-2 bg-surface-2 rounded">
        <div
          className={cn('h-full rounded', isBreach ? 'bg-red-500' : 'bg-primary')}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className={cn('text-xs font-mono', isBreach && 'text-red-500 font-bold')}>
        {currentCount.toLocaleString()}/{threshold.toLocaleString()}
      </span>
      {isBreach && <span className="text-xs bg-red-500 text-white px-1 rounded">!</span>}
    </div>
  );
}
```

### Audit Row with actor_type

The audit table gets a new column:

```sql
ALTER TABLE audit ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user';
-- 'user' = normal query, 'admin' = admin console mutation
```

All INSERT paths for admin actions include `actor_type = 'admin'`. The audit table remains INSERT-only (T-3-AUDIT invariant).

### Redaction Reuse (ADM-06)

The existing `redact()` function in `audit.go:148` applies read-time retention projection. For the admin audit sub-view, reuse the same function with `isAdminRequest(c)` — admin token holders see raw rows; others see `[REDACTED:XXd+ ...]` sentinels. The function signature is:

```go
func redact(rows []store.AuditRow, retention time.Duration, now time.Time, isAdmin bool)
```

ADM-06 requires admin audit rows (actor_type=admin) to also go through the same redaction — which is already the case since `redact()` operates on the result set after `QueryAudit`, before `c.JSON`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|------------|-----|
| Admin auth | Custom token validation | `isAdminRequest()` from `audit.go:114` | Uses `crypto/subtle.ConstantTimeCompare` (ASVS V6.2.1); already tested and deployed |
| Tab component | Build new tab system | `web/src/components/ui/tabs.tsx` (`Tabs` component) | Already has pill/underline variants, controlled/uncontrolled mode, `role=tab` semantics |
| Redaction | Implement new redaction | `audit.go::redact()` | Already handles retention projection with `[REDACTED:XXd+ ...]` sentinels; tested in Phase 6 |
| Admin dashboard shell | Build from scratch | `EvalDashboard.tsx` pattern | Already has top bar, subroute tabs, URL sync via `popstate`; proven pattern |

## Common Pitfalls

### Pitfall 1: Mutating admin data through the console
**What goes wrong:** Adding POST/PUT/DELETE endpoints to `/admin` routes breaks the read-only guarantee.
**How to avoid:** Source-grep lock on each handler file — zero mutation verbs. Companion test `TestAdminBatchesRetrievalOnly` mirrors `TestAdminKPIs/retrieval_only` pattern.
**Warning signs:** `grep -cE "POST|PUT|DELETE|PATCH" admin_batches.go` returns non-zero.

### Pitfall 2: Admin token timing attack
**What goes wrong:** Using `==` for token comparison enables timing attacks.
**How to avoid:** Always use `crypto/subtle.ConstantTimeCompare` — already in `isAdminRequest()`, must not be bypassed.
**Warning signs:** Any token comparison using `==` or `!=`.

### Pitfall 3: Audit table mutation
**What goes wrong:** Admin view tempts adding DELETE/archival of old audit rows.
**How to avoid:** T-3-AUDIT INSERT-only invariant; `actor_type` column ADDITIVE only (no mutation path).
**Warning signs:** Any DELETE or UPDATE statement touching audit rows.

## Code Examples

### isAdminRequest — constant-time token comparison
```go
// internal/server/audit.go:114 — verified
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" {
        return false
    }
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" {
            got = string(v)
        }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}
```

### redact — read-time retention projection
```go
// internal/server/audit.go:148 — verified
func redact(rows []store.AuditRow, retention time.Duration, now time.Time, isAdmin bool) {
    if isAdmin {
        return // admin sees raw rows
    }
    boundary := now.Add(-retention)
    days := int(retention.Hours() / 24)
    for i := range rows {
        if rows[i].Timestamp.Before(boundary) {
            reason := fmt.Sprintf("REDACTED:%dd+ (ts=%s, boundary=%s)",
                days,
                rows[i].Timestamp.UTC().Format(time.RFC3339),
                boundary.UTC().Format(time.RFC3339))
            rows[i].Query = reason
            rows[i].VerdictJSON = reason
        }
    }
}
```

### Tabs component usage
```typescript
// web/src/components/ui/tabs.tsx — verified
import { Tabs } from '@/components/ui/tabs';

<Tabs
  value={subroute}
  onValueChange={setSubroute}
  items={[
    { value: 'batches', label: '导入批次' },
    { value: 'kpis', label: 'KPI 定义' },
    { value: 'skills', label: 'Skill 管理' },
    { value: 'schedules', label: '定时任务' },
  ]}
/>
```

### Existing store methods for admin views
```go
// ListImportBatches — for ADM-02
func (s *SQLiteStore) ListImportBatches(ctx context.Context, dataset string) ([]ImportBatchRow, error)

// LatestKPICallByFunc — for ADM-03 (already used by kpi_admin.go)
func (s *SQLiteStore) LatestKPICallByFunc(ctx context.Context) (map[string]time.Time, error)

// ListReportRuns — for ADM-05
func (s *SQLiteStore) ListReportRuns(ctx context.Context, skillID, period string, limit int) ([]ReportRunRow, error)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No admin console | `/admin` four-view tab dashboard | Phase 20 | Aggregates all data chain visibility |
| No audit actor_type | `actor_type: admin` tag on admin mutations | Phase 20 | Full audit trail of admin actions |
| No capacity monitoring | Per-table threshold bars | Phase 20 | Proactive audit growth visibility |
| 4th frozen lock pending | `data.lock.json` | Phase 20 | Final milestone closure |

**Deprecated/outdated:**
- None — Phase 20 is net-new functionality building on proven patterns.

## Assumptions Log

> All claims in this research were verified against the codebase — no assumed claims.

## Open Questions

1. **What is the threshold for audit capacity bars?**
   - What we know: Three tables need thresholds — `audit` (main), `kpi_calls`, `report_runs`. Existing eval dashboard has no capacity bar pattern to reuse.
   - What's unclear: The specific numeric thresholds (e.g., 10,000 / 50,000 / 100,000 rows). These are operational decisions.
   - Recommendation: Make thresholds configurable via env vars (`SKILLFORGE_AUDIT_CAPACITY`, `SKILLFORGE_KPI_CALLS_CAPACITY`, `SKILLFORGE_REPORT_RUNS_CAPACITY`) with sensible defaults (10,000 / 50,000 / 5,000). Planner should include a task to define these in the implementation.

2. **Does `actor_type` column need a migration or is it a new table?**
   - What we know: The audit table DDL is in `internal/store/sqlite/audit.go`. The column is ADDITIVE (not mutation).
   - What's unclear: Whether existing rows should be backfilled with `actor_type = 'user'` or left as NULL.
   - Recommendation: `ALTER TABLE audit ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user'`; existing rows get `DEFAULT 'user'` from the SQL default. No backfill needed.

3. **Skill eval result for ADM-04 — how to surface last eval result per Skill?**
   - What we know: Eval results live in `eval/frozen/*.lock.json` files on disk. `GET /api/eval/frozen` returns lock set info.
   - What's unclear: Whether to read lock files directly or use an in-memory index for the admin view.
   - Recommendation: Mirror the `FilesystemEvalReporter.FrozenSets()` pattern — read lock files at request time (not cached), since they change rarely. Add `GET /api/admin/skill-eval-status` that maps skill IDs to their latest `*.lock.json` entries.

## Environment Availability

> No external dependencies. All required tools are already in the project.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Backend admin handlers | Yes | 1.22+ | — |
| Node.js | Frontend admin dashboard | Yes | 18+ | — |
| SQLite | Store for audit + all admin data | Yes (via `modernc.org/sqlite` / `mattn/go-sqlite3`) | Latest | — |

**No missing dependencies.**

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify/assert` (backend); Vitest + React Testing Library (frontend) |
| Config file | None new — mirrors existing `06-eval-dashboard-production-hardening` pattern |
| Quick run command | `go test ./internal/server/... -run TestAdmin -count=1` |
| Full suite command | `go test ./... -count=1 && cd web && npx vitest run --reporter=dot` |

### Phase Requirements Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|------------------|--------------|
| ADM-01 | X-Admin-Token gate rejects invalid/missing token | unit | `TestAdminKPIs/retrieval_only` (existing pattern) | `kpi_admin_test.go` exists |
| ADM-02 | Import batch list returns rows with status/rowcount/raw_size | unit | `TestListImportBatches` (new) | `import_batches_test.go` — no |
| ADM-03 | KPI list returns catalog + latest_calls | unit | `TestAdminKPIs` (existing) | `kpi_admin_test.go` exists |
| ADM-04 | Skill list returns hot-reload status + eval results | unit | `TestListSkillsHandler_RegistryVersionAndFailedStatus` (existing) | `skills_test.go` exists |
| ADM-05 | Schedule list returns report_runs with status/next-fire | unit | `TestListReportRuns` (existing) | `report_runs_test.go` exists |
| ADM-06 | Admin mutations INSERT audit rows with actor_type=admin | unit | `TestAdminAuditActorType` (new) | — |

### Wave 0 Gaps
- `internal/server/admin_batches.go` + `admin_batches_test.go` — covers ADM-02
- `internal/server/admin_skills.go` + `admin_skills_test.go` — covers ADM-04
- `internal/server/admin_schedules.go` + `admin_schedules_test.go` — covers ADM-05
- `internal/server/admin_audit.go` + `admin_audit_test.go` — covers ADM-06 (actor_type INSERT)
- `web/src/components/admin/AdminDashboard.tsx` — main shell
- `web/src/components/admin/ImportBatchList.tsx` — ADM-02 view
- `web/src/components/admin/SkillManagementList.tsx` — ADM-04 view
- `web/src/components/admin/ScheduledTaskList.tsx` — ADM-05 view
- `web/src/components/admin/CapacityBar.tsx` — capacity monitoring component
- SQLite migration: `ALTER TABLE audit ADD COLUMN actor_type TEXT NOT NULL DEFAULT 'user'`
- `eval/frozen/data.lock.json` — 4th frozen lock

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | X-Admin-Token via `crypto/subtle.ConstantTimeCompare` (ASVS V6.2.1) |
| V3 Session Management | no | No sessions in admin console |
| V4 Access Control | yes | `isAdminRequest()` gate on all admin endpoints; admin token env-gated |
| V5 Input Validation | yes | All store queries use `?`-parameterized SQL (T-3-SQLI); no admin mutation inputs reach SQL |
| V6 Cryptography | yes | `crypto/subtle.ConstantTimeCompare` for token comparison |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Timing attack on admin token | Information Disclosure | `crypto/subtle.ConstantTimeCompare` — already in place |
| Audit row tampering | Tampering | T-3-AUDIT INSERT-only; `actor_type` is ADDITIVE column only |
| Retention policy bypass | Information Disclosure | `redact()` runs in HTTP layer, not store — already tested |
| Admin endpoint CSRF | Spoofing | Admin console is a separate SPA route; token is env-gated, not cookie-based |
| Path traversal on reports | Tampering | `reportPathAllowlist` regex already in `reportsRoute` |

## Sources

### Primary (HIGH confidence)
- `internal/server/audit.go` — `isAdminRequest()` gate, `redact()` function (verified by direct read)
- `internal/server/kpi_admin.go` — existing `GET /api/admin/kpis` handler pattern (verified by direct read)
- `internal/server/server.go` — route registration pattern for admin endpoints (verified by direct read)
- `web/src/components/eval/EvalDashboard.tsx` — admin shell pattern to reuse (verified by direct read)
- `web/src/components/ui/tabs.tsx` — Tabs primitive (verified by direct read)
- `web/src/App.tsx` — `route === 'admin'` branch point (verified by direct read)
- `internal/store/store.go` — `Store` interface with all list methods (verified by direct read)
- `internal/store/sqlite/report_runs.go` — `ListReportRuns` implementation (verified by direct read)

### Secondary (MEDIUM confidence)
- `eval/frozen/fanout.lock.json` — lock file structure for `data.lock.json` reference (cross-referenced with fanout.lock.json structure)
- `06-VERIFICATION.md` — Phase 6 redaction mechanism confirmed via direct read

### Tertiary (LOW confidence)
- None — all key claims verified directly.

## Metadata

**Confidence breakdown:**
- Admin auth pattern: HIGH — `isAdminRequest()` verified in `audit.go:114`
- Architecture patterns: HIGH — `EvalDashboard` shell + `Tabs` component verified in source
- Store interface: HIGH — all list methods verified in `store.go`
- Security mitigations: HIGH — `subtle.ConstantTimeCompare` + `redact()` verified in source

**Research date:** 2026-08-29
**Valid until:** 2026-09-28 (30 days — patterns are stable, no fast-moving dependencies)
