---
phase: 48-ci
reviewed: 2026-10-20T15:34:24Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - verify-phase-48.sh
  - .github/workflows/ci.yml
findings:
  critical: 0
  warning: 2
  info: 5
  total: 7
status: issues_found
---

# Phase 48: Code Review Report (gap-closure round 5)

**Reviewed:** 2026-10-20T15:34:24Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

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

## Warnings

### WR-R5-01: The P11 fixture loop cannot attribute rejections — a deleted or emptied fixture counts as a pin, so the 12 evasion fixtures can be silently removed while P11 stays green

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

### WR-R5-02: The 48-22 install-surface env declarations are pinned by nothing — removing `GOTOOLCHAIN: local` (or `GOSUMDB`/`GOPROXY`) keeps every partition green

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

## Info

### IN-R5-01: `PUBLICATION_TOKENS` got no parsed layer in the 48-22 conversion — `push: 'true'` and a private-registry `tags:` value evade the publication rule everywhere outside P7's two judged steps

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

### IN-R5-02: Two assertion groups in the reviewed delta have no evidence binding of their own

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

### IN-R5-03: P1's `pull_request_target` scan still judges raw source (carried from round 4 IN-03, still present)

**File:** `verify-phase-48.sh:265-279`

**Issue:** (Round-4 IN-03, unchanged by the round-5 delta, and recorded `open` in the on-disk
ledger.) `grep -rn 'pull_request_target' "$WF_DIR"` reads raw workflow text, so a workflow comment
such as "we never use pull_request_target" reds P1 — while P8 strips comments for the same class
of concern (`:1099`) and 48-22 made the parsed document primary for exactly this reason. The
exit-code discipline added in round 4 is intact and correct; only the false-positive direction
remains. **Fix:** scan `nocomment` text (or the parsed `on:` keys) for the event name, keeping the
rc==1 requirement.

### IN-R5-04: The push-to-main trigger comment still claims a mitigation it does not provide (carried from round 4 IN-07, still present)

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

### IN-R5-05: Dead code left by the round-5 edits

**File:** `verify-phase-48.sh:296` (unused import), `:1225` and `:1267` (duplicate assignment)

**Issue:** `import re, sys, yaml` in the P2/P3 probe never uses `re` (grep over the probe body:
zero `re.` occurrences) — round-4's probe needed it for the count regex, and the split left it
behind. In `scan_trust_surface`, `basename = os.path.basename(p)` is computed twice per file
(`:1225` for the parsed walk, `:1267` for the textual scan); the second computation is redundant.
Both are harmless but they are exactly the kind of drift the column-0 drill markers are meant to
make attributable. **Fix:** drop `re` from the import and the second `basename` assignment.

---

_Reviewed: 2026-10-20T15:34:24Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
