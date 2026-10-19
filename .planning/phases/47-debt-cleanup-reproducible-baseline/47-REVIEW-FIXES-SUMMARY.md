---
phase: 47-debt-cleanup-reproducible-baseline
type: review-fixes
scope: [HIGH-1, MEDIUM-1]
out_of_scope_left_open: [MEDIUM-2, LOW-1, LOW-2, LOW-3, INFO-1, INFO-2, INFO-3]
source_review: .planning/phases/47-debt-cleanup-reproducible-baseline/47-REVIEW.md
completed: 2026-10-19
commits:
  - 659879b
  - 95ed0a3
---

# Phase 47 Review Fixes — HIGH-1 and MEDIUM-1

Two findings from `47-REVIEW.md` were fixed, each in its own atomic commit.
**Everything else in that review was left exactly as it was** — MEDIUM-2
(`keyword_cooccur` attachment order-dependence in `graph_edges.go`) and the
three LOW items are untouched and remain open. This phase is **not** clean.

---

## HIGH-1 — the gate reported `ALL PASSED` for a partition it never ran

### Root cause

`verify-phase-47.sh`'s frontend segment had one non-fatal branch: if
`command -v npx` failed, it printed an info line and moved on. Two places an
auditor reads still claimed the partition had run — the closing banner
(`=== Phase 47 Gate: ALL PASSED ===`) and the `Partitions verified:` list
(`P6 frontend line — tsc --noEmit + vitest run ...`). On a host without Node
the gate therefore exited 0 having executed **0 of 688** frontend tests.

### Decision: labeled SKIP, not a hard FAIL

The review suggested failing hard with a `SKIP_FE=1` opt-out. **Rejected**, for
one concrete reason: this repository's hard constraint is that every Go
partition runs through `wsl.exe`, and **the WSL instance on this box has no
node installed at all** (which is precisely why P6 is written to run on the
Windows side). A WSL-only host would be permanently unable to run the gate,
and a gate nobody can run degrades to a gate nobody runs — the exact failure
mode this phase exists to prevent. A hard fail would also punish the operator
for an environment fact with no in-script remedy.

So: a skipped FE partition is a **clearly-labeled SKIP**, and the label is
built so it cannot be mistaken for a pass at any of the three places a reader
looks:

1. `P6_ran` is set to `1` **only** after both `tsc --noEmit` and
   `npx vitest run` ran and exited 0. It is the single source of truth.
2. When it is not `1` the banner changes from `ALL PASSED` to
   `PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED`, followed by two more
   lines naming the reason and stating that no frontend evidence was produced.
3. P6 moves **out of** `Partitions verified:` into a separate
   `NOT VERIFIED (no evidence either way — this is NOT a pass and NOT a fail):`
   block.

The skip itself remains a **tool check only** — it never inspects a test name, a
result, or any waivable condition, so the zero-waiver property is untouched.
`SKIP_FE=1` is kept: it records the skip as an explicit operator decision
instead of an environment fact, and says so in the output.

### Proof — real run with `npx` masked out of PATH

```bash
PATH_SAFE=$(echo "$PATH" | tr ':' '\n' | grep -v -i 'nodejs\|npm' | paste -sd:)
env PATH="$PATH_SAFE" bash -c 'command -v npx || echo NPX_ABSENT_CONFIRMED'
#   -> NPX_ABSENT_CONFIRMED     (wsl.exe, grep, mktemp still resolve)
env PATH="$PATH_SAFE" bash verify-phase-47.sh
```

Verbatim output (ANSI colour codes stripped for readability):

```
=== Phase 47 Verification (WSL Ubuntu-22.04, CGO_ENABLED=1) ===
INFO P1: build + vet (CGO_ENABLED=1)
PASS P1 build + vet
INFO P2: ontology suite (./internal/ontology/...)
ok  	github.com/skillforge/skillforge/internal/ontology/emitter	0.194s
ok  	github.com/skillforge/skillforge/internal/ontology/induction	0.036s
ok  	github.com/skillforge/skillforge/internal/ontology/shacl	0.025s
ok  	github.com/skillforge/skillforge/internal/ontology/store	0.333s
PASS P2 ontology suite
INFO P3: targeted repairs (exit code + >= 6 --- PASS)
PASS P3 targeted repairs (6 --- PASS, floor 6)
INFO P4: full internal suite (./internal/...)
?   	github.com/skillforge/skillforge/internal/vecmath	[no test files]
ok  	github.com/skillforge/skillforge/internal/verdict	0.011s
ok  	github.com/skillforge/skillforge/internal/wire	0.023s
PASS P4 full internal suite
INFO P5a: determinism (TestDeriveEdgesDeterministic -count=100)
PASS P5a determinism -count=100
INFO P5b: determinism (-shuffle=on over knowledge + ontology)
PASS P5b determinism -shuffle=on
PASS P5 determinism strength (100x + shuffle)
INFO P6: frontend (tsc --noEmit + vitest run)
INFO P6: npx absent from PATH on this host — frontend partition was NEVER RUN (re-run with node on PATH, or set SKIP_FE=1 to record this as an operator decision)
INFO P7: disposition record (19 IDs, closed vocabulary, non-empty evidence)
PASS P7 disposition record (19 IDs, each one row, closed 去向, non-empty 证据)

=== Phase 47 Gate: PARTIAL PASS — FRONTEND PARTITION NOT VERIFIED ===
=== This gate did NOT pass in full. P6 did not run: npx absent from PATH on this host — frontend partition was NEVER RUN (re-run with node on PATH, or set SKIP_FE=1 to record this as an operator decision) ===
=== No frontend evidence was produced — neither a pass nor a failure. ===

Partitions verified:
  P1 build + vet            — CGO_ENABLED=1 build ./... + vet ./internal/{ontology,server,knowledge}/...
  P2 ontology suite         — shacl/store/induction/emitter, exit-code authoritative
  P3 targeted repairs       — 4 repaired tests, >= 6 --- PASS (floor measured 2026-10-19)
  P4 full internal suite    — ./internal/... hermetic, exit-code authoritative
  P5 determinism strength   — TestDeriveEdgesDeterministic -count=100 + one -shuffle=on run
  P7 disposition record     — 19 v1.6 leftover IDs, closed 去向 vocabulary, non-empty 证据

NOT VERIFIED (no evidence either way — this is NOT a pass and NOT a fail):
  P6 frontend line          — npx absent from PATH on this host — frontend partition was NEVER RUN (re-run with node on PATH, or set SKIP_FE=1 to record this as an operator decision)

MASKED_GATE_EXIT=0
```

Mechanical assertions on that log, not an eyeball check:

```
$ grep -c "ALL PASSED" /tmp/masked47.log
0
$ grep -n "P6" /tmp/masked47.log
22:INFO P6: frontend (tsc --noEmit + vitest run)
23:INFO P6: npx absent from PATH on this host — frontend partition was NEVER RUN ...
28:=== This gate did NOT pass in full. P6 did not run: npx absent from PATH ...
40:  P6 frontend line          — npx absent from PATH on this host — ...   <- the NOT VERIFIED block
```

`ALL PASSED` appears **zero** times, and every remaining `P6` line is either the
partition's own info line or the `NOT VERIFIED` entry — P6 is absent from
`Partitions verified:`.

### Real environment still exits 0

```
$ bash verify-phase-47.sh
```

Tail of the run (ANSI stripped):

```
PASS P5 determinism strength (100x + shuffle)
INFO P6: frontend (tsc --noEmit + vitest run)
PASS P6a tsc --noEmit

 Test Files  85 passed (85)
      Tests  689 passed (689)
   Duration  44.88s

PASS P6b frontend vitest run
INFO P7: disposition record (19 IDs, closed vocabulary, non-empty evidence)
PASS P7 disposition record (19 IDs, each one row, closed 去向, non-empty 证据)

=== Phase 47 Gate: ALL PASSED ===

Partitions verified:
  P1 build + vet            — CGO_ENABLED=1 build ./... + vet ./internal/{ontology,server,knowledge}/...
  P2 ontology suite         — shacl/store/induction/emitter, exit-code authoritative
  P3 targeted repairs       — 4 repaired tests, >= 6 --- PASS (floor measured 2026-10-19)
  P4 full internal suite    — ./internal/... hermetic, exit-code authoritative
  P5 determinism strength   — TestDeriveEdgesDeterministic -count=100 + one -shuffle=on run
  P6 frontend line          — tsc --noEmit + vitest run (Windows side)
  P7 disposition record     — 19 v1.6 leftover IDs, closed 去向 vocabulary, non-empty 证据

REAL_GATE_EXIT=0
```

P1–P5 and P7 are byte-identical to the masked run above, `ALL PASSED` is back
**because** the frontend partition genuinely ran (689 tests / 85 files, the 689
being this fix's new case), and P6 appears under `Partitions verified:` with the
stale "npx-absent guard is a tool check" parenthetical replaced by the plain
command description it now actually describes.

---

## MEDIUM-1 — a failed changelog refetch still reported a successful rollback as `回滚失败`

### Root cause

The guard plan 47-04 added — `try { onRollbackComplete?.(); } catch {}` — wrapped
only the callback. But it sat **after** `await fetchChangelog().then(setChangelog)`,
which was inside the *same* outer `try`. So a rejection from the refetch fell
through to the outer `catch` and rendered `回滚失败：…` for a rollback that had
genuinely succeeded, **and** skipped the callback, so the REL-02 banner refresh
was lost on exactly the path where the operator is already confused.

The three original tests could not catch it: `stubFetch(true)` answered the
changelog URL successfully in all of them.

### Fix

The POST result alone now decides the rollback's outcome; everything after it is
a secondary refresh outside the classification:

```tsx
const result = await rollbackTBox(targetVersion);   // only this decides success
// ... success toast (literal unchanged) ...
try {
  onRollbackComplete?.();
} catch {
  /* a parent refetch failure must never repurpose a real rollback */
}
void fetchChangelog().then(setChangelog).catch(() => {
  /* secondary refresh only — never a rollback outcome */
});
```

Two properties the criteria demanded, both structural rather than incidental:

- The callback is invoked **unconditionally**, before the refetch is even
  issued, so REL-02 cannot depend on the changelog at all.
- A changelog rejection has **no path** to the `回滚失败` toast — it is not
  awaited and its rejection is swallowed.

This is a deliberate change of *ordering* from what 47-04 recorded ("callback
fires after the changelog refetch"), and `47-04-SUMMARY.md` has been amended to
say so rather than left silently stale.

### New test

Fourth case in `TimelineView.test.tsx`, with a dedicated stub
(`stubFetchChangelogRefetchFails`) whose mount GET answers normally and whose
**POST-rollback refetch answers HTTP 500** — precisely the gap the original
stub left. It asserts: the callback fires once; the refetch really happened and
really failed (2 changelog calls, so it cannot pass vacuously); the success
toast stands; `回滚失败` is absent and no `.sf-toast-error` exists; the button
re-enables (so `finally` still runs).

### Proof of non-vacuity — revert, fail, restore, pass

Component temporarily reverted to `HEAD` (`git checkout --
web/.../TimelineView.tsx`, the `await fetchChangelog()` back at line 156), new
test kept:

```
× a changelog refetch failure after a successful rollback is not reported as a rollback failure
  ❯ TimelineView.test.tsx:160  await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))

  <body> contained, simultaneously:
    <div class="sf-toast-success">已回滚至 v2（当前 v4）· 新增 2 条 violation</div>
    <div class="sf-toast-error">回滚失败：{"error":"changelog handler exploded"}</div>

  Test Files  1 failed (1)
       Tests  1 failed | 3 passed (4)
```

Both toasts in one document — the exact user-visible defect the review
described — and the spy never fired. Restored:

```
✓ src/components/admin/governance/TimelineView.test.tsx (4 tests) 191ms
 Test Files  1 passed (1)
      Tests  4 passed (4)
```

### The record was corrected, not deleted

`47-04-SUMMARY.md` asserted: *"A completed rollback can no longer be reported as
a failed one."* That was **false of the committed code**. The claim is kept
verbatim and annotated with a dated correction block stating what was wrong,
what the true invariant is, and which test now pins it. The superseded
key-decision about callback ordering is marked `[SUPERSEDED]` in the same file.
A wrong claim that was caught and corrected on the record is worth more than a
claim quietly rewritten.

---

## Cross-cutting verification

| Gate | Result |
|---|---|
| `cd web && npx tsc --noEmit` | exit **0** |
| `cd web && npm test` | exit 0 — **85 files / 689 tests passed, 0 failed, 0 skipped** (688 baseline + 1 new case) |
| `bash verify-phase-47.sh` (real PATH) | exit **0** — full output above |
| `bash -n verify-phase-47.sh` | syntax OK |
| `git status --porcelain` (tracked) | clean after both commits; no `git add -A` / `git add .` was used |
| `.planning/STATE.md` / `.planning/ROADMAP.md` | not modified |

Both fixes were verified in a clean `HEAD` worktree state before committing —
the gate proof above ran against the committed script, and the vitest proof
above ran against the committed component.

## Left open, deliberately

| Finding | Status |
|---|---|
| MEDIUM-2 — `keyword_cooccur` representative-chunk attachment depends on input order (`graph_edges.go:232-241`) | **open, untouched** |
| LOW-1 — the `GovernanceTab` → `onRollbackComplete` wiring has no test | open, untouched |
| LOW-2 — `verify-phase-47.sh` hardcodes `/mnt/d/CODE/ClaudeCode/ai-agent` | open, untouched |
| LOW-3 — the `induction.go:489` comment's claim (b) is false on the failure branch | open, untouched |
| INFO-1 / INFO-2 / INFO-3 | open, informational |

`47-REVIEW.md`'s frontmatter is now `status: partially_resolved` with a
`resolution:` block naming both commits and an explicit `open:` list. It was
**not** marked `clean`, because it is not.
