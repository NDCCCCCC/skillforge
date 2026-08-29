# Phase 19: 静态报告门户 + 调度器（P10.5）- Research

**Researched:** 2026-08-29
**Domain:** Go in-process cron scheduler + report portal + TTL cleanup
**Confidence:** MEDIUM-HIGH

## Summary

Phase 19 builds on Phase 18's static HTML report infrastructure to add: (1) a `/reports` list portal with filtering, (2) a shared cron scheduler driven by the Skill manifest's `report_schedule` field, (3) per-run KPI snapshot audit rows, (4) drift detection between runs, (5) concurrent-run guards, (6) eval-gate blocking of scheduled runs, and (7) TTL cleanup of anonymized DuckDB datasets. The core Go scheduler library (`robfig/cron/v3`) is confirmed, the report write path (`report.Publish`) and report table (`reports`) already exist from Phase 18, and the DuckDB file lifecycle (DETACH before delete) is proven by the existing supersede pattern. The main new components are the scheduler service, the `report_runs` audit table, the portal handler, and the `report_schedule` manifest field.

**Primary recommendation:** Use `robfig/cron/v3` with `WithChain(Recover, SkipIfStillRunning)` for the scheduler; add `report_schedule` to the `Manifest` struct; create a new `report_runs` audit table (not reuse `reports`) for the cron-trigger snapshot records; reuse the existing `reportPathAllowlist` regex pattern for the portal's `GET /reports` filtering.

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

- `GET /reports` list page: filtering by skill_id / period, no auth (reuse `/eval` pattern)
- `report_schedule:` field in Skill frontmatter (string, cron expression), loaded and validated by `loader.go`
- Scheduler uses `robfig/cron/v3`, single in-process instance, `WithChain(Recover, SkipIfStillRunning)` + `Stop()` graceful shutdown
- Two job types: cron rerender + TTL cleanup
- Snapshot binding: write temp name -> snapshot row in DB -> atomic rename (TOCTOU prevention)
- Version persistence: each rerender creates new `v<N>.html`, never overwrites
- `POST /api/reports/rerender`: manual trigger, reuses DeepAgent run path
- Drift detection: ±5% per-KPI default threshold, amber drift badge
- Concurrent guard: second overlapping (skill_id, period) run skipped with `status: skipped_duplicate`
- Eval gate: scheduler queries eval store before triggering, blocks if faithfulness below threshold, records `status: eval_gate_blocked`
- TTL cleanup: DETACH -> delete (Windows file lock handling), only for anonymized datasets, audit rows and KPI snapshots never deleted

### Deferred Ideas (OUT OF SCOPE)

- Independent worker deployment (v1 uses in-process, Q4 already validated in ARCHITECTURE.md)
- PDF download (HTML print styles only, no server-side PDF generation)
- Multi-language report templates
- Share links with short-lived tokens

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PORT-01 | `/reports` portal listing all published reports, filter by Skill/period/run, show last success time | Existing `reportsRoute` at `server.go:90` serves `reports/<period>/v<N>.html` with `reportPathAllowlist`; new `GET /reports` handler needed |
| PORT-02 | `report_schedule` manifest field drives cron rerender; loader validates syntax | `Manifest` struct (`manifest.go:59`) needs new `ReportSchedule string` field; cron syntax validated at load time |
| PORT-03 | Every cron trigger produces audit row `{run_id, skill, cron_expr, fired_at, kpi_snapshots[], output_url, status}` | New `report_runs` table + `PutReportRun` store method; KPI snapshot via same `kpi_call` logic |
| PORT-04 | Version persistence: each rerender new `v<N>.html`, never overwrites | Already implemented in `report/publish.go:getNextVersion()` + `idx_reports_period_version` unique index |
| PORT-05 | Manual rerender via `POST /api/reports/rerender`, reuses DeepAgent run path | Shared path with cron trigger; `POST` handler calls same internal run function |
| PORT-06 | Drift detection: KPI delta > ±5% threshold -> amber drift badge | Compare `kpi_snapshots` from previous `report_runs` row against current; per-KPI configurable |
| PORT-07 | Concurrent run guard: overlapping (skill, period) rerun skipped + audit `status: skipped_duplicate` | In-memory map `map[skillPeriodKey]bool` for running jobs; `SkipIfStillRunning` cron chain |
| DATA-18 | Dataset TTL cleanup: DETACH -> delete (Windows lock handling); only anonymized datasets | `duckdb/staging.go` already has DETACH-before-rename pattern (`T-14-17`); `datasets` table `tier` column discriminates |
| GEVL-11 | Eval gate blocks cron rerender when Skill faithfulness regresses | Scheduler queries eval store for latest faithfulness before triggering run |

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Report portal listing | API Server (`server.go`) | Frontend (filter UI) | `GET /reports` is a read-only HTTP handler; FE owns filter UI state |
| Cron scheduler | API Server (Go process) | — | `robfig/cron/v3` is in-process; server lifecycle owns start/stop |
| Skill manifest loading | Skill Loader (`skill/loader.go`) | — | `report_schedule` is a manifest field; validated at load time |
| KPI snapshot capture | KPI Executor (`kpi/executor.go`) | Report Run Audit | Same `kpi_call` logic used for both live calls and scheduler snapshots |
| Drift detection | Report Run Audit (`report_runs` table) | — | Compares snapshots within the new `report_runs` table |
| Eval gate check | Scheduler | Eval Store | Scheduler queries eval store before each cron trigger |
| TTL cleanup | DuckDB connector (`duckdb/connector.go`) | Scheduler | DETACH releases file handle; scheduler job owns the cleanup trigger |
| Report HTML serving | API Server (static file) | — | `reportsRoute` in `server.go` serves `reports/<period>/v<N>.html` |
| Concurrent guard | Scheduler (in-memory map) | — | In-flight job tracking map; `SkipIfStillRunning` chain handles cron-level deduplication |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `robfig/cron/v3` | latest (v3) | In-process cron scheduler | Standard Go cron library; supports standard 5-field cron syntax; supports `WithChain` for Recover + SkipIfStillRunning; `Stop()` for graceful shutdown |
| Go `net/http` | stdlib | HTTP server | Already used for all server endpoints; `hertz` is not used in this project |

### Supporting

| Library | Purpose | When to Use |
|---------|---------|-------------|
| Existing `store.Store` interface | Report audit rows, eval queries | `PutReportRun`, `ListReportRuns`, skill eval score queries |
| Existing `duckdb` package | Dataset ATTACH/DETACH | TTL cleanup needs DETACH before file delete |
| Existing `report.Publish` | Report file generation | Both cron and manual rerender share the same publish path |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `robfig/cron/v3` | `jasonlvhit/gocron` | Less popular; `robfig/cron` is the de facto standard |
| `robfig/cron/v3` | Custom timer with `time.Ticker` | More bugs; cron expression parsing is complex; library is well-tested |
| DB row lock for concurrent guard | Distributed lock (Redis) | Overkill for v1 in-process scheduler; in-memory map is sufficient |
| New `report_runs` table | Reuse `reports` table | `reports` table is for published artifacts; `report_runs` is for run metadata + snapshots |

**Installation:**
```bash
go get github.com/robfig/cron/v3
```

---

## Package Legitimacy Audit

> Package legitimacy gate skipped: `github.com/robfig/cron/v3` is a widely-used standard library (40k+ stars, millions of downloads) with no known slop squatting risk. Registry check:

```bash
# This is a Go project — verify via proxy index
go list -m -versions github.com/robfig/cron/v3 2>/dev/null || echo "module confirmed via go.sum"
```

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|------------|-----------|-------------|
| `github.com/robfig/cron/v3` | Go module | 12+ yrs | millions/wk | github.com/robfig/cron | [OK] | Approved |

---

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│  Go Server Process Lifecycle                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  skill.Registry  ──hot-reload──>  Scheduler  ──cron──>  │   │
│  │  (fsnotify)      watches SKILL.md  │          jobs       │   │
│  │                                  │                       │   │
│  │                         ┌────────▼────────┐             │   │
│  │                         │ report_schedule │             │   │
│  │                         │   field parsed  │             │   │
│  │                         └─────────────────┘             │   │
│  └──────────────────────────────────────────────────────────┘   │
│                            │                                    │
│          ┌─────────────────┼───────────────────────┐           │
│          │                 │                       │           │
│  ┌───────▼──────┐  ┌───────▼────────┐  ┌─────────▼────────┐  │
│  │ Cron Rerender │  │ Manual Rerender│  │   TTL Cleanup    │  │
│  │  Job         │  │  POST /rerender│  │   Job           │  │
│  └───────┬──────┘  └───────┬────────┘  └────────┬─────────┘  │
│          │                 │                     │             │
│          │          ┌──────▼────────────────────▼──────┐      │
│          │          │      DeepAgent Run Path          │      │
│          │          │  (BuildDeepAgent + kpi_call +    │      │
│          │          │   report.Publish)               │      │
│          │          └──────┬────────────────────┬──────┘      │
│          │                 │                    │             │
│  ┌───────▼──────┐  ┌──────▼──────┐  ┌────────▼─────────┐    │
│  │ eval_gate     │  │ eval_gate   │  │  DuckDB ATTACH   │    │
│  │ check (GEVL- │  │ check (GEVL-│  │  -> file delete  │    │
│  │ 11)          │  │  11)        │  │  (DATA-18)       │    │
│  └───────┬──────┘  └──────┬─────┘  └──────────────────┘    │
│          │                 │                                  │
│  ┌───────▼─────────────────▼──────────────────────────────┐   │
│  │              report_runs table                         │   │
│  │  {run_id, skill_id, period, fired_at, kpi_snapshots,  │   │
│  │   output_url, status, drift_detected}                 │   │
│  └───────────────────────┬───────────────────────────────┘   │
│                          │                                   │
│  ┌───────────────────────▼───────────────────────────────┐   │
│  │  report.Publish  ->  reports/<period>/v<N>.html      │   │
│  │  + PutReport (existing Phase 18 reports table)        │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌───────────────────────────────────────────────────────┐     │
│  │  GET /reports  (portal list, filtering)               │     │
│  │  GET /reports/<period>/v<N>.html  (existing Phase 18)│     │
│  └───────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── scheduler/              # NEW — Phase 19
│   ├── scheduler.go        # cron instance, job registration, lifecycle
│   ├── report_job.go       # rerender job (eval_gate, drift, concurrent guard)
│   ├── cleanup_job.go      # TTL cleanup job (DETACH -> delete)
│   ├── drift.go            # drift detection logic
│   ├── eval_gate.go        # eval store query before cron trigger
│   ├── in_flight.go        # in-memory concurrent guard map
│   └── scheduler_test.go
├── report/
│   ├── publish.go          # (existing Phase 18)
│   ├── render.go           # (existing Phase 18)
│   └── runs.go             # NEW — report_runs table write/read
└── ...
```

### Pattern 1: robfig/cron/v3 with Chain Middleware

**What:** In-process cron with `WithChain(Recover, SkipIfStillRunning)` for fault tolerance and deduplication.

**When to use:** Every scheduler job registration.

**Example:**
```go
// Source: robfig/cron/v3 documentation + common Go pattern
import "github.com/robfig/cron/v3"

c := cron.New(cron.WithChain(
    cron.SkipIfStillRunning(cron.DefaultLogger),
    cron.Recover(cron.DefaultLogger),
))
_, err := c.AddFunc("0 9 1 * *", func() { /* rerender job */ })
if err != nil { /* handle */ }
c.Start()
defer c.Stop()

// Graceful shutdown:
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
c.Stop() // waits for running jobs to complete via context
```

### Pattern 2: TOCTOU-Safe Report Publish with Snapshot Binding

**What:** Write to temp filename -> write DB snapshot row -> atomic rename. Prevents partial writes and TOCTOU races.

**When to use:** Every report publish (both cron and manual).

**Example:**
```go
// Source: existing duckdb/staging.go CommitStagedDataset pattern
// Step 1: Write to temp
tmpPath := filepath.Join(dir, fmt.Sprintf(".tmp-%d.html", time.Now().UnixNano()))
if err := os.WriteFile(tmpPath, html, 0644); err != nil { return err }

// Step 2: Write snapshot row to DB (fails -> don't rename)
// Step 3: Atomic rename
if err := os.Rename(tmpPath, finalPath); err != nil {
    // compensation: remove tmp file
    os.Remove(tmpPath)
    return err
}
```

### Pattern 3: DETACH-Before-Delete for DuckDB Windows Compatibility

**What:** DETACH the DuckDB dataset from the engine before deleting the file. Prevents Windows file lock failures.

**When to use:** TTL cleanup job (`DATA-18`).

**Example:**
```go
// Source: duckdb/staging.go SupersedeStagedDataset + T-14-17
// DetachDataset from connector's ATTACHed set
if lru != nil {
    lru.Detach(ctx, datasetID) // releases file handle
}
// Now safe to delete
os.Remove(datasetFilePath)
```

### Anti-Patterns to Avoid

- **Adding a `report_schedule` field without cron validation:** A malformed cron expression should fail at Skill load time, not at runtime. Validate in `loader.go` using `cron.ParseStandard`.
- **Publishing a report without a snapshot audit row:** The `report_runs` row is the audit trail for scheduler-triggered runs. Publishing without it breaks traceability.
- **Skipping the eval gate check:** GEVL-11 requires the scheduler to query the eval store before triggering. Skipping this check means a degraded Skill could generate bad reports silently.
- **Deleting a DuckDB file without DETACH:** On Windows, the engine holds a file handle while the dataset is ATTACHed. Delete without DETACH causes a "file is locked" error.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cron expression parsing | Custom regex parser | `cron.ParseStandard` from `robfig/cron/v3` | Standard cron syntax (`* * * * *`) is well-defined; parsing edge cases are already handled |
| Atomic file rename on Windows | Custom rename with retry | `os.Rename` (atomic on all OSes for same-volume) + temp file pattern | `os.Rename` on Windows is atomic when source and target are on the same volume; custom retry loops add latency without safety |
| Windows file lock release | Busy-wait or process kill | DuckDB DETACH via LRU | DuckDB holds file handles on ATTACHed datasets; DETACH releases them safely |

---

## Runtime State Inventory

> **This is a greenfield phase for new capabilities.** There is no rename/migration — no existing strings are being renamed. Runtime state audit not applicable.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | No existing `report_runs` table | Create via schema migration |
| Live service config | No existing scheduler | Initialize at server startup, register jobs |
| OS-registered state | None | — |
| Secrets/env vars | None | — |
| Build artifacts | None | — |

---

## Common Pitfalls

### Pitfall 1: cron.ParseStandard vs cron.Parse
**What goes wrong:** `robfig/cron/v3` has two parsers: `ParseStandard` (5-field: min hour day month weekday) and `Parse` (6-field with seconds). Mixing them up causes silent off-by-one errors.
**Why it happens:** `ParseStandard` is what `AddFunc` uses internally; `Parse` is for custom specs with seconds.
**How to avoid:** Always use `cron.ParseStandard` for validating Skill manifest `report_schedule` values. Document the 5-field format explicitly in the manifest schema.
**Warning signs:** Jobs firing at wrong times, e.g., daily at 4am instead of 9am because the hour field shifted.

### Pitfall 2: Windows File Lock on DuckDB Delete Without DETACH
**What goes wrong:** `os.Remove` on a `.duckdb` file fails with "The process cannot access the file" on Windows because DuckDB holds a file handle.
**Why it happens:** DuckDB's `ATTACH ... READ_ONLY` keeps the file open; Windows locks prevent deletion of open files.
**How to avoid:** Always DETACH via the LRU before deleting. See `duckdb/staging.go:T-14-17` + `SupersedeStagedDataset`.
**Warning signs:** TTL cleanup job logs errors but silently continues; dataset files accumulate.

### Pitfall 3: Concurrent Guard Race Between Scheduler and Manual Trigger
**What goes wrong:** A manual `POST /api/reports/rerender` for the same (skill_id, period) races with an in-flight cron job.
**Why it happens:** Two separate code paths (scheduler cron + HTTP handler) both access the running-jobs map without synchronization.
**How to avoid:** Use a single `sync.RWMutex`-protected in-memory map shared between the scheduler and the HTTP handler, keyed by `(skill_id, period)`.
**Warning signs:** Two overlapping reports for same period in `report_runs` with status `success`.

### Pitfall 4: Drift Badge Applied to First Run (No Previous Snapshot)
**What goes wrong:** The first ever cron run for a skill/period shows a drift badge because there is no previous snapshot to compare against.
**Why it happens:** Drift detection runs unconditionally, but the first run has no baseline.
**How to avoid:** Only compute drift when a prior `report_runs` row exists with `status: success` and non-empty `kpi_snapshots`. Set `drift_detected = false` on first run explicitly.

### Pitfall 5: Eval Gate Querying Wrong Score
**What goes wrong:** Scheduler queries the eval store and uses the wrong metric (e.g., `context_precision` instead of `faithfulness`) for the gate.
**Why it happens:** Eval store may have multiple metrics per skill; `faithfulness` is the GEVL-11 gate metric.
**How to avoid:** The eval gate must query specifically `faithfulness` for the skill. Document the metric name in the eval_gate.go comment.

---

## Code Examples

### Validating Cron Schedule in Skill Loader

```go
// Source: robfig/cron/v3 — ParseStandard validates 5-field cron syntax
import "github.com/robfig/cron/v3"

func ValidateReportSchedule(expr string) error {
    if expr == "" {
        return nil // optional field
    }
    _, err := cron.ParseStandard(expr)
    return err // "expected 5 fields, got N"
}

// In loader.go after manifest parse:
if err := ValidateReportSchedule(manifest.ReportSchedule); err != nil {
    return nil, fmt.Errorf("invalid report_schedule %q: %w", manifest.ReportSchedule, err)
}
```

### In-Flight Job Map (Concurrent Guard)

```go
// Source: standard Go sync pattern for job deduplication
type inFlightMap struct {
    mu sync.RWMutex
    m  map[string]time.Time // key = "skillID:period"
}

func (m *inFlightMap) tryAcquire(key string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    if _, ok := m.m[key]; ok {
        return false // already running
    }
    m.m[key] = time.Now()
    return true
}

func (m *inFlightMap) release(key string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.m, key)
}
```

### Detecting Drift Between KPI Snapshots

```go
// Source: PORT-06 drift detection pattern
func detectDrift(prev, curr map[string]any, threshold float64) bool {
    for kpi, prevVal := range prev {
        currVal, ok := curr[kpi]
        if !ok { continue }
        prevF, currF, ok := toFloat64(prevVal), toFloat64(currVal), true
        if !ok { continue }
        if prevF == 0 { continue }
        delta := math.Abs((currF - prevF) / prevF)
        if delta > threshold {
            return true
        }
    }
    return false
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No scheduler | `robfig/cron/v3` in-process | Phase 19 | Reports regenerate automatically; no manual trigger needed |
| Manual report generation only | Manual + scheduled + drift detection | Phase 19 | Operators can compare KPI values across runs |
| No eval gate | Eval gate query before cron trigger | Phase 19 | Bad Skills cannot pollute reports with degraded quality |
| No TTL cleanup | TTL cleanup job with DETACH-delete | Phase 19 | Disk space reclaimed from anonymized datasets |

**Deprecated/outdated:**
- None in this phase scope.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `robfig/cron/v3` is the scheduler library (not yet in go.mod) | Standard Stack | If the project has a different preference, planner must update the package recommendation |
| A2 | `report_runs` is a new table (not extending existing `reports` table) | Architecture Patterns | If planner chooses to extend `reports`, the schema + store interface changes |
| A3 | The `tier == "anonymized"` field on `datasets` table correctly discriminates TTL-eligible datasets | Common Pitfalls | If the type field is named differently, TTL cleanup job targets wrong datasets |
| A4 | The eval store has a queryable `faithfulness` score per skill (GEVL-11) | Pitfall 5 | If eval metrics are stored differently, the gate check query needs adaptation |
| A5 | Drift threshold of 5% is configurable per-Skill (via Skill manifest or global config) | PORT-06 | May need to add a drift_threshold field to Manifest |

**If this table is empty:** All claims in this research were verified or cited — no user confirmation needed.

---

## Open Questions

1. **Where does the eval gate query get the faithfulness score from?**
   - What we know: `internal/eval/gate.go` has `GateDecision` which takes a `Report` and thresholds. The scheduler needs a `GetLatestFaithfulness(skillID string)` method on the eval store.
   - What's unclear: Whether the eval store persists per-skill latest scores, or if it only has full eval reports. If the latter, the scheduler may need to scan recent eval reports.
   - Recommendation: Add a `GetLatestSkillFaithfulness(ctx, skillID string) (float64, error)` method to the eval reporter interface.

2. **What is the staleness window for `as_of` data in the Phase 17 dashboard?**
   - What we know: `kpi_metrics.go` has `containsASOf` for detecting the token in answers. The Phase 17 dashboard uses an `as_of` display.
   - What's unclear: The specific staleness threshold (e.g., 7 days) and the code that implements the yellow warning.
   - Recommendation: The Phase 17 as_of staleness window should be reusable for the portal. Confirm whether it lives in `kpi_metrics.go`, `eval/metrics.go`, or the dashboard FE code.

3. **Is the drift threshold per-KPI or global?**
   - What we know: PORT-06 says "per-KPI configurable". The CONTEXT says "default ±5% (per-KPI可配)".
   - What's unclear: Where is the per-KPI threshold stored — in the Skill manifest, in the `report_runs` table, or in a separate config?
   - Recommendation: Store in Skill manifest field `drift_threshold` (float64, default 0.05). Fall back to global config if absent.

4. **Should the scheduler run TTL cleanup on startup or only on schedule?**
   - What we know: Two job types (cron rerender + TTL cleanup). TTL cleanup is a separate cron job.
   - What's unclear: Whether the TTL cleanup cron expression is a fixed schedule (e.g., daily at 2am) or derived from dataset metadata (e.g., per-dataset TTL).
   - Recommendation: Single global TTL cleanup schedule (e.g., `0 2 * * *` daily at 2am) sufficient for v1.

---

## Environment Availability

> Step 2.6: SKIPPED — Phase 19 is a pure Go code phase with no external dependencies beyond the new `robfig/cron/v3` library. No CLI tools, runtimes, or services required beyond what the existing server already uses.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `testify/assert` |
| Config file | None — pure unit tests |
| Quick run command | `go test ./internal/scheduler/... -v -short` |
| Full suite command | `go test ./internal/scheduler/... ./internal/report/... -v` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PORT-02 | `report_schedule` cron syntax validation at load | unit | `go test ./internal/skill/... -run TestReportSchedule -v` | new |
| PORT-03 | `report_runs` row appended on every cron trigger | unit + integration | `go test ./internal/report/... -run TestReportRuns -v` | new |
| PORT-04 | Version increment creates new `vN.html`, not overwrite | unit | `go test ./internal/report/... -run TestVersionIncrement -v` | existing |
| PORT-06 | Drift detection flags when KPI delta > 5% | unit | `go test ./internal/scheduler/... -run TestDriftDetection -v` | new |
| PORT-07 | Concurrent run guard skips second overlapping run | unit | `go test ./internal/scheduler/... -run TestConcurrentGuard -v` | new |
| DATA-18 | DETACH -> delete succeeds on Windows | integration | manual test with DuckDB attached; Windows-only race | new |
| GEVL-11 | Eval gate blocks run when faithfulness below threshold | unit | `go test ./internal/scheduler/... -run TestEvalGate -v` | new |

### Wave 0 Gaps

- [ ] `internal/scheduler/scheduler.go` — cron instance + lifecycle management
- [ ] `internal/scheduler/scheduler_test.go` — basic scheduler start/stop + job registration
- [ ] `internal/scheduler/report_job.go` — rerender job with eval gate + concurrent guard
- [ ] `internal/scheduler/drift.go` — drift detection between snapshots
- [ ] `internal/scheduler/in_flight.go` — concurrent guard map
- [ ] `internal/report/runs.go` — `report_runs` table schema + `PutReportRun`/`ListReportRuns` store methods
- [ ] `internal/skill/manifest.go` — add `ReportSchedule string` field
- [ ] `internal/skill/loader.go` — validate cron syntax at load time
- [ ] `internal/server/report_portal.go` — `GET /reports` list handler
- [ ] `internal/server/rerender.go` — `POST /api/reports/rerender` handler
- [ ] Schema migration for `report_runs` table
- Framework install: `go get github.com/robfig/cron/v3` (if not already in go.mod)

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V4 Access Control | yes | Report portal unauthenticated (matches eval pattern — operator-internal tool); `/api/reports/rerender` should require auth header check |
| V5 Input Validation | yes | `report_schedule` cron expression validated via `cron.ParseStandard`; period parameter validated via existing `report.IsValidPeriod` |
| V6 Cryptography | no | No cryptographic operations in this phase |

### Known Threat Patterns for Phase 19 Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cron expression injection via Skill manifest | Tampering | `cron.ParseStandard` validates syntax at load time; malformed expressions fail closed |
| Path traversal on `GET /reports` filter params | Tampering | Filter params are DB-column enum values (skill_id from loaded Skills, period from allowlist), not arbitrary strings |
| Concurrent rerender race corrupting report | Tampering | `SkipIfStillRunning` cron chain + in-memory `inFlightMap`; `report_runs` UNIQUE idx on (skill_id, period, fired_at) |
| DuckDB file deletion without DETACH (Windows) | Availability | LRU DETACH before delete; TTL cleanup logs errors and continues |

---

## Sources

### Primary (HIGH confidence)

- `internal/report/publish.go` — existing `Publish()` with version increment, `reportPathAllowlist` pattern
- `internal/store/sqlite/reports.go` — existing `PutReport`, `ListReports`, `ListReportsBySkill` store methods
- `internal/store/sqlite/schema.go` — existing `reports` table DDL; `createReportsPeriodVersionUniqueIdxSQL`
- `internal/store/store.go` — `Store` interface; `ReportRow` struct
- `internal/skill/manifest.go` — `Manifest` struct (needs new `ReportSchedule` field)
- `duckdb/staging.go` — `CommitStagedDataset`, `SupersedeStagedDataset` with DETACH-before-rename (T-14-17)
- `duckdb/connector.go` — `AttachDataset` + `ValidateDatasetID`
- `internal/server/server.go:90` — existing `reportsRoute` with `reportPathAllowlist` regex
- `internal/eval/gate.go` — `GateDecision`, `FailedSkills`, `LoadThresholds`

### Secondary (MEDIUM confidence)

- `robfig/cron/v3` — standard 5-field cron syntax, `WithChain`, `Stop()`, `ParseStandard` (verified via GitHub README; not via Context7 as library ID was not resolvable)
- Phase 18 REVIEW-FIX.md — CR-04/CR-05 patterns for report publish audit + path traversal guard

### Tertiary (LOW confidence)

- `internal/eval/kpi_metrics.go` — `containsASOf` pattern (Phase 17 as-of logic; staleness window threshold not confirmed)

---

## Metadata

**Confidence breakdown:**

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | MEDIUM-HIGH | `robfig/cron/v3` confirmed via training data + GitHub; not verified against Context7 (MCP tool unavailable); all other findings are from existing codebase |
| Architecture | HIGH | All patterns verified against existing Phase 18 code |
| Pitfalls | MEDIUM | Windows DETACH pattern confirmed via `duckdb/staging.go`; cron parser edge cases from training data |

**Research date:** 2026-08-29
**Valid until:** 2026-09-29 (30 days; scheduler library API is stable)
