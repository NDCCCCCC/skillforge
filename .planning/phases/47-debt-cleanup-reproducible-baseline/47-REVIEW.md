---
phase: 47-debt-cleanup-reproducible-baseline
review_type: advisory-source-review
reviewed: 2026-10-19
baseline_commit: 44d87d4
head_commit: 9d603a6
scope:
  - internal/knowledge/graph_edges.go
  - internal/knowledge/graph_edges_test.go
  - internal/ontology/induction/induction.go
  - internal/ontology/induction/induction_test.go
  - internal/ontology/emitter/emitter_test.go
  - internal/server/admin_governance.go
  - internal/server/server.go
  - web/src/components/admin/governance/TimelineView.tsx
  - web/src/components/admin/governance/GovernanceTab.tsx
  - web/src/components/admin/governance/TimelineView.test.tsx
  - web/src/__tests__/DetailPanel.test.tsx
  - verify-phase-47.sh
  - verify-phase-45.sh
  - verify-phase-46.sh
status: partially_resolved
blocking: false
resolution:
  resolved:
    - finding: HIGH-1
      commit: 659879b
      note: "verify-phase-47.sh no longer prints ALL PASSED or lists P6 under 'Partitions verified' when the frontend partition never ran; the banner becomes 'PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED' and P6 moves to a 'NOT VERIFIED' block. Exit code stays 0 so WSL-only hosts can still run the gate."
    - finding: MEDIUM-1
      commit: 95ed0a3
      note: "The rollback POST result alone decides the outcome; onRollbackComplete fires before the changelog refetch and the refetch is fire-and-forget. Pinned by a fourth TimelineView.test.tsx case and mutation-verified."
  open:
    - MEDIUM-2
    - LOW-1
    - LOW-2
    - LOW-3
    - INFO-1
    - INFO-2
    - INFO-3
  evidence: .planning/phases/47-debt-cleanup-reproducible-baseline/47-REVIEW-FIXES-SUMMARY.md
still_open_note: "MEDIUM-2 (keyword_cooccur representative-chunk attachment order-dependence in graph_edges.go) and the three LOW items were deliberately left untouched and remain unfixed. This review is NOT clean."
severity_counts:
  high: 1
  medium: 2
  low: 3
  info: 3
  total: 9
verification_run:
  - "wsl: go vet ./internal/knowledge/ ./internal/ontology/... -> rc 0"
  - "wsl: go test ./internal/knowledge/ -run '^TestDeriveEdgesDeterministic$' -count=20 -> ok"
  - "wsl: go test ./internal/ontology/emitter/ ./internal/ontology/induction/ -count=1 -> ok"
  - "web: npx vitest run src/components/admin/governance/ src/__tests__/DetailPanel.test.tsx -> 29/29 passed"
  - "bash -n verify-phase-{45,46,47}.sh -> syntax OK"
---

# Phase 47 Advisory Source Review

**Status: `issues_found`** — 1 HIGH, 2 MEDIUM, 3 LOW, 3 INFO. Nothing here blocks the phase:
the core deliverables (REL-01 determinism fix, REL-02 cache write, hermetic fixtures,
waiver retirement) are real fixes and the gate is honest about test outcomes. The HIGH
finding is a coverage hole in the new gate, not a masked test failure — no `|| true`,
`-run` narrowing, `grep -v FAIL`, `set +e`, or exit-code masking exists anywhere in the
three gate scripts.

---

## What I verified as genuinely sound (so it is not re-litigated)

**REL-01 is a real fix, not test-taming.**
- Every remaining map iteration that feeds edge construction is either gone or provably
  order-free: `internal/knowledge/graph_edges.go:159-164` (groups → sorted key slice),
  `:249` (sources → sorted by `source`, which is the `sourceTokens` map key and therefore
  unique — a strict total order), `:581-585` (`distinct` → `sort.Strings(ids)`).
  `jaccard()` (`:112-127`) builds a set but never ranges over it.
- The final comparator **is** a strict total order. `Edge` (`graph_edges.go:47-52`) has
  exactly four fields — `From`, `To`, `EdgeType`, `Weight` — and all four are compared
  (`:278-293`). Two edges tied on all four keys are byte-identical values, so their
  relative order is unobservable. The `sort.Slice`-is-unstable hazard the summary
  describes is genuinely closed, not merely narrowed.
- The hardened test would fail against the old implementation. It compares the whole
  sorted slice with one `assert.Equal` (which subsumes the old `require.Len`), runs 100
  iterations instead of 1 pair, and sorts on the same four keys as the implementation.
  The pre-fix flake landed on `From`/`To`/`EdgeType` mis-ordering — which a
  whole-slice structural equality cannot survive. Strictly stronger than the old test,
  not merely re-shaped.

**REL-02 `SetShaclReport` is correctly scoped and safe.**
- Lock scope: `RollbackTBox` takes `m.mu.Lock()` at `induction.go:334` with
  `defer m.mu.Unlock()` at `:335`; the new write at `:497` is inside that critical
  section, so it really is ordered against the Steps 3-5 file writes. Claim (b) is
  true *on the success path* (see LOW-3).
- No aliasing / no torn read: `report` is freshly constructed by
  `shacl.ValidateWithTrigger` (`internal/ontology/shacl/validate.go:155`, returns
  `*Report`) and is never mutated after the `atomic.Value.Store`. Store/Load in
  `sync/atomic` are sequentially consistent, and the read side
  (`admin_governance.go:61`) only type-asserts and reads fields, so a reader can never
  see a half-written report.
- No `atomic.Value` type-consistency panic: the other writer is
  `cmd/server/main.go:947` storing the result of `shacl.Validate`
  (`validate.go:143`, also `*Report`). Both writers store the identical concrete type
  `*shacl.Report`, which is the classic way `atomic.Value` panics — it does not here.
- Same store instance on both sides: `induction.NewManager(..., ontologyKG)`
  (`main.go:957`) and `Deps.Ontology.KG` are the same object, so the write is visible
  to the admin handler.

**`admin_governance.go` / `server.go` are genuinely comment-only.** Verified
mechanically, not by eye:
`git diff 44d87d4..HEAD -- internal/server/ | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' | grep -vE '^[+-][[:space:]]*//' | grep -vE '^[+-]$'`
produced no output.

**FE refactor is a genuine extraction.** `refreshShaclReport` is defined once
(`GovernanceTab.tsx:112-121`); the only two `fetchShaclReport()` call sites are the
pre-existing mount `useEffect` and that function. No duplicated logic survives in
`onRetry`. It never rethrows, and it installs its own `.catch`, so no unhandled
promise rejection is reachable from the parent call site.

**Emitter fixtures still assert the emitter's contract.** `TestEmitSubCategoryInstanceCount`
drives `emitter.go`'s real `loadKBCategories` scoping guard (`if ent.Name() == "sql-guides"`)
through a fixture whose non-target category carries its own nested sub-directory — the
control is falsifiable by construction, and the summary records a mutation run that
turned it red (3 vs 2). `TestEmitSkillInstanceCount` asserts `len(fixtureSkills)`, a
value derived from the tree the test builds, not from the repo.
`TestEmitTierInstanceCount`'s corpus pin is genuinely safe on a clean clone: `git ls-tree
HEAD knowledge/` returns exactly 5 tracked trees and no tracked files, and the working
tree matches (`authority, draft, historical, personal, sql-guides`). Untracked-dir
divergence would break it, but a `git archive HEAD` checkout cannot have any.

**`DetailPanel.test.tsx` retarget is a strengthening, not a weakening.** It clicks the
user-visible 文档 chip (`KnowledgeGraphBrowser.tsx:122`, `mode` default `'ontology'` at
`:75`), and adds a `toHaveTextContent('知识库为空')` pin on top. The ontology empty
state it stopped covering is still pinned by `OntologyGraphView.test.tsx:319`, so no
coverage was lost.

**The gates are exit-code authoritative — I could not find a hidden waiver.**
- No `|| true` sits on any test command. The three `|| true` uses
  (`verify-phase-47.sh:79, 87, 235`) are on `grep -c`/`grep -o` inside helpers.
- `run_go_test`'s `local rc=$?` (`:96`) is correct — I verified empirically that `local`
  does not clobber the captured status (the classic bug is `local x=$(cmd)`, not this).
- The anti-vacuous `--- PASS` floor works: `pass_lines` on an empty log yields `0`, and
  `[ 0 -lt 6 ]` fails. Verified in a shell, not assumed.
- `-shuffle=on` is off for P3 (so the floor's decomposition is stable), and the
  authoritative partitions P2/P4 run whole packages with no `-run` narrowing.
- `set -uo pipefail` with explicit per-segment rc checks; no `set +e` region; no
  pipeline whose exit code comes from `head`/`tail` (tails run *after* the rc check).
- The waiver retirement in `verify-phase-45.sh` / `verify-phase-46.sh` is a real
  hardening, and 45 in particular gained the exit-code check it previously lacked
  entirely — the gate can no longer pass on `waiver_ok`'s say-so alone.
- Syntax-clean under `bash -n`; `verify-phase-47.sh` is mode 755.

---

## Findings

### HIGH-1 — `verify-phase-47.sh` reports `ALL PASSED` for a partition it never ran

**File:** `verify-phase-47.sh:190-215`, plus the closing banner at `:253-262`

**Defect.** The FE partition is the only segment in the script with a non-fatal branch,
and when it takes that branch the script still prints
`=== Phase 47 Gate: ALL PASSED ===` and the closing block still advertises
`P6 frontend line — tsc --noEmit + vitest run (Windows side; npx-absent guard is a tool
check)` as a *verified* partition. `tsc --noEmit` and `npx vitest run` (688 tests /
85 files at HEAD) never execute; the gate exits 0.

**Failure scenario.** Run `bash verify-phase-47.sh` on any host without Node on PATH —
a WSL-only shell (which is the documented way to run every Go command in this script,
per the header at `:36-39` and `env-wsl-backend-runbook`), a slim CI container, or a
fresh dev box. `command -v npx` fails, the `else` branch is skipped, and the operator
sees a green gate and a "Partitions verified" list that includes P6. A TypeScript type
error or a broken governance test introduced after this commit ships behind a green
Phase 47 gate. The info line at `:191` is honest about the skip, but the two places an
auditor reads — the banner and the partition list — both claim it ran.

**Suggested fix.** Make the absence of a required tool a hard failure by default and
gate it behind an explicit opt-out that is visible in the command line, e.g.:

```bash
if ! command -v npx >/dev/null 2>&1; then
  if [ "${SKIP_FE:-0}" = "1" ]; then
    info "P6: SKIPPED BY OPERATOR (SKIP_FE=1) — frontend NOT verified"
  else
    fail "P6: npx absent — frontend partition would be silently skipped (set SKIP_FE=1 to accept)"
  fi
fi
```

and track whether P6 ran, printing `P6 ... — SKIPPED` rather than listing it under
"Partitions verified" when it did not.

---

### MEDIUM-1 — a failed changelog refetch still reports a successful rollback as `回滚失败`

**File:** `web/src/components/admin/governance/TimelineView.tsx:143-171` (specifically `:156` vs `:160-164`)

**Defect.** The new `try { onRollbackComplete?.(); } catch {}` was added specifically so
that "a completed rollback can no longer be reported as a failed one" (T-47-14), but it
is placed **after** `await fetchChangelog().then(setChangelog);` at `:156`, which is
still inside the same outer `try`. Any rejection from that refetch falls through to the
`catch` at `:165` and renders `回滚失败：${errMsg}`. The guard therefore protects a
strictly narrower case than the invariant it is documented as enforcing, and the
`47-04-SUMMARY.md:504` claim ("A completed rollback can no longer be reported as a
failed one") is not true of the committed code.

**Failure scenario.** `POST /api/admin/ontology/rollback` returns 200 and the TBox is
genuinely rolled back (server-side cache now says `trigger: "rollback"`). The
immediately following `GET /api/admin/ontology/changelog` fails — token expired
mid-session, a 500 from the changelog handler, a dropped connection. The success toast
from `:147` is already appended to `document.body`, so the operator sees *both*
`已回滚至 v2（当前 v4）…` and `回滚失败：…` simultaneously. Worse, `onRollbackComplete`
never fires, so the banner keeps showing the boot report — the REL-02 defect this
phase exists to fix reappears on exactly the failure path where the user is already
confused. The new test suite cannot catch this: `stubFetch(true)` answers the changelog
URL successfully in all three cases, so no test ever exercises a changelog rejection.

**Note:** line `:156` is pre-existing, so this is an *incomplete mitigation of a
pre-existing bug*, not a regression introduced here. It is in scope because the phase
explicitly claims the invariant, and because the fix was placed so as to look complete.

**Suggested fix.** Move the refetch + callback out of the rollback's success/failure
classification — the POST result alone should decide the toast:

```tsx
const result = await rollbackTBox(targetVersion);   // only this decides success
// ... success toast ...
void fetchChangelog()
  .then(setChangelog)
  .catch(() => { /* changelog is a refresh, not part of the rollback outcome */ });
try { onRollbackComplete?.(); } catch { /* swallowed */ }
```

and add a fourth test case that makes the changelog fetch reject, asserting the success
toast stands and no `.sf-toast-error` appears.

---

### MEDIUM-2 — `ComputeEdges` is still input-order dependent for `keyword_cooccur` attachment

**File:** `internal/knowledge/graph_edges.go:232-241`

**Defect.** The fix determinized *map* iteration, but the `From`/`To` node IDs of every
`keyword_cooccur` edge come from a per-source **representative chunk ID** chosen by a
first-match scan over the caller-supplied `docs` slice:

```go
for src, tokens := range sourceTokens {
    var representativeID int64
    for _, d := range docs {
        if d.Source == src {
            representativeID = d.ID
            break          // <-- first row wins; depends on slice order
        }
    }
    ...
}
```

Sorting `sources` by `source` (`:249`) does not help: `source` is unique, so the sort
is a no-op on the ambiguity — two `DocumentMeta` rows sharing a `Source` but differing in
`ID` (the normal case: one source is many chunks) yield *different* `From`/`To` values
depending on which row the backend happened to return first.

**Failure scenario.** `ComputeEdges`'s production input comes from
`retriever.ListMetadata`. The Milvus implementation
(`internal/retriever/milvus/client.go:239-300`) issues a `Query` with an empty expression
over a `WithLimit(16384)` window across however many segments the collection currently
has, with no `ORDER BY` and no dedup, and returns rows in segment/insertion order.
After a compaction or a segment seal, the same corpus returns the two chunks of
`car-policy.md` in the opposite order. `representativeID` flips from chunk 5 to chunk 7,
every `keyword_cooccur` edge re-attaches to a different node, and the GRF-02
"same corpus → byte-identical `[]Edge`" property is violated end-to-end — with a
different symptom than the one closed here (attachment, not direction). Because
`EdgeGraph.Neighbors` is a directed traversal keyed on `From`, the reachable node set
changes with it.

The unit test cannot see this: `TestDeriveEdgesDeterministic` passes one fixed `metas`
slice, so it proves "same input → same output" (which is now true and well tested) and
says nothing about "same corpus → same graph".

**Suggested fix.** Pick the representative deterministically from the data rather than
from the input order:

```go
for src, tokens := range sourceTokens {
    representativeID := int64(0)
    for _, d := range docs {
        if d.Source == src && (representativeID == 0 || d.ID < representativeID) {
            representativeID = d.ID
        }
    }
    ...
}
```

Same reasoning applies to the `sort.Slice(chunks, ...)` at `:167` (see INFO-1).

---

### LOW-1 — the REL-02 FE hand-off itself has no committed test

**File:** `web/src/components/admin/governance/GovernanceTab.tsx:167`; `TimelineView.test.tsx:1-121`

**Defect.** `47-04` closes REL-02's frontend half with one line —
`<TimelineView onRollbackComplete={refreshShaclReport} />`. The new test file renders
`TimelineView` directly with its own spy, and `GovernanceTab` is rendered by **no** test
in the repo (`grep -rl GovernanceTab web/src` finds only source files). Deleting the
`onRollbackComplete` prop at `:167` — the exact line that makes the post-rollback
report reach the banner — leaves all three new tests green. The only evidence that the
wiring works is the jsdom banner-flip trace quoted at `47-04-SUMMARY.md:527-537`, whose
harness was deliberately deleted.

**Failure scenario.** A future refactor of `GovernanceTab`'s view switcher (e.g. hoisting
`TimelineView` into a `React.memo`, or restructuring the `view === 'timeline' &&` guard)
drops the prop. `refreshShaclReport` is no longer called after a rollback, the banner
silently keeps showing the boot report, and the whole gate stays green because the FE
suite passes 688/688. That is precisely the "green hides defect" failure mode T-47-19
describes.

**Suggested fix.** Add a `GovernanceTab.test.tsx` that renders the real component with a
URL-routed `fetch` stub, clicks `governance-view-timeline` then
`governance-rollback-2`, and asserts the banner text flips from `（启动时）` to
`（回滚后）`. This is a ~40-line test and it is the only thing that would make the
phase's headline FE claim re-verifiable.

---

### LOW-2 — `verify-phase-47.sh` hardcodes the WSL repo path instead of deriving it

**File:** `verify-phase-47.sh:58` vs `:64`

**Defect.** `REPO=/mnt/d/CODE/ClaudeCode/ai-agent` is a literal absolute path used by
`wsl()` (`:67`) for every Go segment, while P6/P7 correctly address the repo through the
self-derived `ROOT` (`:64`). The header at `:61-63` claims "P6 and P7 address repository
paths through this, so neither depends on the caller's cwd" — true, but the seven
substantive partitions all do depend on this machine's drive letter and directory depth.
(The uppercase `CODE` also diverges from the documented `/mnt/d/code/...` convention;
DrvFs resolves either, so that part is cosmetic.)

**Failure scenario.** Clone the repo anywhere else — a colleague's `D:\code\other\`, a
CI runner, a `/home/u/src/skillforge` WSL path. Every Go partition fails with
`cd: ...: No such file or directory`, which is at least loud, not silent. But combined
with HIGH-1 it produces a confusing failure mode: on a Linux box without WSL the Go
partitions red-light for a *path* reason while the FE partition silently skips, so the
operator's first read is not "this gate isn't portable here."

**Suggested fix.** Derive it the same way as `ROOT` — translate the Windows-side path
into the WSL path once, e.g. `REPO="$(printf '%s' "$ROOT" | sed 's#^\([A-Za-z]\):#\/mnt/\L\1#')"` —
or fail fast with an explicit message when `wsl.exe -d Ubuntu-22.04 -- true` does not
return 0.

---

### LOW-3 — the new comment's claim (b) is false on the failure branch twenty lines above it

**File:** `internal/ontology/induction/induction.go:489-492` vs `:469-484`

**Defect.** The justification for the new cache write states:

> (b) it happens inside the same `m.mu` critical section as the v{N+1} / pointer /
> version-file writes (Steps 3-5), so there is no window where the served report
> disagrees with the on-disk TBox

That is true of the write at `:497`. It is false of the sibling branch at `:469-484`,
where Steps 3-5 have already advanced the on-disk TBox to `v{next}` and the
`ValidateWithTrigger` call then fails — so the cache keeps serving a report that
describes `v{next-1}` while the pointer TTL and `tbox_version.txt` say `v{next}`. That
is precisely the disagreement claim (b) denies, and it is the deliberate D-02c
trade-off. The comment reads as a blanket property of the site rather than a property
of the success path.

**Failure scenario.** Rollback #1 validates cleanly → cache = `trigger: "rollback"`,
`validated_at = T1`. Rollback #2 persists `v{N+2}` and then hits a shapes-load failure →
cache still = the `T1` report describing `v{N+1}`. `GET /api/admin/governance/shacl`
returns 200 and the banner renders `（回滚后） 2026-10-19T10:00`, which reads as
"post-rollback state" while being one rollback and one version behind. An operator has
no signal in the payload that the report predates the current version other than
eyeballing `validated_at` against the changelog.

**Suggested fix.** Keep the tradeoff, but state it honestly at the site — e.g. amend (b)
to "on the success path there is no window ...; on the D-02c failure path the served
report deliberately describes the previous successful validation, so `validated_at` must
be compared against `tbox_version.txt` by the reader" — or, better, record the
validated TBox version on the `shacl.Report` so the staleness is machine-detectable.

---

### INFO-1 — the group sort is not a total order if a `(tier, source)` group holds two rows with the same `ID`

**File:** `internal/knowledge/graph_edges.go:167`

**Defect.** `sort.Slice(chunks, func(i, j int) { return chunks[i].ID < chunks[j].ID })`
orders only by `ID`. Two `DocumentMeta` rows with the same `(tier, source)` and the same
`ID` tie, and `sort.Slice` is unstable, so the surviving order (and therefore the `a`/`b`
assignment in the `VersionChain` pair loop at `:197-211`, which decides edge direction)
depends on the unstable sort. `ListMetadata` performs no dedup in either backend
(`internal/retriever/milvus/client.go:239+`, `internal/retriever/mongo/mongo.go:459`).

This is pre-existing and unchanged by the phase, and it is the same "not a strict total
order" class the phase correctly fixed at `:278-293` — worth closing for consistency
while the file is open. Adding `ID` uniqueness to the group sort (or deduping on read)
makes the whole function order-independent given a multiset input.

---

### INFO-2 — the "zero-waiver" property is documented but not self-enforcing

**File:** `verify-phase-47.sh` (whole file)

**Defect.** The summary's strongest claim about the gate is a set of greps run *by
hand* against the script (`grep -v '^#' verify-phase-47.sh | grep -cE
'waiver_ok|WAIVED_FAIL|waived()|t.Skip|allowlist'` → 0, and the `--- FAIL: Test`
backstop → 0). I confirmed both are 0 today: the only `--- FAIL` literal in the file is
the generic diagnostic regex `'--- FAIL: [^[:space:]]*'` at `:87`, which cannot match
the backstop pattern. But nothing inside the script re-checks its own text, so the
property is maintained by reviewer discipline rather than by the artifact.

**Failure scenario.** A future plan that needs one documented failure adds
`[ "$rc" -ne 0 ] && grep -q -- '--- FAIL: TestEmitFoo' && waived "..."` to
`run_go_test`. Every manual grep a reviewer would run returns 0 only if they re-run it —
and the natural place such a branch lands (inside a helper, with the name assembled in a
variable) is exactly where the greps are least likely to look.

**Suggested fix.** Add a self-check partition (P0) at the top of the script that greps
`$0` for the forbidden patterns and fails if any appear — roughly the two lines the
summary already documents, moved from prose into the gate.

---

### INFO-3 — stale emitter contract comment now contradicts the test

**File:** `internal/ontology/emitter/emitter.go:84-88`

**Defect.** `Emit()`'s doc comment still says "emits one individual per Skill (8
expected) … and one SubCategory per sub-directory under sql-guides (7 expected)". Both
numbers were corpus artifacts of the working tree, and the two tests that used to encode
them are now hermetic. A future reader has to reconcile three different expectations
(8, 7 in the production comment; 3, 2 in the tests; 5, 6 in `TestEmitTierInstanceCount`
plus `git ls-tree`). Not a behavior change — the phase correctly did not touch
`emitter.go` — but the phase that specifically rewrote these expectations should have
left the production doc describing the *contract* rather than the counts. `emitter.go:86`
in particular is the sentence the D-45-DEF-01 investigation kept landing on.

---

## Per-file disposition

| File | Verdict |
|---|---|
| `internal/knowledge/graph_edges.go` | Real fix. Total order closed; two residual input-order dependencies (MEDIUM-2, INFO-1) live outside the map-iteration channel this phase addressed. |
| `internal/knowledge/graph_edges_test.go` | Genuinely hardened, non-vacuous, would fail pre-fix. Nothing to change. |
| `internal/ontology/induction/induction.go` | Correct lock scope, correct type for `atomic.Value`, no aliasing, no torn read. Comment overclaim only (LOW-3). |
| `internal/ontology/induction/induction_test.go` | Both branches non-vacuous; the seeded-boot-report pattern is right. `cr.ErrorCount != violations` is near-tautological but does pin "same report object", which is the intent. |
| `internal/ontology/emitter/emitter_test.go` | Fixtures assert the emitter's contract; the nested control is falsifiable. Corpus pin on the tier test is genuinely clean-clone-safe (verified via `git ls-tree`). |
| `internal/server/admin_governance.go`, `server.go` | **Verified comment-only.** No behavior change; the doc correction is what let WARNING-1 be found. |
| `TimelineView.tsx` | Extraction and callback contract are correct; the guard is placed after a still-vulnerable await (MEDIUM-1). |
| `GovernanceTab.tsx` | Genuine single implementation, no duplicated logic, no unhandled rejection path. Wiring untested (LOW-1). |
| `TimelineView.test.tsx` | Non-vacuous (mutation-verified per summary and re-checkable). Does not cover a changelog-fetch rejection (MEDIUM-1) or the `GovernanceTab` wiring (LOW-1). |
| `DetailPanel.test.tsx` | Clean retarget, strictly stronger, no coverage lost. |
| `verify-phase-47.sh` | Exit-code authoritative throughout; anti-vacuous floor verified in a shell; no hidden waiver found. HIGH-1 (unrun partition reported as verified) and LOW-2 (hardcoded path) are the gaps. |
| `verify-phase-45.sh`, `verify-phase-46.sh` | Net hardening; 45 in particular gained the exit-code check it lacked. No masking introduced. |

## Note on the adversarial brief's specific hypotheses

- *"Any remaining map iteration feeds edge construction?"* — No. All four map ranges are
  either gone or provably order-free; see the verified section.
- *"Is the comparator a strict total order?"* — Yes. `Edge` has exactly the four
  compared fields, so exact ties are indistinguishable values, not distinct edges.
- *"Could a stored pointer be aliased to mutable state?"* — No.
- *"Could a concurrent reader observe a torn report?"* — No; `atomic.Value` is
  sequentially consistent and the report is immutable post-Store.
- *"Is `refreshShaclReport` really shared / can a rejected refetch surface a wrong
  toast?"* — Shared: yes. Wrong toast: yes, but from `TimelineView.tsx:156`, not from
  `refreshShaclReport`. Unhandled promise rejection: no.
- *"Would the hardened test fail against the old implementation?"* — Yes, and by
  reasoning the detection probability rose from ~17-25% per pair to ~100% over 100 runs.
- *"Is `TestEmitTierInstanceCount`'s corpus pin safe on a clean clone?"* — Yes, verified
  against `git ls-tree HEAD knowledge/` (5 trees, 0 files).
- *"Any production behavior silently changed to make a test pass?"* — No. Only two
  production Go files changed, both as documented features/fixes; the two server files
  are comment-only; the FE change is additive. Nothing was bent.
