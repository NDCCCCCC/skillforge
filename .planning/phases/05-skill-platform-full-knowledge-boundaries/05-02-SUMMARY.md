---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 02
subsystem: skill-platform
tags: [go, eino-adk, bge-m3, milvus, mongo, sentinel-0, pre-filter, tdd, ui-message-chunk]

# Dependency graph
requires:
  - phase: 02-compliance-vertical-slice
    provides: Skill Manifest + TierEnum + retriever.Hit/Document types
  - phase: 03-agentic-retrieval-trust-evaluation
    provides: VerdictPipeline + Retriever interface + Stub retriever shape
  - phase: 05-01
    provides: Skill Registry atomic.Pointer snapshot + Entry.DescVec + DecisionPayload wire fields
provides:
  - "Skill Router (BGE-M3 cosine against registry snapshot) with ambiguity thresholds (0.4 low-floor / 0.08 margin) — D5-05"
  - "routeThenRun wiring in chatHandler BEFORE VerdictPipeline + data-decision trace emit — SC-1"
  - "retriever.Document + Hit gain EffectiveFrom + EffectiveTo int64 (sentinel-0, NOT nullable) — D5-15"
  - "Retriever.Search gains now int64 parameter; backends co-layer \$or effective_to PRE-filter — D5-15/T-5-08"
  - "Milvus schema adds effective_from + effective_to INT64 columns; non-nullable by default (Milvus SDK has no WithNullable)"
  - "Mongo docRow + Insert + Search filter gain effective_from/to + \$or clause"
  - "Indexer parseEffectiveWindow reads YAML frontmatter; validateEffective rejects inverted/negative windows — KNOW-02 extension"
  - "splitFrontmatter renamed from stripFrontmatter; returns (body, frontmatter) so the effective window can be parsed before stripping"
  - "All Search callers (pipeline.go, retrieve_tool.go, agent_test stubRetriever, cmd/eval, cmd/spiked) updated for the new signature"
affects:
  - "05-03: pipeline.go can now thread an external `now` (e.g. eval time-travel) — signature already accepts the param"
  - "05-04: FE consumes the route trace chunk + SkillPicker empty-state on Selected=''"
  - "05-06: redline pass can operate on Hit.EffectiveFrom/EffectiveTo without an extra round-trip"

# Tech tracking
tech-stack:
  added:
    - "internal/vecmath leaf package — cosine extracted from mongo/rank to break import cycle (router.go + mongo.race both need it)"
  patterns:
    - "sentinel-0 design for policy-version window: zero = perpetual, negative or inverted rejected at indexer"
    - "compute-now-once-at-pipeline-entry: pipeline.go and retrieve_tool.go both compute `now := time.Now().Unix()` at the boundary, not deep inside the backend"
    - "co-layered PRE-filter (NOT post-filter): tier \$in + effective_to \$or in the SAME bson.D / Milvus expr — neither is a fallback"
    - "frontmatter return from stripFrontmatter → splitFrontmatter: payload + body returned together so future fields can be extracted without re-parsing"
    - "package-internal hook (yamlUnmarshal) mirrors indexSeedHook: tests can swap the YAML decoder"

key-files:
  created:
    - "internal/skill/router.go — Router type + DecisionResult + AmbiguityLowFloor/Margin consts + NewRouter (renamed from New to avoid Registry.New collision)"
    - "internal/skill/router_test.go — 7 tests (highest-cosine, top-1 low, top-1/top-2 close, empty snapshot, embedder error, no-magic-numbers, single-skill)"
    - "internal/vecmath/cosine.go — leaf package, single Cosine function with full docstring (extracted from mongo/rank)"
    - "internal/retriever/milvus/filter_test.go — 4 cases (tiers-only, empty-tiers, effective-to-sentinel, effective-to-now-zero)"
  modified:
    - "internal/server/chat.go — routeThenRun helper added; chatHandler routes BEFORE VerdictPipeline when Registry+Router wired"
    - "internal/server/server.go — Deps gains Router + Registry fields; pipeline.go threads `now := time.Now().Unix()` once at entry"
    - "internal/server/testhelpers_test.go — stubRetriever.Search signature gains _ int64 for now"
    - "internal/retriever/retriever.go — Document + Hit gain EffectiveFrom/EffectiveTo; Retriever.Search gains now int64"
    - "internal/retriever/milvus/filter.go — BuildMilvusFilter(tiers, now) co-layers effective_to PRE-filter"
    - "internal/retriever/milvus/seed.go — schema gains effective_from + effective_to INT64 columns; SeedDocuments writes them"
    - "internal/retriever/milvus/search.go — OutputFields gains effective_*; Hit rows populate the new fields"
    - "internal/retriever/mongo/mongo.go — Search gains now; Insert writes effective_*; filter gains \$or clause"
    - "internal/retriever/mongo/rank.go — docRow gains EffectiveFrom + EffectiveTo; Hit rows populate them"
    - "internal/retriever/mongo/mongo_test.go — 13 existing Search calls updated to pass testNow const; extractTiersFromFilter scans for 'tier' key (NOT index 0); new TestMongoSearch_FilterShapeIncludesEffectiveToOr + TestMongoInsert_WritesEffectiveFields"
    - "internal/knowledge/indexer.go — splitFrontmatter returns (body, fm); parseEffectiveWindow reads YAML; validateEffective rejects inverted/negative; yamlUnmarshal hook for testability"
    - "internal/knowledge/indexer_test.go — TestSplitFrontmatter + TestParseEffectiveWindow added; validateEffective cases covered"
    - "internal/agent/retrieve_tool.go — ret.Search call passes now := time.Now().Unix() per call"
    - "internal/agent/agent_test.go — stubRetriever.Search signature updated for compile-time interface guard"
    - "cmd/eval/main.go — r.Search call passes now; godoc updated"
    - "cmd/spiked/milvus.go — BuildMilvusFilter + wrapped.Search pass now; godoc updated"
    - "cmd/spiked/milvus_test.go — BuildMilvusFilter smoke test updated for 2-arg signature; timeNow() helper for deterministic integration test"

key-decisions:
  - "AmbiguityLowFloor=0.4, AmbiguityMargin=0.08 — exported consts (NOT magic numbers inside Route). Calibration deferred to Phase 6 GEVL work per RESEARCH §Open Question 3."
  - "Cosine extracted to internal/vecmath leaf package (no second vector-math copy): router.go needs cosine without creating a cycle through internal/retriever/mongo/seed.go (which itself imports internal/skill for TierEnum). mongo/rank.go re-exports as Cosine for backward compat."
  - "NewRouter (not New): both Router.New and Registry.New live in the internal/skill package — the type qualifier would shadow; renamed to disambiguate."
  - "sentinel-0 (NOT nullable): Milvus SDK entity.Field has NO WithNullable method (verified against milvus-sdk-go v2.4.2) — the schema is structurally non-nullable. Mongo treats missing fields as 0 (bson.D default + the \$or accepting literal 0). Sidesteps NULL-evaluates-FALSE pitfall entirely."
  - "Pipeline computes now := time.Now().Unix() ONCE at entry, not per retriever call (T-5-10): deterministic snapshot per pipeline run, no clock skew between calls. retrieve_tool.go computes per-call because each tool invocation is an independent request."
  - "splitFrontmatter (renamed from stripFrontmatter) returns (body, fm) so future fields can be extracted without re-parsing. Pre-existing TestStripFrontmatter renamed to TestSplitFrontmatter; semantics unchanged."
  - "parseEffectiveWindow is fail-soft on malformed YAML (warns to stderr, defaults to 0/0): a stray frontmatter bug must NOT brick the indexer. validateEffective catches inverted/negative downstream."
  - "extractTiersFromFilter scans for the 'tier' key rather than asserting index 0: the bson.D filter now has tier + \$or entries co-layered (D5-15), so the helper had to be robust against clause additions."

patterns-established:
  - "sentinel-0 for new optional fields: zero is the explicit 'perpetual / default' state — never use nullable for policy-version metadata"
  - "compute-once-at-boundary: any 'now'-like parameter passed into a chain is computed at the boundary and threaded, not re-evaluated per call"
  - "co-layered PRE-filter: tier + effective clauses in the same bson.D / Milvus expr (NOT sequential post-filter), tier first for selectivity short-circuit"
  - "package-internal unmarshal hook (yamlUnmarshal) mirrors indexSeedHook: tests can swap the decoder without standing up real I/O"
  - "static guard for production wiring: TestIndexer_ProductionWiresValidateEffective (asserts src contains 'validateEffective(') mirrors TestIndexer_ValidateTiersRunsBeforeSeedDocs for the tier path"

requirements-completed: [SYSK-05, CMPL-06, CMPL-08]

# Metrics
duration: 45min
completed: 2026-08-12
---

# Phase 5 Plan 2: Skill Router + Chunk Schema Migration Summary

**Skill Router with route-then-run + retriever gains effective_from/to (sentinel-0) PRE-filter co-layered at the same database layer as the tier filter**

## Performance

- **Duration:** ~45 min (Task 1: 4 commits ~15min; Task 2: 3 commits ~30min including the import-cycle resolution)
- **Started:** 2026-08-12T09:45:00Z
- **Completed:** 2026-08-12T10:30:00Z
- **Tasks:** 2/2
- **Files modified:** 19 (Task 1: 7 files / 4 commits; Task 2: 16 files / 3 commits)

## Accomplishments

- **Task 1 — Skill Router + route-then-run:** `Router.Route(ctx, query, snap)` embeds the query via the production Embedder, computes cosine vs every `Entry.DescVec`, and returns `DecisionResult { Selected, Scores (sorted desc), Ambiguous }`. `AmbiguityLowFloor=0.4` + `AmbiguityMargin=0.08` are exported consts (calibration deferred to Phase 6). `chatHandler` routes BEFORE invoking `VerdictPipeline` when `Registry` + `Router` are wired; emits the routing decision via the existing `data-decision` wire type (no new wire type — D5-04 reserves the budget for redline). Empty snapshot → `Selected='' + Ambiguous=true` (FE renders the existing SkillPicker empty state). Embedder error surfaces as a 500 with the server error envelope (no hash-based fallback that would silently mask a BGE-M3 outage).
- **Cosine reuse, no second vector-math copy:** Plan called for `REUSE the existing cosine()` per RESEARCH §Don't Hand-Roll. The naive import (`internal/retriever/mongo.Cosine`) would have created a cycle because `internal/retriever/mongo/seed.go` imports `internal/skill` for `TierEnum`. Resolved by extracting `cosine` into a leaf package `internal/vecmath/cosine.go`; both `internal/skill/router.go` and `internal/retriever/mongo/rank.go` import it. `mongo/rank.go` re-exports as `Cosine` so all internal mongo callers stay unchanged.
- **Task 2 — Chunk schema migration (D5-15 sentinel-0 design):** `retriever.Document` + `Hit` gain `EffectiveFrom int64` + `EffectiveTo int64`. Sentinel-0 (NOT nullable): zero is the explicit perpetual state. `Retriever.Search` gains `now int64` parameter; the pipeline computes `now := time.Now().Unix()` ONCE at entry (T-5-10 — deterministic snapshot per pipeline run, no clock skew between calls).
- **Milvus schema:** `effective_from + effective_to INT64` columns added to `EnsureCollection`. The Milvus SDK `entity.Field` has NO `WithNullable` method (verified against v2.4.2) — INT64 columns are structurally non-nullable, so the NULL-evaluates-FALSE pitfall is sidestepped entirely. `SeedDocuments` writes the new columns; `Search` requests them in `OutputFields` and populates the returned `Hit` rows.
- **Mongo backend:** `Search` filter becomes `bson.D{{tier: {$in: tiers}}, {$or: [{effective_to: 0}, {effective_to: {$gte: now}}]}`. The tier clause runs first (selectivity win — a default-deny Skill never evaluates the effective clause). `Insert` writes `effective_from/to` into the new bson.M fields. `docRow` + `Hit` round-trip the values.
- **Indexer (KNOW-02 extension):** `splitFrontmatter` (renamed from `stripFrontmatter`) returns `(body, frontmatter)` so `parseEffectiveWindow` can read the YAML `effective_from` / `effective_to` keys BEFORE stripping. `validateEffective` rejects `effective_to < 0` and inverted windows (both nonzero AND `effective_to < effective_from`) — fail-fast at the indexer layer before any backend write. The static guard `TestIndexer_ProductionWiresValidateEffective` asserts the production `Index` path calls `validateEffective` (mirroring the existing tier-mandatory guard).
- **All Search callers updated:** `internal/server/pipeline.go` (production), `internal/agent/retrieve_tool.go` (ReAct tool — computes `now` per call because each tool invocation is independent), `cmd/eval/main.go` (per-case), `cmd/spiked/milvus.go` (spike integration). Test stubs in `internal/server/testhelpers_test.go` and `internal/agent/agent_test.go` updated for compile-time `retriever.Retriever` conformance.
- **Backward compat (sentinel-0 = 0):** Existing chunks without `effective_*` frontmatter get the Go zero value (0/0) — the perpetual sentinel. The `\$or` accepts the literal 0, so they pass the PRE-filter unchanged. No migration needed for the Phase 2 ~30-row fixture corpus.
- **No regressions:** full `go test ./...` passes (`internal/agent`, `internal/eval`, `internal/knowledge`, `internal/retriever/milvus`, `internal/retriever/mongo`, `internal/server`, `internal/skill`, `internal/store`, `internal/verdict`, `internal/wire`, `cmd/spiked`).

## Task Commits

| Task | Commit | Type | Files |
| ---- | ------ | ---- | ----- |
| 1 (RED) | `4d64f0a` | test | router_test.go + export mongo.Cosine |
| 1 (GREEN) | `07d85af` | feat | router.go + leaf package vecmath |
| 1 (RED) | `60c61e0` | test | chat_test.go + testhelpers route-then-run |
| 1 (GREEN) | `4b11e7d` | feat | chat.go routeThenRun + server.go Deps wiring |
| 2 (RED) | `7eba9fa` | test | milvus/filter_test + mongo/mongo_test + knowledge/indexer_test |
| 2 (GREEN #1) | `7eba9fa` | (consolidated) | — |
| 2 (GREEN) | `92b908c` | feat | retriever + milvus + mongo + indexer + pipeline + tool + eval + spiked |

Note: Task 2 RED and the GREEN implementation landed as a single combined 92b908c because the GREEN work spanned 16 files in one logical schema migration; the test commits listed here represent the cumulative RED coverage across milvus/filter_test, mongo/mongo_test, and knowledge/indexer_test.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Import cycle prevented direct `mongo.Cosine` reuse in router.go**
- **Found during:** Task 1 GREEN
- **Issue:** Plan instructed "REUSE the existing cosine implementation: export `Cosine` from the mongo package and import it in router.go". The direct import `internal/retriever/mongo` from `internal/skill/router.go` would have created a cycle because `internal/retriever/mongo/seed.go` imports `internal/skill` for `TierEnum`.
- **Fix:** Extracted `cosine` into a new leaf package `internal/vecmath/cosine.go`. Both `internal/skill/router.go` and `internal/retriever/mongo/rank.go` import the leaf package. `mongo/rank.go` re-exports as `Cosine` so all internal mongo callers stay byte-identical.
- **Files modified:** `internal/vecmath/cosine.go` (new), `internal/retriever/mongo/rank.go`
- **Commit:** `07d85af`

**2. [Rule 1 — Bug] `New` constructor collision between Router and Registry**
- **Found during:** Task 1 GREEN
- **Issue:** Both `Router.New` and `Registry.New` live in the `internal/skill` package. Plan called for `func New(emb embedder.Embedder) *Router` — the bare name would shadow the Registry constructor at every call site within the package.
- **Fix:** Renamed `Router` constructor to `NewRouter`. Tests + chat.go updated.
- **Files modified:** `internal/skill/router.go`, `internal/skill/router_test.go`, `internal/server/chat.go`
- **Commit:** `07d85af` (refactored in this commit, before any caller landed)

**3. [Rule 2 — Missing critical functionality] Milvus SDK has no `WithNullable` on `entity.Field`**
- **Found during:** Task 2 GREEN (`go build ./...`)
- **Issue:** Plan instructed "WithNullable(false)" on the `effective_from/to` INT64 columns. The milvus-sdk-go v2.4.2 `entity.Field` type does NOT expose a `WithNullable` method (verified via `go doc`). INT64 columns are non-nullable by default.
- **Fix:** Removed `WithNullable(false)` calls. Updated docstring to reflect the SDK reality: "Milvus INT64 columns are non-nullable by default — the SDK's entity.Field type has no WithNullable method (verified against milvus-sdk-go v2.4.2), so the schema is structurally non-nullable. NULL-evaluates-FALSE is therefore not a concern here." The sentinel-0 design rationale (zero = perpetual default) is preserved; the structural mitigation is even stronger than the plan assumed.
- **Files modified:** `internal/retriever/milvus/seed.go`
- **Commit:** `92b908c`

**4. [Rule 2 — Missing critical functionality] `stripFrontmatter` could not return the frontmatter**
- **Found during:** Task 2 GREEN
- **Issue:** Plan said "extend the frontmatter parser to extract effective_from / effective_to BEFORE stripping". The existing `stripFrontmatter` returned only `body []byte` — the YAML frontmatter was discarded during stripping, so `parseEffectiveWindow` had no input to read.
- **Fix:** Renamed `stripFrontmatter` → `splitFrontmatter` with signature `(body, frontmatter []byte)`. Existing `TestStripFrontmatter` renamed to `TestSplitFrontmatter`; semantics unchanged (test asserts on body only). New `parseEffectiveWindow` reads the YAML via a `yamlUnmarshal` hook (mirroring `indexSeedHook`) so tests can swap the decoder.
- **Files modified:** `internal/knowledge/indexer.go`, `internal/knowledge/indexer_test.go`
- **Commit:** `92b908c`

**5. [Rule 1 — Bug] Existing `extractTiersFromFilter` hardcoded index 0**
- **Found during:** Task 2 GREEN
- **Issue:** Plan added a second bson.D entry (`$or` on `effective_to`); the existing `extractTiersFromFilter` test helper asserted `len(d)==1` and read `d[0]`, which would have failed every Mongo Search test after the schema change.
- **Fix:** Updated helper to scan for the `tier` key (rather than hardcoding index 0). Updated docstring + added justification: "D5-15: scan, don't hardcode index 0 — so the helper stays robust against future clause additions." `TestFilterShapeIsTierIn` rewritten to scan for the `tier` key, preserving the trust-boundary assertion (still catches `bson.D → bson.M` drift + `[]string → []any` drift).
- **Files modified:** `internal/retriever/mongo/mongo_test.go`
- **Commit:** `92b908c`

### Plan-stated Assumptions Updated

**6. Phase-3 Search callers also include `cmd/eval/main.go` + `cmd/spiked/milvus.go`**
- **Found during:** Task 2 GREEN (`go vet ./...`)
- **Issue:** Plan mentioned "Update internal/agent/callbacks.go and any other Phase-3 Search callers similarly". The actual Phase-3 callers are `cmd/eval/main.go` (per-case parallel search for calibration MaxScore) + `cmd/spiked/milvus.go` (BuildMilvusFilter + wrapped.Search). `internal/agent/retrieve_tool.go` is the production ReAct tool.
- **Fix:** Updated all three callers; spike `BuildMilvusFilter` smoke test rewritten to assert substring containment (the effective clause is now appended, so exact-string equality would have failed).
- **Files modified:** `internal/agent/retrieve_tool.go`, `cmd/eval/main.go`, `cmd/spiked/milvus.go`, `cmd/spiked/milvus_test.go`
- **Commit:** `92b908c`

### Auth Gates

None — no auth errors encountered during execution.

## Self-Check

- [x] All Task 1 + Task 2 commits exist (`4d64f0a`, `07d85af`, `60c61e0`, `4b11e7d`, `7eba9fa`, `92b908c`)
- [x] `go test ./...` exits 0
- [x] `go vet ./...` exits 0
- [x] `go build ./...` exits 0
- [x] Router is wired into chatHandler BEFORE VerdictPipeline (verified by `TestChatHandler_RoutesBeforePipeline`)
- [x] `BuildMilvusFilter` output contains `effective_to == 0 or effective_to >=` (asserted by `TestBuildMilvusFilter_EffectiveToSentinel`)
- [x] Mongo Search bson filter contains `$or` with effective_to clauses (asserted by `TestMongoSearch_FilterShapeIncludesEffectiveToOr`)
- [x] Indexer production code calls `validateEffective(` (asserted by `TestIndexer_ProductionWiresValidateEffective`)
- [x] VerdictPipeline contains exactly one `time.Now().Unix()` call
- [x] `Document.EffectiveFrom` + `EffectiveTo` are int64 (not pointers / nullable types)
- [x] All Search callers (pipeline.go, retrieve_tool.go, eval, spiked, test stubs) updated for the new signature

## Self-Check: PASSED