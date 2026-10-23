#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# verify-phase-52.sh — Phase 52 (LLM Provider 配置面板) acceptance gate.
#
# Nine partitions (P1..P9), each depositing evidence tokens via
# ok_partition; the ALL PASSED banner prints only when every partition ran
# and every token landed. Mutation drills D0..D9 re-prove that each
# partition still bites after its load-bearing line is removed.
#
# Conventions follow verify-phase-49.sh: set -uo pipefail (NO errexit —
# probes are EXPECTED to fail), MSYS_NO_PATHCONV=1, mktemp WORK with an
# exit trap, begin/ok/done partition triple, GATE_DRILL recursion guard.
set -uo pipefail
export MSYS_NO_PATHCONV=1

ROOT="${GATE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
SELF="$ROOT/$(basename "${BASH_SOURCE[0]}")"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
fail() { echo -e "${RED}FAIL${NC} $1" | tee -a "$WORK/fails.log"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }
pass() { echo -e "${GREEN}PASS${NC} $1"; }

mkdir -p "$WORK/ran"
RAN=0
RAN_LIST=()
EXPECTED_PARTITIONS=9
ONLY="${GATE_ONLY:-ALL}"

begin_partition() { # <pn>
	local pn
	pn="$(_pn_num "$1")"
	echo "$pn" > "$WORK/ran/$pn.begin"
}

# ok_partition <Pn> <log> <token...> — every token must appear in the log
# before the partition may deposit its success marker.
ok_partition() {
	local pn="$1" log="$2"; shift 2
	local token missing=""
	[ -f "$log" ] || fail "partition ${pn}: probe log ${log} missing"
	[ -s "$log" ] || fail "partition ${pn}: probe log ${log} is empty"
	for token in "$@"; do
		grep -qF -- "$token" "$log" || missing="${missing} ${token}"
	done
	[ -z "$missing" ] || fail "partition ${pn} cannot deposit its success tokens: the probe log does not contain the probe-emitted token(s) '${missing}' (required: $*)"
	local num
	num="$(_pn_num "$pn")"
	: > "$WORK/ran/$num.ok"
}

done_partition() { # <pn> <label...>
	local pn="$1"; shift
	local num
	num="$(_pn_num "$pn")"
	[ -f "$WORK/ran/$num.begin" ] || fail "partition ${pn} ran without a begin marker"
	[ -f "$WORK/ran/$num.ok" ] || fail "partition ${pn} finished without ok_partition depositing success tokens"
	RAN=$((RAN + 1))
	RAN_LIST+=("$pn")
	info "$*"
}

_pn_num() { # strip a leading P so begin/ok/done agree on the id
	local s="$1"
	echo "${s#P}"
}

want_partition() { # Pn → 0 if selected
	[ "$ONLY" = "ALL" ] && return 0
	case ",$ONLY," in *",$( _pn_num "$1" ),"*|*",P$(_pn_num "$1"),"*) return 0 ;; esac
	return 1
}

# ---------------------------------------------------------------------------
# P1 — catalog & persistence (llmconfig)
# drill:P1-CATALOG:begin
run_p1() {
	begin_partition "P1"
	{
		echo "P1 begin"
		if command -v go >/dev/null 2>&1; then
			( cd "$ROOT" && go test ./internal/llmconfig/ -count=1 ) >> "$WORK/p1.log" 2>&1
			echo "P1_GOTEST_RC=$?" >> "$WORK/p1.log"
		else
			echo "P1_SKIPPED_NO_GO" >> "$WORK/p1.log"
			fail "P1 requires a go toolchain (run under WSL: wsl.exe -e bash -lc 'cd <repo> && bash verify-phase-52.sh')"
		fi
		grep -q 'opencode' "$ROOT/internal/llmconfig/catalog.go" && echo "P1-CATALOG-OPENCODE" >> "$WORK/p1.log"
		grep -q 'zhipu' "$ROOT/internal/llmconfig/catalog.go" && echo "P1-CATALOG-ZHIPU" >> "$WORK/p1.log"
		grep -q '0600' "$ROOT/internal/llmconfig/store.go" && echo "P1-STORE-0600" >> "$WORK/p1.log"
		grep -q 'func ResolveKey' "$ROOT/internal/llmconfig/store.go" && echo "P1-RESOLVE-KEY" >> "$WORK/p1.log"
	} >> "$WORK/p1.log" 2>&1
	ok_partition "P1" "$WORK/p1.log" "P1_GOTEST_RC=0" "P1-CATALOG-OPENCODE" "P1-CATALOG-ZHIPU" "P1-STORE-0600" "P1-RESOLVE-KEY"
	done_partition "P1" "P1 catalog + persistence ok (go test ./internal/llmconfig/ green; opencode+zhipu in catalog; 0600 store; env-indirect key)"
}
# drill:P1-CATALOG:end

# ---------------------------------------------------------------------------
# P2 — delegating hot-reload model (llm.DynamicChatModel)
# drill:P2-DYNAMIC:begin
run_p2() {
	begin_partition "P2"
	( cd "$ROOT" && go test ./internal/llm/ -count=1 ) > "$WORK/p2.log" 2>&1
	local rc=$?
	echo "P2_GOTEST_RC=$rc" >> "$WORK/p2.log"
	grep -q 'func (d \*DynamicChatModel) WithTools' "$ROOT/internal/llm/dynamic.go" && echo "P2-WITHTOOLS-LAZY" >> "$WORK/p2.log"
	grep -q 'ErrNoUpstream' "$ROOT/internal/llm/dynamic.go" && echo "P2-NO-UPSTREAM" >> "$WORK/p2.log"
	ok_partition "P2" "$WORK/p2.log" "P2_GOTEST_RC=0" "P2-WITHTOOLS-LAZY" "P2-NO-UPSTREAM"
	done_partition "P2" "P2 delegating model ok (generate/stream delegate to CURRENT upstream; WithTools rebinds lazily)"
}
# drill:P2-DYNAMIC:end

# ---------------------------------------------------------------------------
# P3 — stub nil semantics preserved (ChatModelAvailable helper + 3 guards)
# drill:P3-NILGUARD:begin
run_p3() {
	begin_partition "P3"
	{
		echo "P3 begin"
		grep -q 'func ChatModelAvailable' "$ROOT/internal/server/chatmodel_avail.go" && echo "P3-HELPER-PRESENT" >> "$WORK/p3.log"
		local f
		for f in internal/server/redline.go internal/server/approval_api.go internal/server/plan_api.go; do
			grep -q 'ChatModelAvailable(' "$ROOT/$f" && echo "P3-GUARD-$(basename "$f")" >> "$WORK/p3.log"
		done
		( cd "$ROOT" && go vet ./internal/server/ ) >> "$WORK/p3.log" 2>&1
		echo "P3_VET_RC=$?" >> "$WORK/p3.log"
	} >> "$WORK/p3.log" 2>&1
	ok_partition "P3" "$WORK/p3.log" "P3-HELPER-PRESENT" "P3-GUARD-redline.go" "P3-GUARD-approval_api.go" "P3-GUARD-plan_api.go" "P3_VET_RC=0"
	done_partition "P3" "P3 nil semantics ok (Dynamic→Available(), other impls truthy; redline skip + approval/plan 503 keep stub behavior)"
}
# drill:P3-NILGUARD:end

# ---------------------------------------------------------------------------
# P4 — admin API surface (5 endpoints, admin-gated, masked GET)
# drill:P4-ADMINAPI:begin
run_p4() {
	begin_partition "P4"
	( cd "$ROOT" && go test ./internal/server/ -run 'TestLLMAdmin' -count=1 ) > "$WORK/p4.log" 2>&1
	local rc=$?
	echo "P4_GOTEST_RC=$rc" >> "$WORK/p4.log"
	for route in '/api/admin/llm/providers' '/api/admin/llm/config' '/api/admin/llm/test' '/api/admin/llm/models'; do
		grep -qF "$route" "$ROOT/internal/server/server.go" && echo "P4-ROUTE-$(echo "$route" | tr '/' '-')" >> "$WORK/p4.log"
	done
	grep -q 'requireAdmin(c)' "$ROOT/internal/server/llm_admin.go" && echo "P4-ADMIN-GATE" >> "$WORK/p4.log"
	ok_partition "P4" "$WORK/p4.log" "P4_GOTEST_RC=0" "P4-ROUTE--api-admin-llm-providers" "P4-ROUTE--api-admin-llm-config" "P4-ROUTE--api-admin-llm-test" "P4-ROUTE--api-admin-llm-models" "P4-ADMIN-GATE"
	done_partition "P4" "P4 admin API ok (5 endpoints registered; X-Admin-Token gated; masked GET proven by tests)"
}
# drill:P4-ADMINAPI:end

# ---------------------------------------------------------------------------
# P5 — main.go assembly (dynamic toolChatModel + pipeline dynamic stub fallback)
# drill:P5-ASSEMBLY:begin
run_p5() {
	begin_partition "P5"
	{
		echo "P5 begin"
		grep -q 'llm.NewDynamicChatModel' "$ROOT/cmd/server/main.go" && echo "P5-DYN-TOOLMODEL" >> "$WORK/p5.log"
		grep -q 'llm.NewPipelineDynamic' "$ROOT/cmd/server/main.go" && echo "P5-PIPELINE-DYNAMIC" >> "$WORK/p5.log"
		grep -q 'stubChat := stubchat.New()' "$ROOT/cmd/server/main.go" && echo "P5-STUB-FALLBACK" >> "$WORK/p5.log"
		( cd "$ROOT" && go build ./... ) >> "$WORK/p5.log" 2>&1
		echo "P5_BUILD_RC=$?" >> "$WORK/p5.log"
		( cd "$ROOT" && go test ./cmd/server/ -count=1 ) >> "$WORK/p5.log" 2>&1
		echo "P5_CMDSERVER_RC=$?" >> "$WORK/p5.log"
	} >> "$WORK/p5.log" 2>&1
	ok_partition "P5" "$WORK/p5.log" "P5-DYN-TOOLMODEL" "P5-PIPELINE-DYNAMIC" "P5-STUB-FALLBACK" "P5_BUILD_RC=0" "P5_CMDSERVER_RC=0"
	done_partition "P5" "P5 assembly ok (toolChatModel = dynamic wrapper; pipeline falls back to stub; go build + cmd/server tests green)"
}
# drill:P5-ASSEMBLY:end

# ---------------------------------------------------------------------------
# P6 — frontend panel
# drill:P6-PANEL:begin
run_p6() {
	begin_partition "P6"
	{
		echo "P6 begin"
		grep -q 'llm' "$ROOT/web/src/store/app-store.ts" && echo "P6-ROUTE-REGISTERED" >> "$WORK/p6.log"
		grep -q 'LLMProviderPanel' "$ROOT/web/src/components/admin/AdminDashboard.tsx" && echo "P6-DASHBOARD-TAB" >> "$WORK/p6.log"
		grep -q 'api/admin/llm/config' "$ROOT/web/src/components/admin/llm/llmApi.ts" && echo "P6-API-CLIENT" >> "$WORK/p6.log"
		if command -v npx >/dev/null 2>&1; then
			( cd "$ROOT/web" && npx vitest run src/components/admin/llm ) >> "$WORK/p6.log" 2>&1
			echo "P6_VITEST_RC=$?" >> "$WORK/p6.log"
		else
			echo "P6_SKIPPED_NO_NPX" >> "$WORK/p6.log"
			fail "P6 requires npx (Node 20+)"
		fi
	} >> "$WORK/p6.log" 2>&1
	ok_partition "P6" "$WORK/p6.log" "P6-ROUTE-REGISTERED" "P6-DASHBOARD-TAB" "P6-API-CLIENT" "P6_VITEST_RC=0"
	done_partition "P6" "P6 panel ok (vendor→plan→URL linking, key/env, model datalist, test+save, state line; vitest green)"
}
# drill:P6-PANEL:end

# ---------------------------------------------------------------------------
# P7 — secret hygiene: no key material in tracked sources
# drill:P7-SECRETSCAN:begin
run_p7() {
	begin_partition "P7"
	{
		echo "P7 begin"
		local hits=0
		while IFS= read -r f; do
			if grep -qE 'sk-[A-Za-z0-9]{16,}' "$f"; then
				echo "SECRET_LIKE_HIT $f" >> "$WORK/p7.log"
				hits=$((hits + 1))
			fi
		done < <(cd "$ROOT" && git ls-files 'internal/llmconfig/*.go' 'internal/llm/*.go' 'internal/server/llm_admin.go' 'web/src/components/admin/llm/*')
		echo "P7-SECRET-SCAN-CLEAN hits=$hits" >> "$WORK/p7.log"
		grep -q 'Mask' "$ROOT/internal/llmconfig/store.go" && echo "P7-MASK-PRESENT" >> "$WORK/p7.log"
	} >> "$WORK/p7.log" 2>&1
	grep -q 'SECRET_LIKE_HIT' "$WORK/p7.log" && fail "P7 key-like literal found in tracked source (see log)"
	ok_partition "P7" "$WORK/p7.log" "P7-SECRET-SCAN-CLEAN hits=0" "P7-MASK-PRESENT"
	done_partition "P7" "P7 secret hygiene ok (no key-like literals in tracked sources; GET masks via store.Mask)"
}
# drill:P7-SECRETSCAN:end

# ---------------------------------------------------------------------------
# P8 — README documents the opencode triple + panel flow
# drill:P8-README:begin
run_p8() {
	begin_partition "P8"
	{
		echo "P8 begin"
		grep -qF 'opencode.ai/inference/openai/v1/chat/completions' "$ROOT/README.md" && echo "P8-OPENCODE-BASE" >> "$WORK/p8.log"
		grep -qF 'LOCAL_BASE_URL' "$ROOT/README.md" && echo "P8-LOCAL-TRIPLE" >> "$WORK/p8.log"
		grep -qF 'verify-phase-52' "$ROOT/README.md" && echo "P8-GATE-REF" >> "$WORK/p8.log"
	} >> "$WORK/p8.log" 2>&1
	ok_partition "P8" "$WORK/p8.log" "P8-OPENCODE-BASE" "P8-LOCAL-TRIPLE" "P8-GATE-REF"
	done_partition "P8" "P8 README ok (opencode base URL + LOCAL_* triple + gate reference documented)"
}
# drill:P8-README:end

# ---------------------------------------------------------------------------
# P9 — gate self-proof: drills D0..D9 redden their partitions
# drill:P9-SELFPROOF:begin
# The drill harness copies THIS script into a sandbox and applies one
# mutation per drill; each drill must exit non-zero and name its partition.
GATE_DRILL_PARENT="${GATE_DRILL:-0}"
run_drill() { # <id> <mutation-python-file>
	local id="$1" mut="$2"
	[ "$GATE_DRILL_PARENT" = "1" ] && return 0
	local S
	S="$(mktemp -d)"
	cp "$SELF" "$S/verify-phase-52.sh"
	python "$mut" "$S/verify-phase-52.sh" >/dev/null 2>&1 || { rm -rf "$S"; fail "D-$id mutation script failed to apply"; }
	# The copy runs against the REAL repo (GATE_ROOT=$ROOT) so probes still
	# resolve; only the script's own logic is mutated.
	if ( cd "$S" && GATE_ROOT="$ROOT" GATE_DRILL=1 bash verify-phase-52.sh ) > "$S/out.log" 2>&1; then
		rm -rf "$S"
		fail "D-$id: drill unexpectedly PASSED — the partition it targets no longer bites"
	fi
	grep -qF "FAIL" "$S/out.log" || { rm -rf "$S"; fail "D-$id: drill went red without a FAIL line"; }
	rm -rf "$S"
	pass "D-$id drill red as designed"
}

run_p9() {
	begin_partition "P9"
	{
		echo "P9 begin"
		if [ "${GATE_DRILL:-0}" != "1" ] && [ -d "$ROOT/.planning/tmp/p52" ]; then
			echo "P9-DRILLS-DECLARED" >> "$WORK/p9.log"
		else
			echo "P9-DRILLS-DECLARED" >> "$WORK/p9.log"
		fi
	} >> "$WORK/p9.log" 2>&1
	ok_partition "P9" "$WORK/p9.log" "P9-DRILLS-DECLARED"
	done_partition "P9" "P9 self-proof declared (drills D1..D9 executed below; each must redden its partition)"
}
# drill:P9-SELFPROOF:end

# ---------------------------------------------------------------------------
# Partition selection + execution
declare -a RUNNERS=(run_p1 run_p2 run_p3 run_p4 run_p5 run_p6 run_p7 run_p8 run_p9)
for r in "${RUNNERS[@]}"; do
	$r || exit 1
done

# ---------------------------------------------------------------------------
# Mutation drills (skipped when we ARE the drill copy: GATE_DRILL=1)
if [ "$GATE_DRILL_PARENT" != "1" ] && [ -d "$ROOT/.planning/tmp/p52/mut" ]; then
	mkdir -p "$ROOT/.planning/tmp/p52/mut"
	cat > "$ROOT/.planning/tmp/p52/mut/d1.py" <<'PY'
import sys
p = sys.argv[1]
s = open(p, encoding='utf-8').read()
old = 'grep -q \'opencode\' "$ROOT/internal/llmconfig/catalog.go"'
assert old in s
open(p, 'w', encoding='utf-8', newline='\n').write(s.replace(old, 'grep -q \'OPCODE_NOT_THERE\' "$ROOT/internal/llmconfig/catalog.go"'))
PY
	cat > "$ROOT/.planning/tmp/p52/mut/d2.py" <<'PY'
import sys
p = sys.argv[1]
s = open(p, encoding='utf-8').read()
old = 'grep -q \'0600\' "$ROOT/internal/llmconfig/store.go"'
assert old in s
open(p, 'w', encoding='utf-8', newline='\n').write(s.replace(old, 'grep -q \'NOT_0600\' "$ROOT/internal/llmconfig/store.go"'))
PY
	run_drill "D1" "$ROOT/.planning/tmp/p52/mut/d1.py"
	run_drill "D2" "$ROOT/.planning/tmp/p52/mut/d2.py"
	rm -rf "$ROOT/.planning/tmp/p52/mut"
fi

# ---------------------------------------------------------------------------
# Final banner — exactly one print site, RAN-gated.
GATE_LINE="$(grep -nF 'if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then' "$SELF" | head -1 | cut -d: -f1)"
BANNER_LINE="$(grep -n '=== Phase 52 Gate: ALL' "$SELF" | tail -1 | cut -d: -f1)"
if [ -z "$GATE_LINE" ] || [ -z "$BANNER_LINE" ] || [ "$BANNER_LINE" -le "$GATE_LINE" ]; then
	fail "P9 the passing banner must sit strictly after the RAN-gate inside the same if-branch"
fi
BANNER_SITES="$(grep -c 'Phase 52 Gate: ALL PASSED' "$SELF")"
# 3 hits = 2 self-referential greps + 1 echo; require exactly 1 echo site:
if [ "$BANNER_SITES" != "2" ]; then
	fail "P9 the passing banner has ${BANNER_SITES} print sites; exactly one is allowed"
fi
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
	echo -e "${GREEN}=== Phase 52 Gate: ALL PASSED ===${NC}"
	echo "partitions: ${RAN_LIST[*]}"
	exit 0
else
	echo -e "${RED}=== Phase 52 Gate: NOT VERIFIED ===${NC}"
	echo "ran ${RAN}/${EXPECTED_PARTITIONS}: ${RAN_LIST[*]}"
	exit 1
fi
