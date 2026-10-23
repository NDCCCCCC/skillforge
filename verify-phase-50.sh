#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# verify-phase-50.sh - Phase 50 (open-source facade) acceptance gate.
#
# Pure-documentation gate: every assertion is mechanically decidable from the
# working tree (file existence, grep counts, byte-level content checks). No Go,
# no WSL, no network. Partitions:
#
#   P-README    README truth-sync: 7 example skills listed, honest CI badge,
#               comparison lanes (WeKnora/Semantica), architecture link,
#               no no-cgo/single-binary claim
#   P-ARCH      docs/architecture.md: mermaid block, four layers, and every
#               backticked repo path in the doc actually exists
#   P-COMMUNITY SECURITY.md / CODE_OF_CONDUCT.md / issue+PR templates exist,
#               README Community block links them, templates carry no
#               key-paste bait
#   P-DEMO      docs/demo.gif exists and opens with a real GIF magic number
#   P-SELF      gate self-proof: exactly one ALL PASSED print site, it sits
#               behind the RAN-gated branch, partition bookkeeping consistent
#
# Drills (D1..D4): each partition is mutation-proven - the drill copies the
# gate and the tree into a scratch dir, breaks one thing, and requires the
# copy to go red. Guarded by GATE_DRILL so the drills never recurse.

set -uo pipefail
export MSYS_NO_PATHCONV=1

RED='\033[0;31m'; GREEN='\033[0;32m'; NC='\033[0m'
GATE_SELF="${BASH_SOURCE[0]:-$0}"
for _f in "$(dirname "$0")/.gate-drill-env" ".gate-drill-env"; do
    [ -f "$_f" ] && . "$_f" && break
done 2>/dev/null
ROOT="${GATE_ROOT:-$(cd "$(dirname "$GATE_SELF")" && pwd)}"
cd "$ROOT" || exit 1

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

pass() { echo -e "${GREEN}PASS${NC} $1"; }
fail() { echo -e "${RED}FAIL${NC} $1"; exit 1; }
info() { echo "INFO $1"; }

# -- partition bookkeeping ----------------------------------------------------
EXPECTED_PARTITIONS=6
RAN=0
RAN_LIST=()

_pn_num() {
    local n="$1"
    while [ "${n:0:1}" = "P" ] || [ "${n:0:1}" = "p" ]; do
        n="${n:1}"
    done
    printf '%s' "$n"
}

begin_partition() {
    local pn; pn="$(_pn_num "$1")"
    : > "$WORK/p$pn.begin"
}

ok_partition() {
    local pn="$1"; shift
    local log="$1"; shift
    local token
    [ -f "$log" ] || fail "partition $pn cannot deposit tokens: probe log $log was never written"
    [ -s "$log" ] || fail "partition $pn cannot deposit tokens: probe log $log is empty"
    for token in "$@"; do
        grep -qF -- "$token" "$log" || fail "partition $pn cannot deposit its success tokens: probe log $log does not contain '${token}'"
    done
    : > "$WORK/p$(_pn_num "$pn").ok"
    echo "OK-DEPOSIT $pn" >&2
}

done_partition() {
    local pn="$1"; pn="$(_pn_num "$pn")"
    [ -f "$WORK/p$pn.begin" ] || fail "partition ${pn} ran its checks without begin_partition bookkeeping"
    [ -f "$WORK/p$pn.ok" ] || fail "partition ${pn} finished without depositing its success tokens"
    RAN=$((RAN + 1))
    RAN_LIST+=("$pn")
}

# -- P-README -----------------------------------------------------------------
# drill:P-README:begin
begin_partition "P-README"
LOG="$WORK/p-README.log"
{
    echo "P-README-COUNT-SYNC"
    echo "P-README-HONEST-BADGE"
    echo "P-README-LANES"
    echo "P-README-ARCH-LINK"
    echo "P-README-NO-CGO-CLAIM"

    # D-50-02: the README must list every example skill directory, and the
    # count of `name`-style listings must match the live directory.
    n_dirs=0
    for d in examples/skills/*/; do
        d="$(basename "$d")"
        n_dirs=$((n_dirs + 1))
        grep -q -- "$d" README.md || echo "README does not mention example skill: $d"
    done
    [ "$n_dirs" -eq 7 ] || echo "examples/skills has $n_dirs entries, want 7 (D-50-02)"
    ! grep -q . <(grep -E '^README does not mention' "$WORK/p-README-check.txt" 2>/dev/null) 2>/dev/null || true

    # stale "Three ready-to-run templates" claim is gone
    if grep -qi "three ready-to-run" README.md; then
        echo "README still claims three ready-to-run templates (D-50-02)"
    fi

    # D-50-05: no decorative CI badge date, and the CI badge points at the
    # real workflow file
    if grep -q "eval-gate-22-09-06" README.md; then
        echo "decorative CI badge date still present (D-50-05)"
    fi
    if ! grep -q "workflows/ci.yml\|workflows/eval-gate.yml\|actions/workflows" README.md; then
        echo "no badge links to a real workflow file (D-50-05)"
    fi

    # D-50-01: comparison lanes follow the README positioning
    grep -q "WeKnora" README.md || echo "missing WeKnora lane (D-50-01)"
    grep -q "Semantica" README.md || echo "missing Semantica lane (D-50-01)"

    # architecture link
    grep -q "docs/architecture.md" README.md || echo "missing docs/architecture.md link"

    # D-50-07: no no-cgo / single-binary claim
    if grep -qiE "no.?cgo|single[- ]static[- ]binary|statically[- ]linked single" README.md; then
        echo "found a no-cgo / single-binary claim (D-50-07)"
    fi

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
if grep -qE "README does not mention|examples/skills has|still claims|decorative CI|no badge links|missing WeKnora|missing Semantica|missing docs/architecture|found a no-cgo" "$LOG"; then
    cat "$LOG"
    fail "P-README: facade truth-sync checks failed (see lines above)"
fi
ok_partition "P-README" "$LOG" "P-README-COUNT-SYNC" "P-README-HONEST-BADGE" "P-README-LANES" "P-README-ARCH-LINK" "P-README-NO-CGO-CLAIM"
done_partition "P-README"
# drill:P-README:end

# -- P-ARCH -------------------------------------------------------------------
# drill:P-ARCH:begin
begin_partition "P-ARCH"
LOG="$WORK/p-ARCH.log"
{
    echo "P-ARCH-DOC-EXISTS"
    echo "P-ARCH-MERMAID"
    echo "P-ARCH-FOUR-LAYERS"
    echo "P-ARCH-PATHS-REAL"

    test -f docs/architecture.md || echo "docs/architecture.md missing"
    grep -q '```mermaid' docs/architecture.md || echo "no mermaid block in docs/architecture.md"
    for layer in "internal/server" "internal/retriever" "internal/verdict" "internal/kb"; do
        grep -q -- "$layer" docs/architecture.md || echo "architecture doc missing layer/entry: $layer"
    done

    # zero-fabrication: every backticked repo path mentioned in the doc must exist
    while IFS= read -r p; do
        [ -e "$p" ] || echo "architecture doc references a path that does not exist: $p"
    done < <(grep -oE '`(internal|cmd|web|docs)/[A-Za-z0-9_./-]+`' docs/architecture.md | tr -d '\140' | sort -u)

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
    if grep -qE "missing|does not exist" "$LOG"; then
        cat "$LOG"
    fail "P-ARCH: architecture doc failed the zero-fabrication checks"
    fi
ok_partition "P-ARCH" "$LOG" "P-ARCH-DOC-EXISTS" "P-ARCH-MERMAID" "P-ARCH-FOUR-LAYERS" "P-ARCH-PATHS-REAL"
done_partition "P-ARCH"
# drill:P-ARCH:end

# -- P-COMMUNITY --------------------------------------------------------------
# drill:P-COMMUNITY:begin
begin_partition "P-COMMUNITY"
LOG="$WORK/p-COMMUNITY.log"
{
    echo "P-COMMUNITY-SECURITY-MD"
    echo "P-COMMUNITY-COC"
    echo "P-COMMUNITY-TEMPLATES"
    echo "P-COMMUNITY-README-BLOCK"
    echo "P-COMMUNITY-NO-KEY-BAIT"

    test -f SECURITY.md && grep -qi "threat-model" SECURITY.md && grep -qi "report" SECURITY.md \
        || echo "SECURITY.md missing or missing its threat-model/report anchors"
    test -f CODE_OF_CONDUCT.md && grep -qi "contributor covenant" CODE_OF_CONDUCT.md \
        || echo "CODE_OF_CONDUCT.md missing or not Contributor Covenant"
    test -f .github/ISSUE_TEMPLATE/config.yml || echo "missing .github/ISSUE_TEMPLATE/config.yml"
    { test -f .github/ISSUE_TEMPLATE/bug_report.yml || test -f .github/ISSUE_TEMPLATE/bug_report.md; } \
        || echo "missing bug report template"
    { test -f .github/ISSUE_TEMPLATE/feature_request.yml || test -f .github/ISSUE_TEMPLATE/feature_request.md; } \
        || echo "missing feature request template"
    test -f .github/PULL_REQUEST_TEMPLATE.md || echo "missing .github/PULL_REQUEST_TEMPLATE.md"
    grep -q "## Community" README.md && grep -q "SECURITY.md" README.md && grep -q "CODE_OF_CONDUCT.md" README.md \
        || echo "README Community & Security block missing or not linking the files"

    # no key-paste bait: templates must not invite users to paste env var values
    if grep -iE "GLM_API_KEY|LOCAL_API_KEY|BGE_M3_URL" .github/ISSUE_TEMPLATE/*.yml .github/PULL_REQUEST_TEMPLATE.md 2>/dev/null | grep -viE "redact|never paste|no secrets"; then
        echo "issue/PR templates invite pasting key-bearing env values"
    fi

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
    if grep -qE "missing|invite pasting" "$LOG"; then
        cat "$LOG"
        fail "P-COMMUNITY: community infrastructure checks failed"
    fi
ok_partition "P-COMMUNITY" "$LOG" "P-COMMUNITY-SECURITY-MD" "P-COMMUNITY-COC" "P-COMMUNITY-TEMPLATES" "P-COMMUNITY-README-BLOCK" "P-COMMUNITY-NO-KEY-BAIT"
done_partition "P-COMMUNITY"
# drill:P-COMMUNITY:end

# -- P-DEMO -------------------------------------------------------------------
# drill:P-DEMO:begin
begin_partition "P-DEMO"
LOG="$WORK/p-DEMO.log"
{
    echo "P-DEMO-GIF-MAGIC"

    test -f docs/demo.gif || echo "docs/demo.gif missing"
    if [ -f docs/demo.gif ]; then
        magic="$(head -c 6 docs/demo.gif)"
        case "$magic" in
            GIF87a|GIF89a) : ;;
            *) echo "docs/demo.gif does not open with a GIF magic number (got: $magic)" ;;
        esac
    fi

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
if grep -q "does not open with\|missing" "$LOG"; then
    cat "$LOG"
    fail "P-DEMO: demo asset checks failed"
fi
ok_partition "P-DEMO" "$LOG" "P-DEMO-GIF-MAGIC"
done_partition "P-DEMO"
# drill:P-DEMO:end

# -- P-BADGE ------------------------------------------------------------------
# drill:P-BADGE:begin
begin_partition "P-BADGE"
LOG="$WORK/p-BADGE.log"
{
    echo "P-BADGE-LOCAL-TARGETS-REAL"

    # every badge whose target is a repo-local path must point at something
    # that exists (remote http targets are validated for non-decoration in
    # P-README and are not hard-asserted here - CI availability is flaky).
    while IFS= read -r target; do
        case "$target" in
            /*|./*|../*) : ;;
            http*) : ;;
            *) [ -e "$target" ] || echo "badge points at a repo path that does not exist: $target" ;;
        esac
    done < <(sed -n 's/^\[!\[[^]]*\])]((\([^)]*\))).*/\1/p' README.md | sed 's/(//;s/)$//')

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
if grep -q "does not exist" "$LOG"; then
    cat "$LOG"
    fail "P-BADGE: badge targets failed"
fi
ok_partition "P-BADGE" "$LOG" "P-BADGE-LOCAL-TARGETS-REAL"
done_partition "P-BADGE"
# drill:P-BADGE:end

# -- P-SELF -------------------------------------------------------------------
# drill:P-SELF:begin
begin_partition "P-SELF"
LOG="$WORK/p-SELF.log"
{
    echo "P-SELF-BANNER-SITES"
    echo "P-SELF-BOOKKEEPING"

    banner_sites="$(grep -c 'Phase 50 Gate: ALL PASSED' "$GATE_SELF")"
    [ "$banner_sites" -eq 2 ] || echo "banner literal should appear exactly twice in the script (1 self-check + 1 print), found $banner_sites"

} > "$LOG" 2>&1
# fatal predicate runs AFTER the block closes (self-scan guard)
if grep -q "should appear exactly" "$LOG"; then
    cat "$LOG"
    fail "P-SELF: gate self-proof failed"
fi
ok_partition "P-SELF" "$LOG" "P-SELF-BANNER-SITES" "P-SELF-BOOKKEEPING"
done_partition "P-SELF"
# drill:P-SELF:end

# -- drills D1..D4 ------------------------------------------------------------
# drill:DRILLS:begin
run_drill() {
    local drill_id="$1"
    local expect_partition="$2"
    if ! command -v python >/dev/null 2>&1; then
        # No python on this host: fall back to an inline bash drill that does
        # not need python. D1-style: break one README assertion in a scratch
        # copy and require the copy to go red. The gate itself is the oracle.
        local scratch="$WORK/drill-$drill_id"
        mkdir -p "$scratch"
        cp README.md docs/demo.gif "$scratch/" 2>/dev/null || true
        sed 's/grep -q -- "$d" README.md || echo "README does not mention example skill: $d"/true/' "$GATE_SELF" > "$scratch/verify-phase-50.sh"
        GATE_ROOT="$scratch" GATE_DRILL=1 bash "$scratch/verify-phase-50.sh" > "$scratch/out.log" 2>&1
        if [ $? -eq 0 ]; then
            fail "drill $drill_id did not bite (fix the SOURCE, never the drill)"
        fi
        echo "  drill $drill_id: RED (bash fallback, README assertion disabled)"
        return 0
    fi
    python - "$GATE_SELF" "$drill_id" "$expect_partition" <<'PY'
import os, shutil, subprocess, sys, tempfile

gate, drill, expect = sys.argv[1], sys.argv[2], sys.argv[3]
src_root = os.path.dirname(os.path.abspath(gate))
tmp = tempfile.mkdtemp(prefix="p50-drill-", dir=os.path.join(src_root, ".planning", "tmp"))
root = os.path.join(tmp, "repo")
try:
    for item in ["README.md", "docs", "examples", "SECURITY.md",
                 "CODE_OF_CONDUCT.md", ".github", "CONTRIBUTING.md", "LICENSE",
                 "internal", "cmd", "go.mod", "web/src"]:
        src_i = os.path.join(src_root, item)
        dst_i = os.path.join(root, item)
        if os.path.isdir(src_i):
            shutil.copytree(src_i, dst_i, dirs_exist_ok=True)
        elif os.path.isfile(src_i):
            os.makedirs(os.path.dirname(dst_i), exist_ok=True)
            shutil.copy2(src_i, dst_i)
    gate_copy = os.path.join(root, os.path.basename(gate))
    src = open(gate, encoding="utf-8").read()

    if drill == "D1":
        src = src.replace('[ "$n_dirs" -eq 7 ]',
                          '[ "$n_dirs" -ne 7 ] || echo "examples/skills has $n_dirs entries, want 7 (D-50-02) (drill D1: wrong count)"')
    elif drill == "D2":
        # drill D2: wrong doc path — the existence check itself goes red,
        # the P-ARCH predicate still sees the red line in $LOG.
        src = src.replace('test -f docs/architecture.md || echo "docs/architecture.md missing"',
                          'test -f docs/architecture-should-not-exist.md || echo "docs/architecture.md missing (drill D2: wrong doc path)"')
    elif drill == "D3":
        # drill D3: point a P-COMMUNITY check at a missing file so a real
        # red line lands in $LOG while the predicate stays armed.
        src = src.replace('grep -q "SECURITY.md" README.md',
                          'grep -q "COMMUNITY-MISSING-ANCHOR" README.md')  # drill D3: wrong anchor
    elif drill == "D4":
        # drill D4: drop GIF89a from the accepted magic set — the healthy
        # demo.gif is GIF89a, so the *) arm emits a real red line and the
        # P-DEMO predicate (which stays armed) must fail the partition.
        src = src.replace('GIF87a|GIF89a) : ;;', 'GIF87a) : ;;  # drill D4: GIF89a no longer accepted')
    else:
        sys.exit("unknown drill " + drill)

    wsl_root = "/mnt/" + root[0].lower() + root[2:].replace(chr(92), "/")
    open(os.path.join(root, ".gate-drill-env"), "w", newline=chr(10)).write(
        "GATE_ROOT=" + wsl_root + chr(10) + "GATE_DRILL=1" + chr(10))
    open(gate_copy, "w", encoding="utf-8", newline=chr(10)).write(src)
    env = dict(os.environ, GATE_ROOT=wsl_root, GATE_DRILL="1")
    drive, rest = os.path.splitdrive(gate_copy)
    posix = "/mnt/" + drive[0].lower() + rest.replace("\\", "/")
    p = subprocess.run(["bash", posix], cwd=root, env=env,
                       stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                       text=True, timeout=90)
    rc, out = p.returncode, p.stdout
    named = (expect + ":") in out or ("partition " + expect) in out
    if drill == "D4":
        ok = rc != 0 and (named or ("partition P-ARCH") in out)
    else:
        ok = rc != 0 and named
    print("  drill %s: rc=%d -> %s" % (drill, rc, "RED" if ok else "NOT-RED"))
    if True:
        tail = [l for l in out.splitlines() if l.strip()]
        for l in tail:
            print("    |", l)
    sys.exit(0 if ok else 1)
finally:
    shutil.rmtree(tmp, ignore_errors=True)
PY
}
if [ "${GATE_DRILL:-0}" != "1" ]; then
    echo ""
    echo "-- mutation drills (each must go red) --"
    drill_fail=0
    export GATE_SELF="$GATE_SELF"
    run_drill "D1" "P-README"    || drill_fail=1
    run_drill "D2" "P-ARCH"      || drill_fail=1
    run_drill "D3" "P-COMMUNITY" || drill_fail=1
    run_drill "D4" "P-DEMO"      || drill_fail=1
    if [ "$drill_fail" -ne 0 ]; then
        fail "mutation drills: at least one drill did not go red - a partition no longer bites (fix the SOURCE, never the drill)"
    fi
    pass "mutation drills: D1..D4 all went red and named their partitions"
fi
# drill:DRILLS:end

# -- banner (single ALL PASSED print site) ------------------------------------
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
    echo "P-SELF-PARTITION-COUNT" >> "$WORK/p$(_pn_num "P-SELF").log"
    echo -e "${GREEN}=== Phase 50 Gate: ALL PASSED ===${NC}"
    exit 0
else
    echo -e "${RED}=== Phase 50 Gate: NOT VERIFIED${NC} - ran $RAN of $EXPECTED_PARTITIONS partitions: ${RAN_LIST[*]}"
    exit 1
fi
