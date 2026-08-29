# Plan 16-01 Execution Summary

**Plan:** 16-01 (Wave 1) — internal/kpi CGO-free skeleton + ValidateKPIFile + LoadKPIs + Registry
**Status:** COMPLETE
**Executed:** 2026-08-24

## Tasks Completed

| Task | Description | Commit | Status |
|------|-------------|--------|--------|
| Task 1 | CGO-free skeleton + WSL probe TestProbeJSONSerializeSQL (lock A1/A2/A3) | `b5f9173` | PASS |
| Task 2 | define.go YAML structs + validate_text.go Layer 1 fail-closed (9-rule set) | `f8f2630` | PASS |
| Task 3 | kpis/loans.yaml + loader.go LoadKPIs pipeline + registry.go immutable Registry | `ed9023b` | PASS |

## Files Changed

### Production (8 files)
- `internal/kpi/kpi.go` — Package doc (CGO-free constraint, Phase 16 positioning, controlled file discipline)
- `internal/kpi/define.go` — KPIFile/KPIDef/ParamSpec YAML structs + ParamTypeString/ParamTypeEnum vocabulary
- `internal/kpi/validate_text.go` — ValidateKPIFile 9-rule Layer 1 validator (hermetic)
- `internal/kpi/loader.go` — LoadKPIs(dir) five-stage pipeline + signature hash + :param→$param rewrite
- `internal/kpi/registry.go` — Immutable Registry (Lookup/Catalog/Names/Signature) + CompiledKPI/CompiledParam/CatalogEntry
- `internal/kpi/validate_sql.go` — Placeholder (16-05)
- `internal/kpi/executor.go` — Placeholder (16-06)
- `internal/kpi/kmin.go` — Placeholder (16-06)

### Test (4 hermetic + 1 WSL)
- `internal/kpi/validate_text_test.go` — 25 TestValidateKPIFile branches + TestParamSpecCompile + TestKPINamePattern
- `internal/kpi/loader_test.go` — 12 TestLoadKPIs scenarios (valid/bad/dupes/empty/symlink/signature/subdir/rewrite/params)
- `internal/kpi/registry_test.go` — TestKPIRegistryImmutableAtRuntime (reflect lock) + defensive copy + sorted + real YAML
- `internal/kpi/wsl/probe_test.go` — TestProbeJSONSerializeSQL (6 SQL probes, A1/A2/A3 lock)

### Controlled files
- `kpis/loans.yaml` — 2 KPI definitions (loan_overdue_rate_by_status + loan_balance_bucket_distribution)
- `kpis/README.md` — Field reference + fail-closed semantics + v1 no-period explanation

### WSL probe fixtures (6 files)
- `internal/kpi/wsl/testdata/probe-1.json` through `probe-6.json`

## WSL Probe Results (A1/A2/A3 Lock)

| Probe | SQL | Result | Locked Behavior |
|-------|-----|--------|----------------|
| 1 | Valid SELECT with ds_loans.data | `error:false`, `schema_name:"ds_loans"`, `table_name:"data"` | A3 confirmed: schema/catalog split form (schema_name + table_name, NOT a single compound identifier) |
| 2 | Invalid SQL | `error:true`, `error_type:"parser"` | A1 confirmed: error in envelope, not Go exception |
| 3 | INSERT INTO | `error:true`, `error_type:"not implemented"`, `"Only SELECT statements can be serialized to json!"` | A1 confirmed: non-SELECT returns error=true (stronger than assumed: it errors, not just non-SELECT_NODE) |
| 4 | Cross-alias JOIN (ds_loans + ds_cards) | `error:false`, `from_table.type:"JOIN"`, left=`schema_name:"ds_loans"`, right=`schema_name:"ds_cards"` | A3 confirmed: JOIN tree has left/right BASE_TABLE nodes with schema_name; walkable |
| 5 | SELECT 42 (empty FROM) | `error:false`, `from_table.type:"EMPTY"` | Zero table references: valid AST, validator must independently reject |
| 6 | Subquery in select list | Subquery appears as `class:"SUBQUERY"` with nested `from_table.schema_name:"ds_loans"` | Subquery table refs walkable via subquery.from_table recursion |

### Key discovery for 16-05 (validate_sql.go)
- Table references use **schema_name + table_name** (not a single compound "ds_loans.data" identifier)
- JOIN tree: `from_table.type:"JOIN"` with `left` and `right` sub-objects (each BASE_TABLE)
- Subqueries: `select_list[].class:"SUBQUERY"` with nested `subquery.node.from_table`
- Empty FROM: `from_table.type:"EMPTY"` — AST walker must handle this
- Non-SELECT: `error:true` with `error_type:"not implemented"` — stronger than assumed, both error path and non-SELECT_NODE path reject

## Verification Status

| Verification | Command | Result |
|-------------|---------|--------|
| Task 1 hermetic | `go build ./internal/kpi/... && go vet ./internal/kpi/...` | PASS |
| Task 1 WSL | `wsl ... go test ./internal/kpi/wsl/ -run TestProbeJSONSerializeSQL -count=1 -v` | PASS (6/6 probes green) |
| Task 2 hermetic | `go test ./internal/kpi/ -run 'TestValidateKPIFile\|TestParamSpecCompile' -count=1` | PASS (25 branches + 2 compile checks) |
| Task 3 hermetic | `go test ./internal/kpi/ -run 'TestLoadKPIs\|TestKPIRegistryImmutableAtRuntime\|TestRegistryCatalog' -count=1` | PASS (all scenarios) |

## Success Criteria Met

- [x] LoadKPIs compiles 2-KPI immutable registry from kpis/ with verifiable signature
- [x] Nine Layer 1 fail-closed branches + cross-file dupes + symlink + partial-load ban all hermetic-locked
- [x] A1/A2/A3 locked by probe (INSERT errors; JOIN tree walkable; schema_name/table_name split form)
- [x] TestKPIRegistryImmutableAtRuntime green (KPI-01 no-registration-backdoor structural lock first segment)
