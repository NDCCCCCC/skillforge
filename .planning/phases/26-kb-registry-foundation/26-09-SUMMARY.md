---
phase: 26-kb-registry-foundation
plan: 09
subsystem: boot-assembly + retrieval
tags: [kb-registry, kb-open, production-assembly, provisional-sweep, milvus-recreate, reseed-gate, wsl-gate, tdd, kb-m-01, kb-m-02, kb-m-05, kb-m-06, hyg-03]
requires:
  - "internal/kb Open (seed merge semantics, 26-02)"
  - "SQLiteStore kb.Store adapter + knowledge_bases table (26-02)"
  - "WalkRegisteredDirs + IndexAuditHook/SetIndexAuditHook seam (26-03)"
  - "server Deps.KBRegistry + BuildToolsForSkill reg threading (26-05/26-06)"
provides:
  - "cmd/server PRODUCTION boot order: store -> kb.Open(seed, sqliteStore) -> audit hooks -> skill loader (kb.Open line < skill.NewLoader line, test-locked)"
  - "kbDirAuditHook: INSERT-only PutAudit wrapper bound to knowledge.SetIndexAuditHook + mongo.SetSeedAuditHook"
  - "milvus.RecreateCollection + ShouldRecreate/RecreateEnvVar: the SKILLFORGE_MILVUS_RECREATE one-shot drop+re-seed door, fail-closed by default"
  - "PROVISIONAL (26-0*) seed snapshots deleted repo-wide (cmd/server, cmd/eval, cmd/spike-{history,fanout,deep,hitl}); grep == 0"
  - "WSL full-gate evidence baseline (HYG-03 BE): check-cgo + build + vet + go test ./... all green"
affects:
  - "26-10 (operator checkpoint): the live re-seed execution + closeout consumes this plan's gate and evidence"
  - "operators: SKILLFORGE_KB_SEED / SKILLFORGE_MILVUS_RECREATE are new documented env surface (.env.example)"
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "assembly helper extracted from main() following the buildStore/buildRetriever split convention, test-locked on boot ORDER by line position"
    - "nil-embedded-interface fake as an INSERT-only lock (any non-PutAudit store call panics the test)"
    - "conservative env gate: literal \"1\" only, because the destructive path costs a full re-embed"
    - "transient :memory: registry store for measurement harnesses (never mutate the deployment's persisted registry)"
key-files:
  created:
    - cmd/server/main_test.go
    - internal/retriever/milvus/client_recreate_test.go
  modified:
    - cmd/server/main.go
    - cmd/eval/main.go
    - cmd/spike-history/main.go
    - cmd/spike-fanout/main.go
    - cmd/spike-deep/main.go
    - cmd/spike-hitl/main.go
    - internal/retriever/milvus/client.go
    - internal/retriever/milvus/config.go
    - .env.example
decisions:
  - "D26-09-A: buildKBRegistry(ctx, seedPath, st store.Store) asserts kb.Store in-function instead of taking a kb.Store parameter — buildStore keeps its wide store.Store signature and a wrong adapter becomes a named boot failure, not a compile-time-only guarantee"
  - "D26-09-B: the empty-registry boot line is [WARN], not the plan sketch's [OK] — T-26-09-1's mitigation says an empty registry must be an explicit WARN; logging [OK] for the state that fails every tier Skill would be a log-level lie"
  - "D26-09-C: the audit wrapper passes the walk detail to FrozenArgs VERBATIM — the plan's jsonMarshal(detail) sketch would double-encode the walker's already-compact JSON into a quoted string"
  - "D26-09-D: CLI binaries assemble through kb.Open too (orchestrator assembly contract, zero PROVISIONAL in cmd/): cmd/eval over the deployment's PERSISTENT store (eval must measure the registry production serves), the four spikes over a TRANSIENT :memory: store (a harness must neither mutate the persisted registry nor depend on the cwd's DB — the spike-history legacy-ALTER concern)"
  - "D26-09-E: one registry per process — spike-deep hoists kbReg shared by loader + hitl tool construction; spike-history shares it between buildRetriever and firstManifest; spike-fanout's corpus path uses the same assembly as its loader"
  - "D26-09-F: RecreateCollection keeps the plan's (ctx, collection) signature but rejects an empty name before any call — the drop target must be explicit, never a silent default"
metrics:
  duration: ~35 min
  completed: 2026-09-07
  tasks: 2
  commits: 6
  test_functions_added: 11
  files: 11
  insertions: 1100
  deletions: 191
status: complete
---

# Phase 26 Plan 09: Production Assembly + Re-Seed Gate Summary

**One-liner:** cmd/server boots the knowledge surface from `kb.Open(seed, sqliteStore)` in a provable store → registry → audit-hooks → loader order (admin mutations now survive a restart), every unknown knowledge dir lands as an INSERT-only row in the real audit table, the five CLI binaries' PROVISIONAL seed snapshots are gone in favor of the same production assembly, and a conservative `SKILLFORGE_MILVUS_RECREATE=1` one-shot door can drop + re-seed a legacy-CN-tier vector library — all proven by a fully green WSL gate (`make check-cgo` + `go build` + `go vet` + `go test ./...`).

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `buildKBRegistry` (kb.Open over the 26-02 SQLiteStore, `kb.Store` assertion, absent-seed degraded state + WARN, malformed-seed fatal naming the file) + boot reorder (store → registry → `kbDirAuditHook` bound to both corpus seams → loader) + `cmd/server/main_test.go` (7 tests) | `b952c8d` (RED), `8856602` (GREEN) |
| 1+ | PROVISIONAL sweep: `cmd/eval`, `cmd/spike-history`, `cmd/spike-fanout`, `cmd/spike-deep`, `cmd/spike-hitl` all route through `kb.Open` (`dd65cef`) | `dd65cef` |
| 2 | `milvus.RecreateCollection` (drop → EnsureCollection, index rebuilt) + `ShouldRecreate`/`RecreateEnvVar` (literal `"1"` only) + `buildRetriever` gate read BEFORE `Ensure`/`Index` + `.env.example` docs + 4 hermetic tests + cmd/server wiring lock | `ea4e3fc` (RED), `2c0be51` (GREEN), `fc0cf39` (wiring lock) |

### Task 1 — production assembly (KBM-01/02/05/06)

Boot order is now the 26-02 target sequence, and it is **provable** (`TestBootAssemblyOrder` reads `main.go` and asserts offsets):

```
buildStore (line 323) → buildKBRegistry (335) → SetIndexAuditHook (346) → skill.NewLoader (352) → … → buildRetriever
```

- `buildKBRegistry(ctx, seedPath, st)` = `kb.Open` over the SQLiteStore (the compile-time `var _ kb.Store = (*SQLiteStore)(nil)` assertion is what makes the adapter claim true). Absent seed over an empty table = the one legal degraded state, announced with a WARN naming the seed path and the consequence (`TestEmptyRegistryBootsWithWarning`). A malformed seed returns an error naming the file → `log.Fatalf` (`TestBuildKBRegistryFromStore/malformed…`).
- `Deps.KBRegistry` carries the ONE store-backed registry consumed by the loader, the skill registry, the wizard, `knowledge.New`, and 26-05's admin endpoints. `TestBuildKBRegistryFromStore/deactivation_survives_a_reopen` deactivates `personal`, reopens the registry over the same store, and fails if the seed resurrected the switch (Pitfall 6a).
- `kbDirAuditHook(ctx, st)` is the INSERT-only wrapper: one `PutAudit` row per decision, action enum in `Action`, walk detail in `FrozenArgs`, `SessionID/SkillID/Query` empty. Bound to **both** `knowledge.SetIndexAuditHook` and `mongo.SetSeedAuditHook` before the retriever exists, so the boot-time corpus index writes into the real table (`TestAuditHookWritesInsertOnly` on a nil-embedded fake where any non-PutAudit call panics; `TestAuditHookLandsInSqliteAuditTable` reads the row back through `QueryAudit`).

### Task 1+ — PROVISIONAL sweep (orchestrator assembly contract)

`grep -rn "PROVISIONAL (26-0" cmd/ internal/` == **0** (the only remaining `PROVISIONAL` literal in the repo is the assertion string inside the regression lock itself).

| Binary | Assembly |
|--------|----------|
| `cmd/eval` | store hoisted ahead of the registry; `kb.Open` over the **persistent** deployment store (`buildStore` now returns `*sqlite.SQLiteStore` for the `kb.Store` seam) |
| `cmd/spike-history` | `openKBRegistry(ctx)` hoisted once; shared by `buildRetriever` and `firstManifest` |
| `cmd/spike-fanout` | same helper; the milvus corpus path now uses the same assembly as the loader path |
| `cmd/spike-deep` | same helper, hoisted and shared by `loadManifestLive` + hitl tool construction |
| `cmd/spike-hitl` | `smokeKBRegistry` = `kb.Open` over a transient single-connection `:memory:` store (same posture as its checkpoint store), cwd-upward seed lookup preserved |

### Task 2 — re-seed gate (Pitfall 1a) + WSL full gate

- `milvus.RecreateCollection(ctx, collection)`: `DropCollection` → `EnsureCollection` (schema + IVF_FLAT index). Empty collection name rejected before any call.
- `milvus.ShouldRecreate(envValue)` / `milvus.RecreateEnvVar = "SKILLFORGE_MILVUS_RECREATE"`: opens on the literal `"1"` **only** — `"true"`, `"yes"`, `"on"`, any casing/padding, and unset all stay closed (`TestRecreateEnvGateParsing` covers 16 closed values). The cost asymmetry is the reason: a missed re-seed is a documented step undone; a surprise re-seed wipes a corpus.
- `buildRetriever`'s milvus branch reads the gate **before** `mc.Ensure` and `knowledge.Index`, WARNs naming the collection, the env var and the re-embed cost, and lets the boot-time index re-seed the empty collection. `TestDropOnlyOnRecreatePath` proves `Ensure` alone records zero drops; `grep -rn DropCollection internal/retriever/milvus/ | grep -v _test` hits only the `RecreateCollection` body.
- **No live Milvus/BGE call was made** — the gate is hermetically tested against a mock `client.Client` (nil-embedded interface, so any unexpected SDK call panics the test). The live re-seed execution is 26-10's operator checkpoint.
- `cmd/server` does not construct a `BuildRunner` (it wires `adk.Runner` + VerdictPipeline + `Deps`), so the 26-06 BuildRunner injection chain has no cmd/server site; `Deps.KBRegistry` is the single injection point here. No gap.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking/scope] CLI PROVISIONAL sweep (5 binaries)**
- **Found during:** Task 1
- **Issue:** the plan's `files_modified` listed only `cmd/server/*` and the milvus pair, but the orchestrator's ASSEMBLY CONTRACT requires `grep -rn "PROVISIONAL (26-0" cmd/ internal/` == 0; 26-03/26-04/26-06 had left nine snapshot sites across `cmd/eval` + four spike binaries (D26-04-C kept them greppable *for this plan's* sweep).
- **Fix:** every site now assembles through the production `kb.Open` over the 26-02 SQLiteStore (`dd65cef`); per-binary store choices recorded as D26-09-D/E.
- **Files modified:** cmd/eval/main.go, cmd/spike-{history,fanout,deep,hitl}/main.go
- **Commit:** `dd65cef`

**2. [Rule 1 - Bug] Boot-order lock false-positive from a doc comment**
- **Found during:** Task 1 verification
- **Issue:** `TestBootAssemblyOrder` matched `skill.NewLoader(` in the package doc comment, which precedes any code.
- **Fix:** order assertions now search from `func main() {` onward; presence checks stay file-wide.
- **Commit:** `8856602`

**3. [Rule 1 - Bug] Gate-order lock false-positive from the mongo branch**
- **Found during:** Task 2 verification
- **Issue:** `mc.Ensure(ctx)` first appears in `buildRetriever`'s mongo branch, so the "gate read precedes Ensure" assertion compared against the wrong branch.
- **Fix:** the lock is scoped to the milvus branch (`milvusAddr := envRequired(` onward).
- **Commit:** `fc0cf39`

**4. [Decision] Plan-sketch corrections (documented, not user-blocking)**
- Empty-registry log level `[WARN]` over the sketch's `[OK]` (D26-09-B, threat model T-26-09-1 outranks the interface sketch).
- `FrozenArgs` gets the walk detail verbatim instead of `jsonMarshal(detail)` (D26-09-C, avoids double-encoded JSON in the audit row).
- `RecreateEnvVar` const + `ShouldRecreate` pure function added so the read point and the gate cannot drift (the plan's grep gate ≥ 2 satisfied: main.go read point + config.go gate + .env.example doc).

### Environment notes (not deviations)

- `go test ./cmd/server/` and `./internal/server/` cannot link on this host's Windows MSYS2 gcc (go-duckdb, the pre-existing D15-01-1 limit). Both run **green under WSL** in the full gate below. Windows-hermetic packages from the plan's verification list (`internal/kb`, `internal/skill`, `internal/knowledge`) plus `internal/externalauth`, `internal/nl2sql` are green on Windows.
- State/ROADMAP/REQUIREMENTS bookkeeping intentionally not touched per orchestrator constraint; requirements KBM-01/02/05/06 evidence lives here for 26-10's closeout.

## Verification

Automated evidence (all commands run 2026-09-07):

**WSL full gate (Task 2 verification subject, HYG-03 BE evidence):**

```
$ wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && make check-cgo && go build ./... \
    && go vet ./internal/... ./cmd/... && echo WSL-BUILD-VET-OK"
gcc (Ubuntu 11.4.0-1ubuntu1~22.04.3) 11.4.0
WSL-BUILD-VET-OK

$ wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go test ./... -count=1"
ok  cmd/eval 0.087s            ok  cmd/server 0.106s        ok  cmd/spike-deep 21.052s
ok  cmd/spike-fanout 0.240s    ok  cmd/spike-history 0.043s ok  cmd/spike-hitl 0.108s
ok  internal/agent 5.581s      ok  internal/agent/callbacks ok  internal/agent/hitl 2.612s
ok  internal/agent/tools       ok  internal/auditchain      ok  internal/auditprov
ok  internal/data 4.147s       ok  internal/duckdb 11.535s  ok  internal/embedder/bge 2.352s
ok  internal/eval 0.792s       ok  internal/externalauth    ok  internal/history
ok  internal/kb 0.048s         ok  internal/knowledge 0.057s ok internal/kpi
ok  internal/kpi/wsl 4.240s    ok  internal/llm             ok  internal/nl2sql
ok  internal/platformfloor     ok  internal/report          ok  internal/retriever/milvus
ok  internal/retriever/mongo   ok  internal/runtime         ok  internal/scheduler
ok  internal/server 14.646s    ok  internal/server/classifier ok internal/server/rewrite
ok  internal/skill 6.784s      ok  internal/store           ok  internal/store/sqlite 6.868s
ok  internal/verdict           ok  internal/wire            ok  scripts/eval_snapshot
ok  scripts/spiked
(45 packages: 39 ok, 6 no-test-files, 0 FAIL, 0 skip)
```

**Windows hermetic gate:**

```
$ go test ./internal/externalauth/ ./internal/nl2sql/ ./internal/skill/ ./internal/kb/ -count=1
ok  internal/externalauth 0.513s   ok  internal/nl2sql 0.136s
ok  internal/skill 7.713s          ok  internal/kb 0.481s
$ go test ./internal/kb/ ./internal/skill/ ./internal/knowledge/ -count=1   (server: WSL, see D15-01-1)
ok  internal/skill 8.003s          ok  internal/knowledge 0.410s
```

**cmd/server suite (WSL, CGO):**

```
--- PASS: TestBuildKBRegistryFromStore (4 subtests incl. deactivation-survives-reopen)
--- PASS: TestAuditHookWritesInsertOnly / TestAuditHookLandsInSqliteAuditTable
--- PASS: TestEmptyRegistryBootsWithWarning / TestBootAssemblyOrder
--- PASS: TestKBSeedEnvDefault / TestMilvusRecreateGateWiring
ok  github.com/skillforge/skillforge/cmd/server
```

**Structural greps:**

```
grep -rn "PROVISIONAL (26-0" cmd/ internal/ --include=*.go   → 0
grep -c "PROVISIONAL" cmd/server/main.go                     → 0
grep -n "kb.Open(" cmd/server/main.go                        → line 206 (< skill.NewLoader 352)
grep -n "IndexAuditHook" cmd/server/main.go                  → line 346 (PutAudit wrapper)
grep -n "SKILLFORGE_MILVUS_RECREATE" cmd/server/main.go internal/retriever/milvus/*.go .env.example → 8 (read point + gate + docs)
grep -rn "DropCollection" internal/retriever/milvus/ | grep -v _test → RecreateCollection body only (client.go:123-124)
```

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-09-1 (DoS: assembly order / empty-registry black hole) | order test-locked by line position; empty registry = explicit WARN naming seed path + consequence; malformed seed = `log.Fatalf` naming the file |
| T-26-09-2 (Tampering: RECREATE misfire) | literal-`"1"` gate (16 closed values tested) + WARN receipt + drop exists only on the recreate path; one-shot semantics documented in code + `.env.example` |
| T-26-09-3 (Repudiation: audit loss) | `kbDirAuditHook` → real `PutAudit` INSERT-only row, bound before any corpus walk, readable back through `QueryAudit`; write failure logs `[ERROR]` (best-effort, never silent) |
| T-26-09-4 (live migration on unreachable stack) | accepted/deferred to 26-10: no live Milvus/BGE call made in this plan; capability + hermetic evidence only |
| T-26-SC (dependency install) | accepted: zero new dependencies |

## Known Stubs

None. No placeholder, TODO, or unwired data path was introduced; the re-seed door is real code gated behind an explicit operator env var, and its live execution is deliberately 26-10's operator checkpoint (not a stub).

## Self-Check: PASSED

- Files: `cmd/server/main.go`, `cmd/server/main_test.go`, `cmd/eval/main.go`, `cmd/spike-{history,fanout,deep,hitl}/main.go`, `internal/retriever/milvus/{client,config}.go`, `internal/retriever/milvus/client_recreate_test.go`, `.env.example` — all present in `git diff --stat 6af7f50..HEAD` (11 files, +1100/−191, zero tracked-file deletions).
- Commits: `b952c8d`, `8856602`, `dd65cef`, `ea4e3fc`, `2c0be51`, `fc0cf39` — all present in `git log`.
- Gates: WSL check-cgo/build/vet/test all exit 0; marker grep == 0; re-seed gate closed by default.
