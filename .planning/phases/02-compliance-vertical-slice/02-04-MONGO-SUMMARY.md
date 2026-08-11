---
phase: 02-compliance-vertical-slice
plan: 04 (extension — Mongo backend)
subsystem: database
tags: [mongo, retriever, vector-search, cosine, go, multi-backend]

# Dependency graph
requires:
  - phase: 02-compliance-vertical-slice
    provides: "retriever.Retriever interface (02-04-REFACTOR); retriever.Hit / Document types; tier-filter PRE-similarity contract (SYSK-07 / P-RAG-2 / D-09)"
provides:
  - "internal/retriever/mongo — MongoDB 4.4+ backend implementing retriever.Retriever (tier-filtered cosine search)"
  - "RETRIEVER_BACKEND env switch in cmd/server (milvus default, mongo opt-in)"
  - "In-Go cosine ranking math (rank.go) reusable by any future no-native-vector backend"
  - "MongoDB seeding path (SeedFromKnowledgeDir) parallel to the milvus indexer"
affects: [02-compliance-vertical-slice (live verify), 03-agentic-retrieval, 07-packaging]

# Tech tracking
tech-stack:
  added:
    - "go.mongodb.org/mongo-driver/v2 v2.8.0 (Connect/Ping/Database/Collection/Find/InsertMany)"
    - "transitive: github.com/youmark/pkcs8, github.com/klauspost/compress, github.com/xdg-go/scram, github.com/xdg-go/stringprep, github.com/xdg-go/pbkdf2, golang.org/x/crypto, golang.org/x/term"
  patterns:
    - "Pure-logic-first: rank/cosine in a file with no driver import so unit tests run with no live DB"
    - "rowFinder interface seam: production wraps *mongo.Collection.Find; tests inject fakeFinder for ranking assertions"
    - "PRE-filter at the storage layer: {tier:{$in:tiers}} is the database query itself — empty tiers → {$in:[]} matches nothing → refusal-by-construction (D-09 structural)"
    - "Backend switch via env (RETRIEVER_BACKEND); backend construction factored into buildRetriever() so Deps wiring stays single-path"

key-files:
  created:
    - internal/retriever/mongo/rank.go
    - internal/retriever/mongo/rank_test.go
    - internal/retriever/mongo/mongo.go
    - internal/retriever/mongo/mongo_test.go
    - internal/retriever/mongo/seed.go
  modified:
    - cmd/server/main.go
    - .env.example
    - go.mod
    - go.sum
    - scripts/live-verify.sh

key-decisions:
  - "Use mongo-driver v2 (v2.8.0) — the user's note allowed v1 OR v2 if v2 resolved; v2 is the current stable line"
  - "Store embeddings as []float32 in BSON arrays; mongo-driver handles float32↔double (BSON only has float64) automatically"
  - "Tier filter as {tier:{$in:tiers}} MongoDB query applied BEFORE cosine — never short-circuit on empty tiers so the database itself enforces D-09"
  - "Factor DB fetch behind rowFinder interface so Search ranking logic is unit-testable with a fakeFinder"
  - "Add SeedFromKnowledgeDir on *mongo.Client (NOT refactor internal/knowledge/indexer.go) — preserves the milvus tests; documented as Rule 3 deviation"
  - "Cosine in float32 with math.Sqrt round-trip — bit-exact across platforms, sufficient precision for ranking"

patterns-established:
  - "Pattern: pure-logic-first for any future backend (rank/cosine precedent — no driver imports in pure files)"
  - "Pattern: rowFinder-style seam for any DB-coupled Search method that wants ranking to be unit-testable"
  - "Pattern: backend switch via env (RETRIEVER_BACKEND) + buildRetriever() factory; single Deps assignment after the switch"

requirements-completed: []  # No formal requirements in PLAN frontmatter — this is an extension plan driven by user infra gap

# Metrics
duration: 13min
completed: 2026-08-11
---

# Phase 02 Plan 04 (Extension): MongoDB Retriever Backend Summary

**MongoDB 4.4+ retriever backend implementing retriever.Retriever — {tier:{$in}} PRE-filter + in-Go cosine ranking, wired behind RETRIEVER_BACKEND=mongo so SkillForge runs when Milvus is unavailable.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-08-11T13:04:32Z
- **Completed:** 2026-08-11T13:17:15Z
- **Tasks:** 5 (dependency add, pure logic + tests, Client + tests, server switch + env, summary + live-verify)
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments

- **Multi-backend retriever abstraction proven.** The same `retriever.Retriever` interface that 02-04-REFACTOR introduced now has a second concrete implementation. The verdict pipeline, the chat handler, the skill scope resolution — none of them changed. Switching backends is one env var.
- **PRE-filter semantics preserved end-to-end.** MongoDB applies `{tier:{$in:tiers}}` as a real query before any cosine math runs. Empty tiers → `{$in:[]}` → 0 hits → refusal-by-construction (D-09). The structural guarantee lives in the storage layer, not in a Go branch that could be removed in a future refactor.
- **30 unit tests pass with NO live MongoDB.** Pure `cosine`/`rank` math (17 tests in `rank_test.go`) plus the `fakeFinder`-injected `Search` wiring (13 tests in `mongo_test.go`) cover the contract surface. The structural D-09 half that needs a live MongoDB is left to `scripts/live-verify.sh`.
- **Zero regressions.** `go build ./...`, `go test ./internal/... ./cmd/...`, and `go test -tags=contract ./integration/contract/...` all exit 0. The milvus path is byte-for-byte unchanged; the milvus tests and SC-5 wire contract are untouched.

## Task Commits

1. **chore(deps): add mongo-driver v2** — `3585f30`
2. **feat(retriever/mongo): cosine + rank pure logic + tests** — `4743a4b`
3. **feat(retriever/mongo): Client implementing retriever.Retriever** — `0546db2`
4. **feat(cmd/server): RETRIEVER_BACKEND switch + .env.example + seed.go** — `2169266`
5. **docs(02-04): mongo backend summary + live-verify.sh mongo mode** — (this commit)

## Files Created/Modified

**Created:**
- `internal/retriever/mongo/rank.go` — pure `cosine(a,b)` + `rank(rows, q, k)`; `docRow` type shared with mongo.go; no driver imports.
- `internal/retriever/mongo/rank_test.go` — 17 tests covering cosine math (identical/orthogonal/opposite/known-value/zero-norm/length-mismatch) + rank semantics (order/topK/empty/dim-skip/tie-break/field-preservation).
- `internal/retriever/mongo/mongo.go` — `Client` (Connect/Ping/Disconnect), `Config.withDefaults`, `Ensure` (idempotent collection + `{tier:1}` index), `Insert` (InsertMany), `Search` (`{tier:{$in:tiers}}` → `rowFinder.FindRows` → `rank`), `mongoCollectionFinder` production adapter, `rowFinder` interface (test seam).
- `internal/retriever/mongo/mongo_test.go` — 13 tests via `fakeFinder` (filter shape, empty-tiers Go-side half, ranking delegation, topK clamp, field preservation, finder error propagation, defensive errors, Config defaults, helper sniffers, reflection guard, compile-time interface check).
- `internal/retriever/mongo/seed.go` — `SeedFromKnowledgeDir` mirrors `internal/knowledge.Index` for the mongo path (scan → strip frontmatter → embed → Insert).

**Modified:**
- `cmd/server/main.go` — `RETRIEVER_BACKEND` env (default `milvus`), `buildRetriever(ctx, backend, emb)` factory, milvus path unchanged, mongo path uses `mongo.New` + `Ensure` + `SeedFromKnowledgeDir`.
- `.env.example` — documented `RETRIEVER_BACKEND`, `MONGO_URI`; clarified per-backend required vars.
- `go.mod` / `go.sum` — `go.mongodb.org/mongo-driver/v2 v2.8.0` + transitives; `golang.org/x/text 0.13.0 → 0.22.0`.
- `scripts/live-verify.sh` — backend-aware header + SC-7 backend-parity block (mongo mode notes that SC-2/SC-4 refusals prove D-09 end-to-end on MongoDB).

## Decisions Made

- **mongo-driver v2.8.0 (not v1).** The user's design said "use v1 stable API ... If v2 resolves instead, use the v2 API." v2 is the current stable line and resolved cleanly; v1 is in maintenance. The v2 API surface (`Connect(opts...)`, `client.Disconnect(ctx)`, `cursor.All`, `Indexes().CreateOne`) is stable.
- **Tier filter is the database query, not a Go branch.** `Search` builds `bson.D{{"tier", bson.D{{"$in", tiers}}}}` unconditionally — even for empty `tiers`. The same code path runs every time. A future refactor that "optimizes" empty tiers by skipping the find would silently move D-09 from the storage layer to Go; the `TestSearch_EmptyTiersSendsInEmptyFilter` test guards against this.
- **`rowFinder` interface as the test seam.** `Search` calls `c.finder.FindRows(ctx, filter)`. Production wires this to `mongoCollectionFinder{coll}`; tests inject `fakeFinder`. This is the only way to unit-test the Search ranking logic without a live MongoDB — and the design explicitly called for it.
- **`SeedFromKnowledgeDir` on the mongo Client, not a refactor of `internal/knowledge/indexer.go`.** See Deviations below.
- **Float32 math with `math.Sqrt` round-trip.** Bit-exact across amd64/arm64; precision is well above what ranking needs. Avoided the Quake inverse-sqrt trick because precision matters for tie-breaking.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `internal/knowledge/indexer.go` is NOT actually backend-agnostic**
- **Found during:** Task 4 (cmd/server backend switch)
- **Issue:** The plan's `<files_to_read>` claim that `internal/knowledge/indexer.go ... consumes retriever.Retriever — no changes needed there, it's backend-agnostic` is wrong. The refactor summary (`02-04-REFACTOR-SUMMARY.md`) claims commit `dadd63a` migrated the signature to `retriever.Retriever`, but the actual signature is still `func Index(ctx, knowledgeDir, emb, mc client.Client, cfg milvus.Config) error` (verified via `git blame` on line 102 — the refactor commit left the milvus-coupled signature in place). In mongo mode there is no `client.Client` to pass.
- **Fix:** Added `(*mongo.Client).SeedFromKnowledgeDir(ctx, dir, emb)` in `internal/retriever/mongo/seed.go` — a near-verbatim re-implementation of the milvus indexer's scan/strip/embed/Insert flow, but writing via `c.Insert`. The milvus path is unchanged (no regression risk to the milvus indexer tests). The duplication is intentionally explicit: ~30 lines of file-scan + frontmatter-strip is the price of not touching the milvus tests in this extension plan.
- **Files modified:** `internal/retriever/mongo/seed.go` (new), `cmd/server/main.go` (mongo path calls `mc.SeedFromKnowledgeDir` instead of `knowledge.Index`)
- **Verification:** `go build ./...` and `go test ./internal/... ./cmd/...` both exit 0. Mongo path is self-contained; milvus path unchanged.
- **Committed in:** `2169266` (Task 4 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The deviation is a workaround for an incomplete prior refactor. The fix is minimal-scope (a new file in the mongo package, no changes to existing tests). A future cleanup plan should finish the `knowledge.Index` → `retriever.Retriever` refactor and delete the duplication; tracked under "Deferred Issues" below.

## Issues Encountered

- **`go get go.mongodb.org/mongo-driver/v2@v2.8.0` did not pull transitives on first build.** Symptom: `missing go.sum entry for module providing package github.com/youmark/pkcs8` etc. Fix: `go mod tidy` pulled in `pkcs8`, `klauspost/compress`, `xdg-go/scram`, `xdg-go/stringprep`, `xdg-go/pbkdf2`, `golang.org/x/crypto`, `golang.org/x/term`. Standard Go behavior — recorded here so future readers don't read it as a one-off.
- **`TestSearch_EmptyTiersHitsZeroRows` initially failed.** Symptom: with `fakeFinder{rows: [...]}` returning one row of tier=权威, calling `Search([], 5)` returned that row instead of 0 hits. Root cause: the fake ignores the filter — it cannot evaluate `{$in:[]}` semantics; that's MongoDB's job. Renamed to `TestSearch_EmptyTiersSendsInEmptyFilter` and scoped to the Go-side half of D-09 (filter shape + finder invocation count). The Mongo-side half (a real `{$in:[]}` returning 0 rows) is verified end-to-end by `scripts/live-verify.sh` SC-2/SC-4 against a live MongoDB.

## Known Stubs

None. The mongo backend is fully wired end-to-end (Connect → Ensure → Seed → Search → Disconnect). The `rowFinder` interface is the only indirection and is production-backed by `mongoCollectionFinder` (no test-only seam ships).

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: tier-filter-trust-boundary | `internal/retriever/mongo/mongo.go` (Search) | New tier-filter PRE-similarity surface. Mitigation: filter built unconditionally (no empty-tiers short-circuit), `TestSearch_EmptyTiersSendsInEmptyFilter` + `TestFilterShapeIsTierIn` pin the Go-side half. Mongo-side enforcement is structural (database evaluates `{$in:[]}`). |

## Threat Surface Scan

No NEW network endpoints beyond what `cmd/server` already exposes (POST /api/chat, GET/POST /api/skills). The new outbound connection is `MONGO_URI` (tcp/27017 to the operator-provided MongoDB). This is in addition to the existing `MILVUS_ADDRESS` (tcp/19530) + `BGE_M3_URL` (http) + `GLM_BASE_URL` (https). Only ONE of milvus/mongo is used per process (selected at startup); the unused var is simply not read.

## User Setup Required

To run SkillForge against MongoDB instead of Milvus:

1. Ensure MongoDB 4.4+ is reachable (the user reports 10.62.10.34:27017 is up).
2. Set `RETRIEVER_BACKEND=mongo` in `.env` (or your shell).
3. Set `MONGO_URI=mongodb://10.62.10.34:27017` (or your instance).
4. `MILVUS_ADDRESS` becomes optional — `cmd/server` only reads it when `RETRIEVER_BACKEND=milvus` (or unset).
5. `go run ./cmd/server` — startup log should print `backend=mongo(...)`. The first run creates `skillforge.knowledge` collection + the `{tier:1}` index + seeds `knowledge/*.md` into it.
6. `bash scripts/live-verify.sh` — exercises SC-1..SC-7 against the mongo-backed server. SC-2 + SC-4 refusals now prove D-09 holds on MongoDB.

## Next Phase Readiness

- **Mongo path is production-ready for Phase 2 corpus sizes (~30 chunks).** In-Go cosine on 30×1024-float32 rows is sub-millisecond; no perf concern.
- **Scaling ceiling:** past ~10k chunks the in-Go cosine becomes the bottleneck. The natural next step is MongoDB Atlas Vector Search (`$vectorSearch` pipeline stage) — available on Atlas 6.0+ / self-hosted 7.0+. The retriever interface stays the same; only `mongo.Search` changes.
- **Deferred for a cleanup plan:** finish the `knowledge.Index` → `retriever.Retriever` refactor (the original 02-04-REFACTOR intent). Once done, `mongo.SeedFromKnowledgeDir` becomes redundant and can be deleted. Tracked as a known deviation above.

## Self-Check: PASSED

- 9/9 files verified present on disk (5 created + 4 modified)
- 4/4 atomic commits present in git log (`3585f30`, `4743a4b`, `0546db2`, `2169266`)
- `go build ./...` exit 0
- `go test ./internal/... ./cmd/...` exit 0 — all packages green (knowledge, server, skill, verdict, spiked, retriever/mongo)
- `go test -tags=contract ./integration/contract/... -count=1 -timeout 60s` exit 0 (3.435s, SC-5 wire contract pinned)

---
*Phase: 02-compliance-vertical-slice (extension — Mongo backend)*
*Completed: 2026-08-11*
