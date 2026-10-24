# Release engineering gate (Phase 51, SC4): goreleaser config, workflow,
# README release docs and the permission gradient, all mutation-proven.
# Never requires network, a goreleaser install, or a real tag push.
set -uo pipefail
export MSYS_NO_PATHCONV=1

ROOT="${GATE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
cd "$ROOT"
SELF="verify-phase-51.sh"
# drill-env handoff: a drill leaves this file in its scratch copy; the
# inner gate must NOT run drills again.
for _f in "$(dirname "$0")/.gate-drill-env" ".gate-drill-env"; do
    if [ -f "$_f" ]; then . "$_f"; break; fi
done

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
pass() { echo -e "[0;32mPASS[0m $1"; }
info() { echo "info: $1"; }
fail() { echo -e "[0;31mFAIL[0m $1"; exit 1; }
_pn_num() { echo "$1" | sed "s/^P-//"; }
EXPECTED_PARTITIONS=5
RAN=0
RAN_LIST=()
begin_partition() { echo "$1" > "$WORK/p-$(_pn_num "$1").begin"; }
ok_partition() {
    local pn="$1" log="$2"; shift 2
    local tok missing=""
    [ -f "$log" ] || fail "partition ${pn} has no probe log at ${log}"
    [ -s "$log" ] || fail "partition ${pn} probe log is empty at ${log}"
    for tok in "$@"; do grep -qF -- "$tok" "$log" || missing="$missing $tok"; done
    [ -z "$missing" ] || fail "partition ${pn} cannot deposit its success tokens: ${log} lacks ${missing}"
    : > "$WORK/ran/$(_pn_num "$pn").ok"
}
done_partition() {
    local num; num="$(_pn_num "$1")"
    [ -f "$WORK/p-${num}.begin" ] || fail "partition $1 ran without its begin marker"
    [ -f "$WORK/ran/${num}.ok" ] || fail "partition $1 finalized without a deposit marker"
    RAN=$((RAN + 1)); RAN_LIST+=("$1")
}
mkdir -p "$WORK/ran"

# ---------------------------------------------------------------- P-CONFIG
# drill:P-CONFIG:begin
begin_partition "P-CONFIG"
LOG="$WORK/p-CONFIG.log"
{
    if [ -f .goreleaser.yaml ]; then
        echo "P-CONFIG-EXISTS"
        grep -q "^version: 2" .goreleaser.yaml && echo "P-CONFIG-V2"
        grep -q "project_name: skillforge" .goreleaser.yaml && echo "P-CONFIG-NAME"
        grep -q "main: ./cmd/server" .goreleaser.yaml && echo "P-CONFIG-MAIN"
        grep -q "CGO_ENABLED=1" .goreleaser.yaml && echo "P-CONFIG-CGO"
        grep -q "windows" .goreleaser.yaml && echo "P-CONFIG-WINDOWS"
    else
        echo ".goreleaser.yaml missing"
    fi
    if command -v goreleaser >/dev/null 2>&1; then
        goreleaser check >> "$LOG.ck" 2>&1 && echo "GORELEASER-CHECK-OK"
    else
        echo "goreleaser not installed - schema check skipped (GORELEASER-SKIPPED)"
    fi
    # go-direct schema anchor: the config must at least parse as the
    # document goreleaser v2 expects (version line + builds list).
    grep -q "^builds:" .goreleaser.yaml && echo "P-CONFIG-BUILDS-LIST"
} > "$LOG" 2>&1
if grep -qE "missing|does not exist" "$LOG"; then
    cat "$LOG"
    fail "P-CONFIG: goreleaser config checks failed"
fi
ok_partition "P-CONFIG" "$LOG" "P-CONFIG-EXISTS" "P-CONFIG-V2" "P-CONFIG-NAME" "P-CONFIG-MAIN" "P-CONFIG-CGO" "P-CONFIG-WINDOWS" "P-CONFIG-BUILDS-LIST"
done_partition "P-CONFIG"
# drill:P-CONFIG:end

# -------------------------------------------------------------- P-WORKFLOW
# drill:P-WORKFLOW:begin
begin_partition "P-WORKFLOW"
LOG="$WORK/p-WORKFLOW.log"
{
    WF=.github/workflows/release.yml
    if [ -f "$WF" ]; then
        echo "P-WORKFLOW-EXISTS"
        grep -q "tags:" "$WF" && grep -q "v\*" "$WF" && echo "P-WORKFLOW-TAGS-ONLY"
        for j in build-linux build-windows release; do grep -q "^  $j:" "$WF" || echo "workflow missing job: $j"; done
        awk '/^  release:/,0 { if ($0 ~ /contents: write/) f=1 } END { exit f?0:1 }' "$WF" && echo "P-WORKFLOW-RELEASE-WRITE"
        grep -q "msys2" "$WF" || echo "windows job is not msys2-based"
        grep -q "CGO_ENABLED" "$WF" || echo "windows job does not set CGO"
        grep -q "gh-release" "$WF" && grep -q "generate_release_notes" "$WF" && echo "P-WORKFLOW-GH-RELEASE"
        grep -q "goreleaser" "$WF" || echo "linux job does not run goreleaser"
    else
        echo "release.yml missing"
    fi
} > "$LOG" 2>&1
if grep -qE "missing|not msys2|does not set CGO" "$LOG"; then
    cat "$LOG"
    fail "P-WORKFLOW: release workflow checks failed"
fi
ok_partition "P-WORKFLOW" "$LOG" "P-WORKFLOW-EXISTS" "P-WORKFLOW-TAGS-ONLY" "P-WORKFLOW-RELEASE-WRITE" "P-WORKFLOW-GH-RELEASE"
done_partition "P-WORKFLOW"
# drill:P-WORKFLOW:end

# --------------------------------------------------------------- P-README
# drill:P-README:begin
begin_partition "P-README"
LOG="$WORK/p-README.log"
{
    grep -q "^## Releases" README.md && echo "P-README-SECTION"
    grep -q "skillforge-server-linux-amd64" README.md && echo "P-README-ASSET-NAME"
    grep -q "stub mode" README.md && echo "P-README-STUB-CAVEAT"
    grep -q "Releases page" README.md && echo "P-README-RELEASES-LINK"
} > "$LOG" 2>&1
mr=""
for tok in P-README-SECTION P-README-ASSET-NAME P-README-STUB-CAVEAT P-README-RELEASES-LINK; do
    grep -qF "$tok" "$LOG" || mr="$mr $tok"
done
if [ -n "$mr" ]; then cat "$LOG"; fail "P-README: release docs missing tokens:$mr"; fi
ok_partition "P-README" "$LOG" "P-README-SECTION" "P-README-ASSET-NAME" "P-README-STUB-CAVEAT" "P-README-RELEASES-LINK"
done_partition "P-README"
# drill:P-README:end

# ------------------------------------------------------------- P-SNAPSHOT
# drill:P-SNAPSHOT:begin
begin_partition "P-SNAPSHOT"
LOG="$WORK/p-SNAPSHOT.log"
SNAP="dist/skillforge-server_linux_amd64_v1/skillforge-server"
{
    if [ -f "$SNAP" ]; then
        [ "$(head -c 4 "$SNAP" | od -An -tx1 | tr -d ' 
')" = "7f454c46" ] && echo "P-SNAPSHOT-ELF"
    else
        echo "snapshot binary not built on this host (P-SNAPSHOT-SKIPPED)"
    fi
} > "$LOG" 2>&1
if grep -q "not built on this host" "$LOG"; then
    info "P-SNAPSHOT: skipped honestly (no local snapshot)"
else
    grep -q "P-SNAPSHOT-ELF" "$LOG" || { cat "$LOG"; fail "P-SNAPSHOT: snapshot binary is not a linux ELF"; }
fi
if grep -q "P-SNAPSHOT-ELF" "$LOG"; then
    ok_partition "P-SNAPSHOT" "$LOG" "P-SNAPSHOT-ELF"
else
    ok_partition "P-SNAPSHOT" "$LOG" "P-SNAPSHOT-SKIPPED"
fi
done_partition "P-SNAPSHOT"
# drill:P-SNAPSHOT:end

# ---------------------------------------------------------------- P-SELF
# drill:P-SELF:begin
begin_partition "P-SELF"
LOG="$WORK/p-SELF.log"
{
    BEGIN_COUNT=$(find "$WORK" -name "*.begin" 2>/dev/null | wc -l)
    OK_COUNT=$(find "$WORK/ran" -name "*.ok" 2>/dev/null | wc -l)
    echo "P-SELF-BEGINS-$BEGIN_COUNT"
    echo "P-SELF-OKS-$OK_COUNT"
    BANNER_LINE=$(grep -n "=== Phase 51 Gate: ALL PASSED ===" "$SELF" | tail -1 | cut -d: -f1)
    RAN_LINE=$(grep -n 'if \[ "$RAN" -eq "$EXPECTED_PARTITIONS" \]' "$SELF" | tail -1 | cut -d: -f1)
    if [ -n "$BANNER_LINE" ] && [ -n "$RAN_LINE" ] && [ "$BANNER_LINE" -gt "$RAN_LINE" ]; then
        echo "P-SELF-BANNER-GATED"
    else
        echo "the passing banner is not gated behind the RAN check"
    fi
} > "$LOG" 2>&1
grep -q "P-SELF-BANNER-GATED" "$LOG" || { cat "$LOG"; fail "P-SELF: banner must sit inside the RAN-gated branch"; }
grep -q "P-SELF-BEGINS-5" "$LOG" || { cat "$LOG"; fail "P-SELF: expected 5 begin markers"; }
grep -q "P-SELF-OKS-4" "$LOG" || { cat "$LOG"; fail "P-SELF: expected 4 predecessor ok markers (own deposit lands after this check)"; }
ok_partition "P-SELF" "$LOG" "P-SELF-BANNER-GATED" "P-SELF-BEGINS-5" "P-SELF-OKS-4"
done_partition "P-SELF"
# drill:P-SELF:end

# ------------------------------------------------------------------ drills
# drill:DRILLS:begin
if [ "${GATE_DRILL:-0}" != "1" ]; then
for drill_id in D1 D2 D3 D4; do
  GATE_DRILL_ID="$drill_id" python - "$ROOT" "$SELF" <<'PYD'
import os, re, shutil, subprocess, sys, tempfile
root, gate = sys.argv[1], sys.argv[2]
if os.name == "nt" and re.match(r"^/[a-zA-Z]/", root):
    root = root[1].upper() + ":" + root[2:]
root = os.path.abspath(root)
tmp = tempfile.mkdtemp(dir=os.path.join(root, ".planning", "tmp"))
for item in [".goreleaser.yaml", ".github", "README.md", "verify-phase-51.sh", ".planning/phases/51-release-engineering"]:
    src = os.path.join(root, item)
    dst = os.path.join(tmp, item)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    (shutil.copytree if os.path.isdir(src) else shutil.copy2)(src, dst)
snap = os.path.join(root, "dist", "skillforge-server_linux_amd64_v1", "skillforge-server")
if os.path.isfile(snap):
    d = os.path.join(tmp, "dist", "skillforge-server_linux_amd64_v1")
    os.makedirs(d, exist_ok=True)
    shutil.copy2(snap, os.path.join(d, "skillforge-server"))
gate_copy = os.path.join(tmp, os.path.basename(gate))
src = open(gate, encoding="utf-8").read()
drill = os.environ.get("GATE_DRILL_ID", "")
if drill == "D1":
    src = src.replace("version: 2", "version: 1", 1)
elif drill == "D2":
    src = src.replace("tags:", "branches:", 1)
elif drill == "D3":
    src = src.replace("contents: write", "contents: read", 1)
elif drill == "D4":
    src = src.replace("## Releases", "## Archives", 1)
else:
    sys.exit("unknown drill " + drill)
open(os.path.join(tmp, ".gate-drill-env"), "w", newline=chr(10)).write("GATE_ROOT=" + root.replace(chr(92), "/") + chr(10) + "GATE_DRILL=1" + chr(10))
open(gate_copy, "w", encoding="utf-8", newline=chr(10)).write(src)
if os.name == "nt":
    bash = os.path.join(os.environ.get("ProgramFiles", "C:\Program Files"), "Git", "bin", "bash.exe")
    r = subprocess.run([bash, "--noprofile", "--norc", gate_copy], capture_output=True, text=True, timeout=300, cwd=tmp)
else:
    r = subprocess.run(["bash", gate_copy], capture_output=True, text=True, timeout=300)
out = (r.stdout or "") + (r.stderr or "")
expect = {"D1": "P-CONFIG", "D2": "P-WORKFLOW", "D3": "P-WORKFLOW", "D4": "P-README"}[drill]
if r.returncode == 0:
    print("  drill %s: rc=0 -> NOT-RED" % drill)
    print("\n".join("    | " + l for l in out.splitlines()[-8:]))
    sys.exit(1)
if (expect + ":") not in out and ("partition " + expect) not in out:
    print("  drill %s: rc=%d but did not name %s -> NOT-RED" % (drill, r.returncode, expect))
    print("\n".join("    | " + l for l in out.splitlines()[-8:]))
    sys.exit(1)
print("  drill %s: rc=%d -> RED (%s named)" % (drill, r.returncode, expect))
PYD
status=$?
if [ "$status" -ne 0 ]; then
    fail "mutation drills: drill $drill_id did not go red (fix the SOURCE, never the drill)"
fi
done
rm -rf "$ROOT/.planning/tmp"/tmp.* 2>/dev/null
pass "mutation drills: D1..D4 all went red and named their partitions"
# drill:DRILLS:end
fi

# ------------------------------------------------------------------ banner
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
    echo -e "[0;32m=== Phase 51 Gate: ALL PASSED ===[0m"
else
    NOT_VERIFIED=""
    for pn in P-CONFIG P-WORKFLOW P-README P-SNAPSHOT P-SELF; do
        seen=""
        for s in "${RAN_LIST[@]}"; do [ "$s" = "$pn" ] && seen=1; done
        [ -n "$seen" ] || NOT_VERIFIED="$NOT_VERIFIED $pn"
    done
    echo "=== Phase 51 Gate: NOT VERIFIED - partitions not run:$NOT_VERIFIED (ran $RAN of $EXPECTED_PARTITIONS) ==="
    exit 1
fi
## Release engineering gate (Phase 51, SC4): goreleaser config, workflow,
# README release docs and the permission gradient, all mutation-proven.
# Never requires network, a goreleaser install, or a real tag push.
set -uo pipefail
export MSYS_NO_PATHCONV=1

ROOT="${GATE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
cd "$ROOT"
SELF="verify-phase-51.sh"
# drill-env handoff: a drill leaves this file in its scratch copy; the
# inner gate must NOT run drills again.
for _f in "$(dirname "$0")/.gate-drill-env" ".gate-drill-env"; do
    if [ -f "$_f" ]; then . "$_f"; break; fi
done

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
pass() { echo -e "[0;32mPASS[0m $1"; }
info() { echo "info: $1"; }
fail() { echo -e "[0;31mFAIL[0m $1"; exit 1; }
_pn_num() { echo "$1" | sed "s/^P-//"; }
EXPECTED_PARTITIONS=5
RAN=0
RAN_LIST=()
begin_partition() { echo "$1" > "$WORK/p-$(_pn_num "$1").begin"; }
ok_partition() {
    local pn="$1" log="$2"; shift 2
    local tok missing=""
    [ -f "$log" ] || fail "partition ${pn} has no probe log at ${log}"
    [ -s "$log" ] || fail "partition ${pn} probe log is empty at ${log}"
    for tok in "$@"; do grep -qF -- "$tok" "$log" || missing="$missing $tok"; done
    [ -z "$missing" ] || fail "partition ${pn} cannot deposit its success tokens: ${log} lacks ${missing}"
    : > "$WORK/ran/$(_pn_num "$pn").ok"
}
done_partition() {
    local num; num="$(_pn_num "$1")"
    [ -f "$WORK/p-${num}.begin" ] || fail "partition $1 ran without its begin marker"
    [ -f "$WORK/ran/${num}.ok" ] || fail "partition $1 finalized without a deposit marker"
    RAN=$((RAN + 1)); RAN_LIST+=("$1")
}
mkdir -p "$WORK/ran"

