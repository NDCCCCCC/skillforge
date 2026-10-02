# kpis/ — KPI definition files (Phase 16, P10.2)

KPI (Key Performance Indicator) definitions are YAML files authored by data
staff, versioned in git, and loaded fail-closed at service startup. The agent
calls KPIs by name via a single `kpi_call` tool — it never sees or generates
SQL (DATA-09 / KPI-01).

## Field reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `dataset` | string | yes | Logical dataset name (must match datasets registry; no `__`) |
| `description` | string | yes | Human-readable summary of this file's KPIs |
| `k_min` | int | yes | Per-file k_min default (0 = defer to dataset/global) |
| `kpis` | list | yes | One or more KPI definitions (see below) |
| `kpis[].name` | string | yes | KPI identifier (`^[A-Za-z][A-Za-z0-9_]{0,63}$`) |
| `kpis[].description` | string | yes | Human-readable explanation (shown in tool catalog) |
| `kpis[].sql` | string | yes | SELECT template with `:param` placeholders |
| `kpis[].count_column` | string | yes | SELECT alias holding aggregate count (for k_min) |
| `kpis[].max_rows` | int | no | Result set size limit (0 = use package default) |
| `kpis[].params` | map | no | Named parameter declarations |
| `kpis[].params.<name>.type` | string | yes | `string` or `enum` (vocabulary default-deny) |
| `kpis[].params.<name>.values` | list | enum only | Enum value allowlist (also differential whitelist, D16-D2) |
| `kpis[].params.<name>.pattern` | string | string only | Regex constraint (must compile at load time) |
| `kpis[].params.<name>.required` | bool | no | Whether the parameter must be provided (default false) |

## Fail-closed semantics

Any single file failing validation prevents the entire directory from loading.
The server refuses to start with invalid KPI files — partial state never occurs
(D16-A1). Errors name the failing field, rule, and decision anchor.

## v1: no period parameter

KPI SQL references `ds_{dataset}.data` without a period filter. The system
automatically selects the latest period from the datasets registry (D16-B4).
Historical replay uses `kpi_calls` audit rows, not re-querying (KPI-02).

## Changing a KPI = changing a file = going through git review

There is no registration API, admin write endpoint, or runtime mutation path
(KPI-01). The YAML file is the single source of truth.
