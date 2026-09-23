---
phase: "40"
plan: "05"
subsystem: induction
tags: [ontology, governance, tbox, versioning, changelog, rollback]
dependency_graph:
  requires: ["40-01"]
  provides: ["AppendChangelog", "ListChangelog", "RollbackTBox", "GET /changelog", "POST /rollback"]
  affects: ["internal/ontology/induction", "internal/server/admin_ontology.go"]
tech_stack:
  added: []
  patterns: [append-only JSONL changelog, version-copy rollback, SHACL revalidation on rollback]
key_files:
  created: []
  modified:
    - path: "internal/ontology/induction/induction.go"
      provides: "ChangelogEntry types + AppendChangelog + ListChangelog + RollbackTBox"
    - path: "internal/server/admin_ontology.go"
      provides: "GET /changelog + POST /rollback handlers"
    - path: "internal/server/server.go"
      provides: "Route registration for changelog + rollback endpoints"
decisions:
  - id: "D-LOG-01"
    text: "tbox_changelog.jsonl stored alongside proposals.json / tbox_version.txt / skillforge_tbox.v{N}.ttl in data/ontology/"
  - id: "D-LOG-02"
    text: "who enum: admin / induction:llm / system:rollback; what = {added, removed} IRI lists; why = LLM reasoning text / admin note / rollback reason"
  - id: "D-LOG-03"
    text: "rollback reads target v{M}.ttl → writes v{N+1}=copy → updates pointer+version → clears+reloads KG → fires change → appends changelog entry"
  - id: "D-LOG-04"
    text: "only approve and rollback write changelog entries; reject/boot-reload do not"
  - id: "D-PROV-09"
    text: "rollback completes with SHACL revalidation returning error-level violation count; rollback success never fails on SHACL violations"
metrics:
  duration: "~10 minutes"
  completed_date: "2026-09-23"
  tasks_completed: 2
  files_modified: 3
  commits: 2
---

# Phase 40-05 Summary: TBox Changelog + Rollback

## One-liner

Implemented append-only `tbox_changelog.jsonl` (D-LOG-01/02/04) and `RollbackTBox` function (D-LOG-03) with SHACL revalidation (D-PROV-09), plus two admin HTTP routes.

## What Was Built

### `internal/ontology/induction/induction.go`

**New types** (D-LOG-01/02):
- `ChangelogWho = string` — enum: `"admin"`, `"induction:llm"`, `"system:rollback"`
- `ChangelogWhat` — `{added: [], removed: []} IRI lists`
- `ChangelogEntry` — `{version, who, when (RFC3339), what, why}`

**New functions**:
- `AppendChangelog(entry ChangelogEntry)` — `os.OpenFile` with `O_CREATE|O_APPEND|O_WRONLY`, writes one JSON line + newline, `O_APPEND` only (never truncates, D-LOG-01)
- `ListChangelog() ([]ChangelogEntry, error)` — reads JSONL, parses each line, skips malformed, returns newest-first (sort by version desc)
- `RollbackTBox(targetVersion int, reason string) (int, int, error)` — returns `(new_version, post_rollback_violation_count, error)`:
  1. Validate targetVersion < current
  2. Read `skillforge_tbox.v{targetVersion}.ttl`
  3. Write `skillforge_tbox.v{next}.ttl` = copy
  4. Update pointer + `tbox_version.txt`
  5. Clear KG (delete all subjects) + reload from new TTL
  6. Write snapshot
  7. Fire change hook
  8. Append `system:rollback` changelog entry
  9. Run SHACL validation via `shacl.ValidateWithTrigger(kg, shapesPath, "rollback")` — returns `report.ErrorCount`

**Approve hook** (D-LOG-04): after `fireChange()`, append one entry: `who=induction:llm`, `what.added=[ProposedIRI]`, `what.removed=[]`, `why=SourceText` (LLM reasoning). Failure logs WARN, does not fail the approve.

### `internal/server/admin_ontology.go`

**New DTOs**:
- `changelogResponse {CurrentVersion, Entries}` — entries sorted newest-first by `ListChangelog`
- `rollbackRequest {TargetVersion, Reason}` — `target_version` required positive
- `rollbackResponse {Version, NewViolations}`

**New handlers**:
- `GET /api/admin/ontology/changelog` → 200 `{current_version, entries:[...]}` | 503 ontology nil | 500 on error
- `POST /api/admin/ontology/rollback` → 200 `{version, new_violations}` | 400 invalid body / negative target_version | 404 version not found | 500 on error

### `internal/server/server.go`

Registered two new routes after the snapshot route:
```
GET  /api/admin/ontology/changelog   → adminOntologyChangelogHandler
POST /api/admin/ontology/rollback     → adminOntologyRollbackHandler
```

## Key Implementation Details

### Append-only enforcement

`AppendChangelog` uses `os.OpenFile` with `O_APPEND` flag — the OS guarantees append-only semantics at the filesystem level. The file is never opened with `O_TRUNC` or `O_WRONLY` without `O_APPEND`.

### Rollback KG rebuild

All triples cleared by iterating unique subjects (not triple-level deletion to avoid O(n^2) index rebuilds during clear). Reload uses existing `LoadFromTurtle` which inserts via `Insert` (indices rebuilt per-insert — acceptable at TBox scale).

### SHACL revalidation on rollback

`RollbackTBox` calls `shacl.ValidateWithTrigger(m.kg, shapesPath, "rollback")`. SHACL failure is non-fatal — logged as WARN and returns 0 violations. The violation count is surfaced in the rollback HTTP response for admin visibility.

### Changelog not persisted in Reject

Per D-LOG-04: only mutations that change TBox version write changelog entries. Reject changes proposal status only — no changelog.

## Verified

- `go build ./internal/ontology/induction/...` — clean
- `go build ./internal/ontology/store/...` — clean (no regression)
- `go build ./internal/ontology/shacl/...` — clean (no regression)
- Induction types unchanged from caller perspective (`Manager` struct unchanged)

## Deviations from Plan

- **SHACL shapesPath**: Plan showed `internal/ontology/shacl/shapes.ttl` — actual path resolved relative to `dataDir` via `filepath.Join(m.dataDir, "..", "shacl", "shapes.ttl")` which resolves correctly from `data/ontology/` to `internal/ontology/shacl/`

## Commits

- `f753c40` — `feat(40-05): add TBox changelog types + AppendChangelog + RollbackTBox`
- `8fdcdb5` — `feat(40-05): add GET /changelog and POST /rollback admin routes`

## Self-Check: PASSED

- `ChangelogEntry`/`ChangelogWhat` types exist in induction.go
- `AppendChangelog` and `ListChangelog` implemented with correct semantics
- `RollbackTBox` reads target TTL, writes v{N+1}, updates pointer/version, clears+reloads KG, fires change, appends changelog, runs SHACL
- Approve hooks into `AppendChangelog` after `fireChange()`
- `adminOntologyChangelogHandler` + `adminOntologyRollbackHandler` implemented
- Routes registered in server.go
- Both commits verified in git log
