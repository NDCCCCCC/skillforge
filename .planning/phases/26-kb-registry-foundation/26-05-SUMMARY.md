---
phase: 26-kb-registry-foundation
plan: 05
subsystem: server-admin-trust
tags: [kb-registry, admin-surface, insert-only-audit, check-order, public-projection, counter-evidence, tdd, kb-m-02, kb-m-03]
requires:
  - "internal/kb Registry interface carrying Create/Deactivate (26-01)"
  - "kb.Open store-backed registry (26-02) — Create/Deactivate persist, so every mutation owes an audit row"
  - "server Deps.KBRegistry + the 503 fail-closed POST /api/skills posture (26-04)"
provides:
  - "POST /api/admin/kb + POST /api/admin/kb/:slug/deactivate — the token-gated KBM-02 mutation surface with INSERT-only audit rows (action_kb_created / action_kb_deactivated, FrozenArgs = entry snapshot, ActorType=admin)"
  - "GET /api/admin/kb — signature + every entry, deactivated flagged (deactivate ≠ delete: no delete endpoint exists)"
  - "GET /api/knowledge/bases — token-free public projection, active-only, slug/display_name/role only (26-08 FE wire contract)"
  - "counter-evidence pass resolves its retrieval set via kb.Registry.SlugsByRole(RoleHistorical); nil registry non-fatal, empty set constructive D-09 zero-hit"
  - "refusal suggestion keyword derives from the first authority-role base's DisplayName (+fixed suffix); hardcoded keyword deleted"
  - "knowledge.go docs/doc endpoints registry-driven; the 4-value tier maps are gone; traversal gate = kb.ValidateSlug regex + registry Lookup"
affects:
  - web (26-08 consumes GET /api/admin/kb + GET /api/knowledge/bases wire shapes verbatim)
  - internal/agent (26-06 citability gate: the counter-evidence channel's historical set is now registry-sourced)
  - cmd/server (26-09: routes are live the moment Deps.KBRegistry is wired by kb.Open)
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "composite endpoint: kpi_admin.go's read envelope + external_auth.go's POST verbs, gated admin-403-first / nil-registry-503-second"
    - "tests assemble the REAL store-backed registry via kb.Open over a one-entry seed + a 3-method memKBStore (no hand-rolled Registry double, no sqlite)"
    - "fake store.Store via embedded nil interface — unimplemented methods panic on first use (2 lines instead of 200)"
    - "double path-traversal gate: whitelist REGEX (holds for every future slug) + registry membership (unregistered AND deactivated refuse)"
key-files:
  created:
    - internal/server/kb_admin.go
    - internal/server/kb_admin_test.go
    - internal/server/pipeline_tier_test.go
  modified:
    - internal/server/annotate.go
    - internal/server/annotate_test.go
    - internal/server/pipeline.go
    - internal/server/knowledge.go
    - internal/server/knowledge_docs_test.go
    - internal/server/server.go
    - internal/server/skills.go
    - internal/store/store.go
decisions:
  - "D26-05-A: 409/404 decided from List() membership BEFORE the registry call, not by string-matching registry errors — Create/Deactivate failures then map honestly (validation → 400, ErrReadOnly → 503, persistence → 500 generic per W-3)"
  - "D26-05-B: the public projection degrades to 503 on a nil registry rather than an empty 200 — server.go's own Deps.KBRegistry contract says nil degrades KB surfaces fail-closed, and 'no bases' is a different operational fact than 'registry unwired'"
  - "D26-05-C: docsHandler iterates ActiveSlugs() so a draft-role base stays browsable (today's behavior); RetrievalEligible() would have silently dropped it"
  - "D26-05-D: audit rows carry UserID 'admin' + ActorType 'admin' so the existing ADM-06 admin audit sub-view surfaces KB mutations with zero new filter plumbing"
  - "D26-05-E: kbRegistryNotWiredError is the shared const between the new admin surface and the 26-04 /api/skills gate (kills the duplicate string 26-10 would flag)"
metrics:
  duration: ~75 min
  completed: 2026-09-07
  tasks: 3
  commits: 6
  test_functions_added: 20
  files: 11
  insertions: 1257
  deletions: 128
status: complete
---

# Phase 26 Plan 05: KB Admin Surface + Server Trust Surface Summary

**One-liner:** the KB registry gains its deliberate mutation surface — token-gated create/deactivate endpoints where every change lands an INSERT-only audit row and a 403 always precedes the 503 — while the citation moat's inputs (counter-evidence retrieval set, refusal suggestion keyword, docs/doc browsing) all switch their source to the registry with byte-identical wire behavior.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | `kb_admin.go`: GET/POST `/api/admin/kb` + POST `/api/admin/kb/:slug/deactivate` + public GET `/api/knowledge/bases`; 4 routes registered in server.go; `AuditRow.Action` godoc lists all three Phase 26 kb actions | `704c66b` (RED), `2d325d2` (GREEN) |
| 2 | counter-evidence retrieval set = `SlugsByRole(kb.RoleHistorical)` (nil → non-fatal error, empty → constructive zero-hit); `ruleBasedSuggestion(query, tiers, reg)` derives the keyword from the first authority-role base's DisplayName + fixed suffix; hardcoded `[]string{"历史"}` and `"权威制度"` deleted | `f177d0e` (RED), `7cb6ae7` (GREEN) |
| 3 | `validKnowledgeTiers` + `tierDirEn` deleted; docsHandler walks `ActiveSlugs()` (slug = directory name, mapping layer gone); docContentHandler gated by `kb.ValidateSlug` regex + registry `Lookup` | `f58b1c9` (RED), `96730fa` (GREEN) |

Trust semantics locked by the 20 new tests (all hermetic — no sqlite, no live Milvus/Mongo):

- **CHECK ORDER (the headline lock)** — `TestKBAdminCheckOrder`: nil registry + no token answers **403 on all three admin routes**, never 503; the body must not contain "not wired". An unauthenticated caller cannot probe whether the KB registry is configured (T-26-05-2). `TestKBAdminNilRegistry503` is the positive contrast with a valid token.
- **fail-closed token gate** — `TestKBAdminUnsetEnv403`: unset `SKILLFORGE_ADMIN_TOKEN` denies all three admin routes even with a token attached.
- **mutation + audit coupling** — `TestKBAdminCreate201` / `TestKBAdminDeactivate`: exactly one `action_kb_created` / `action_kb_deactivated` row per mutation, `FrozenArgs` carries the entry snapshot, `ActorType: admin`; rejected creates/deactivations write **zero** rows.
- **deactivate, never destroy** — after deactivation the entry is still in `List()` (flagged `active:false`) while `Lookup` refuses it: the skill-validation and retrieval surfaces lose it, the admin view keeps it. `TestKBAdminCreateDuplicate409` proves a **deactivated** slug still conflicts on create (re-creating would reset an identity audit rows reference).
- **author sees the rule** — invalid slugs 400 with the whitelist pattern text; an unknown role 400s naming all four vocabulary values (`TestKBAdminCreateSlugValidation`).
- **information-minimized public surface** — `TestKnowledgeBasesPublic`: no token, active entries only, and the raw payload contains no `description`, no `enabled`, no `created_at`, no deactivated slug.
- **moat untouched by the source swap** — `TestCounterEvidence_NotGrounded` (the chunksByID snapshot) stays green byte-for-byte; `TestCounterEvidenceUsesRegistryRoles` asserts the retrieval set equals the deps' own registry's `SlugsByRole(RoleHistorical)` (no literal in the assertion); `TestCounterEvidenceEmptyRoleZeroHit` pins the constructive empty set (search still runs, empty-state chunk still emits); `TestCounterEvidenceNilRegistryNonFatal` pins the wiring-bug path.
- **open-slug traversal regression** — `TestDocContentHandlerValidatesSlug` + `TestDocContentHandler_PathTraversalRejected`: `tier=../cmd` → 400 on the regex; `tier=ghost` → 400 naming the active set; `tier=authority` → the unchanged happy path; `TestDocContentHandlerDeactivated400` extends the gate to deactivated bases; `TestDocsHandlerEmptyRegistry` makes the empty registry a 200 `{tiers:[]}`, never a 500.

## Verification (plan `<verify>` + `<verification>` commands)

| Command | Result |
|---------|--------|
| WSL `go test ./internal/server/ -run 'TestKBAdmin\|TestKnowledgeBasesPublic' -count=1 -v` | PASS (10/10) |
| WSL `go test ./internal/server/ -run 'TestCounterEvidence\|TestPipeline' -count=1 -v` | PASS (incl. the untouched `TestCounterEvidence_NotGrounded` snapshot) |
| WSL `go test ./internal/server/ -run 'TestDocs\|TestDocContent' -count=1 -v` | PASS (8/8) |
| WSL `go test ./internal/server/ -count=1` | PASS (12.5s, full package) |
| WSL `go test ./... -count=1` | zero FAIL lines repo-wide |
| WSL `go vet ./internal/server/` | CLEAN |
| WSL `go build ./...` | GREEN (exit 0 — the authoritative whole-tree gate) |
| `grep -rn '\[\]string{"历史"}\|权威制度\|validKnowledgeTiers\|tierDirEn' internal/server/ --include='*.go' \| grep -v _test` | **0 hits** |
| `grep -c "isAdminRequest" internal/server/kb_admin.go` | 4 (≥3: one per admin handler + godoc) |
| `grep -rn "action_kb_created\|action_kb_deactivated" internal/ --include='*.go' \| grep -v _test` | only the two `putKBAdminAuditRow` call sites + comments — no UPDATE/DELETE accompanies them |

Note on the plan's "Windows hermetic" wording: `internal/server` imports `store/sqlite` (CGO-tainted), so per the project runbook every suite here ran under WSL Go; Windows stops at the pre-existing D15-01-1 duckdb LINK blocker (26-04's finding, unchanged).

## Deviations from Plan

**1. [Rule 3 - Blocking] `knowledge_docs_test.go` migrated, not just `knowledge_test.go`**
- **Found during:** Task 3
- **Issue:** the docs/doc behavior tests live in `knowledge_docs_test.go` (the harness file with `newKnowledgeTestHertz` + the cwd-to-repo-root chdir), not `knowledge_test.go`. The registry-driven change breaks all five existing tests there (they assert CN tier names and nil-registry deps).
- **Fix:** migrated those five tests onto a `testKBReg()` registry and slug vocabulary, and added the four new behavior tests in the same file; `knowledge_test.go` (graph/chunk handler tests) needed no change.
- **Commit:** `f58b1c9`, `96730fa`

**2. [Rule 3 - Blocking] `ruleBasedSuggestion` signature change rippled to all 4 pipeline call sites**
- **Found during:** Task 2
- **Issue:** threading the registry into the suggestion keyword changes the function signature; pipeline.go has four call sites (lines 279/304/339/505), none of which the plan enumerated.
- **Fix:** all four now pass `s.KBRegistry`. Mechanical, no behavior change at any call site.
- **Commit:** `7cb6ae7`

**3. [Rule 2 - Missing critical functionality] 409/404 decided from `List()` membership, not by string-matching registry errors**
- **Found during:** Task 1
- **Issue:** `Create`/`Deactivate` return plain errors; distinguishing duplicate→409 / unknown→404 by substring match would be fragile and would also mis-map persistence failures onto 4xx.
- **Fix:** membership is checked against `List()` (which includes deactivated entries) before the mutation; remaining `Create` errors are 400 (author-facing rule text), `ErrReadOnly` is 503 (wiring defect, same family as nil registry), and a `Deactivate` persistence failure is a generic 500 with the detail logged (W-3).
- **Commit:** `2d325d2`

**4. [Form] `skills.go` now references the shared `kbRegistryNotWiredError` const**
- The plan names the constant; the identical string already lived inline in the 26-04 `/api/skills` gate. Pointing skills.go at the const removes the duplicate the 26-10 zero-hardcode audit would flag. Zero behavior change (byte-identical string). (`2d325d2`)

**5. [Form] The Task 3 acceptance grep `grep -c "Lookup(" knowledge.go ≥ 2` reads 1**
- Both handlers ARE registry-driven: `docContentHandler` uses `Lookup` (plus `ActiveSlugs()` in its 400 message); `docsHandler` iterates `ActiveSlugs()` — the exact "active bases, ascending" set it needs — rather than `List()` + a manual active filter or a redundant per-slug `Lookup`. The grep's intent (both handlers decide through the registry) holds; the letter does not, because the read that fits docsHandler is `ActiveSlugs`. Recorded rather than writing redundant `Lookup` calls to satisfy a proxy grep. (`96730fa`)

**6. [Form] The plan's `TestKBAdminGetList` + public-projection fixtures ride a store-backed registry, not a hand-rolled `kb.Registry` fake**
- The plan suggested an in-package fake implementing `kb.Registry`. Instead the tests assemble the REAL registry via `kb.Open` over a one-entry temp seed + a 3-method `memKBStore`, so the 409/404/active-only semantics the handlers depend on are the production semantics (a test-local copy could have drifted). Deactivated fixtures are pre-persisted rows the seed does not declare — kb.Open's step 4 keeps them by design. (`704c66b`, `2d325d2`)

**7. [Rule 1 - Consistency] pipeline.go's Step-12 comment still said "历史 tier"**
- The Task 2 literal greps do not cover comments, but the comment vocabulary rule does; the Step-12 counter-evidence description now reads "the registry's historical-role bases". Comment-only. (`7cb6ae7`)

## Notes for Downstream Plans

- **26-06 (agent/hitl):** the counter-evidence channel's retrieval set is now registry-sourced — the citability gate you add at `ChunkAccumulator.Add` should keep comparing against `kb.CitableRole` only; nothing in this plan changed the whitelist itself.
- **26-08 (FE):** the wire shapes are frozen exactly as the plan's `<interfaces>` block wrote them: `GET /api/admin/kb → {signature, bases:[{slug,display_name,role,description,active,created_at,updated_at}]}` (RFC3339 timestamps), `POST /api/admin/kb → 201 entry | 400 {error: rule text} | 409 | 503`, `POST …/deactivate → 200 {slug, active:false}`, `GET /api/knowledge/bases → {bases:[{slug,display_name,role}]}` (active only, no token). Expect **503** (not an empty 200) from every KB surface when the registry is unwired.
- **26-09 (kb.Open wiring):** no cmd changes are needed for this plan — the routes are already registered and go live the moment `Deps.KBRegistry` is wired. `PUT /api/skills`' gate and these handlers share `kbRegistryNotWiredError`.
- **26-10 (zero-hardcode audit):** `internal/server` non-test files now contain zero CN tier vocabulary. Test files legitimately carry CN only where the seed's display name is the fixture (`pipeline_tier_test.go` pins the byte-identical keyword) — the same license `annotate_test.go` already had. Pre-existing Go 1.25 `gofmt -l` drift remains in untouched regions of `server.go`/`pipeline.go`/`knowledge.go`/`store.go` (26-02's documented finding); only files created by this plan were formatted.

## Known Stubs

None. `grep -rn "TODO\|FIXME"` over the eleven touched files returns nothing. The suggestion-keyword suffix const (`suggestionRoleSuffix = "制度"`) is the deliberate surviving half of the old hardcoded keyword, mandated by the plan's "DisplayName + 既有后缀" and locked byte-identical by `TestPipelineSuggestionUsesDisplayName`.

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-05-1 (EoP: KB mutation surface) | all three admin handlers gate on `isAdminRequest` (crypto/subtle, unset env → 403); no delete endpoint anywhere; every mutation writes one INSERT-only row with the entry snapshot in `FrozenArgs` (`TestKBAdminCreate201`, `TestKBAdminDeactivate`, `TestKBAdminUnsetEnv403`) |
| T-26-05-2 (Info disclosure: config probing) | CHECK ORDER locked by `TestKBAdminCheckOrder` (403 precedes 503, body leaks nothing); public projection asserts its own minimization on the raw payload (`TestKnowledgeBasesPublic`) |
| T-26-05-3 (Tampering: tier path traversal) | double gate — `kb.ValidateSlug` regex + registry `Lookup` — with the `tier=../cmd` regression and the deactivated-slug case both pinned (`TestDocContentHandlerValidatesSlug`, `TestDocContentHandlerDeactivated400`) |
| T-26-05-4 (EoP: counter-evidence citation leak) | retrieval set is registry-resolved and role-scoped; the signature still refuses chunksByID and the snapshot regression stays green (`TestCounterEvidenceUsesRegistryRoles`, `TestCounterEvidence_NotGrounded`) |
| T-26-05-5 (Repudiation: audit write failure) | accepted best-effort posture documented at `putKBAdminAuditRow` (log at WARN, never roll back) — consistent with chat.go/runner_setup.go; failures are operator-visible in the server log |
| T-26-SC (dependency legitimacy) | zero new dependencies — `go.mod` untouched |

## Self-Check: PASSED

- Commits exist: `704c66b`, `2d325d2`, `f177d0e`, `7cb6ae7`, `f58b1c9`, `96730fa` — all FOUND in `git log 435c66e..HEAD` (11 files, +1257/−128).
- TDD gate sequence: a `test(26-05)` RED commit precedes each `feat(26-05)` GREEN commit for all three tasks (compile-failure RED for Tasks 1–2's signature-facing tests, behavioral 7-test RED for Task 3).
- Files exist: `internal/server/{kb_admin,kb_admin_test,pipeline_tier_test}.go` created; the eight modified files all appear in `git diff --numstat 435c66e..HEAD`.
- Gates re-verified after the final commit: full `internal/server` suite PASS, repo-wide `go test ./...` zero FAIL, `go vet` clean, `go build ./...` GREEN (WSL), banned-literal grep 0.
