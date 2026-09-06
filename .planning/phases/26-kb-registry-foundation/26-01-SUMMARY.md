---
phase: 26-kb-registry-foundation
plan: 01
subsystem: knowledge-registry
tags: [kb-registry, contract-layer, fail-closed, tdd, kb-m-01, kb-m-03]
requires: []
provides:
  - "internal/kb Registry interface (read surface + create/deactivate) — the single identity source for 26-02..26-09"
  - "internal/kb Store persistence seam (sqlite adapter lands in 26-02)"
  - "closed Role vocabulary + CitableRole citation-whitelist single source (KBM-03)"
  - "ValidateSlug / ValidateNewSlug slug whitelist + MaxSlugLen single home"
  - "LoadSeedFile / LoadSeedRegistry controlled seed loader + knowledge_bases.yaml 4-baseline seed"
affects:
  - internal/skill (26-04 delegates ResolveTiers to kb.Registry.ResolveReads)
  - internal/knowledge + internal/retriever (26-03 registry-driven walk/filter)
  - internal/server (26-05 admin endpoints + annotate/pipeline/knowledge)
  - internal/agent (26-06 citability gate + hitl enum)
  - internal/store/sqlite (26-02 implements kb.Store)
tech-stack:
  added: [] # zero new dependencies (yaml.v3 already in go.mod)
  patterns:
    - "atomic.Pointer copy-on-write snapshot (mirrors internal/skill/registry.go:88-116)"
    - "controlled-file fail-closed loader (mirrors internal/externalauth/views.go)"
    - "sorted deterministic sha256 Signature (mirrors internal/kpi/loader.go:255-260)"
    - "leaf-package narrow-interface discipline (mirrors internal/kpi DatasetLister)"
key-files:
  created:
    - internal/kb/kb.go
    - internal/kb/slug.go
    - internal/kb/seed.go
    - internal/kb/registry.go
    - internal/kb/kb_test.go
    - internal/kb/slug_test.go
    - internal/kb/seed_test.go
    - internal/kb/registry_test.go
    - knowledge_bases.yaml
  modified: []
decisions:
  - "D26-01-A: Registry is the INTERFACE (kb.go, contract layer); the concrete impl is the unexported snapshot — the plan's 'type Registry interface' + 'func New() *Registry' pair is not legal Go"
  - "D26-01-B: the Registry interface carries Create/Deactivate (KBM-02 mutation surface); a read-only registry fails closed with ErrReadOnly"
  - "D26-01-C: ValidateSlug (pattern + data reservation) vs ValidateNewSlug (adds role-name reservation) — the 4 baseline seed slugs stay loadable while creation-time reservation holds"
  - "D26-01-D: draft-role handling asymmetry is intentional: slug reads drop draft (empty default-deny set), role: wildcards error on zero eligible expansion"
  - "D26-01-E: no seed-file size cap — PROJECT.md v1.2 zero-hardcode standard (limits must be config-driven); symlink guard only"
metrics:
  duration: ~18 min
  completed: 2026-09-06
  tasks: 3
  commits: 7
  test_functions: 27
  files: 9
  insertions: 1832
status: complete
---

# Phase 26 Plan 01: KB Registry Foundation Summary

**One-liner:** `internal/kb` leaf package — closed four-value Role vocabulary, slug whitelist with reserved words, atomic copy-on-write Registry with `ResolveReads` as the sole reads choke point (draft dropped, role wildcards expanded, unknown/deactivated fail-closed), Store persistence seam, and a fail-closed controlled seed loader plus the 4-baseline `knowledge_bases.yaml`.

## What Was Built

| Task | Deliverable | Commits |
|------|-------------|---------|
| 1 | Contract layer: `Role` constants + `Roles()`/`IsRole()` + `CitableRole` + `MaxSlugLen` + `Entry` + `Registry` interface + `Store` seam + `ErrReadOnly`; slug whitelist + reserved-word map with named conflict reasons | `213e3d0` (RED), `c8e6366` (GREEN) |
| 2 | `LoadSeedFile` (strict `KnownFields` decode, symlink guard, per-entry closed-set validation naming file+index+slug, absent = only legal degraded state) + `LoadSeedRegistry` + `knowledge_bases.yaml` + the registry read snapshot (`Lookup`/`List`/`ActiveSlugs`/`SlugsByRole`/`RetrievalEligible`/`Signature`) | `4b6dea9` (RED), `ab0fb99` (GREEN) |
| 3 | `ResolveReads` (data: guard, slug top-segment + dedupe, `role:` wildcard over retrieval-eligible members, draft dropped, active-set error lists) + `Create`/`Deactivate` (store write before atomic swap, no delete verb) + read-only and restart-persistence postures | `8e7fd47` (RED), `b6febee` (GREEN), `65e3a7e` (build fix) |

Trust semantics locked by tests (27 test functions, all hermetic on Windows, no CGO):

- **draft never retrievable** — `TestRetrievalEligibleExcludesDraft` + `TestDraftRoleNeverRetrievable` + `TestResolveReadsDraftDropped`: role semantics outrank the entry switch; `RetrievalEligible` is the single arbitration point (A3 codeified).
- **deactivated = invisible** — `TestLookupActiveOnly` / `TestResolveReadsDeactivatedSlug`: `Lookup` fail-closed, skill validation loses the slug, admin `List` still shows it (deactivate, never destroy).
- **unregistered fail-closed with the active set in the message** — `TestResolveReadsUnknownSlug`.
- **controlled seed fail-closed** — malformed YAML, per-field violations, duplicate slugs, unknown top-level keys and missing retrieval switches all fail naming file + entry index + slug; absent file is the one legal degraded state.
- **signature determinism** — `TestSignatureDeterministic`: insertion-order and timestamp insensitive, sensitive to every identity field.
- **switch belongs to ops** — `TestRestartDeactivateStaysDeactivated`: a persisted deactivation survives a registry rebuild.

## Verification (plan `<verify>` commands)

| Command | Result |
|---------|--------|
| `go test ./internal/kb/ -count=1` | PASS (0.4s, 27/27 test functions) |
| `go vet ./internal/kb/` | CLEAN |
| `go build ./...` | Type-checks the whole tree; only `cmd/server` fails at LINK — the pre-existing MSYS2 gcc/duckdb CGO blocker (D15-01-1 deferred item). Verified unrelated: `go list -deps ./cmd/server` contains no `internal/kb`. |
| `knowledge_bases.yaml` in git | TRACKED (`git ls-files --error-unmatch` OK) |

Acceptance criteria from the plan all hold: CN display names appear only in the yaml (`grep -rn "权威\|历史\|个人\|草稿" internal/kb/ --include='*.go'` == 0 — including test files, asserted via code points), one `regexp.MustCompile` + one reserved map in slug.go, `atomic.Pointer` ×2 in registry.go, zero `TierEnum` references, leaf-package import discipline, `go test ./internal/skill/` regression still green.

## Deviations from Plan

**1. [Rule 3 - Blocking] `Registry` cannot be both an interface and a struct — naming resolution**
- **Found during:** Task 1
- **Issue:** The plan's `<interfaces>` block declares `type Registry interface {...}` (kb.go) while Task 3 declares `func New(entries []Entry) *Registry` / `func (r *Registry) Create(...)`. Both cannot exist in one Go package. Downstream plans confirm the interface must win the name: 26-04 passes `reg kb.Registry` everywhere and defines `Deps.KBRegistry kb.Registry`, and 26-05 calls `s.KBRegistry.Create(req)` on that field.
- **Fix:** `Registry` is the interface; the concrete type is the unexported `snapshot` (the research's own Pattern R2 name). `New(entries []Entry) Registry` returns the interface. To keep `Create`/`Deactivate` reachable through `Deps.KBRegistry` (26-05), the interface carries the two mutators, which fail closed with `ErrReadOnly` when no Store is wired — this is also exactly what `TestReadOnlyRegistry` asserts.
- **Action needed by 26-02:** `26-02-PLAN.md` line 69 writes `func Open(ctx, seedPath, st Store) (*Registry, error)`; that must read `(Registry, error)` (or construct the in-package `*snapshot`). One-word adaptation, recorded here so the 26-02 executor does not rediscover it.
- **Commit:** `c8e6366`, `b6febee`

**2. [Rule 1 - Bug] Plan's ValidateSlug accept/reject lists contradict each other on role names**
- **Found during:** Task 1
- **Issue:** `TestValidateSlugAccepts` requires `"authority"`/`"historical"` to pass while `TestValidateSlugRejects` requires the four role names to be rejected. Both cannot hold for one function — and if `ValidateSlug` rejected role names, `LoadSeedFile` could not load the 4 baseline KBs whose slugs *are* the role names (breaking `TestLoadSeedFileGolden` and the whole phase).
- **Fix:** `ValidateSlug` = pattern + the `data` channel-prefix reservation (used by the seed loader and any existing-slug context); new `ValidateNewSlug` = `ValidateSlug` + refusal of the four role names (used only by `Create`). One reserved map carries the human-readable conflict reason for each word; the role-name home stays `Roles()`/`IsRole()`.
- **Commit:** `c8e6366`

**3. [Rule 3 - Blocking] Task 2's verify command needs registry read methods declared as Task 3 artifacts**
- **Found during:** Task 2
- **Issue:** `TestLoadSeedRegistry` / `TestDraftRoleNeverRetrievable` (Task 2 behavior) call `New`, `ActiveSlugs`, `RetrievalEligible` and `Signature`, but the plan assigns those to Task 3.
- **Fix:** Task 2 landed the read half of `registry.go` (snapshot + `New` + five read methods + `Signature`); Task 3 added `ResolveReads`/`Create`/`Deactivate` and widened `New`/`LoadSeedRegistry` from the concrete type to the `Registry` interface (the intermediate concrete return kept each commit honest — no stub methods at any point).
- **Commit:** `ab0fb99`, `b6febee`

**4. [Rule 1 - Bug] `containsString` lived in a test file while production code used it**
- **Found during:** final `go build ./...` gate
- **Issue:** `go test ./internal/kb/` passed because the helper was compiled with the tests, but `go build ./internal/kb/` failed (`undefined: containsString` in `ResolveReads`).
- **Fix:** moved the helper into `registry.go`; re-verified `go build ./...`.
- **Commit:** `65e3a7e`

**5. [Form] Reserved map carries reasons (`map[string]string`) instead of `map[string]bool`**
- The plan asked for the `validMaskKinds` bool-map *form*, but also required every reserved-word error to name its conflict reason. A single reason-carrying map satisfies "exactly one reserved map" without a parallel switch. (`c8e6366`)

**6. [Form] `slugPattern` is composed from `MaxSlugLen` rather than written as a second literal**
- `regexp.MustCompile(fmt.Sprintf(...))` keeps the length ceiling in the single home the plan mandates (26-03 will project it into the Milvus column width); tests pin the pattern/`MaxSlugLen` relationship at the exact 20-char boundary. (`c8e6366`)

**7. [Rule 2 - Scope guard] No seed-file size cap**
- The plan mentioned a "symlink/size guard"; only the symlink guard was implemented. A hardcoded byte limit would be a new hardcode under the PROJECT.md v1.2 zero-hardcode standard (limits must be config- or registry-driven), and the seed is a git-controlled file. Recorded so 26-10's zero-hardcode audit does not flag the absence. (`ab0fb99`)

## Notes for Downstream Plans

- **`kb.Open` (26-02) must return `Registry`, not `*Registry`** — see Deviation 1. It can build the in-package `*snapshot` directly.
- **Draft asymmetry is contract, not accident:** slug reads (`draft/**`) resolve to an *empty, error-free* set (default-deny reaching D-09 at search time); `role:draft` (or any zero-eligible wildcard) is a *load-time error* containing `matches zero active`. 26-04's `ResolveTiers` delegation inherits both.
- **`CitableRole`** is the only citability comparison consumers may write (26-06 gate).
- **Display names are pinned byte-for-byte** by `TestLoadSeedFileGolden` (asserted via code points to keep this package free of CN literals) — the refusal-copy and frozen-eval dependency the plan flagged.
- CN-literal-free Go in this package was achieved *including tests* (the plan's gate as written has no `_test.go` exclusion), so 26-10 can adopt the gate verbatim.

## Known Stubs

None. Every exported symbol has a full implementation; no TODO/FIXME/placeholder was introduced (`grep -rn "TODO\|FIXME" internal/kb/` == 0).

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-01-1 slug traversal/injection | `^[a-z][a-z0-9-]{0,19}$` whitelist (composed from `MaxSlugLen`) + reserved words; `../`, `_`, uppercase, non-ASCII all rejected (`TestValidateSlugRejects`) |
| T-26-01-2 repudiation | deterministic sha256 `Signature()` over sorted identity fields; ready to ride 26-05 audit rows |
| T-26-01-3 seed tampering | `KnownFields(true)` strict decode + symlink guard + closed-set per-entry validation + file/index/slug-naming errors; absent = only degraded state |
| T-26-01-4 draft privilege escalation | double-locked by `TestResolveReadsDraftDropped` + `TestRetrievalEligibleExcludesDraft`; a seed slip cannot make draft searchable |
| T-26-SC package legitimacy | zero new dependencies (go.mod untouched) |

## Self-Check: PASSED

- Files exist: `internal/kb/{kb,slug,seed,registry}.go` + 4 test files + `knowledge_bases.yaml` — all FOUND.
- Commits exist: `213e3d0`, `c8e6366`, `4b6dea9`, `ab0fb99`, `8e7fd47`, `b6febee`, `65e3a7e` — all FOUND in `git log 24ab08a..HEAD`.
- `go test ./internal/kb/ -count=1` PASS; `go vet ./internal/kb/` CLEAN; `go build ./internal/kb/` OK.
