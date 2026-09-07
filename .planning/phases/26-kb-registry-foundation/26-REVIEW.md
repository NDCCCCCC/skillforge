---
phase: 26-kb-registry-foundation
reviewed: 2026-09-07T04:20:00Z
depth: standard
files_reviewed: 34
files_reviewed_list:
  - internal/kb/kb.go
  - internal/kb/open.go
  - internal/kb/registry.go
  - internal/kb/seed.go
  - internal/kb/slug.go
  - internal/store/sqlite/kb_registry.go
  - internal/store/sqlite/schema.go
  - internal/knowledge/walk.go
  - internal/knowledge/indexer.go
  - internal/knowledge/graph.go
  - internal/retriever/milvus/client.go
  - internal/retriever/milvus/config.go
  - internal/retriever/milvus/filter.go
  - internal/retriever/mongo/mongo.go
  - internal/retriever/mongo/rank.go
  - internal/retriever/mongo/seed.go
  - internal/skill/tier.go
  - internal/skill/validate.go
  - internal/skill/manifest_builder.go
  - internal/skill/registry.go
  - internal/skill/loader.go
  - internal/runtime/sub_agent_builder.go
  - internal/runtime/fanout_runtime.go
  - internal/runtime/trust_stage.go
  - internal/runtime/orchestrator.go
  - internal/agent/trust_floor.go
  - internal/agent/hitl_tools.go
  - internal/agent/hitl/hitl_memory_facts.go
  - internal/agent/hitl/registry.go
  - internal/agent/tools/retrieve.go
  - internal/server/kb_admin.go
  - internal/server/annotate.go
  - internal/server/knowledge.go
  - internal/server/pipeline.go
  - internal/server/chat.go
  - internal/server/skills.go
  - internal/server/server.go
  - internal/server/runner_setup.go
  - internal/server/plan_api.go
  - internal/data/rules.go
  - internal/data/importer.go
  - cmd/server/main.go
  - cmd/eval/main.go
  - scripts/verify-phase-26.sh
  - Makefile
  - knowledge_bases.yaml
  - datarules/loans.yaml
  - .env.example
  - eval/frozen/fanout.lock.json
  - docs/threat-model/STRIDE-26.md
findings:
  critical: 4
  warning: 7
  info: 10
  total: 21
status: findings_addressed
fix_round: 2026-09-07T05:20:20Z
fixed: 6
skipped: 0
---

# Phase 26: Code Review Report

**Reviewed:** 2026-09-07T04:20:00Z
**Depth:** standard
**Files Reviewed:** 34 (production scope; ~49 incl. config/docs read for cross-checks)
**Status:** findings_addressed (fix round 2026-09-07: 4/4 Critical + 2/7 Warnings fixed; see Fix Round)

## Summary

The registry core (`internal/kb`) is genuinely well built: copy-on-write snapshots, fail-closed slug validation, structure-vs-switch seed merge (Pitfall 6a/6b both handled), D-09 default-deny in both retriever backends, and the admin surface honors the 403-before-503 check order in all three handlers. `platform_floor.go` is byte-unchanged (last touched at `174ee18`, Phase 14), and `eval/frozen/fanout.lock.json` `eval_set_sha256` recomputes exactly to `sha256(eval/golden/fanout.jsonl)` = `7d74a712…bcbe6a`. The three structural gates pass on this tree (run verified).

However, the phase's central trust claim is overstated by its own wiring. STRIDE-26 T-26-05 asserts the citation whitelist is enforced at "the chunk-insertion points" and that a nil registry is "never a production assembly state". Both claims are false on this tree: a third accumulator (`runtime.SharedChunkAccumulator` / `ApplyTrustStage`) grounds citations with no `kb.Citable` filter, and three production assembly sites (`cmd/server` agent path, `cmd/eval` agent path, `internal/server` ChatHandler `BuildRunner`) never bind the registry. Separately, the Phase 15 import surface is hard-broken in production because `ImporterConfig.KBRegistry` was never wired at boot — `ValidateRules` fails closed on the nil registry and every `loans` import now fails.

Verified-invariant checklist: admin 403-before-503 PASS; public projection minimized PASS; `knowledge_bases` DDL additive + `idxKnowledgeBasesRole` at migrate tail PASS; INSERT-only audit on admin/dir-unregistered rows PASS (but see WR-03 for the boot seed-merge hole); golden lock sha256 PASS; platform floor frozen PASS; gates green PASS (with the WR-04 regex gaps).

## Critical Issues

### CR-01: Production import surface is broken — `ImporterConfig.KBRegistry` never wired at boot

**File:** `cmd/server/main.go:491-494` (contract at `internal/data/importer.go:65-69`, enforcement at `internal/data/rules.go:153-156`, use at `internal/data/importer.go:437`)
**Issue:** Phase 26 made `ValidateRules` fail closed when the registry is nil ("an import rule whose trust tier cannot be checked must not load"), and added `ImporterConfig.KBRegistry` for that purpose. The only production construction site omits the field:

```go
importer = data.NewImporter(data.ImporterConfig{
    SiteSalt: dataSalt,
    RulesDir: datarulesDir,
}, st, lru)
```

`kbReg` is already in scope (built at `main.go:340`), but is not passed. `loadRules` → `LoadRules(path, imp.cfg.KBRegistry)` therefore runs with `nil`, `ValidateRules` returns `"tier … cannot be verified: no knowledge-base registry wired"`, and **every** Preview/Commit/Push/DryRun for the `loans` dataset (the only rule file, `datarules/loans.yaml`) fails in a real boot. Unit tests never catch it because they construct `ImporterConfig{KBRegistry: reg}` directly. This is a regression of a shipped Phase 15 surface introduced by the phase's own config field.
**Fix:**

```go
importer = data.NewImporter(data.ImporterConfig{
    SiteSalt:   dataSalt,
    RulesDir:   datarulesDir,
    KBRegistry: kbReg, // Phase 26 KBM-04: rule-file tier resolves against the live registry
}, st, lru)
```

Add a boot-path test asserting `data.NewImporter` receives a non-nil registry (or a static guard that greps `ImporterConfig{` literals for the field), since the failure mode is a clean 400/500 that looks like a data problem.

### CR-02: Citation whitelist is not enforced on the fan-out path (third insertion point missed)

**File:** `internal/runtime/fanout_runtime.go:208-231` (`SharedChunkAccumulator.Add`), `internal/runtime/trust_stage.go:190-198` (`ApplyTrustStage` chunksByID), `internal/runtime/orchestrator.go:703-706` (`applyTrustStageToMerged` → `chunks = acc.Hits()`)
**Issue:** KBM-03/T-26-05 require that only `kb.CitableRole` chunks enter the grounding closure, enforced at the chunk-insertion points. Phase 26 gated two of the three accumulators (`agent.ChunkAccumulator.Add` via `trust_floor.go:118-158`, and `pipeline.citableChunksByID`). The fan-out shared accumulator is a third insertion point with no gate: `Add(hit)` inserts any hit, `ApplyTrustStage` builds `chunksByID` from all of `in.Chunks`, and the merged-verdict grounding pass consumes `acc.Hits()` unfiltered. A sub-skill whose reads resolve to `historical`/`personal` slugs puts non-authority chunks into the union, and the merged verdict's `[ref-N]` citations are entailment-checked against them — exactly the promotion T-26-05 claims is structurally impossible. Reachability today is latent (the tier-channel `BuildFanOutDispatchTool` is not wired in `BuildRunner`; only `cmd/spike-fanout` and the data-channel variant run it), but the class is the phase's own moat and `kb.Citable`'s godoc ("both the agent-side accumulator gate and the server-side verdict pipeline call it, so the two insertion points cannot drift") is factually incomplete.
**Fix:** give `SharedChunkAccumulator` the same bound registry and filter at insertion, mirroring the agent accumulator:

```go
func (s *SharedChunkAccumulator) SetKBRegistry(reg kb.Registry) { s.kb = reg } // set at orchestrator construction

func (s *SharedChunkAccumulator) Add(hit retriever.Hit) bool {
    if !kb.Citable(s.kb, hit.Tier) {
        return false // non-authority material never enters the grounding union
    }
    ...
}
```

…and add the same `kb.Citable` filter where `ApplyTrustStage` builds `chunksByID` (defense in depth), plus a regression test asserting a `historical`-tier hit never grounds a merged citation.

### CR-03: Nil registry is a real production assembly state — three boot wirings contradict T-26-05

**Files:** `cmd/server/main.go:444-451` (`agent.AgentConfig`), `cmd/eval/main.go:994-1003` (`buildAgent`), `internal/server/runner_setup.go:163-181` (ChatHandler `BuildRunner`)
**Issue:** STRIDE-26 T-26-05 states a nil registry is "the explicit legacy posture (tests/degraded paths), **never a production assembly state**", and `AgentConfig.KBRegistry`'s doc says "the production registry is wired at boot by Plan 26-09". None of the three sites sets the field, although `kbReg` is in scope in all of them (`main.go:340`, `cmd/eval/main.go:512`) and the sibling sites do wire it (`internal/server/approval_api.go:495`, `internal/server/plan_api.go:432`). Consequences:
- `cmd/server` with `USE_AGENT_PATH=true`: `newRunAccumulator` binds nothing, so the citable gate in `ChunkAccumulator.Add` is inert — the production agent path silently runs the pre-26 posture where `historical`/`personal` chunks can be cited (fail-open, trust contract).
- `cmd/eval`: the eval CLI measures the ungated posture, so golden/frozen numbers are calibrated against a different trust contract than the one `approval_api`/`plan_api` serve.
- `internal/server` ChatHandler: a tier-channel skill that declares `writes: [memory:facts]` reaching this path fails hard with `"kb registry required"` (fail-closed, availability), because `BuildToolsForSkill` refuses to build the HITL tool without a registry.

**Fix:** set `KBRegistry: kbReg` in all three constructions (one line each), and add a static guard (source-grep gate or constructor test) that fails when an `AgentConfig{`/`RunnerBuildConfig{`/`ImporterConfig{` literal omits the registry, since every one of these failures is invisible in unit tests.

### CR-04: Shared HITL singletons are mutated per-request without synchronization — data race and cross-session audit mis-attribution

**Files:** `internal/agent/hitl_tools.go:124-128,175-178`, `internal/agent/hitl/registry.go:161-170`, `internal/agent/hitl/hitl_memory_facts.go:202-214,222-226`
**Issue:** `hitlHandlers` entries are package-global singletons, but `BuildRunner` runs per HTTP request (`internal/server/runner_setup.go:163`, `approval_api.go:484`, `plan_api.go:429`). For any skill declaring `memory:facts`, each request executes `hitl.BindKBRegistry(reg)` (writes `t.tierEnum`, `t.defaultTier`) and `mft.WithContext(sessionID, skillID, ruleMatcher)` (writes `t.sessionID`, `t.skillID`, `t.ruleMatcher`) on the same shared instance, while concurrent requests read those fields in `Info()` and `InvokableRun`. The code comments justify this with "BuildToolsForSkill is always called from the single-threaded Agent-construction path" — that is not true on this tree; Hertz serves each request on its own goroutine. Two consequences: (a) a genuine Go data race (`go test -race` on two concurrent `/api/chat` write-skill requests would flag it), and (b) with two in-flight write requests, the last `WithContext` wins, so a `memory_facts_write` interrupt and its `action_pending_approval` audit row can be attributed to the wrong session/skill — an audit-integrity (repudiation) defect. Phase 26 widened the exposure by adding `BindKBRegistry` to this same per-request path.
**Fix:** stop binding per-request state on shared singletons: construct a fresh `MemoryFactsTool` per `BuildToolsForSkill` call (it is a value-like holder), or protect the bindable fields with a mutex and pass `sessionID/skillID` through the tool-call context instead of instance fields. At minimum, snapshot `tierEnum` once at boot and make `WithContext` allocate a per-session clone returned into the whitelist slice.

## Warnings

### WR-01: Deactivating a KB with indexed rows permanently 500s `GET /api/knowledge/graph`

**File:** `internal/knowledge/graph.go:196-198` (consumed at `internal/server/knowledge.go:165-179`)
**Issue:** `Build` errors with `ErrInvalidTier` when any `ListMetadata` row's tier is not an *active* registry slug. KBM-02 deliberately keeps a deactivated base's data on disk/in the vector store and the repo has no delete verb, and the Milvus collection is only cleaned by the one-shot `SKILLFORGE_MILVUS_RECREATE` re-seed (mongo only on next boot re-seed). Therefore the normal admin `POST /api/admin/kb/:slug/deactivate` leaves stale rows that make `Build` return an error, and the whole graph endpoint answers 500 until an operator re-seeds. Deactivation is documented to "hide a base", not break an operator surface.
**Fix:** treat a non-active tier as "skip this row" (or exclude it from the node set) rather than a hard error — e.g. `if _, ok := b.reg.Lookup(m.Tier); !ok { continue }` — and keep `ErrInvalidTier` only for genuinely corrupt rows (empty tier), logging the skipped count. If the strict rejection is intentional, the deactivate handler must trigger/require a re-index and the docs must say so.

### WR-02: Registry signature and entry set are swapped as two independent atomics (torn read)

**File:** `internal/kb/registry.go:30-34,71-79`
**Issue:** `snapshot` stores `entries` and `sig` as two `atomic.Pointer`s. The comment says "swapped together", but `swap` performs two separate `Store` calls, so a concurrent reader can observe the new entry map with the previous signature (or vice versa). `Signature()` is exactly the value audit rows and admin responses carry "so *which registry version produced this answer* stays replayable" — a torn read lets an audit row certify a signature that does not match the entry set that served the answer.
**Fix:** pack both into one immutable struct behind a single pointer:

```go
type snapshotState struct {
    entries map[string]Entry
    sig     string
}
entries atomic.Pointer[snapshotState]
func (s *snapshot) current() *snapshotState { return s.entries.Load() }
```

### WR-03: Boot-time seed-merge structure changes write no audit row, contradicting four contracts

**File:** `internal/kb/open.go:96-128` (upsert sites); contracts at `internal/kb/kb.go:169-172`, `internal/kb/open.go:39-41`, `internal/store/sqlite/schema.go:343-347`, `internal/store/store.go:83-87`
**Issue:** Every contract states "the kb package never writes audit rows; the boot/admin caller records every structure or switch change it drives as an insert-only audit row". `cmd/server` binds audit hooks only for the corpus walk (`main.go:351-352`, `action_kb_dir_unregistered`); nothing records the `action_kb_*` consequences of `kb.Open`'s own writes: a PR that adds a base (`main.go:102` insert), or changes a base's `role`/`display_name`/`description` (`open.go:125` structure-only upsert), silently mutates the trust-bearing registry with zero audit trail. A role flip in the seed is the single most consequential change in the system (it can turn a base citable/non-citable) and it is currently unreplayable — the exact gap the "signature rides every audit row" promise exists to close.
**Fix:** add an optional audit callback to `Open` (the same seam shape as `knowledge.DirAuditFn`), wire it in `buildKBRegistry` to `PutAudit` with actions `action_kb_seed_created` / `action_kb_seed_structure_updated`, and emit only when a row actually changed (the diff guard at `open.go:109` already computes this).

### WR-04: `verify-phase-26.sh` Stage 03 has false-negative gaps (raw strings and `\u` escapes bypass the ban)

**File:** `scripts/verify-phase-26.sh:154-170`
**Issue:** The CN-tier data-value ban only matches ASCII double-quoted exact literals: `grep -E "\"(权威|历史|个人|草稿)\""`. A data value written as a **backtick raw string** (`` tier := `历史` ``) or as **Go unicode escapes** (`"历史"`) is a real data value the gate reports as clean. Verified there are zero such occurrences today (so the gate is not lying on this tree), but the gate certifies a property it does not actually enforce, and the same blind spot applies to future Stage-01/02 migrations. Gate scope is also hardcoded to `internal cmd`, so any new production Go package root is silently unswept.
**Fix:** extend the Stage-03 match to a character class of quote forms — `grep -E "[\"'\x60](权威|历史|个人|草稿)[\"'\x60]"` — and add an explicit probe stage asserting an injected backtick literal is RED (the file already claims D8-04 negative-proof discipline; extend it to the new quote forms). Document the `internal cmd` scope decision in the header next to the whitelists.

### WR-05: Refusal-suggestion helpers have drifted apart while claiming to be logic-equivalent

**Files:** `internal/agent/trust_floor.go:269-308` vs `internal/server/pipeline.go:1059-1070`
**Issue:** `trust_floor.go`'s godoc says it is "Logic-equivalent to the internal/server/pipeline.go helper … so refusal suggestions stay consistent across the old pipeline and the new agent", and that 26-06 moved "that copy" to the registry display name. They are not equivalent: the pipeline version takes `tiers[0]` (a raw slug) first and prepends `keyword + " " + query`, only falling back to the authority display name when tiers are empty; the agent version prefers the authority `DisplayName`, appends `q + " " + keyword`, and falls back to `q + " 制度"`. Same refusal event, two different retry-chip strings depending on which path served it — and the frozen-eval copy assumptions in the comments ("byte-identical to the frozen eval assertions") hold for at most one of them.
**Fix:** keep one implementation. Since `internal/server` already imports `internal/agent`, export the agent version and have `pipeline.go` call it (or move both to a tiny `internal/suggester` package as the TODO note already proposes), then delete the stale "logic-equivalent" claim and re-pin the eval assertion.

### WR-06: Unauthenticated knowledge endpoints serve verbatim content of `personal` and `draft` bases

**Files:** `internal/server/knowledge.go:360-381` (`docContentHandler`), `295-307` (`docsHandler`), `204-245` (`chunkHandler`); routes at `internal/server/server.go:455-458`
**Issue:** `docContentHandler`'s Gate 2 correctly refuses unknown *and deactivated* slugs, but `Lookup` succeeds for any active base — including `personal` ("user-private material") and `draft` ("not-yet-effective", ingestion preview only). Both are in the shipped seed with `retrieval_enabled: true`, and admins can create more. So an unauthenticated caller can enumerate `/api/knowledge/docs`, then read the full markdown of personal/draft corpora via `/api/knowledge/doc?tier=personal&file=…`, and fetch chunk bodies by id via `/api/knowledge/chunk/:id`. The exposure predates Phase 26, but Phase 26 is what made the slug set runtime-open and gave these roles their explicit trust semantics (KBM-03), so the trust model and the route table now disagree. (The `/api/knowledge/bases` projection itself is correctly minimized — no finding there.)
**Fix:** either require the admin token for `personal`/`draft` tiers on the three content endpoints (`if e.Role == kb.RolePersonal || e.Role == kb.RoleDraft { 403 }` after `Lookup`), or record the accepted exposure explicitly in STRIDE-26 with the same rigor as T-26-05-2. Metadata-only endpoints can stay open.

### WR-07: `kb.Store` mutations drop the caller's context entirely

**File:** `internal/kb/registry.go:257-273` (`storeContext()`), used at `registry.go:308,333`
**Issue:** `Create`/`Deactivate` replace the caller's context with `context.Background()` before every store write. The `Registry` verbs therefore cannot honor request deadlines or cancellation: an admin `POST /api/admin/kb` against a wedged sqlite connection blocks for the driver's own timeout with no way for the client or the server to abort it, and any tracing span on the request is not propagated into the write. The godoc acknowledges the choice, but it conflicts with the project's context-propagation convention and is trivially avoidable.
**Fix:** extend the two verbs to `Create(ctx, in)` / `Deactivate(ctx, slug)` and pass the Hertz request context through `kb_admin.go`; fall back to `context.Background()` only in the convenience seed constructor.

## Info

### IN-01: Four hand-rolled int64→decimal renderers

**Files:** `internal/knowledge/graph.go:313-335` (`int64ToChunkID`, comment explicitly avoids `strconv`), `internal/runtime/trust_stage.go:227-243` (`decimalFormat`), `internal/agent/trust_floor.go:209-212` (`chunkKey`), `internal/agent/tools/retrieve.go:219-221` (`ChunkKey`)
**Issue:** Four private copies of `strconv.FormatInt`, two of them with hand-written loop implementations and "avoids an import" justifications.
**Fix:** collapse on `strconv.FormatInt(id, 10)`; keep one shared `chunkKey` helper (the "deferred merge" note in `trust_floor.go` already plans this).

### IN-02: Hand-rolled case-insensitive substring search in the mongo backend

**File:** `internal/retriever/mongo/mongo.go:243-274` (`containsAlreadyExists` + `equalFoldASCII`)
**Issue:** A byte-loop reimplementation of `strings.Contains(strings.ToLower(s), "already exists")` with a "unicode table" justification that does not hold (the string is a fixed ASCII error phrase).
**Fix:** `strings.Contains(strings.ToLower(err.Error()), "already exists")`.

### IN-03: Stale package doc says tier values are still CN characters

**File:** `internal/retriever/mongo/rank.go:31`
**Issue:** The package comment documents `tier string (CN-char tier value: 权威/历史/个人/草稿)` — the exact vocabulary Phase 26 retired. A reader implementing a third backend against this doc would reintroduce the enum.
**Fix:** reword to "tier string (knowledge-base registry slug, e.g. `authority`) — see internal/kb".

### IN-04: `LoadBases` trusts the stored role string without re-validating the closed vocabulary

**File:** `internal/store/sqlite/kb_registry.go:117` (`e.Role = kb.Role(role)`)
**Issue:** A hand-edited or corrupted row with an unknown role loads silently; `RetrievalEligible` would then treat it as searchable (role ≠ draft) while `Citable` refuses it. Defense-in-depth only, since every write path validates.
**Fix:** reject or WARN on `!kb.IsRole(role)` during the scan (the CHECK constraint backstops new writes, not pre-existing rows).

### IN-05: `kb.Create` duplicate-slug race returns 400 instead of 409

**File:** `internal/server/kb_admin.go:202-209` with `internal/kb/registry.go:294-296`
**Issue:** The handler pre-checks duplicates against `List()` and then calls `Create`, which re-checks internally. Two concurrent identical creates both pass the pre-check; the loser surfaces the registry's generic validation error as 400 rather than the documented 409 conflict.
**Fix:** map the registry's "already registered" error to 409 with `errors.Is`/string match, or move the conflict decision entirely into `Create` and return a typed sentinel error.

### IN-06: `ApplyTrustFloor` reads the accumulator's registry field without the mutex

**File:** `internal/agent/trust_floor.go:265` (`ruleBasedSuggestion(acc.kb, …)`) vs `SetKBRegistry` at `104-111` (locked write)
**Issue:** Benign today because production binds at construction, but it is an unlocked read of a field another goroutine may write; `go test -race` would flag it if a test ever binds concurrently.
**Fix:** add a small `registry()` accessor that locks, or set `kb` only in `NewChunkAccumulator` and drop the setter.

### IN-07: Public `/api/knowledge/bases` lists `draft`-role bases as if searchable

**File:** `internal/server/kb_admin.go:302-313` with `internal/kb/registry.go:107-116`
**Issue:** The projection filters on `RetrievalEnabled` only, so the shipped `draft` base (active by seed) is advertised to the FE with `role: "draft"` even though it can never enter a retrieval filter. The badge is misleading rather than a leak.
**Fix:** filter the public projection on `RetrievalEligible()` semantics (`role != draft && RetrievalEnabled`), or have the FE render draft bases as a distinct disabled state.

### IN-08: Seed loader decodes only the first YAML document and has a Lstat/ReadFile TOCTOU

**File:** `internal/kb/seed.go:73-98`
**Issue:** `dec.Decode` reads document one; a second `---`-separated document in the controlled file is silently ignored (an operator could believe a declaration landed). The symlink guard also runs on `Lstat` before `ReadFile`, so a swap between the two syscalls is followed.
**Fix:** loop `Decode` until `io.EOF` and error on any additional document; open the file first (`os.Open` + `Fstat`) and stat the handle, or accept and document the window.

### IN-09: Mongo seed path lacks the size cap and the tier/effective validation the Milvus indexer has

**File:** `internal/retriever/mongo/seed.go:103-131` vs `internal/knowledge/indexer.go:175-181,229-240`
**Issue:** `SeedFromKnowledgeDir` enforces neither `maxDocBytes` nor `validateTiers`/`validateEffective`. The shared walker keeps tiers valid, but an operator dropping a 100 KB file into `knowledge/authority/` is silently embedded and inserted on the mongo backend while the milvus backend warns and skips it — divergent corpus quality between backends.
**Fix:** export `knowledge.MaxDocBytes` + a shared `knowledge.BuildDocs(dir, reg, audit)` helper and have both backends consume it (this also retires the duplicated `stripFrontmatter` at `seed.go:184-198`).

### IN-10: Redundant channel guard in the deep+data branch

**File:** `internal/skill/validate.go:134` (`if m.Agent == "deep" && m.HasDataChannel()`)
**Issue:** The `m.HasDataChannel()` conjunct is always true inside the `if m.HasDataChannel()` branch — dead condition that suggests the author was unsure of the enclosing scope.
**Fix:** `if m.Agent == "deep" { … }`.

---

_Reviewed: 2026-09-07T04:20:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

## Fix Round

**Fixed at:** 2026-09-07T05:20:20Z
**Scope applied:** all 4 Critical findings + WR-01 + WR-02 (the two tractable Warnings). WR-03..WR-07 and IN-01..IN-10 are **open**.
**Worktree:** fixes were applied on an isolated worktree branch (`gsd-reviewfix/26-*`) and fast-forwarded onto `phase-20-p10-6-admin-console`.

| Finding | Outcome | Commit | How |
|---|---|---|---|
| CR-01 | fixed | `e4211c6` | `KBRegistry: kbReg` wired into the boot `data.ImporterConfig` (the only production construction). Added `TestImporterConfigWiresKBRegistry`, a source-grep lock on the literal (the failure is invisible to unit tests). |
| CR-02 | fixed | `adce584` | Third insertion point gated with the single-source predicate `kb.Citable`: `SharedChunkAccumulator.SetKBRegistry` + insertion-time filter in `Add` (dropped hit frees its chunk ID); `TrustStageInput.KBRegistry` + runtime-local `citableChunksByID` for the `ApplyTrustStage` grounding map; `DispatchConfig.KBRegistry` → `FanOutOrchestrator` → bound on every shared accumulator + threaded into both `ApplyTrustStage` call sites; `BuildRunner` forwards the registry. Behavioral test: a merged verdict whose union holds only a historical hit now hits the trust floor (refusal) instead of grounding; control with an authority hit still grounds. |
| CR-03 | fixed | `dc05ca7` | `KBRegistry: kbReg` in `cmd/server`'s `agent.AgentConfig`; `buildAgent(ctx, …, kbReg)` threads it in `cmd/eval` (both call sites); `KBRegistry: s.KBRegistry` in the ChatHandler `BuildRunner` (matching approval_api/plan_api). Grep-level wiring locks added for all three: `TestAgentConfigWiresKBRegistry`, `TestEvalBuildAgentWiresKBRegistry`, `TestChatHandlerBuildRunnerWiresKBRegistry`. |
| CR-04 | fixed (logic changed — human verification recommended) | `629590a` | Request-scoped handlers: new `hitl.HandlerForSession` returns a fresh `MemoryFactsTool` clone per `BuildToolsForSkill` call (`cloneForSession` = value copy + registry rebind + identity), never mutating the shared singletons; `WithContext` deleted; `BindKBRegistry` kept for explicit boot/test construction and documented as off the per-request path. Tests: request-scoping + untouched-singleton assertions, fail-closed nil/unbound registry, and a 16×50 concurrent hammer asserting zero cross-session mis-attribution — clean under `-race`. The T-HITL-07 sha256 body lock was re-ranged 320-422 → 339-441; `InvokableRun`'s body is byte-identical (hash unchanged). |
| WR-01 | fixed | `d9cb8b4` | `GraphBuilder.Build` adopts the walker's three-state semantics: active slug renders; non-active but non-empty tier → skip + one summed WARN (deactivation no longer 500s `GET /api/knowledge/graph`); EMPTY tier stays a hard `ErrInvalidTier`. Two tests rewritten to the new contract + one added for the empty-tier reject. |
| WR-02 | fixed | `d67d8a8` | Registry snapshot packs (entry map, signature) into one immutable `snapshotState` behind a single `atomic.Pointer` — writers store once, readers get a self-consistent pair. `TestSnapshotSignatureAlwaysMatchesEntrySet` hammers `swap` from 4 goroutines while readers recompute the signature of the observed entry set; it reproduces the torn read on the pre-fix two-atomic implementation. |

**Residual notes (not blocking, out of this round's scope):**

- `-race ./internal/agent/` also flags `TestDataChannelFanOut*` through the test fixture `stubKPICaller.Call` (`fanout_tools_test.go:185` appends to a slice from errgroup goroutines without a mutex). Reproduced on the base commit `9054cc4` — pre-existing, unrelated to CR-04.
- `hitl.InjectDeps` still binds the per-request `AuditWriter`/report sink/emit hooks on the shared `plan_approve` + `report:publish` singletons — the same class of race CR-04 fixed for `memory:facts`. The review scoped CR-04 to `BindKBRegistry`/`WithContext`; the remaining seam is documented here for the next round.
- Environmental: this checkout's `core.autocrlf=true` wrote CRLF into tracked `*.jsonl` (not covered by `.gitattributes`), which flipped `TestPlatformFloorLockSelfConsistent` (a raw-byte sha256 of `eval/golden/platform-floor.jsonl`). The working tree was normalized to LF and the test passes; consider `*.jsonl text eol=lf` in `.gitattributes`.

**Gates after the fix round:** `go build ./...` PASS; `go vet ./...` PASS; `go test ./... -count=1` PASS (40 packages, 0 FAIL, WSL); `scripts/verify-phase-26.sh` 3/3 structural gates PASS; platform floor untouched (no commit touches any `platform_floor*.go` — still byte-frozen at `174ee18`).

---

_Fix round: 2026-09-07T05:20:20Z — Claude (gsd-code-fixer)_
