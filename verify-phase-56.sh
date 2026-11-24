#!/usr/bin/env bash
# verify-phase-56.sh -- Phase 56 (operation panel usability) acceptance gate.
# PANEL-01 rule CRUD/matching / PANEL-02 detail IA / PANEL-03 write-hole +
# highlight + empty guidance. Zero waiver; no live Milvus/BGE deps.
set -euo pipefail
cd "$(dirname "$0")"

pass=0; fail=0
ok()   { echo "--- PASS: $1"; pass=$((pass+1)); }
bad()  { echo "--- FAIL: $1"; fail=$((fail+1)); }

echo "=== P1: CGO build + vet ==="
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go vet ./...
ok "P1 build+vet (CGO_ENABLED=1)"

echo "=== P2: package suites ==="
go test ./internal/agent/hitl/ ./internal/store/sqlite/ ./internal/server/ -count=1
ok "P2 hitl+sqlite+server suites green"

echo "=== P3: named gates (56 plan deliverables) ==="
# a. 56-01: rule table migration + three-layer scope + store surface
go test ./internal/store/sqlite/ -run 'TestApprovalRules|TestMigrateLegacyApprovalRules|TestRuleHit|TestRuleEnabledAndExpiry' -count=1 \
  && ok "a. rule schema/migration/toggle/expiry pinned (56-01)" \
  || bad "a. rule schema/migration/toggle/expiry"
# b. 56-02: match engine (exact/prefix/template x three-scope priority,
#    fail-closed, smuggled-key 422)
go test ./internal/store/sqlite/ -run 'TestMatchRuleForArgs' -count=1 \
  && ok "b. match engine tiers/priority/fail-closed pinned" \
  || bad "b. match engine"
# c. 56-02: approve-and-remember wiring + auto-pass + rule-matched frame
go test ./internal/server/ -run 'TestApprovalRemember|TestApprovalOneOff|TestRuleMatchedFrameShape|TestRuleAutoPass' -count=1 \
  && ok "c. approve-and-remember wiring + auto-pass + frame shape" \
  || bad "c. approve-and-remember wiring"
# d. 56-04: resume write-hole closure (action_resumed_answer marker)
go test ./internal/server/ -run '^TestResumeAuditMarkerWritten$' -count=1 \
  && ok "d. resume audit marker written (write-hole closed)" \
  || bad "d. resume audit marker"
# e. 56-02: SQL rule matcher adapters (exact/prefix/store-error-no-match)
go test ./internal/agent/hitl/ -run 'TestSqlRuleMatcher' -count=1 \
  && ok "e. SQL rule matcher adapters" \
  || bad "e. SQL rule matcher"

echo "=== P4: FE vitest (56-03 panels + data layer) ==="
(cd web && npx vitest run \
  src/lib/sessions.test.ts \
  src/components/ApprovalRulesPanel.test.tsx \
  src/components/ApprovalCard.test.tsx \
  src/components/PlanRulesPanel.test.tsx \
  src/__tests__/DetailPanel.test.tsx \
  src/__tests__/StepTree.test.tsx \
  src/__tests__/AgentInspector.test.tsx \
  src/__tests__/citations.test.tsx \
  src/__tests__/ChunkSourceView.test.tsx \
  src/components/AuditTimelineTab.test.tsx \
  src/components/PlanReplayView.test.tsx 2>&1)
ok "P4 FE panels+data-layer vitest green"

echo "=== P5: full internal smoke (not deterministic evidence) ==="
go test ./internal/... -count=1
ok "P5 ./internal/... full smoke"

echo "=== P6: FE full suite (56-04 PANEL-02/03 deliverables included) ==="
(cd web && npx vitest run --reporter=dot 2>&1 | tail -20)
ok "P6 web full vitest suite green"

echo ""
echo "=== verify-phase-56 summary: PASS=$pass FAIL=$fail ==="
if [ "$fail" -ne 0 ]; then
  echo "GATE: RED -- Phase 56 not closed (zero-waiver discipline)"
  exit 1
fi
echo "GATE: GREEN -- Phase 56 operation panel usability closed"
