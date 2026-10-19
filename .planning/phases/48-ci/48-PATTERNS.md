# Phase 48: CI 矩阵 - Pattern Map (gap-closure run)

**Mapped:** 2026-10-20
**Mode:** `--gaps` gap-closure after `gaps_found` re-verification (`48-VERIFICATION.md`, 5/9)
**Files analyzed:** 6 (all already exist and are git-TRACKED from the pre-verification execution run — this run *modifies*, it does not create)
**Analogs found:** 4 / 6 (the 2 without analogs are singleton file types unlikely to be touched)

> **Tracked-source gate:** every path named below was verified via `git ls-files` — none is a
> gitignored install/runtime mirror. All analogs are tracked origins.

---

## Scope Derivation (from 48-VERIFICATION.md "Minimum path to close")

The gap-closure plans modify exactly these files. Each closure item is mapped to its analog.

| # | Closure item (VERIFICATION) | File(s) touched | Analog |
|---|-----------------------------|-----------------|--------|
| 1 | Partition success tokens (`$WORK/ran/<P>.ok`) required by `done_partition`; drop the `:-0` default | `verify-phase-48.sh` | `verify-phase-48.sh` (own skeleton) + `verify-phase-47.sh` (P6_ran precedent) |
| 2 | Pin the `verify` job in P2/P3 (exists, no job- or step-level `if:`, last step is the gate) | `verify-phase-48.sh` | `verify-phase-48.sh` P2/P3 Python probe (own pattern) |
| 3 | Replace `curl \| sh` bootstrap with `go install ...@v2.12.2` or SHA-pinned script + checksum + `[ -x ]` postcondition + `set -o pipefail` | `.github/workflows/ci.yml` | `scripts/verify-phase-16.sh` (checksum-equality) + `eval-gate.yml` (pipefail pattern) |
| 4 | `persist-credentials: false` on the verify job's checkout; pin PyYAML; `-run` vacuity probe; fix stale header comment | `.github/workflows/ci.yml` | `ci.yml` go job lines 73-75 (own persist-credentials pattern) |
| 5 | Delete the `env:` block from eval-gate's `Gate check` step; keep the three secrets on the replay step only | `.github/workflows/eval-gate.yml` | `eval-gate.yml` replay step lines 93-111 (own step-scoped-env pattern) |
| 6 | Re-record the supply-chain threat row; make frontmatter match the register | `.planning/phases/48-ci/48-SECURITY.md` | `48-SECURITY.md` own row format + `.planning/phases/18-skill-p10-4/18-SECURITY.md` |

---

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `verify-phase-48.sh` | utility (local CI-invariant verifier, bash) | batch (fail-fast partitions) | `verify-phase-47.sh` + its own current code | exact (self + lineage) |
| `.github/workflows/ci.yml` | config (GitHub Actions workflow, 4 jobs) | event-driven (PR/push) | `.github/workflows/eval-gate.yml` | role-match |
| `.github/workflows/eval-gate.yml` | config (GitHub Actions workflow, gated live job) | event-driven | itself (only sibling workflow) | exact (self) |
| `.planning/phases/48-ci/48-SECURITY.md` | config (per-phase threat register, markdown+frontmatter) | doc/record | `.planning/phases/18-skill-p10-4/18-SECURITY.md` | role-match |
| `.golangci.yml` | config (linter v2 schema) | transform | none — sole linter config in repo | no analog |
| `.github/actions/setup/action.yml` | config (composite action) | request-response | none — sole composite action in repo | no analog |

---

## Pattern Assignments

### `verify-phase-48.sh` (utility, batch fail-fast partitions) — closure items 1 + 2

**Primary analog:** itself (the file under edit). **Lineage analog:** `verify-phase-47.sh`.

**A1. Partition accounting skeleton — the code being amended** (lines 101-117):

```bash
mkdir -p "$WORK/ran"
begin_partition() { : > "$WORK/ran/$1.begin"; }

EXPECTED_PARTITIONS=11
RAN=0
RAN_LIST=()
done_partition() {
  local pn="${1%% *}"
  if [ ! -f "$WORK/ran/$pn.begin" ]; then
    fail "P11 partition ${pn} reached done_partition without a begin marker — its body did not run (IN-06)"
  fi
  RAN=$((RAN + 1))
  RAN_LIST+=("$1")
  pass "$1"
}
```

The defect (VERIFICATION R2-WR-03): a begin marker proves a body *started*, not that a probe
*ran*. The fix extends the same mechanism: each probe's success deposits `$WORK/ran/<P>.ok`
and `done_partition` requires **begin + ok**. Copy the exact style of the existing marker
check (fail message names the invariant and its ID). `done_partition` call sites: lines 186,
311-312, 510, 561, 635, 704, 939, 1080, 1114, 1317 (11 total — matches
`EXPECTED_PARTITIONS=11` at line 106).

**A2. Drop the `:-0` default that absorbs an unset probe** (line 939):

```bash
done_partition "P8 trust-surface (${TRUST_SURFACE_COUNT:-0} workflow(s) per file (*.yml + *.yaml): read-only permissions, no registry/path/secret surface)"
```

`${TRUST_SURFACE_COUNT:-0}` prints a plausible zero when `scan_trust_surface` (defined
ending line 926, invoked line 934) never set the variable. Under `set -u` (line 61:
`set -uo pipefail`) the un-guarded form makes a deleted probe fail instead. Pattern rule:
**no `:-` default on a variable whose setter is a probe under test.**

**A3. The P11 registry invariant being extended** (lines 1140-1145):

```bash
CALL_SITES=$(grep -cE '^done_partition "' "$SELF")
BEGIN_COUNT=$(find "$WORK/ran" -maxdepth 1 -name '*.begin' | wc -l | tr -d ' ')
if [ "$CALL_SITES" -ne "$EXPECTED_PARTITIONS" ] || [ "$BEGIN_COUNT" -ne "$EXPECTED_PARTITIONS" ]; then
  fail "P11 the registry, the call sites and the begin markers disagree: ..."
fi
```

Adding `.ok` tokens means this half must grow a third count (`find "$WORK/ran" -name '*.ok'`)
and the fail message must name all three. Note the runtime-assembled-token convention at
lines 1150-1152 — a forbidden/self-referential literal is never spelled in source; the `.ok`
suffix, if it is ever grepped for, should follow the same convention.

**A4. Extending P2/P3 with the verify-job contract — the probe shape to copy** (lines 205-224,
the exact `jobs["go"]` load and step-level `if` rejection the fix replicates for `jobs["verify"]`):

```python
jobs = doc.get("jobs") or {}
if "go" not in jobs:
    raise SystemExit("P2: ci.yml has no `go` job")
j = jobs["go"]
steps = j.get("steps") or []
runs = "\n".join(s.get("run", "") for s in steps)

for s in steps:
    if "if" in s:
        label = s.get("name") or s.get("uses") or s.get("run") or "<unnamed step>"
        raise SystemExit(
            "P3: the go job's step %r carries a step-level if: %r - the gate is skippable (WR-01/T-48-07)"
            % (label, s.get("if"))
        )
```

The verify-job contract needs three more assertions in the same probe, all expressible in
this exact style: `jobs["verify"]` exists; no `"if"` key on the job (`j.get("if")`) nor on any
step; the last step's `run` equals/contains `bash verify-phase-48.sh`. Copy the
`raise SystemExit("P<n>: ...")` message shape — partition letter + violation + invariant ID.
PyYAML gotcha already handled at lines 157-165 (`d.get("on", d.get(True))`).

**A5. Lineage pattern — a tool-gated non-fatal branch done honestly** (`verify-phase-47.sh`
lines 203-237, 275-281): `P6_ran` is a single integer set to 1 only after both probes ran and
exited 0; the closing banner branches on it and prints `NOT VERIFIED` + the skip reason
otherwise. This is the semantic the `.ok` tokens generalize: **the banner's truth source is
"did the probe run", never "did the partition's body start".** Note verify-phase-48.sh's own
header (lines 47-51) explicitly *tightens* this: here every partition is statically decidable,
so a missing partition is exit 1, not a softer banner — keep that stance; `.ok` is required,
not advisory.

**A6. Empty-log rule (WR-06) — reference shape** (`verify-phase-48.sh` lines 356-373
`lint_run`; same in `verify-phase-47.sh` lines 103-115 `run_go_test`):

```bash
( cd "$(native "$dir")" && golangci-lint "${args[@]}" ) >"$log" 2>&1
local rc=$?
if [ ! -s "$log" ]; then
  fail "P4 the linter produced no output (rc=$rc) — infrastructure failure, no evidence of a lint run"
fi
return "$rc"
```

Any new probe added in this closure (e.g. the `-run` vacuity check, or an install postcondition)
must write its own log and fail on an empty log — same as the P4 preflight at lines 330-343,
which prints `LINT NOT VERIFIED` and `exit 1` when tooling is absent (never folds to PASS).

**A7. Zero-match enumeration rule** (lines 146-148): enumerate to a list file, then
`[ "$WF_COUNT" -ge 1 ] || fail "... a partition that finds zero files must not pass"`. Any
new enumeration (`.ok` tokens, verify-job scan) inherits this.

---

### `.github/workflows/ci.yml` (config, event-driven, 4 jobs) — closure items 3 + 4

**Primary analog:** `.github/workflows/eval-gate.yml` (the only other workflow; already
carries the WR-03 pipefail fix this file's install step lacks).

**B1. The offending install step being replaced** (`ci.yml` lines 199-204):

```yaml
      - name: install pinned golangci-lint
        run: |
          LINT_BIN="$(go env GOPATH)/bin"
          mkdir -p "$LINT_BIN"
          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$LINT_BIN" v2.12.2
          echo "$LINT_BIN" >> "$GITHUB_PATH"
```

Defects per VERIFICATION R2-CR-01 / R2-WR-02: mutable `HEAD` ref, no checksum, curl's exit
dropped by the pipe, no postcondition, runner token persisted. The replacement shape
(`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`, or SHA-pinned
install.sh + release `checksums.txt`) must keep: the version literal matching
`LINT_VERSION="2.12.2"` in verify-phase-48.sh line 121, and add a `[ -x "$LINT_BIN/golangci-lint" ]`
postcondition plus `set -o pipefail` if any pipe remains.

**B2. Checksum-equality pattern to copy** (`scripts/verify-phase-16.sh` lines 359-371 — the
repo's existing byte-identity guard):

```bash
check() { local f=$1 want=$2 label=$3; local got; got=$(sha256sum "eval/frozen/${f}" | cut -d' ' -f1); if [ "${got}" != "${want}" ]; then echo "FATAL: ${f} sha256 mismatch (${label})"; echo "  expected: ${want}"; echo "  actual:   ${got}"; exit 1; fi }
check compliance.lock.json ${EXPECTED_COMPLIANCE_SHA} 'Phase 14 baseline UNCHANGED'
```

Adaptation for ci.yml: fetch `checksums.txt` for the pinned release, compute the binary's
sha256, compare with expected/actual printed on mismatch, exit 1. Same "expected / actual"
evidence shape.

**B3. pipefail pattern to copy** (`eval-gate.yml` lines 98-105 — the WR-03 fix, already
mutation-proven):

```yaml
        # WR-03: without `shell: bash` GitHub runs `bash -e {0}` (no pipefail),
        # so the pipeline reports tee's status and a non-zero ./bin/eval exit
        # is discarded. `set -o pipefail` stays in the body as belt-and-braces
        # so the intent survives a later removal of the `shell:` key.
        shell: bash
        working-directory: ${{ github.workspace }}
        run: |
          set -o pipefail
          ...
```

Any ci.yml step that pipes (install, checksum verify) copies both halves: `shell: bash` AND
`set -o pipefail` in the body.

**B4. `persist-credentials: false` — copy into the verify job's checkout** (`ci.yml` lines
68-75, the go job's existing IN-10 hardening; the verify job's checkout at lines 192 lacks it):

```yaml
      - uses: actions/checkout@v7
        with:
          persist-credentials: false
```

**B5. Pin PyYAML** (`ci.yml` lines 206-207, R2-IN-03):

```yaml
      - name: python deps for the gate (PyYAML)
        run: python -m pip install --quiet --disable-pip-version-check pyyaml
```

Pattern: pin to the locally validated version (48-RESEARCH.md §Supporting: PyYAML 6.0.3)
using pip's `pyyaml==6.0.3` form; keep the same step-name style.

**B6. Step-name + inline-invariant-comment style** — every step in this file names its
invariant ID in a comment above it (lines 68-72 IN-10, 95-100 D-19, 169-184 IN-07/D-21/D-29).
New/edited steps keep this. Also fix the stale header at lines 29-31 ("Three independent
jobs" vs four) — the header is the file's own invariant register; P10-style prose/code
consistency applies.

**B7. Structural constraints the edit must not break** (enforced by `verify-phase-48.sh`, so
they are regression surfaces): no step-level `if:` on go-job steps (P2/P3, lines 218-224;
P6/P7 carry the same rule for fe/docker); no job-level `if:` on go (line 269-270); go-job
permissions map exactly `{contents: read, pull-requests: read}` (lines 286-290); checkout
immediately before `./.github/actions/setup` (lines 294-295); zero
`continue-on-error`/`paths:`/`pull_request_target`/`secrets.*`/`go-version:` literals
(P1/P8/P9). The verify job currently has NO job-level permissions block (lines 182-183) —
keep it that way unless the closure explicitly re-decides it.

---

### `.github/workflows/eval-gate.yml` (config, event-driven, gated live job) — closure item 5

**Analog:** its own replay step — the pattern the `Gate check` step must be brought into
line with.

**C1. The defect** (lines 113-117 — the env block to DELETE):

```yaml
      - name: Gate check
        env:
          GLM_API_KEY: ${{ secrets.GLM_API_KEY }}
          BGE_M3_URL: ${{ secrets.BGE_M3_URL }}
          MILVUS_ADDRESS: ${{ secrets.MILVUS_ADDRESS }}
        working-directory: ${{ github.workspace }}
        run: |
          # Exit code != 0 fails the workflow. Threshold comes from
          # examples/skills/compliance/eval-gate.json.
          ./bin/eval \
            --gate-check eval/reports/ci/compliance.json \
            --skills-dir examples/skills
```

Evidence it cannot consume them: `cmd/eval` short-circuits `--gate-check` before any
bootstrap (`cmd/eval/main.go:462-464`).

**C2. The correct pattern, already in the same file** (lines 93-97): the three secrets are
scoped to the *consuming* step only:

```yaml
      - name: Run frozen compliance replay
        env:
          GLM_API_KEY: ${{ secrets.GLM_API_KEY }}
          BGE_M3_URL: ${{ secrets.BGE_M3_URL }}
          MILVUS_ADDRESS: ${{ secrets.MILVUS_ADDRESS }}
```

The WR-08 comment at lines 63-68 records the invariant ("the three live secrets are
deliberately NOT job-level … scoped to the two consuming steps below"). After the edit the
word "two" must become "one" (or the step name), keeping prose == code — this file is parsed
by P9 (exact-equality on the `if:` at lines 974-986 of verify-phase-48.sh) and its
secret-mapping references are counted by P9's "secrets intact" check, so deleting the env
block must keep all six `${{ secrets.X }}` substitutions present in the file (the replay
step retains three; the count of *references* stays 3, and P9's assertion surface must be
re-checked against the edit).

**C3. Also in this file (already correct, do not regress):** gate `if:` lines 56-59;
complement notice lines 149-152 + `::notice::` at 158; `go-version-file: go.mod` at 84-87;
`shell: bash` + pipefail at 102-105; `upload-artifact@v7` at 128.

---

### `.planning/phases/48-ci/48-SECURITY.md` (config, threat register) — closure item 6

**Primary analog:** itself (row format). **Secondary analog:**
`.planning/phases/18-skill-p10-4/18-SECURITY.md` (canonical compact register: frontmatter
`threats_open` must equal the count of open rows ≥ `block_on`; Audit Trail row appended per
pass; Sign-Off checkboxes re-confirmed).

**D1. Row format to copy** (`48-SECURITY.md` line 44 header, rows 46-65):

```
| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
```

The new supply-chain row (T-48-21, next free ID after T-48-20) follows the T-48-10 shape —
Category `Tampering`, Component `ci.yml verify job`, Disposition `mitigate`, Mitigation cell
naming the enforcing assertion AND the mutation that proves it (see rows T-48-02/T-48-10 for
the "assertion + mutation" cell convention), Status `closed` only after the fix lands.

**D2. Frontmatter-vs-prose consistency — the exact failure being repaired** (lines 5-18):
the file already documents one `threats_open` correction ("A machine-readable verdict must
never contradict its own report"). The repair appends an audit-trail row (format at lines
177-181) rather than silently overwriting, and updates the arithmetic paragraph at lines
131-133 ("20 rows; 0 open …") plus the Sign-Off (lines 187-192). `48-11-SUMMARY.md` carries
`threat_flag: supply-chain` routed to this file — the row must cite it.

**D3. "Why these closures changed" section** (lines 83-133) is the established format for
recording a closure that a later pass falsified and re-closed with evidence. The round-2
entry (R2-CR-01) extends this section the same way the "Re-closure (plan 48-10, 2026-10-19)"
block (lines 111-129) does: table of `Falsified closure | Rows | Now proven by | Mutation that bites`.

---

## Shared Patterns

### No silent green / no vacuous pass (the phase's own header invariant)
**Source:** `verify-phase-48.sh` lines 5-55 (header), 330-343 (preflight `LINT NOT VERIFIED` + exit 1),
356-373 (empty-log guard), 146-148 (zero-match fails)
**Apply to:** every new or edited probe in `verify-phase-48.sh` and every new step in `ci.yml`
```bash
if [ ! -s "$log" ]; then
  fail "… produced no output (rc=$rc) — infrastructure failure, no evidence of a … run"
fi
```
A probe that finds nothing, a tool that is missing, and a job that never ran are all
**non-zero exits or explicit NOT VERIFIED banners** — never a PASS line with a plausible zero.

### Exit-code authority through pipelines
**Source:** `.github/workflows/eval-gate.yml` lines 98-105 (`shell: bash` + `set -o pipefail`)
**Apply to:** `ci.yml` install/checksum steps; any new pipe in `verify-phase-48.sh` (file already
declares `set -uo pipefail` at line 61 — new probes must not bypass it with inline `|| true`)

### Supply-chain pinning
**Source:** `scripts/verify-phase-16.sh` lines 359-371 (expected/actual sha256 with FATAL on mismatch);
`.golangci.yml` lines 1-8 (the "no long-lived exemption" stance this extends)
**Apply to:** the golangci-lint bootstrap in `ci.yml`; the PyYAML pin
```bash
got=$(sha256sum "$f" | cut -d' ' -f1); [ "$got" != "$want" ] && { echo "FATAL: mismatch"; echo "  expected: $want"; echo "  actual:   $got"; exit 1; }
```

### Secret surface minimization (step-level env only)
**Source:** `.github/workflows/eval-gate.yml` lines 63-68 (WR-08 comment) + 93-97 (replay step)
**Apply to:** `eval-gate.yml` Gate check (delete), and as the standing rule for any future
`ci.yml` step: a secret appears only in the step that consumes it; `ci.yml` carries zero
`secrets.*` beyond the implicit runner token (asserted by P8, lines 911-921).

### Register == code (prose never outruns the artifact)
**Source:** `48-SECURITY.md` lines 5-18 (documented self-correction), 137-172 ("What this audit
does NOT establish")
**Apply to:** `48-SECURITY.md` supply-chain row; `ci.yml` header (lines 29-31 stale count);
`verify-phase-48.sh` P10's own SC-4 wording check (lines 1086-1114) is the mechanism this
pattern generalizes.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.golangci.yml` | config (linter v2) | transform | Sole linter config in the repo; already verified against v2.12.2 schema (`config verify` exit 0) and mutation-proven by P4 sentinel D. Unlikely to change in this closure; if it does, the only in-repo reference shapes are its own header comments and P4's sentinels (lines 374-510). |
| `.github/actions/setup/action.yml` | config (composite action) | request-response | Sole composite action in the repo; contract pinned by P5 (lines 516-561: three pinned majors, zero checkout, every caller checks out first). Not named by any round-2 finding. |

---

## Metadata

**Analog search scope:** `.github/workflows/`, `.github/actions/`, repo-root `verify-phase-*.sh`,
`scripts/verify-phase-*.sh`, `.planning/phases/*/*-SECURITY.md`, `.golangci.yml`
**Files scanned:** 6 targets + 6 analogs read in full or in targeted sections
**Line references verified against:** current worktree at branch `phase-20-p10-6-admin-console` (post gap-closure wave, 15 commits `e112488`..`2387b61` per VERIFICATION)
**Pattern extraction date:** 2026-10-20
