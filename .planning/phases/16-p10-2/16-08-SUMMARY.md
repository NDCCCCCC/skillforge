# Plan 16-08 Execution Summary

**Plan:** 16-08 (Wave 4) — SSE data-reference emission wiring + KPI pipeline assembly + admin read-only endpoint
**Status:** COMPLETE (hermetic + WSL-verified; no live stack required)
**Executed:** 2026-08-27

## One-liner

The KPI aggregation channel is now assembled end-to-end: `kpis/` loads at startup (fail-closed), a data-channel Skill's `kpi_call` reaches a real Executor, every successful snapshot publishes a `kpi_reference` data-decision carrying its `DataRef` citation, a KPI hop is never mislabelled `retrieve`, and operators have a retrieval-only `GET /api/admin/kpis` view of catalog + signature + last-invocation times.

## Tasks Completed

| Task | Description | Commit | Status |
|------|-------------|--------|--------|
| Task 1 | `callbacks.go` Tool OnStart tool-name dispatch (kpi_call ⊥ DecisionRetrieve) + `TestCallbackToolDispatch` 4 faces | `0cc1ad3` | PASS |
| Task 2 | `cmd/server/main.go` KPI pipeline assembly (env + loader + executor + adapters) + `internal/server` DataRefEmit wiring + Deps KPI fields | `1f1aed4` | PASS |
| Task 3 | `internal/server/kpi_admin.go` GET /api/admin/kpis + route + `TestAdminKPIs` 4 faces | `c94e39d` | PASS |

## Files Changed

### Production

- `internal/agent/callbacks.go` (MODIFIED) — `toolHandler.OnStart` dispatches on tool identity: the hop counter advances for every tool, but `DecisionRetrieve` is emitted only for non-`kpi_call` tools. New `isKPICallTool(info, input)` helper. Decision-constant godoc rewritten for the two-channel era (why the callback layer cannot carry `DataRef`: `DecisionEmitFunc` is a 5-scalar signature and widening it would couple `internal/agent` to `internal/wire`).
- `internal/server/runner_setup.go` (MODIFIED) — `ChatHandler` passes `KPIExecutor` / `KPIRegistry` / `DataRefEmit` into `agent.BuildRunner`. New `(*Deps).dataRefEmitter(c)` builds the per-request closure that publishes `wire.DecisionKPIReference` + `DataRef` through `runnerEmitDecision` (the existing `data-decision` envelope — D16 overload, no new chunk type).
- `internal/server/server.go` (MODIFIED) — `Deps.KPIExecutor` + `Deps.KPIRegistry`; route registration for `GET /api/admin/kpis` under a Phase 16 comment block.
- `internal/server/kpi_admin.go` (NEW) — `kpiAdminHandler` + `kpiAdminResponse` / `kpiCatalogEntry` envelopes + `kpiPipelineNotWiredError`. Admin gate → 403; nil registry → 503; audit read failure → 500; otherwise 200 with `{signature, kpis[], latest_calls{}}`.
- `cmd/server/main.go` (MODIFIED) — `SKILLFORGE_KPIS_DIR` env (default `kpis`), assembly section 6c-3 (dedicated `:memory:` DuckDB connector + `AttachLRU` + `NewDuckDBSQLSerializer` + `LoadKPIs` + `NewExecutor`), `storeDatasetLister` / `storeDatasetMeta` adapters, `shortSignature` log helper, Deps wiring, startup route log line.

### Tests

- `internal/agent/callbacks_test.go` (NEW) — `TestCallbackToolDispatch`: (a) kpi_call OnStart emits zero decisions while the hop counter still advances; (b) retrieve OnStart still emits exactly one `DecisionRetrieve` carrying the parsed query (Phase 3 regression); (c) mixed 3-hop timeline yields exactly one decision at step 2 and never a `kpi_reference` from this layer; (d) nameless-`RunInfo` shape-probe fallback routes `kpiCallArgs` to the kpi_call branch while retrieve-shaped args stay conservative.
- `internal/server/kpi_admin_test.go` (NEW) — `TestAdminKPIs`: 403 (missing / wrong / env-unset), 200 with registry signature + both KPI names + dataset + RFC3339 `latest_calls` from two real `kpi_calls` rows in an in-memory sqlite store, 503 on nil registry, and the structural retrieval-only lock (zero mutation verbs in `kpi_admin.go`; exactly one `/api/admin/kpis` route binding and it is `GET`).

## Plan-Requested Findings

### RunInfo tool-name availability (measured, not assumed)

`callbacks.RunInfo.Name` **does** carry the tool name. Verified in the module cache: `eino@v0.9.14/compose/tool_node.go:896-899` (`runToolCallTaskByInvoke`) and `:935-939` (`runToolCallTaskByStream`) both build `&callbacks.RunInfo{Name: task.name, Type: task.meta.componentImplType, Component: task.meta.component}` before invoking handlers. So Level 1 of the dispatch is authoritative and the argument-shape probe is a genuine fallback, not the primary mechanism.

Two secondary facts that shaped the test: `RunInfo` is a type alias for the internal struct (`callbacks/interface.go:41`), and `utils/callbacks`'s template routes `OnStart` on `info.Component` (`template.go:208-209`), so a test must set **both** `Name` and `Component: components.ComponentOfTool` to reach `toolHandler.OnStart` at all.

### `DecisionPayload.Step` for data-reference parts —取舍

`Step: 0`. The hop counter is `internal/agent`'s per-Run `stepCounter`, unexported and not observable from the server boundary where the emitter closure is built; there is no seam that hands it to the tool closure. The FE consumes data-reference parts by decision **kind** (`kpi_reference` → data-citation chip), not by hop ordinal, so an honest zero costs nothing while a fabricated ordinal would be a small lie in the audit-adjacent stream. Documented inline on `dataRefEmitter`. If a future phase wants real hop numbers, the clean route is to thread the counter into `DataRefEmitFunc`'s signature rather than to guess at the server.

## Architectural Decisions

1. **KPI pipeline is wired unconditionally, unlike the importer.** `SKILLFORGE_DATA_SALT` is the L1 HMAC site secret for the *write* side; KPI calls only *read* already-desensitized datasets and never re-derive a hash. Gating analytics on the salt would disable a perfectly valid read-only deployment. The difference and its rationale are commented at the assembly site.
2. **`LoadKPIs` failure is `log.Fatalf`; an empty directory is the one legal degraded state.** Same posture as Skill Scan (`main.go:201`): controlled files are load-bearing (D16-A1), so a malformed `.yaml` means the service does not start. Zero definitions loads an empty registry and the `[OK]` line says so explicitly (`signature=(empty registry — no KPI definitions loaded)`).
3. **Dispatch is conservative on undecidable input.** No `RunInfo` name + unparseable/foreign arguments → treated as *not* kpi_call, i.e. emit `retrieve` (Phase 3 behavior). An extra retrieve decision on an unknown tool is a cosmetic timeline artifact; blanking decisions for every tool would blank the observability surface.
4. **The admin endpoint is retrieval-only by structure, not by convention.** KPI definitions load once from controlled files, so any HTTP mutation surface would be exactly the registration backdoor the design exists to prevent (Pitfall 14). The lock is two source-greps in `TestAdminKPIs`: zero mutation verbs in the handler file, and the route table binds `/api/admin/kpis` once, for `GET`.
5. **Audit-read failure is 500, not an empty map.** An empty `latest_calls` reads as "never called" to an operator — a materially wrong triage signal. A nil `Store` (degraded mode, consistent with the rest of the package) still yields an empty map; a real error surfaces.

## Deviations from Plan

**1. [Rule 1 — simplification] `storeDatasetMeta` computes "latest" in Go over `ListDatasets` instead of adding an `ORDER BY created_at DESC LIMIT 1` store method.**
- The plan sketched a `getLatestBatchSQL`-shaped store query. Adding a store method + SQL + its tests for a bounded registry table exceeded the plan's blast radius, and `store.ListDatasets` already returns every field the adapter needs (`Dataset`, `Period`, `FilePath`, `KMinOverride`, `CreatedAt`).
- The adapter filters by logical `Dataset` and picks `max(CreatedAt)` — the same row `ORDER BY created_at DESC LIMIT 1` would return. Zero rows → empty `Period`, which the executor turns into a `dataset_not_available` refusal (never a fabricated `as_of`). Documented in the adapter godoc.
- Files: `cmd/server/main.go`. Commit `1f1aed4`.

**2. [Documented note] Task 1's godoc target line moved.** The plan pointed at `callbacks.go:44-46` ("retrieve — Tool OnStart fired"). That text lives in the decision-constant block, which was rewritten wholesale (two-channel dispatch rules + the `DecisionEmitFunc`-cannot-carry-`DataRef` note the plan's reminder ③ asked for) rather than patched in place.

**3. [Documented note] `DataRefEmitFunc` return is fire-and-forget.** The signature (`func(wire.DataReference)`) has no error return, so emission failures are logged as `[WARN]` at the emitter and not propagated — the SSE stream is already in flight and the authoritative copy of the same data is the `kpi_calls` audit row (KPI-02).

## Verification

| Verification | Command | Result |
|--------------|---------|--------|
| Task 1 dispatch + neighbours | `go test ./internal/agent/ -run 'TestCallbackToolDispatch\|TestNoSQLGenerationSurface\|TestToolsForChannel' -count=1` | PASS |
| Full agent tree (Windows hermetic) | `go test ./internal/agent/... -count=1` | PASS (4 packages) |
| Internal build (Windows) | `go build ./internal/...` + `go vet ./internal/server/` | PASS |
| CGO binary build (WSL) | `go build ./...` | PASS |
| vet (WSL) | `go vet ./internal/server/ ./cmd/server/` | PASS |
| Task 3 endpoint | `go test ./internal/server/ -run TestAdminKPIs -count=1 -v` (WSL) | PASS 4/4 |
| Server tree regression (WSL) | `go test ./internal/server/... -count=1` | PASS (server 12.75s + classifier + rewrite) |
| Full repo (WSL) | `go test ./... -count=1` | 1 pre-existing FAIL package — see below |
| GET-only grep | `grep -cE "POST\|PUT\|DELETE\|PATCH" internal/server/kpi_admin.go` | 0 |

### Pre-existing failure (NOT introduced by 16-08)

`internal/kpi/wsl` fails 2 tests: `TestKPIEngineIntegration` (fixture YAML trips the D16-A2 bidirectional `:param`↔`params` coverage check — `params has SQL missing: [status]`) and `TestNameLexiconParity/kpi_vs_duckdb_lexicon_agreement` (64-char cap disagreement between the kpi and duckdb identifier lexicons).

Confirmed pre-existing by re-running that package with `internal/kpi` + `internal/agent` checked out at the 16-07 baseline commit `dc69291`: **same FAIL**. Plan 16-08 touches neither package. Recorded as inherited debt for the wave/phase gate — likely a 16-05/16-06 fixture-vs-validator drift (the fixture writes `:status` inside a `WHERE (:status IS NULL OR …)` clause shape the Layer 1 extractor no longer credits) plus a lexicon boundary off-by-one. Both are hermetic-fixable and belong to whoever owns the 16-06 WSL triple lock, not to this plan's blast radius.

`-race` full-suite run was not attempted: it is the wave-merge gate in the plan's verification block, and running it would not change the inherited-failure disposition above.

## Threat-Model Coverage

| Threat | Mitigation Applied | Lock |
|--------|-------------------|------|
| T-16-08-01 admin endpoint over-reach | `isAdminRequest` constant-time token gate; unset env denies everyone | `TestAdminKPIs/no_admin_token` (3 assertions) |
| T-16-08-02 admin endpoint grows a write face | Retrieval-only handler + single `GET` route + double source-grep | `TestAdminKPIs/retrieval_only` |
| T-16-08-03 malformed KPI file degrades silently | `LoadKPIs` error → `log.Fatalf`; empty directory is the only legal degraded state and is logged as such | assembly section 6c-3 |
| T-16-08-04 tool mislabelling pollutes the timeline | Tool-name dispatch; kpi_call emits nothing from the callback layer | `TestCallbackToolDispatch` (4 sub-tests) |
| T-16-08-05 half-run data Skill | Two layers: `BuildRunner` fail-closed on nil KPI deps (16-07) + admin 503 on nil registry | `TestAdminKPIs/registry_not_wired` + `tools_for_channel_test.go` |
| T-16-08-SC supply chain | Zero new modules | `go.mod` untouched |

## Environment Limits (D15-01-1)

`cmd/server` has been a CGO build since the Phase 15 importer assembly; MSYS2 gcc on this Windows host links sqlite but not duckdb. All `cmd/server` builds and all `internal/server` test runs for this plan were therefore executed in WSL2 Ubuntu-22.04. The constraint is pre-existing Phase 15 status quo, restated in the assembly-site comment so a future reader does not attribute it to the KPI pipeline. Task 1 (`internal/agent`) is fully Windows-hermetic.

## Hand-off Notes

- **Phase 17 (chat number Q&A)** now has its runtime floor: `data` channel Skill → `kpi_call` → executor → `kpi_reference` SSE part with `DataRef`. The FE consumes it off the existing `data-decision` discriminator; no new chunk type to register.
- **Phase 20 ADM-03** can build on `GET /api/admin/kpis` directly — the catalog + signature + `latest_calls` composition is the probe the plan's Open Q3 wanted, and `store.ListKPICalls` is already available for the per-call drill-down.
- **`internal/kpi/wsl` inherited failures** should be closed before the phase gate; see the pre-existing-failure section for the two root-cause leads.
- **If real hop numbers are ever needed on data-reference parts**, thread the step through `DataRefEmitFunc` rather than reconstructing it at the server (see the Step 取舍 above).

## Self-Check

| Item | Status |
|------|--------|
| callbacks.go dispatches on KPICallToolName; kpi_call emits no retrieve | PASS |
| Decision godoc carries the two-channel dispatch narrative | PASS |
| TestCallbackToolDispatch green (4 sub-tests) | PASS |
| `go test ./internal/agent/... -count=1` zero regressions | PASS |
| main.go contains `SKILLFORGE_KPIS_DIR` + `LoadKPIs` + `log.Fatalf` + `[OK] … KPI pipeline wired` | PASS |
| main.go comments the salt-independence and the Windows/WSL CGO reality | PASS |
| BuildRunner call site injects KPIExecutor / KPIRegistry / DataRefEmit | PASS |
| DataRefEmit closure emits `wire.DecisionKPIReference` via runnerEmitDecision | PASS |
| kpi_admin.go: admin gate + 403 + Signature() + LatestKPICallByFunc | PASS |
| `grep -cE "POST\|PUT\|DELETE\|PATCH" kpi_admin.go` == 0 | PASS |
| server.go registers /api/admin/kpis for GET only | PASS |
| TestAdminKPIs green (4 sub-tests) | PASS |
| `go build ./...` (WSL) exit 0 | PASS |
| Server suite zero regressions (WSL) | PASS |
| 3 commits, one per task | PASS (`0cc1ad3`, `1f1aed4`, `c94e39d`) |
