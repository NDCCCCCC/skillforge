#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# verify-phase-55.sh -- Phase 55 (摄取流水线前端视图) acceptance gate.
#
# Six partitions (P1..P6), each depositing evidence tokens via
# ok_partition; the ALL PASSED banner prints only when every partition ran
# and every token landed.
#
# Conventions follow verify-phase-52.sh: set -uo pipefail (NO errexit --
# probes are EXPECTED to fail), MSYS_NO_PATHCONV=1, mktemp WORK with an
# exit trap, begin/ok/done partition triple.
#
# ZERO WAIVER: a skipped/todo'd test is a broken gate, not a green one.
# No live Milvus / BGE / GLM dependency: every probe is vitest + grep.
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
EXPECTED_PARTITIONS=6
ONLY="${GATE_ONLY:-ALL}"

begin_partition() { echo "$(_pn_num "$1")" > "$WORK/ran/$(_pn_num "$1").begin"; }

ok_partition() {
	local pn="$1" log="$2"; shift 2
	local token missing=""
	[ -f "$log" ] || fail "partition ${pn}: probe log ${log} missing"
	[ -s "$log" ] || fail "partition ${pn}: probe log ${log} is empty"
	for token in "$@"; do
		grep -qF -- "$token" "$log" || missing="${missing} ${token}"
	done
	[ -z "$missing" ] || fail "partition ${pn} cannot deposit its success tokens: the probe log does not contain '${missing}' (required: $*)"
	: > "$WORK/ran/$(_pn_num "$pn").ok"
}

done_partition() {
	local pn="$1"; shift
	local num; num="$(_pn_num "$pn")"
	[ -f "$WORK/ran/$num.begin" ] || fail "partition ${pn} ran without a begin marker"
	[ -f "$WORK/ran/$num.ok" ] || fail "partition ${pn} finished without ok_partition depositing success tokens"
	RAN=$((RAN + 1))
	RAN_LIST+=("$pn")
	info "$*"
}

_pn_num() { local s="$1"; echo "${s#P}"; }

want_partition() {
	[ "$ONLY" = "ALL" ] && return 0
	case ",$ONLY," in *",$(_pn_num "$1"),"*|*",P$(_pn_num "$1"),"*) return 0 ;; esac
	return 1
}

require_tool() {
	command -v "$1" >/dev/null 2>&1 || fail "$2 requires $1 ($3)"
}

# ---------------------------------------------------------------------------
# P1 -- typecheck + store/admin wiring structure
run_p1() {
	begin_partition "P1"
	{
		echo "P1 begin"
		require_tool npx P1 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx tsc --noEmit ) >> "$WORK/p1.log" 2>&1
		echo "P1_TSC_RC=$?" >> "$WORK/p1.log"
		STORE="$ROOT/web/src/store/app-store.ts"
		grep -q 'ingestFocusJobId' "$STORE" && echo "P1-STORE-FOCUS" >> "$WORK/p1.log"
		grep -q 'setIngestFocusJobId' "$STORE" && echo "P1-STORE-FOCUS-SETTER" >> "$WORK/p1.log"
		UNION_COUNT="$(grep -c "| 'ingest'" "$STORE")"
		[ "$UNION_COUNT" -ge 2 ] && echo "P1-STORE-UNION-COUNT=2" >> "$WORK/p1.log"
		grep -q "adminSubroute === 'ingest'" "$ROOT/web/src/components/admin/AdminDashboard.tsx" && echo "P1-VIEWBODY-BRANCH" >> "$WORK/p1.log"
	} >> "$WORK/p1.log" 2>&1
	ok_partition "P1" "$WORK/p1.log" "P1_TSC_RC=0" "P1-STORE-FOCUS" "P1-STORE-FOCUS-SETTER" "P1-STORE-UNION-COUNT=2" "P1-VIEWBODY-BRANCH"
	done_partition "P1" "P1 typecheck + wiring ok"
}

# ---------------------------------------------------------------------------
# P2 -- lint (zero warnings)
run_p2() {
	begin_partition "P2"
	{
		echo "P2 begin"
		require_tool npx P2 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx eslint src --max-warnings=0 ) >> "$WORK/p2.log" 2>&1
		echo "P2_LINT_RC=$?" >> "$WORK/p2.log"
	} >> "$WORK/p2.log" 2>&1
	ok_partition "P2" "$WORK/p2.log" "P2_LINT_RC=0"
	done_partition "P2" "P2 lint ok"
}

# ---------------------------------------------------------------------------
# P3 -- ingest component suite + file inventory
run_p3() {
	begin_partition "P3"
	{
		echo "P3 begin"
		require_tool npx P3 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx vitest run src/components/admin/ingest ) >> "$WORK/p3.log" 2>&1
		echo "P3_VITEST_RC=$?" >> "$WORK/p3.log"
		for f in types.ts ingestApi.ts IngestJobTimeline.tsx IngestStepInspect.tsx IngestStepMatrix.tsx IngestLogStream.tsx IngestTab.tsx; do
			[ -f "$ROOT/web/src/components/admin/ingest/$f" ] && echo "P3-FILE-$f" >> "$WORK/p3.log"
		done
		for f in types.test.ts ingestApi.test.ts IngestJobTimeline.test.tsx IngestStepInspect.test.tsx IngestStepMatrix.test.tsx IngestLogStream.test.tsx IngestTab.test.tsx; do
			[ -f "$ROOT/web/src/components/admin/ingest/$f" ] && echo "P3-TEST-$f" >> "$WORK/p3.log"
		done
	} >> "$WORK/p3.log" 2>&1
	ok_partition "P3" "$WORK/p3.log" \
		"P3_VITEST_RC=0" \
		"P3-FILE-types.ts" "P3-FILE-ingestApi.ts" "P3-FILE-IngestJobTimeline.tsx" \
		"P3-FILE-IngestStepInspect.tsx" "P3-FILE-IngestStepMatrix.tsx" \
		"P3-FILE-IngestLogStream.tsx" "P3-FILE-IngestTab.tsx" \
		"P3-TEST-types.test.ts" "P3-TEST-ingestApi.test.ts" "P3-TEST-IngestJobTimeline.test.tsx" \
		"P3-TEST-IngestStepInspect.test.tsx" "P3-TEST-IngestStepMatrix.test.tsx" \
		"P3-TEST-IngestLogStream.test.tsx" "P3-TEST-IngestTab.test.tsx"
	done_partition "P3" "P3 ingest suite ok"
}

# ---------------------------------------------------------------------------
# P4 -- regression surface (nav count, upload handoff, retry surface) + BE reads
run_p4() {
	begin_partition "P4"
	{
		echo "P4 begin"
		require_tool npx P4 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx vitest run \
			src/components/admin/DocUpload.test.tsx \
			src/components/admin/pipeline/PipelineTab.test.tsx \
			src/components/admin/AdminDashboard.test.tsx ) >> "$WORK/p4.log" 2>&1
		echo "P4_VITEST_RC=$?" >> "$WORK/p4.log"
		require_tool go P4 "Go toolchain"
		( cd "$ROOT" && go test ./internal/server/ -run 'TestIngest' -count=1 ) >> "$WORK/p4.log" 2>&1
		echo "P4_GOTEST_RC=$?" >> "$WORK/p4.log"
		grep -qF '/api/admin/ingest/jobs"' "$ROOT/internal/server/server.go" && echo "P4-ROUTE-LIST" >> "$WORK/p4.log"
		grep -q 'func buildIngestJobDTO' "$ROOT/internal/server/admin_ingest.go" && echo "P4-SHARED-PROJECTION" >> "$WORK/p4.log"
	} >> "$WORK/p4.log" 2>&1
	ok_partition "P4" "$WORK/p4.log" "P4_VITEST_RC=0" "P4_GOTEST_RC=0" "P4-ROUTE-LIST" "P4-SHARED-PROJECTION"
	done_partition "P4" "P4 regression ok"
}

# ---------------------------------------------------------------------------
# P5 -- contract structure + copy verbatim + forbidden-surface scan
run_p5() {
	begin_partition "P5"
	ING="$ROOT/web/src/components/admin/ingest"
	{
		echo "P5 begin"
		grep -q 'admin-tab-ingest' "$ROOT/web/src/components/admin/AdminDashboard.tsx" && echo "P5-NAVENTRY" >> "$WORK/p5.log"
		grep -q 'FileStack' "$ROOT/web/src/components/admin/AdminDashboard.tsx" && echo "P5-ICON" >> "$WORK/p5.log"
		grep -q "label: '摄取任务'" "$ROOT/web/src/components/admin/AdminDashboard.tsx" && echo "P5-NAVLABEL" >> "$WORK/p5.log"
		grep -rq 'data-testid="ingest-tab"' "$ING" && echo "P5-TESTID-TAB" >> "$WORK/p5.log"
		grep -rq 'ingest-step-retry-' "$ING" && echo "P5-TESTID-RETRY" >> "$WORK/p5.log"
		grep -rq 'ingest-step-expand-' "$ING" && echo "P5-TESTID-EXPAND" >> "$WORK/p5.log"
		grep -rq '重试此步骤' "$ING" && echo "P5-COPY-RETRY" >> "$WORK/p5.log"
		grep -rq '已重新入队步骤' "$ING" && echo "P5-COPY-TOAST" >> "$WORK/p5.log"
		grep -q '已创建摄取任务，正在打开流水线视图…' "$ROOT/web/src/components/admin/DocUpload.tsx" && echo "P5-COPY-HANDOFF" >> "$WORK/p5.log"
		grep -rq '重试从上游最近成功步骤的产物继续' "$ING" && echo "P5-COPY-HINT" >> "$WORK/p5.log"
		grep -rq '确定性错误 — 不自动重试' "$ING" && echo "P5-COPY-DEAD" >> "$WORK/p5.log"
		grep -rq '本任务暂无过程日志' "$ING" && echo "P5-COPY-LOG-EMPTY" >> "$WORK/p5.log"
		if ! grep -q 'doc-upload-parsing' "$ROOT/web/src/components/admin/DocUpload.tsx"; then echo "P5-FAKESTATE-GONE" >> "$WORK/p5.log"; fi
		# The fake delay was a real timer call (`setTimeout(`). Prose in the
		# header comment explains the retired behaviour and must not trip it.
		if ! grep -q 'setTimeout(' "$ROOT/web/src/components/admin/DocUpload.tsx"; then echo "P5-NO-FAKE-DELAY" >> "$WORK/p5.log"; fi
		# `confirm(` as a CALL is forbidden; the header comment naming the rule
		# (`no window.confirm`) is documentation, not usage.
		if ! grep -rqE 'window\.confirm\(|confirm\(' "$ING"; then echo "P5-NO-CONFIRM" >> "$WORK/p5.log"; fi
		if ! grep -rq '全部重试' "$ING"; then echo "P5-NO-BATCH" >> "$WORK/p5.log"; fi
		# Raw hex is forbidden in the ingest components. The only allowed
		# exception is the shared StatusBadge tone palette, which this
		# directory never re-declares -- so a hit here means a local copy crept
		# in. Skip *.test.* (they legitimately quote expected values).
		if ! grep -rqE --include='*.tsx' --include='*.ts' --exclude='*.test.ts' --exclude='*.test.tsx' '#[0-9a-fA-F]{3,6}\b' "$ING"; then echo "P5-NO-HEX" >> "$WORK/p5.log"; fi
		if ! grep -rq 'EventSource' "$ING"; then echo "P5-POLLING-ONLY" >> "$WORK/p5.log"; fi
		( cd "$ROOT" && git diff HEAD -- web/package.json | grep -qE '^[+-]\s*"' ) && fail "P5 web/package.json must not gain dependencies"
		echo "P5-NO-DEPS" >> "$WORK/p5.log"
	} >> "$WORK/p5.log" 2>&1
	ok_partition "P5" "$WORK/p5.log" \
		"P5-NAVENTRY" "P5-ICON" "P5-NAVLABEL" \
		"P5-TESTID-TAB" "P5-TESTID-RETRY" "P5-TESTID-EXPAND" \
		"P5-COPY-RETRY" "P5-COPY-TOAST" "P5-COPY-HANDOFF" "P5-COPY-HINT" "P5-COPY-DEAD" "P5-COPY-LOG-EMPTY" \
		"P5-FAKESTATE-GONE" "P5-NO-FAKE-DELAY" \
		"P5-NO-CONFIRM" "P5-NO-BATCH" "P5-NO-HEX" "P5-POLLING-ONLY" "P5-NO-DEPS"
	done_partition "P5" "P5 contract ok"
}

# ---------------------------------------------------------------------------
# P6 -- zero waiver + full FE smoke
run_p6() {
	begin_partition "P6"
	ING="$ROOT/web/src/components/admin/ingest"
	{
		echo "P6 begin"
		if ! grep -rqE '\.skip\(|t\.skip|test\.todo|WAIVED' "$ING"; then echo "P6-ZERO-WAIVER-INGEST" >> "$WORK/p6.log"; fi
		if ! grep -rqE '\.skip\(|test\.todo|WAIVED' "$ROOT/web/src/components/admin/DocUpload.test.tsx"; then echo "P6-ZERO-WAIVER-UPLOAD" >> "$WORK/p6.log"; fi
		require_tool npx P6 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx vitest run --reporter=dot ) >> "$WORK/p6.log" 2>&1
		echo "P6_FULL_RC=$?" >> "$WORK/p6.log"
	} >> "$WORK/p6.log" 2>&1
	ok_partition "P6" "$WORK/p6.log" "P6-ZERO-WAIVER-INGEST" "P6-ZERO-WAIVER-UPLOAD" "P6_FULL_RC=0"
	done_partition "P6" "P6 zero-waiver + full FE suite ok"
}

# ---------------------------------------------------------------------------
want_partition "P1" && run_p1
want_partition "P2" && run_p2
want_partition "P3" && run_p3
want_partition "P4" && run_p4
want_partition "P5" && run_p5
want_partition "P6" && run_p6

# ---------------------------------------------------------------------------
# Final banner -- exactly one print site, RAN-gated.
GATE_LINE="$(grep -nF 'if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then' "$SELF" | head -1 | cut -d: -f1)"
BANNER_LINE="$(grep -n '=== Phase 55 Gate: ALL' "$SELF" | tail -1 | cut -d: -f1)"
if [ -z "$GATE_LINE" ] || [ -z "$BANNER_LINE" ] || [ "$BANNER_LINE" -le "$GATE_LINE" ]; then
	fail "banner must sit strictly after the RAN-gate inside the same if-branch"
fi
BANNER_SITES="$(grep -c 'Phase 55 Gate: ALL PASSED' "$SELF")"
if [ "$BANNER_SITES" != "2" ]; then
	fail "the passing banner has ${BANNER_SITES} print sites; exactly one is allowed"
fi
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
	echo -e "${GREEN}=== Phase 55 Gate: ALL PASSED ===${NC}"
	echo "partitions: ${RAN_LIST[*]}"
	exit 0
else
	echo -e "${RED}=== Phase 55 Gate: NOT VERIFIED ===${NC}"
	echo "ran ${RAN}/${EXPECTED_PARTITIONS}: ${RAN_LIST[*]}"
	exit 1
fi
