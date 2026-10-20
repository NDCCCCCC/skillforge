#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# verify-phase-48.sh — Phase 48 CI gate self-proof (SC-5).
#
# WHAT THIS SCRIPT IS: proof that the GitHub Actions gate will really run and
# really bite. Everything here is statically decidable on a host that has
# Python and the pinned linter — no WSL, no Node, no Docker, no live LLM /
# Milvus / BGE-M3 / GLM. That is deliberate: a partition whose outcome depends
# on a machine's optional tooling is a partition that can be skipped, and a
# skipped partition that still reports green is the Phase 47 HIGH-1 class.
#
# Partitions (fail-fast, in order):
#   P1  workflow-parse  — every .github/workflows/*.yml and *.yaml plus the
#                         composite action parses under PyYAML; the trigger
#                         block is reachable; no workflow targets
#                         pull_request_target
#   P2  go-line         — the go job carries the exact command strings the
#                         local gate runs (the determinism step keeps its
#                         -list existence guard against a materialised list), is hermetic
#                         (CGO_ENABLED only, no live-service env), captures
#                         its shuffle seed, and the verify job it shares the
#                         file with ends at the gate script
#   P3  lint-line       — lint is pinned, blocking, and holds the permission
#                         its changed-lines mode needs to not silently widen;
#                         the verify job is if-free with a module-proxy
#                         install and a credential-free checkout
#   P4  lint-teeth      — the config verifies AND a sentinel module with an
#                         unchecked json.Unmarshal is really reported, while
#                         the two noise classes the config is meant to absorb
#                         stay quiet
#   P5  composite       — the toolchain action carries the three pinned majors
#                         and no checkout; every ci.yml job checks out first
#   P6  fe-line         — the fe job runs the three exact frontend commands
#                         under web/, with no hard-coded test count
#   P7  docker-line     — both Dockerfiles build with push: false and
#                         per-image GHA cache scopes; no publish path
#   P8  trust-surface   — the whole workflow directory: read-only permissions,
#                         zero registry surface, zero path filters, zero
#                         secrets beyond the runner token
#   P9  eval-gate-gate  — the eval-gate workflow's fork/Dependabot boundary,
#                         its visible skip notice, its secret wiring intact
#                         exactly once with a secret-free Gate check, its
#                         un-widened trigger, and the directory-wide
#                         single Go version source
#   P10 sc4-wording     — ROADMAP SC-4's corrected sentence: the old
#                         hermetic-precedent wording is gone and the
#                         live-workflow replacement is present
#   P11 self-probe      — the registry matches the done_partition call sites,
#                         the begin markers AND the success tokens, the
#                         passing banner has exactly one RAN-gated print
#                         site, and an empty partition is proven to FAIL
#
# NO WAIVERS. Unlike Phase 47 — whose frontend partition P6 may exit 0 as
# PARTIAL because a WSL-only host has no Node — every partition here is
# statically decidable, so a missing partition is a NON-ZERO exit, not a
# softer banner. The closing banner is gated on the RAN counter against
# EXPECTED_PARTITIONS and never on a hard-coded string.
#
# The empty-log rule (WR-06, inherited from Phase 47) is not negotiable: a
# grep-only judgment lets a crashed toolchain pass as "zero findings found".
# Every linter invocation below writes its own log and an empty log fails.
#
# Environment: run `bash verify-phase-48.sh` from the repo root (Git Bash on
# Windows). golangci-lint is NOT on the default PATH on the maintainer's host;
# export its bin directory first, or the P4 preflight will refuse.

set -uo pipefail

export MSYS_NO_PATHCONV=1

# ROOT — the repository this script lives in, resolved from the script's own
# location, so no partition depends on the caller's cwd or on a `cd` performed
# by an earlier partition.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ROOT_NATIVE — the same path in the form a native Windows binary understands.
# Under Git Bash, $ROOT is the MSYS spelling (/d/code/...), which Python (a
# native Win32 build here) cannot open; every Python probe below is handed the
# converted path instead. On a POSIX host cygpath is absent and native() is the
# identity, so the same source runs unchanged on a Linux runner.
native() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
  else
    printf '%s' "$1"
  fi
}

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}PASS${NC} $1"; }
fail() { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Every log path below is absolute under $WORK, so a partition cannot deposit
# evidence next to the sources it is judging — a log file that appears in the
# repo root is both an untracked artefact and a sign that a relative path
# leaked past the temp dir.

# Body-emitted partition markers (IN-06) and probe-emitted success tokens
# (R2-WR-03). A `done_partition` call site is not evidence that its partition
# body ran: commenting out a body while leaving the call site used to satisfy
# both the call-site count and the RAN counter. Each body now creates its own
# begin marker as its first statement, and done_partition refuses without it.
#
# A begin marker alone is still not enough: it proves a body STARTED, not that
# the probe the body certifies RAN. M8 (48-VERIFICATION) commented out P8's
# scan_trust_surface call, kept both markers' bookkeeping, and printed
# `PASS P8 trust-surface (0 workflow(s) ...)` under the green banner.
#
# So success tokens are EVIDENCE-BOUND (T-48-61 / R2-WR-03), and since the M13
# round they are bound PER ASSERTION GROUP, not per probe. A probe carrying
# many assertions behind ONE token let a partition's whole assertion block be
# deleted while the shared token — printed at the end of a sibling group —
# still deposited PASS for both partitions (the M13 shape: one shared token
# for P2 and P3).
# ok_partition therefore takes the partition, the log path that partition's
# OWN probe wrote, and ONE OR MORE literal tokens, one per assertion group; it
# refuses unless the log exists, is non-empty, and contains EVERY token. Only
# then does it write the .ok marker, which still means "begin AND every
# probe-emitted token". done_partition refuses without begin AND ok, and the
# deposit never precedes the assertion it certifies.
mkdir -p "$WORK/ran"
begin_partition() { : > "$WORK/ran/$1.begin"; }
ok_partition() {
  if [ "$#" -lt 3 ]; then
    fail "partition called ok_partition without its probe log and at least one probe-emitted token — a success token can only be deposited from probe output (R2-WR-03)"
  fi
  local pn="$1"
  local log="$2"
  shift 2
  if [ ! -f "$log" ]; then
    fail "partition ${pn} cannot deposit its success tokens: the probe log ${log} does not exist — the probe that certifies ${pn} did not run (R2-WR-03)"
  fi
  if [ ! -s "$log" ]; then
    fail "partition ${pn} cannot deposit its success tokens: the probe log ${log} is empty — infrastructure failure, no evidence the probe ran (R2-WR-03)"
  fi
  local token missing=''
  for token in "$@"; do
    if ! grep -qF -- "$token" "$log"; then
      missing="${missing:+${missing} }${token}"
    fi
  done
  if [ -n "$missing" ]; then
    fail "partition ${pn} cannot deposit its success tokens: the probe log ${log} does not contain the probe-emitted token(s) '${missing}' (required: ${*}) — a deleted or neutralised assertion group cannot deposit PASS (R2-WR-03)"
  fi
  : > "$WORK/ran/$pn.ok"
}

# Partition registry. The closing banner compares RAN against
# EXPECTED_PARTITIONS; a shortfall exits non-zero.
EXPECTED_PARTITIONS=11
RAN=0
RAN_LIST=()
done_partition() {
  local pn="${1%% *}"
  if [ ! -f "$WORK/ran/$pn.begin" ]; then
    fail "P11 partition ${pn} reached done_partition without a begin marker — its body did not run (IN-06)"
  fi
  if [ ! -f "$WORK/ran/$pn.ok" ]; then
    fail "P11 partition ${pn} reached done_partition without a success token — its probe did not run to success (R2-WR-03)"
  fi
  RAN=$((RAN + 1))
  RAN_LIST+=("$1")
  pass "$1"
}

# The linter version this gate is written against. A different binary is an
# unverified gate, not a passing one.
#
# COORDINATED BUMP (IN-01 / 48-VERIFICATION advisory 4): the pinned version is
# deliberately duplicated across ci.yml, because a workflow `with:` value
# cannot read this script. Bump together with LINT_VERSION in verify-phase-48.sh whenever the pinned golangci-lint version changes:
#   1. the go job's `golangci-lint-action` `version:` literal in
#      .github/workflows/ci.yml, and
#   2. the verify job's `go install .../cmd/golangci-lint@v...` line in the
#      same file.
# Drift is caught loudly at both ends: this gate's P4 preflight refuses any
# binary whose version is not LINT_VERSION, and ci.yml's P3 assertion pins the
# action's `version:` and the install line byte-for-byte — a half-bumped pair
# reds the gate instead of silently passing.
LINT_VERSION="2.12.2"

# Two tokens this gate asserts the ABSENCE of, in ci.yml. They are assembled at
# runtime rather than written out: a gate script that spells a forbidden CI
# keyword in its own source (even in a comment or a Python probe) makes the
# repo-wide grep over this file meaningless — the grep can no longer tell the
# workflow's text from the checker's text. Read -r splits on whitespace, so the
# second field must be empty at the end.
SOFT_KEY="$(printf '%s%s' "continue" "-on-error")"
WHOLE_FILES_FLAG="--whole-files"

# Two more banned ci.yml tokens (R2-WR-01 / R2-CR-01), assembled under the same
# rule as SOFT_KEY: the fetch-and-pipe bootstrap prefix and the install-script
# name. The verify job must install its linter through the Go module proxy, so
# neither joined literal may appear anywhere in the workflow's source — and
# neither may appear joined here, or the same repo-wide-grep argument above
# would apply to this file too.
FETCH_TOKEN="$(printf '%s %s' "curl" "-sSfL")"
SCRIPT_TOKEN="$(printf '%s%s' "install" ".sh")"

echo "=== Phase 48 Verification (static CI gate self-proof) ==="

# ---------------------------------------------------------------------------
# P1: workflow-parse
# ---------------------------------------------------------------------------
info "P1: workflow parse + trigger safety"
begin_partition "P1"
WF_DIR="$ROOT/.github/workflows"
COMPOSITE="$ROOT/.github/actions/setup/action.yml"

[ -d "$WF_DIR" ] || fail "P1 no workflow directory at $WF_DIR — a partition that finds zero files must not pass"

# Enumerate to a list file first: a glob evaluated inline is indistinguishable
# from an empty result, and an empty result must fail rather than pass silently.
find "$WF_DIR" -maxdepth 1 -type f \( -name '*.yml' -o -name '*.yaml' \) -print | sort > "$WORK/p1-list.txt"
WF_COUNT=$(wc -l < "$WORK/p1-list.txt" | tr -d ' ')
[ "$WF_COUNT" -ge 1 ] || fail "P1 matched zero *.yml / *.yaml files under $WF_DIR — a partition that finds zero files must not pass"

# PyYAML implements YAML 1.1, where the bare key `on:` loads as the boolean
# True. A parser that reaches only for d["on"] sees a workflow with no trigger
# block at all — which is how a "parsed fine" check can be true of a file whose
# trigger nobody has looked at. The trigger block is therefore read back
# explicitly and required to be present on every workflow.
: > "$WORK/p1.log"
while IFS= read -r f; do
  [ -f "$f" ] || fail "P1 expected workflow file is missing: $f"
  python -c '
import sys, yaml
d = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
if not isinstance(d, dict):
    raise SystemExit("top level is not a mapping")
on = d.get("on", d.get(True))
if on is None:
    raise SystemExit("no trigger block (PyYAML 1.1 reads the bare key `on:` as True)")
print("P1-PARSED")
' "$(native "$f")" >> "$WORK/p1.log" || fail "P1 YAML parse / trigger read failed: $f"
done < "$WORK/p1-list.txt"

# The success token is emitted by the per-file probe itself, and the evidence
# must cover EVERY enumerated file. A probe that is deleted or neutered leaves
# this log short, and the deposit below refuses (R2-WR-03).
P1_PARSED=$(grep -cF 'P1-PARSED' "$WORK/p1.log" || true)
[ "$P1_PARSED" -eq "$WF_COUNT" ] || fail "P1 parsed ${P1_PARSED} workflow(s) but enumerated ${WF_COUNT} — the per-file parse probe did not run to success for every file (R2-WR-03)"

# The composite action is parsed too, but it legitimately has no trigger block
# — it is only ever reached through a caller workflow.
# P1-B: composite parse
[ -f "$COMPOSITE" ] || fail "P1 composite toolchain action missing: $COMPOSITE"
python -c '
import sys, yaml
d = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
if (d.get("runs") or {}).get("using") != "composite":
    raise SystemExit("composite action is not `runs.using: composite`")
print("P1-COMPOSITE-OK")
' "$(native "$COMPOSITE")" >> "$WORK/p1.log" || fail "P1 composite action parse failed: $COMPOSITE"

# P1-C: pull_request_target scan
# D-11: pull_request_target executes fork-controlled workflow code against the
# base repository's secrets and a mutating token. Report file:line, never just
# the file name.
grep -rn 'pull_request_target' "$WF_DIR" > "$WORK/p1-prt.txt" 2>/dev/null
p1_prt_rc=$?
if [ "$p1_prt_rc" -eq 0 ]; then
  sed 's/^/    /' "$WORK/p1-prt.txt"
  fail "P1 pull_request_target found (D-11) — listed above with file:line"
fi
# grep's no-match status is 1. Requiring it explicitly means a `:` substitution
# (status 0) fails: the scan must have produced the decisive exit code, not
# merely not-have-failed (R2-WR-03).
if [ "$p1_prt_rc" -ne 1 ]; then
  fail "P1 the pull_request_target scan returned rc=${p1_prt_rc}, want 1 (no match) — a neutralised or deleted probe cannot deposit PASS (R2-WR-03)"
fi
# The scan's decisive exit-code assertion ran to success, so its group emits
# its own token (WR-03: one token per assertion group, not one per probe).
printf 'P1-NO-PRT\n' >> "$WORK/p1.log"

ok_partition "P1" "$WORK/p1.log" "P1-PARSED" "P1-COMPOSITE-OK" "P1-NO-PRT"

done_partition "P1 workflow-parse ($WF_COUNT workflow(s) *.yml / *.yaml + composite parse, zero pull_request_target; per-assertion success tokens P1-PARSED P1-COMPOSITE-OK P1-NO-PRT)"

# ---------------------------------------------------------------------------
# P2 + P3: the go job's structural contract, decided by one Python probe so the
#          two partitions cannot disagree about what the file says.
# ---------------------------------------------------------------------------
info "P2/P3: go line + lint line structure"
begin_partition "P2"
begin_partition "P3"
CI="$ROOT/.github/workflows/ci.yml"
[ -f "$CI" ] || fail "P2 ci.yml missing: $CI"

python - "$(native "$CI")" "$SOFT_KEY" "$WHOLE_FILES_FLAG" "$FETCH_TOKEN" "$SCRIPT_TOKEN" <<'PY' > "$WORK/p23.log" 2>&1
import re, sys, yaml

path, soft_key, whole_flag = sys.argv[1], sys.argv[2], sys.argv[3]
fetch_token, script_token = sys.argv[4], sys.argv[5]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)

jobs = doc.get("jobs") or {}
if "go" not in jobs:
    raise SystemExit("P2: ci.yml has no `go` job")
j = jobs["go"]
steps = j.get("steps") or []
runs = "\n".join(s.get("run", "") for s in steps)

# WR-01 / T-48-07: a step-level `if:` disables a gate step without changing its
# command text. `false`, `always()`, `success()` and a negated expression all
# change whether the step runs, so the key itself is the violation. No
# allowlist: this job needs no conditional step (the composite action's own
# `if: inputs.X == 'true'` conditions live in .github/actions/setup/action.yml,
# not here, and are not the subject of this rule).
for s in steps:
    if "if" in s:
        label = s.get("name") or s.get("uses") or s.get("run") or "<unnamed step>"
        raise SystemExit(
            "P3: the go job's step %r carries a step-level if: %r - the gate is skippable (WR-01/T-48-07)"
            % (label, s.get("if"))
        )
print("P3-STEP-IF-OK")

# --- P2 --------------------------------------------------------------------
for frag in (
    "go build ./...",
    "go vet ./internal/ontology/... ./internal/server/... ./internal/knowledge/...",
    "go test ./... -count=1",
    "-run '^TestDeriveEdgesDeterministic$' -count=100",
):
    if frag not in runs:
        raise SystemExit("P2: go job is missing the command fragment: %s" % frag)
print("P2-CMDS-OK")

# R2-WR-04: `go test -run` with zero matching tests exits 0, so a renamed test
# makes the determinism step pass vacuously. The step carrying -count=100 must
# also prove the pinned test still exists. The list is materialised to a file
# and grepped there — a pipeline would let `grep -q` close the pipe while
# `go test -list` writes its trailing `ok` line, which is SIGPIPE / exit 141 on
# a Linux runner (round-4 gap SC-1).
det_steps = [s for s in steps if "-count=100" in (s.get("run") or "")]
if len(det_steps) != 1:
    raise SystemExit(
        "P2: expected exactly one -count=100 determinism step, found %d" % len(det_steps)
    )
det_run = det_steps[0]["run"]
if "-list '^TestDeriveEdgesDeterministic$'" not in det_run:
    raise SystemExit(
        "P2: the -count=100 determinism step lost the -list existence guard - a renamed test would pass vacuously (R2-WR-04)"
    )
if "go-test-list.txt" not in det_run:
    raise SystemExit(
        "P2: the -count=100 determinism step does not materialise the -list output to go-test-list.txt - the guard must not run through a pipe (R2-WR-04 / SC-1)"
    )
if "grep -qx 'TestDeriveEdgesDeterministic'" not in det_run:
    raise SystemExit(
        "P2: the -count=100 determinism step does not grep the materialised list for the pinned test name - the rename-proof guard is gone (R2-WR-04)"
    )
if "-list '^TestDeriveEdgesDeterministic$' |" in det_run:
    raise SystemExit(
        "P2: the -count=100 determinism step pipes the -list output into grep - `grep -q` closes the pipe early and the producer takes SIGPIPE -> exit 141 on Linux (SC-1); materialise the list and grep the file instead"
    )
print("P2-DET-OK")

shuffled = [s for s in steps if "-shuffle=on" in (s.get("run") or "")]
if not shuffled:
    raise SystemExit("P2: go job has no -shuffle=on step")
if "-v" not in shuffled[0]["run"].split("-shuffle=on")[1]:
    raise SystemExit("P2: the -shuffle=on step has no -v, so the seed never reaches the log (D-19)")
print("P2-SHUFFLE-OK")

env = j.get("env") or {}
if str(env.get("CGO_ENABLED")) != "1":
    raise SystemExit("P2: go job CGO_ENABLED is %r, want \"1\"" % (env.get("CGO_ENABLED"),))

# IN-05: hermeticity is the union of the job-level map and every step-level
# `env:` map. A step can add RETRIEVER_BACKEND / JUDGE_MODEL / BGE_M3_MODEL
# without touching the job map, which is exactly the live-path flip D-16
# forbids — so the claim is judged on the union, never on the job map alone.
keys = set(env)
for s in steps:
    keys |= set(s.get("env") or {})
extra = [k for k in keys if k != "CGO_ENABLED"]
if extra:
    raise SystemExit(
        "P2: go job declares env beyond CGO_ENABLED (job or step level): %s"
        % ", ".join(sorted(extra))
    )

# D-16: naming any of these flips cmd/spike-* off its t.Skip onto the live
# path, which is exactly what makes the go line non-hermetic.
for name in ("GLM_API_KEY", "BGE_M3_URL", "MILVUS_ADDRESS"):
    if name in src:
        raise SystemExit("P2: ci.yml references live-service env %s" % name)
print("P2-HERMETIC-OK")

# --- P3 --------------------------------------------------------------------
if soft_key in src:
    raise SystemExit("P3: ci.yml contains %s — the gate is non-blocking" % soft_key)
if "if" in j:
    raise SystemExit("P3: the go job carries a job-level `if:` — the gate is skippable")
if whole_flag in src:
    raise SystemExit("P3: ci.yml contains %s (D-04)" % whole_flag)
print("P3-BLOCKING-OK")

lint = [s for s in steps if "golangci/golangci-lint-action" in (s.get("uses") or "")]
if len(lint) != 1:
    raise SystemExit("P3: expected exactly one golangci-lint step, found %d" % len(lint))
with_ = lint[0].get("with") or {}
if with_.get("version") != "v2.12.2":
    raise SystemExit("P3: linter version is %r, want v2.12.2" % (with_.get("version"),))
if with_.get("only-new-issues") is not True:
    raise SystemExit("P3: only-new-issues is not true — the gate widens to whole-repo lint")

# D-29: only-new-issues fetches the PR diff via pulls.get. Without the scope
# the action only warns and degrades to whole-repo lint, so the map is compared
# for exact equality — an extra scope is as wrong as a missing one.
if j.get("permissions") != {"contents": "read", "pull-requests": "read"}:
    raise SystemExit(
        "P3: go job permissions are %r, want exactly {contents: read, pull-requests: read} (D-29)"
        % (j.get("permissions"),)
    )

# D-30: a repository-local action resolves from github.workspace, so checkout
# has to precede it — the composite cannot bootstrap its own checkout.
if len(steps) < 2 or steps[0].get("uses") != "actions/checkout@v7" or steps[1].get("uses") != "./.github/actions/setup":
    raise SystemExit("P3: checkout must precede the local composite action (D-30)")

# IN-10: this job executes checked-out fork content (`go test ./...`) and is
# the only job carrying an extra scope. The runner token must not be persisted
# into the checkout's local git config, where fork-run code can read it.
if (steps[0].get("with") or {}).get("persist-credentials") is not False:
    raise SystemExit(
        "P3: the go job's checkout persists credentials - fork-authored code runs in this job (IN-10)"
    )
print("P3-LINT-OK")

# --- R2-WR-01: the verify job is pinned by the probe that judges the go job --
# The IN-07 fix (wire this verifier into CI) was itself unpinned: deleting the
# job, or conditioning it on an `if:`, used to keep every partition green
# (mutations M9/M10). The contract below lives in THIS probe, so the two
# partitions cannot disagree about the fourth job either.
if "verify" not in jobs:
    raise SystemExit(
        "P2: ci.yml has no `verify` job (IN-07) - the phase gate is not wired into CI"
    )
v = jobs["verify"]
vsteps = v.get("steps") or []

# The 48-07 rule extended to the fourth job: a job-level or step-level `if:`
# neutralises the gate while leaving its command text intact (M9's shape).
if "if" in v:
    raise SystemExit(
        "P3: the verify job carries a job-level `if:` (%r) - the phase gate is skippable (R2-WR-01)"
        % (v.get("if"),)
    )
for s in vsteps:
    if "if" in s:
        label = s.get("name") or s.get("uses") or s.get("run") or "<unnamed step>"
        raise SystemExit(
            "P3: the verify job's step %r carries a step-level if: %r - the phase gate is skippable (R2-WR-01)"
            % (label, s.get("if"))
        )

# The gate is the job's terminus: moving it behind any other step reds here.
if not vsteps or "bash verify-phase-48.sh" not in (vsteps[-1].get("run") or ""):
    last_label = vsteps[-1].get("name") if vsteps else "<no steps>"
    raise SystemExit(
        "P2: the verify job's last step must run the phase gate (bash verify-phase-48.sh); the last step is %r"
        % (last_label,)
    )

# IN-10 extended: this job executes the very script that audits fork-reachable
# content, so its checkout must not persist the runner token either.
if (vsteps[0].get("uses") or "") != "actions/checkout@v7" or (
    vsteps[0].get("with") or {}
).get("persist-credentials") is not False:
    raise SystemExit(
        "P3: the verify job's first step must be actions/checkout@v7 with persist-credentials: false (IN-10)"
    )

# R2-CR-01: the linter is installed through the Go module proxy (checksum-DB
# verified), never by fetching and executing a bootstrap script — and the two
# fetch/script tokens are runtime-assembled in the caller, so this probe never
# spells them joined either.
vruns = "\n".join(s.get("run", "") for s in vsteps)
if "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" not in vruns:
    raise SystemExit(
        "P3: the verify job does not install golangci-lint through the pinned module-proxy line (go install .../cmd/golangci-lint@v2.12.2) (R2-CR-01)"
    )
for token in (fetch_token, script_token):
    if token in src:
        raise SystemExit(
            "P3: ci.yml contains the fetch/script bootstrap token %r - the install surface must be the module proxy (R2-CR-01)"
            % token
        )

print("P3-VERIFYJOB-OK")
PY
p23_rc=$?
cat "$WORK/p23.log"
[ "$p23_rc" -ne 0 ] && fail "P2/P3: the go job's structural contract is violated (see the line above)"

# One shared probe certifies two partitions (D-29/P2/P3 disagree-proof), and
# each assertion group inside it prints its OWN token (WR-03 / M13): a
# partition's deposit requires every one of ITS groups to have run, so
# deleting either partition's assertion block leaves that partition's tokens
# missing instead of borrowing the sibling's evidence.
ok_partition "P2" "$WORK/p23.log" "P2-CMDS-OK" "P2-DET-OK" "P2-SHUFFLE-OK" "P2-HERMETIC-OK"
ok_partition "P3" "$WORK/p23.log" "P3-STEP-IF-OK" "P3-BLOCKING-OK" "P3-LINT-OK" "P3-VERIFYJOB-OK"

done_partition "P2 go-line (5 command fragments, materialised-list determinism shape: -list redirects to go-test-list.txt, grep -qx reads the file, a pipe directly after -list is rejected; CGO_ENABLED=1 only, shuffle seed under -v, verify job ends at the gate; per-assertion success tokens P2-CMDS-OK P2-DET-OK P2-SHUFFLE-OK P2-HERMETIC-OK)"
done_partition "P3 lint-line (pinned v2.12.2, blocking, permissions map exact, verify job if-free with module-proxy install; per-assertion success tokens P3-STEP-IF-OK P3-BLOCKING-OK P3-LINT-OK P3-VERIFYJOB-OK)"

# ---------------------------------------------------------------------------
# P4: lint-teeth
# ---------------------------------------------------------------------------
info "P4: lint has teeth (config verify + sentinels)"
begin_partition "P4"

# TOOLING PREFLIGHT FIRST, and it depends on nothing but the shell — no Python,
# no Node, no Go, no Docker. If the linter is off PATH this partition must
# still be REACHABLE and must still refuse. Absent or wrong-version tooling is
# an unverified gate, never a passing one, so this path exits at once and never
# reaches the sentinels.
#
# Both refusal reasons share ONE print site. The not-verified literal is counted
# by later plans' own greps, so a second occurrence anywhere in this file — even
# in a comment — would make those checks unable to tell this script's text from
# the workflow's text.
lint_missing_reason=''
if ! command -v golangci-lint >/dev/null 2>&1; then
  lint_missing_reason="golangci-lint is absent from PATH (want v${LINT_VERSION})"
else
  lint_version=$(golangci-lint --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
  if [ "$lint_version" != "$LINT_VERSION" ]; then
    lint_missing_reason="golangci-lint reports version '${lint_version:-<unparsed>}', want ${LINT_VERSION}"
  fi
fi
if [ -n "$lint_missing_reason" ]; then
  echo -e "${YELLOW}LINT NOT VERIFIED${NC}"
  echo "${lint_missing_reason}; the lint gate produced no evidence."
  exit 1
fi

# lint_run <logname> <workdir> <args...> — run the pinned linter into its own
# log and fail the gate when that log is empty (WR-06). Returns the linter's
# exit code so the caller can assert on it.
#
# This script exports MSYS_NO_PATHCONV=1 (inherited from verify-phase-47.sh, so
# that a WSL path is never mangled), and that setting also stops the MSYS
# runtime from translating a /d/code/... argument into D:\code\... for a NATIVE
# binary. golangci-lint here is a native Win32 build, so every path handed to it
# is converted explicitly instead — otherwise the linter silently runs against
# a directory that does not exist, which would surface as an empty log (caught)
# or, worse, as a config that "verified" against nothing.
lint_run() {
  local log="$WORK/$1" dir="$2"
  shift 2
  local -a args=()
  local a
  for a in "$@"; do
    case "$a" in
      /*) args+=("$(native "$a")") ;;
      *) args+=("$a") ;;
    esac
  done
  ( cd "$(native "$dir")" && golangci-lint "${args[@]}" ) >"$log" 2>&1
  local rc=$?
  if [ ! -s "$log" ]; then
    fail "P4 the linter produced no output (rc=$rc) — infrastructure failure, no evidence of a lint run"
  fi
  return "$rc"
}

# config_verify — `golangci-lint config verify` prints NOTHING when the config
# is accepted, so for that one invocation silence is its documented success
# signal (the same shape as `tsc --noEmit` in verify-phase-47.sh P6, and for
# the same reason). Applying the empty-log rule blindly here would fail the gate
# on a perfectly good config forever, which is how a guard gets deleted instead
# of fixed. The rule's real target — a crashed toolchain passing as "zero
# findings" — is not weakened: the exit code stays authoritative, and a
# non-silent failure fails immediately. What is added is an explicit evidence
# line, so the log is never empty and an auditor can still see what was judged.
golangci-lint config verify --config "$(native "$ROOT/.golangci.yml")" > "$WORK/p4-config.log" 2>&1
p4_cfg_rc=$?
{
  echo "exit code: ${p4_cfg_rc}"
  echo "tool output: $( [ -s "$WORK/p4-config.log" ] && echo present || echo 'none (success is silent)' )"
} >> "$WORK/p4-config.log"
[ "$p4_cfg_rc" -eq 0 ] || fail "P4 golangci-lint config verify rejected .golangci.yml (rc=$p4_cfg_rc)"
info "P4: config verify accepted .golangci.yml"

# Sentinel A — the violation. An unchecked json.Unmarshal must be reported, and
# reported as errcheck specifically: a red run from some other cause would
# prove only that the binary can fail.
SA="$WORK/sentinel-a"
mkdir -p "$SA"
printf 'module sentinel\n\ngo 1.21\n' > "$SA/go.mod"
cat > "$SA/main.go" <<'GOEOF'
// Package main is the sentinel module for the lint-teeth partition.
package main

import "encoding/json"

func main() {
	var v any
	json.Unmarshal([]byte("{}"), &v)
}
GOEOF
lint_run "p4-sentinel-a.log" "$SA" run --config "$ROOT/.golangci.yml" ./...
p4_a_rc=$?
if [ "$p4_a_rc" -eq 0 ]; then
  fail "P4 sentinel A: the linter exited 0 on an unchecked json.Unmarshal — the gate has no teeth"
fi
# The word alone is not evidence: a config diagnostic, a deprecation warning or
# another linter's header can all contain it. Evidence is the attribution the
# linter emits for the finding under test (IN-01).
if ! grep -qE 'is not checked \(errcheck\)' "$WORK/p4-sentinel-a.log"; then
  tail -20 "$WORK/p4-sentinel-a.log"
  fail "P4 sentinel A: the run failed but did not name errcheck — a different cause is failing, not the gate"
fi
printf 'P4-A-ERRCHECK\n' >> "$WORK/p4-sentinel-a.log"
pass "P4 sentinel A: an unchecked json.Unmarshal is reported as errcheck"

# Sentinel B — absorbed noise. A bare deferred Close is what the
# std-error-handling preset exists to absorb, so it must stay quiet. Asserting
# the run is green here is how the config earns "noise absorbed by a preset"
# rather than "noise absorbed by an exemption list nobody reads" (D-03).
SB="$WORK/sentinel-b"
mkdir -p "$SB"
printf 'module sentinelb\n\ngo 1.21\n' > "$SB/go.mod"
cat > "$SB/main.go" <<'GOEOF'
// Package main is the sentinel module for the deferred-close check.
package main

import "os"

func main() {
	f, _ := os.Create("sentinel-b.tmp")
	defer f.Close()
}
GOEOF
lint_run "p4-sentinel-b.log" "$SB" run --config "$ROOT/.golangci.yml" ./...
p4_b_rc=$?
[ "$p4_b_rc" -eq 0 ] || { tail -20 "$WORK/p4-sentinel-b.log"; fail "P4 sentinel B: a bare deferred Close was reported — the std-error-handling preset is not absorbing it (D-03)"; }
# A silent run could also be a probe that never executed. The linter's own
# clean-run line is the evidence that this module was actually judged.
if [ ! -s "$WORK/p4-sentinel-b.log" ]; then
  fail "P4 sentinel B produced no output — infrastructure failure, no evidence the noise-absorption probe ran (R2-WR-03)"
fi
if ! grep -qF '0 issues' "$WORK/p4-sentinel-b.log"; then
  tail -20 "$WORK/p4-sentinel-b.log"
  fail "P4 sentinel B: the probe log does not report '0 issues' — a neutralised or deleted probe cannot certify absorption (R2-WR-03)"
fi
printf 'P4-B-QUIET\n' >> "$WORK/p4-sentinel-b.log"
pass "P4 sentinel B: a bare deferred Close stays quiet (std-error-handling)"

# Sentinel C — the disabled rule. An interface-shaped handler legitimately
# ignores a parameter, and revive's unused-parameter false-fires on exactly
# that shape (D-03).
SC="$WORK/sentinel-c"
mkdir -p "$SC"
printf 'module sentinelc\n\ngo 1.21\n' > "$SC/go.mod"
cat > "$SC/main.go" <<'GOEOF'
// Package main is the sentinel module for the unused-parameter check.
package main

// Handle mimics an MCP-shaped handler: the transport contract fixes the
// signature, so the ignored parameter is the interface's shape, not a defect.
func Handle(ctx string, request any) any {
	return request
}

func main() {
	_ = Handle("ctx", "req")
}
GOEOF
lint_run "p4-sentinel-c.log" "$SC" run --config "$ROOT/.golangci.yml" ./...
p4_c_rc=$?
[ "$p4_c_rc" -eq 0 ] || { tail -20 "$WORK/p4-sentinel-c.log"; fail "P4 sentinel C: an unused parameter was reported — the revive rule is not disabled (D-03)"; }
if [ ! -s "$WORK/p4-sentinel-c.log" ]; then
  fail "P4 sentinel C produced no output — infrastructure failure, no evidence the unused-parameter probe ran (R2-WR-03)"
fi
if ! grep -qF '0 issues' "$WORK/p4-sentinel-c.log"; then
  tail -20 "$WORK/p4-sentinel-c.log"
  fail "P4 sentinel C: the probe log does not report '0 issues' — a neutralised or deleted probe cannot certify the rule is disabled (R2-WR-03)"
fi
printf 'P4-C-QUIET\n' >> "$WORK/p4-sentinel-c.log"
pass "P4 sentinel C: an ignored parameter stays quiet (revive rule disabled)"

# Sentinel D — the positive revive probe (WR-04). A/B/C together cannot tell
# "revive is configured" from "revive is absent": C's unused parameter stays
# quiet for the trivial reason that no revive rule ran. This module carries a
# missing package comment, the one violation only revive's `package-comments`
# rule reports. Deleting `linters.enable: [revive]` strips that attribution and
# turns the gate red here.
#
# The unchecked json.Unmarshal is load-bearing, not decoration. With revive
# removed the errcheck finding keeps BOTH the exit code non-zero and the log
# non-empty, so lint_run's empty-log guard can never abort ahead of sentinel D's
# own attribution check. A package-comment-only module goes silent (rc 0, no
# output) the instant revive disappears, and which branch then surfaces the
# failure is a formatting detail of the binary — this shape guarantees the
# branch that runs names revive.
SD="$WORK/sentinel-d"
mkdir -p "$SD"
printf 'module sentinel-d\n\ngo 1.21\n' > "$SD/go.mod"
cat > "$SD/main.go" <<'GOEOF'
package main

import "encoding/json"

func main() {
	var v any
	json.Unmarshal([]byte("{}"), &v)
}
GOEOF
lint_run "p4-sentinel-d.log" "$SD" run --config "$ROOT/.golangci.yml" ./...
p4_d_rc=$?
if [ "$p4_d_rc" -eq 0 ]; then
  fail "P4 sentinel D: a missing package comment stayed quiet — revive is not running (WR-04)"
fi
if ! grep -q 'revive' "$WORK/p4-sentinel-d.log"; then
  tail -20 "$WORK/p4-sentinel-d.log"
  fail "P4 sentinel D: the run failed but did not name revive — a different cause is failing, not the revive rule (WR-04)"
fi
printf 'P4-D-REVIVE\n' >> "$WORK/p4-sentinel-d.log"
pass "P4 sentinel D: a missing package comment is reported as revive"

# Each sentinel is its own assertion group, so each emits its own token
# (WR-03). The four probe logs concatenate into ONE evidence file, and the
# concatenation is deliberately missing-file-tolerant: a deleted sentinel's
# log will not exist, and an unguarded `cat` of all four paths would abort
# before the deposit could name the missing token. Truncate first, then append
# each log only when it exists — no `|| true`, no unguarded cat.
: > "$WORK/p4.log"
for s in a b c d; do if [ -f "$WORK/p4-sentinel-$s.log" ]; then cat "$WORK/p4-sentinel-$s.log" >> "$WORK/p4.log"; fi; done

ok_partition "P4" "$WORK/p4.log" "P4-A-ERRCHECK" "P4-B-QUIET" "P4-C-QUIET" "P4-D-REVIVE"

done_partition "P4 lint-teeth (config verify + sentinel A errcheck and sentinel D revive fire, sentinels B and C stay quiet; per-assertion success tokens P4-A-ERRCHECK P4-B-QUIET P4-C-QUIET P4-D-REVIVE)"

# ---------------------------------------------------------------------------
# P5: composite
# ---------------------------------------------------------------------------
info "P5: composite resolution order"
begin_partition "P5"

[ -f "$COMPOSITE" ] || fail "P5 composite toolchain action missing: $COMPOSITE"

python - "$(native "$COMPOSITE")" "$(native "$CI")" <<'PY' > "$WORK/p5.log" 2>&1
import sys, yaml

composite_path, ci_path = sys.argv[1], sys.argv[2]
composite = yaml.safe_load(open(composite_path, encoding="utf-8"))
src = open(composite_path, encoding="utf-8").read()

if (composite.get("runs") or {}).get("using") != "composite":
    raise SystemExit("P5: .github/actions/setup/action.yml is not `runs.using: composite`")

# D-22: the three toolchain majors live in exactly one place.
for action in ("actions/setup-go@v7", "actions/setup-node@v7", "docker/setup-buildx-action@v4"):
    n = src.count(action)
    if n != 1:
        raise SystemExit("P5: composite contains %s %d time(s), want exactly 1" % (action, n))

# D-30: a repository-local action resolves from github.workspace, so the
# composite cannot bootstrap its own checkout — a second checkout would also
# reset the caller's workspace.
if "actions/checkout" in src:
    raise SystemExit("P5: composite contains a checkout — callers must check out first (D-30)")

ci = yaml.safe_load(open(ci_path, encoding="utf-8"))
jobs = ci.get("jobs") or {}
if not jobs:
    raise SystemExit("P5: ci.yml has no jobs")

for name, job in jobs.items():
    steps = job.get("steps") or []
    uses = [s.get("uses") for s in steps]
    if not uses or uses[0] != "actions/checkout@v7":
        raise SystemExit("P5: job %r does not check out first (D-30)" % name)
    if "./.github/actions/setup" not in uses[1:]:
        raise SystemExit("P5: job %r never calls the composite after checkout" % name)

print("COMPOSITE-ORDER-OK")
PY
p5_rc=$?
cat "$WORK/p5.log"
[ "$p5_rc" -ne 0 ] && fail "P5 composite resolution order is violated (see the line above)"

ok_partition "P5" "$WORK/p5.log" "COMPOSITE-ORDER-OK"

done_partition "P5 composite (three pinned majors, no checkout, every job checks out before setup; evidence-bound success token COMPOSITE-ORDER-OK)"

# ---------------------------------------------------------------------------
# P6: fe-line
# ---------------------------------------------------------------------------
info "P6: fe line"
begin_partition "P6"

python - "$(native "$CI")" <<'PY' > "$WORK/p6.log" 2>&1
import re, sys, yaml

path = sys.argv[1]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)
jobs = doc.get("jobs") or {}

if "fe" not in jobs:
    raise SystemExit("P6: ci.yml has no `fe` job (CICD-02)")

fe_job = jobs["fe"]
steps = fe_job.get("steps") or []

# P6-A: job-level if
# R2-WR-01 class, extended to the fe line (D10): a job-level `if:` makes the
# whole gate skippable while its command text stays intact. P3 already applies
# this rule to the go and verify jobs; the key's presence is the violation, and
# there is no allowlist.
if "if" in fe_job:
    raise SystemExit(
        "P6: the fe job carries a job-level if: %r - the gate is skippable (R2-WR-01)"
        % (fe_job.get("if"),)
    )
print("P6-JOB-IF-OK")

# P6-B: step-level if
# WR-01 / T-48-07: same rule as P2/P3 — any step-level `if:` can disable a gate
# step while its command text stays intact. No allowlist: neither the fe job nor
# the docker job needs a conditional step (artifact-upload `if: always()` lives
# in eval-gate.yml, which neither partition judges).
for s in steps:
    if "if" in s:
        label = s.get("name") or s.get("uses") or s.get("run") or "<unnamed step>"
        raise SystemExit(
            "P6: the fe job's step %r carries a step-level if: %r - the gate is skippable (WR-01)"
            % (label, s.get("if"))
        )
print("P6-STEP-IF-OK")

# P6-C: commands
runs = {(s.get("run") or "").strip(): s.get("working-directory") for s in steps if "run" in s}
for cmd in ("npm ci", "npx tsc --noEmit", "npx vitest run"):
    if cmd not in runs:
        raise SystemExit("P6: fe job is missing the step: %s" % cmd)
    if runs[cmd] != "web":
        raise SystemExit("P6: fe step %r has working-directory %r, want 'web'" % (cmd, runs[cmd]))
print("P6-CMDS-OK")

# P6-D: count literals
# A test count in a gate is a future silent-drift failure: the suite grows,
# the literal does not, and the gate starts asserting a stale fact.
#
# IN-04: anchor to how a count is actually written, in both orders:
#   digits-first  `689 tests`
#   word-first    `Tests  84 passed`   (the Vitest summary line)
# The gap between the count word and the number is spaces/tabs with at most one
# `:` or `=`. It is deliberately not `[^\n]{0,40}`: this file's own step name
# `go test ./... (CICD-01 / D-15)` puts `test` 19 characters before `15`, and a
# loose gap would red the gate on its own baseline. Confining the gap to
# `[ \t]` (not `\s`) also keeps it from spanning the newline between a
# `timeout-minutes: 45` line and a following step name. Ports (8084) and years
# (1984) match neither shape.
count_phrase = re.compile(
    r"(?:\b\d{2,}[ \t]+(?:tests?|suites?|files?|passed)\b)"
    r"|(?:\b(?:tests?|suites?|files?|passed)\b[ \t]*[:=]?[ \t]*\d{2,}\b)",
    re.IGNORECASE,
)
m = count_phrase.search(src)
if m:
    raise SystemExit("P6: ci.yml hard-codes a test-count phrase: %r" % m.group(0))

# The two Go-test totals this file must never pin, as standalone tokens.
for literal in (r"\b666\b", r"\b689\b"):
    m = re.search(literal, src)
    if m:
        raise SystemExit("P6: ci.yml hard-codes a test-count literal: %r" % m.group(0))

print("P6-COUNTS-OK")
PY
p6_rc=$?
cat "$WORK/p6.log"
[ "$p6_rc" -ne 0 ] && fail "P6 the fe line is violated (see the line above)"

ok_partition "P6" "$WORK/p6.log" "P6-JOB-IF-OK" "P6-STEP-IF-OK" "P6-CMDS-OK" "P6-COUNTS-OK"

done_partition "P6 fe-line (npm ci, tsc --noEmit, vitest run under web/, no count literals, rejects both job- and step-level if:; per-assertion success tokens P6-JOB-IF-OK P6-STEP-IF-OK P6-CMDS-OK P6-COUNTS-OK)"

# ---------------------------------------------------------------------------
# P7: docker-line
# ---------------------------------------------------------------------------
info "P7: docker line"
begin_partition "P7"

python - "$(native "$CI")" <<'PY' > "$WORK/p7.log" 2>&1
import sys, yaml

path = sys.argv[1]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)
jobs = doc.get("jobs") or {}

if "docker" not in jobs:
    raise SystemExit("P7: ci.yml has no `docker` job (CICD-03)")

docker_job = jobs["docker"]
steps = docker_job.get("steps") or []

# P7-A: job-level if
# R2-WR-01 class, extended to the docker line (D11): a job-level `if:` makes
# the whole gate skippable while its build steps stay intact. Same rule P3
# applies to the go and verify jobs; no allowlist.
if "if" in docker_job:
    raise SystemExit(
        "P7: the docker job carries a job-level if: %r - the gate is skippable (R2-WR-01)"
        % (docker_job.get("if"),)
    )
print("P7-JOB-IF-OK")

# P7-B: step-level if
# WR-01: the two Docker image builds are the docker line's gate steps. A
# step-level `if:` on either one makes CICD-03 silently unverified.
for s in steps:
    if "if" in s:
        label = s.get("name") or s.get("uses") or s.get("run") or "<unnamed step>"
        raise SystemExit(
            "P7: the docker job's step %r carries a step-level if: %r - the gate is skippable (WR-01)"
            % (label, s.get("if"))
        )
print("P7-STEP-IF-OK")

# P7-C: builds
builds = [s for s in steps if (s.get("uses") or "").startswith("docker/build-push-action")]
if len(builds) != 2:
    raise SystemExit("P7: expected exactly 2 docker build steps, found %d" % len(builds))

files = [(b.get("with") or {}).get("file") for b in builds]
if files != ["Dockerfile.backend", "Dockerfile.frontend"]:
    raise SystemExit("P7: build step Dockerfiles are %r, want backend then frontend" % (files,))
print("P7-BUILDS-OK")

# P7-D: push
for b in builds:
    with_ = b.get("with") or {}
    if with_.get("context") != ".":
        raise SystemExit("P7: build step %r context is %r, want '.'" % (b.get("name"), with_.get("context")))
    # D-24: CI never publishes an image and never produces a local one.
    if with_.get("push") is not False:
        raise SystemExit("P7: build step %r does not carry push: false" % (b.get("name"),))
print("P7-PUSH-OK")

# P7-E: no load
for bad in ("load: true", "docker compose build"):
    if bad in src:
        raise SystemExit("P7: ci.yml contains %r (D-24/D-25)" % bad)
print("P7-NO-LOAD-OK")

# P7-F: cache scopes
# D-23: per-image scopes. A duplicate is how the second image starts evicting
# the first, and a missing one falls back to the shared default scope.
for scope in (
    "cache-from: type=gha,scope=backend",
    "cache-from: type=gha,scope=frontend",
    "cache-to: type=gha,mode=max,scope=backend",
    "cache-to: type=gha,mode=max,scope=frontend",
):
    n = src.count(scope)
    if n != 1:
        raise SystemExit("P7: ci.yml contains %r %d time(s), want exactly 1" % (scope, n))
print("P7-CACHE-OK")
PY
p7_rc=$?
cat "$WORK/p7.log"
[ "$p7_rc" -ne 0 ] && fail "P7 the docker line is violated (see the line above)"

ok_partition "P7" "$WORK/p7.log" "P7-JOB-IF-OK" "P7-STEP-IF-OK" "P7-BUILDS-OK" "P7-PUSH-OK" "P7-NO-LOAD-OK" "P7-CACHE-OK"

done_partition "P7 docker-line (both Dockerfiles, push: false, per-image cache scopes, rejects both job- and step-level if:; per-assertion success tokens P7-JOB-IF-OK P7-STEP-IF-OK P7-BUILDS-OK P7-PUSH-OK P7-NO-LOAD-OK P7-CACHE-OK)"

# ---------------------------------------------------------------------------
# scan_trust_surface <workflow-dir> — ONE implementation, two callers.
#
# P8 calls it with the real .github/workflows; P11 half 2 calls it with
# synthetic fixtures. Sharing the function is the whole point: a private probe
# copy could drift from what P8 actually runs, and then the probe would prove
# nothing about the real scan. Every rule P8 has always had lives here, and the
# four vacuity rules (zero files / empty jobs / absent permissions / empty
# permissions) are failure conditions in their own right — `every value is
# read-or-none` is satisfied VACUOUSLY by a mapping with no values, which is
# exactly the "found nothing, reported green" class this phase exists to close.
#
# Returns non-zero and writes its reason to $WORK/trust-surface.log. It never
# calls fail(): P11 must observe the failure rather than abort on it. Callers
# keep the exit-code-authoritative failure at their own site.
#
# Every fixture file is scanned by the same per-file rules the real directory
# runs — the path-filter, secret, permission and publication-token rules are
# evaluated once per enumerated workflow, never against one privileged name.
# ---------------------------------------------------------------------------
scan_trust_surface() {
  local dir="$1"
  local log="$WORK/trust-surface.log"
  local list="$WORK/trust-surface-list.txt"
  : > "$log"

  # GitHub Actions accepts both extensions. Enumerating only *.yml made a
  # *.yaml workflow invisible to every rule below (WR-05).
  find "$dir" -maxdepth 1 -type f \( -name '*.yml' -o -name '*.yaml' \) -print 2>/dev/null | sort > "$list"
  TRUST_SURFACE_COUNT=$(wc -l < "$list" | tr -d ' ')
  if [ "$TRUST_SURFACE_COUNT" -lt 1 ]; then
    echo "trust-surface: zero *.yml/*.yaml files under $dir — a partition that finds nothing must not pass" >> "$log"
    return 1
  fi

  # The phase's three-job matrix must exist under its known name — P2/P5/P9
  # reference ci.yml directly. Keep a subject guard, but do not privilege
  # that one filename for the rules themselves: they are now per-file.
  local ci_subject=0
  local f
  while IFS= read -r f; do
    case "$(basename "$f")" in
      ci.yml|ci.yaml) ci_subject=1 ;;
    esac
  done < "$list"
  if [ "$ci_subject" -ne 1 ]; then
    echo "trust-surface: no ci.yml or ci.yaml under $dir — the path-filter and secret rules have no subject" >> "$log"
    return 1
  fi

  local -a args=()
  while IFS= read -r f; do args+=("$(native "$f")"); done < "$list"

  python - "${args[@]}" >> "$log" 2>&1 <<'PY'
import os, sys, re, yaml

paths = sys.argv[1:]
if not paths:
    raise SystemExit("trust-surface: zero workflow files to scan — a partition that finds nothing must not pass")

# Secret allowlist (T-48-04 / D-14). GITHUB_TOKEN is the runner-provided
# token and is available everywhere. The three live-service names are
# permitted ONLY in the one workflow that legitimately consumes them (P9
# independently asserts those three mappings stay intact). This replaces the
# old "whatever ci.yml happens to contain is fine" accident with an explicit,
# named exception.
ALLOWED_ANYWHERE = {"GITHUB_TOKEN"}
ALLOWED_IN_EVAL_GATE = {"GLM_API_KEY", "BGE_M3_URL", "MILVUS_ADDRESS"}
EVAL_GATE_BASENAME = "eval-gate.yml"

# T-48-10: matched case-insensitively against comment-stripped text. The old
# uppercase-only list could not see a real publication command.
PUBLICATION_TOKENS = (
    "ghcr.io",
    "docker.io",
    "quay.io",
    "docker/login-action",
    "registry:",
    "dockerhub",
    "docker push",
    "packages: write",
    "push: true",
)

# D-26/D-29: a value-shaped write scope. Line-anchored so prose such as
# "this workflow writes the PR comment" is not mistaken for a permission.
WRITE_VALUE = re.compile(r"^\s*[A-Za-z0-9_-]+:\s*write(-all)?\s*$")
SECRET_REF = re.compile(r"secrets\.([A-Za-z_][A-Za-z0-9_]*)")


def judge_permissions_string(p, where, value):
    # IN-03: GitHub accepts string shorthand. `none` grants nothing and is
    # inside D-29's ceiling. `read-all` is DELIBERATELY rejected: it grants
    # every read scope (actions, checks, packages, id-token, …), which is
    # wider than D-29's recorded two-scope enumeration
    # ({contents: read, pull-requests: read}). Accepting it would require a
    # D-29 amendment, not a code change.
    if value == "none":
        return
    if value in ("read-all", "write-all"):
        raise SystemExit(
            "trust-surface: %s %s permissions grants %r; %r exceeds D-29's "
            "{contents: read, pull-requests: read} ceiling (D-26/D-29)"
            % (p, where, value, value)
        )
    raise SystemExit(
        "trust-surface: %s %s permissions grants unknown string %r; only `none` is allowed (D-26/D-29)"
        % (p, where, value)
    )


for p in paths:
    src = open(p, encoding="utf-8").read()
    # Every rule below judges comment-stripped text: a comment that mentions
    # a path filter, a secret or a publication token is not the surface
    # itself (IN-02).
    nocomment = re.sub(r"(?m)#.*$", "", src)
    doc = yaml.safe_load(src)
    if not isinstance(doc, dict):
        raise SystemExit("trust-surface: %s does not parse as a mapping" % p)

    # Vacuity rule 1: an empty workflow has no permissions to be wrong about,
    # so "zero write scopes" must not be satisfied by having no scopes at all.
    jobs = doc.get("jobs")
    if not isinstance(jobs, dict) or not jobs:
        raise SystemExit(
            "trust-surface: %s has an empty or absent jobs: mapping — an empty partition must not pass" % p
        )

    # T-48-10: no image-publication capability in ANY workflow file.
    lower = nocomment.lower()
    for bad in PUBLICATION_TOKENS:
        if bad in lower:
            raise SystemExit(
                "trust-surface: %s contains forbidden registry/publication token %r "
                "(case-insensitive match, T-48-10)" % (p, bad)
            )

    # D-26 / D-29: value-shaped write detection over comment-stripped lines.
    for lineno, line in enumerate(nocomment.splitlines(), 1):
        if WRITE_VALUE.match(line):
            raise SystemExit(
                "trust-surface: %s:%d contains a write scope (D-26/D-29)" % (p, lineno)
            )

    # Vacuity rules 2 and 3: a workflow with no permissions key has not
    # declared a read-only ceiling, and an empty mapping declares nothing.
    perms = doc.get("permissions")
    if perms is None:
        raise SystemExit(
            "trust-surface: %s declares no permissions: mapping — an undeclared ceiling is not a read-only ceiling (D-26/D-29)" % p
        )
    if isinstance(perms, str):
        judge_permissions_string(p, "workflow-level", perms)
    elif isinstance(perms, dict):
        if not perms:
            raise SystemExit(
                "trust-surface: %s declares an empty permissions: mapping — an empty ceiling must not pass vacuously (D-26/D-29)" % p
            )
        for scope, value in perms.items():
            if value not in ("read", "none"):
                raise SystemExit(
                    "trust-surface: %s in workflow-level grants %r=%r; only read/none are allowed (D-26/D-29)"
                    % (p, scope, value)
                )
    else:
        raise SystemExit(
            "trust-surface: %s workflow-level permissions is %r, expected a mapping or string (D-26/D-29)"
            % (p, perms)
        )

    # Job-level permissions. The old isinstance(..., dict) branch silently
    # skipped string forms — `write-all` was judged only by the raw text scan
    # and `read-all` was not judged at all (IN-03).
    for name, job in jobs.items():
        if not isinstance(job, dict):
            continue
        job_perms = job.get("permissions")
        if job_perms is None:
            continue
        if isinstance(job_perms, str):
            judge_permissions_string(p, "job %r" % name, job_perms)
        elif isinstance(job_perms, dict):
            for scope, value in job_perms.items():
                if value not in ("read", "none"):
                    raise SystemExit(
                        "trust-surface: %s in job %r grants %r=%r; only read/none are allowed (D-26/D-29)"
                        % (p, name, scope, value)
                    )
        else:
            raise SystemExit(
                "trust-surface: %s job %r permissions is %r, expected a mapping or string (D-26/D-29)"
                % (p, name, job_perms)
            )

# T-48-03: parsed path-filter walk
    # WR-02 (round-4 review) / 48-VERIFICATION advisory 2: GitHub judges path
    # filters on the document it PARSES, and a quoted key (`"paths":`) or a
    # spaced key (`paths :`) is valid YAML that the textual rule below cannot
    # see — its substring `paths:` never occurs in either spelling. The parsed
    # document is therefore the PRIMARY judge. The trigger block is read with
    # the PyYAML 1.1 shape in mind (a bare `on:` loads as the boolean True).
    # A sequence trigger (`on: [push, pull_request]`) carries bare event names
    # with no configuration mapping, so there is nothing to inspect and this
    # walk is simply skipped for that file. The textual rule below is retained
    # UNCHANGED as the second layer, so every shape it caught is still caught.
    trigger = doc.get("on", doc.get(True))
    if isinstance(trigger, dict):
        for event, config in trigger.items():
            if isinstance(config, dict) and ("paths" in config or "paths-ignore" in config):
                bad = "paths" if "paths" in config else "paths-ignore"
                raise SystemExit(
                    "trust-surface: %s filters event %r with a %r key — a path-filtered "
                    "required check sits Pending on a PR that touches no matched path (T-48-03)"
                    % (p, event, bad)
                )
# T-48-03: end parsed path-filter walk

    # T-48-03: a path-filtered required check sits Pending forever on a PR
    # that touches no matched path. Per-file: a second workflow does not get
    # an exemption just because it is not named ci.yml. SECOND LAYER: the
    # parsed walk above judges the document GitHub executes; this textual rule
    # is retained unchanged so the P11 mutation drill stays attributable and
    # no shape it caught is lost.
    for bad in ("paths:", "paths-ignore:"):
        if bad in nocomment:
            raise SystemExit(
                "trust-surface: %s contains %s — a path-filtered required check sits Pending (T-48-03)"
                % (p, bad)
            )

# T-48-04: parsed secret walk
    # WR-02 (round-4 review) / 48-VERIFICATION advisory 2, PRIMARY layer over
    # the textual scan below. The textual scan judges comment-stripped text,
    # and `#` inside a quoted YAML scalar is DATA, not a comment — so
    # `run: "echo x# ${{ secrets.DEPLOY_KEY }}"` hides its reference from the
    # strip while GitHub still executes it, and `toJSON(secrets)` (which
    # expands the whole secrets context) has no name after a dot for the
    # textual regex to find. Walk every string VALUE of the parsed document
    # (dict values, list items, scalar strings) and reject the same shapes
    # under the same allowlist semantics. The textual `SECRET_REF` scan below
    # is retained UNCHANGED as the second layer.
    basename = os.path.basename(p)

    def walk_strings(node):
        if isinstance(node, str):
            yield node
        elif isinstance(node, dict):
            for v in node.values():
                for s in walk_strings(v):
                    yield s
        elif isinstance(node, list):
            for v in node:
                for s in walk_strings(v):
                    yield s

    for s in walk_strings(doc):
        for m in SECRET_REF.finditer(s):
            name = m.group(1)
            if name in ALLOWED_ANYWHERE:
                continue
            if basename == EVAL_GATE_BASENAME and name in ALLOWED_IN_EVAL_GATE:
                continue
            raise SystemExit(
                "trust-surface: %s parsed value references secrets.%s; only secrets.GITHUB_TOKEN "
                "is allowed outside eval-gate.yml (T-48-04)"
                % (p, name)
            )
        if "secrets[" in s:
            raise SystemExit(
                "trust-surface: %s parsed value contains a dynamic secrets[ reference (T-48-04)" % p
            )
        lowered = s.lower()
        if "tojson(secrets" in lowered or "tojson( secrets" in lowered:
            raise SystemExit(
                "trust-surface: %s parsed value contains a toJSON(secrets) expression — it "
                "expands the whole secrets context (T-48-04)" % p
            )
# T-48-04: end parsed secret walk

    # T-48-04 / D-14: a public fork PR must never need or see a secret.
    # SECOND LAYER: the parsed walk above is primary; this comment-stripped
    # textual scan is retained unchanged so the P11 mutation drill stays
    # attributable (stray-secret stays rejected with the walk deleted).
    basename = os.path.basename(p)
    for name in SECRET_REF.findall(nocomment):
        if name in ALLOWED_ANYWHERE:
            continue
        if basename == EVAL_GATE_BASENAME and name in ALLOWED_IN_EVAL_GATE:
            continue
        raise SystemExit(
            "trust-surface: %s references secrets.%s; only secrets.GITHUB_TOKEN is allowed outside eval-gate.yml (T-48-04)"
            % (p, name)
        )

print("TRUST-SURFACE-OK (%d workflow file(s), path filters and secret references checked per file)" % len(paths))
PY
  return $?
}

# ---------------------------------------------------------------------------
# P8: trust-surface — the whole workflow directory
# ---------------------------------------------------------------------------
info "P8: trust surface (permissions / registry / path filters / secrets)"
begin_partition "P8"

scan_trust_surface "$ROOT/.github/workflows"
p8_rc=$?
cat "$WORK/trust-surface.log"
[ "$p8_rc" -ne 0 ] && fail "P8 the trust surface is violated (see the line above)"

ok_partition "P8" "$WORK/trust-surface.log" "TRUST-SURFACE-OK"

# R2-WR-03 / A2: TRUST_SURFACE_COUNT is set by the probe under test, so it
# carries NO `:-` default. A default would print a plausible zero and pass the
# partition after the probe was deleted (the M8 mutation); under
# `set -uo pipefail` (header) the bare unbound variable aborts instead.
done_partition "P8 trust-surface (${TRUST_SURFACE_COUNT} workflow(s) per file (*.yml + *.yaml): read-only permissions, no registry/path/secret surface; evidence-bound success token TRUST-SURFACE-OK)"

# ---------------------------------------------------------------------------
# P9: eval-gate-gate — the fork boundary, the visible skip, and the Go version
# ---------------------------------------------------------------------------
info "P9: eval-gate gate + the directory-wide Go version source"
begin_partition "P9"

EVAL_GATE="$ROOT/.github/workflows/eval-gate.yml"
[ -f "$EVAL_GATE" ] || fail "P9 no eval-gate workflow at $EVAL_GATE — a partition with no subject is NOT VERIFIED, never a pass"

python - "$(native "$EVAL_GATE")" "$(native "$WF_DIR")" "$(native "$ROOT/.github/actions")" <<'PY' > "$WORK/p9.log" 2>&1
import os, sys, yaml

gate_path, wf_dir, actions_dir = sys.argv[1], sys.argv[2], sys.argv[3]
src = open(gate_path, encoding="utf-8").read()
doc = yaml.safe_load(src)

# --- the two job keys -------------------------------------------------------
jobs = doc.get("jobs") or {}
want = ["eval-gate", "eval-gate-skip-notice"]
if sorted(jobs) != want:
    raise SystemExit(
        "P9 eval-gate.yml job keys are %r, want exactly %r (D-09/D-10)"
        % (sorted(jobs), want)
    )
print("P9-JOBS-OK")

# --- the fork / Dependabot gate, asserted as the normalized expression ------
# WR-07 / T-48-02 / T-48-18: three independent substring checks pass on an
# expression that still contains every clause but no longer gates anything.
# Appending a top-level `|| true` or wrapping the whole expression in a
# negation keeps all three texts present. The assertion is therefore exact
# equality against the normalized expected literal (D-09), with the Dependabot
# clause kept inside it so the failure message still shows an auditor the full
# approved shape. No substring fallback: equality is the assertion.
gate_if = str((jobs.get("eval-gate") or {}).get("if") or "")
actual = " ".join(gate_if.split())
expected = (
    "(github.event_name != 'pull_request' || "
    "github.event.pull_request.head.repo.full_name == github.repository) "
    "&& github.actor != 'dependabot[bot]'"
)
if actual != expected:
    raise SystemExit(
        "P9 the eval-gate job's if: is %r, want exactly %r "
        "(D-09; WR-07 - a neutralising || true or outer negation must not pass)"
        % (actual, expected)
    )
print("P9-GATE-EXPR-OK")

# --- the visible skip notice (D-10 / P4) ------------------------------------
# A skipped job reports "Success" on GitHub, so the notice job is what keeps a
# skip from being read as a live-gate pass.
#
# WR-02 / T-48-15: the notice must be the exact logical complement of the gate,
# normalized the same way. The old `event == 'pull_request' && (fork ||
# dependabot)` form omitted the region where a Dependabot-attributed push skips
# the gate silently, so the relationship is asserted structurally rather than
# as two independent texts. A non-complement notice, a bare (un-negated) gate,
# or an appended `|| true` all fail this equality.
notice = jobs.get("eval-gate-skip-notice") or {}
notice_if = str(notice.get("if") or "")
notice_actual = " ".join(notice_if.split())
notice_expected = "!(" + expected + ")"
if notice_actual != notice_expected:
    raise SystemExit(
        "P9 the eval-gate-skip-notice job's if: is %r, want the exact complement %r "
        "(D-10; WR-02 - a non-complement notice leaves a skipped gate invisible)"
        % (notice_actual, notice_expected)
    )
if not any("::notice::" in (s.get("run") or "") for s in (notice.get("steps") or [])):
    raise SystemExit(
        "P9 eval-gate-skip-notice has no run: step containing the literal ::notice:: (D-10/P4)"
    )
print("P9-NOTICE-OK")

# --- the live wiring stays intact, and stays minimal ------------------------
# Deleting the three secret mappings would make the workflow green while
# silently ceasing to be a live gate — the opposite of the D-09 intent.
# R2-WR-05 tightens the other direction: the replay step is the ONLY consumer
# (cmd/eval short-circuits --gate-check before any bootstrap), so each mapping
# occurs EXACTLY once and the offline `Gate check` step carries no `env:` key
# at all. A re-widened env block is dead credential surface; a duplicated grant
# is a widened one.
GATE_CHECK = "Gate check"
gate_step = next(
    (s for s in ((jobs.get("eval-gate") or {}).get("steps") or []) if s.get("name") == GATE_CHECK),
    None,
)
if gate_step is None:
    raise SystemExit("P9 eval-gate.yml has no %r step" % GATE_CHECK)
if "env" in gate_step:
    raise SystemExit(
        "P9 the %r step carries an env: key - the offline threshold comparison consumes no secret, so any env block there is dead credential surface (R2-WR-05)"
        % GATE_CHECK
    )

for secret in ("secrets.GLM_API_KEY", "secrets.BGE_M3_URL", "secrets.MILVUS_ADDRESS"):
    n = src.count(secret)
    if n != 1:
        raise SystemExit(
            "P9 eval-gate.yml references %s %d time(s), want exactly 1 - a duplicated grant widens the secret surface, a missing one fakes the live gate (R2-WR-05)"
            % (secret, n)
        )
print("P9-WIRING-OK")

# --- the trigger block, normalized for PyYAML's YAML-1.1 boolean `on:` ------
triggers = doc.get("on", doc.get(True))
if not isinstance(triggers, dict):
    raise SystemExit(
        "P9 eval-gate.yml has no readable trigger block (PyYAML reads the bare `on:` key as True)"
    )
for event in ("pull_request", "push"):
    branches = (triggers.get(event) or {}).get("branches")
    if not isinstance(branches, list) or "main" not in branches:
        raise SystemExit(
            "P9 eval-gate.yml %s.branches is %r, want a list containing 'main'" % (event, branches)
        )
print("P9-TRIGGERS-OK")

# --- one Go version source for the whole directory (D-17 / P6) --------------
targets = []
for base in (wf_dir, actions_dir):
    for dirpath, _dirnames, filenames in os.walk(base):
        for fn in filenames:
            if fn.endswith((".yml", ".yaml")):
                targets.append(os.path.join(dirpath, fn))
if not targets:
    raise SystemExit(
        "P9 scanned zero workflow/action files — a partition that finds nothing must not pass"
    )

for path in sorted(targets):
    text = open(path, encoding="utf-8").read()
    for lineno, line in enumerate(text.splitlines(), 1):
        # `go-version:` (the literal) and `go-version-file:` are different keys;
        # only the former carries a version that can drift from go.mod.
        if "go-version:" in line:
            raise SystemExit(
                "P9 %s:%d carries a `go-version:` literal — a hard build break under GOTOOLCHAIN=local (D-17/P6)"
                % (path, lineno)
            )
    data = yaml.safe_load(text) or {}
    steps = []
    for job in (data.get("jobs") or {}).values():
        steps.extend(job.get("steps") or [])
    steps.extend(((data.get("runs") or {}).get("steps")) or [])
    for step in steps:
        if (step.get("uses") or "").startswith("actions/setup-go"):
            if (step.get("with") or {}).get("go-version-file") != "go.mod":
                raise SystemExit(
                    "P9 %s: a setup-go step does not carry `go-version-file: go.mod` (D-17/P6)"
                    % path
                )

print("P9-GOVERSION-OK")
PY
p9_rc=$?
cat "$WORK/p9.log"
[ "$p9_rc" -ne 0 ] && fail "P9 the eval-gate gate or the Go version source is violated (see the line above)"

ok_partition "P9" "$WORK/p9.log" "P9-JOBS-OK" "P9-GATE-EXPR-OK" "P9-NOTICE-OK" "P9-WIRING-OK" "P9-TRIGGERS-OK" "P9-GOVERSION-OK"

done_partition "P9 eval-gate-gate (exact fork/Dependabot gate expression, complement skip notice, secrets intact exactly once with a secret-free Gate check, one Go version source; per-assertion success tokens P9-JOBS-OK P9-GATE-EXPR-OK P9-NOTICE-OK P9-WIRING-OK P9-TRIGGERS-OK P9-GOVERSION-OK)"

# ---------------------------------------------------------------------------
# P10: sc4-wording — the ROADMAP's SC-4 sentence, resolved through $ROOT
# ---------------------------------------------------------------------------
info "P10: ROADMAP SC-4 wording (D-12/P22)"
begin_partition "P10"

ROADMAP="$ROOT/.planning/ROADMAP.md"
[ -f "$ROADMAP" ] || fail "P10 no ROADMAP at $ROADMAP — a partition with no subject is NOT VERIFIED, never a pass"

# Both phrases are named separately: "the correction is missing" and "the
# correction is wrong" are different failures, and a single boolean would
# collapse them into one indistinguishable red.
# Nothing is asserted about test counts. ROADMAP's Goal line carries a stale
# count that D-12 explicitly leaves alone, and a gate that pinned a count would
# be a future silent-drift failure.
SC4_OLD='既有 hermetic 先例'
SC4_NEW='既有 **live** workflow'

sc4_old_state=absent
grep -qF "$SC4_OLD" "$ROADMAP" && sc4_old_state=present
sc4_new_state=absent
grep -qF "$SC4_NEW" "$ROADMAP" && sc4_new_state=present
info "P10: SC-4 old wording『${SC4_OLD}』= ${sc4_old_state} (want absent) · replacement『${SC4_NEW}』= ${sc4_new_state} (want present)"

if [ "$sc4_old_state" = present ]; then
  grep -nF "$SC4_OLD" "$ROADMAP" | sed 's/^/    /'
  fail "P10 ROADMAP.md still carries the corrected-away SC-4 phrase『${SC4_OLD}』(D-12/P22)"
fi
if [ "$sc4_new_state" != present ]; then
  fail "P10 ROADMAP.md is missing the replacement SC-4 wording『${SC4_NEW}』(D-12/P22)"
fi

# Evidence for the deposit. `grep -q` writes no stdout, so the two presence
# probes above cannot deposit a log themselves. On the asserted-success branch
# only, re-run them without -q: the replacement phrase is present by
# construction, so p10-new.log carries the matched ROADMAP line; the old phrase
# is absent by design, so p10-old.log is legitimately empty. The concatenation
# is non-empty and contains "$SC4_NEW" — the token the probe emitted (R2-WR-03).
grep -F "$SC4_NEW" "$ROADMAP" > "$WORK/p10-new.log"
grep -F "$SC4_OLD" "$ROADMAP" > "$WORK/p10-old.log"
cat "$WORK/p10-new.log" "$WORK/p10-old.log" > "$WORK/p10.log"

ok_partition "P10" "$WORK/p10.log" "$SC4_NEW"

done_partition "P10 sc4-wording (old hermetic-precedent phrase absent, live-workflow wording present; evidence-bound success token $(printf '%s' "$SC4_NEW"))"

# ---------------------------------------------------------------------------
# P11: self-probe — the partition that proves the other ten are not decorative
#
# Execution order is deliberate (registry count LAST):
#   Half 1: the passing banner is pinned to exactly one site inside the
#           RAN-gated branch. Phase 47's first verifier shipped its green
#           banner unconditionally with zero partitions run; a second print
#           site is precisely how that recurs.
#   Half 2: an empty partition is NOT VERIFIED, not PASS. The synthetic
#           fixtures run through scan_trust_surface — the SAME function P8
#           runs — and are required to FAIL.
#   Half 3: the registry is real. EXPECTED_PARTITIONS is recomputed against
#           the done_partition call sites, the begin markers and the success
#           tokens — all four numbers must agree. This half runs LAST because
#           it demands P11's own success token, deposited above.
#
# All three halves run on every invocation. There is no flag, argument or
# environment variable that disables them.
# ---------------------------------------------------------------------------
info "P11: self-probe (banner truth + non-vacuity + registry/token accounting)"
begin_partition "P11"

SELF="$ROOT/$(basename "${BASH_SOURCE[0]}")"

# --- Half 1: exactly one print site, and it is RAN-gated -------------------
# The token is assembled at runtime: spelling it here would make the count see
# the checker's own text, and the count would always be one too high.
BANNER_TOKEN="$(printf '%s %s' 'ALL' 'PASSED')"
RAN_GATE_HEAD='if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]'
GATE_TOKEN="${RAN_GATE_HEAD}; then"

BANNER_SITES=$(grep -cF "$BANNER_TOKEN" "$SELF")
if [ "$BANNER_SITES" -ne 1 ]; then
  fail "P11 the passing banner has ${BANNER_SITES} print sites; exactly one is allowed — a second, unconditional site is how a green banner ships with zero partitions run"
fi

BANNER_LINE=$(grep -nF "$BANNER_TOKEN" "$SELF" | head -1 | cut -d: -f1)
GATE_LINE=$(grep -nF "$GATE_TOKEN" "$SELF" | head -1 | cut -d: -f1)
if [ -z "$GATE_LINE" ]; then
  fail "P11 the RAN-gated branch is missing — the banner is not gated on RAN vs EXPECTED_PARTITIONS"
fi
if [ "$BANNER_LINE" -le "$GATE_LINE" ]; then
  fail "P11 the passing banner is printed at line ${BANNER_LINE}, at or before the RAN gate at line ${GATE_LINE} — it is not inside the gated branch"
fi
# The printed range is materialised BEFORE the grep consumes it. Under the
# header's `set -o pipefail`, a quiet `grep -q` closes the pipe at its first
# match; if the printed range ever outgrows the pipe buffer, sed takes EPIPE
# and the pipeline reports 141 — read as "no else found" in exactly the
# scenario this check exists for, a banner moved further down (48-VERIFICATION
# advisory 1 / WR-01). Capturing the range into BANNER_RANGE and feeding
# printf from it keeps the consumer a non-quiet grep that reads to
# end-of-input, so the detector cannot silently stop detecting as the range
# grows.
BANNER_RANGE="$(sed -n "$((GATE_LINE + 1)),${BANNER_LINE}p" "$SELF")"
if printf '%s\n' "$BANNER_RANGE" | grep -E '^[[:space:]]*else[[:space:]]*$' >/dev/null; then
  fail "P11 the passing banner sits after an else: it is no longer inside the RAN-gated branch"
fi
info "P11: banner print sites ${BANNER_SITES} (line ${BANNER_LINE}, RAN gate line ${GATE_LINE})"

# --- Half 2: an empty partition must fail, not pass vacuously ---------------
# The fixtures live under $WORK and are never written into the repository.
P11_FIX="$WORK/p11-fixtures"
mkdir -p "$P11_FIX/empty-jobs" "$P11_FIX/empty-perms" "$P11_FIX/no-perms" "$P11_FIX/empty-dir" \
  "$P11_FIX/two-files" "$P11_FIX/yaml-ext" "$P11_FIX/lowercase-registry" "$P11_FIX/stray-secret" \
  "$P11_FIX/quoted-paths" "$P11_FIX/space-paths" "$P11_FIX/tojson-secrets" "$P11_FIX/quoted-hash-secret"

cat > "$P11_FIX/empty-jobs/ci.yml" <<'YEOF'
name: fixture-empty-jobs
on:
  push:
    branches: [main]
permissions: {}
jobs: {}
YEOF

cat > "$P11_FIX/empty-perms/ci.yml" <<'YEOF'
name: fixture-empty-perms
on:
  push:
    branches: [main]
permissions: {}
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF

cat > "$P11_FIX/no-perms/ci.yml" <<'YEOF'
name: fixture-no-perms
on:
  push:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF

# The four fixtures below exercise the rules Task 1 hoisted into the per-file
# loop. Each directory contains a valid ci.yml subject plus exactly one
# offending surface, so the ONLY reason it can be rejected is the rule named
# in the fixture. A clean, D-29-compliant ci.yml subject, shared by the
# multi-file fixtures.
clean_ci_fixture() {
  cat > "$1" <<'YEOF'
name: fixture-ci
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF
}

# two-files/ — a SECOND file (not ci.yml) carries a path filter (T-48-03).
clean_ci_fixture "$P11_FIX/two-files/ci.yml"
cat > "$P11_FIX/two-files/release.yml" <<'YEOF'
name: fixture-two-files-release
on:
  push:
    branches: [main]
    paths:
      - "internal/**"
permissions:
  contents: read
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF

# yaml-ext/ — the offending file has the OTHER valid extension (T-48-10 /
# WR-05), and the offending surface is a write permission.
clean_ci_fixture "$P11_FIX/yaml-ext/ci.yml"
cat > "$P11_FIX/yaml-ext/other.yaml" <<'YEOF'
name: fixture-yaml-ext
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      packages: write
    steps:
      - run: "true"
YEOF

# lowercase-registry/ — a publication command in lowercase (IN-02).
cat > "$P11_FIX/lowercase-registry/ci.yml" <<'YEOF'
name: fixture-lowercase-registry
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: docker push ghcr.io/org/img:latest
YEOF

# stray-secret/ — a non-runner secret reference in ci.yml itself (T-48-04).
cat > "$P11_FIX/stray-secret/ci.yml" <<'YEOF'
name: fixture-stray-secret
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "${{ secrets.DEPLOY_KEY }}"
YEOF

# The four fixtures below pin the WR-02 evasion shapes (48-VERIFICATION
# advisory 2 / 48-REVIEW.md WR-02): valid YAML that the retained textual
# rules cannot see but the parsed-document walks reject. Each is a standalone
# ci.yml carrying a valid on: block, permissions: contents: read and a
# non-empty jobs: mapping, so the only reason it can be rejected is the rule
# under test. The two path fixtures use a QUOTED key and a SPACED key
# precisely so the retained textual `paths:` / `paths-ignore:` substring rule
# cannot see them; the two secret fixtures carry no `secrets.NAME` reference
# the retained textual SECRET_REF scan can see — `toJSON(secrets)` has no
# name after the dot, and in quoted-hash-secret the `#` starts a comment for
# the comment-stripping while the parsed document still holds the whole value.

# quoted-paths/ — a QUOTED "paths": key on pull_request (T-48-03, parsed walk).
cat > "$P11_FIX/quoted-paths/ci.yml" <<'YEOF'
name: fixture-quoted-paths
on:
  pull_request:
    "paths":
      - "internal/**"
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF

# space-paths/ — a SPACED `paths :` key on pull_request (T-48-03, parsed walk).
cat > "$P11_FIX/space-paths/ci.yml" <<'YEOF'
name: fixture-space-paths
on:
  pull_request:
    paths :
      - "internal/**"
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "true"
YEOF

# tojson-secrets/ — toJSON(secrets) expands the whole secrets context; there
# is no secrets.NAME after a dot for the textual scan (T-48-04, parsed walk).
cat > "$P11_FIX/tojson-secrets/ci.yml" <<'YEOF'
name: fixture-tojson-secrets
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "${{ toJSON(secrets) }}"
YEOF

# quoted-hash-secret/ — `#` inside a quoted scalar is data, so the textual
# comment strip hides the reference while GitHub still executes it
# (T-48-04, parsed walk).
cat > "$P11_FIX/quoted-hash-secret/ci.yml" <<'YEOF'
name: fixture-quoted-hash-secret
on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: "echo x# ${{ secrets.DEPLOY_KEY }}"
YEOF

P11_FIXTURE_COUNT=0
: > "$WORK/p11-fixtures.log"
# The four WR-02 fixtures sit at the END of the list on purpose: with the
# parsed path walk deleted, two-files is still rejected by the retained
# textual rule and the loop reaches quoted-paths — the first fixture only the
# deleted walk was catching; with the parsed secret walk deleted, quoted-paths
# and space-paths are still rejected by the intact parsed path walk and the
# loop reaches tojson-secrets — the first fixture only the deleted secret walk
# was catching.
for fixture in empty-jobs empty-perms no-perms empty-dir \
               two-files yaml-ext lowercase-registry stray-secret \
               quoted-paths space-paths tojson-secrets quoted-hash-secret; do
  # WR-R5-01, guard 1 — file presence BEFORE the scan. The mkdir list above
  # would recreate a deleted fixture's directory, and the zero-files branch
  # inside scan_trust_surface would then "reject" that EMPTY directory exactly
  # as it rejects empty-dir — the R11/R12 vacuity class. A fixture with no
  # workflow file is a deleted or never-written fixture, never a pin.
  if [ "$fixture" != "empty-dir" ]; then
    fixture_files=$(find "$P11_FIX/$fixture" -maxdepth 1 -type f \( -name '*.yml' -o -name '*.yaml' \) 2>/dev/null | wc -l | tr -d ' ')
    [ "${fixture_files:-0}" -ge 1 ] || fail "P11 fixture '${fixture}' has no workflow file to scan — a deleted or never-written fixture is a vacuous pin, not evidence (WR-R5-01)"
  fi
  # WR-R5-01, guard 2 — every fixture declares the rule that rejects it. The
  # required substrings are the FIRST lines scan_trust_surface actually emits
  # for these fixtures (verified against a live run of the committed tree), so
  # a rejection only counts as evidence when its REASON matches. A fixture
  # added later without a registered reason cannot pass: the default arm
  # refuses. Two registrations are deliberately layer-aware:
  #   yaml-ext — the publication token, not the WRITE_VALUE line scan:
  #     `packages: write` is itself a PUBLICATION_TOKEN and the publication
  #     check runs before the write-scope line scan, so the publication rule
  #     is what the first line names.
  #   two-files — `path-filtered`, the substring BOTH path-filter layers
  #     print (the parsed walk's `filters event … with a 'paths' key` and the
  #     retained textual `contains paths:`): the fixture's unquoted `paths:`
  #     key is caught by either layer, so after a parsed-walk deletion (the
  #     R12 compound drill) it stays attributable to the path-filter RULE and
  #     the loop reaches quoted-paths — the fixture the deleted walk alone
  #     was catching. quoted-paths and space-paths keep the parsed-only
  #     substring: no textual layer can see their shapes, which is exactly
  #     the attribution the R9 drill reads.
  #   stray-secret / quoted-hash-secret — `references secrets.` for the same
  #     reason: the parsed walk prints `parsed value references secrets.X`
  #     and the retained textual scan prints `references secrets.X`, so the
  #     fixture stays attributable to the secret RULE after a walk deletion
  #     (the R10 drill) and the loop reaches tojson-secrets — the fixture
  #     only the deleted secret walk was catching.
  case "$fixture" in
    empty-jobs)                      want='empty or absent jobs';             class_token='P11-VACUITY-OK' ;;
    empty-perms)                     want='empty permissions';                class_token='P11-VACUITY-OK' ;;
    no-perms)                        want='declares no permissions';          class_token='P11-VACUITY-OK' ;;
    empty-dir)                       want='zero *.yml/*.yaml files';          class_token='P11-VACUITY-OK' ;;
    two-files)                       want='path-filtered';                    class_token='P11-PATHFILTER-OK' ;;
    quoted-paths|space-paths)        want="with a 'paths' key";               class_token='P11-PATHFILTER-OK' ;;
    yaml-ext)                        want='registry/publication token';       class_token='P11-PERMSWRITE-OK' ;;
    lowercase-registry)              want='registry/publication token';       class_token='P11-PUBLICATION-OK' ;;
    stray-secret|quoted-hash-secret) want='references secrets.';              class_token='P11-SECRET-OK' ;;
    tojson-secrets)                  want='toJSON(secrets';                   class_token='P11-SECRET-OK' ;;
    *) fail "P11 fixture '${fixture}' has no registered rejection reason — every fixture must declare the rule that rejects it (WR-R5-01)" ;;
  esac
  scan_trust_surface "$P11_FIX/$fixture"
  p11_fixture_rc=$?
  if [ "$p11_fixture_rc" -eq 0 ]; then
    cat "$WORK/trust-surface.log"
    fail "P11 verifier is vacuous on an empty partition: scan_trust_surface returned success on the '${fixture}' fixture"
  fi
  # The reason is rendered through sed rather than interpolated into fail():
  # the probe's message quotes a NATIVE Windows path whose backslashes echo -e
  # would read as escape sequences (\t, \e, \c …) — which mangles the evidence
  # and can swallow the newline. Output that is never re-interpreted cannot be
  # mangled. The first line is captured into a variable first, so neither the
  # mismatch renderer nor the evidence line runs through a head|sed pipe.
  first_line="$(head -1 "$WORK/trust-surface.log")"
  if ! printf '%s\n' "$first_line" | grep -qF -- "$want"; then
    printf '%s\n' "$first_line" | sed 's/^/       /'
    fail "P11 fixture '${fixture}' rejected for the wrong reason: wanted '${want}' (WR-R5-01)"
  fi
  # WR-R5-01, guard 3 — the zero-files reason belongs to empty-dir alone. For
  # any other fixture it means the fixture vanished and the scan found
  # nothing; name the vacuity class even if a future fixture's registered
  # reason ever accidentally contained the zero-files text.
  if [ "$fixture" != "empty-dir" ] && printf '%s\n' "$first_line" | grep -qF 'zero *.yml/*.yaml files'; then
    fail "P11 fixture '${fixture}' was rejected by vacuity (zero files) — a deleted fixture must not count as a pin (WR-R5-01)"
  fi
  P11_FIXTURE_COUNT=$((P11_FIXTURE_COUNT + 1))
  echo "fixture '${fixture}' rejected for '${want}' (rc=${p11_fixture_rc})" >> "$WORK/p11-fixtures.log"
  printf '%s\n' "$class_token" >> "$WORK/p11-fixtures.log"
  echo "  P11: fixture '${fixture}' rejected for '${want}' (rc=${p11_fixture_rc}):"
  printf '%s\n' "$first_line" | sed 's/^/       /'
done

# The fixture loop is a probe too: twelve rejections are its success output. A
# loop that never ran (or was neutered) leaves the count and the log short, and
# the deposit refuses (R2-WR-03).
[ "$P11_FIXTURE_COUNT" -eq 12 ] || fail "P11 the fixture loop ran ${P11_FIXTURE_COUNT} fixture(s), want 12 — a neutralised or deleted loop cannot deposit PASS (R2-WR-03)"

# P11's own success token deposits right after its last decisive assertion —
# BEFORE the registry count below, which demands that token to exist. The
# reverse order would be a self-failing gate.
#
# WR-R5-01: one deposit token per fixture CLASS, not one all-matching
# `rejected`. A single token that any rejection satisfied let a fixture (or a
# rule + its fixtures) vanish while its siblings' evidence still deposited
# PASS. Every class must have deposited for P11 to pass.
ok_partition "P11" "$WORK/p11-fixtures.log" "P11-VACUITY-OK" "P11-PATHFILTER-OK" "P11-PERMSWRITE-OK" "P11-PUBLICATION-OK" "P11-SECRET-OK"

# --- Half 3 (LAST): the registry, the call sites, the markers and the tokens
# agree -----------------------------------------------------------------------
# All three counts are observable effects, and each catches a different lie:
#   CALL_SITES  — source text (a call site can survive a commented-out body)
#   BEGIN_COUNT — one marker per body that STARTED
#   OK_COUNT    — one token per probe that RAN to a passed exit-code assertion
# The count is deliberately the partition's LAST act: by now every other
# partition has deposited both of its markers, so a four-way agreement here is
# a statement about the whole run, not about what has happened so far.
CALL_SITES=$(grep -cE '^done_partition "' "$SELF")
BEGIN_COUNT=$(find "$WORK/ran" -maxdepth 1 -name '*.begin' | wc -l | tr -d ' ')
OK_COUNT=$(find "$WORK/ran" -maxdepth 1 -name '*.ok' | wc -l | tr -d ' ')
if [ "$CALL_SITES" -ne "$EXPECTED_PARTITIONS" ] || [ "$BEGIN_COUNT" -ne "$EXPECTED_PARTITIONS" ] || [ "$OK_COUNT" -ne "$EXPECTED_PARTITIONS" ]; then
  fail "P11 the registry, the done_partition call sites, the begin markers and the success tokens disagree: registry=${EXPECTED_PARTITIONS}, call sites=${CALL_SITES}, begin markers=${BEGIN_COUNT}, success tokens=${OK_COUNT} — a partition whose body did not run, whose probe did not run to success, or a call site added without a registry entry, fails here"
fi
info "P11: registry ${CALL_SITES}/${EXPECTED_PARTITIONS}, begin markers ${BEGIN_COUNT}/${EXPECTED_PARTITIONS}, success tokens ${OK_COUNT}/${EXPECTED_PARTITIONS}"

done_partition "P11 self-probe (registry == call sites == begin markers == evidence-bound success tokens, one RAN-gated banner site, 12 fixtures rejected for their registered reasons: 4 vacuity + 8 per-file; per-class tokens P11-VACUITY-OK P11-PATHFILTER-OK P11-PERMSWRITE-OK P11-PUBLICATION-OK P11-SECRET-OK)"

# ---------------------------------------------------------------------------
# Closing banner — gated on RAN vs EXPECTED_PARTITIONS, never on a literal.
# ---------------------------------------------------------------------------
echo ""
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
  echo -e "${GREEN}=== Phase 48 Gate: ALL PASSED ===${NC}"
else
  echo -e "${YELLOW}=== Phase 48 Gate: NOT VERIFIED ===${NC}"
  echo -e "${YELLOW}=== ${RAN} of ${EXPECTED_PARTITIONS} partitions ran. This gate did NOT pass. ===${NC}"
  for p in P1 P2 P3 P4 P5 P6 P7 P8 P9 P10 P11; do
    seen=0
    for r in ${RAN_LIST[@]+"${RAN_LIST[@]}"}; do
      case "$r" in "$p "*) seen=1 ;; esac
    done
    [ "$seen" -eq 1 ] || echo -e "${YELLOW}=== did not verify: ${p} ===${NC}"
  done
  exit 1
fi

echo ""
echo "Partitions verified:"
echo "  P1 workflow-parse — every .github/workflows/*.yml / *.yaml + the composite action parse; trigger block read; zero pull_request_target; per-assertion success tokens P1-PARSED / P1-COMPOSITE-OK / P1-NO-PRT"
echo "  P2 go-line        — the 5 CI command fragments + determinism materialised-list shape (list file, grep -qx on the file, no pipe after -list), CGO_ENABLED=1 and nothing else, verify job ends at the gate; per-assertion success tokens P2-CMDS-OK / P2-DET-OK / P2-SHUFFLE-OK / P2-HERMETIC-OK"
echo "  P3 lint-line      — golangci-lint-action pinned + blocking + only-new-issues, permissions map exact, verify job if-free with module-proxy install; per-assertion success tokens P3-STEP-IF-OK / P3-BLOCKING-OK / P3-LINT-OK / P3-VERIFYJOB-OK"
echo "  P4 lint-teeth     — config verify exit 0, sentinel A reported as errcheck, sentinel D reported as revive, sentinels B and C silent with '0 issues'; per-assertion success tokens P4-A-ERRCHECK / P4-B-QUIET / P4-C-QUIET / P4-D-REVIVE"
echo "  P5 composite      — three pinned majors in one file, zero checkout, every job checks out first; evidence-bound success token"
echo "  P6 fe-line        — npm ci, npx tsc --noEmit, npx vitest run under web/, zero count literals, rejects both job- and step-level if:; per-assertion success tokens P6-JOB-IF-OK / P6-STEP-IF-OK / P6-CMDS-OK / P6-COUNTS-OK"
echo "  P7 docker-line    — both Dockerfiles, push: false twice, per-image cache scopes, zero publish surface, rejects both job- and step-level if:; per-assertion success tokens P7-JOB-IF-OK / P7-STEP-IF-OK / P7-BUILDS-OK / P7-PUSH-OK / P7-NO-LOAD-OK / P7-CACHE-OK"
echo "  P8 trust-surface  — per file (*.yml + *.yaml): read-only permissions, no registry/path/secret surface; evidence-bound success token"
echo "  P9 eval-gate-gate — exact fork/Dependabot gate expression, complement ::notice:: skip, secrets intact exactly once, Gate check carries no env, one Go version source; per-assertion success tokens P9-JOBS-OK / P9-GATE-EXPR-OK / P9-NOTICE-OK / P9-WIRING-OK / P9-TRIGGERS-OK / P9-GOVERSION-OK"
echo "  P10 sc4-wording   — ROADMAP SC-4: old hermetic-precedent phrase absent, live-workflow wording present; evidence-bound success token"
echo "  P11 self-probe    — registry == call sites == begin markers == evidence-bound success tokens, one RAN-gated banner site, empty partitions and per-file mutations proven to fail"
