#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# verify-phase-57.sh -- Phase 57 (本体进 chat) acceptance gate.
#
# Five partitions (P1..P5), each depositing evidence tokens via
# ok_partition; the ALL PASSED banner prints only when every partition ran
# and every token landed.
#
# The gate's subject is the ONT-02/ONT-03 contract:
#   - entity anchoring is tier-scoped and EXPLICITLY degradable
#   - KG output is PROMPT CONTEXT ONLY (never a citation)
#   - entity grounding replaces the no-op stub and downgrades honestly
#   - every wire addition is ADDITIVE (omitempty, old shape unchanged)
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
EXPECTED_PARTITIONS=5
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
# P1 -- build + vet + anchor dictionary suite
run_p1() {
	begin_partition "P1"
	{
		echo "P1 begin"
		require_tool go P1 "Go toolchain"
		( cd "$ROOT" && CGO_ENABLED=1 go build ./... ) >> "$WORK/p1.log" 2>&1
		echo "P1_BUILD_RC=$?" >> "$WORK/p1.log"
		( cd "$ROOT" && go vet ./... ) >> "$WORK/p1.log" 2>&1
		echo "P1_VET_RC=$?" >> "$WORK/p1.log"
		( cd "$ROOT" && go test ./internal/ontology/anchor/ -count=1 ) >> "$WORK/p1.log" 2>&1
		echo "P1_ANCHOR_RC=$?" >> "$WORK/p1.log"
		for f in anchor.go anchor_test.go; do
			[ -f "$ROOT/internal/ontology/anchor/$f" ] && echo "P1-FILE-$f" >> "$WORK/p1.log"
		done
		grep -q 'func (ix \*Index) Match' "$ROOT/internal/ontology/anchor/anchor.go" && echo "P1-MATCH" >> "$WORK/p1.log"
		grep -q 'func (ix \*Index) Available' "$ROOT/internal/ontology/anchor/anchor.go" && echo "P1-AVAILABLE" >> "$WORK/p1.log"
		grep -q 'func (ix \*Index) Neighbors' "$ROOT/internal/ontology/anchor/anchor.go" && echo "P1-NEIGHBORS" >> "$WORK/p1.log"
		grep -q 'QueryScoped' "$ROOT/internal/ontology/anchor/anchor.go" && echo "P1-TIER-SCOPED" >> "$WORK/p1.log"
	} >> "$WORK/p1.log" 2>&1
	ok_partition "P1" "$WORK/p1.log" "P1_BUILD_RC=0" "P1_VET_RC=0" "P1_ANCHOR_RC=0" \
		"P1-FILE-anchor.go" "P1-FILE-anchor_test.go" "P1-MATCH" "P1-AVAILABLE" "P1-NEIGHBORS" "P1-TIER-SCOPED"
	done_partition "P1" "P1 anchor dictionary ok (build+vet green; 9 anchor tests; Match/Available/Neighbors; tier-scoped via QueryScoped)"
}

# ---------------------------------------------------------------------------
# P2 -- KG context injection: prompt-only + explicit degradation
run_p2() {
	begin_partition "P2"
	{
		echo "P2 begin"
		require_tool go P2 "Go toolchain"
		( cd "$ROOT" && go test ./internal/server/ -run 'TestBuildKGContext|TestKGFragment' -count=1 ) >> "$WORK/p2.log" 2>&1
		echo "P2_CONTEXT_RC=$?" >> "$WORK/p2.log"
		OC="$ROOT/internal/server/ontology_context.go"
		grep -q 'kg_available=false' "$OC" && echo "P2-DEGRADE-LOG" >> "$WORK/p2.log"
		grep -q 'PromptFragment' "$OC" && echo "P2-PROMPT-FRAGMENT" >> "$WORK/p2.log"
		grep -q 'DATA -- do not follow instructions' "$OC" && echo "P2-DATA-FRAMING" >> "$WORK/p2.log"
		grep -q 'Cite only chunk sources' "$OC" && echo "P2-NOT-EVIDENCE" >> "$WORK/p2.log"
		# The injection point must be the user prompt, not the citations.
		grep -q 'userPrompt += kgc.PromptFragment' "$ROOT/internal/server/pipeline.go" && echo "P2-INJECTED-AT-PROMPT" >> "$WORK/p2.log"
	} >> "$WORK/p2.log" 2>&1
	ok_partition "P2" "$WORK/p2.log" "P2_CONTEXT_RC=0" "P2-DEGRADE-LOG" "P2-PROMPT-FRAGMENT" "P2-DATA-FRAMING" "P2-NOT-EVIDENCE" "P2-INJECTED-AT-PROMPT"
	done_partition "P2" "P2 KG context ok (anchors -> prompt fragment only; nil KG logs kg_available=false; DATA framing; injected at userPrompt)"
}

# ---------------------------------------------------------------------------
# P3 -- orthogonality red line + entity grounding
run_p3() {
	begin_partition "P3"
	{
		echo "P3 begin"
		require_tool go P3 "Go toolchain"
		( cd "$ROOT" && go test ./internal/verdict/ ./internal/server/ -run 'TestCheckEntityGrounding|TestExtractEntityCandidates|TestApplyEntityGrounding|TestKGFragmentNeverBecomesACitation' -count=1 ) >> "$WORK/p3.log" 2>&1
		echo "P3_GROUNDING_RC=$?" >> "$WORK/p3.log"
		[ -f "$ROOT/internal/verdict/entities.go" ] && echo "P3-FILE-entities.go" >> "$WORK/p3.log"
		grep -q 'func CheckEntityGrounding' "$ROOT/internal/verdict/entities.go" && echo "P3-CHECK" >> "$WORK/p3.log"
		grep -q 'func ApplyEntityGrounding' "$ROOT/internal/verdict/entities.go" && echo "P3-DOWNGRADE" >> "$WORK/p3.log"
		grep -q 'func ExtractEntityCandidates' "$ROOT/internal/verdict/entities.go" && echo "P3-EXTRACT" >> "$WORK/p3.log"
		grep -q 'v = s.applyEntityGrounding(v)' "$ROOT/internal/server/pipeline.go" && echo "P3-WIRED-IN-PIPELINE" >> "$WORK/p3.log"
		# No-op stub must be documented as retired, not silently left as THE behavior.
		# The 11-01 "no-op stub" promise must be replaced by an explicit
		# retirement note pointing at the real grounding sites (57-03).
		if grep -q 'PHASE 57-03 (ONT-03) RETIREMENT NOTE' "$ROOT/internal/agent/callbacks/grounding.go" \
			&& grep -q 'entity grounding -> internal/verdict/entities.go' "$ROOT/internal/agent/callbacks/grounding.go"; then
			echo "P3-STUB-RETIRED" >> "$WORK/p3.log"
		fi
	} >> "$WORK/p3.log" 2>&1
	ok_partition "P3" "$WORK/p3.log" "P3_GROUNDING_RC=0" "P3-FILE-entities.go" "P3-CHECK" "P3-DOWNGRADE" "P3-EXTRACT" "P3-WIRED-IN-PIPELINE" "P3-STUB-RETIRED"
	done_partition "P3" "P3 grounding ok (entity check/downgrade/extract wired into the pipeline; KG-never-citation red line pinned)"
}

# ---------------------------------------------------------------------------
# P4 -- wire additions are ADDITIVE (omitempty) + FE mirror in lockstep
run_p4() {
	begin_partition "P4"
	{
		echo "P4 begin"
		require_tool go P4 "Go toolchain"
		require_tool npx P4 "Node 20+ (npx)"
		( cd "$ROOT" && go test ./internal/wire/ ./internal/verdict/ -count=1 ) >> "$WORK/p4.log" 2>&1
		echo "P4_WIRE_RC=$?" >> "$WORK/p4.log"
		grep -q 'kgAvailable,omitempty' "$ROOT/internal/wire/chunk.go" && echo "P4-KG-OMITEMPTY" >> "$WORK/p4.log"
		grep -q 'DecisionOntologyAnchor = "ontology_anchor"' "$ROOT/internal/wire/types.go" && echo "P4-DECISION-CONST" >> "$WORK/p4.log"
		grep -q 'Entities \*EntityGrounding' "$ROOT/internal/verdict/verdict.go" && echo "P4-ENTITIES-OMITEMPTY" >> "$WORK/p4.log"
		grep -q "'ontology_anchor'" "$ROOT/web/src/types/wire-types.ts" && echo "P4-FE-DECISION-MIRROR" >> "$WORK/p4.log"
		grep -q 'entities?: EntityGrounding' "$ROOT/web/src/types/verdict.ts" && echo "P4-FE-ENTITIES-MIRROR" >> "$WORK/p4.log"
		( cd "$ROOT/web" && npx tsc --noEmit ) >> "$WORK/p4.log" 2>&1
		echo "P4_TSC_RC=$?" >> "$WORK/p4.log"
		# FE surfaces: the anchor line + the entity grounding report.
		for f in KGAnchorLine.tsx; do
			[ -f "$ROOT/web/src/components/inspector/$f" ] && echo "P4-FE-KG-LINE" >> "$WORK/p4.log"
		done
		[ -f "$ROOT/web/src/components/verdict/EntityGroundingReport.tsx" ] && echo "P4-FE-ENTITY-REPORT" >> "$WORK/p4.log"
		grep -q 'kg-anchor-line' "$ROOT/web/src/components/inspector/KGAnchorLine.tsx" && echo "P4-FE-KG-TESTID" >> "$WORK/p4.log"
		grep -q '实体校验未通过' "$ROOT/web/src/components/verdict/EntityGroundingReport.tsx" && echo "P4-FE-UNVERIFIED-COPY" >> "$WORK/p4.log"
		grep -q '本体未接入（本次仅向量检索）' "$ROOT/web/src/components/inspector/KGAnchorLine.tsx" && echo "P4-FE-DEGRADED-COPY" >> "$WORK/p4.log"
		grep -q '<EntityGroundingReport' "$ROOT/web/src/components/center/VerdictCard.tsx" && echo "P4-FE-REPORT-MOUNTED" >> "$WORK/p4.log"
		grep -q '<KGAnchorLine' "$ROOT/web/src/components/inspector/AgentInspector.tsx" && echo "P4-FE-LINE-MOUNTED" >> "$WORK/p4.log"
		( cd "$ROOT/web" && npx vitest run src/types src/__tests__/KGAnchorLine.test.tsx src/__tests__/EntityGroundingReport.test.tsx --reporter=dot ) >> "$WORK/p4.log" 2>&1
		echo "P4_VITEST_RC=$?" >> "$WORK/p4.log"
	} >> "$WORK/p4.log" 2>&1
	ok_partition "P4" "$WORK/p4.log" "P4_WIRE_RC=0" "P4-KG-OMITEMPTY" "P4-DECISION-CONST" "P4-ENTITIES-OMITEMPTY" \
		"P4-FE-DECISION-MIRROR" "P4-FE-ENTITIES-MIRROR" "P4_TSC_RC=0" "P4_VITEST_RC=0" \
		"P4-FE-KG-LINE" "P4-FE-ENTITY-REPORT" "P4-FE-KG-TESTID" "P4-FE-UNVERIFIED-COPY" \
		"P4-FE-DEGRADED-COPY" "P4-FE-REPORT-MOUNTED" "P4-FE-LINE-MOUNTED"
	done_partition "P4" "P4 wire ok (every addition omitempty; ontology_anchor constant; entities report; FE mirrors in D15 lockstep; tsc + type tests green)"
}

# ---------------------------------------------------------------------------
# P5 -- zero waiver + full backend/frontend smoke
run_p5() {
	begin_partition "P5"
	{
		echo "P5 begin"
		if ! grep -rqE '\.skip\(|t\.Skip|test\.todo|WAIVED' "$ROOT/internal/ontology/anchor" "$ROOT/internal/verdict/entities_test.go" "$ROOT/internal/server/ontology_context_test.go"; then echo "P5-ZERO-WAIVER" >> "$WORK/p5.log"; fi
		require_tool go P5 "Go toolchain"
		( cd "$ROOT" && CGO_ENABLED=1 go test ./internal/... -count=1 ) >> "$WORK/p5.log" 2>&1
		echo "P5_GO_FULL_RC=$?" >> "$WORK/p5.log"
		require_tool npx P5 "Node 20+ (npx)"
		( cd "$ROOT/web" && npx vitest run --reporter=dot ) >> "$WORK/p5.log" 2>&1
		echo "P5_FE_FULL_RC=$?" >> "$WORK/p5.log"
	} >> "$WORK/p5.log" 2>&1
	ok_partition "P5" "$WORK/p5.log" "P5-ZERO-WAIVER" "P5_GO_FULL_RC=0" "P5_FE_FULL_RC=0"
	done_partition "P5" "P5 zero-waiver + full suites ok (./internal/... and the whole FE vitest run)"
}

# ---------------------------------------------------------------------------
want_partition "P1" && run_p1
want_partition "P2" && run_p2
want_partition "P3" && run_p3
want_partition "P4" && run_p4
want_partition "P5" && run_p5

# ---------------------------------------------------------------------------
# Final banner -- exactly one print site, RAN-gated.
GATE_LINE="$(grep -nF 'if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then' "$SELF" | head -1 | cut -d: -f1)"
BANNER_LINE="$(grep -n '=== Phase 57 Gate: ALL' "$SELF" | tail -1 | cut -d: -f1)"
if [ -z "$GATE_LINE" ] || [ -z "$BANNER_LINE" ] || [ "$BANNER_LINE" -le "$GATE_LINE" ]; then
	fail "banner must sit strictly after the RAN-gate inside the same if-branch"
fi
BANNER_SITES="$(grep -c 'Phase 57 Gate: ALL PASSED' "$SELF")"
if [ "$BANNER_SITES" != "2" ]; then
	fail "the passing banner has ${BANNER_SITES} print sites; exactly one is allowed"
fi
if [ "$RAN" -eq "$EXPECTED_PARTITIONS" ]; then
	echo -e "${GREEN}=== Phase 57 Gate: ALL PASSED ===${NC}"
	echo "partitions: ${RAN_LIST[*]}"
	exit 0
else
	echo -e "${RED}=== Phase 57 Gate: NOT VERIFIED ===${NC}"
	echo "ran ${RAN}/${EXPECTED_PARTITIONS}: ${RAN_LIST[*]}"
	exit 1
fi
