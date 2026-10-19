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
#   P1  workflow-parse  — every .github/workflows/*.yml plus the composite
#                         action parses under PyYAML; the trigger block is
#                         reachable; no workflow targets pull_request_target
#   P2  go-line         — the go job carries the exact command strings the
#                         local gate runs, is hermetic (CGO_ENABLED only, no
#                         live-service env), and captures its shuffle seed
#   P3  lint-line       — lint is pinned, blocking, and holds the permission
#                         its changed-lines mode needs to not silently widen
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
#                         its visible skip notice, its intact secret wiring,
#                         its un-widened trigger, and the directory-wide
#                         single Go version source
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

# Partition registry. The closing banner compares RAN against
# EXPECTED_PARTITIONS; a shortfall exits non-zero.
EXPECTED_PARTITIONS=9
RAN=0
RAN_LIST=()
done_partition() {
  RAN=$((RAN + 1))
  RAN_LIST+=("$1")
  pass "$1"
}

# The linter version this gate is written against. A different binary is an
# unverified gate, not a passing one.
LINT_VERSION="2.12.2"

# Two tokens this gate asserts the ABSENCE of, in ci.yml. They are assembled at
# runtime rather than written out: a gate script that spells a forbidden CI
# keyword in its own source (even in a comment or a Python probe) makes the
# repo-wide grep over this file meaningless — the grep can no longer tell the
# workflow's text from the checker's text. Read -r splits on whitespace, so the
# second field must be empty at the end.
SOFT_KEY="$(printf '%s%s' "continue" "-on-error")"
WHOLE_FILES_FLAG="--whole-files"

echo "=== Phase 48 Verification (static CI gate self-proof) ==="

# ---------------------------------------------------------------------------
# P1: workflow-parse
# ---------------------------------------------------------------------------
info "P1: workflow parse + trigger safety"
WF_DIR="$ROOT/.github/workflows"
COMPOSITE="$ROOT/.github/actions/setup/action.yml"

[ -d "$WF_DIR" ] || fail "P1 no workflow directory at $WF_DIR — a partition that finds zero files must not pass"

# Enumerate to a list file first: a glob evaluated inline is indistinguishable
# from an empty result, and an empty result must fail rather than pass silently.
find "$WF_DIR" -maxdepth 1 -type f -name '*.yml' -print | sort > "$WORK/p1-list.txt"
WF_COUNT=$(wc -l < "$WORK/p1-list.txt" | tr -d ' ')
[ "$WF_COUNT" -ge 1 ] || fail "P1 matched zero *.yml files under $WF_DIR — a partition that finds zero files must not pass"

# PyYAML implements YAML 1.1, where the bare key `on:` loads as the boolean
# True. A parser that reaches only for d["on"] sees a workflow with no trigger
# block at all — which is how a "parsed fine" check can be true of a file whose
# trigger nobody has looked at. The trigger block is therefore read back
# explicitly and required to be present on every workflow.
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
' "$(native "$f")" || fail "P1 YAML parse / trigger read failed: $f"
done < "$WORK/p1-list.txt"

# The composite action is parsed too, but it legitimately has no trigger block
# — it is only ever reached through a caller workflow.
[ -f "$COMPOSITE" ] || fail "P1 composite toolchain action missing: $COMPOSITE"
python -c '
import sys, yaml
d = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
if (d.get("runs") or {}).get("using") != "composite":
    raise SystemExit("composite action is not `runs.using: composite`")
' "$(native "$COMPOSITE")" || fail "P1 composite action parse failed: $COMPOSITE"

# D-11: pull_request_target executes fork-controlled workflow code against the
# base repository's secrets and a mutating token. Report file:line, never just
# the file name.
if grep -rn 'pull_request_target' "$WF_DIR" > "$WORK/p1-prt.txt" 2>/dev/null; then
  sed 's/^/    /' "$WORK/p1-prt.txt"
  fail "P1 pull_request_target found (D-11) — listed above with file:line"
fi

done_partition "P1 workflow-parse ($WF_COUNT workflow(s) + composite parse, zero pull_request_target)"

# ---------------------------------------------------------------------------
# P2 + P3: the go job's structural contract, decided by one Python probe so the
#          two partitions cannot disagree about what the file says.
# ---------------------------------------------------------------------------
info "P2/P3: go line + lint line structure"
CI="$ROOT/.github/workflows/ci.yml"
[ -f "$CI" ] || fail "P2 ci.yml missing: $CI"

python - "$(native "$CI")" "$SOFT_KEY" "$WHOLE_FILES_FLAG" <<'PY' > "$WORK/p23.log" 2>&1
import sys, yaml

path, soft_key, whole_flag = sys.argv[1], sys.argv[2], sys.argv[3]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)

jobs = doc.get("jobs") or {}
if "go" not in jobs:
    raise SystemExit("P2: ci.yml has no `go` job")
j = jobs["go"]
steps = j.get("steps") or []
runs = "\n".join(s.get("run", "") for s in steps)

# --- P2 --------------------------------------------------------------------
for frag in (
    "go build ./...",
    "go vet ./internal/ontology/... ./internal/server/... ./internal/knowledge/...",
    "go test ./... -count=1",
    "-run '^TestDeriveEdgesDeterministic$' -count=100",
):
    if frag not in runs:
        raise SystemExit("P2: go job is missing the command fragment: %s" % frag)

shuffled = [s for s in steps if "-shuffle=on" in (s.get("run") or "")]
if not shuffled:
    raise SystemExit("P2: go job has no -shuffle=on step")
if "-v" not in shuffled[0]["run"].split("-shuffle=on")[1]:
    raise SystemExit("P2: the -shuffle=on step has no -v, so the seed never reaches the log (D-19)")

env = j.get("env") or {}
if str(env.get("CGO_ENABLED")) != "1":
    raise SystemExit("P2: go job CGO_ENABLED is %r, want \"1\"" % (env.get("CGO_ENABLED"),))
extra = [k for k in env if k != "CGO_ENABLED"]
if extra:
    raise SystemExit("P2: go job declares env beyond CGO_ENABLED: %s" % ", ".join(sorted(extra)))

# D-16: naming any of these flips cmd/spike-* off its t.Skip onto the live
# path, which is exactly what makes the go line non-hermetic.
for name in ("GLM_API_KEY", "BGE_M3_URL", "MILVUS_ADDRESS"):
    if name in src:
        raise SystemExit("P2: ci.yml references live-service env %s" % name)

# --- P3 --------------------------------------------------------------------
if soft_key in src:
    raise SystemExit("P3: ci.yml contains %s — the gate is non-blocking" % soft_key)
if "if" in j:
    raise SystemExit("P3: the go job carries a job-level `if:` — the gate is skippable")
if whole_flag in src:
    raise SystemExit("P3: ci.yml contains %s (D-04)" % whole_flag)

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

print("STRUCT-OK")
PY
p23_rc=$?
cat "$WORK/p23.log"
[ "$p23_rc" -ne 0 ] && fail "P2/P3: the go job's structural contract is violated (see the line above)"

done_partition "P2 go-line (5 command fragments, CGO_ENABLED=1 only, shuffle seed under -v)"
done_partition "P3 lint-line (pinned v2.12.2, blocking, permissions map exact)"

# ---------------------------------------------------------------------------
# P4: lint-teeth
# ---------------------------------------------------------------------------
info "P4: lint has teeth (config verify + sentinels)"

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
if ! grep -q 'errcheck' "$WORK/p4-sentinel-a.log"; then
  tail -20 "$WORK/p4-sentinel-a.log"
  fail "P4 sentinel A: the run failed but did not name errcheck — a different cause is failing, not the gate"
fi
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
pass "P4 sentinel C: an ignored parameter stays quiet (revive rule disabled)"

done_partition "P4 lint-teeth (config verify + sentinel A fires, sentinels B and C stay quiet)"

# ---------------------------------------------------------------------------
# P5: composite
# ---------------------------------------------------------------------------
info "P5: composite resolution order"

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

done_partition "P5 composite (three pinned majors, no checkout, every job checks out before setup)"

# ---------------------------------------------------------------------------
# P6: fe-line
# ---------------------------------------------------------------------------
info "P6: fe line"

python - "$(native "$CI")" <<'PY' > "$WORK/p6.log" 2>&1
import sys, yaml

path = sys.argv[1]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)
jobs = doc.get("jobs") or {}

if "fe" not in jobs:
    raise SystemExit("P6: ci.yml has no `fe` job (CICD-02)")

steps = jobs["fe"].get("steps") or []
runs = {(s.get("run") or "").strip(): s.get("working-directory") for s in steps if "run" in s}
for cmd in ("npm ci", "npx tsc --noEmit", "npx vitest run"):
    if cmd not in runs:
        raise SystemExit("P6: fe job is missing the step: %s" % cmd)
    if runs[cmd] != "web":
        raise SystemExit("P6: fe step %r has working-directory %r, want 'web'" % (cmd, runs[cmd]))

# A test count in a gate is a future silent-drift failure: the suite grows,
# the literal does not, and the gate starts asserting a stale fact.
for literal in ("666", "689", "84 "):
    if literal in src:
        raise SystemExit("P6: ci.yml hard-codes an FE test-count literal: %r" % literal)

print("FE-LINE-OK")
PY
p6_rc=$?
cat "$WORK/p6.log"
[ "$p6_rc" -ne 0 ] && fail "P6 the fe line is violated (see the line above)"

done_partition "P6 fe-line (npm ci, tsc --noEmit, vitest run under web/, no count literals)"

# ---------------------------------------------------------------------------
# P7: docker-line
# ---------------------------------------------------------------------------
info "P7: docker line"

python - "$(native "$CI")" <<'PY' > "$WORK/p7.log" 2>&1
import sys, yaml

path = sys.argv[1]
src = open(path, encoding="utf-8").read()
doc = yaml.safe_load(src)
jobs = doc.get("jobs") or {}

if "docker" not in jobs:
    raise SystemExit("P7: ci.yml has no `docker` job (CICD-03)")

steps = jobs["docker"].get("steps") or []
builds = [s for s in steps if (s.get("uses") or "").startswith("docker/build-push-action")]
if len(builds) != 2:
    raise SystemExit("P7: expected exactly 2 docker build steps, found %d" % len(builds))

files = [(b.get("with") or {}).get("file") for b in builds]
if files != ["Dockerfile.backend", "Dockerfile.frontend"]:
    raise SystemExit("P7: build step Dockerfiles are %r, want backend then frontend" % (files,))

for b in builds:
    with_ = b.get("with") or {}
    if with_.get("context") != ".":
        raise SystemExit("P7: build step %r context is %r, want '.'" % (b.get("name"), with_.get("context")))
    # D-24: CI never publishes an image and never produces a local one.
    if with_.get("push") is not False:
        raise SystemExit("P7: build step %r does not carry push: false" % (b.get("name"),))

for bad in ("load: true", "docker compose build"):
    if bad in src:
        raise SystemExit("P7: ci.yml contains %r (D-24/D-25)" % bad)

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

print("DOCKER-LINE-OK")
PY
p7_rc=$?
cat "$WORK/p7.log"
[ "$p7_rc" -ne 0 ] && fail "P7 the docker line is violated (see the line above)"

done_partition "P7 docker-line (both Dockerfiles, push: false, per-image cache scopes)"

# ---------------------------------------------------------------------------
# P8: trust-surface — the whole workflow directory
# ---------------------------------------------------------------------------
info "P8: trust surface (permissions / registry / path filters / secrets)"

find "$WF_DIR" -maxdepth 1 -type f -name '*.yml' -print | sort > "$WORK/p8-list.txt"
P8_COUNT=$(wc -l < "$WORK/p8-list.txt" | tr -d ' ')
[ "$P8_COUNT" -ge 1 ] || fail "P8 matched zero *.yml files under $WF_DIR — a partition that finds zero files must not pass"

p8_args=()
while IFS= read -r f; do p8_args+=("$(native "$f")"); done < "$WORK/p8-list.txt"

python - "$(native "$CI")" "${p8_args[@]}" <<'PY' > "$WORK/p8.log" 2>&1
import sys, re, yaml

ci_path = sys.argv[1]
paths = sys.argv[2:]
if not paths:
    raise SystemExit("P8: zero workflow files to scan — a partition that finds nothing must not pass")

for p in paths:
    src = open(p, encoding="utf-8").read()

    # D-26 / D-29: read-only ceiling. The raw scan also catches a write scope
    # in a shape PyYAML might normalize away.
    if "write" in src:
        raise SystemExit("P8: %s contains a write scope (D-26/D-29)" % p)

    doc = yaml.safe_load(src)
    maps = []
    if isinstance(doc.get("permissions"), dict):
        maps.append(("workflow-level", doc["permissions"]))
    for name, job in (doc.get("jobs") or {}).items():
        if isinstance(job.get("permissions"), dict):
            maps.append(("job %r" % name, job["permissions"]))
    for where, mapping in maps:
        for scope, value in mapping.items():
            if value not in ("read", "none"):
                raise SystemExit(
                    "P8: %s in %s grants %r=%r; only read/none are allowed (D-26/D-29)"
                    % (p, where, scope, value)
                )

    # T-48-10: no image-publication capability anywhere in the directory.
    for bad in ("docker/login-action", "registry:", "GHCR", "DOCKERHUB", "packages: write"):
        if bad in src:
            raise SystemExit("P8: %s contains forbidden registry surface: %s (T-48-10)" % (p, bad))

ci_src = open(ci_path, encoding="utf-8").read()

# T-48-03: a path-filtered required check sits Pending forever on a PR that
# touches no matched path. Per-line skipping must use a job-level `if:`.
for bad in ("paths:", "paths-ignore:"):
    if bad in ci_src:
        raise SystemExit("P8: ci.yml contains %s — a path-filtered required check sits Pending (T-48-03)" % bad)

# T-48-04 / D-14: a public fork PR must never need or see a secret. The only
# permitted reference is the runner-provided token.
for name in re.findall(r"secrets\.([A-Za-z_][A-Za-z0-9_]*)", ci_src):
    if name != "GITHUB_TOKEN":
        raise SystemExit("P8: ci.yml references secrets.%s; only secrets.GITHUB_TOKEN is allowed (T-48-04)" % name)

print("TRUST-SURFACE-OK (%d workflow file(s))" % len(paths))
PY
p8_rc=$?
cat "$WORK/p8.log"
[ "$p8_rc" -ne 0 ] && fail "P8 the trust surface is violated (see the line above)"

done_partition "P8 trust-surface (${P8_COUNT} workflow(s): read-only permissions, no registry/path/secret surface)"

# ---------------------------------------------------------------------------
# P9: eval-gate-gate — the fork boundary, the visible skip, and the Go version
# ---------------------------------------------------------------------------
info "P9: eval-gate gate + the directory-wide Go version source"

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

# --- the fork / Dependabot gate, asserted clause by clause (D-09 / P3) ------
# A gate that loses any ONE clause re-opens the fork hole, so a substring
# search for the whole expression would pass on two clauses out of three.
gate_if = str((jobs.get("eval-gate") or {}).get("if") or "")
for clause in (
    "github.event_name != 'pull_request'",
    "head.repo.full_name == github.repository",
    "github.actor != 'dependabot[bot]'",
):
    if clause not in gate_if:
        raise SystemExit(
            "P9 the `eval-gate` job's if: is missing the clause: %s (D-09/P3)" % clause
        )

# --- the visible skip notice (D-10 / P4) ------------------------------------
# A skipped job reports "Success" on GitHub, so the notice job is what keeps a
# skip from being read as a live-gate pass.
notice = jobs.get("eval-gate-skip-notice") or {}
notice_if = str(notice.get("if") or "")
for clause in ("github.event_name == 'pull_request'", "dependabot[bot]"):
    if clause not in notice_if:
        raise SystemExit(
            "P9 the `eval-gate-skip-notice` job's if: is missing: %s (D-10/P4)" % clause
        )
if not any("::notice::" in (s.get("run") or "") for s in (notice.get("steps") or [])):
    raise SystemExit(
        "P9 eval-gate-skip-notice has no run: step containing the literal ::notice:: (D-10/P4)"
    )

# --- the live wiring stays intact -------------------------------------------
# Deleting the three secret mappings would make the workflow green while
# silently ceasing to be a live gate — the opposite of the D-09 intent.
for secret in ("secrets.GLM_API_KEY", "secrets.BGE_M3_URL", "secrets.MILVUS_ADDRESS"):
    if secret not in src:
        raise SystemExit(
            "P9 eval-gate.yml no longer references %s — removing the wiring turns a live gate into a fake one"
            % secret
        )

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

print(
    "EVALGATE-OK (%d jobs, 3-clause gate, notice present) + GOVERSION-OK (%d file(s))"
    % (len(jobs), len(targets))
)
PY
p9_rc=$?
cat "$WORK/p9.log"
[ "$p9_rc" -ne 0 ] && fail "P9 the eval-gate gate or the Go version source is violated (see the line above)"

done_partition "P9 eval-gate-gate (3-clause fork gate, visible skip notice, secrets intact, one Go version source)"

# ---------------------------------------------------------------------------
# Closing banner — gated on RAN vs EXPECTED_PARTITIONS, never on a literal.
# ---------------------------------------------------------------------------
echo ""
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
  echo -e "${GREEN}=== Phase 48 Gate: ALL PASSED ===${NC}"
else
  echo -e "${YELLOW}=== Phase 48 Gate: NOT VERIFIED ===${NC}"
  echo -e "${YELLOW}=== ${RAN} of ${EXPECTED_PARTITIONS} partitions ran. This gate did NOT pass. ===${NC}"
  for p in P1 P2 P3 P4 P5 P6 P7 P8 P9; do
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
echo "  P1 workflow-parse — every .github/workflows/*.yml + the composite action parse; trigger block read; zero pull_request_target"
echo "  P2 go-line        — the 5 CI command fragments, CGO_ENABLED=1 and nothing else, shuffle seed recoverable under -v"
echo "  P3 lint-line      — golangci-lint-action pinned + blocking + only-new-issues, permissions map exact"
echo "  P4 lint-teeth     — config verify exit 0, sentinel A reported as errcheck, sentinels B and C silent"
echo "  P5 composite      — three pinned majors in one file, zero checkout, every job checks out first"
echo "  P6 fe-line        — npm ci, npx tsc --noEmit, npx vitest run under web/, zero count literals"
echo "  P7 docker-line    — both Dockerfiles, push: false twice, per-image cache scopes, zero publish surface"
echo "  P8 trust-surface  — read-only permissions, zero registry/path-filter/secret references (directory-wide)"
echo "  P9 eval-gate-gate — 3-clause fork/Dependabot gate, visible ::notice:: skip, secrets intact, one Go version source"
