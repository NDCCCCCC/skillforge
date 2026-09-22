---
phase: 39-ecl-pipeline
plan: 06
subsystem: boot-pipeline
tags: [ecl, indexer, routing, purge, tbox, upgrade, kg-restore, boot, env-gate, milvus]

# Dependency graph
requires:
  - phase: 39-01
    provides: store DeleteSubject/InsertUnique, emitter 8-class TBox + NoteChunk sentinel (the upgrade trigger string), SanitizeLocal
  - phase: 39-02
    provides: milvus DeleteByExpr (caller-built expr, verbatim) + GetMaxIDCollection collection-wide watermark; retriever.Document +4 fields
  - phase: 39-03
    provides: ecl.IsSecFragment routing predicate (secRegex AND non-empty section frontmatter)
  - phase: 39-04
    provides: ecl.NewLLMExtractor(cm) frozen cross-plan contract
  - phase: 39-05
    provides: ecl.NewManager(ns, kbDir, kg, ex, fs, seedFn) + SetOnChange/SetIDAllocator/CASRunning, sqlite ecl_failures FailureStore, BuildDocuments
provides:
  - "knowledge.SetECLMode + Index routing branch (__secNN+section skip placed BEFORE maxDocBytes — ordering regression-guarded behavioral + static) and marker-scoped seedDocs purge (sql-guides legacy rows only via section_count == 0)"
  - "emitter.UpgradeIfNeeded: 3-state boot reconcile (fresh v1 / strict no-op on sentinel / one-time 6→8 v{N+1} with approved-induction triple replay)"
  - "induction.ApprovedTriples: single-owner reader of the approved proposal queue"
  - "store.Restore: D-KG-02 snapshot-first KG restore with InsertUnique dedupe merge; TTL-count return for boot-log continuity"
  - "Boot wiring: ECL manager armed (cm + KG + sqlite FailureStore + milvus SeedFunc closure with section_count > 0 purge + watermark ID allocator), shared kgDebounce, deps.ECL field — never auto-run (D-Run-01)"
affects: [39-07 (admin routes bind deps.ECL + CASRunning/RetryNote), 39-08 (FE consumes status contract), 39-09 (verify script)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-mode flag with exported setter (SetECLMode mirrors indexSeedHook seam pattern) — set once at boot before the boot-time Index, toggled by tests"
    - "Marker-scoped purge decision factored into a pure function (purgeExprFor) table-tested in the indexer suite"
    - "3-state upgrade reconcile keyed on a pinned byte sentinel (skf:NoteChunk a owl:Class) — never re-emit on up-to-date state"
    - "Snapshot-first restore + dedupe merge via existing InsertUnique (no new store API)"
    - "Static source-order guard test (ECL branch strictly before maxDocBytes in indexer.go source) mirroring TestIndexer_ProductionWiresValidateEffective"

key-files:
  created:
    - internal/ontology/emitter/upgrade.go
    - internal/ontology/emitter/upgrade_test.go
    - internal/ontology/store/restore.go
    - internal/ontology/store/restore_test.go
  modified:
    - internal/knowledge/indexer.go
    - internal/knowledge/indexer_test.go
    - internal/ontology/induction/induction.go
    - cmd/server/main.go
    - internal/server/server.go
    - .env.example

key-decisions:
  - "ECL cohabitation tier is the literal \"sql-guides\" (eclCohabitationTier const) — D-ECL-01 scopes ECL to sql-guides only, and seedTiers yields slugs with no signal for which the ECL pipeline claims"
  - "ECL-off seedDocs path kept byte-identical (ONE DeleteTierRows call over the whole batch scope); the per-tier loop exists only under eclMode — the rollback direction wipes stale ECL rows via the legacy tier purge (RESEARCH Pattern 3)"
  - "SetECLMode + gate parse moved BEFORE buildRetriever (plan text placed it in the ontology section, after boot Index had already run) — routing must govern the first seed, not just later indexes"
  - "deps.ECL field pre-added in server.Deps (Plan 07 owns server.go but runs later; the plan's deps.ECL wiring cannot compile without the field) — Plan 07 registers routes against this pre-added nil-posture field"
  - "Restore returns the TTL-loaded count (not store Len) so the boot log line keeps its pre-Phase-39 continuity semantics"
  - "Unreadable/corrupt snapshot ERRORS the restore instead of silently falling back to TTL-only — that fallback IS the ABox wipe the function exists to prevent"
  - "ECL SeedFunc purge expr is a compile-time constant (tier literal, no interpolation) — %q escaping duty only applies to the indexer's dynamic path (purgeExprFor)"

patterns-established:
  - "Boot reconcile pattern: helper next to the code it fixes (UpgradeIfNeeded in emitter, Restore in store) keeps main.go thin wiring"
  - "Ownership-marker cohabitation: two producers share one Milvus tier, each purging only its own generation (section_count == 0 vs > 0)"

requirements-completed: [ONT-10, ONT-11, ONT-13]

# Metrics
duration: 23 min
completed: 2026-09-22
---

# Phase 39 Plan 06: ECL Boot Integration (routing, purge, TBox upgrade, KG restore, wiring) Summary

**ECL pipeline wired into its two highest-risk seams: the indexer routing branch + marker-scoped sql-guides purge (Pitfall 1) and the boot sequence — one-time TBox 6→8 upgrade (Pitfall 4), snapshot-first KG restore with dedupe merge (Pitfall 3 / D-KG-02), and the armed-but-never-auto-run ECL manager behind SKILLFORGE_ECL_ENABLED (D-Run-01).**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-22T02:59:54Z
- **Completed:** 2026-09-22T03:23:22Z
- **Tasks:** 3
- **Files modified:** 10 (4 created, 6 modified)

## Accomplishments
- Indexer: `SetECLMode` flag + routing branch between splitFrontmatter and maxDocBytes (oversize `__secNN` fragments reach the ECL skip, not the size guard — behavioral contrast tests + static source-order guard); `seedDocs` ECL mode purges the sql-guides cohabiting tier via `tier in ["sql-guides"] and section_count == 0` (DeleteByExpr, %q-escaped — T-39-01) while every other tier and the ECL-off path keep DeleteTierRows byte-identical; decision factored into pure `purgeExprFor` (table-tested)
- `emitter.UpgradeIfNeeded`: fresh boot emits v1; a sentinel-bearing pointer is a strict no-op (content proven unchanged even with mutated skills/knowledge trees); a Phase-38 pointer triggers exactly one v{N+1} emit + pointer swap + version bump with `induction.ApprovedTriples` replay (approved proposals survive; rejected ones don't); atomic writes throughout; errors returned, caller owns Fatal posture
- `store.Restore`: snapshot-first LoadSnapshot, TBox TTL parsed into a temp store and folded in via InsertUnique (dedupe), TTL-only fallback on missing snapshot, corrupt snapshot errors loud (never the silent fallback that would itself wipe ABox)
- Boot (cmd/server/main.go): gate parsed per A3 (off|false|0 disable; unset = on) and `knowledge.SetECLMode` applied BEFORE buildRetriever; ontology middle replaced with UpgradeIfNeeded → Restore → boot WriteSnapshot of the MERGED store; `ecl.NewManager` armed with cm + KG + sqlite FailureStore + SeedFunc closure (marker purge `section_count > 0`, GetMaxIDCollection watermark allocator, boot-shared embedder), SetOnChange sharing the induction kgDebounce (D-Boot-01); mongo/none deployments get nil + WARN (A7/T-39-16); `.env.example` documents the RECREATE → Run ECL → unset deploy sequence (open-question resolution #2)

## Task Commits

Each task was committed atomically:

1. **Task 1: Indexer routing branch + marker-scoped purge mode** - `3d6675c` (feat)
2. **Task 2: TBox upgrade trigger + snapshot-first KG restore** - `be97844` (feat)
3. **Task 3: main.go boot wiring + env gate + .env.example** - `370a3f3` (feat)

## Files Created/Modified
- `internal/knowledge/indexer.go` — eclMode flag + SetECLMode, routing branch (load-bearing placement), purgeExprFor, seedDocs ECL/ECL-off purge swap
- `internal/knowledge/indexer_test.go` — routing on/off contrast, oversize ordering proof + static guard, purge decision table
- `internal/ontology/emitter/upgrade.go` — UpgradeIfNeeded 3-state reconcile + appendApprovedTriples + readVersion + writeFileAtomic
- `internal/ontology/emitter/upgrade_test.go` — 5 cases: fresh v1 (idempotent), sentinel no-op under source mutation, one-time v2 upgrade, approved-proposal replay (rejected excluded), pointer read-error
- `internal/ontology/store/restore.go` — Restore (D-KG-02 order)
- `internal/ontology/store/restore_test.go` — 5 cases: ABox survives, identical-TTL dedupe (repeat-stable), TTL extras once, missing-snapshot fallback, corrupt-snapshot error
- `internal/ontology/induction/induction.go` — ApprovedTriples(dataDir) exported helper (queue format single-owner)
- `cmd/server/main.go` — gate + SetECLMode before buildRetriever; ontology middle rewritten (upgrade → restore → merged snapshot); ECL manager wiring; deps.ECL
- `internal/server/server.go` — Deps.ECL *ecl.Manager field with nil-posture godoc (Rule 3, see deviations)
- `.env.example` — SKILLFORGE_ECL_ENABLED row + 3-step deploy sequence comment

## Decisions Made
- **eclCohabitationTier is a literal "sql-guides"** with a comment citing D-ECL-01's sql-guides-only scope — seedTiers yields slugs but carries no claim signal, and the plan explicitly sanctioned the literal.
- **ECL-off purge stays one whole-batch DeleteTierRows call** (byte-identical acceptance criterion) — the per-tier loop exists only under eclMode; the plan's "coherent both-directions" rollback property is documented at the branch.
- **Upgrade no-op proven by content equality across boots** (pointer bytes + version file) rather than mtime — mtime is flaky cross-platform; byte-identity is the stronger, deterministic assertion.
- **Restore's returned count = TTL triples loaded**, keeping the boot log line's meaning continuous with the pre-Phase-39 LoadFromTurtle line.
- **ID watermark failure degrades to WARN + allocation from 1** (documented PK-collision risk in the log) rather than failing boot — the ECL pipeline is admin-triggered, so the operator sees the warning before any run.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] server.Deps gained the ECL field in this plan (Plan 07's file)**
- **Found during:** Task 3 (deps literal wiring)
- **Issue:** The plan's action `deps.ECL = eclManager` and acceptance criterion "disabled → deps.ECL nil" reference a field that does not exist yet — internal/server/server.go is Plan 07's file and Plan 07 runs after this plan; without the field the boot wiring cannot compile, and Plan 07 (whose files exclude main.go) would have no wiring point for the manager it exposes
- **Fix:** Added `ECL *ecl.Manager` beside Deps.Ontology with the nil-posture godoc Plan 07's spec describes, plus the `ecl` import; main.go sets `ECL: eclManager` in the deps literal
- **Files modified:** internal/server/server.go
- **Verification:** `go build ./... && go vet ./internal/server/` clean; full server suite green
- **Committed in:** 370a3f3
- **Note for Plan 07's executor:** the field is pre-added — do NOT re-declare it; your Task 1 reduces to route registration + admin_pipeline.go handlers against `s.ECL`.

**2. [Rule 1 - Bug] SetECLMode / gate parse placed before buildRetriever, not in the ontology section**
- **Found during:** Task 3 (wiring order review)
- **Issue:** The plan placed `knowledge.SetECLMode(eclEnabled)` in the ontology section (after kgDebounce) — but the boot-time `knowledge.Index` runs inside buildRetriever earlier in main(); the routing branch and marker purge would only govern SUBSEQUENT indexes, so the first boot would legacy-index all 185 fragments, exactly the double-generation state D-ECL-01 exists to prevent
- **Fix:** Gate parsed and `knowledge.SetECLMode(eclEnabled)` applied immediately before the buildRetriever step, with a comment explaining the ordering; manager construction stays in the ontology section per plan
- **Files modified:** cmd/server/main.go
- **Verification:** boot-order review; `go build ./...` clean; full suite green
- **Committed in:** 370a3f3

**3. [Test augmentation] Ordering proof (Task 1 test b) carries a static source-order guard**
- **Found during:** Task 1 (test design)
- **Issue:** An oversize `__secNN` file is observably excluded under BOTH branch positions (guard-first and branch-first produce the same hook outcome), so the behavioral test alone cannot prove "branch precedes the guard"
- **Fix:** Kept the plan's behavioral assertions (oversize excluded with ECL on; small fragment indexed with ECL off — proving the branch owns the exclusion) and added a static guard asserting `ecl.IsSecFragment(` appears before `len(body) > maxDocBytes` in indexer.go source, mirroring the existing TestIndexer_ProductionWiresValidateEffective pattern
- **Files modified:** internal/knowledge/indexer_test.go
- **Verification:** TestIndexer_ECLBranchPrecedesMaxDocBytes green
- **Committed in:** 3d6675c

**Total deviations:** 3 auto-fixed (1 blocking, 1 bug, 1 test augmentation)
**Impact on plan:** All three are correctness/integration necessities; no scope creep. The server.go field is a deliberate early slice of Plan 07's declared surface, coordinated via this note.

## Issues Encountered
- One transient `go test ./internal/knowledge/` FAIL on the first post-Task-1 run that never reproduced (3 consecutive greens + targeted `-v` reruns of every touched test green; full `./...` suite green twice afterwards). No action taken.

## User Setup Required
None - no external service configuration required (`.env.example` documents the optional one-time Phase 39 deploy sequence: `SKILLFORGE_MILVUS_RECREATE=1` boot → admin Run ECL → unset).

## Next Phase Readiness
- Plan 07 binds: `deps.ECL` field already present (do not re-declare); register the 3 `/api/admin/pipeline/*` routes and handlers against `CASRunning()`/`Status()`/`RetryNote` exactly as its plan specifies
- Plan 08 (FE) and Plan 09 (verify script) are unaffected by these deviations
- No blockers; full `go test ./... -count=1` green (31 packages), `go build ./...` + `go vet` clean; boot grep-proof: zero `.Run(` calls in cmd/server/main.go

## Self-Check: PASSED

- All 4 created files exist on disk (`internal/ontology/emitter/upgrade.go`, `upgrade_test.go`, `internal/ontology/store/restore.go`, `restore_test.go`); 6 modified files committed
- Commits verified: 3d6675c, be97844, 370a3f3 in `git log`
- Plan verification re-run: `go build ./... && go vet ./cmd/server/` clean; `go test ./internal/knowledge/ ./internal/ontology/... -count=1` green; full `go test ./... -count=1` green
- Acceptance criteria re-verified per task: routing ordering (behavioral + static), purge expr exact string, upgrade 3-state table, restore dedupe arithmetic, boot grep-proof (`Run` absent), `.env.example` deploy sequence present

---
*Phase: 39-ecl-pipeline*
*Completed: 2026-09-22*
