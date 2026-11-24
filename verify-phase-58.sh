#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# verify-phase-58.sh -- Phase 58 (来源标注 wire) acceptance gate.
#
# Six partitions (P1..P6), each depositing evidence tokens via
# ok_partition; the ALL PASSED banner prints only when every partition ran
# and every token landed.
#
# The gate's subject is the PROV contract:
#   - Citation provenance fields are ADDITIVE (absent == vector, old shape
#     byte-identical)
#   - channel fusion aggregates by CHUNK PK (never by text) with RRF
#   - the data-retrieval frame carries server-derived counts only
#   - the FE never infers a channel and never fabricates a zero
#   - ROI-01: the numbers persist (audit_meta.retrieval_stats)
#
# Conventions follow verify-phase-52.sh: set -uo pipefail (NO errexit),
# MSYS_NO_PATHCONV=1, mktemp WORK + exit trap, begin/ok/done triple.
# ZERO WAIVER. No live Milvus / BGE / GLM dependency.
set -uo pipefail
export MSYS_NO_PATHCONV=1

ROOT="${GATE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
SELF="$ROOT/$(basename "${BASH_SOURCE[0]}")"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
fail() { echo -e "${RED}FAIL${NC} $1" | tee -a "$WORK/fails.log"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $1"; }

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
	[ -z "$missing" ] || fail "partition ${pn} cannot deposit its success tokens: missing'${missing}' (required: $*)"
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

require_tool() { command -v "$1" >/dev/null 2>&1 || fail "$2 requires $1 ($3)"; }

# ---------------------------------------------------------------------------
# P1 -- additive Citation provenance + build/vet
run_p1() {
	begin_partition "P1"
	{
		echo "P1 begin"
		require_tool go P1 "Go toolchain"
		( cd "$ROOT" && CGO_ENABLED=1 go build ./... ) >> "$WORK/p1.log" 2>&1
		echo "P1_BUILD_RC=$?" >> "$WORK/p1.log"
		( cd "$ROOT" && go vet ./... ) >> "$WORK/p1.log" 2>&1
		echo "P1_VET_RC=$?" >> "$WORK/p1.log"
		( cd "$ROOT" && go test ./internal/verdict/ -run 'TestCitationProvenanceIsAdditive' -count=1 ) >> "$WORK/p1.log" 2>&1
		echo "P1_ADDITIVE_RC=$?" >> "$WORK/p1.log"
		grep -q 'json:"origin,omitempty"' "$ROOT/internal/verdict/citation.go" && echo "P1-ORIGIN-OMITEMPTY" >> "$WORK/p1.log"
		grep -q 'json:"via,omitempty"' "$ROOT/internal/verdict/citation.go" && echo "P1-VIA-OMITEMPTY" >> "$WORK/p1.log"
		grep -q 'json:"channels,omitempty"' "$ROOT/internal/verdict/citation.go" && echo "P1-CHANNELS-OMITEMPTY" >> "$WORK/p1.log"
		grep -q 'type ViaPath struct' "$ROOT/internal/verdict/citation.go" && echo "P1-VIAPATH" >> "$WORK/p1.log"
	} >> "$WORK/p1.log" 2>&1
	ok_partition "P1" "$WORK/p1.log" "P1_BUILD_RC=0" "P1_VET_RC=0" "P1_ADDITIVE_RC=0" \
		"P1-ORIGIN-OMITEMPTY" "P1-VIA-OMITEMPTY" "P1-CHANNELS-OMITEMPTY" "P1-VIAPATH"
	done_partition "P1" "P1 additive citation provenance ok"
}

# ---------------------------------------------------------------------------
# P2 -- fusion: chunk-PK aggregation + RRF + hybrid raw channel set
run_p2() {
	begin_partition "P2"
	{
		echo "P2 begin"
		require_tool go P2 "Go toolchain"
		( cd "$ROOT" && go test ./internal/retriever/ -count=1 ) >> "$WORK/p2.log" 2>&1
		echo "P2_FUSE_RC=$?" >> "$WORK/p2.log"
		[ -f "$ROOT/internal/retriever/fuse.go" ] && echo "P2-FILE-fuse.go" >> "$WORK/p2.log"
		grep -q 'func FuseChannels' "$ROOT/internal/retriever/fuse.go" && echo "P2-FUSE" >> "$WORK/p2.log"
		grep -q 'RRFRankConstant = 60' "$ROOT/internal/retriever/fuse.go" && echo "P2-RRF-60" >> "$WORK/p2.log"
		grep -q 'ChannelHybrid' "$ROOT/internal/retriever/fuse.go" && echo "P2-HYBRID" >> "$WORK/p2.log"
		grep -qE 'Channels +\[\]string' "$ROOT/internal/retriever/retriever.go" && echo "P2-RAW-CHANNELS" >> "$WORK/p2.log"
		if ! grep -qE 'map\[string\]\*HybridResult' "$ROOT/internal/retriever/fuse.go"; then echo "P2-PK-KEYED" >> "$WORK/p2.log"; fi
	} >> "$WORK/p2.log" 2>&1
	ok_partition "P2" "$WORK/p2.log" "P2_FUSE_RC=0" "P2-FILE-fuse.go" "P2-FUSE" "P2-RRF-60" "P2-HYBRID" "P2-RAW-CHANNELS" "P2-PK-KEYED"
	done_partition "P2" "P2 fusion ok (chunk-PK keyed, RRF k=60, hybrid keeps the raw channel set)"
}

# ---------------------------------------------------------------------------
# P3 -- data-retrieval frame: server-derived counts + pre-refusal broadcast
run_p3() {
	begin_partition "P3"
	{
		echo "P3 begin"
		require_tool go P3 "Go toolchain"
		( cd "$ROOT" && go test ./internal/server/ -run 'TestBuildRetrievalStats' -count=1 ) >> "$WORK/p3.log" 2>&1
		echo "P3_STATS_RC=$?" >> "$WORK/p3.log"
		grep -q 'data-retrieval' "$ROOT/internal/wire/chunk.go" && echo "P3-WIRE-TYPE" >> "$WORK/p3.log"
		grep -q 'type RetrievalStats struct' "$ROOT/internal/wire/chunk.go" && echo "P3-STATS-STRUCT" >> "$WORK/p3.log"
		grep -q 'type RetrievalChannels struct' "$ROOT/internal/wire/chunk.go" && echo "P3-CHANNELS-STRUCT" >> "$WORK/p3.log"
		grep -q 'emitRetrievalStats(c, stats)' "$ROOT/internal/server/pipeline.go" && echo "P3-EMITTED" >> "$WORK/p3.log"
		EMIT_LINE="$(grep -n 'emitRetrievalStats(c, stats)' "$ROOT/internal/server/pipeline.go" | head -1 | cut -d: -f1)"
		REFUSAL_LINE="$(grep -n 'if len(chunks) == 0' "$ROOT/internal/server/pipeline.go" | head -1 | cut -d: -f1)"
		if [ -n "$EMIT_LINE" ] && [ -n "$REFUSAL_LINE" ] && [ "$EMIT_LINE" -lt "$REFUSAL_LINE" ]; then echo "P3-BEFORE-REFUSAL" >> "$WORK/p3.log"; fi
	} >> "$WORK/p3.log" 2>&1
	ok_partition "P3" "$WORK/p3.log" "P3_STATS_RC=0" "P3-WIRE-TYPE" "P3-STATS-STRUCT" "P3-CHANNELS-STRUCT" "P3-EMITTED" "P3-BEFORE-REFUSAL"
	done_partition "P3" "P3 data-retrieval ok (registered chunk type; counts from the fused set; emitted before the refusal gates)"
}

# ---------------------------------------------------------------------------
# P4 -- FE: no inference, no fabricated zero, mirrors in lockstep
run_p4() {
	begin_partition "P4"
	{
		echo "P4 begin"
		require_tool npx P4 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx tsc --noEmit ) >> "$WORK/p4.log" 2>&1
		echo "P4_TSC_RC=$?" >> "$WORK/p4.log"
		( cd "$ROOT/web" && npx vitest run src/__tests__/OriginBadge.test.tsx src/__tests__/RetrievalPart.test.tsx src/__tests__/RefusalRetrievalStats.test.tsx src/__tests__/AuditRowChannels.test.tsx src/types --reporter=dot ) >> "$WORK/p4.log" 2>&1
		echo "P4_VITEST_RC=$?" >> "$WORK/p4.log"
		grep -q "'ontology_anchor'" "$ROOT/web/src/types/wire-types.ts" && echo "P4-FE-DECISION-MIRROR" >> "$WORK/p4.log"
		grep -q "'data-retrieval'" "$ROOT/web/src/types/wire-types.ts" && echo "P4-FE-RETRIEVAL-MIRROR" >> "$WORK/p4.log"
		grep -q 'origin?:' "$ROOT/web/src/types/verdict.ts" && echo "P4-FE-CITATION-ORIGIN" >> "$WORK/p4.log"
		grep -q 'export function isRetrievalStats' "$ROOT/web/src/types/verdict.ts" && echo "P4-FE-GUARD" >> "$WORK/p4.log"
		grep -q 'function resolveCitationOrigin' "$ROOT/web/src/components/chat/parts/OriginBadge.tsx" && echo "P4-FE-RESOLVER" >> "$WORK/p4.log"
		grep -q 'data-testid="origin-badge"' "$ROOT/web/src/components/chat/parts/OriginBadge.tsx" && echo "P4-FE-BADGE" >> "$WORK/p4.log"
		grep -q 'data-testid="retrieval-summary"' "$ROOT/web/src/components/chat/parts/RetrievalPart.tsx" && echo "P4-FE-SUMMARY" >> "$WORK/p4.log"
		grep -q 'data-testid="refusal-retrieval-stats"' "$ROOT/web/src/components/center/RefusalCard.tsx" && echo "P4-FE-REFUSAL-STATS" >> "$WORK/p4.log"
		grep -q 'data-testid="audit-row-channels"' "$ROOT/web/src/components/AuditTimelineTab.tsx" && echo "P4-FE-AUDIT-CHANNELS" >> "$WORK/p4.log"
	} >> "$WORK/p4.log" 2>&1
	ok_partition "P4" "$WORK/p4.log" "P4_TSC_RC=0" "P4_VITEST_RC=0" \
		"P4-FE-DECISION-MIRROR" "P4-FE-RETRIEVAL-MIRROR" "P4-FE-CITATION-ORIGIN" "P4-FE-GUARD" \
		"P4-FE-RESOLVER" "P4-FE-BADGE" "P4-FE-SUMMARY" "P4-FE-REFUSAL-STATS" "P4-FE-AUDIT-CHANNELS"
	done_partition "P4" "P4 FE ok (D15 mirrors, single origin resolver, guarded stats, refusal + audit surfaces)"
}

# ---------------------------------------------------------------------------
# P5 -- ROI-01 persistence: audit_meta.retrieval_stats
run_p5() {
	begin_partition "P5"
	{
		echo "P5 begin"
		require_tool go P5 "Go toolchain"
		( cd "$ROOT" && go test ./internal/store/sqlite/ -run 'TestRetrievalStatsBySession' -count=1 ) >> "$WORK/p5.log" 2>&1
		echo "P5_ROI_RC=$?" >> "$WORK/p5.log"
		grep -q 'retrieval_stats' "$ROOT/internal/store/sqlite/schema.go" && echo "P5-DDL-COLUMN" >> "$WORK/p5.log"
		grep -qF 'func (s *SQLiteStore) RetrievalStatsBySession(' "$ROOT/internal/store/sqlite/audit.go" && echo "P5-READER" >> "$WORK/p5.log"
		grep -q 'meta.RetrievalStats = retrievalStats' "$ROOT/internal/server/pipeline.go" && echo "P5-WRITER" >> "$WORK/p5.log"
		grep -qE 'Channels +\*retrievalChannelsDTO' "$ROOT/internal/server/audit_api.go" && echo "P5-AUDIT-PROJECTION" >> "$WORK/p5.log"
		if ! grep -q 'Store.PutAudit(ctx' "$ROOT/internal/server/retrieval_stats.go"; then echo "P5-NO-SECOND-AUDIT-ROW" >> "$WORK/p5.log"; fi
	} >> "$WORK/p5.log" 2>&1
	ok_partition "P5" "$WORK/p5.log" "P5_ROI_RC=0" "P5-DDL-COLUMN" "P5-READER" "P5-WRITER" "P5-AUDIT-PROJECTION" "P5-NO-SECOND-AUDIT-ROW"
	done_partition "P5" "P5 ROI ok (stats persist in audit_meta, read back per session, projected to the audit list; no second audit row)"
}

# ---------------------------------------------------------------------------
# P6 -- zero waiver + full suites
run_p6() {
	begin_partition "P6"
	{
		echo "P6 begin"
		if ! grep -rqE '\.skip\(|t\.Skip|test\.todo|WAIVED' "$ROOT/internal/retriever/fuse_test.go" "$ROOT/internal/verdict/entities_test.go" "$ROOT/internal/server/retrieval_stats_test.go" "$ROOT/web/src/__tests__/OriginBadge.test.tsx" "$ROOT/web/src/__tests__/RetrievalPart.test.tsx"; then echo "P6-ZERO-WAIVER" >> "$WORK/p6.log"; fi
		require_tool go P6 "Go toolchain"
		( cd "$ROOT" && CGO_ENABLED=1 go test ./internal/... -count=1 ) >> "$WORK/p6.log" 2>&1
		echo "P6_GO_FULL_RC=$?" >> "$WORK/p6.log"
		require_tool npx P6 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx vitest run --reporter=dot ) >> "$WORK/p6.log" 2>&1
		echo "P6_FE_FULL_RC=$?" >> "$WORK/p6.log"
	} >> "$WORK/p6.log" 2>&1
	ok_partition "P6" "$WORK/p6.log" "P6-ZERO-WAIVER" "P6_GO_FULL_RC=0" "P6_FE_FULL_RC=0"
	done_partition "P6" "P6 zero-waiver + full suites ok"
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
BANNER_LINE="$(grep -n '=== Phase 58 Gate: ALL' "$SELF" | tail -1 | cut -d: -f1)"
if [ -z "$GATE_LINE" ] || [ -z "$BANNER_LINE" ] || [ "$BANNER_LINE" -le "$GATE_LINE" ]; then
	fail "banner must sit strictly after the RAN-gate inside the same if-branch"
fi
BANNER_SITES="$(grep -c 'Phase 58 Gate: ALL PASSED' "$SELF")"
if [ "$BANNER_SITES" != "2" ]; then
	fail "the passing banner has ${BANNER_SITES} print sites; exactly one is allowed"
fi
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
	echo -e "${GREEN}=== Phase 58 Gate: ALL PASSED ===${NC}"
	echo "partitions: ${RAN_LIST[*]}"
	exit 0
else
	echo -e "${RED}=== Phase 58 Gate: NOT VERIFIED ===${NC}"
	echo "ran ${RAN}/${EXPECTED_PARTITIONS}: ${RAN_LIST[*]}"
	exit 1
fi
