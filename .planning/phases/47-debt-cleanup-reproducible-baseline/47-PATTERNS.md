# Phase 47: 清债与可复现基线 — Pattern Map

**Mapped:** 2026-10-19
**Files analyzed:** 14 (10 code/test, 1 verification/script, 3 planning-doc) — one row per line of the classification table below
**Analogs found:** 14 / 14 (every file has a concrete in-repo precedent; several are "modify in place" with an internal sibling pattern)

> Derived from `47-CONTEXT.md` (no RESEARCH.md — deliberately skipped). Every excerpt below is read from the working tree at mapping time.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/knowledge/graph_edges.go` | utility (domain algorithm) | transform / batch | self + `computeCitationEdges` (`graph_edges.go:550-574`) | exact (in-file precedent for sort-before-pair) |
| `internal/knowledge/graph_edges_test.go` | test | transform | self (`:50-90`) + `store_emitter_conformance_test.go` (fixture discipline) | exact |
| `internal/ontology/induction/induction.go` | service (domain) | request-response / CRUD | self `RollbackTBox` (`:463-476`) + boot writer (`cmd/server/main.go:934-953`) | exact |
| `internal/ontology/store/store.go` | model / store | request-response | self (`:251-262`) — READ ONLY | exact |
| `internal/server/admin_governance.go` | controller | request-response | self (`:42-62`) | exact |
| `internal/ontology/emitter/emitter_test.go` | test | file-I/O | `TestEmitMultiLineDescriptionParses` (`:234-270`) + `store_emitter_conformance_test.go:79-118` | exact |
| `web/src/components/admin/governance/TimelineView.tsx` | component | request-response | self `handleRollback` (`:121-156`) + `ShaclReportView` `onRetry` prop (`:120-142`) | exact |
| `web/src/components/admin/governance/GovernanceTab.tsx` | component | request-response | self (`:96-163`), esp. WR-07 refetch block (`:141-158`) | exact |
| `web/src/components/admin/governance/TimelineView.test.tsx` (NEW, recommended) | test | request-response | `ShaclReportView.test.tsx:20-63` | role-match |
| `verify-phase-47.sh` (CREATE) — **repo root, not `scripts/`** | config / verification script | batch | `verify-phase-46.sh` (full), `verify-phase-45.sh` (P1..P4) | exact |
| `.planning/phases/47-.../47-DEBT-DISPOSITION.md` (CREATE) | planning doc (master table) | n/a | `42-triage.md` §1/§2 | exact (structure) |
| `.planning/REQUIREMENTS.md` + `.planning/ROADMAP.md` (MODIFY, REL-04 + D-01d wording) | planning doc | n/a | self Traceability table (`REQUIREMENTS.md:53-69`) and SC-1 (`ROADMAP.md:81`) | exact |
| `.planning/STATE.md` Deferred Items index (MODIFY, D-03c) + `47-VERIFICATION.md` frontmatter pointer (D-03b) | planning doc | n/a | `STATE.md:63-73`; `44-VERIFICATION.md:1-22` | exact |
| `internal/server/platform/verify_main.go` (DELETE, untracked) | test harness (stale) | n/a | none (untracked) | no analog needed |

**Path correction to the orchestrator's expected set:** `scripts/verify-phase-47.sh` should be **`verify-phase-47.sh` at the repo root**. The two direct precedents (`verify-phase-45.sh`, `verify-phase-46.sh`) live at the repo root; `scripts/` also contains older phase scripts but the immediate precedent (Phases 40, 41, 45, 46) is root-level. Creating it under `scripts/` would break the discoverable convention.

---

## Pattern Assignments

### `internal/knowledge/graph_edges.go` (utility, transform)

**Analog:** self. The determinism contract and the fix shape both already exist in this file.

**Root-cause site — `sources` built from map iteration** (`graph_edges.go:206-233`):

```go
// 2. KeywordCooccur: Jaccard on filename tokens across sources.
sourceTokens := make(map[string][]string)    // source -> tokens
seenSource := make(map[string]bool)
for _, d := range docs {
    if seenSource[d.Source] {
        continue
    }
    seenSource[d.Source] = true
    sourceTokens[d.Source] = tokenizeFilename(d.Source)
}

// Collect unique sources and their representative chunk IDs.
type srcInfo struct {
    source   string
    chunkID  int64
    tokens   []string
}
sources := make([]srcInfo, 0, len(sourceTokens))
for src, tokens := range sourceTokens {          // <-- RANDOM MAP ORDER
    var representativeID int64
    for _, d := range docs {
        if d.Source == src {
            representativeID = d.ID
            break
        }
    }
    sources = append(sources, srcInfo{source: src, chunkID: representativeID, tokens: tokens})
}

threshold := jaccardThreshold()
for i := 0; i < len(sources); i++ {
    for j := i + 1; j < len(sources); j++ {
        score := jaccard(sources[i].tokens, sources[j].tokens)
        if score >= threshold {
            edges = append(edges, Edge{
                From:     int64ToChunkID(sources[i].chunkID),
                To:       int64ToChunkID(sources[j].chunkID),
                EdgeType: EdgeTypeKeywordCooccur,
                Weight:   score,
            })
        }
    }
}
```

`i < j` pair enumeration makes `From`/`To` a direct function of slice order. Map order flips the direction → this is the real REL-01 root cause (confirmed by CONTEXT D-01: 34/200 failures, all `From`/`To`/`EdgeType`, zero `Weight` failures).

**In-file precedent for "sort before pairing"** (`computeCitationEdges`, `graph_edges.go:550-574`) — copy this shape:

```go
distinct := make(map[string]bool)
for _, c := range v.Citations {
    distinct[c.ChunkID] = true
}
if len(distinct) < 2 {
    continue
}
ids := make([]string, 0, len(distinct))
for id := range distinct {
    ids = append(ids, id)
}
sort.Strings(ids)                     // <-- determinism before O(n²) pairing
weight := 1.0 / float64(len(ids))
for i := 0; i < len(ids); i++ {
    for j := i + 1; j < len(ids); j++ {
        edges = append(edges, Edge{From: ids[i], To: ids[j], EdgeType: EdgeTypeCitationCooccur, Weight: weight})
    }
}
```

**Groups map iteration** (`graph_edges.go:147-157`) — the same class of issue, though the final sort masks its ordering; CONTEXT D-01a asks for deterministic keys anyway:

```go
groups := make(map[string][]retriever.DocumentMeta)
for _, d := range docs {
    key := tierSourceKey(d.Tier, d.Source)
    groups[key] = append(groups[key], d)
}

var edges []Edge

// 1. SectionAdjacent + VersionChain: iterate over each (tier, source) group.
for _, chunks := range groups {
```

**Final comparator — where the `Weight` tie-break goes** (`graph_edges.go:261-270`):

```go
// Sort edges for deterministic output (GRF-02: same input → byte-identical output).
sort.Slice(edges, func(i, j int) bool {
    if edges[i].From != edges[j].From {
        return edges[i].From < edges[j].From
    }
    if edges[i].To != edges[j].To {
        return edges[i].To < edges[j].To
    }
    return edges[i].EdgeType < edges[j].EdgeType
})
```

Add `Weight` as the fourth key (required because `citation_cooccur` can emit duplicate `(From,To,EdgeType)` keys with different weights across audit rows, and `sort.Slice` is not stable). Do **not** change the public contract or the `Edge` struct.

**Determinism contract to preserve** (`graph_edges.go:129-131`):

```go
// ComputeEdges derives all deterministic relation edges from a corpus of
// DocumentMeta rows. It is a pure function: identical docs input always
// produces byte-for-byte identical []Edge output (GRF-02 determinism gate).
```

**Neighbors is directed** (`graph_edges.go:364-374`) — the reason direction flips are a real functional bug, not a test artifact (CONTEXT D-01c):

```go
for _, edge := range g.adj[current] {
    if !visited[edge.To] {
```

---

### `internal/knowledge/graph_edges_test.go` (test, transform)

**Analog:** self, current test at `:50-90`.

**Current weak shape to replace** (`graph_edges_test.go:64-89`):

```go
require.Len(t, edges1, len(edges2), "both calls must produce same edge count")

// Sort by (From, To, EdgeType) for byte-for-byte comparison.
sorted := func(e []Edge) []Edge {
    sorted := make([]Edge, len(e))
    copy(sorted, e)
    sort.Slice(sorted, func(i, j int) bool {
        if sorted[i].From != sorted[j].From {
            return sorted[i].From < sorted[j].From
        }
        if sorted[i].To != sorted[j].To {
            return sorted[i].To < sorted[j].To
        }
        return sorted[i].EdgeType < sorted[j].EdgeType
    })
    return sorted
}

a := sorted(edges1)
b := sorted(edges2)
for i := range a {
    assert.Equal(t, a[i].From, b[i].From, "From must match at index %d", i)
    assert.Equal(t, a[i].To, b[i].To, "To must match at index %d", i)
    assert.Equal(t, a[i].EdgeType, b[i].EdgeType, "EdgeType must match at index %d", i)
    assert.InDelta(t, a[i].Weight, b[i].Weight, 0, "Weight must match at index %d", i)
}
```

Required changes (CONTEXT D-01b):
1. Keep the `(From, To, EdgeType)` sort key — it is correct and must not be loosened.
2. Compare the **whole struct**: `assert.Equal(t, a, b, "edge list must be identical")` (or `require.Equal` on `[]Edge`), replacing the four per-field asserts.
3. Drive the assertion from a loop of **N ≥ 100 calls** (`for run := 0; run < n; run++ { ... }`), with the first call as the reference. Use `t.Run(fmt.Sprintf("run_%d", run), ...)` only if failure locality matters; otherwise a single message with `run` index is enough.
4. Add a one-line comment citing GRF-02 and the `sources` map-order root cause.
5. Do **not** add `InDelta` tolerances beyond exact `0` for the float compare (D-01c forbids widening tolerance).

**Test-file conventions to keep** (`graph_edges_test.go:19-44`): `testify` (`assert`/`require`), table-driven style for the type-specific tests, and the `stubAuditQuerier` helper for citation cases. No new dependencies.

---

### `internal/ontology/induction/induction.go` (service, request-response)

**Analog:** self `RollbackTBox`, plus the boot writer it must match.

**Current Step 11 — report computed then discarded** (`induction.go:463-476`):

```go
	// Step 11: Run SHACL revalidation and return violation count (D-PROV-09).
	// D-46-02: rollback consumes the same env-driven shapes path list as
	// boot (the single read point lives in the shacl package) — the former
	// hard-coded data/shacl/shapes.ttl join never resolved and reduced this
	// revalidation to a silent no-op.
	report, err := shacl.ValidateWithTrigger(m.kg, shacl.ShapesPathsFromEnv(), "rollback")
	if err != nil {
		// SHACL failure is not a rollback failure — log and return 0
		// violations (D-PROV-09 warn-and-return-0 posture; boot/rollback
		// shapes parity per D-46-02).
		fmt.Fprintf(os.Stderr, "[WARN] induction: post-rollback SHACL validation: %v\n", err)
		return next, 0, nil
	}
	return next, report.ErrorCount, nil
```

**Fix (D-02a/D-02c):** immediately before `return next, report.ErrorCount, nil`, call `m.kg.SetShaclReport(report)`. On the error branch, leave the previous report untouched (no `Store(nil)` — D-02d). `m.kg` is `*store.Store` (`induction.go:104`), and the method already exists — no new imports, no new API.

**The writer it must mirror** (`cmd/server/main.go:934-953`):

```go
	shaclShapesPaths := shacl.ShapesPathsFromEnv()
	shaclReport, shaclErr := shacl.Validate(ontologyKG, shaclShapesPaths...)
	if shaclErr != nil {
		// Non-fatal: log and continue. Boot proceeds without blocking on SHACL.
		log.Printf("[WARN] cmd/server: SHACL validation: %v", shaclErr)
	} else {
		// D-SHACL-02: error-level entities → quarantine; warnings stay in KG.
		for _, q := range shaclReport.Quarantined {
			for _, t := range ontologyKG.Query(q.Entity, "", "") {
				ontologyKG.Quarantine(t)
			}
		}
		// Cache the report so GET /api/admin/governance/shacl can serve it.
		ontologyKG.SetShaclReport(shaclReport)
```

Note the deliberate asymmetry to **document only** (D-05): boot executes `Quarantine`, rollback does not. Do not add quarantine execution in this phase.

**Warn-and-continue posture** already used twice in the same function (`induction.go:419`, `:459-461`) — keep `fmt.Fprintf(os.Stderr, "[WARN] ...")`, never return an error from cache-write failure (the write cannot fail anyway — it is `atomic.Value.Store`).

**Test to extend:** `TestRollbackShaclParity` (`induction_test.go:379-453`) — case 1 asserts `violations > 0`; add an assertion that `kg.GetShaclReport()` is non-nil, type-asserts to `*shacl.Report`, and has `Trigger == "rollback"` / `ErrorCount == violations`. Case 2 (`missing_shapes_warn_and_return_zero`, `:455+`) should assert the cache is **unchanged** (still nil if never set) — this pins D-02c. Existing fixture assembly (`induction_test.go:382-435`) is the pattern to copy: `t.TempDir()` + TTL files + `t.Setenv("SKILLFORGE_SHACL_SHAPES", shapesPath)` + `NewManager(testNS, dir, nil, kg)`.

---

### `internal/ontology/store/store.go` (READ ONLY, model/store)

**Analog:** self. No changes permitted.

`store.go:251-262`:

```go
// SetShaclReport caches the SHACL validation report on the store.
// The report is stored as any to avoid a circular import with the
// shacl package.
func (s *Store) SetShaclReport(report any) {
	s.shaclReport.Store(report)
}

// GetShaclReport returns the cached SHACL report, or nil if validation has
// not run yet. Caller type-asserts to *shacl.Report.
func (s *Store) GetShaclReport() any {
	return s.shaclReport.Load()
}
```

Constraints from CONTEXT D-02d: never `Store(nil)` with an untyped nil (panic — `atomic.Value` requires a consistent concrete type and rejects nil interface); a typed-nil would turn GET into `200 null`. No `Clear`/sentinel API in this phase.

---

### `internal/server/admin_governance.go` (controller, request-response)

**Analog:** self. Comment-only change.

**Stale comment to correct** (`admin_governance.go:40-44`):

```go
// adminGovernanceShaclHandler implements GET /api/admin/governance/shacl.
//
// Returns the cached SHACL validation report from boot or post-rollback.
// No live re-validation — the report reflects the state at the time
// validation last ran (D-SHACL-02: boot runs validation every boot).
```

After REL-02, "post-rollback" becomes literally true (it currently is not). Update the comment to name both call sites: boot (`cmd/server/main.go`) and `RollbackTBox` (`internal/ontology/induction/induction.go`). Also check `internal/server/server.go:691` and `admin_governance.go:23` (the wire-contract header already says "cached report from boot or post-rollback") for the same stale wording — the audit explicitly names `admin_governance.go:23,42 / server.go:691`.

**Handler shape stays unchanged** (`admin_governance.go:45-63`):

```go
func (s *Deps) adminGovernanceShaclHandler(ctx context.Context, c *app.RequestContext) {
	dep, ok := s.requireOntology(c)
	if !ok {
		return
	}

	report := dep.KG.GetShaclReport()
	if report == nil {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "report not available"})
		return
	}
	// Type-assert the interface{} back to *shacl.Report.
	r, ok := report.(*shacl.Report)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, graphResponseError{Error: "report not available"})
		return
	}
	c.JSON(http.StatusOK, r)
}
```

No wire change (D-02f): `types.ts:53` already declares `trigger: 'boot' | 'rollback'`.

---

### `internal/ontology/emitter/emitter_test.go` (test, file-I/O)

**Analog (same file): `TestEmitMultiLineDescriptionParses`** — an in-file, already-hermetic temp-fixture test.

**Repo-root-dependent tests to rewrite** (`emitter_test.go:44-57`, `:73-85`):

```go
func TestEmitSkillInstanceCount(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, n := loadGraph(t, ttl)
	t.Logf("emitted %d triples", n)
	skills := countByType(t, g, DefaultNS+"Skill")
	if skills != 8 {
		t.Errorf("Skill instances = %d, want 8", skills)
	}
}

func TestEmitSubCategoryInstanceCount(t *testing.T) {
	root := projectRoot(t)
	e := New(filepath.Join(root, "skills"), filepath.Join(root, "knowledge"), DefaultNS)
	ttl, err := e.Emit()
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	g, _ := loadGraph(t, ttl)
	subs := countByType(t, g, DefaultNS+"SubCategory")
	if subs != 7 {
		t.Errorf("SubCategory instances = %d, want 7", subs)
	}
}
```

Measured drift at mapping time: `ls skills/` = 8 dirs, `git ls-files skills` = 3 (`kpi-dashboard`, `simple-qa`, `sql-guides`); `knowledge/sql-guides/` has 6 subdirs both locally and in git. Hence clean-clone failures: Skill 3 vs 8 and SubCategory 6 vs 7 (CONTEXT D-04b/D-04c).

**Hermetic fixture pattern to copy — same-file precedent** (`emitter_test.go:234-252`):

```go
func TestEmitMultiLineDescriptionParses(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, "skills")
	kb := filepath.Join(root, "knowledge")
	if err := os.MkdirAll(filepath.Join(skills, "multi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(kb, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: multi\ndescription: |\n  第一行描述\n  second line with \"quote\" and \\ backslash\nagent: ChatModelAgent\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(skills, "multi", "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(skills, kb, DefaultNS)
	ttl, err := e.Emit()
```

**Cross-package precedent for a synthetic KB tree** (`store_emitter_conformance_test.go:79-113`) — copy the explicit directory list, including the `sql-guides` constraint:

```go
	root := t.TempDir()
	skills := filepath.Join(root, "skills")
	kb := filepath.Join(root, "knowledge")
	require.NoError(t, os.MkdirAll(filepath.Join(skills, "multi"), 0o755))
	for _, dir := range []string{
		// SubCategory individuals are only emitted under sql-guides
		// (emitter.go loadKBCategories: `if ent.Name() == "sql-guides"`).
		filepath.Join(kb, "sql-guides", "取数口径"),
		filepath.Join(kb, "authority"),
		filepath.Join(kb, "draft"),
	} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	...
	ttl, err := emitter.New(skills, kb, emitter.DefaultNS).Emit()
	require.NoError(t, err)
```

**Emitter contract the fixture must feed** (`emitter.go:130-158`): one `SubCategory` per **sub-directory under `sql-guides` only**; all top-level dirs under `KBRoot` become `KBCategory`. So:
- `TestEmitSkillInstanceCount` fixture: create exactly K `skills/<name>/SKILL.md` dirs and assert `K` (e.g. 2–3, derived from the fixture — not the repo).
- `TestEmitSubCategoryInstanceCount` fixture: create exactly M subdirs under `<kb>/sql-guides` and assert `M`; keep the `emitter.go` contract untouched (D-04b: do not "make the implementation match 7").
- Do **not** simply change `want 7` to `want 6` — that still pins external corpus (CONTEXT D-04b).
- `projectRoot(t)` (`:300-308`) becomes unused by these two tests; keep it only if other tests still need it, otherwise delete to avoid dead code.

**Helper conventions:** reuse `loadGraph`, `countByType`, `contains` from the same file (`:35-42`, `:315-333`, `:344-346`). `countByType` counts only subjects prefixed `skill_`/`cat_`/`sub_`, which the synthetic fixtures satisfy.

---

### `web/src/components/admin/governance/TimelineView.tsx` (component, request-response)

**Analog:** self plus the existing callback-prop contract in `ShaclReportView`.

**Current rollback completion — only changelog is refetched** (`TimelineView.tsx:121-156`; the key line is `:144-145`):

```tsx
      // Success toast
      const toast = document.createElement('div');
      toast.className = 'sf-toast-success';
      toast.textContent = `已回滚至 v${targetVersion}（当前 v${result.version}）· 新增 ${result.new_violations} 条 violation`;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 5000);

      // Refetch all three surfaces + switch view to shacl
      await fetchChangelog().then(setChangelog);
```

The comment claims "all three surfaces" but only changelog is refetched — this is the FE half of REL-02 (D-02b). Keep the toast text exactly as-is (D-02e).

**Callback contract to add (D-02b, "existing callback style, no new UI")** — copy `ShaclReportViewProps` (`ShaclReportView.tsx:120-142`):

```tsx
interface ShaclReportViewProps {
  report: ShaclReport | null;
  error?: string | null;
  onRetry?: () => void;
}
```

Apply the same shape to `TimelineView`:

```tsx
interface TimelineViewProps {
  /** Fired after a successful rollback so GovernanceTab can refetch the cached SHACL report. */
  onRollbackComplete?: () => void;
}
```

Call it after `await fetchChangelog().then(setChangelog)` (or immediately after the toast, before the changelog refetch — either is acceptable; the refetch is independent). Do not add buttons, panels, or new view state. Fire-and-forget is acceptable if the parent's refetch swallows its own errors, but a rejected promise from the prop must not turn a successful rollback into the failure toast — wrap in `try { onRollbackComplete?.(); } catch {}` or have the parent pass a non-throwing handler.

**Component signature to touch** (`TimelineView.tsx:107-113`): `export default function TimelineView()` → `export default function TimelineView({ onRollbackComplete }: TimelineViewProps)`.

**Error/toast conventions:** success = `.sf-toast-success`, failure = `.sf-toast-error`, 5000 ms auto-remove (`:138-152`). Keep them.

---

### `web/src/components/admin/governance/GovernanceTab.tsx` (component, request-response)

**Analog:** self. The refetch + stale-error-clearing block is the exact pattern to factor into a shared handler.

**Current mount-only fetch** (`GovernanceTab.tsx:96-105`):

```tsx
export default function GovernanceTab() {
  const [view, setView] = useState<GovernanceView>('shacl');
  const [shaclReport, setShaclReport] = useState<ShaclReport | null>(null);
  const [shaclError, setShaclError] = useState<string | null>(null);

  useEffect(() => {
    fetchShaclReport()
      .then(setShaclReport)
      .catch((e: unknown) => setShaclError(e instanceof Error ? e.message : String(e)));
  }, []);
```

**Refetch pattern to reuse — WR-07 block** (`GovernanceTab.tsx:141-158`):

```tsx
      {view === 'shacl' && (
        <ShaclReportView
          report={shaclReport}
          error={shaclError}
          onRetry={() => {
            // WR-07: a successful retry must clear the stale error —
            // ShaclReportView renders the failure branch whenever error is
            // truthy, so leaving it set kept the view stuck on
            // 「校验报告加载失败」 even after the refetch succeeded.
            fetchShaclReport()
              .then((r) => {
                setShaclError(null);
                setShaclReport(r);
              })
              .catch((e: unknown) => setShaclError(e instanceof Error ? e.message : String(e)));
          }}
        />
      )}
      {view === 'provenance' && <ProvenanceGraphView />}
      {view === 'timeline' && <TimelineView />}
```

Recommended refactor for D-02b: extract this body into a `refreshShaclReport` function (an arrow function defined in the component, or `useCallback` if lint requires), then:
- use it in `onRetry`
- pass it to `<TimelineView onRollbackComplete={refreshShaclReport} />`

Keep the `WR-07` semantics: successful refetch clears `shaclError`; failed refetch sets it. The refreshed report's `trigger` is then `'rollback'`, so the existing banner branch (`:83`) renders 「回滚后」 — the dead branch recovery requires no render change.

**Existing render branch that becomes live again** (`GovernanceTab.tsx:81-91`, inside `HealthBanner`):

```tsx
      <div className="ml-auto text-xs text-text-muted font-mono">
        校验时点 {new Date(report.validated_at).toLocaleString()}
        （{report.trigger === 'boot' ? '启动时' : '回滚后'}）· shapes{' '}
```

No new UI, no new view (`:20`, `:22-26`), no router (local `view` state).

---

### `web/src/components/admin/governance/TimelineView.test.tsx` (NEW, recommended for SC-2)

**Analog:** `ShaclReportView.test.tsx`.

**Test scaffolding to copy** (`ShaclReportView.test.tsx:20-63`):

```tsx
import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import '@testing-library/jest-dom/vitest';
import ShaclReportView from './ShaclReportView';
import type { QuarantinedEntity, ShaclReport, ShaclViolation, ViolationLayer } from './types';

function makeReport(overrides: Partial<ShaclReport> = {}): ShaclReport {
  return {
    health: 'error',
    error_count: 1,
    warning_count: 0,
    warnings: [],
    quarantined: [],
    triple_count: 176,
    shapes_path: 'data/shacl/shapes.ttl,data/shacl/entities.ttl',
    validated_at: '2026-10-08T00:00:00Z',
    trigger: 'boot',
    ...overrides,
  };
}

afterEach(cleanup);
```

For the rollback flow, stub `fetch` with `vi.stubGlobal('fetch', vi.fn(...))` exactly as `governanceApi.test.ts:30-50` does (two responses in sequence: `fetchChangelog` payload then `fetchShaclReport` payload), stub `window.confirm` to `true`, click `governance-rollback-<v>`, and assert the `onRollbackComplete` spy fired. `data-testid` hooks already exist: `governance-rollback-${entry.version}` (`TimelineView.tsx:93`) and `governance-view-*` (`GovernanceTab.tsx:128`). No new testids required.

---

### `verify-phase-47.sh` (CREATE — repo root) (config, batch)

**Analog:** `verify-phase-46.sh` (the newest and strictest). Copy its header, helpers, and P1..P4 skeleton; then remove the waiver.

**Header + environment contract** (`verify-phase-46.sh:1-49`):

```bash
#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
...
set -uo pipefail

export MSYS_NO_PATHCONV=1

REPO=/mnt/d/CODE/ClaudeCode/ai-agent
GO=/usr/local/go/bin/go

# wsl <command> — run a gate command inside WSL at the repo root.
wsl() { wsl.exe -d Ubuntu-22.04 -- bash -c "cd $REPO && $1"; }

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass()    { echo -e "${GREEN}PASS${NC} $1"; }
fail()    { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info()    { echo -e "${YELLOW}INFO${NC} $1"; }
waived()  { echo -e "${YELLOW}WAIVED${NC} $1"; }   # DELETE/omit for Phase 47

WORK="$(mktemp -d /tmp/verify47-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

# pass_lines <log> — number of --- PASS lines in a -v go test log
# (verify-phase-39.sh anti-vacuous pattern).
pass_lines() { grep -c -- '--- PASS' "$1" || true; }
```

**P1 — build + vet** (`verify-phase-46.sh:79-82`):

```bash
info "P1: build + vet (CGO_ENABLED=1)"
wsl "CGO_ENABLED=1 $GO build ./... && CGO_ENABLED=1 $GO vet ./internal/ontology/... ./internal/server/..." \
  || fail "P1 build+vet failed"
pass "P1 build + vet"
```

**P2/P4 — exit-code-authoritative, empty-log guard** (`verify-phase-46.sh:102-116`, `:144-158`). This is the piece Phase 47 must harden by **removing `waiver_ok`** (phase 46 still waives D-45-DEF-01; Phase 47 must be zero-waiver, D-04a):

```bash
info "P2: ontology suite (./internal/ontology/...)"
wsl "CGO_ENABLED=1 $GO test ./internal/ontology/... -count=1" >"$WORK/p2.log" 2>&1
p2_rc=$?
if [ ! -s "$WORK/p2.log" ]; then
  fail "P2 produced no output (rc=$p2_rc) — infrastructure failure, no evidence of a test run"
fi
if [ "$p2_rc" -ne 0 ]; then
  fail_detail "$WORK/p2.log" "P2"
  fail "P2 ontology suite failed (rc=$p2_rc)"
fi
```

`fail_detail` (copy from `:84-91`) keeps diagnostics:

```bash
fail_detail() {
  echo "--- $2 log tail ---"
  tail -30 "$1"
  echo "--- $2 unique --- FAIL set ---"
  grep -o -- '--- FAIL: [^[:space:]]*' "$1" | sort -u || true
}
```

**P3 — anti-vacuous --- PASS count** (`verify-phase-46.sh:121-138`): keep the `rc` + `pass_lines >= N` pairing. For Phase 47 the targeted segment should cover the two repaired areas, e.g. `-run 'TestDeriveEdgesDeterministic|TestRollbackShaclParity|TestEmitSkillInstanceCount|TestEmitSubCategoryInstanceCount'`, and the threshold must be the count of top-level target tests (plus sub-tests) verified at execution time.

**New segment required by D-01e/D-04e — determinism strength** (no existing analog; compose from P3's pattern):

```bash
info "P5: determinism strength (100x + shuffle)"
wsl "CGO_ENABLED=1 $GO test ./internal/knowledge/ -count=100 -run '^TestDeriveEdgesDeterministic$'" >"$WORK/p5a.log" 2>&1
[ $? -eq 0 ] || { fail_detail "$WORK/p5a.log" "P5a"; fail "P5a determinism -count=100 failed"; }
wsl "CGO_ENABLED=1 $GO test ./internal/knowledge/ ./internal/ontology/... -count=1 -shuffle=on" >"$WORK/p5b.log" 2>&1
[ $? -eq 0 ] || { fail_detail "$WORK/p5b.log" "P5b"; fail "P5b -shuffle=on failed"; }
pass "P5 determinism strength (100x + shuffle)"
```

Also add a **mechanical disposition check** (D-03f): grep every ID (`D-45-DEF-01`, `D-45-DEF-02`, `D-45-DEF-03`, `D-46-DEF-01`, `D-46-DEF-02`, `WARNING-1`, `WARNING-2`, `BL-01`, `BL-02`, `BL-03`, `BL-04`) in `47-DEBT-DISPOSITION.md` and require a non-empty disposition cell per row.

**Do not keep** (D-04f): `waiver_ok`, the `WAIVED_FAIL` constant, any `t.Skip`/`WAIVED` logic. Phase 47's gate must be green with an empty waiver.

---

### `.planning/phases/47-.../47-DEBT-DISPOSITION.md` (CREATE, planning doc)

**Analog:** `42-triage.md` — single-file master table precedent (D-03a). Its two most reusable structural pieces:

**Bucket summary table** (`42-triage.md:30-42`):

```markdown
## 1. Bucket summary

| Bucket | Count | Closure path |
|---|---|---|
| `already-resolved` | 5 | Close as-is; update frontmatter; move to resolved/ |
| `stale`（无活动 / activity 滞留） | 6 | Close as stale; 30+ 天无活动 |
| `superseded`（v1.4 绕过） | 3 | Close as superseded; 不需 fix |
| `needs-code`（42-02 范围） | 1 | 39-REVIEW IN-01..09 (9 个 IN-* fix) + 任何 49 项中 needs-code 的 |
| `needs-live`（42-03 runbook） | 11 | operator + intranet; bundle 5 live gates + 6 v1.3 inherited 中 live |
| `needs-human-judgment` | 4 | 决策卡；非 fix |
| **Total** | **30** | |
```

**Per-ID master table** (`42-triage.md:50-58`) — the column shape to adapt (CONTEXT D-03a asks for `ID / 来源 / 去向 / 证据`):

```markdown
| # | Item | Path | Frontmatter status | Bucket | Closure path |
|---|---|---|---|---|---|
| D1 | agent-llm-query-rewrite | `.planning/debug/agent-llm-query-rewrite.md` | `verified-encoding-fix-closed` | `already-resolved` | move to `debug/resolved/` |
```

Recommended Phase 47 table: `| ID | 来源 | 去向 | 证据 |` with 去向 allowed values `v1.7 修复` / `延期+理由` / `ack backlog`. Required IDs (from CONTEXT + audit): `D-45-DEF-01`, `D-45-DEF-02`, `D-45-DEF-03`, `D-46-DEF-01`, `D-46-DEF-02`, `WARNING-1`, `WARNING-2`, `BL-01`, `BL-02`, `BL-03`, `BL-04`, plus the rollback-quarantine asymmetry (`D-05`) and the 46-REVIEW Info-level 7 items as an ack-backlog row.

Evidence sources to cite (do not re-derive): `45-.../deferred-items.md`, `46-.../deferred-items.md`, `v1.6-MILESTONE-AUDIT.md:25,30,31`, `v1.6-REQUIREMENTS.md:40-43`.

---

### `.planning/REQUIREMENTS.md` + `.planning/ROADMAP.md` (MODIFY, REL-04 + D-01d)

**Analog:** the existing Traceability table (`REQUIREMENTS.md:53-69`) — append rows in the same two-column form; do not restructure.

```markdown
| Requirement | Phase | Status |
|-------------|-------|--------|
...
| REL-01 | Phase 47 | Pending |
| REL-02 | Phase 47 | Pending |
| REL-03 | Phase 51 | Pending |
| REL-04 | Phase 47 | Pending |
```

REL-04's actual addition: four `BL-01..04` rows (their definitions are in `.planning/milestones/v1.6-REQUIREMENTS.md:40-43`, do not edit that archive per D-03e):

```markdown
- **BL-01**: 双 resolver 去重（`resolveSkillTiers` vs `resolveSkillTiersFromKG`）
- **BL-02**: server 包 lint 清扫
- **BL-03**: live 抽取 prompt 对齐 writes-only 的残余微调
- **BL-04**: DetailPanel/emitter 两个 deferred 测试
```

Map each to an owning phase (e.g. BL-04 → Phase 47 as the hermeticize work; BL-02 → Phase 48 lint; choose explicitly and record in the disposition doc). Update the `Coverage:` line at `:69` to reflect the new denominator.

**D-01d wording correction** — `ROADMAP.md:81` and `REQUIREMENTS.md:29` currently say "浮点 Weight 确定性化". Per measured evidence they must be rewritten to "边方向确定性（消除 map 迭代序）". Do not leave the old wording in place; CONTEXT explicitly permits the correction.

---

### `.planning/STATE.md` index (MODIFY) + `47-VERIFICATION.md` frontmatter pointer

**D-03c — keep index rows, link to master table.** Existing table (`STATE.md:63-73`):

```markdown
## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| tech_debt | D-46-DEF-02 TestDeriveEdgesDeterministic flaky | → Phase 47 修复 (REL-01) | v1.6 |
| tech_debt | audit WARNING-1 rollback SHACL 缓存刷新缺口 | → Phase 47 修复 (REL-02) | v1.6 |
| tech_debt | BL-01..04 Traceability 补录 | → Phase 47 处置 (REL-04) | v1.6 |
| tech_debt | D-45-DEF-01/02/03、D-46-DEF-01、owl:Class quirk WARNING-2 | → Phase 47 逐项处置记录 | v1.6 |
```

Keep one row per ID and add a link to `47-DEBT-DISPOSITION.md`; the table is an index, not the authoritative evidence.

**D-03b — `47-VERIFICATION.md` frontmatter pointer.** Direct precedent (`44-VERIFICATION.md:1-22`):

```yaml
---
phase: "44-sparql-test-fix"
verified: 2026-09-28T12:35:00Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
overrides: []
re_verification: false
gaps: []
deferred:
  - truth: "Skill hasReadAccess visibility decision (D-SPARQL-05)"
    addressed_in: "v1.6 backlog"
    evidence: "44-DECISIONS.md D-SPARQL-05: deferred to discuss-phase for v1.6 follow-up"
  - truth: "comment predicate hardening (D-SPARQL-07)"
    addressed_in: "v1.6 backlog"
    evidence: "44-DECISIONS.md v1.6 backlog net addition; no SPARQL test exercises comment in this path"
human_verification: []
---
```

In Phase 47 use a single `deferred:` entry (or a dedicated `disposition_record:` key) whose `evidence` is `47-DEBT-DISPOSITION.md` plus the master-table anchor. `47-VERIFICATION.md` is produced at verification time — the planner should note the requirement, not create the file now.

---

## Shared Patterns

### Deterministic iteration before O(n²) pairing
**Source:** `internal/knowledge/graph_edges.go:550-574` (`computeCitationEdges`)
**Apply to:** `ComputeEdges` `sources`, `groups` key iteration.
Copy the map→slice→`sort`→pair sequence. Never let Go map order determine `From`/`To`.

### Comparator completeness (full total order)
**Source:** `internal/knowledge/graph_edges.go:261-270`
**Apply to:** every `sort.Slice` over `Edge`.
Use **all** fields that can distinguish two entries (`From`, `To`, `EdgeType`, `Weight`). `sort.Slice` is unstable; duplicate keys must still be ordered deterministically.

### Warn-and-continue for non-critical side effects
**Source:** `internal/ontology/induction/induction.go:419`, `:459-461`, `:469-475`; `cmd/server/main.go:937-939`
**Apply to:** `SetShaclReport` on the SHACL-error branch.
Position: log to stderr with `[WARN]`, never return an error for a cache/audit side effect that does not invalidate the primary operation.

### Hermetic temp-fixture tests
**Source:** `internal/ontology/store/store_emitter_conformance_test.go:79-118`; `internal/ontology/emitter/emitter_test.go:234-252`
**Apply to:** `TestEmitSkillInstanceCount`, `TestEmitSubCategoryInstanceCount`.
`t.TempDir()` + explicit `os.MkdirAll` + `os.WriteFile(SKILL.md)` + `New(tempRoots, DefaultNS)`. Assert against the fixture's own count. Never walk the repo root in a test that must pass on a clean clone.

### Exit-code-authoritative gates + empty-log guard
**Source:** `verify-phase-46.sh:96-116` (WR-06 comment and implementation)
**Apply to:** every `go test` segment in `verify-phase-47.sh`.
A non-zero rc is a failure; an empty log is a failure. `grep`-only judgment is explicitly forbidden — infrastructure failures produce no `--- FAIL` lines.

### FE refetch via optional callback prop
**Source:** `ShaclReportView.tsx:120-142` (`onRetry`) consumed in `GovernanceTab.tsx:141-158`
**Apply to:** `TimelineView.onRollbackComplete`.
Parent owns the fetch/state; child emits a signal. Successful refetch clears the stale error and replaces the report. No new UI, no new state store.

### Toast conventions
**Source:** `TimelineView.tsx:137-152`
**Apply to:** the rollback success/failure paths (unchanged).
`document.createElement('div')` + `.sf-toast-success` / `.sf-toast-error` + `setTimeout(remove, 5000)`.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| N-times loop assertion (`TestDeriveEdgesDeterministic`) | test | transform | No existing test in the repo loops the same pure function N≥100 times; compose from the existing testify style at `graph_edges_test.go:50-90`. |
| Determinism-strength gate segment (`-count=100` + `-shuffle=on`) | verification script | batch | No prior `verify-phase-*.sh` runs `-count=100` or `-shuffle=on`; compose from the P3 rc+`pass_lines` pattern (`verify-phase-46.sh:121-138`). |
| `internal/server/platform/verify_main.go` (DELETE) | stale harness | n/a | Untracked (`git ls-files` empty), runtime-stale since Phase 44; no analog and no replacement needed. Deleting it is a working-tree cleanup only (D-04d). |

---

## Metadata

**Analog search scope:** `internal/knowledge/`, `internal/ontology/{induction,store,emitter,shacl}/`, `internal/server/`, `cmd/server/`, `web/src/components/admin/governance/`, `web/src/__tests__/`, repo-root `verify-phase-*.sh`, `.planning/phases/{42,44,45,46}/`.
**Files scanned:** 22 (12 read in full, 10 via targeted line-range reads/greps).
**Pattern extraction date:** 2026-10-19.
**Working-tree facts pinned at mapping time:** `git ls-files skills` = 3 vs `ls skills/` = 8; `knowledge/sql-guides/` = 6 subdirs both tracked and local (test wants 7); `internal/server/platform/verify_main.go` exists but is untracked; `SetShaclReport` is called only at `cmd/server/main.go:950`.
