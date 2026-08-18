---
phase: 10
slug: p8-1
status: draft
nyquist_compliant: true
created: 2026-08-18
revised: 2026-08-18
revision_note: "Regenerated after checker B1-B6 + W1-W8 fixes; test names map to ACTUAL names in the revised 10-01..10-04 plans; Wave 0 rows removed (no Wave 0 plan exists in this phase)."
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (BE)** | `go test` 1.22+ with `testify/assert` + `internal/testdata` golden fixtures |
| **Framework (FE)** | vitest + @testing-library/react |
| **Config file** | None — existing infrastructure ships with project (Phase 3 SC-6 + Phase 4 vitest 280/280 + Phase 8 eino v0.9.14 hermetic four-gate) |
| **Quick run command (BE)** | `go test ./internal/history/... ./internal/server/... -count=1` |
| **Quick run command (FE)** | `cd web && npx vitest run --reporter=basic` |
| **Full suite command (BE)** | `go test ./... -count=1` |
| **Full suite command (FE)** | `cd web && npx vitest run` |
| **Type-check (FE)** | `cd web && npx tsc --noEmit` |
| **Build (FE)** | `cd web && npx vite build` |
| **Spike gate command** | `go run ./cmd/spike-history --via-pipeline --summarize --baseline-lock=eval/frozen/compliance.lock.json --out=reports/ --cost-limit=5.00` (Phase 10 runs via `--via-pipeline`; legacy direct path remains default for Phase 9 reproducibility) |
| **Estimated runtime (quick)** | ~30s |
| **Estimated runtime (full suite)** | ~3 min (hermetic) |
| **Estimated runtime (spike gate)** | ~75 min (live GLM-5.2 + Milvus via `--via-pipeline` path) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/history/... ./internal/server/... -count=1` + `cd web && npx vitest run --reporter=basic`
- **After every plan wave:** Run full suite (`go test ./... -count=1` + `cd web && npx vitest run`)
- **Before `/gsd:verify-work`:** Full suite green + spike gate exit 0
- **Max feedback latency:** 90s (BE full suite) / 60s (FE vitest) / 75min (spike live gate)

---

## Per-Task Verification Map

Test names below are the ACTUAL test function names present in the revised 10-01..10-04 plans (2026-08-18 revision).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|--------|
| 10-01-T1a | 01 | 1 | MEM-04 | T-10-01 | `RenderConclusion` deterministic (Answer / ComplianceStatus / malformed) | unit | `go test ./internal/history/... -run TestRenderConclusion -v -count=1` | ⬜ pending |
| 10-01-T1b | 01 | 1 | MEM-04 | T-10-01 | `RenderRefusal` user-facing refusal rendering (matches chat.go refusalText semantics) | unit | `go test ./internal/history/... -run TestRenderRefusal -v -count=1` | ⬜ pending |
| 10-01-T1c | 01 | 1 | MEM-04 | T-10-02b | Project() renders verdictJSON via RenderConclusion; zero RenderRefusal references (B3 gate) | unit + grep | `go test ./internal/history/... -run TestProject_FixturesStillGreen -v -count=1` and `grep -c RenderRefusal internal/history/projection.go` == 0 | ⬜ pending |
| 10-01-T2a | 01 | 1 | MEM-04 | T-10-02 | Neutral placeholder zero refusal-semantic words (canonical 11-entry list) | unit | `go test ./internal/history/... -run TestNeutralPlaceholder_NoRefusalSemanticWords -count=1` | ⬜ pending |
| 10-01-T2b | 01 | 1 | MEM-04 | T-10-02 | `IsRefusalContent` table-driven over 5 reasons + 3 non-refusal | unit | `go test ./internal/history/... -run TestIsRefusalContent -v -count=1` | ⬜ pending |
| 10-01-T2c | 01 | 1 | MEM-05 | T-10-03 | Budget-PRIMARY window: budget>0 keeps MORE than 3 pairs (B1 regression) | unit | `go test ./internal/history/... -run TestTruncateToCharBudget_BudgetPrimaryKeepsMoreThanThreePairs -count=1` | ⬜ pending |
| 10-01-T2d | 01 | 1 | MEM-05 | T-10-03 | Oldest-first drop + SafeguardFloor | unit | `go test ./internal/history/... -run 'TestTruncateToCharBudget_OldestFirstDrop|TestTruncateToCharBudget_SafeguardFloor' -count=1` | ⬜ pending |
| 10-01-T2e | 01 | 1 | MEM-05 | T-10-03 | Budget<=0 falls back to D9-B3 6-message ceiling (FallbackMessageCeiling) | unit | `go test ./internal/history/... -run TestTruncateToCharBudget_ZeroBudgetLegacyFallback -count=1` | ⬜ pending |
| 10-01-T2f | 01 | 1 | MEM-04 | T-10-02 | 8 exported fixtures green (D9-B4→D10-B3 evolution documented) | unit | `go test ./internal/history/... -run TestProject_FixturesStillGreen -count=1` | ⬜ pending |
| 10-01-T3a | 01 | 1 | MEM-04 | T-10-04 | `RefusalClassifierInput` has exactly 2 fields, no `History` | unit | `go test ./internal/server/classifier/... -run TestRefusalClassifierInput_NoHistoryField -count=1` | ⬜ pending |
| 10-01-T3b | 01 | 1 | MEM-04 | T-10-04 | `Valid()` table-driven (Decision enum + Confidence range + ConfidenceFloor) | unit | `go test ./internal/server/classifier/... -run TestRefusalClassifierOutput_Valid -count=1` | ⬜ pending |
| 10-01-T3c | 01 | 1 | MEM-04 | T-10-04 | Classifier types.go does NOT import history or store (D10-A1) | unit | `go test ./internal/server/classifier/... -run TestClassifierImports_NoHistoryPackage -count=1` | ⬜ pending |
| 10-02-T1a | 02 | 2 | MEM-01 | T-10-05 | Rewriter happy path + pass-through + timeout + malformed-JSON + 4k trim | unit | `go test ./internal/server/rewrite/... -v -count=1` | ⬜ pending |
| 10-02-T1b | 02 | 2 | MEM-04 | T-10-06 | Classifier 7 cases: Allow, AllowJustAboveFloor, Refuse, DefaultDenyOnParseFailure, DefaultDenyOnInvalidDecision, DefaultDenyOnLowConfidence (W3), DefaultDenyOnConfidenceOutOfRange | unit | `go test ./internal/server/classifier/... -v -count=1` | ⬜ pending |
| 10-02-T2a | 02 | 2 | MEM-04 | T-10-09 | `audit_meta` DDL + insert + cascade + nullability | unit | `go test ./internal/store/sqlite/... -run 'TestAuditMetaDDL|TestAuditMetaInsertQuery|TestAuditMetaCascadeDelete|TestAuditMetaNullableOptionalColumns' -count=1` | ⬜ pending |
| 10-02-T3a | 02 | 2 | MEM-01..05 | T-10-07 | Pipeline branches through history → rewrite → retrieve → classify → generate (8 tests) | unit | `go test ./internal/server/... -run 'TestVerdictPipeline_HistoryAwareness|TestVerdictPipeline_ClassifierRefuses|TestVerdictPipeline_RewriterFallback|TestVerdictPipeline_ClassificationTypeIsolation|TestVerdictPipeline_NilRewriterStillFlowSHistory|TestVerdictPipeline_HistoryChunkRefsSanitized|TestVerdictPipeline_BudgetTruncatedEvent|TestVerdictPipeline_RewriteEmitsEvent' -count=1` | ⬜ pending |
| 10-02-T4a | 02 | 2 | MEM-01 | T-10-11 | Production binary builds with rewriter + classifier wired (B2 key fix) | build | `go build ./cmd/server && go vet ./... && go test ./... -count=1 -timeout 120s -race` | ⬜ pending |
| 10-03-T1a | 03 | 3 | MEM-02 | T-10-11 | GET /api/session/:id/history 8 tests (empty / valid / SC9 / unknown-200+empty / invalid / missing-store / display-cap / handler-does-not-Project) | integration | `go test ./internal/server/... -run TestSessionHistoryAPI -count=1` and `! grep -q 'history\.Project' internal/server/handlers_session_history.go` | ⬜ pending |
| 10-03-T2a | 03 | 3 | MEM-02 | T-10-12 | FE restore: fetchHistory + historyToUIMessages (5 vitest tests) | unit | `cd web && npx vitest run --reporter=basic chat-restore` | ⬜ pending |
| 10-03-T2b | 03 | 3 | MEM-02 | T-10-14 | useSkillAutoSwitch hook (3 vitest tests) | unit | `cd web && npx vitest run --reporter=basic skill-auto-switch` | ⬜ pending |
| 10-03-T2c | 03 | 3 | MEM-02 | T-10-13 | HistoryRewriteHint component (2 vitest tests) | unit | `cd web && npx vitest run --reporter=basic HistoryRewriteHint` | ⬜ pending |
| 10-03-T2d | 03 | 3 | MEM-02 | T-10-15 | D15 drift alarm: TypeScript types remain zero-error | typecheck | `cd web && npx tsc --noEmit` exit 0 (after the single `cd web` in verify; checker W5 fix) | ⬜ pending |
| 10-04-T1a | 04 | 4 | MEM-04 | T-10-04 | --via-pipeline composition hermetic dry-run test | unit | `go test ./cmd/spike-history/... -run 'TestBuildPipelineMessages_Structure|TestRunEntry_ViaPipelineDryRun' -count=1` | ⬜ pending |
| 10-04-T1b | 04 | 4 | MEM-01..05 | T-10-SC | Pre-flight: full BE suite + contract + isolation -race + FE vitest + tsc + grounding | suite | `go test ./... -count=1 -timeout 120s -race && go test -tags=contract ./... -count=1 && go test ./internal/server/... -run TestSessionIsolation50x5 -count=1 -race && go test ./internal/verdict/... -run TestGroundingChecker -count=1` | ⬜ pending |
| 10-04-T1c | 04 | 4 | MEM-04 | T-10-17 | Frozen lock BINDING gate (B6 fix): `git diff --quiet` + clean porcelain | byte-compare | `git diff --quiet eval/frozen/compliance.lock.json && git status --porcelain eval/frozen/ \| wc -l` == 0 | ⬜ pending |
| 10-04-T2 | 04 | 4 | MEM-01..05 | T-10-20 | Live 35-case spike re-run via `--via-pipeline` (35 rows total + summarize exit 0/1) | spike | `go run ./cmd/spike-history --via-pipeline ... --summarize=reports/ --baseline-lock=eval/frozen/compliance.lock.json` | ⬜ pending |
| 10-04-T3 | 04 | 4 | MEM-01..05 | T-10-18 | Phase closeout: SUMMARY (6 sections) + ROADMAP + REQUIREMENTS + STATE updated | integration | `test -s .planning/phases/10-p8-1/10-04-SUMMARY.md && grep -c MEM .planning/phases/10-p8-1/10-04-SUMMARY.md` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Grep Gates (D10-A1 / D10-A2 / B3 / B4)

| Gate ID | Purpose | Command | Expected Output |
|---------|---------|---------|-----------------|
| GG-A1-1 | Classifier struct has no `History` field (D10-A1 type-level isolation) | `grep -c 'History' internal/server/classifier/types.go` | 0 |
| GG-A1-2 | Classifier types.go does not import history or store | `grep -c 'internal/history\|internal/store' internal/server/classifier/types.go` | 0 |
| GG-A1-3 | Classifier implementation file (Plan 02) does not load or project history | `! grep -n 'GetSession\|loadHistory\|historyMsgs\|history\.Project' internal/server/classifier/classifier.go` | empty |
| GG-A2-1 | Neutral placeholder template zero refusal-semantic words (canonical 11-entry list, comment-exempt) | `grep -v '^[[:space:]]*//' internal/history/neutral_placeholder.go \| grep -cE '拒绝\|不能\|无权\|无法\|不得\|不允许\|未在\|无可\|谢绝'` | 0 |
| GG-B3-1 | Agent-path projection never references RenderRefusal (B3) — | `grep -c RenderRefusal internal/history/projection.go` | 0 |
| GG-B4-1 | Handler user-facing path never calls Project() (B4) — | `! grep -n 'history\.Project' internal/server/handlers_session_history.go` | empty |
| GG-B6-1 | Frozen lock byte-identical (B6 binding) — | `git diff --quiet eval/frozen/compliance.lock.json && git status --porcelain eval/frozen/ \| wc -l` | exit 0 + 0 |
| GG-D14 | Zustand store has no `messages` field | `! grep -n 'messages: ' web/src/store/app-store.ts \| grep -v '//'` | empty |

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-of-phase spike live re-run via `--via-pipeline` | MEM-04 (refusal drift) + MEM-03 (cite-penetration) + MEM-01..05 | Live GLM-5.2 + Milvus stack requires Docker compose + GLM_API_KEY + BGE_M3_URL + Milvus tier-filter setup; cannot be hermetic | `docker compose up -d milvus etcd minio` + `ollama pull bge-m3` + `pop .env` + `go run ./cmd/spike-history --via-pipeline --category=history_injection_adversarial --with-history=true --history-length=short --out=reports/ --seed=42 --cost-limit=5.00` (mirror Phase 9 09-06 sequence with `--via-pipeline` added); observe `reports/summary.md` exit 0 + 4 gates PASS |
| Per-call cost monitoring | MEM-05 | Real $$; cost-cap threshold requires human confirmation before re-run | During spike, watch `[cost]` log; abort manually if >$5.00/segment |
| Frozen lock provenance verification | T-10-17 (B6 fix) | Provenance note reconciliation is operator-confirmable | Confirm sha256sum output first 8 hex == `5cf700f3` (full sha `5cf700f357a4a7022195ec233106c5398fe2c5562045ca8894996efe23cecded`); confirm STATE.md/08-05's `50bb1688...` is acknowledged as stale / hashed at a different scope |

*All other behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or pre-task dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] No Wave 0 plan exists in this phase — Wave 0 rows removed (per checker W1)
- [x] No watch-mode flags
- [x] Feedback latency < 90s (BE) / 60s (FE) / 75min (spike live gate)
- [x] `nyquist_compliant: true` set in frontmatter (revised after checker W1)

**Approval:** pending (until execution completes — `/gsd:execute-phase 10` then `/gsd:verify-work 10` are the unblock gates)