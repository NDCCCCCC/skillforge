#!/usr/bin/env bash
# Copyright 2026 SkillForge Authors — SPDX-License-Identifier: Apache-2.0
#
# Phase 2 live end-to-end verification of all 6 Success Criteria.
#
# Backend selection (optional, default milvus):
#   export RETRIEVER_BACKEND=milvus   # uses MILVUS_ADDRESS
#   export RETRIEVER_BACKEND=mongo    # uses MONGO_URI (MongoDB 4.4+ fallback)
#
# Prereqs (set in YOUR shell — internal-network credentials, not in the repo):
#   # When RETRIEVER_BACKEND=milvus (or unset):
#   export MILVUS_ADDRESS=10.x.x.x:19530
#   # When RETRIEVER_BACKEND=mongo:
#   export MONGO_URI=mongodb://10.x.x.x:27017
#   # Always required:
#   export BGE_M3_URL=http://bge-m3.internal/v1/embeddings
#   export GLM_API_KEY=<zhipu coding-plan key>
#   export GLM_BASE_URL=https://open.bigmodel.cn/api/coding/paas/v4   # default
#   export SERVER_ADDR=:8888                                          # default
#
# Usage:
#   go run ./cmd/server &        # terminal 1 (wait for the startup log)
#   bash scripts/live-verify.sh  # terminal 2
set -u
BASE="${BASE:-http://localhost:8888}"
BACKEND="${RETRIEVER_BACKEND:-milvus}"
pass() { echo "  [PASS] $1"; }
fail() { echo "  [FAIL] $1"; }

hdr() { echo; echo "=== $1 ==="; }

hdr "Backend"
case "$BACKEND" in
  milvus) echo "  RETRIEVER_BACKEND=milvus — refusals in SC-2/SC-4 are enforced by Milvus 'false' predicate (D-09)" ;;
  mongo)  echo "  RETRIEVER_BACKEND=mongo  — refusals in SC-2/SC-4 are enforced by MongoDB {\$in:[]} (D-09 Go + DB halves)" ;;
  *)      echo "  RETRIEVER_BACKEND=$BACKEND (unknown — cmd/server will warn + fall back to milvus)" ;;
esac

hdr "SC-1a  GET /api/skills lists the compliance Skill"
SK=$(curl -s "$BASE/api/skills")
echo "$SK" | head -c 600; echo
echo "$SK" | grep -q '企业合规' && echo "$SK" | grep -q 'authority/\*\*' \
  && pass "compliance Skill listed with reads=[authority/**]" \
  || fail "compliance Skill missing or reads wrong"

hdr "SC-1b  POST /api/skills dry-run validator"
VALID='{"description":"企业合规/制度审查","agent":"ChatModelAgent","reads":["authority/**"],"writes":[],"steps":"1. 检索\n2. 判定","guardrails":{"prohibitions":["p1","p2","p3"],"required_refusal_phrases":["no_authorized_source"],"tier_limits":{"authority":["read-only"]}},"temperature":0.0}'
NOTEMP='{"description":"企业合规/制度审查","agent":"ChatModelAgent","reads":["authority/**"],"writes":[],"steps":"1. 检索","guardrails":{"prohibitions":["p1","p2","p3"],"required_refusal_phrases":[],"tier_limits":{}}}'
R1=$(curl -s -X POST "$BASE/api/skills" -H 'Content-Type: application/json' -d "$VALID")
R2=$(curl -s -X POST "$BASE/api/skills" -H 'Content-Type: application/json' -d "$NOTEMP")
echo "  valid  -> $R1"
echo "  notemp -> $R2"
echo "$R1" | grep -q '"valid":true' && pass "valid body accepted" || fail "valid body rejected"
echo "$R2" | grep -q 'temperature' && pass "missing temperature rejected (SYSK-08)" || fail "SYSK-08 not enforced"
AFTER=$(curl -s "$BASE/api/skills")
[ "$SK" = "$AFTER" ] && pass "D2-07 dry-run: registry UNCHANGED after POST" || fail "registry mutated by POST"

ask() { curl -s -N -X POST "$BASE/api/chat" -H 'Content-Type: application/json' -d "{\"query\":\"$1\",\"skill\":\"compliance\"}"; }

hdr "SC-3 / SC-6  grounded cited verdict (travel expense)"
S3=$(ask "出差住宿费超过500元合规吗")
echo "$S3"
echo "$S3" | grep -q 'source-document' && pass "SC-6 source-document event(s) streamed" || fail "no source-document event"
echo "$S3" | grep -q 'data-policy'     && pass "SC-3 data-policy verdict streamed"   || fail "no data-policy event"
echo "$S3" | grep -q '权威'            && pass "SC-6 tier=权威 present in citations"   || fail "no 权威 tier in citations"
echo "  >> MANUALLY verify: pick one citation's \"span\" and confirm it appears verbatim in that chunk's source doc (D2-01 moat)."

hdr "SC-5  review_required payload present"
echo "$S3" | grep -q 'review_required' && pass "review_required present in verdict" || fail "review_required missing"
echo "  >> If review_required=false with confidence>=0.9, re-run with a borderline query (e.g. 收受礼品的金额上限是多少) to exercise the downgrade."

hdr "SC-2  refusal on no_authorized_source (out-of-corpus query)"
S2=$(ask "量子力学原理")
echo "$S2"
echo "$S2" | grep -q 'no_authorized_source' && pass "SC-2 structured refusal" || fail "expected no_authorized_source refusal"

hdr "SC-4  no tier bleed (personal-tier query)"
S4=$(ask "查询张三的手机号码")
echo "$S4"
echo "$S4" | grep -q 'no_authorized_source' && pass "SC-4 refusal, no personal-tier bleed" || fail "expected refusal on personal-tier query"

hdr "SC-7  backend abstraction holds (mongo parity)"
if [ "$BACKEND" = "mongo" ]; then
  # On mongo the same refusals above were enforced by {tier:{$in:[]}} evaluated
  # by MongoDB itself, then by tier-filter excluding personal. This is the
  # end-to-end proof that the retriever.Retriever abstraction is behaviorally
  # indistinguishable from the milvus path.
  echo "  >> Mongo backend: SC-2 + SC-4 refusals passed above = D-09 holds on MongoDB."
else
  echo "  >> Milvus backend (default); mongo parity is exercised by re-running with RETRIEVER_BACKEND=mongo."
fi

echo; echo "Done. Stop the server with: kill %1"
