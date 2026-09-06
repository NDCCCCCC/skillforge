---
phase: 26-kb-registry-foundation
plan: 03
subsystem: knowledge-retrieval
tags: [kb-registry, fail-closed, walker-convergence, know-02, d09, tdd, kb-m-05, kb-m-06]
requires:
  - "internal/kb Registry interface (26-01) — Lookup/List/ActiveSlugs/Signature"
  - "knowledge_bases.yaml controlled seed (26-01) — the cmd-side seed snapshot source"
provides:
  - "internal/knowledge.WalkRegisteredDirs — the single registry-driven directory walker (three-state table) for every corpus consumer"
  - "knowledge.Index(ctx, dir, emb, mc, cfg, reg) — registry-driven index with validateTiers(Lookup) KNOW-02 gate and indexAuditHook seam"
  - "mongo.SeedFromKnowledgeDir(ctx, dir, emb, reg) — walker-converged mongo seeding with seedInsertHook/seedAuditHook seams"
  - "knowledge.New(r, reg) — N-dynamic graph node set (active entries, ascending slug)"
  - "milvus.DefaultTierMaxLen = kb.MaxSlugLen — slug ceiling single home"
affects:
  - internal/skill (26-04 resolves reads against the same registry the indexer/gate now consume)
  - internal/server (26-05 admin endpoints + knowledge path gate + annotate role resolution)
  - internal/agent + internal/runtime (26-06 consume slug tiers end to end)
  - cmd/server (26-09 swaps the PROVISIONAL seed snapshot for kb.Open)
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "shared walker with a three-state decision table (active/deactivated/unregistered) as the single corpus entry point"
    - "package-level audit hook + exported Set*Hook setter (callback injection instead of a store param — Pitfall 7)"
    - "char-exact filter-template lock test (TestBuildMilvusFilterSlugs) guarding a value-source swap"
    - "parity pair: the same tree shape asserted once per backend with the walker as the shared oracle"
key-files:
  created:
    - internal/knowledge/walk.go
    - internal/knowledge/indexer_registry_test.go
    - internal/knowledge/walk_test.go
    - internal/retriever/mongo/seed_test.go
  modified:
    - internal/knowledge/indexer.go
    - internal/knowledge/graph.go
    - internal/knowledge/graph_test.go
    - internal/knowledge/indexer_test.go
    - internal/knowledge/indexer_know02_test.go
    - internal/retriever/mongo/seed.go
    - internal/retriever/mongo/mongo.go
    - internal/retriever/mongo/rank.go
    - internal/retriever/milvus/config.go
    - internal/retriever/milvus/filter.go
    - internal/retriever/milvus/filter_test.go
    - internal/vecmath/cosine.go
    - internal/store/store.go
    - cmd/server/main.go
    - cmd/eval/main.go
    - cmd/spike-history/main.go
    - cmd/spike-fanout/main.go
decisions:
  - "D26-03-A: the audit callback is a package-level hook with an exported setter (SetIndexAuditHook / SetSeedAuditHook) — the plan's letter ('unexported var, production binds it in main.go') is not legal Go across packages, and the callback seam (Pitfall 7) is preserved either way"
  - "D26-03-B: cmd/server's store build moved ahead of buildRetriever so the boot-time corpus index can write its unregistered-dir audit rows — also the boot order 26-02 documented as the 26-09 target (store migrate → kb registry → skill loader)"
  - "D26-03-C: the graph re-check rejects deactivated slugs too (Lookup semantics), and ErrInvalidTier gained an Active []string field so the error lists the live slug set — same fail-closed message discipline as the indexer gate"
  - "D26-03-D: CLI binaries (cmd/eval, cmd/spike-*) keep the audit hooks nil (WARN-only) so eval/spike runs never write rows into the server's audit trail; only cmd/server binds the INSERT-only PutAudit wrapper"
  - "D26-03-E: the parity pair is two tests (knowledge/walk_test.go + mongo/seed_test.go) because a package knowledge test cannot import mongo (mongo imports knowledge); both use WalkRegisteredDirs as the shared oracle on the identical tree shape"
metrics:
  duration: ~40 min
  completed: 2026-09-06
  tasks: 3
  commits: 5
  test_functions: 50
  files: 17
  insertions: 1585
status: complete
---

# Phase 26 Plan 03: Registry-Driven Index & Retrieval Layers Summary

**One-liner:** one registry-driven directory walker (active → index, deactivated → WARN skip, unregistered → WARN skip + INSERT-only audit row via a callback seam) now feeds both the Milvus indexer and the mongo seeder, the KNOW-02 main gate resolves through `kb.Registry.Lookup` on both the index and graph sides, Milvus/Mongo filter signatures stay byte-identical with only the value source moved to registry slugs, and all four cmd binaries compile against the new signatures on a PROVISIONAL seed snapshot.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `internal/knowledge/walk.go` (`WalkRegisteredDirs` + `DirAuditFn`) + registry-driven `Index(..., reg)` with the nil-registry fail-closed guard, `validateTiers(docs, reg)` KNOW-02 gate, and the `indexAuditHook` seam; RED `a5bcaad`, GREEN `0529b5b` | `a5bcaad`, `0529b5b` |
| 2 | mongo seed converged onto the shared walker (`tierDirs` hand-copy deleted), `seedInsertHook`/`seedAuditHook` seams, `milvus.DefaultTierMaxLen = kb.MaxSlugLen`, filter/mongo/rank/vecmath comment layers de-CN'd; RED `8bc7061`, GREEN `30c5d04` | `8bc7061`, `30c5d04` |
| 3 | `knowledge.New(r, reg)` N-dynamic graph node set + all four cmd binaries wired with the PROVISIONAL seed snapshot and (server) the audit binding; `store.AuditRow.Action` godoc extended | `0c98bb6` |

Trust semantics locked by tests (50 test functions across the knowledge/retriever suites, all hermetic on Windows):

- **three-state directory table** — `TestWalkRegisteredDirs{Active,Unregistered,Deactivated,MissingDirIsSilent}`: active slugs returned ascending; an unregistered dir yields exactly one `action_kb_dir_unregistered` audit row whose JSON detail names the dir AND the knowledge root, and never fails the walk (SC-5); a deactivated dir is skipped with **no** audit row (deactivation is a known operator state, not an anomaly); an empty or absent root is the legal degraded state (SC-4).
- **nil registry fails closed everywhere** — `TestIndexRejectsNilRegistry`, `TestWalkRegisteredDirsNilRegistry`, `TestSeedFromKnowledgeDirNilRegistry`, `TestGraphNilRegistryFailsClosed`: a nil registry would silently index/seed/render nothing, indistinguishable from a healthy empty corpus.
- **KNOW-02 gate through Lookup** — `TestValidateTiersRegistry` (+ migrated `indexer_know02_test.go`): unknown AND deactivated tier values are rejected with the active slug list in the message; the empty-tier branch is preserved verbatim; the static guard `TestIndexer_ValidateTiersRunsBeforeSeedDocs` still locks gate-before-backend ordering.
- **graph nodes = registry** — `TestGraphNodesFromRegistry` (4 active → 4 nodes; deactivating personal removes the node; a fifth base adds a fifth node group), `TestGraphDeterministicOrder` (ascending slug order replaces `TierEnumValues`), `TestGraphInvalidTierListsActiveSlugs`, plus `TestGraphBuilder_RejectsDeactivatedTier`.
- **walker convergence (Pitfall 4)** — `TestWalkerParityWithIndex` and `TestSeedFromKnowledgeDirWalkerParity` drive the identical tree (2 active + 1 deactivated + 1 unregistered) through their respective backends with the walker as the shared oracle: same doc set, same skip decisions, same audit surface.
- **D-09 and the filter template untouched** — `TestBuildMilvusFilterSlugs` asserts `tier in ["authority","historical"] and (effective_to == 0 or effective_to >= N)` character-for-character; `TestBuildMilvusFilter_EmptyTiers` keeps the empty-set `"false"` default-deny; mongo's `$in:[]` + "always build the filter, always run the find" block is byte-identical.

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| Windows `go test ./internal/knowledge/ -run 'TestWalk\|TestIndex\|TestValidateTiers' -count=1` | PASS (Task 1 gate) |
| Windows `go test ./internal/knowledge/ ./internal/retriever/... -count=1` | PASS (knowledge + milvus + mongo) |
| Windows `go vet ./internal/knowledge/ ./internal/retriever/... ./internal/vecmath/ ./cmd/...` | CLEAN |
| WSL `go build ./...` | PASS (exit 0 — the authoritative whole-tree check) |
| WSL `go test ./internal/knowledge/ ./internal/retriever/... ./internal/kb/ ./internal/vecmath/ -count=1` + `go vet` | PASS, vet clean |
| Windows `go build ./...` | Only `cmd/server` fails at LINK — the pre-existing D15-01-1 MSYS2 gcc/duckdb blocker. Proven unrelated: the failure is the `-lduckdb_static` CGO link step and `go list -deps ./cmd/server` shows the duckdb chain; no file this plan touched is in that dependency path (type-check/vet pass clean). |
| `git status --porcelain eval/` | Only the two untracked artifacts that pre-date this plan (`calibration.json`, `report.json`, present in the session-start status); zero tracked frozen-asset modifications |

Acceptance criteria all hold: `knownTierDirs` == 0 hits; `TierEnum` == 0 in non-test files of `internal/knowledge/`, `internal/retriever/`, `internal/vecmath/`; `WalkRegisteredDirs` referenced 3× in `mongo/seed.go` and 4× in `indexer.go`; `func Index` carries `reg kb.Registry`; `kb.MaxSlugLen` is the only length literal in `milvus/config.go` (no `= 20` remains); `TestBuildMilvusFilterSlugs` locks the template; `knowledge.Index(ctx` appears 4× in cmd with `kbReg` on every line; `SeedFromKnowledgeDir(ctx` appears 3× all with `kbReg`; the single `knowledge.New(` production site is two-arg; all 4 cmd files carry the exact `PROVISIONAL (26-03): read-only seed snapshot` marker (grep-verified handoff to 26-09); repo `gofmt` noise in untouched regions is the pre-existing Go 1.25 alignment drift 26-02 documented — files created by this plan are gofmt-clean.

## Deviations from Plan

**1. [Rule 3 - Blocking] The audit hook needed an exported setter to be bindable from main.go**
- **Found during:** Task 1 implementation
- **Issue:** the plan declares `var indexAuditHook func(action, detail string)` with the comment "生产在 main.go 绑 store.PutAudit" — but an unexported package variable cannot be bound from `package main`. Production would have shipped with the hook permanently nil and KBM-06's audit row would never exist.
- **Fix:** kept the unexported var (and the required `indexAuditHook(` call site, satisfying the artifact grep) and added exported one-liner setters `knowledge.SetIndexAuditHook` / `mongo.SetSeedAuditHook`; godoc states the INSERT-only requirement and the bind-once-at-boot contract. Same seam shape, now actually reachable.
- **Commit:** `0529b5b`, `30c5d04`

**2. [Rule 2 - Missing critical wiring] cmd/server's store build moved ahead of the retriever**
- **Found during:** Task 3
- **Issue:** `buildRetriever` (which runs the boot-time corpus index) executed before `buildStore`, so any audit row the walk emitted during boot would have been dropped — the exact rows KBM-06's decision table exists to record.
- **Fix:** `buildStore` moved to step 4 with an ordering comment; this also matches the boot order 26-02's SUMMARY documented as the 26-09 target (store migrate → kb registry → skill loader).
- **Commit:** `0c98bb6`

**3. [Rule 3 - Blocking] Task 2's parity test needed a home in the mongo package**
- **Found during:** Task 2
- **Issue:** the plan assigns the parity test to `internal/knowledge/walk_test.go`, but a `package knowledge` test cannot import `internal/retriever/mongo` (mongo imports knowledge → import cycle); the external `knowledge_test` package could, but then `indexSeedHook` is unreachable.
- **Fix:** split the parity pair: `walk_test.go` proves the knowledge.Index side, new `internal/retriever/mongo/seed_test.go` proves the mongo side; both use `knowledge.WalkRegisteredDirs` as the shared oracle on the identical tree shape (D26-03-E).
- **Commit:** `8bc7061`, `30c5d04`

**4. [Rule 3 - Blocking] mongo seeding needed an insert seam for hermetic tests**
- **Found during:** Task 2
- **Issue:** `SeedFromKnowledgeDir` writes via `c.Reset` + `c.Insert` (real collection handles); the package's existing stub only covers `Search`'s finder, so the plan's "capture insert 的 docs" parity assertion had no seam and would have needed a live MongoDB (forbidden by the hermetic-tests constraint).
- **Fix:** unexported `seedInsertHook` intercepting the whole write phase, mirroring `knowledge.indexSeedHook` (nil in production → the Reset+Insert pair runs unchanged); the empty-tree-no-write test additionally locks that an idle boot never wipes a collection it did not refill.
- **Commit:** `30c5d04`

**5. [Rule 1 - Consistency] Acceptance greps forced two comment files outside the plan's file list**
- **Found during:** Task 2
- **Issue:** the plan's own acceptance criterion (`grep -rn "TierEnum" internal/retriever/ internal/vecmath/ --include='*.go' | grep -v _test` == 0) cannot hold while `mongo/rank.go:75` and a second `vecmath/cosine.go` paragraph still cite "imports internal/skill for TierEnum"; `internal/store/store.go`'s `Action` enum godoc also lagged the new writer once cmd/server started emitting `action_kb_dir_unregistered` rows (research A7: a lagging enum doc breaks AuditFilter consumers).
- **Fix:** comment-only updates in `rank.go`, `cosine.go`, and `store.go` (no logic touched anywhere in these three).
- **Commit:** `30c5d04`, `0c98bb6`

**6. [Task-sequencing note] Task 1's TierEnum-zero grep for internal/knowledge only reaches 0 after Task 3**
- **Issue:** `graph.go` (a Task 3 file, unlocked by the `knowledge.New` signature change) still carried 6 `skill.TierEnum*` references when Task 1 closed.
- **Outcome:** by plan end the grep is 0 — verified after `0c98bb6`. No action needed; recorded so a per-task verifier does not misread the intermediate state.

## Notes for Downstream Plans

- **26-04 (skill):** the value path you consume is now slug end-to-end — `resolvedTiers`, `validateTiers`, the graph re-check, and both filter builders speak `kb.Registry` slugs. `skill.TierEnum`/`TierEnumByValue`/`TierEnumValues` remain in `internal/skill` untouched (your deletion scope); nothing in `internal/knowledge`, `internal/retriever`, or `internal/vecmath` references them anymore.
- **26-05 (admin endpoints):** `action_kb_dir_unregistered` rows are live from cmd/server boot (Timestamp + `UserID:"system"` + Action + FrozenArgs JSON `{"dir":...,"knowledge_dir":...}`). Extend the `AuditRow.Action` godoc list with `action_kb_created`/`action_kb_deactivated` when those writers land; the AuditFilter path already surfaces the new value.
- **26-09 (kb.Open wiring):** replace exactly the four `PROVISIONAL (26-03): read-only seed snapshot` blocks (grep anchor in cmd/server/main.go, cmd/eval/main.go, cmd/spike-history/main.go, cmd/spike-fanout/main.go) with `kb.Open`; cmd/server's store now boots before the retriever, so the 26-02 target ordering is already in place. The audit setter calls stay where they are — only the registry source changes.
- **26-10 (operator gate):** the CN residue risk (Pitfall 1) is now structural: any stale backend row carrying a CN-char or deactivated slug is rejected by the graph re-check (`ErrInvalidTier` names the live slug set), and empty/absent knowledge roots stay the legal D-09 refusal state. Live non-zero-hit verification remains your checkpoint.
- **milvus column width:** `DefaultTierMaxLen` is now an alias of `kb.MaxSlugLen` (20) — schema untouched, and the two can no longer drift.

## Known Stubs

None. The four `PROVISIONAL (26-03): read-only seed snapshot` markers are deliberate handoff anchors mandated by the plan (grep-verified), not stubs: each constructs a fully functional read-only registry via `kb.LoadSeedFile` + `kb.New`, and the replacement (`kb.Open`) is scheduled to plan 26-09. `grep -rn "TODO\|FIXME" internal/knowledge/ internal/retriever/` returns nothing new; the only "placeholder" hit is a pre-existing Phase 2 comment in `indexer.go` about operator-authored empty files.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-03-1 (EoP via validateTiers) | gate resolves `reg.Lookup` fail-closed; deactivated slugs rejected too; errors list the active slug set (`TestValidateTiersRegistry`, `TestGraphInvalidTierListsActiveSlugs`) |
| T-26-03-2 (Info disclosure via unregistered dirs) | content never indexed; WARN + skip + INSERT-only `action_kb_dir_unregistered` row with a JSON detail naming dir + root (`TestWalkRegisteredDirsUnregistered`) |
| T-26-03-3 (DoS via CN residue → silent mass refusal) | value source is single (slugs), stale values are loudly rejected at the graph seam instead of silently matching 0 rows; explicit re-seed anchor delivered via the PROVISIONAL markers for 26-09; live non-zero-hit check stays the 26-10 operator gate |
| T-26-03-4 (Tampering / filter injection) | `BuildMilvusFilter` body, `%q` escaping, and empty-set `"false"` branch byte-identical — locked character-for-character by `TestBuildMilvusFilterSlugs`; mongo `$in` filter doc untouched |
| T-26-03-5 (Repudiation of directory decisions) | audit rows ride the callback seam (`SetIndexAuditHook`/`SetSeedAuditHook` → `store.PutAudit`), append-only, no store param polluting the pure index/seed functions; `AuditRow.Action` enum doc extended |
| T-26-SC (dependency legitimacy) | zero new dependencies — `go.mod` untouched |

## Self-Check: PASSED

- Files exist: `internal/knowledge/{walk.go,indexer_registry_test.go,walk_test.go}`, `internal/retriever/mongo/seed_test.go` — all FOUND; all 13 modified files present in `git diff --stat a21bb50..HEAD` (21 files changed counting tests, 1585 insertions / 321 deletions).
- Commits exist: `a5bcaad`, `0529b5b`, `8bc7061`, `30c5d04`, `0c98bb6` — all FOUND in `git log a21bb50..HEAD` on `phase-20-p10-6-admin-console`.
- TDD gate sequence: `test(26-03)` RED `a5bcaad` precedes `feat(26-03)` GREEN `0529b5b` (Task 1); RED `8bc7061` precedes GREEN `30c5d04` (Task 2).
- Windows hermetic suite + WSL CGO suite both PASS after the final commit; WSL `go build ./...` exit 0.
