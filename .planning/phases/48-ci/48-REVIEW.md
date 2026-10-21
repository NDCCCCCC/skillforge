---
phase: 48-ci
reviewed: 2026-10-21T09:00:10Z
depth: standard
files_reviewed: 1
files_reviewed_list:
  - verify-phase-48.sh
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 48: Code Review Report

> **Round 7 is the current review.** The frontmatter counts above describe the round-7 incremental
> review only (delta `80990b1..HEAD`, one source file: `verify-phase-48.sh`). The round-6 findings are
> dispositioned in `48-REVIEW-DISPOSITION-R6.md` and are **not** re-opened here; the round-6 and
> round-5 sections further down are retained verbatim for provenance only (round 5:
> `48-REVIEW-DISPOSITION-R1.md`). `48-REVIEW-DISPOSITION.md` is the live round-7 ledger;
> `48-REVIEW-DISPOSITION-R1.md` and `48-REVIEW-DISPOSITION-R2.md` are the older rounds' ledgers.

## Round 7 — Incremental Review (gap-closure)

**Reviewed:** 2026-10-21T09:00:10Z
**Depth:** standard
**Files Reviewed:** 1
**Scope:** `git diff 80990b13499f862e8a633490e00d592867841f44..HEAD -- verify-phase-48.sh`
**Status:** issues_found

### Summary

Round 7 changed one source file: `verify-phase-48.sh` (+419 / -47). No workflow file changed
(`git diff 80990b1..HEAD -- .github/` is empty). All three round-6 findings are closed in the code,
and two of the three new P11 fixtures are as isolating as their comments claim. What the delta did
not finish is the job it announced: the comment at `verify-phase-48.sh:2320-2321` states "one
fixture per RULE, each with its own class token, so deleting one rule cannot be masked by a sibling
depositing PASS" — but the trust surface has FOUR write-permission rules, not three. The
workflow-level `permissions:` mapping-value branch has no fixture, no class token and no drill, and
deleting it leaves the gate green while a top-level write scope in a shape the text rule cannot see
is accepted (WR-01).

The second defect is a side effect of the new shared stripper. The stripper itself is **correct**:
the NORMAL/SINGLE/DOUBLE state machine, the `''` escape, the backslash escape and the
unterminated-quote fallback all behave exactly as documented — no correctness bug was found in it.
But it changes what the retained TEXTUAL secret scan can see, which falsifies two of the file's own
fixture-isolation comments and silently moves the R10 drill's red point (WR-02).

Round-6 status:

* **CR-01 — closed.** The trigger judgment now runs on the PARSED `on:` document
  (`doc.get("on", doc.get(True))`, covering mapping key / sequence element / bare scalar) and the
  textual layer only supplies file:line evidence. The flow-style positive canary
  (`verify-phase-48.sh:481`) reproduces the CR-01 shape and now reds in both layers, where the
  quote-blind regex left the text layer at rc 0.
* **WR-01 — closed for three of the four write-permission rules**: the `WRITE_VALUE` line scan →
  `perm-write-line` / `P11-PERMSWRITE-OK`; the job-level mapping walk → `perm-job-write` /
  `P11-JOBPERMS-OK`; the job-level string branch → `perm-job-readall` / `P11-JOBPERMS-STR-OK`; and
  `yaml-ext` now pays its own `P11-YAMLEXT-OK`. The fourth rule is not closed (WR-01 below).
* **WR-02 — closed.** `p4_cfg_bytes=$(wc -c < "$WORK/p4-config.log" | tr -d ' \r\n')`
  (`verify-phase-48.sh:914`) is sampled BEFORE the two appends, so the evidence line reports the
  tool's own output size instead of a self-fulfilling word. A healthy run legitimately reads
  `tool output: 0 byte(s)` — `golangci-lint config verify` prints nothing on success.

The count and class-token bookkeeping is internally consistent: the authoritative count sites agree
(17 fixtures, "4 vacuity + 13 per-file" at `:2413`, `:2443`, `:2476`) and the 8 class tokens in the
reason map (`:2363-2377`) are exactly the 8 `ok_partition` requires (`:2424`). One stale comment
survives (IN-01).

**Findings in this round: 0 critical, 2 warnings, 4 info (6 total).**

## Critical Issues

None. The round-6 critical finding (CR-01) is closed in the code, and no new critical defect was
found: the parsed `on:` judge cannot be defeated by quoting style, and the shared stripper's state
machine is correct on every branch it implements.

## Warnings

### WR-01: The workflow-level `permissions:` mapping-value branch has no fixture and no class token — deleting it keeps the gate green while a top-level write scope is accepted

**File:** `verify-phase-48.sh:1501-1506` (the unpinned branch), `:1420` (`WRITE_VALUE`), `:1474-1485`
(the `WRITE_VALUE` line scan that claims the only candidate fixture first), `:2052-2063`
(`perm-write-line`), `:1524-1530` (the pinned job-level twin), `:2320-2321` (the invariant the delta
asserts)

**Issue:** The trust surface carries four write-permission rules. Round 7 gave three of them an
isolating fixture and a class token of their own. The fourth — the workflow-level mapping VALUE
check — got neither:

```python
        for scope, value in perms.items():
            if value not in ("read", "none"):
                raise SystemExit(
                    "trust-surface: %s in workflow-level grants %r=%r; only read/none are allowed (D-26/D-29)"
                    % (p, scope, value)
                )
```

`perm-write-line` is the only fixture whose top-level `permissions:` mapping carries a write value,
and it never reaches this branch: the `WRITE_VALUE` line scan runs earlier in the same
`for p in paths:` walk and claims the line `  contents: write` — `WRITE_VALUE =
re.compile(r"^\s*[A-Za-z0-9_-]+:\s*write(-all)?\s*$")` (`:1420`) matches it. `empty-perms` covers
only the `not perms` branch, `no-perms` only the `perms is None` branch. The job-level twin of the
same rule IS pinned: `perm-job-write` carries a QUOTED key (`"contents": write`), which
`WRITE_VALUE`'s key pattern `[A-Za-z0-9_-]+` cannot match at any indentation, so only the parsed
job walk can reject it.

**Failure mode:** delete `:1501-1506` and every one of the 17 fixture verdicts is unchanged —
`perm-write-line` is still rejected by `WRITE_VALUE` with its registered reason `contains a write
scope`, `perm-job-write` and `perm-job-readall` are still rejected by the job-level walk, the
fixture count stays 17, and all 8 class tokens still deposit → the gate still prints
`=== Phase 48 Gate: ALL PASSED ===`. The rule that was deleted is the ONLY thing that rejects a
workflow-level write scope written in a shape `WRITE_VALUE` cannot see. Verified by simulating the
scan ordering read-only (no gate run, no file written): with the branch present,
`permissions: {contents: write}` and `permissions:\n  "contents": write` are rejected with
`in workflow-level grants 'contents'='write'`; with the branch deleted, neither line matches
`WRITE_VALUE` (its pattern requires the WHOLE line to be `key: write` with an unquoted key — the
flow form starts `{`, the quoted-key form starts `"`) and the workflow is accepted. That is a
D-26/D-29 ceiling violation the gate would report as clean.

**Fix:** add an 18th fixture whose top-level `permissions:` mapping carries a write value the text
rule cannot see — e.g.

```yaml
permissions:
  "contents": write
```

registered `want="in workflow-level grants 'contents'='write'"` with its own class token (e.g.
`P11-WF-PERMS-OK`), and raise the count assertions to 18. `packages: write` must NOT be used — it is
a `PUBLICATION_TOKEN` and would be claimed by the publication rule first. Alternatively, extend the
`T-48-84` marker range so the branch is provably live.

### WR-02: The new quote-aware stripper falsifies the two `quoted-hash-secret` isolation comments — the fixture no longer isolates the parsed secret walk

**File:** `verify-phase-48.sh:2152-2157` (the group comment), `:2208-2210` (the fixture comment),
`:2206-2219` (the fixture), `:1605-1620` (the parsed secret walk), `:1638-1645` (the retained
textual secret scan), `:2371` (the registered reason)

**Issue:** Two comments still assert that the textual comment strip HIDES the `secrets.DEPLOY_KEY`
reference in the `quoted-hash-secret` fixture:

* `:2155-2157` — "the two secret fixtures carry no `secrets.NAME` reference the retained textual
  SECRET_REF scan can see — `toJSON(secrets)` has no name after the dot, and in quoted-hash-secret
  the `#` starts a comment for the comment-stripping while the parsed document still holds the whole
  value."
* `:2208-2210` — "quoted-hash-secret/ — `#` inside a quoted scalar is data, so the textual comment
  strip hides the reference while GitHub still executes it (T-48-04, parsed walk)."

Both were true of the quote-blind `re.sub(r"(?m)#.*$", "", src)` and are false of the new shared
stripper. The stripper is quote-aware: the `#` in `run: "echo x# ${{ secrets.DEPLOY_KEY }}"` sits
inside a double-quoted scalar, so the line is emitted INTACT and the retained textual `SECRET_REF`
scan (`:1638`) now sees `secrets.DEPLOY_KEY` itself. The round-7 matrix already records this — R21's
red is the TEXT-layer wording `references secrets.DEPLOY_KEY` (no `parsed value`), and R21-counter
proves the layer switch by reverting the second call site to quote-blind
(`.planning/phases/48-ci/48-27-SUMMARY.md:97-99`). The source comments and the recorded evidence
now disagree.

**Failure mode:** the fixture no longer isolates the parsed secret walk, although the file says it
does. With the parsed walk deleted, `quoted-hash-secret` is still rejected — by the TEXT layer — and
still matches its registered reason `references secrets.` (`:2371`), so the P11 reason map can no
longer distinguish a parsed rejection from a textual one for this fixture. In audit terms the file's
own coverage argument ("so the only reason it can be rejected is the rule under test", `:2152-2153`)
is false for this fixture, and a reader who trusts it would conclude that the textual layer cannot
see a quoted `#` — the exact belief CR-01 was filed against, written back into the file. The parsed
walk itself is still pinned (`tojson-secrets` and `dyn-secrets` are invisible to the textual layer,
so deleting it still reds the gate); what is broken is the isolation argument and the two comments
that carry it.

**Fix:** correct both comments to the new reality — the `#` is data, the textual layer sees the
whole scalar, and the fixture pins the secret RULE rather than the parsed walk — and move the "the
textual scan cannot see this" claim onto `tojson-secrets` / `dyn-secrets`, which still carry it. If
a quoted-`#` fixture that ONLY the parsed walk can reject is wanted, that requires the
quote-blind truncation the delta deliberately removed, so the honest fix is to re-scope the comments
rather than to weaken the stripper.

## Info

### IN-01: The fixture-count comment still says "fourteen rejections" while the loop now runs 17

**File:** `verify-phase-48.sh:2411` (stale comment), `:2413` (the assertion, 17), `:2443` and
`:2476` (the banner texts, 17 = 4 vacuity + 13 per-file)

**Issue:** the comment directly above the count assertion still reads "The fixture loop is a probe
too: fourteen rejections are its success output" — the count changed 14 → 17 in round 7 and this
comment was not updated. The three authoritative sites agree with each other (`:2413` asserts
`-eq 17`; `:2443` and `:2476` both say 17 = 4 + 13; the 8 class tokens in the reason map at
`:2363-2377` are exactly the 8 `ok_partition` requires at `:2424`), so the gate is unaffected. But
this file's standard is that its comments are the audit trail — the comment at `:2411` is the one
that explains the count assertion, and it now contradicts it.

**Fix:** "seventeen rejections are its success output".

### IN-02: The stripper's quote states are unexercised — every canary and fixture passes with SINGLE/DOUBLE/escape handling deleted, so the recorded R21 proof over-claims

**File:** `verify-phase-48.sh:98-206` (the helper), `:479-482` (the four canary files),
`:2206-2219` (the only quoted-`#` fixture), `.planning/phases/48-ci/48-27-PLAN.md:368` (the R21
claim)

**Issue:** the state machine's SINGLE and DOUBLE branches, the `''` escape, the backslash escape and
the unterminated-quote fallback have no test at all. None of the four canaries contains a double
quote (canaries 1 and 4 have no quotes; canaries 2 and 3 use single quotes), and the only `#`
inside a quoted scalar anywhere in the fixture set (`quoted-hash-secret`, plus canaries 2 and 3) is
preceded by a NON-whitespace character (`x`, `n`) — so the whitespace rule alone explains every
observed verdict. Verified by simulation (the helper extracted from the script and executed in a
throwaway namespace; no file written): a mutant that keeps the whitespace rule and deletes the whole
quote state machine produces the IDENTICAL canary verdicts (1, 1, 0, 0) and the identical
`quoted-hash-secret` behaviour.

**Failure mode / consequence for the evidence:** R21 is described as proving "the shared strip is
quote-aware at the second call site too" (`48-27-PLAN.md:368`), but what R21 actually distinguishes
is the new whitespace rule from the old quote-blind regex — not quote-awareness from
whitespace-awareness. The drill is still useful (it does red when the second call site is reverted
to quote-blind), but the claim attached to it is stronger than the drill supports. Practically: a
future simplification of the helper that drops the quote tracking — or breaks the `''`/backslash
handling — would not be caught by the gate. The verdict impact is nil today (the parsed walks are
the primary judges), but the file:line evidence the text layer supplies would silently degrade for
a `#` preceded by whitespace inside a quoted scalar (`run: "echo hello # world"`, where the
quote-aware strip keeps the tail and a whitespace-only strip truncates it).

**Fix:** add a canary whose `#` is preceded by whitespace INSIDE a quoted scalar (e.g.
`on: {push: {branches: ["main # hotfix"]}, pull_request_target: {}}`) and one that exercises the
`''` escape, so the two rules (whitespace boundary, quote state) become separable by the gate; and
re-word the R21 row in `48-27-SUMMARY.md` to what it actually proves.

### IN-03: The P1 text layer is still a red-capable judge with a whole-line literal test, and the quote-aware stripper widens what counts as a non-comment line

**File:** `verify-phase-48.sh:401-419` (the text probe), `:511-557` (the per-file loop and the
failure message at `:545-552`)

**Issue:** the text probe's hit condition is "the literal `pull_request_target` appears on a line
that is not a comment" — not "is the event wired up". The new stripper keeps a `#` inside a quoted
scalar as data, so the rest of that line now counts as non-comment text where the quote-blind strip
used to truncate it. A workflow that merely NAMES the event inside a quoted string — e.g.
`run: "echo '# pull_request_target is banned'"` — is now RED where the quote-blind strip left it
green (verified: the stripper emits that line intact). The parsed judge is the exact one here; the
text layer only needs to supply file:line evidence, but it can still fail the gate on its own
(`p1_hits=1`).

Two related details: (a) no canary covers the new shape — canary 3 is a quoted `#` with NO event
and canary 4 is a whole-line comment, so the "quoted `#` plus a mere mention" case is unpinned in
either direction; (b) the failure message states the event was "reported by the parsed on: mapping
AND by quote-aware comment-stripped text", but the loop only requires that EITHER layer reported it,
so a text-layer-only hit prints a message claiming a parsed-layer verdict that did not happen.

**Fix:** restrict the text layer to evidence (the parsed judge already decides the verdict), or
require the match to sit in a key position; and make the failure message report which layer(s)
actually reported — e.g. separate `p1_text_hits` / `p1_parsed_hits` counters — instead of asserting
both.

### IN-04: The P4 byte count is measured but never asserted, and a failed measurement degrades to an empty number instead of a refusal

**File:** `verify-phase-48.sh:914` (the measurement), `:916-918` (the evidence line), `:68`
(`set -uo pipefail`, no `set -e`)

**Issue:** round 6's WR-02 is genuinely closed — the count is sampled before the appends, `wc -c`
measures the tool's own output, and the expression is portable across GNU and BSD `wc` (`wc -c <
file` → padded number → `tr -d ' \r\n'`). Two residuals remain: (a) the number is reported but
never asserted anywhere, so a neutered tool that exits 0 and prints nothing is indistinguishable
from a healthy one — which is why the recorded R20 / R20b are green drills judging the EVIDENCE LINE
rather than the exit code (`.planning/phases/48-ci/48-27-SUMMARY.md:95-96`); (b) with
`set -uo pipefail` and no `set -e`, a failing `wc` leaves `${p4_cfg_bytes}` empty and the evidence
line reads `tool output:  byte(s)` — the gate stays green on a broken measurement, and the log
then carries a nonsense number instead of a refusal.

**Fix:**

```bash
p4_cfg_bytes=$(wc -c < "$WORK/p4-config.log" | tr -d ' \r\n')
case "$p4_cfg_bytes" in (''|*[!0-9]*) fail "P4 could not measure the config-verify output size (got '${p4_cfg_bytes}')" ;; esac
```

### Round-7 verification evidence

Read-only verification was used throughout; no file was modified except this report.

| Probe | Method | Observed result |
|---|---|---|
| S0 | `bash -n verify-phase-48.sh` | SYNTAX-OK |
| S1 | read the whole current file + `git diff 80990b1..HEAD -- verify-phase-48.sh` (+419 / -47) | delta confined to `verify-phase-48.sh`; `git diff 80990b1..HEAD -- .github/` is empty |
| S2 | extracted the `YAMLSTRIP` helper from the script and executed it in a throwaway Python namespace (no file written) | `''` escape, backslash escape, unterminated-quote fallback and the quoted-`#` all behave as documented — no correctness bug in the state machine |
| S3 | same harness, "whitespace-rule-only" mutant (quote states deleted) | canary verdicts IDENTICAL (1, 1, 0, 0) and `quoted-hash-secret` IDENTICAL → the quote states are unexercised (IN-02) |
| S4 | same harness, `WRITE_VALUE` against the three top-level write shapes | `permissions: {contents: write}` → no match; `permissions:\n  "contents": write` → no match; `permissions:\n  contents: write` → match → only the deleted branch at `:1501-1506` can reject the first two (WR-01) |
| S5 | in-memory simulation of the `scan_trust_surface` rule ordering, with and without `:1501-1506` | `perm-job-write` / `perm-job-readall` / `empty-perms` / `no-perms` verdicts unchanged; `perm-write-line` stays rejected (by `WRITE_VALUE`, with its registered reason) → the branch is deletable with no fixture change (WR-01) |
| S6 | counted the loop list, the `-eq 17` assertion, both banner texts, the 8 reason-map class tokens and the 8 `ok_partition` tokens | 17 / 17 / "4 + 13" / 8 = 8 agree; one stale comment says "fourteen" (IN-01) |
| S7 | inspected the four canary files and the four `T-48-84` marker ranges | marker ranges now at column 0; the canaries use single quotes or none (IN-02) |
| — | full gate run / mutation drills | NOT run (slow, and out of scope for this review) — the mutation claims above are derived from the rule ordering and the fixture set, not from an executed mutation |

---

## Round 6 — Incremental Review (gap-closure)

**Reviewed:** 2026-10-21T02:15:44Z
**Depth:** standard
**Files Reviewed:** 2
**Scope:** `git diff 838766fd..HEAD -- verify-phase-48.sh .github/workflows/ci.yml`
**Status:** issues_found

### Summary

Round 6 changed two files. `verify-phase-48.sh` rebuilt the P1 trigger scan (comment-stripped,
per-file, live canary, decisive-success assertions), pinned the verify job's install-step `env:` map
by exact equality (`P3-INSTENV-OK`), bound the config-verify group to its own token
(`P4-CONFIG-OK`), added the two quoted publication spellings, hoisted the single `basename` binding
above the drill-deleted marker range, and rebuilt the P11 fixture loop (per-fixture reason map,
file-presence guard, five per-class tokens, 12→14 fixtures). `ci.yml` changed one comment only; the
job spans are byte-identical.

**The gate is green on this host** (`bash verify-phase-48.sh`: exit 0, one `ALL PASSED`, 11/11
registry/begin/token, 14/14 fixtures, ~34 s with golangci-lint 2.12.2 / PyYAML 6.0.3 / Python
3.13.2). Most of the round-6 claims hold under mutation — the install-env pin, the `T-48-81` marker
range, the fixture-presence guard, the reason map, the `secrets[` fixture and the quoted-push
fixture all bite (see the drill table at the end). The `ci.yml` comment's factual claims are backed
by the 48-15 run record (run 3 = 39353593172, fast-forward push, lint green).

Three defects remain, one of them on the security-critical rule the delta rewrote:

* **CR-01** — the new comment-stripped P1 scan can miss a **live** `pull_request_target` trigger.
  Demonstrated end-to-end: a workflow whose trigger is a flow mapping with `#` inside a quoted
  scalar earlier on the same line passes P1 and the whole gate.
* **WR-01** — the new `P11-PERMSWRITE-OK` class token is deposited by a *publication-rule*
  rejection (`yaml-ext` → `packages: write`). Both permission assertions it appears to cover can be
  deleted while the gate stays green (two mutation runs).
* **WR-02** — the P4 config-verify evidence line re-stats the log **after** appending to it, so it
  always prints `tool output: present`; the `none (success is silent)` branch is unreachable.

The four Info items are one un-pinned sibling spelling, one diagnostic mislabel, one pre-existing
matcher-breadth residual the new fixture now certifies, and one over-generalized sentence in the new
ci.yml comment.

### Critical Issues

#### CR-01: The comment-stripped P1 scan misses a live `pull_request_target` when a `#` appears in a quoted scalar earlier on the same line — the whole gate reports green with the banned trigger present

**File:** `verify-phase-48.sh:276-290` (probe), `:281` (the strip), `:269-272` (the false
invariant), `:296-301` (canary), `:305-317` (per-file loop)

**Issue:** The new probe judges `re.sub(r"(?m)#.*$", "", src)`. That pattern is not quote-aware, so a
`#` inside a quoted YAML scalar truncates the rest of the physical line — including a real trigger
key on that line. The comment at `:269-272` asserts the opposite as an invariant: “The false-negative
direction stays closed: a real trigger mapping is executable YAML, which the strip never touches.”
That sentence is false.

Verified end-to-end (not just in isolation). With this file present in
`.github/workflows/`:

```yaml
name: zz-review-probe
on: {push: {branches: ['main#hotfix']}, pull_request_target: {}}
permissions:
  contents: read
jobs:
  b:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
```

the gate prints

```
PASS P1 workflow-parse (3 workflow(s) *.yml / *.yaml + composite parse, zero pull_request_target; ...)
=== Phase 48 Gate: ALL PASSED ===
```

The document parses (`yaml.safe_load` sees `on = {'push': {...}, 'pull_request_target': {}}`, so the
trigger block check passes), the strip deletes `pull_request_target: {}` from the judged text, and
no other partition inspects triggers (P8 only judges permissions / publication / paths / secrets).
The canary at `:296` only covers the ordinary block-style/hit direction, so it cannot detect this.
The shape is unusual, but it is legal YAML and a legal GitHub trigger mapping, and D-11 is the one
security rule this partition exists for. A gate that prints “zero pull_request_target” for a tree
that contains one is the “found nothing, reported green” class the file header says the phase closes.

**Fix:** judge the parsed document, which P1 already loads. In the per-file loop, after
`yaml.safe_load`, reject `pull_request_target` as a key of the parsed `on:` mapping (and as an
element of a list-valued `on:`), and keep the textual scan only for file:line evidence:

```python
trigger = d.get("on", d.get(True))
if isinstance(trigger, dict) and "pull_request_target" in trigger:
    raise SystemExit("%s: `on:` mapping declares pull_request_target (D-11)" % path)
if isinstance(trigger, list) and "pull_request_target" in [str(x) for x in trigger]:
    raise SystemExit("%s: `on:` list declares pull_request_target (D-11)" % path)
```

Also extend the canary set with a flow-style positive (`on: {push: {branches: ['x#y']},
pull_request_target: {}}`) so the quote-aware behaviour is itself proven live. If the textual scan
is kept, make the strip quote-aware (track `'`/`"` state) rather than a bare `#.*$` regex — and
preferably share one helper with `scan_trust_surface`, which carries a third copy of the same
idiom (`:1207`).

### Warnings

#### WR-01: `P11-PERMSWRITE-OK` is deposited by the publication rule, so neither permission assertion has a fixture — deleting either keeps the gate green

**File:** `verify-phase-48.sh:2000` (`yaml-ext` arm), `:1993-2007` (reason map), `:2052` (deposit),
`:1229-1234` (write-scope line scan), `:1265-1284` (job-level permissions walk)

**Issue:** The round-6 class-token scheme requires
`P11-VACUITY-OK / P11-PATHFILTER-OK / P11-PERMSWRITE-OK / P11-PUBLICATION-OK / P11-SECRET-OK`, and
the name `PERMSWRITE` reads as the permission-write rule. It is not. The only fixture carrying that
token is `yaml-ext`, whose live first line is:

```
contains forbidden registry/publication token 'packages: write' (case-insensitive match, T-48-10)
```

because `packages: write` is itself in `PUBLICATION_TOKENS` and that check runs first. So the token
certifies the publication rule a second time (`P11-PUBLICATION-OK` already exists). Meanwhile the two
permission assertions are exercised by **no** fixture. Two mutation runs prove it, each ending in
`=== Phase 48 Gate: ALL PASSED ===`:

1. delete the `WRITE_VALUE` loop at `:1229-1234` (the D-26/D-29 “value-shaped write” scan);
2. delete the job-level permissions walk at `:1265-1284` (including the string-shorthand and
   unknown-shape branches added for IN-03 in round 5).

Deleting the publication `packages: write` token *does* red the gate, but only because `yaml-ext`
falls through to `WRITE_VALUE` and then fails its registered reason — i.e. the coverage is a side
effect, not a pin. This is the same evidence-vs-attribution hole WR-R5-01 was raised for, moved one
level up into the class tokens.

**Fix:** add fixtures that isolate each assertion, appended at the end of the loop list with their
own reason and token:

* write-scope scan — a fixture with a job-level `permissions: {id-token: write}` (not in
  `PUBLICATION_TOKENS`), registered `want='contains a write scope'`;
* job-level permission walk — a fixture with the **string shorthand** `permissions: read-all` on a
  job (not matched by `WRITE_VALUE`, which needs `key: write`, and not a publication token),
  registered `want="grants 'read-all'"`;
* then rename or re-scope `P11-PERMSWRITE-OK` so it is deposited by those fixtures, not by
  `yaml-ext` (give `yaml-ext` its own `.yaml`-enumeration token if that is what it is for).

#### WR-02: The P4 config-verify evidence line re-stats a log the same block is appending to, so it always claims `tool output: present`

**File:** `verify-phase-48.sh:681-684` (the reporting block), `:679-687` (the marker range)

**Issue:** The block is:

```bash
{
  echo "exit code: ${p4_cfg_rc}"
  echo "tool output: $( [ -s "$WORK/p4-config.log" ] && echo present || echo 'none (success is silent)' )"
} >> "$WORK/p4-config.log"
```

The group's append redirection is set up before the first command, and the first `echo` writes
`exit code: 0` into the same file. The command substitution in the second `echo` then tests
`[ -s "$WORK/p4-config.log" ]`, which is already true. On a successful `golangci-lint config verify`
the tool itself prints nothing (log size 0 bytes — confirmed), yet the evidence line says `present`.
The `none (success is silent)` branch is unreachable, so the one line added specifically so “an
auditor can still see what was judged” (`:666-668`) records a false fact. The exit code and the
`P4-CONFIG-OK` token are unaffected; this is an evidence-integrity defect, not an exit-status one.

**Fix:** capture the tool's output state *before* appending:

```bash
golangci-lint config verify ... > "$WORK/p4-config.log" 2>&1
p4_cfg_rc=$?
if [ -s "$WORK/p4-config.log" ]; then p4_cfg_out='present'; else p4_cfg_out='none (success is silent)'; fi
{
  echo "exit code: ${p4_cfg_rc}"
  echo "tool output: ${p4_cfg_out}"
} >> "$WORK/p4-config.log"
```

### Info

#### IN-01: The double-quoted publication spelling `push: "true"` has no fixture — deleting that one token keeps the gate green

**File:** `verify-phase-48.sh:1172` (token), `:1903-1922` (the only fixture, which uses the
single-quoted spelling), `:2004` (its reason arm)

**Issue:** Round 6 added two sibling spellings, but `quoted-push` exercises only `push: 'true'`. A
mutation that deletes **only** line 1172 (`'push: "true"'`) runs to `ALL PASSED` — verified. The
script's own WR-03/M13 standard (“one token per assertion group, not one per probe”) argues for
either a second fixture or a fixture carrying both spellings and a per-spelling token.

**Fix:** add a `double-quoted-push` fixture (or a second step in `quoted-push`) with
`push: "true"`, registered against `registry/publication token`; if the tokens are considered one
group, say so in the comment instead of leaving the appearance of per-spelling coverage.

#### IN-02: P1 maps “probe crashed” onto “trigger found” — rc 1 is overloaded, so an undecodable workflow file is reported as a `pull_request_target` hit

**File:** `verify-phase-48.sh:277-290` (`raise SystemExit(1)` at `:288`), `:309-316` (the `case`)

**Issue:** The probe uses rc 1 for a hit (`SystemExit(1)`), but Python also exits 1 on an uncaught
exception. A workflow file that is not valid UTF-8 raises `UnicodeDecodeError`, returns rc 1, and
the loop takes the `1)` arm (`p1_hits=1`) — so the operator sees
`P1 pull_request_target found (D-11)` with a traceback in the evidence instead of “the probe could
not read this file”. Verified: a file with byte `0xff` returns rc 1. Fail-closed (the gate reds), so
this is diagnostic only, but it contradicts the decisive-result discipline the surrounding comments
describe.

**Fix:** return distinct codes (0 clean / 1 hit / 2 internal error) and refuse anything but 0/1 in
the `case`; or have the probe print `P1-PRT-HIT: path:line: text` and have the shell require that
prefix for the hit branch, so an uncaught exception (no prefix) is refused as non-decisive.

#### IN-03: The secret-expression matchers are literal-spelling; whitespace-separated spellings evade the whole trust surface (pre-existing rule, now certified by the new `dyn-secrets` fixture)

**File:** `verify-phase-48.sh:1178` (`SECRET_REF`), `:1365` (`"secrets[" in s`), `:1371-1372`
(`tojson(secrets`), `:1924-1939` (the `dyn-secrets` fixture)

**Issue:** The `secrets[` branch added by round 5 (and pinned by round 6's `dyn-secrets` fixture) is a
literal substring test, and the `SECRET_REF` regex requires `secrets\.` with no whitespace. GitHub
expression whitespace is significant only inside string literals, so bracket/dot access with spaces
is executable but unmatched. Verified against the full gate with a third workflow present:

* `run: echo "${{ secrets [ 'DEPLOY_KEY' ] }}"` → `PASS P8 trust-surface` and `ALL PASSED`;
* `run: echo "${{ secrets . DEPLOY_KEY }}"` → `PASS P8 trust-surface` and `ALL PASSED`.

(The first form is the direct sibling of the fixture's `secrets['DEPLOY_KEY']`; the second is the
dot form with spacing.) This predates the round-6 delta — it is the breadth of the matcher, not the
new attribution loop — but round 6 is where `dyn-secrets` became the evidence for that branch, so it
is recorded here rather than silently assumed covered. No other partition judges this shape.

**Fix:** normalize whitespace out of each parsed string value before the three secret tests
(`re.sub(r'\s+', '', s)`), or tokenize the `${{ }}` expression; then add one spaced-index fixture so
the normalization itself is drilled. (If the project prefers to defer this as a separate threat row,
record it explicitly — do not let the `P11-SECRET-OK` token imply it is covered.)

#### IN-04: The new ci.yml push comment over-generalizes the force-push case (“has no common ancestor”)

**File:** `.github/workflows/ci.yml:41-51` (claim at `:46-47`)

**Issue:** The comment's operational conclusion is right and the round-5 finding is corrected
explicitly at `:49-50` (“limiting the trigger to main does not by itself prevent that fallback; the
force-push discipline does”), and the run-3 measurement it cites is real (`48-15-SUMMARY.md`: run
39353593172, fast-forward push, lint green). One sentence is over-broad: “A FORCE push to main has no
common ancestor”. That was true for the 48-15 history rewrite (run 1), but a rebase-style force push
keeps the rebase base as a common ancestor; the Compare API then succeeds and the diff spans the
whole rewritten range — still a whole-repo-scale lint surface, reached differently. An editor who
trusts the absolute sentence could conclude that only no-common-ancestor force pushes are risky.

**Fix:** “A history-rewriting force push (no common ancestor) fails the diff fetch; any other force
push widens the diff to the rewritten range. Either way, main is never force-pushed.”

### Round-6 verification evidence

All drills were run on this host against the current tree; temporary mutants/workflow files were
removed afterwards (`git status` clean for both scoped files).

| Drill | Mutation | Observed result |
|---|---|---|
| D0 baseline | none | exit 0, one `ALL PASSED`, 11/11 registry/begin/token, 14/14 fixtures |
| D1 | delete the `WRITE_VALUE` loop | **ALL PASSED** (WR-01) |
| D2 | delete the job-level permissions walk | **ALL PASSED** (WR-01) |
| D3 | delete the `# T-48-81` marker range | FAIL: P4 missing `P4-CONFIG-OK` (correct) |
| D4 | remove ci.yml install-step `env:` block | FAIL: P2/P3 contract (correct) |
| D5 | delete the `dyn-secrets` heredoc, keep its dir | FAIL: “fixture has no workflow file” (correct) |
| D6 | delete only `'push: "true"'` | **ALL PASSED** (IN-01) |
| D7 | temp workflow; comment mentions the event | PASS (IN-R5-03 false positive fixed) |
| D8 | temp workflow; `on: [push, pull_request_target]` | FAIL: P1 file:line (correct) |
| D9 | temp workflow; flow trigger, `#` earlier on the line | **ALL PASSED** (CR-01) |
| D10 | temp workflow; `secrets [ 'DEPLOY_KEY' ]` | **ALL PASSED** (IN-03) |
| D11 | temp workflow; `secrets . DEPLOY_KEY` | **ALL PASSED** (IN-03) |

---

## Round 5 — Historical Review (dispositioned; retained for provenance)

> The findings below were dispositioned in `48-REVIEW-DISPOSITION-R1.md` / `48-REVIEW-DISPOSITION.md`.
> They are kept verbatim for the audit trail and are not part of the round-6 counts.

### Summary (round 5)

Round-5 delta reviewed at standard depth: `verify-phase-48.sh` and `.github/workflows/ci.yml`
(diff base `831d912`, plans 48-20/48-21/48-22).

**The four targeted fixes are correct, and this time they were verified by execution, not just
reading.** The full gate was run on this host (golangci-lint 2.12.2, PyYAML 6.0.3, Python 3.13.2,
Git Bash): exit 0, exactly one `ALL PASSED`, registry/begin/token counts 11/11/11, and all 12
P11 fixtures rejected — with the four new evasion fixtures rejected through the NEW parsed-document
code paths, as the plan claims:

* **WR-01 (else-detector SIGPIPE)** — fixed at `verify-phase-48.sh:1556-1568`. The range is
  materialised into `BANNER_RANGE` and fed to a non-quiet grep that reads to end-of-input, so the
  141-on-EPIPE failure mode is gone. `BANNER_SITES == 1` is asserted first, so the remaining
  `head -1` pipes cannot close early on a multi-match run.
* **WR-02 (text-scan evasion shapes)** — fixed at `:1178-1261`. The parsed `on:` walk and the
  recursive parsed-string secret walk are primary, the retained textual rules stay as a second
  layer. Live-run evidence: `quoted-paths` and `space-paths` were rejected with the parsed-walk
  message ("filters event 'pull_request' with a 'paths' key"), `tojson-secrets` and
  `quoted-hash-secret` with the parsed secret-walk message — the textual layers provably did not
  produce those verdicts.
* **WR-03 (per-assertion tokens)** — fixed at `:128-151` (`ok_partition` now takes a required
  token list) and at every deposit site. Every partition's assertion groups now carry a distinct
  token; I traced each group→token mapping (P1 3, P2 4, P3 4, P4 4, P5/P8/P10/P11 1 each,
  P6 4, P7 6, P9 6) and found one residual gap (IN-R5-02), not a regression.
* **IN-01 (install env) / IN-02 (ok_partition arity)** — fixed at `ci.yml:242-245` and
  `:128-131`. The new ci.yml comment's factual claims check out: `go.mod` does pin `go 1.25.5`
  (`go.mod:3`), and the environment variables are step-scoped so they cannot leak to other steps.

No round-4 finding was reintroduced. The two new warnings are gaps *in the verification
mechanism this delta added*, not in CI itself: the fixture loop counts any rejection as a valid
pin (so the pins themselves can be silently removed), and the security-critical env declarations
added by 48-22 are pinned by nothing. Also carried forward: round-4 IN-03 and IN-07 are still
present in these two files and the on-disk disposition ledger (`48-REVIEW-DISPOSITION.md`)
still records every row `open`, so they are re-raised here with provenance rather than lost.

Verification method: full-file reading of both files, cross-checks against `go.mod`, the
round-4 ledger, and the plan summaries, plus execution of `bash verify-phase-48.sh` on this host
and targeted greps (`import re` usage, `GOTOOLCHAIN`/`GOSUMDB`/`GOPROXY` occurrences in the gate).

### Warnings (round 5)

#### WR-R5-01: The P11 fixture loop cannot attribute rejections — a deleted or emptied fixture counts as a pin, so the 12 evasion fixtures can be silently removed while P11 stays green

**File:** `verify-phase-48.sh:1786-1809` (loop), `:1814` (deposit), `:1011-1016` (the zero-files rejection branch the loop collides with)

**Issue:** The loop's only success criteria are a non-zero `scan_trust_surface` exit per fixture,
a total count of 12, and the literal token `rejected` anywhere in the log. But
`scan_trust_surface` also returns non-zero when it *finds nothing* (`:1013-1016`, "zero *.yml/*.yaml
files under $dir"), and the loop's own `empty-dir` fixture proves that path is counted as an
ordinary rejection (the live run shows `fixture 'empty-dir' rejected (rc=1)`). Consequently every
mutation that breaks a *fixture* instead of a rule is invisible:

* delete any of the four new fixture heredocs (`:1713-1775`) — the `mkdir -p` list (`:1574-1576`)
  still creates the empty directory, the scan rejects it with "zero files", the count still
  reaches 12, and the `rejected` token still deposits;
* delete the `mkdir` entry *and* the heredoc — `find` fails silently (`2>/dev/null`, `:1011`)
  and the same path is taken;
* corrupt a fixture's YAML so it fails to parse — rejected, but for a reason no rule under test
  produced.

This is the exact "found nothing, reported green" class the file's own comments say the phase
exists to close (`:990-993`, `:215`, `:221`), now present in the loop that certifies the other
probes. The plan-level drills only pin the rule-deletion direction; the fixture-deletion direction
is unguarded. The two warnings of this round are the same hole seen from both ends: WR-R5-01 lets
the evidence disappear; WR-R5-02 lets the thing the evidence was for disappear.

**Fix:** assert the *reason*, not just the exit code — the reason is already on the first line of
`$WORK/trust-surface.log`. Pair each per-file fixture with a required substring and require the
zero-files reason to be absent for non-vacuity fixtures:

```bash
case "$fixture" in
  quoted-paths|space-paths) want="with a 'paths' key" ;;
  tojson-secrets)           want="toJSON(secrets)" ;;
  quoted-hash-secret|stray-secret) want="parsed value references secrets." ;;
  yaml-ext)                 want="grants 'packages'='write'" ;;
  lowercase-registry)       want="registry/publication token" ;;
  two-files)                want="with a 'paths' key" ;;
  empty-jobs)               want="empty or absent jobs" ;;
  empty-perms)              want="empty permissions" ;;
  no-perms)                 want="declares no permissions" ;;
  empty-dir)                want="zero *.yml/*.yaml files" ;;
esac
head -1 "$WORK/trust-surface.log" | grep -qF "$want" \
  || fail "P11 fixture '${fixture}' was rejected for the wrong reason: $(head -1 "$WORK/trust-surface.log")"
```

and emit one token per fixture class instead of the single all-matching `rejected`.

#### WR-R5-02: The 48-22 install-surface env declarations are pinned by nothing — removing `GOTOOLCHAIN: local` (or `GOSUMDB`/`GOPROXY`) keeps every partition green

**File:** `.github/workflows/ci.yml:242-245` (the declarations), `verify-phase-48.sh:444-499` (the assertion group that should pin them)

**Issue:** 48-22's stated purpose is "declare the install surface instead of inheriting it from
the runner image" — the fix for round-4 IN-01. But no partition asserts the declarations. The
P3 verify-job group (`P3-VERIFYJOB-OK`, `:444-499`) checks the job/step `if:`-freeness, the gate
as terminus, `checkout@v7` + `persist-credentials: false`, the `go install ...@v2.12.2` line, and
the two fetch/script tokens — never the step's `env:` map. A grep of the gate script confirms
`GOTOOLCHAIN` appears only inside a P9 failure message (`:1440`), and `GOSUMDB`/`GOPROXY` appear
nowhere. So a revert to `GOTOOLCHAIN=auto` — the exact behavior IN-01 recorded as the
download-and-execute-another-toolchain exposure — is invisible to the gate, on a file whose
standard is that `persist-credentials` *is* pinned at `:476-481`. The hardening shipped as a
comment, not as a contract.

**Fix:** extend the `P3-VERIFYJOB` group with a parsed assertion on the install step's env map,
next to the persist-credentials check:

```python
inst = next(
    (s for s in vsteps if (s.get("run") or "").find("go install") != -1),
    None,
)
want_env = {
    "GOTOOLCHAIN": "local",
    "GOSUMDB": "sum.golang.org",
    "GOPROXY": "https://proxy.golang.org,direct",
}
if inst is None or (inst.get("env") or {}) != want_env:
    raise SystemExit(
        "P3: the linter install step env is %r, want exactly %r — an inherited "
        "GOTOOLCHAIN/GOSUMDB/GOPROXY surface is the IN-01 exposure (48-22)"
        % (None if inst is None else inst.get("env"), want_env)
    )
```

### Info (round 5)

#### IN-R5-01: `PUBLICATION_TOKENS` got no parsed layer in the 48-22 conversion — `push: 'true'` and a private-registry `tags:` value evade the publication rule everywhere outside P7's two judged steps

**File:** `verify-phase-48.sh:1055-1065` (tokens), `:1112-1119` (textual check), `:936-953` (P7's narrow scope)

**Issue:** 48-22 made path filters and secrets parsed-document-primary but left the third
trust-surface rule (publication) textual-only over comment-stripped text. Two consequences:
`push: 'true'` (a quoted YAML boolean that build-push-action executes as a push) does not contain
the substring `push: true`, and a `tags:` value naming a registry outside the four-token allowlist
(`ghcr.io`, `docker.io`, `quay.io`, `dockerhub`) matches nothing. P7 only judges the `docker` job's
exactly-two build steps, so a publish-capable step in any other job is judged by neither layer.
Practical exploitability is low (a real publish usually also names `ghcr.io`/`docker.io`), hence
Info — but the asymmetry is the residue of the same WR-02 class this round closed for the other
two rules.

**Fix:** in the parsed walk, reject any step `with:` mapping whose `push` value is not `False`
parsed (`push: 'true'` parses to the string `'true'`) and any `with.tags`/`with.push`-adjacent
registry literal, or at minimum extend the textual rule with quoted variants
(`"push: 'true'"`, `"push: \"true\""`).

#### IN-R5-02: Two assertion groups in the reviewed delta have no evidence binding of their own

**File:** `verify-phase-48.sh:1251-1254` (`secrets[` check), `:586-593` (`config verify` group)

**Issue:** (a) The parsed secret walk rejects three shapes — `secrets.NAME`, `secrets[`, and
`toJSON(secrets` — but only the latter two have P11 fixtures and only `secrets.NAME` has a textual
second layer. Deleting the `secrets[` branch (`:1251-1254`) is invisible: no fixture exercises it
and no other partition judges it. (b) The `golangci-lint config verify` group has no token of its
own; P4's deposit requires only the four sentinel tokens. Its coverage today is accidental — if
the config-verify block were deleted, the sentinels would run under the *default* config, and only
sentinel D's `revive` attribution (revive is not a default v2 linter) would red the gate. That is
a real red, but reached by luck of the sentinel-D design, not by the group's own evidence, and a
future sentinel change silently removes the coverage.

**Fix:** (a) add a fixture with `run: echo "${{ secrets['DEPLOY_KEY'] }}"` and pair it with a
reason assertion per WR-R5-01's fix. (b) append a `P4-CONFIG-OK` line to `$WORK/p4-config.log`
after the rc check and require it in `ok_partition "P4"` alongside the four sentinel tokens.

#### IN-R5-03: P1's `pull_request_target` scan still judges raw source (carried from round 4 IN-03, still present)

**File:** `verify-phase-48.sh:265-279`

**Issue:** (Round-4 IN-03, unchanged by the round-5 delta, and recorded `open` in the on-disk
ledger.) `grep -rn 'pull_request_target' "$WF_DIR"` reads raw workflow text, so a workflow comment
such as "we never use pull_request_target" reds P1 — while P8 strips comments for the same class
of concern (`:1099`) and 48-22 made the parsed document primary for exactly this reason. The
exit-code discipline added in round 4 is intact and correct; only the false-positive direction
remains. **Fix:** scan `nocomment` text (or the parsed `on:` keys) for the event name, keeping the
rc==1 requirement.

#### IN-R5-04: The push-to-main trigger comment still claims a mitigation it does not provide (carried from round 4 IN-07, still present)

**File:** `.github/workflows/ci.yml:41-45`

**Issue:** (Round-4 IN-07, unchanged by the round-5 delta, and recorded `open` in the on-disk
ledger.) The comment says push is limited to `main` because "the lint action has no diff on a
manual/push run and would fall back to whole-repo lint over ~175 legacy findings" — but limiting
the trigger to `main` does not prevent that fallback on a `main` push; under the comment's own
premise, the first merge to `main` reds the lint step. In a file whose header is written as an
auditable contract, an unsound justification is a maintenance hazard: the next editor trusts it
and propagates the wrong model. **Fix:** verify one push-to-main run, then correct the comment to
what the action actually does on push events (or add the explicit `--new-from-rev` base the
comment implies).

#### IN-R5-05: Dead code left by the round-5 edits

**File:** `verify-phase-48.sh:296` (unused import), `:1225` and `:1267` (duplicate assignment)

**Issue:** `import re, sys, yaml` in the P2/P3 probe never uses `re` (grep over the probe body:
zero `re.` occurrences) — round-4's probe needed it for the count regex, and the split left it
behind. In `scan_trust_surface`, `basename = os.path.basename(p)` is computed twice per file
(`:1225` for the parsed walk, `:1267` for the textual scan); the second computation is redundant.
Both are harmless but they are exactly the kind of drift the column-0 drill markers are meant to
make attributable. **Fix:** drop `re` from the import and the second `basename` assignment.

---

_Reviewed: 2026-10-21T09:00:10Z (round 7) · 2026-10-21T02:15:44Z (round 6) · 2026-10-20T15:34:24Z (round 5)_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
