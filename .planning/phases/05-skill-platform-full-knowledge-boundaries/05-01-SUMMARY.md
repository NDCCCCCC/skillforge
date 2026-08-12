---
phase: 05-skill-platform-full-knowledge-boundaries
plan: 01
subsystem: skill-platform
tags: [go, fsnotify, atomic-pointer, hot-reload, wire-contract, uimessagechunk, symlink-guard]

# Dependency graph
requires:
  - phase: 02-compliance-vertical-slice
    provides: Loader.Scan + Manifest struct + 7-field validator + tier resolver
  - phase: 03-agentic-retrieval-trust-evaluation
    provides: Agent D16 decision emit pattern + Embedder interface
  - phase: 04-streaming-trust-ui
    provides: wire.UIMessageChunk + 17 wire types + SkillForgeChunk TS mirror + D15 drift alarm
provides:
  - "DecisionPayload + SkillScores/Selected/Ambiguous (additive, omitempty) — D5-04 routing trace"
  - "data-redline wire type (18th, ValidTypes 17->18) + RedlinePayload struct + DisallowUnknownFields — D5-14"
  - "Skill Registry with atomic.Pointer[map[string]*Entry] snapshot, copy-on-write, no reader mutex — D5-06"
  - "fsnotify dir-watch + 1s debounce + V12 ASVS symlink re-check on every reload — D5-07/D5-08/SC-2"
  - "Manifest.DescriptionVec cached embedding (json:\"-\", in-memory only) + SetDescriptionVec setter — D5-01"
  - "parseAndValidate(path) shared by Loader.Scan and Registry.reloadOne (refactor, no API change)"
  - "fsnotify promoted to direct require in go.mod"
affects:
  - "05-02: router uses Registry.Snapshot() + DescriptionVec for cosine scoring"
  - "05-02: pipeline.go consumes DecisionPayload.SkillScores for SC-1 trace"
  - "05-02: pipeline.go conditional redline pass emits data-redline chunks"
  - "05-04: wizard writes SKILL.md -> fsnotify picks it up (closed loop)"

# Tech tracking
tech-stack:
  added:
    - "github.com/fsnotify/fsnotify v1.5.4 (promoted from indirect to direct)"
  patterns:
    - "atomic.Pointer[map[string]*Entry] for lock-free snapshot reads with copy-on-write writes"
    - "parseAndValidate(path) extraction: parse + V12 symlink guard + Validate + ResolveTiers shared by Scan + reloadOne"
    - "1s debounce via time.AfterFunc (canonical fsnotify #553 pattern)"
    - "Dir-level fsnotify watch (NOT per-file — atomic save+rename pitfall)"
    - "Watcher must be created + Add before any directory iteration (Windows fsnotify pitfall)"
    - "D5-08 invariant: validation failure MUST NOT mutate snapshot + MUST call LoadError hook"
    - "additive omitempty wire fields for backward compat (D5-04)"

key-files:
  created:
    - "internal/wire/redline.go — RedlinePayload struct (required fields, no omitempty)"
    - "internal/skill/registry.go — Registry + Entry types, atomic.Pointer snapshot, fsnotify watcher, debounce, reloadOne, V12 symlink re-check"
    - "internal/skill/registry_test.go — 7 tests (snapshot atomic, debounce, validation-failure-keeps-old, symlink-escape, parallel readers, DescriptionVec not serialized)"
  modified:
    - "internal/wire/chunk.go — DecisionPayload +SkillScores/+Selected/+Ambiguous; ValidTypes 17->18 with data-redline"
    - "internal/wire/chunk_test.go — ValidTypes count test (18), DecisionPayload round-trip tests, RedlinePayload required-fields test"
    - "internal/skill/manifest.go — DescriptionVec []float32 (json:\"-\") + SetDescriptionVec setter"
    - "internal/skill/loader.go — parseAndValidate extracted from Scan (refactor)"
    - "web/src/types/wire-types.ts — DecisionPayload +skillScores/+ambiguous; decision union + 'route'|'counter_evidence'; SkillForgeChunk + data-redline variant; _driftCheck preserved"
    - "go.mod — fsnotify promoted from indirect to direct"

key-decisions:
  - "Per-file fsnotify watches abandoned (Pitfall 1): atomic save+rename leaves file watches dangling. Watch the skills DIR; filter events by suffix."
  - "Watcher.Add() must happen BEFORE any directory iteration (Windows pitfall discovered during implementation): otherwise ReadDirectoryChangesW silently drops subsequent events. New() creates watcher + Add, then startupScan runs."
  - "atomic.Pointer[map[string]*Entry] pattern: Store(&nextMap) takes pointer to new map; *Load() dereferences. Copy-on-write: clone the previous map, mutate, Store."
  - "Embedder errors are NON-FATAL during reload: log + reuse prior DescVec (D5-08 forbids silent-drop, and a temporarily-stale vector is preferable to vanishing a Skill)."
  - "Symlink guard runs on EVERY reloadOne (not just startup Scan): atomic save+rename can replace a real SKILL.md with a symlink to /etc/passwd between Scan and the next read. parseAndValidate encapsulates the Lstat + EvalSymlinks check."
  - "Wire field additivity (D5-04): SkillScores/Selected/Ambiguous use omitempty so the original 5-field DecisionPayload still unmarshals — backward-compatible with Phase-3/4 consumers."
  - "data-redline (D5-14) uses Required fields (no omitempty) for originalClause/proposedRewrite/rationale: V8 mitigation per UI-SPEC §Registry Safety (DisallowUnknownFields in the test)."

patterns-established:
  - "atomic.Pointer copy-on-write snapshot for in-memory read-mostly registries"
  - "Shared parse pipeline helper (parseAndValidate) refactored from startup Scan so reload paths reuse the validation gate"
  - "LoadError hook for surfacing watcher rejections to the UI (SC-2 toast) without the watcher needing SSE/HTTP knowledge"
  - "Wire chunk test pattern: round-trip + unknown-field rejection + ValidTypes count via grep (not comment)"

requirements-completed: [SYSK-04, CMPL-07, CMPL-08]

# Metrics
duration: 30min
completed: 2026-08-12
---

# Phase 5 Plan 1: Wire + Skill Registry Summary

**DecisionPayload routing fields + 18th wire type (data-redline) + atomic-snapshot Skill Registry with fsnotify hot-reload and V12 symlink re-check**

## Performance

- **Duration:** ~30 min (Task 2 only — Task 1 already merged at `1741b9b`)
- **Started:** 2026-08-12T09:15:04Z
- **Completed:** 2026-08-12T09:44:54Z
- **Tasks:** 2/2 (Task 1 pre-merged by orchestrator; Task 2 executed here)
- **Files modified:** 9 (Task 2: 4 Go + 1 go.mod + 1 TS; Task 1 contributed 4 Go + 1 TS already merged)

## Accomplishments

- **Wire contract extended (Task 1, already merged):** `DecisionPayload` carries additive `skillScores[]` + `selected` + `ambiguous` fields (D5-04, omitempty for backward compat). `ValidTypes` grew from 17 to 18, adding `data-redline` (D5-14). TS `SkillForgeChunk` union gains `data-redline` variant; `_driftCheck` (D15 drift alarm) remains true.
- **Skill Registry (Task 2):** `Registry` type with `atomic.Pointer[map[string]*Entry]` for lock-free snapshot reads; copy-on-write on every successful reload. `New(skillsDir, embedder)` creates the watcher + `Add` BEFORE `startupScan` (Windows pitfall discovered during implementation — see Deviations #1) then seeds the snapshot. `Watch(ctx)` spawns the consumer goroutine; `Snapshot()` is lock-free; `reloadOne(path)` re-runs V12 symlink guard + Validate + embed and atomically swaps the map.
- **Manifest.DescriptionVec** field added with `json:"-"` tag (in-memory cache only) + `SetDescriptionVec` setter (audit-friendly production path). `parseAndValidate(path)` extracted from `Loader.Scan` so both the startup scan and `reloadOne` share the same parse+symlink-check+validate+tiers pipeline.
- **fsnotify promoted** from indirect to direct require in go.mod (T-5-SC mitigation: package legitimacy verified by `go list -m` returning v1.5.4 already present in the lockfile).
- **D5-08 invariant tested:** `TestRegistry_Reload_ValidationFailure_KeepsOld` asserts that a missing-temperature SKILL.md triggers the `LoadError` hook AND leaves the snapshot's description+version unchanged.
- **V12 ASVS re-check tested:** `TestRegistry_Reload_SymlinkEscape_Rejected` (skipped on Windows where symlink creation requires Developer Mode) replaces SKILL.md with a symlink to outside the skills root and asserts the old version is retained + hook called.
- **No regressions:** full `go test ./...` passes (`internal/agent`, `internal/eval`, `internal/knowledge`, `internal/retriever/mongo`, `internal/server`, `internal/skill`, `internal/store`, `internal/verdict`, `internal/wire`, `cmd/spiked`).

## Task Commits

Task 1 (already merged by orchestrator at `1741b9b`):
1. **Task 1 RED:** `56288bd` — test(05-01): add failing tests for DecisionPayload routing fields + data-redline wire type
2. **Task 1 GREEN:** `1741b9b` — feat(05-01): extend wire with DecisionPayload routing fields + data-redline type

Task 2 (executed in this session):
3. **Task 2 RED:** `e3911d1` — test(05-01): add failing tests for Skill Registry + Manifest.DescriptionVec
4. **Task 2 refactor:** `1bcdc4e` — refactor(05-01): extract parseAndValidate helper from Loader.Scan
5. **Task 2 GREEN:** `e3be9e1` — feat(05-01): implement Skill Registry with atomic.Pointer snapshot + fsnotify

## Files Created/Modified

### Task 1 (merged at 1741b9b)
- `internal/wire/chunk.go` — DecisionPayload +SkillScores/+Selected/+Ambiguous; `ValidTypes["data-redline"] = true` (the 18th key)
- `internal/wire/redline.go` — new file: `RedlinePayload { ClauseID, OriginalClause, ProposedRewrite, Rationale }` with required fields (no omitempty)
- `internal/wire/chunk_test.go` — extended: `TestValidTypes_Eighteen` (counts 18), `TestDecisionPayload_RouteFields` (additive fields round-trip), `TestDecisionPayload_BackwardCompatible` (original 5-field unmarshals), `TestRedlinePayload_RequiredFields` (missing-field error), `TestRedlinePayload_DisallowUnknownFields` (V8 mitigation)
- `web/src/types/wire-types.ts` — DecisionPayload additive fields; decision union gains `'route' | 'counter_evidence'`; `SkillForgeChunk` gains `data-redline` variant; `_driftCheck` still true

### Task 2 (this session)
- `internal/skill/registry.go` — new file: `Registry`, `Entry`, `ReloadDebounce = 1s`, `New`, `Snapshot`, `Watch`, `watchLoop`, `resolveSkillPath`, `scheduleReload`, `reloadOne`, `startupScan`, `SetLoadErrorHook`, `skillNameFromPath`
- `internal/skill/registry_test.go` — new file: `TestRegistry_New_SeedsSnapshot`, `TestRegistry_Snapshot_Atomic`, `TestRegistry_Reload_Debounce`, `TestRegistry_Reload_ValidationFailure_KeepsOld`, `TestRegistry_Reload_SymlinkEscape_Rejected`, `TestRegistry_Snapshot_ParallelReaders`, `TestManifest_DescriptionVec_NotSerialized`
- `internal/skill/manifest.go` — `DescriptionVec []float32` field with `json:"-"` tag; `SetDescriptionVec(v []float32)` setter
- `internal/skill/loader.go` — `parseAndValidate(path) (*Manifest, []string, error)` extracted (V12 symlink guard + read + parse + Validate + ResolveTiers); `Scan` refactored to call it. Behavior preserved for all 11 existing TestLoader_* tests.
- `go.mod` — `github.com/fsnotify/fsnotify v1.5.4 // indirect` → `github.com/fsnotify/fsnotify v1.5.4` (no lockfile churn; just remove the `indirect` marker)

## Decisions Made

- **Watcher.Add() before any directory iteration (Windows fsnotify pitfall).** During implementation of the debounce test, fsnotify stopped delivering events after `startupScan` called `filepath.WalkDir`. Root cause: ReadDirectoryChangesW silently invalidates the watcher's subscription when subsequent directory operations occur on the watched dir before the Add completes its async registration. **Fix:** moved `fsnotify.NewWatcher() + watcher.Add(root)` into `New()` BEFORE `startupScan` runs. Watch() just starts the consumer goroutine; the watcher is owned by Registry for its lifetime. Documented in registry.go doc comment + this deviation.
- **atomic.Pointer[map[string]*Entry] idiom** per the corrected 05-PATTERNS.md (the original PATTERNS.md had a typo where `Store(next)` was shown without `&`; the corrected pattern is `Store(&nextMap)` + `*Load()` to dereference). Verified via the test suite.
- **Symlink guard moves from inside WalkDir to inside parseAndValidate.** The original loader.go had the guard inside the WalkDir callback (using `d.Type()&fs.ModeSymlink != 0`). Reload paths don't go through WalkDir, so the guard had to be replicated there — extracting it into `parseAndValidate` ensures both paths share the EXACT same check (T-5-01 mitigation; no drift risk).
- **Manifest.DescriptionVec setter (not direct field assignment).** Added `SetDescriptionVec(v []float32)` so audit grep for `DescriptionVec =` catches any non-setter writes (Rule 2-style hardening). Mirrors the existing `SetResolvedTiersForTest` pattern.
- **Watch-loop path filter handles both Linux/macOS per-file events AND Windows dir-level events.** `resolveSkillPath` accepts either an event.Name that already ends in SKILL.md (Linux) or an event.Name that is the parent directory containing a SKILL.md (Windows ReadDirectoryChangesW WRITE event). Verified empirically with a Windows ReadDirectoryChangesW test.
- **Embedder error is non-fatal on reload** (log + reuse prior DescVec). D5-08 forbids silent-drop, but D5-01 routing is a UX feature not a security boundary. Cost of an embedder hiccup must not be "Skill vanishes"; the Skill stays routable on its previously-cached vector until the next successful reload.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Watcher.Add must precede any directory iteration on Windows**
- **Found during:** Task 2 (TestRegistry_Reload_Debounce failed despite a faithful implementation)
- **Issue:** The plan calls for `New()` to scan + `Watch()` to add the watcher. On Windows fsnotify v1.5.4, ReadDirectoryChangesW silently invalidates the watcher's subscription if the watched directory is iterated (filepath.WalkDir, os.ReadDir, even direct os.Stat on a child) between `fsnotify.NewWatcher()` and the first event the watcher would otherwise see. Empirically reproduced: Add-then-Walk = events fire; Walk-then-Add = events never fire, even after subsequent writes. This is a Windows-specific pitfall not documented in upstream fsnotify but confirmed via ReadDirectoryChangesW semantics.
- **Fix:** Restructured New() to (1) create the watcher, (2) `Add(root)`, (3) THEN run startupScan. Watch() merely starts the consumer goroutine; the watcher is owned by Registry for its lifetime. Documented in registry.go New() doc comment with explicit warning.
- **Files modified:** internal/skill/registry.go
- **Verification:** TestRegistry_Reload_Debounce passes (3 writes within 1s coalesce to one reload); TestRegistry_New_SeedsSnapshot still passes; all 11 TestLoader_* tests still pass after the Loader.Scan refactor.
- **Committed in:** e3be9e1 (Task 2 GREEN)

**2. [Rule 1 - Bug] Manifest field tab alignment was wrong after DescriptionVec insertion**
- **Found during:** Task 2 (manifest.go edit)
- **Issue:** The yaml/json tags on the existing 7 fields were aligned to the longest existing field name; inserting `DescriptionVec []float32` (16 chars) broke the alignment, which `gofmt` accepts but `gofmt -s` (and `golangci-lint`'s gofmt check) would flag.
- **Fix:** Re-aligned all yaml/json tag columns to the new max width.
- **Files modified:** internal/skill/manifest.go
- **Verification:** `gofmt -d` reports no diffs.
- **Committed in:** e3911d1 (Task 2 RED, which included the manifest.go field add)

---

**Total deviations:** 2 auto-fixed (1 blocking infrastructure, 1 cosmetic lint)
**Impact on plan:** Deviation #1 is essential for cross-platform correctness — without it the registry would be effectively dead on Windows (the developer's primary platform per CLAUDE.md). Deviation #2 is hygiene only. No scope creep.

## Issues Encountered

- **fsnotify event suppression on Windows (debugging took ~25 min).** The debounce test failed with NO event messages in the watchLoop logs (events were simply not arriving). Iterative debug reproduction narrowed it to: `filepath.WalkDir(tmp, ...)` BEFORE `fsnotify.NewWatcher() + Add(tmp)` → events never fire. Verified with two standalone Go programs (debug + debug2) to isolate the variable. The fix (Add-first) is documented in registry.go's New() comment so future maintainers don't re-introduce the bug.
- **TestLoader_ScanRejectsSymlinkEscape + TestRegistry_Reload_SymlinkEscape_Rejected are skipped on Windows.** Both tests use `os.Symlink` which requires Developer Mode or admin elevation on Windows. The existing TestLoader_ test was already gated by `runtime.GOOS == "windows"` skip; the new TestRegistry_ test follows the same pattern. The symlink-rejection code paths are still compiled in on every platform.

## User Setup Required

None - no external service configuration required. fsnotify is already in go.mod (just promoted from indirect to direct; no new packages installed, no download).

## Next Phase Readiness

Plan 05-02 (router + pipeline.go wiring) can now:
- Call `Registry.Snapshot()` to enumerate Skills for cosine routing (D5-01)
- Read `Entry.DescVec` for the cached embedding (no extra embedder call per request)
- Emit the `data-decision` chunks with `SkillScores` + `Selected` already wired (D5-04 from Task 1)
- Emit `data-redline` chunks when verdict flags a policy gap (D5-14 from Task 1)

Plan 05-04 (wizard) can rely on the closed loop:
- Wizard writes `skills/<name>/SKILL.md` → fsnotify picks it up via `Watch()` → next `/api/chat` query routes to it via `Snapshot()` + cosine

Concerns:
- The watcher is owned by `Registry` for its lifetime (created in `New`, closed when Watch's goroutine exits). If a future plan needs to stop watching temporarily (e.g., during bulk migrations), it will need to cancel the `ctx` passed to `Watch()`. No current plan does this.
- `parseAndValidate` computes `root` from path as `filepath.Dir(filepath.Dir(path))`. This assumes every `SKILL.md` lives at `<root>/<skill>/SKILL.md` — which holds for both `Loader.Scan` (WalkDir with subdir filter) and `Registry.Watch` (events filtered to `*SKILL.md` inside watched dir). If a future plan introduces a layout like `skills/<group>/<skill>/SKILL.md`, both the WalkDir filter and the watcher's resolveSkillPath will need to be updated.
- `atomic.Pointer[map[string]*Entry]` has an inherent race window: a reader that calls Snapshot() while a writer is in the middle of `for k, v := range *oldSnap { next[k] = v }` will see the OLD map (via `*Load()` before the writer's Store). That is the intended behavior (D5-06 in-flight pinning), but readers MUST NOT mutate the returned map — the `Snapshot()` doc comment says so.

---
*Phase: 05-skill-platform-full-knowledge-boundaries*
*Completed: 2026-08-12*