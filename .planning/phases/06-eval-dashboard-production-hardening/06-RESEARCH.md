# Phase 6: Eval Dashboard & Production Hardening - Research

**Researched:** 2026-08-13
**Domain:** Go-native trust-evaluator hardening (CI gate + dashboard + audit privacy + reproducibility) on a Phase-3-shipped `internal/eval` + `internal/store` substrate
**Confidence:** HIGH (researches a LIVE, already-shipped codebase + well-known patterns: Hertz handlers, GitHub Actions, SQLite read projections, React 19 data panels)

## Summary

Phase 6 turns the Phase-3-shipped Go-native trust evaluator (`internal/eval` + `cmd/eval`, which today emits a batch `eval/report.json` + `eval/calibration.json`) into four production surfaces: (1) a **CI promotion gate** that blocks a PR dropping any Skill's faithfulness below its per-Skill threshold (GEVL-07), (2) a **per-Skill metrics dashboard** with trend + frozen-baseline overlay (GEVL-06), (3) a **queryable audit UI** with row-level retention redaction + binary admin-token access split (SC-3 + SC-5), and (4) **strengthened reproducibility** via frozen eval sets + `.lock.json` pin sidecars (SC-6). A fifth deliverable, SC-4 (50 sessions × 5 turns concurrency isolation), scales an existing Phase-3 test — it adds no new enforcement, only test volume.

The single most important research finding is that **almost everything the SC list asks for already has a Phase-3-shipped hook**: `store.QueryAudit(AuditFilter)` is already 100% `?`-parameterized (T-3-SQLI regression-tested) — `/api/audit` is a 1:1 HTTP mapping; `eval.Report` already carries `Overall` + `PerCategory` means — the dashboard reads these; `TestRunnerReproducibility` already gates SC-6 at `1e-9` — Phase 6 relaxes it to `1e-6` and adds a live-jitter layer. The genuine new build is: 3 backend handler files (`audit.go`, `eval.go` + a retention projection) + 1 `cmd/eval` flag extension (`--archive` / `--frozen` / `--judge-models`) + 1 greenfield `.github/workflows/eval-gate.yml` + 3 frontend route surfaces (Skills table, Trend panel, Audit table) + 1 concurrency test scale-up. No new Go dependencies. Two optional frontend dependencies (recharts, react-router) — both flagged for planner decision.

**Primary recommendation:** Extend `internal/server` with `audit.go` + `eval.go` handler files (lean per CONTEXT D6 "Claude's Discretion"); place the retention/redaction as a **read-time server-side projection** (never a store write — preserves T-3-AUDIT INSERT-only); resolve "citation precision" = `context_precision` and "refusal accuracy" = `PerCategory["refusal-required"]["context_recall"]` (NO new metric); and **resolve the two CONTEXT discrepancies** before planning — (a) eval-gate.json co-location path (`skills/<name>/`, NOT `.claude/skills/<name>/`), and (b) the `prompt_hash` semantics gap (it hashes only `skillName+modelVersion` today, so the frozen lock's pin is weaker than SC-6 implies).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions (D6-01 .. D6-16 — research THESE, no alternatives)

**Dashboard UI (D6-01..04):**
- **D6-01:** `/eval` is an independent full-screen route (admin mode). The three-panel D-04/D12 transport-swap invariant is untouched — `/eval` is a separate route with its own transport (no SSE). Entering `/eval` hides the chat three-panel; returning to `/` restores it. `LeftSidebar` gains an "Eval Dashboard" nav entry.
- **D6-02:** Trend history source = `eval/reports/*.json` + `eval/reports/INDEX.json` manifest (file-based, git-friendly). `cmd/eval --archive` appends `eval/reports/<iso-ts>.json` (full report) AND updates `INDEX.json` (lightweight array of `{ts, skill, judge_model, faithfulness, context_precision, answer_relevance, context_recall, refusal_accuracy, frozen_version}`). Rejected: new sqlite `EvalRun` table; GitHub API pull of Actions history.
- **D6-03:** Refresh model = manual Refresh button + 30s low-frequency poll. Rejected: SSE push of eval-completion; manual-only.
- **D6-04:** Dashboard = Skills table + Trend panel; Audit table lives on `/eval/audit` sub-route.

**Audit query + redaction (D6-05..08):**
- **D6-05:** `/api/audit` = parameterized `store.QueryAudit` + LIMIT/OFFSET pagination. Query params `user`, `skill`, `since`, `until`, `refusal_reason`, `limit`, `offset` map 1:1 to `store.AuditFilter`. Rejected: cursor + server-side sort; GraphQL.
- **D6-06:** Retention + redaction = `RetentionDays` parameter + `X-Admin-Token` access split. Default 90d via `SKILLFORGE_RETENTION_DAYS`. Rows past `row.Timestamp + retention` have `Query`/`Answer`/`VerdictJSON` replaced with `"[REDACTED:<reason>]"`. Valid `X-Admin-Token` (= `SKILLFORGE_ADMIN_TOKEN` env, default unset) → raw rows; no token → retention-redacted. Rejected: background TTL delete; per-field encryption-at-rest; per-row `access_policy` column (violates T-3-AUDIT INSERT-only).
- **D6-07:** Frontend redaction UX = visible `[REDACTED]` badge + hover detail (timestamp + retention boundary). Raw content never expandable from UI without admin token. Rejected: plain-text `[REDACTED]`; hiding expired rows.
- **D6-08:** Access policy = admin vs non-admin binary (no ACL in v1). v2 SCALE-* owns real RBAC.

**CI gate (D6-09..12):**
- **D6-09:** GitHub Actions `eval-gate.yml` triggers on `pull_request` ONLY (not `push` to main). Path-filtered to `skills/*/SKILL.md` OR `eval/golden/*` OR `eval/frozen/*` OR `internal/eval/**` OR `internal/agent/**`. Rejected: PR + push-main both run; scheduled weekly.
- **D6-10:** Per-Skill threshold via `<skill-dir>/eval-gate.json`, default 0.9. Skills without the file default to `0.9`. Gate fails if ANY Skill's `overall.faithfulness < threshold`. Phase 6 ships ONE Skill → one threshold = 0.9. Rejected: uniform 0.9; dynamic threshold from `calibration.json` (semantic mismatch — that's the retrieval floor, not faithfulness).
- **D6-11:** PR feedback = `gh pr comment` markdown table + uploaded `report.json`/`calibration.json` artifacts. Rejected: status-check red-X only (SC-2 requires a comment naming the Skill); Slack/Discord webhook.
- **D6-12:** Cost control = all PRs run; manual `[skip-eval]` override deferred to Phase 7. Rejected: `[skip-eval]` from day one; main-branch-only.

**Reproducibility + cross-eval (D6-13..16):**
- **D6-13:** Frozen eval sets = `eval/frozen/<skill>.jsonl` + sidecar `.lock.json` pinning `model_ver`, `prompt_hash`, `seed`, `eval_set_version`. `cmd/eval --frozen=<skill>` reads both, asserts the pin, runs metrics. Rejected: `go:embed`; Git LFS.
- **D6-14:** Cross-eval = `cmd/eval --judge-models <a,b>` dual-report diff → `eval/reports/<judge>/<ts>.json` each. Dashboard "Cross-eval" tab flags any Skill with `>5%` faithfulness delta (the Pitfall 8 self-eval-bias signal). Rejected: dashboard-triggered async run; docs-only manual switching.
- **D6-15:** Reproducibility tolerance = layered — StubJudge `1e-6` (strict, strengthened from `1e-9`) + live judge `1e-3` (jitter-tolerant). Both gate Phase 6 exit. Rejected: StubJudge-only; uniform `1e-9`.
- **D6-16:** Trend panel overlays frozen-set baseline as gray dashed reference + "diff vs baseline" action. Rejected: trend shows live history only; trend decoupled from baseline.

### Claude's Discretion (research options, recommend — planner decides)

1. **"Citation precision" + "refusal accuracy" semantics** → RESOLVED below (no new metric): citation precision = `context_precision`; refusal accuracy = derived from `refusal-required` category's `context_recall`.
2. **SC-4 50×5 test positioning** → RESOLVED below: new `TestSessionIsolation50x5` alongside the existing 2-session test (same file, scales the invariant).
3. **`/api/eval/*` + `/api/audit` endpoint design + handler placement** → RESOLVED below: extend `internal/server` (NOT a new `internal/evalapi` package); 3 new handler files; introduce an `EvalReporter` seam on `Deps` for testability.
4. **`cmd/eval` flag taxonomy + `--archive`/`--frozen`/`--judge-models` interactions** → RESOLVED below (flag matrix + INDEX.json schema).
5. **Retention/redaction wrapper placement** → RESOLVED below: server-layer read-time projection (NOT store layer); preserves T-3-AUDIT INSERT-only.
6. **Admin token absence behavior** → RESOLVED below: unset token → all requests `isAdmin=false` → return retention-redacted rows (demo works OOB); raw access only when env set.
7. **SC-6 frozen-reproducibility mechanics + prompt_hash** → RESOLVED below, with the **prompt_hash strengthening decision** flagged as a planner gate (see Open Question Q1).
8. **Dashboard trend chart approach** → RESOLVED below: recharts primary (React 19 compat verified); hand-rolled SVG alternative for the anti-dep stance.

### Deferred Ideas (OUT OF SCOPE — ignore)
- Real multi-role RBAC / SSO / per-row `access_policy` (v2 SCALE-02).
- DATA-* financial data layer.
- Per-Skill golden-set seeding for wizard-generated Skills (later phase).
- Real-user-sampled golden-set augmentation (D3-06 deferred).
- `/api/skills/reload` rescan route (D5-IA-05 — not added).
- PostgreSQL production store backend (D3-09/D18 leaves the slot).
- Live SSE push of eval-completion (D6-03).
- `[skip-eval]` PR label / `workflow_dispatch` override (Phase 7).
- Open-source packaging DIST-* (Phase 7).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description (from REQUIREMENTS.md) | Research Support |
|----|------------------------------------|------------------|
| **GEVL-06** | Eval dashboard surfaces per-Skill metrics (faithfulness / citation precision / refusal accuracy) with trend over time | `eval.Report.Overall` + `Report.PerCategory` already carry the 4 metrics (Phase 3). "Citation precision" = `context_precision`; "refusal accuracy" = `PerCategory["refusal-required"]["context_recall"]` (derived — see §Architecture Patterns). Dashboard reads `INDEX.json` (D6-02) for trend; `/api/eval/reports` serves it. |
| **GEVL-07** | CI gating on faithfulness < 0.9 fails Skill promotion | `.github/workflows/eval-gate.yml` (greenfield — confirmed no `.github/` exists) runs `cmd/eval`, parses `report.json`, fails if any Skill's `overall.faithfulness < threshold` (per-Skill `eval-gate.json`, default 0.9), posts `gh pr comment`. |

**SC traceability (SC-3/4/5/6 carry no dedicated v1 requirement — ROADMAP SC list governs phase scope per CONTEXT):**

| SC | Requirement | Research Support |
|----|-------------|------------------|
| SC-1 | Per-Skill metrics dashboard with trend | GEVL-06 — `/eval` route + Skills table + Trend panel |
| SC-2 | CI gate blocks promotion | GEVL-07 — `eval-gate.yml` + per-Skill threshold |
| SC-3 | Queryable audit UI + row-level redaction | (no v1 req) — `/api/audit` maps `store.QueryAudit`; `/eval/audit` table |
| SC-4 | 50×5 session isolation | (no v1 req) — scales `chat_session_test.go::TestSessionIsolationConcurrent` |
| SC-5 | Retention/redaction policy enforced | (no v1 req) — server-layer read-time projection (D6-06) |
| SC-6 | Eval runs reproducible | (no v1 req) — `eval/frozen/<skill>.jsonl` + `.lock.json` + 2-layer tolerance (D6-15) |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Eval trend history storage | Filesystem (`eval/reports/INDEX.json`) | — | D6-02 locked: file-based, git-diffable, zero extra dependency; eval metadata is NOT audit data (the INSERT-only T-3-AUDIT invariant does not apply), so a sqlite `EvalRun` table was explicitly rejected. |
| Per-Skill metric computation | `internal/eval` (Go lib, shipped Phase 3) | — | The 4 metrics + `Judge` + `LoadGolden` are reused as-is; Phase 6 EXTENDS, never rewrites. |
| Audit query + filter | `internal/store.Store.QueryAudit` (shipped Phase 3) | `internal/server` HTTP handler | The SQL filter is already 100% `?`-parameterized; `/api/audit` is a thin HTTP mapping. Tier = API/Backend. |
| Retention redaction | `internal/server` (read-time projection) | — | MUST be server-layer (read-time), NOT store-layer — the store preserves T-3-AUDIT INSERT-only + raw rows; redaction is an access-policy concern applied between `QueryAudit` and the HTTP response. |
| Access policy (admin split) | `internal/server` middleware | — | `X-Admin-Token` check is a request-time HTTP concern; the store has no concept of "admin". |
| CI gate decision | `.github/workflows/eval-gate.yml` (YAML + small Go/step logic) | `cmd/eval` (emits the data) | The gate is CI-process logic; `cmd/eval` produces `report.json` but does NOT exit non-zero on low faithfulness in Phase 3 scope — the workflow parses + decides. (Consider extracting a pure `gateDecision()` Go function for unit testability — see Validation Architecture.) |
| Dashboard rendering | Browser (React 19) | Vite dev server proxy → Hertz (prod single-origin) | `/eval` route is admin-mode; its own (non-SSE) transport; reads `/api/eval/*` + `/api/audit` over plain JSON fetch. |
| Frozen-set pin assertion | `cmd/eval` (CLI flag `--frozen`) | `internal/eval` (lock loader) | The pin check runs in the CLI before metrics; the library loads + exposes lock fields. |

## Standard Stack

### Core (all already in go.mod — Phase 6 adds ZERO Go dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `cloudwego/eino` | v0.7.7 | Agent + ChatModel (judge + agent) | Already the locked agent framework; the eval `Judge` wraps `model.ToolCallingChatModel`. `[VERIFIED: go.mod]` |
| `cloudwego/hertz` | v0.10.6 | HTTP server for `/api/audit` + `/api/eval/*` | Already the locked HTTP server; the new handlers register on the same `server.Hertz` via `server.New()`. `[VERIFIED: go.mod + internal/server/server.go]` |
| `modernc.org/sqlite` | (transitive via `internal/store/sqlite`) | Audit store backend | Pure-Go, zero-ops; `QueryAudit` + `AuditFilter` already shipped. `[VERIFIED: internal/store/sqlite]` |
| `stretchr/testify` | v1.11.1 | Test assertions | Standard Go test lib; already used across `internal/eval/*_test.go` + `chat_session_test.go`. `[VERIFIED: go.mod]` |
| React 19 + Vite + TS + Tailwind v4 + Zustand v5 | (web/package.json) | Dashboard frontend | Locked stack; the dashboard is a new route on this substrate. `[VERIFIED: web/package.json]` |

### Supporting (Phase 6 may add — BOTH flagged for planner decision)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `recharts` | 3.10.1 | Trend line chart + dashed frozen-baseline overlay + tooltip | **Primary recommendation for the Trend panel (D6-16).** React 19 peer-compatible (`^19.0.0` in peers); the de-facto React charting lib; handles axes/responsive-container/tooltip that would be real work to hand-roll. No `postinstall` script (safe). `[VERIFIED: npm registry — npm view recharts; peers include React 19]` BUT `[ASSUMED]` (slopcheck unavailable — see Package Legitimacy Audit). |
| `react-router` | 8.3.0 (unified; `react-router-dom@7.18.2` is the v7 shim) | Real URL routes for `/eval` + `/eval/audit` | **Alternative to a minimal in-store router.** Use IF the planner wants shareable deep links + browser back-button. The modern package is `react-router` (v7 merged the `-dom` suffix away); `react-router-dom@7` is still published as a thin re-export. `[VERIFIED: npm registry]` BUT `[ASSUMED]`. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `recharts` (trend chart) | Hand-rolled SVG `<polyline>` + dashed `<line>` | Aligns with the project's strong anti-dependency stance (D14, shadcn copy-paste, LangChain banned for bloat). Feasible for a line + dashed baseline + scatter points. **Use IF** the planner wants zero new FE deps; recharts is ~95KB gzipped. The trend panel is a headline SC-1 feature, so polish matters — recharts is the lower-risk choice; hand-rolled is the lower-bloat choice. |
| `react-router` (routes) | Minimal in-store router: `route: 'chat' \| 'eval' \| 'eval-audit'` field on `app-store.ts` + a `popstate`/`hashchange` listener (~30 LOC) | Aligns with D14 (everything in app-store) + 5 phases of avoiding react-router. Only 3 routes, no nesting beyond 1 level, admin-mode (no user deep links). **Lean toward the in-store router** given the project pattern; react-router is the standard but heavier option. |
| `eval/reports/INDEX.json` (trend history) | sqlite `EvalRun` table | Rejected by D6-02 — pollutes audit-store semantic boundary + INSERT-only rule does not apply to eval metadata. File-based wins. |
| Server-layer retention projection | Store-layer `View` / redact-in-SQL | Rejected — would either add an UPDATE/DELETE path (violates T-3-AUDIT) or push access-policy into the store (wrong tier). Read-time server projection is correct. |

**Installation (frontend only — Go side adds nothing):**
```bash
# Option A (recharts + minimal in-store router — recommended lean):
cd web && npm install recharts

# Option B (recharts + react-router — if planner wants real URLs):
cd web && npm install recharts react-router
```

**Version verification (run before finalizing the planner's Standard Stack):**
```bash
npm view recharts version            # → 3.10.1 (verified 2026-08-13)
npm view recharts peerDependencies   # → react: ^16||^17||^18||^19  (React 19 OK)
npm view react-router version        # → 8.3.0
npm view react-router-dom version    # → 7.18.2
# Go side: no new deps — go.mod is unchanged.
```

## Package Legitimacy Audit

> slopcheck could NOT be installed in this environment (`pip install slopcheck` failed both with and without `--break-system-packages`; `slopcheck` not on PATH). Per the graceful-degradation protocol, **every recommended package below is tagged `[ASSUMED]`** and the planner MUST gate each install behind a `checkpoint:human-verify` task before `npm install`. The Go side adds no new packages (all verified present in `go.mod`).

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| `recharts` | npm | mature (last published 2026-07-25 per `npm view recharts time.modified`) | high (de-facto React charting lib) | github.com/recharts/recharts | unavailable → `[ASSUMED]` | Approved — planner inserts `checkpoint:human-verify` before `npm install recharts`. No `postinstall` script (verified `npm view recharts scripts.postinstall` → empty). |
| `react-router` (optional) | npm | mature | very high | github.com/remix-run/react-router | unavailable → `[ASSUMED]` | Approved (optional) — planner inserts `checkpoint:human-verify` if chosen over the in-store router. |

**Packages removed due to slopcheck [SLOP] verdict:** none (no packages flagged — slopcheck did not run).
**Packages flagged as suspicious [SUS]:** none.

*Both packages are extremely well-known and their existence + React 19 peer compatibility were verified directly via `npm view` against the npm registry. The `[ASSUMED]` tag is purely because slopcheck was unavailable — it is NOT a slopsquat signal. The planner's `checkpoint:human-verify` is a one-line confirmation that `github.com/recharts/recharts` and `github.com/remix-run/react-router` are the expected repos (they are).*

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────────────────────────┐
   PR opened              │  .github/workflows/eval-gate.yml         │
   (skills/ OR eval/ OR   │  ┌─────────────────────────────────────┐ │
   internal/eval OR       │  │ 1. path-filter (D6-09)              │ │
   internal/agent changed)│  │ 2. go run ./cmd/eval --frozen=...   │ │
        │                 │  │    └─ reads eval/frozen/<skill>.jsonl│ │
        ▼                 │  │    └─ reads eval/frozen/<skill>.lock│ │
   ┌────────────┐   report.json   ┌────────────────┐  │ 3. parse report.json          │ │
   │ cmd/eval   │────────────────▶│ eval/reports/  │  │    gateDecision(report, thresh)│ │
   │ (batch CLI)│   --archive ───▶│  INDEX.json    │  │    → fail if faithfulness<thr  │ │
   │            │                 │  <ts>.json     │  │ 4. gh pr comment (markdown)    │ │
   │ --golden   │                 │  <judge>/<ts>  │  │ 5. upload report+calibration   │ │
   │ --frozen   │                 └──────┬─────────┘  └────────────────────────────────┘ │
   │ --archive  │                        │                       │ PR comment + ❌ check  │
   │ --judge-   │                        │                       ▼                       │
   │  models    │                        │              ┌─────────────────┐              │
   └─────┬──────┘                        │              │ GitHub PR       │              │
         │ backfill                      │              │ (SC-2 gate)     │              │
         ▼                               │              └─────────────────┘              │
   ┌────────────┐                        │              └────────────────────────────────┘
   │internal/   │                        │  30s poll
   │ store.     │                        │  (D6-03)
   │ UpdateAudit│                        │
   │ Metrics    │                        │              ┌──────────────────────────────┐
   └────────────┘                        └──────────────▶│ Dashboard route /eval        │
                                                         │  ├─ Skills table (SC-1)      │
   ┌────────────┐   QueryAudit(filter)   ┌──────────┐    │  ├─ Trend panel (SC-1/SC-6) │
   │ Hertz      │◀───────────────────────│ /api/    │    │  │   recharts: line + dashed │
   │ internal/  │   redacted OR raw      │ eval/*   │    │  │   frozen baseline overlay │
   │ server     │───────────────────────▶│ /api/    │    │  │   + >5% delta flag (D6-16)│
   │            │   X-Admin-Token ───────▶│ audit    │    │  └─ Cross-eval tab (D6-14)  │
   │  audit.go  │   isAdmin? (D6-06/08)  │          │    ├──────────────────────────────┤
   │  eval.go   │                        └──────┬───┘    │ Audit route /eval/audit      │
   │  retention │                               │        │  ├─ filter (user/skill/time/ │
   │  projection│                               │        │  │  refusal_reason) (SC-3)    │
   └─────┬──────┘                               │        │  ├─ [REDACTED 90d+] badge     │
         │                                      │        │  │  + hover detail (D6-07)    │
         ▼                                      │        │  └─ row-level redaction toggle│
   ┌────────────┐                               │        └──────────────────────────────┘
   │internal/   │                               │                     ▲
   │ store.     │   INSERT-only (T-3-AUDIT)     │                     │ reads item.manifest.*
   │ Store      │   WHERE session_id=? (SC-9)   │        ┌────────────┴─────────────┐
   │ (SQLite)   │◀──────────────────────────────┘        │ GET /api/skills          │
   │            │                                         │ (Phase 5 — dashboard's   │
   │ audit row  │   50×5 concurrency (SC-4)               │ Skills table data source)│
   │ session_id │   TestSessionIsolation50x5              └──────────────────────────┘
   └────────────┘   partitions cleanly
```

**Trace the primary use cases:**
- **SC-2 (CI gate):** PR → `eval-gate.yml` → `cmd/eval --frozen` → `report.json` → `gateDecision()` → `gh pr comment` + status check.
- **SC-1 (dashboard trend):** `cmd/eval --archive` writes `INDEX.json` → Dashboard 30s-polls `/api/eval/reports` → Trend panel renders line + dashed baseline.
- **SC-3/SC-5 (audit + redaction):** `/eval/audit` → `/api/audit?...` → server runs `QueryAudit` → retention projection redacts expired rows (unless `X-Admin-Token`) → JSON → table renders `[REDACTED]` badges.

### Recommended Project Structure (additions ONLY)

```
.github/
└── workflows/
    └── eval-gate.yml                      # NEW (SC-2/GEVL-07) — greenfield; confirmed no .github/ exists

eval/
├── frozen/                                # NEW (D6-13)
│   ├── compliance.jsonl                  #   pinned copy of the 50-case golden set
│   └── compliance.lock.json              #   pin sidecar: {model_ver, prompt_hash, seed, eval_set_version}
├── reports/                               # NEW (D6-02) — git-friendly trend history
│   ├── INDEX.json                        #   lightweight array of run summaries
│   └── <iso-ts>.json                     #   full report copy per --archive run
│   └── <judge>/<iso-ts>.json             #   cross-eval per-judge report (D6-14)
├── golden/compliance.jsonl                # existing (Phase 3)
├── report.json / calibration.json / README.md / NOTICE  # existing

internal/
├── eval/                                  # existing — EXTEND only
│   ├── frozen.go                          # NEW — LockFile type + LoadLock + AssertPin
│   └── archive.go                         # NEW — WriteArchive (report copy + INDEX.json append)
├── server/                                # existing — EXTEND only
│   ├── server.go                          # MODIFY — register 3 new routes + EvalReporter seam on Deps
│   ├── audit.go                           # NEW (SC-3/SC-5) — /api/audit handler + retention projection + admin-token middleware
│   ├── eval.go                            # NEW (SC-1) — /api/eval/reports + /api/eval/frozen handlers
│   ├── eval_test.go / audit_test.go       # NEW — handler + projection tests
│   └── chat_session_test.go               # MODIFY — add TestSessionIsolation50x5 (SC-4)
├── store/                                 # existing — UNCHANGED (retention is server-layer, not store)

cmd/eval/
└── main.go                                # MODIFY — add --archive / --frozen / --judge-models flags + gate decision helper

web/src/
├── App.tsx                                # MODIFY — branch on route (chat three-panel vs /eval admin)
├── store/app-store.ts                     # MODIFY — add route state (+ eval slice: reports, auditRows, filters)
├── components/
│   ├── eval/                              # NEW — dashboard surfaces
│   │   ├── EvalDashboard.tsx              #   /eval main (Skills table + Trend panel + Cross-eval tab)
│   │   ├── SkillsTable.tsx                #   one row per Skill (faith / cp / ar / refusal_acc + ❌/✅ badge)
│   │   ├── TrendPanel.tsx                 #   recharts line + dashed frozen baseline + delta flag
│   │   ├── CrossEvalTab.tsx               #   per-judge diff + >5% flag (D6-14)
│   │   └── AuditTable.tsx                 #   /eval/audit (filter bar + [REDACTED] badges)
│   └── layout/LeftSidebar.tsx             # MODIFY — "Eval Dashboard" nav entry (D6-01)

skills/compliance/
├── SKILL.md                               # existing
└── eval-gate.json                         # NEW (D6-10) — {"faithfulness_threshold": 0.9}  ← see Open Question Q2 on path
```

### Pattern 1: Read-Time Retention Projection (D6-06) — THE privacy crux

**What:** Redact `Query` + `VerdictJSON` on audit rows whose `Timestamp` is older than `retention`, UNLESS the request carries a valid `X-Admin-Token`. Applied in the HTTP handler AFTER `store.QueryAudit`, BEFORE serialization.

**When to use:** Every `/api/audit` response. This is the dual property: the audit row stays immutable in storage (T-3-AUDIT "we can prove what happened"); the content is masked at read time ("we don't keep raw PII past retention").

**Example:**
```go
// Source: derived from internal/store/store.go (AuditRow schema) + CONTEXT D6-06.
// The AuditRow has Query + VerdictJSON (the "answer"); there is NO separate Answer field —
// VerdictJSON IS the answer payload. SC-5 text says "query and answer fields redacted".

const defaultRetentionDays = 90

// redact applies the retention projection to a slice of audit rows IN PLACE.
// isAdmin=true → rows returned verbatim (the admin-token access policy).
// isAdmin=false → rows older than (now - retention) have Query + VerdictJSON replaced.
func redact(rows []store.AuditRow, retention time.Duration, now time.Time, isAdmin bool) {
    if isAdmin {
        return
    }
    boundary := now.Add(-retention)
    for i := range rows {
        if rows[i].Timestamp.Before(boundary) {
            reason := fmt.Sprintf("REDACTED:%dd+ (ts=%s, boundary=%s)",
                int(retention.Hours()/24), rows[i].Timestamp.UTC().Format(time.RFC3339),
                boundary.UTC().Format(time.RFC3339))
            rows[i].Query = reason
            rows[i].VerdictJSON = reason
        }
    }
}

// isAdminRequest reads X-Admin-Token off the request and constant-time-compares it
// against SKILLFORGE_ADMIN_TOKEN. Unset env → isAdmin=false for ALL requests
// (demo works OOB with retention-redacted rows; raw access requires the env).
func isAdminRequest(c *app.RequestContext) bool {
    expected := os.Getenv("SKILLFORGE_ADMIN_TOKEN")
    if expected == "" {
        return false // D6-08 + Claude's Discretion #6: unset → no admin anywhere
    }
    var got string
    c.Request.Header.VisitAll(func(k, v []byte) {
        if string(k) == "X-Admin-Token" {
            got = string(v)
        }
    })
    return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1 // ASVS V6: constant-time
}
```

### Pattern 2: `cmd/eval` Flag Matrix (D6 Claude's Discretion #2 + #4)

**What:** The new flags compose cleanly. The eval-set source (`--golden` vs `--frozen`) is mutually exclusive; `--archive` and `--judge-models` are orthogonal.

| `--golden=<path>` | `--frozen=<skill>` | `--judge-models=<a,b>` | `--archive` | Semantics |
|:-:|:-:|:-:|:-:|:-|
| ✓ | ✗ | ✗ | ✗ | Phase-3 behavior (live golden, single judge, overwrite report.json) |
| ✓ | ✗ | ✓ | ✓ | Cross-eval against live golden (D6-14), archived |
| ✗ | ✓ | ✗ | ✓ | **Strict reproducibility** (frozen + StubJudge in test) OR **live-frozen** (frozen + real judge, 1e-3 tolerance, D6-15) |
| ✗ | ✓ | ✓ | ✓ | Cross-eval against frozen set (judges vary, eval set pinned) |
| ✓ | ✓ | — | — | **ERROR** — mutually exclusive (both specify the eval-set source) |

**INDEX.json schema (D6-02):**
```json
{
  "version": 1,
  "runs": [
    {
      "ts": "2026-08-13T10-05-21Z",
      "skill": "compliance",
      "judge_model": "glm-5.2",
      "faithfulness": 0.92,
      "context_precision": 0.85,
      "answer_relevance": 0.88,
      "context_recall": 0.79,
      "refusal_accuracy": 0.95,
      "frozen_version": "compliance-2026-08-13-v1",
      "report_path": "2026-08-13T10-05-21Z.json"
    }
  ]
}
```
`refusal_accuracy` = `Report.PerCategory["refusal-required"]["context_recall"]` (derived — see Pattern 3). `report_path` is relative to `eval/reports/`.

### Pattern 3: "Citation precision" + "Refusal accuracy" — NO new metric (Claude's Discretion #1)

**What:** The SC-1 metric list ("faithfulness, citation precision, refusal accuracy") maps onto the 4 existing Phase-3 metrics with zero new code:

| SC-1 / SC-3 dashboard label | Source | Ragas-aligned? |
|-----------------------------|--------|----------------|
| **Faithfulness** | `Report.Overall["faithfulness"]` | Yes — ports Ragas v0.1.21 prompt VERBATIM `[CITED: docs.ragas.io/en/stable/concepts/metrics/available_metrics/faithfulness/]` |
| **Citation precision** | `Report.Overall["context_precision"]` (display-renamed) | Yes — Ragas `context_precision` IS "whether retrieved contexts are useful for answering" `[CITED: docs.ragas.io/en/stable/concepts/metrics/available_metrics/context_precision/]`. "Citation precision" is the SC text's plain-English name for the same concept. |
| **Refusal accuracy** | `Report.PerCategory["refusal-required"]["context_recall"]` (derived) | Derived. The golden set's `refusal-required` cases carry `ground_truth: "REFUSAL"`. `ContextRecall` returns `1.0` if the system refused, `0.0` otherwise (see `internal/eval/metrics.go:184-195`). The category mean = (correct refusals) / (total refusal-required cases) = the refusal-accuracy rate. No new metric, no new judge call. |

**Key insight:** `internal/eval/metrics.go` already special-cases `groundTruth == "REFUSAL"` → returns `1.0` on refusal, `0.0` on answer. This IS the refusal-accuracy signal, already computed, already aggregated in `PerCategory`. `[VERIFIED: internal/eval/metrics.go:184-195 + dataset.go PerCategory aggregation]`

### Pattern 4: `EvalReporter` seam on `Deps` (testability — D6 Claude's Discretion #3)

**What:** The `/api/eval/*` handlers should NOT read `eval/reports/` directly off the filesystem — that couples handler tests to disk. Introduce a thin interface on `Deps` (mirrors the existing `KnowledgeBuilder` pattern at `server.go:98-101`):

```go
// internal/server/eval.go
type EvalReporter interface {
    Index() (EvalIndex, error)                  // reads eval/reports/INDEX.json
    Report(ts string) (eval.Report, error)      // reads one eval/reports/<ts>.json
    FrozenSets() ([]FrozenSetInfo, error)       // scans eval/frozen/*.lock.json
}
```
Production wires a filesystem-backed impl at `cmd/server/main.go`; tests inject an in-memory stub. The `/api/audit` handler reuses the existing `Deps.Store` (no new seam — `store.QueryAudit` is already the interface).

### Anti-Patterns to Avoid

- **Putting retention redaction in the store layer** — violates T-3-AUDIT (INSERT-only) and pushes access policy into the wrong tier. The store returns RAW rows; the server handler projects. `[CONTEXT specifics §"Retention redaction is a READ-time projection, never a write"]`
- **Adding a sqlite `EvalRun` table** — D6-02 explicitly rejected; eval metadata ≠ audit data, and the file-based `INDEX.json` is git-diffable (an OSS-contributor win). Don't re-litigate.
- **Making `cmd/eval` exit non-zero on low faithfulness** — the CLI's job is to EMIT data (Phase 3 contract, `cmd/eval/main.go:53-56` comment). The CI gate decision belongs in the workflow (or a pure `gateDecision()` Go helper the workflow calls). Conflating them couples the batch tool to the promotion policy.
- **Adding new `SkillForgeChunk` wire types for the dashboard** — the `/eval` route has its OWN (non-SSE) transport. D-04/D12/D15 invariants are preserved by ISOLATION; do not touch the chat wire. `[CONTEXT code_context §Established Patterns]`
- **A `messages` field in the dashboard's Zustand slice** — D14 bans it app-wide; the dashboard slice holds `reports`/`auditRows`/`filters`, never `messages`. `[VERIFIED: web/src/store/app-store.ts file-header comment]`
- **Goroutine-spawn per session in the SC-4 test without a WaitGroup + `-race`** — the existing `TestSessionIsolationConcurrent` pattern (`chat_session_test.go:267-313`) is the template; scale it, don't rewrite it.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Trend line chart with dashed baseline overlay + tooltip | A bespoke canvas/SVG chart engine | `recharts` `<LineChart>` + `<ReferenceLine strokeDasharray>` (OR hand-rolled SVG IF planner picks the anti-dep option) | Axes, responsive scaling, hover tooltips, and a dashed reference line are each real work; recharts composes them. The project's anti-dep stance makes hand-rolled defensible ONLY for a minimal line+points. |
| SQL filter for `/api/audit` | New WHERE-clause builder | `store.QueryAudit(AuditFilter)` — already 100% `?`-parameterized, T-3-SQLI regression-tested (DROP TABLE survives in 3 filter fields) | The Phase-3 filter is the SQLi-safe surface; `/api/audit` is a 1:1 param map. Rebuilding it re-introduces the injection vector. `[VERIFIED: internal/store/sqlite/audit.go:85-168]` |
| LIMIT/OFFSET pagination | Custom cursor + sort index | Add `Offset int` to `AuditFilter` + `LIMIT ? OFFSET ?` in the existing QueryAudit SQL | D6-05 explicitly rejected cursor+server-sort as over-engineered for ~50-row dev datasets. A 2-line SQL addition is the proportionate response. |
| Per-case metric computation | Re-derive faithfulness/precision/recall | `internal/eval.{Faithfulness, ContextPrecision, AnswerRelevancy, ContextRecall}` | Shipped + tested in Phase 3 (D19). Phase 6 reads `Report`, never recomputes. |
| Reproducibility (strict layer) | A new deterministic harness | The existing `TestRunnerReproducibility` + `StubJudge` + `deterministicEmbedder` (SHA-derived) | Already gates SC-6 at `1e-9`; Phase 6 relaxes to `1e-6` + adds a live layer. The seed infrastructure exists. `[VERIFIED: internal/eval/eval_test.go:51-76, 187-215]` |
| Session-isolation enforcement | New partitioning logic | The existing `WHERE session_id=?` in `GetSession` + `AppendTurn` UNIQUE constraint | SC-4 adds TEST VOLUME (50×5), not new enforcement. The invariant is unchanged (D18). `[VERIFIED: internal/store/sqlite/session.go + chat_session_test.go]` |

**Key insight:** This phase is a HARDENING phase on a shipped substrate. The temptation to rebuild is the trap — almost every SC is satisfied by a thin adapter over Phase-3 code. The new build is handlers + flags + workflow + frontend surfaces.

## Runtime State Inventory

> This phase is primarily ADDITIVE (new routes, new files, new endpoints) but touches 3 strings/behaviors that have runtime state implications.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `skillforge.db` audit table has live rows with `prompt_hash` computed from the CURRENT `computePromptHash(skillName, modelVersion)` (skillName+modelVersion only — see Open Question Q1). If Q1 is resolved by STRENGTHENING the hash to include the rendered prompt body, existing rows' `prompt_hash` values become stale (the hash algorithm changed). | **Code edit + data note.** No migration needed (prompt_hash is a reproducibility key, not a lookup key with external consumers; dev-scale DB). But the planner MUST decide Q1 before shipping, and the `--frozen` lock's `prompt_hash` pin semantics depend on it. |
| Live service config | None — Phase 6 adds new env vars (`SKILLFORGE_RETENTION_DAYS`, `SKILLFORGE_ADMIN_TOKEN`) read at request time; no running service re-config. `JUDGE_MODEL` already exists (Phase 3). | None. Document the 2 new env vars in `eval/README.md` + `.env.example` (the latter is Phase 7 DIST-03, but listing them now is cheap). |
| OS-registered state | None — no Task Scheduler / launchd / systemd registrations. | None — verified by the greenfield `.github/workflows/` (no CI exists to re-register). |
| Secrets/env vars | NEW: `SKILLFORGE_ADMIN_TOKEN` (raw-audit access gate); NEW CI secret: `GLM_API_KEY` in GitHub Actions (the eval gate calls GLM). Existing: `GLM_API_KEY`, `JUDGE_MODEL`, `BGE_M3_URL`, etc. (Phase 3). | **CI secret setup (manual, outside the repo).** `GLM_API_KEY` MUST be a GitHub Actions secret (D6-09: never logged, never on the default branch via push). The workflow YAML references `${{ secrets.GLM_API_KEY }}`. |
| Build artifacts | `eval/reports/` + `eval/frozen/` are NEW directories created on first `--archive`/`--frozen` run. No installed-package artifacts (no Go deps added; FE deps land in `web/node_modules` as normal). | **.gitignore decision.** `eval/reports/*.json` individual runs are likely git-tracked (the trend history IS the artifact — D6-02 rationale "OSS contributors can read history in git diff"). Decide: track `INDEX.json` + baseline run, `.gitignore` the rest? Planner calls. |

**Nothing found in category:** OS-registered state — explicitly verified (no `.github/`, no scripts registering OS tasks).

## Common Pitfalls

### Pitfall 1: `prompt_hash` is WEAKER than SC-6 implies (reproducibility hole)
**What goes wrong:** The frozen `.lock.json` pins `prompt_hash` (D6-13). A planner/implementer assumes "prompt_hash pins the rendered prompt." It does NOT — `computePromptHash` at `internal/server/chat.go:420-425` hashes ONLY `skillName + modelVersion`. If someone edits the Skill's prompt body WITHOUT changing its name or the model, the `prompt_hash` is unchanged — yet the actual prompt DID change. The frozen gate would pass while the system drifted.
**Why it happens:** The chat.go:416-419 comment explicitly defers the lift: "The system-prompt body is NOT hashed in Phase 3 because the agent's prompt is a compile-time constant — Phase 5 lifts the rendered prompt body into the hash input." Phase 5 shipped without doing this lift.
**How to avoid:** Resolve Open Question Q1 BEFORE planning the frozen-set tasks. Two paths: (a) strengthen `computePromptHash` to hash the rendered prompt body (small change in chat.go + cmd/eval + the lock writer), accepting that existing audit rows' hashes change; OR (b) keep the current semantics and DOCUMENT that the frozen lock pins `(skillName, modelVersion)`, not the prompt body — then add a content hash of the frozen JSONL itself to the lock as the real drift signal.
**Warning signs:** A frozen-set test that passes after editing `skills/compliance/SKILL.md` steps/agent text but not the name.

### Pitfall 2: eval-gate.json path mismatch (CONTEXT discrepancy)
**What goes wrong:** CONTEXT D6-10 says the per-Skill threshold file is at `.claude/skills/<skill>/eval-gate.json`. But the product Skills live at `skills/<name>/SKILL.md` (verified — the ONLY product Skill is `skills/compliance/SKILL.md`). `.claude/skills/` holds META skills (spike-findings, sketch-findings), NOT the SkillForge product Skills. Putting eval-gate.json under `.claude/skills/compliance/` would orphan it from the Skill it gates.
**Why it happens:** CONTEXT conflated the two `skills/` directories. The D6 "specifics" text says "co-located with the Skill it gates — same directory as SKILL.md," which is the stronger signal.
**How to avoid:** Place `eval-gate.json` at `skills/<skill>/eval-gate.json` (i.e. `skills/compliance/eval-gate.json`), matching the real `skills/<name>/SKILL.md` layout. The CI path-filter (D6-09: `skills/*/SKILL.md`) already points at the correct directory.
**Warning signs:** A workflow that scans `.claude/skills/*/eval-gate.json` finds nothing (the product threshold never loads → default 0.9 always applies → the per-Skill threshold feature is silently dead).

### Pitfall 3: Live reproducibility test burning GLM quota in `go test ./...`
**What goes wrong:** SC-6's live-jitter layer (`--frozen` + real judge, `1e-3` tolerance, D6-15) requires real GLM API calls. If wired as a plain `go test` function, every `go test ./internal/eval/...` run (per task commit, per wave merge) burns ~50 LLM calls × 5-10 min + real money, and is non-deterministic (network/tokens).
**Why it happens:** SC-6's two-layer tolerance explicitly wants the live path; an implementer wires it naively into the default test suite.
**How to avoid:** Guard the live reproducibility test behind a build tag (`//go:build liveeval`) OR skip-when-`GLM_API_KEY`-unset OR skip-when-not-CI. The default `go test ./...` runs ONLY the StubJudge strict layer (`1e-6`, fast, hermetic). The live layer runs in the eval-gate workflow (which has the secret + budget) — NOT in the per-commit quick suite.
**Warning signs:** CI runs taking 10+ minutes on every push; GLM quota exhaustion; flaky `go test` under load.

### Pitfall 4: Retention redaction applied at the wrong tier (T-3-AUDIT violation)
**What goes wrong:** An implementer "efficiently" adds an `UPDATE audit SET query='[REDACTED]' WHERE ts < ?` path (or a TTL delete job) to enforce retention. This destroys the immutable audit trail — the compliance-audit crux ("we can prove what happened") is lost.
**Why it happens:** Retention-as-delete is the intuitive reading of "queries past the retention window have fields redacted in storage" (SC-5 text). But CONTEXT D6-06 + specifics override: "Retention redaction is a READ-time projection, never a write."
**How to avoid:** Redaction lives in the HTTP handler (Pattern 1 above), BETWEEN `store.QueryAudit` and `c.JSON(...)`. The store package gains NO UPDATE/DELETE path. The meta-record (timestamp, session_id, skill_id, refusal_reason, metrics) survives; only `Query` + `VerdictJSON` are masked.
**Warning signs:** Any new `UPDATE` or `DELETE` SQL string in `internal/store/sqlite/audit.go`; a goroutine with a ticker in `cmd/server`.

### Pitfall 5: SC-4 concurrency test that doesn't exercise the real isolation boundary
**What goes wrong:** A 50×5 test that spawns 50 goroutines but reuses a shared session_id, OR asserts only on HTTP response bodies (not on `store.GetSession`), passes without proving isolation.
**Why it happens:** Naive scaling of the 2-session test without preserving its load-bearing assertions (`msgsA` contains ONLY sess-A's query; `msgsB` ONLY sess-B's).
**How to avoid:** The 50×5 test MUST (a) give each goroutine a UNIQUE `X-Session-Id`, (b) do 5 sequential POSTs per session, (c) after all settle, assert `GetSession(sessN)` returns exactly 10 messages (5 user + 5 assistant), AND (d) for each session, assert NONE of its messages contain any OTHER session's query text. Run under `go test -race`. Use the existing `cannedChatModel` (refuses every request → exercises the refusal-path audit write too).
**Warning signs:** A test with `assert.Len(msgs, 10)` but no cross-session leakage assertion; a test not run under `-race`.

### Pitfall 6: CI gate parsing `report.json` with a fragile jq one-liner
**What goes wrong:** The workflow does `jq '.overall.faithfulness < 0.9'` and fails closed/open on a malformed report, OR can't name WHICH Skill dropped (SC-2 requires the PR comment name the Skill).
**Why it happens:** report.json is `{overall: {...}, per_category: {...}, cases: [...]}` — flat for ONE skill, but the gate must be per-Skill for multi-Skill futures. A jq path that reads `.overall.faithfulness` works for Phase 6's single Skill but doesn't generalize.
**How to avoid:** Extract a pure Go helper `gateDecision(report eval.Report, thresholds map[string]float64) (failed []SkillResult)` and unit-test it. The workflow runs a tiny `go run ./cmd/evalgate` (or `cmd/eval --gate-check`) that reads report.json + the per-Skill thresholds, prints the markdown table + exits non-zero on failure. This makes the gate logic testable in Go (fast, hermetic) and the workflow a thin shell.
**Warning signs:** Inline `jq` in the workflow YAML; no Go unit test for the threshold decision.

### Pitfall 7: `INDEX.json` concurrent-write corruption
**What goes wrong:** Two `cmd/eval --archive` runs race on `INDEX.json` (e.g. a cross-eval `--judge-models a,b` writes 2 entries, OR CI + a local run overlap). Read-modify-write of a JSON array is not atomic.
**Why it happens:** `--archive` appends to `INDEX.json`; naive `os.ReadFile` + append + `os.WriteFile` is a classic TOCTOU.
**How to avoid:** Write to a temp file + `os.Rename` (atomic on POSIX; on Windows the rename-over-existing needs `MOVEFILE_REPLACE_EXISTING` — `os.Rename` handles this in Go 1.5+ for same-volume). For the cross-eval case, serialize the 2 judge runs within the single CLI process (they share one `INDEX.json`). Concurrent PROCESSES are rare (CI vs local) — document that `--archive` is not safe under concurrent processes (a reasonable Phase 6 limit; the file IS git-tracked so the canonical history is git, not the FS).
**Warning signs:** Truncated/empty `INDEX.json`; JSON parse errors on the dashboard.

## Code Examples

### Example 1: `/api/audit` handler with retention projection (SC-3 + SC-5)
```go
// Source: derived from internal/store/store.go (AuditFilter) + CONTEXT D6-05/D6-06.
// Maps query params 1:1 to the existing parameterized filter.

func (s *Deps) auditHandler(ctx context.Context, c *app.RequestContext) {
    if s.Store == nil {
        c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "store not initialized"})
        return
    }
    f := store.AuditFilter{
        UserID:        string(c.Query("user")),
        SkillID:       string(c.Query("skill")),
        RefusalReason: string(c.Query("refusal_reason")),
        Since:         parseTime(c.Query("since")),
        Until:         parseTime(c.Query("until")),
        Limit:         atoiOr(c.Query("limit"), 100),
        // Offset: atoiOr(c.Query("offset"), 0),  // requires adding Offset to AuditFilter + LIMIT/OFFSET SQL
    }
    rows, err := s.Store.QueryAudit(ctx, f)
    if err != nil {
        c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }
    retention := retentionDays()              // SKILLFORGE_RETENTION_DAYS default 90
    redact(rows, retention, time.Now(), isAdminRequest(c))  // Pattern 1
    c.JSON(http.StatusOK, rows)
}
```

### Example 2: Frozen-set lock + pin assertion (SC-6, D6-13)
```go
// Source: derived from internal/eval/dataset.go (Case) + CONTEXT D6-13/D6-15.

// LockFile is the .lock.json sidecar pinning the frozen set's reproducibility inputs.
type LockFile struct {
    ModelVersion   string `json:"model_ver"`      // e.g. "glm-5.2" — --frozen asserts live agent model matches
    PromptHash     string `json:"prompt_hash"`    // computePromptHash(skillName, modelVer) — see Pitfall 1
    Seed           int64  `json:"seed"`           // for the deterministic strict layer
    EvalSetVersion string `json:"eval_set_version"` // human-readable content tag, e.g. "compliance-2026-08-13-v1"
    SkillName      string `json:"skill_name"`     // the skillName input to computePromptHash
}

// AssertPin verifies the live runtime matches the frozen lock. Called by cmd/eval --frozen.
// Returns an error naming the drifted field (the SC-6 failure signal).
func AssertPin(lock LockFile, liveModel, liveSkillName string) error {
    if liveModel != lock.ModelVersion {
        return fmt.Errorf("frozen pin: model_ver drift (lock=%s, live=%s)", lock.ModelVersion, liveModel)
    }
    liveHash := computePromptHash(liveSkillName, liveModel) // MUST match chat.go's hash EXACTLY
    if liveHash != lock.PromptHash {
        return fmt.Errorf("frozen pin: prompt_hash drift (lock=%s, live=%s)", lock.PromptHash, liveHash)
    }
    return nil
}
```

### Example 3: Strengthened `TestRunnerReproducibility` + new live path (SC-6, D6-15)
```go
// Source: extends internal/eval/eval_test.go::TestRunnerReproducibility (currently asserts 1e-9).

// STRICT layer (hermetic, default suite) — strengthened from 1e-9 to 1e-6 (the prod-style tolerance).
func TestRunnerReproducibility(t *testing.T) {
    // ... existing setup unchanged ...
    for i, s := range samples {
        for _, m := range []string{"faith", "arel", "cprec", "crecall"} {
            v1 := run1[i][m].(float64)
            v2 := run2[i][m].(float64)
            assert.True(t, math.Abs(v1-v2) < 1e-6,  // was 1e-9; D6-15 new strict tolerance
                "case %s metric %s: drift %v", s.ID, m, v1-v2)
        }
    }
}

// LIVE layer (jitter-tolerant, NOT in default suite — see Pitfall 3).
//go:build liveeval
func TestRunnerLiveReproducibility(t *testing.T) {
    if os.Getenv("GLM_API_KEY") == "" { t.Skip("GLM_API_KEY unset") }
    // ... build real judge via NewGLMChatModel, load frozen set, run TWICE ...
    for i, s := range samples {
        for _, m := range []string{"faith", "arel", "cprec", "crecall"} {
            v1 := run1[i][m].(float64)
            v2 := run2[i][m].(float64)
            assert.True(t, math.Abs(v1-v2) < 1e-3,  // D6-15 jitter tolerance
                "case %s metric %s: live drift %v", s.ID, m, v1-v2)
        }
    }
}
```

### Example 4: Trend panel with frozen-baseline overlay (SC-1 + SC-6, D6-16)
```tsx
// Source: recharts 3.x API (verified peers support React 19). Alternative: hand-rolled SVG.
import { LineChart, Line, ReferenceLine, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

// data = INDEX.json runs mapped to [{ts, faithfulness}], baseline = frozen lock's recorded faithfulness.
<TrendPanel runs={runs} baseline={frozenBaseline} deltaThresholdPct={5} />

function TrendPanel({ runs, baseline, deltaThresholdPct }: Props) {
  const latest = runs.at(-1)?.faithfulness ?? 0;
  const driftPct = baseline ? Math.abs(latest - baseline) * 100 : 0;
  return (
    <div>
      {baseline && driftPct > deltaThresholdPct && (
        <div role="alert" data-testid="trend-drift-flag">
          ⚠ Live faithfulness drifted {driftPct.toFixed(1)}% from frozen baseline (D6-14/D6-16)
        </div>
      )}
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={runs}>
          <XAxis dataKey="ts" /><YAxis domain={[0, 1]} />
          <Tooltip /><Line dataKey="faithfulness" stroke="var(--status-compliant)" dot />
          {baseline && (
            <ReferenceLine y={baseline} stroke="var(--text-muted)" strokeDasharray="4 4"
              label={{ value: 'frozen baseline', fill: 'var(--text-muted)' }} />
          )}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ragas as Python sidecar | Go-native evaluator (`internal/eval`) | D6 reversed 2026-08-12 (Phase 3) | Single-language stack; faithfulness ports Ragas v0.1.21 prompt VERBATIM (the headline number stays "Ragas 口径"); eliminates Python/ragas pip friction. Phase 6 STAYS Go-native — cross-eval swaps judge_model within Go. |
| `react-router-dom` v6 | `react-router` v7+ (unified package; `-dom` is a thin shim) | React Router v7 (late 2024) | If the planner picks react-router, depend on `react-router` (NOT `react-router-dom`) for v7+ semantics. The `-dom` package still publishes as a re-export for compat. |
| GitHub Actions `pull_request`触发 with path-filter dorny/paths-filter | Native `paths:` filter on the `pull_request` trigger | Actions runner v2.300+ | No third-party action needed for path-filtering — the native `on: pull_request: paths:` is sufficient for D6-09. |
| Per-row `access_policy` column for redaction | Read-time projection + binary admin token | CONTEXT D6-06/D6-08 | The "access requires an explicit access policy" SC-5 text is satisfied by the binary token split; a real ACL is v2. |

**Deprecated/outdated:**
- `react-router-dom` v6 API (`<Switch>`, `useHistory`) — superseded by v7 data routers (`<RouterProvider>`, `useNavigate`). Use v7+ if at all.
- Any Python/Ragas eval harness — D6 reversed; do not reintroduce.

## Assumptions Log

> slopcheck was unavailable, so package claims carry `[ASSUMED]`. Other claims are verified against the live codebase.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `recharts@3.10.1` works with React 19 + Vite + Tailwind v4 without config changes | Standard Stack / Supporting | MEDIUM — peer deps list React 19 (verified), but a Tailwind v4 CSS-reset interaction is possible; the planner's `checkpoint:human-verify` catches it. Fallback: hand-rolled SVG. |
| A2 | `react-router` (if chosen) integrates cleanly with the existing `App.tsx` composition root | Standard Stack / Alternatives | LOW — standard; but the project has avoided it for 5 phases, so the in-store router is the safer lean. |
| A3 | `computePromptHash` strengthening (Q1 option a) won't break downstream consumers | Runtime State / Pitfall 1 | LOW-MEDIUM — `prompt_hash` is a reproducibility key read only by the new frozen-lock assertion + the dashboard; no external consumer. Dev-scale DB. Verified no grep hit outside `internal/store` + `internal/server/chat.go`. |
| A4 | GitHub-hosted Actions runners have Go 1.25 available (or the workflow pins `setup-go@v5` with `go-version: 1.25.x`) | CI gate | LOW — `actions/setup-go@v5` supports any Go version; pin it explicitly. `go.mod` requires `go 1.25.0`. |
| A5 | The 50×5 concurrency test completes in <60s under `-race` with `cannedChatModel` | Validation Architecture | LOW-MEDIUM — 250 sequential-canned POSTs against an in-process Hertz + in-memory SQLite; the existing 2-session test is ~sub-second. If slow, reduce via table-driven scaling or mark as wave-merge-only. |
| A6 | `recharts` + `react-router` are NOT slopsquatted (slopcheck could not verify) | Package Legitimacy Audit | VERY LOW — both are among the most-installed npm packages; `npm view` confirmed canonical repos (recharts/recharts, remix-run/react-router). `[ASSUMED]` tag is procedural, not evidential. |

**If this table's only `[ASSUMED]`-of-substance items are A1/A2/A3:** they are all planner-gated (Q1) or `checkpoint:human-verify`-gated (A1/A2). No silent assumptions flow into the plan.

## Open Questions

1. **`prompt_hash` strengthening (BLOCKING — resolves before planning frozen-set tasks).**
   - What we know: `computePromptHash` (`internal/server/chat.go:420-425`) hashes ONLY `skillName+modelVersion`. The chat.go comment says Phase 5 would lift the rendered prompt body into the hash; that lift was NOT done. SC-6 + D6-13 say the frozen lock pins `prompt_hash`.
   - What's unclear: does Phase 6 (a) STRENGTHEN the hash to include the rendered system prompt body (making the frozen pin meaningful for prompt drift), or (b) KEEP the current semantics and pin the frozen JSONL content hash instead?
   - Recommendation: **(a) strengthen** — it's a ~5-line change in `chat.go` + a matching one in the lock writer, makes the frozen pin meaningful, and `prompt_hash` has no external consumer. Migration cost: existing audit rows' hashes change (acceptable — dev-scale, reproducibility key not lookup key). If the planner wants (b), the lock MUST add a `eval_set_sha256` field pinning the frozen JSONL content as the real drift signal.

2. **`eval-gate.json` path (BLOCKING — resolves before planning the CI gate).**
   - What we know: CONTEXT D6-10 says `.claude/skills/<skill>/eval-gate.json`; the D6 "specifics" says "co-located with the Skill it gates — same directory as SKILL.md." The product Skill is at `skills/compliance/SKILL.md`.
   - What's unclear: which path wins?
   - Recommendation: **`skills/<skill>/eval-gate.json`** (i.e. `skills/compliance/eval-gate.json`) — honors the "co-located" specificity, matches the D6-09 path-filter (`skills/*/SKILL.md`), and is where a fork contributor would look. The `.claude/skills/` path is a CONTEXT typo.

3. **Trend chart library: recharts vs hand-rolled SVG (planner decision).**
   - What we know: recharts 3.10.1 supports React 19 (peers verified); the project has a strong anti-dep stance (D14, shadcn copy-paste); the trend panel is a headline SC-1 feature.
   - Recommendation: **recharts** for polish + lower risk; the ~95KB gzipped is proportionate to a headline dashboard feature. Hand-rolled SVG is the defensible anti-dep alternative for a line + dashed baseline + points.

4. **Routing: react-router vs minimal in-store router (planner decision).**
   - What we know: only 3 routes (`/`, `/eval`, `/eval/audit`); admin-mode (no user deep-links needed); 5 phases of avoiding react-router.
   - Recommendation: **minimal in-store router** (a `route` field on `app-store.ts` + a `popstate` listener) — aligns with D14 + the project pattern. Use react-router only if shareable URLs become a requirement.

5. **Should `cmd/eval --archive` runs be git-tracked?**
   - What we know: D6-02 rationale "OSS contributors can read history in git diff" implies tracking. But CI runs on every PR would pollute git history if committed.
   - Recommendation: **track `INDEX.json` + ONE canonical baseline run** (the published-baseline artifact Phase 7 DIST-09 reads); `.gitignore` the per-PR run files. The dashboard reads whatever is on disk locally; git carries only the baseline.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `cmd/eval`, `internal/server`, all Go tests | ✓ | 1.25.0 (`go.mod`) | — |
| Node.js + npm | Frontend dashboard build/test | ✓ (web/package.json exists) | per web/lockfile | — |
| `modernc.org/sqlite` | audit store (pure-Go, no cgo) | ✓ | transitive via internal/store/sqlite | — |
| Milvus OR MongoDB | retriever (for `cmd/eval` live runs) | ✓ (configurable via `RETRIEVER_BACKEND`) | per env | Mongo fallback (D11) |
| BGE-M3 endpoint | embedder (for `cmd/eval` live runs) | ✓ (env `BGE_M3_URL`) | per env | — |
| GLM Coding Plan key | judge + agent (live eval + CI gate) | ✓ locally (env); **CI: MUST be added as GitHub Actions secret** | `GLM_API_KEY` env | StubJudge path for hermetic tests |
| GitHub Actions | CI gate (SC-2/GEVL-07) | ✓ (service available; repo has no `.github/` yet — greenfield) | — | — |
| slopcheck | Package Legitimacy Audit | ✗ | — | Graceful degradation: all recommended pkgs `[ASSUMED]` + planner `checkpoint:human-verify` |

**Missing dependencies with no fallback:** none that block execution. The only manual setup step is adding `GLM_API_KEY` as a GitHub Actions secret (cannot be automated from the repo).

**Missing dependencies with fallback:** slopcheck (fell back to `[ASSUMED]` + manual verify).

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` → this section is REQUIRED and feeds the VALIDATION.md the planner derives.

### Test Framework

| Property | Value |
|----------|-------|
| Framework (BE) | Go `testing` + `stretchr/testify` v1.11.1 (existing) |
| Framework (FE) | Vitest 2.x + @testing-library/react 16 (existing — `web/package.json`) |
| Config file (BE) | none (standard `go test`) |
| Config file (FE) | `web/vitest.config.*` (existing) |
| Quick run command (BE) | `go test ./internal/... ./cmd/...` |
| Quick run command (FE) | `cd web && npx tsc --noEmit && npx vitest run` |
| Full suite command (BE) | `go test -race ./...` |
| Full suite command (FE) | `cd web && npx tsc --noEmit && npx vitest run && npx vite build` |

### Phase Requirements → Test Map

| Req/SC | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| **GEVL-06 / SC-1** | Dashboard renders Skills table (per-Skill: faith/cp/ar/refusal_acc + ❌/✅ badge) from `/api/eval/reports` | FE unit | `cd web && npx vitest run src/components/eval/SkillsTable.test.tsx -t "renders"` | ❌ Wave 0 |
| **GEVL-06 / SC-1** | Trend panel renders line + dashed frozen baseline + >5% drift flag | FE unit | `cd web && npx vitest run src/components/eval/TrendPanel.test.tsx` | ❌ Wave 0 |
| **GEVL-06 / SC-1** | `/api/eval/reports` serves INDEX.json; `/api/eval/reports/:ts` serves one report | BE unit (handler) | `go test ./internal/server/ -run TestEvalReports` | ❌ Wave 0 |
| **GEVL-07 / SC-2** | `gateDecision(report, thresholds)` fails when ANY Skill's faithfulness < threshold; names the Skill | BE unit (pure fn) | `go test ./internal/eval/ -run TestGateDecision` (or `./cmd/evalgate/`) | ❌ Wave 0 |
| **GEVL-07 / SC-2** | PR comment posted as markdown table on failure; report.json+calibration.json uploaded | **manual / CI-only** | n/a (GitHub Actions runtime) — verified by opening a test PR | manual (SC-2 PR) |
| **SC-3** | `/api/audit?user=&skill=&since=&until=&refusal_reason=&limit=&offset=` maps to QueryAudit + paginates | BE unit (handler) | `go test ./internal/server/ -run TestAuditHandler` | ❌ Wave 0 |
| **SC-3** | Audit table renders rows + filter bar | FE unit | `cd web && npx vitest run src/components/eval/AuditTable.test.tsx` | ❌ Wave 0 |
| **SC-3** | SQLi regression still holds (DROP TABLE in params survives) | BE unit (regression) | `go test ./internal/store/sqlite/ -run TestSQLi` (existing — verify still green) | ✅ (Phase 3) |
| **SC-4** | 50 sessions × 5 turns: each GetSession(sessN) has exactly 10 msgs; no cross-session query leakage | BE integration (`-race`) | `go test -race ./internal/server/ -run TestSessionIsolation50x5 -count=1` | ❌ Wave 0 (scales existing `TestSessionIsolationConcurrent`) |
| **SC-5** | Rows past retention → Query + VerdictJSON redacted (non-admin); raw (admin token) | BE unit (projection) | `go test ./internal/server/ -run TestRetentionRedact` | ❌ Wave 0 |
| **SC-5** | `SKILLFORGE_RETENTION_DAYS` override + unset `SKILLFORGE_ADMIN_TOKEN` → all non-admin | BE unit | `go test ./internal/server/ -run TestAdminToken` | ❌ Wave 0 |
| **SC-6** | StubJudge strict layer: `abs(run1-run2) < 1e-6` (strengthened from 1e-9) | BE unit (hermetic) | `go test ./internal/eval/ -run TestRunnerReproducibility` | ✅ (Phase 3 — MODIFY tolerance) |
| **SC-6** | Frozen lock `AssertPin` detects model_ver / prompt_hash drift | BE unit | `go test ./internal/eval/ -run TestAssertPin` | ❌ Wave 0 |
| **SC-6** | Live jitter layer: `abs(run1-run2) < 1e-3` (real judge) | BE integration (`//go:build liveeval`) | `go test -tags liveeval ./internal/eval/ -run TestRunnerLiveReproducibility` (CI only) | ❌ Wave 0 |
| **D6-14** | Cross-eval `--judge-models a,b` writes 2 reports; >5% delta flagged | BE unit (diff fn) + manual | `go test ./internal/eval/ -run TestCrossEvalDiff` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit (quick, <30s):** `go test ./internal/... ./cmd/...` (excludes `-tags liveeval`) + `cd web && npx tsc --noEmit && npx vitest run`. This runs the hermetic SC-6 strict layer, the redaction/audit/gate unit tests, and the FE component tests. Does NOT run the 50×5 concurrency test or the live-jitter test.
- **Per wave merge (full, ~1-3m):** `go test -race ./...` (INCLUDES `TestSessionIsolation50x5`) + full FE suite + `npx vite build`. The `-race` flag is mandatory for SC-4. Excludes `-tags liveeval`.
- **Phase gate (SC-2 + SC-6 live):** (1) Open a sacrificial PR that intentionally drops a Skill's faithfulness → confirm the gate fails + the PR comment names the Skill + threshold (manual SC-2 verification). (2) Run `go test -tags liveeval ./internal/eval/` once with `GLM_API_KEY` set to confirm the live-jitter layer (or run `cmd/eval --frozen` twice and diff). (3) Full suite green. (4) Manual UAT: dashboard renders real trend data from a local `--archive` run; audit table shows `[REDACTED 90d+]` badges on old rows.

**Rationale for sampling rates:**
- **SC-2 (CI gate):** The gate logic is unit-tested as a pure Go function (`gateDecision`) per task commit; the GitHub Actions wiring is verified ONCE at phase gate via a sacrificial PR (the only way to truly test a workflow is to run it). Re-running on every wave would burn GLM quota.
- **SC-4 (concurrency):** Run per-wave under `-race` (not per-commit) — 250 canned POSTs take ~seconds but `-race` adds overhead; the invariant (D18 `WHERE session_id=?`) is unchanged from Phase 3, so per-commit risk is low.
- **SC-6 (reproducibility tolerance):** The strict `1e-6` layer runs per-commit (hermetic, instant). The live `1e-3` layer runs ONLY at phase gate (real GLM quota + non-determinism; guarded by `//go:build liveeval`). This split is the Pitfall-3 counter-measure.

### Wave 0 Gaps

- [ ] `web/src/components/eval/SkillsTable.test.tsx` — covers GEVL-06/SC-1 (table render)
- [ ] `web/src/components/eval/TrendPanel.test.tsx` — covers GEVL-06/SC-1 + SC-6 (baseline overlay + drift flag)
- [ ] `web/src/components/eval/AuditTable.test.tsx` — covers SC-3 (filter + redaction badge)
- [ ] `internal/server/eval_test.go` — covers `/api/eval/reports` + `/api/eval/frozen` handlers
- [ ] `internal/server/audit_test.go` — covers `/api/audit` handler + retention projection + admin token
- [ ] `internal/eval/frozen_test.go` — covers `LoadLock` + `AssertPin` + `WriteArchive`/INDEX.json
- [ ] `internal/eval/gate_test.go` (or `cmd/evalgate`) — covers `gateDecision` pure function
- [ ] `internal/server/chat_session_test.go::TestSessionIsolation50x5` — covers SC-4 (scales existing test)
- [ ] `//go:build liveeval` guard on the new live-reproducibility test — Pitfall 3 counter-measure
- [ ] FE test fixtures: `__fixtures__/index.json` (sample INDEX.json) + `__fixtures__/report.json` (sample report) for dashboard tests
- [ ] No framework install needed — Go testing + Vitest both present.

## Security Domain

> `security_enforcement` is not set in `.planning/config.json` → defaults to enabled. This section is REQUIRED.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | partial (static-token gate, not full auth) | `X-Admin-Token` = `SKILLFORGE_ADMIN_TOKEN` env; **constant-time compare** (`crypto/subtle.ConstantTimeCompare`) — NOT `==`. ASVS V6.2.1. |
| V3 Session Management | no (the dashboard has no login session; the chat session_id is unchanged from Phase 3) | — |
| V4 Access Control | **yes** | Binary admin/non-admin split (D6-08). `/api/audit` raw access requires the token; retention-redacted rows are the default. IDOR concern: `/api/audit` exposes ALL users' queries → the admin token + redaction are the control. A real RBAC is v2 SCALE-*. |
| V5 Input Validation | yes | `/api/audit` query params validated + bound via the existing parameterized `AuditFilter` (T-3-SQLI regression-tested). `offset`/`limit` must be integer-clamped (reject negative/huge). Path-traversal: `/api/eval/reports/:ts` — the `:ts` MUST be validated against a strict regex (`^[0-9T:-]+Z$`) before filesystem read (a `..` in `:ts` is the attack vector). |
| V6 Cryptography | yes | (1) `X-Admin-Token` constant-time compare. (2) `prompt_hash` uses SHA-256 (already). (3) `GLM_API_KEY` as GitHub Actions secret — never in logs, never in report.json/calibration.json (verified: the report writer has no env access by design — `internal/eval/chatmodel.go:121-131`). |
| V8 Sensitive Data Protection | **yes — this is the core SC-5 concern** | Retention redaction of `Query`/`VerdictJSON` (the PII-bearing fields) at read time. Raw rows only via admin token. Verdicts may contain employee names / PII from compliance queries → redaction is the control. |
| V9 Communications | partial | `X-Admin-Token` over plain HTTP in dev is acceptable (local-first); production should be behind TLS (D11/D18 leave this to deployment; the Hertz server does not terminate TLS in v1). |
| V7 Logging | yes | Audit logs themselves ARE the security artifact (GEVL-03). The retention projection preserves the meta-record (timestamp/session/skill/refusal_reason/metrics) while masking content — this is the compliance-audit crux. Do NOT log `X-Admin-Token` values. |

### Known Threat Patterns for this stack (Go + Hertz + SQLite + GitHub Actions)

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via `/api/audit` params | Tampering | Reuse `store.QueryAudit` (100% `?`-parameterized, T-3-SQLI). NEVER build SQL in the handler. Existing regression test (`DROP TABLE` survives) must stay green. `[VERIFIED: internal/store/sqlite/audit.go]` |
| Path traversal via `/api/eval/reports/:ts` | Tampering / Information disclosure | Strict-regex-validate `:ts` before `os.ReadFile`. Reject any path containing `/`, `\`, or `..`. Confine reads to `eval/reports/`. |
| IDOR on `/api/audit` (any user reads any user's queries) | Information disclosure | Binary admin token gates raw rows; non-admin gets only retention-redacted rows. v2 RBAC for finer control. |
| Audit tampering | Tampering / Repudiation | T-3-AUDIT INSERT-only invariant (no UPDATE/DELETE path). Retention redaction is READ-TIME ONLY — the stored row is immutable. Adding any write path in the store for retention is the anti-pattern (Pitfall 4). |
| Secret exfiltration via CI logs | Information disclosure | `GLM_API_KEY` as GitHub Actions secret (referenced via `${{ secrets.GLM_API_KEY }}`); never `echo`d; the eval CLI never writes it to report/calibration files (verified). `pull_request` trigger only (D6-09) — secrets are NOT available to forked-PR workflows by GitHub default (the safe path). |
| Token timing attack | Spoofing | `subtle.ConstantTimeCompare` for `X-Admin-Token` (NOT `==`). |
| Audit-row content leakage in error messages | Information disclosure | Handler errors return generic messages; the retention projection runs BEFORE any error serialization. |

## Sources

### Primary (HIGH confidence — codebase + official docs)
- **Live codebase reads** (the authoritative source for a hardening phase):
  - `internal/eval/{metrics,judge,dataset,calibration,chatmodel,eval_test}.go` — the shipped evaluator (4 metrics, Judge/StubJudge, LoadGolden, Calibrate, TestRunnerReproducibility @ 1e-9, NewGLMChatModel/JUDGE_MODEL slot). `[VERIFIED: Read tool]`
  - `cmd/eval/main.go` — the CLI flag set (`--golden`, `--report`, `--calibration`, `--limit`, `--topk`) + bootstrap + per-case flow. Phase 6 extends these flags. `[VERIFIED: Read tool]`
  - `internal/store/store.go` + `internal/store/sqlite/audit.go` — `AuditRow` schema, `AuditFilter` (parameterized), `QueryAudit` (100% `?`), `UpdateAuditMetrics` (COALESCE). `[VERIFIED: Read tool]`
  - `internal/server/server.go` + `skills.go` + `chat.go` — Hertz surface, `Deps` DI container, `GET /api/skills` shape (`item.manifest.*`), `computePromptHash` (skillName+modelVersion only). `[VERIFIED: Read tool]`
  - `internal/server/chat_session_test.go` — the 2-session isolation test SC-4 scales. `[VERIFIED: Read tool]`
  - `eval/README.md` + `eval/NOTICE` — operational doc + Ragas Apache-2.0 attribution + Pitfall 8 counter-measures. `[VERIFIED: Read tool]`
  - `internal/eval/testdata/frozen_5.jsonl` — the frozen-set seed schema. `[VERIFIED: Read tool]`
  - `web/src/store/app-store.ts` — Zustand store (D14 no-`messages`); `web/src/App.tsx` (single-route composition root); `web/package.json` (ai v7, zustand v5, lucide-react, NO router/chart lib). `[VERIFIED: Read tool]`
  - `go.mod` (Go 1.25.0, Eino v0.7.7, Hertz v0.10.6, testify v1.11.1). `[VERIFIED: Read tool]`
  - `.planning/config.json` (`nyquist_validation: true`). `[VERIFIED: Read tool]`
- **Ragas official docs** (metric semantics): `docs.ragas.io/en/stable/concepts/metrics/available_metrics/{faithfulness,context_precision}/` — confirms "citation precision" = `context_precision`. `[CITED]`
- **npm registry** (package verification): `npm view recharts` (3.10.1, peers incl. React 19, no postinstall, repo recharts/recharts); `npm view react-router` (8.3.0); `npm view react-router-dom` (7.18.2). `[VERIFIED: Bash]`
- **Project skills**: `.claude/skills/spike-findings-ai-agent/SKILL.md` (semantic-determinism philosophy, structured-output determinism) + `.claude/skills/sketch-findings-ai-agent/SKILL.md` (three-panel layout 260/flex/420, dark denova-inspired theme, status colors compliant/needs-info/non-compliant/unanswered — reused for the dashboard's ❌/✅ badges). `[VERIFIED: Read tool]`

### Secondary (MEDIUM confidence)
- `.planning/phases/06-eval-dashboard-production-hardening/06-CONTEXT.md` — the 16 locked decisions + 8 discretion items (authoritative). `[VERIFIED: Read tool]`
- `.planning/STATE.md` — D6 reversal (Go-native), D18/D19 (store + eval), open question #7 (retention/redaction → resolved by D6-06/D6-08). `[VERIFIED: Read tool]`
- WebSearch on Ragas metrics (docs.ragas.io + confident-ai.com + vectara.com) — cross-verified the 4-metric standard + context_precision semantics. `[CITED]`

### Tertiary (LOW confidence — flagged for validation)
- None material. The two `[ASSUMED]`-tagged package recommendations (recharts, react-router) are `npm view`-verified to exist with correct peers; the `[ASSUMED]` tag is procedural (slopcheck unavailable), not evidential.

## Metadata

**Confidence breakdown:**
- Standard stack (Go side): HIGH — all verified present in `go.mod`; Phase 6 adds nothing.
- Standard stack (FE side): HIGH-MEDIUM — recharts/react-router verified on npm + React 19 peers; MEDIUM only because the planner picks the dep vs anti-dep option.
- Architecture: HIGH — maps directly onto shipped Phase-3 interfaces (`store.Store`, `eval.Report`, `Deps`).
- Pitfalls: HIGH — derived from reading the actual code (prompt_hash comment, AuditFilter parameterization, INSERT-only invariant, TestRunnerReproducibility tolerance).
- SC-4/SC-6 specifics: HIGH-MEDIUM — the test scale-up + tolerance relaxation are mechanical; the live-jitter layer's `1e-3` is a planner-tunable.
- prompt_hash semantics (Pitfall 1 / Q1): HIGH that the gap exists (read the code + comment); the RESOLUTION is a planner decision (presented both options).

**Research date:** 2026-08-13
**Valid until:** 2026-09-13 (30 days — stable; the codebase substrate is shipped + the SC list is locked in CONTEXT.md)
