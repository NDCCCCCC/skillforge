---
quick_task: 260906-w62
verified: 2026-09-07T00:00:00Z
status: human_needed
score: 7/7 must-haves verified
overrides_applied: 0
re_verification: # n/a — initial verification
  previous_status: none
  previous_score: none
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Boot the real server process with EXTERNAL_AUTH_ENABLED=1 (and external_views/ present) and confirm the '[OK] cmd/server: external auth channel wired' log, then POST /api/external-auth/request and observe 201 with an xreq-* id; then boot WITHOUT the gate and confirm 503 on the same route."
    expected: "Gate set → channel wired + 201; gate unset → byte-identical 503 fail-closed behavior."
    why_human: "main() env parsing and process boot are not covered by any httptest (tests construct Deps directly); the env gate branch in cmd/server/main.go:490-508 is only exercised by a real process start."
  - test: "Run a live chat session with a Skill declaring reads: [data:external:authorized] (examples/skills/external-lookup), ask an individual-level question, watch the agent call needs_external_query, approve via POST /approve with X-Admin-Token, then let the agent re-invoke with query_id."
    expected: "Pending card with SQL preview + query_id, then masked rows (account last-4 only) after approval; no unmasked field ever surfaced."
    why_human: "The hermetic tests stub the ChatModel with a canned preview; the real LLM tool-calling loop, the pending-card UX in the agent narrative, and refusal phrasing are live-model behavior grep cannot observe."
  - test: "Read README.md end to end as one reader and confirm a single three-moats vocabulary (approve-and-remember / trust-contract kernel / CI-integrated trust gate) across the architecture paragraph, moats section, diagrams, and prose — and that the External Authorized Channel section reads as honest rather than oversold."
    expected: "No reader meets two different moat trios; channel section discloses the un-gated /api/external-auth/request and the reference-impl boundaries clearly."
    why_human: "Narrative consistency and disclosure tone are prose judgments; greps confirm token counts and absences but not reader-level coherence."
---

# Quick Task 260906-w62: v1.1.1 Cleanup — Wire Phase 21 External Authorized Channel — Verification Report

**Task Goal:** make the Phase 21 External Authorized Channel reachable end-to-end — manager wired into cmd/server behind EXTERNAL_AUTH_ENABLED=1, needs_external_query agent tool, data:external:authorized reads allowlist, eval/frozen/external-auth.jsonl golden set, README/REQUIREMENTS/CHANGELOG/21-SUMMARY reconciliation. Trust contract: fail-closed defaults, INSERT-only audit, DATA-09 pass-through, admin-token gates ordered before the nil-manager 503.
**Verified:** 2026-09-07
**Status:** human_needed (all 7 truths verified in code; 3 live-process/prose items routed to human)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | With EXTERNAL_AUTH_ENABLED=1, POST /api/external-auth/request returns 201 with a minted xreq-* request_id — never 503 | ✓ VERIFIED | `cmd/server/main.go:490-508` (6c-5: env gate → LoadViewsDir → NewMemoryStore/NewMemorySource/NewManager; log.Fatalf on bad view file; empty-registry WARN), Deps assignment at `:641`. `TestExternalAuth_WiredPath` ① asserts 201 + `xreq-` prefix + StatePending over the real route table. `TestExternalAuth_DegradedPath` asserts 503 on all six routes with nil manager + correct admin token (the gate-unset equivalent). WSL `go build ./cmd/server` PASS |
| 2 | A Skill declaring reads: [data:external:authorized] passes skill.Validate and its Runner sees needs_external_query in the tool set | ✓ VERIFIED | `internal/skill/validate.go:103` pre-scan accepts both data modes; `validate_test.go:371` `external_authorized_data_mode_passes` (wantErr false) + `:391` `truly_unknown_data_mode_rejected` (fail-closed kept); `internal/agent/external_channel_test.go` 3 faces (tool present / nil-manager fail-fast naming the tool + "fail-closed" / aggregated_only never sees it); `internal/agent/runner_setup.go:346-356` real branch gated on `slices.Contains(cfg.Manifest.Reads, skill.ReadDataExternalAuthorized)`; published `examples/skills/external-lookup/SKILL.md` loads through the production Loader (`TestExamplesValidate` count 6→7). internal/skill + internal/agent suites green |
| 3 | needs_external_query produces its SQL preview only via nl2sql.Generate and returns {status: pending, query_id, sql_preview, view}; never composes SQL in Go (DATA-09) | ✓ VERIFIED | `internal/agent/tools/external_query_tool.go:255` is the only preview producer (`nl2sql.Generate(ctx, cfg.ChatModel, schemaSnapshot, query)`); construction-time schema snapshot at `:150-169`; `TestExternalToolNoSQLSurface` (`:256-272`) comment-strips the tool source and asserts zero `SELECT `/`INSERT `/`UPDATE `/`DELETE ` tokens; live grep on the current tree = 0; `TestExternalTool_FreshCallPending` asserts pending + `xreq-` + the canned preview verbatim; pre-existing `TestNoSQLGenerationSurface` still green |
| 4 | After a data officer approves, a second tool invocation carrying query_id executes via the TTL token and returns masked rows | ✓ VERIFIED | Tool branch ② (`external_query_tool.go:226-245`): StateApproved → `Manager.ExecuteWithToken(req.TTLTokenID)` → status "executed" + rows + MaskApplied; expired token → refusal `token_expired` (no retry); rejected → refusal; pending re-invoked → pending again (DATA-13). `TestExternalTool_ApproveThenExecuteMaskedRows` asserts the account value has no raw digits beyond last-4; `TestExternalAuth_EndToEndMasking` proves the same over HTTP; `TestExternalTool_PendingStaysPending` and `TestExternalTool_UnknownQueryIDRefused` lock the other faces |
| 5 | pending / approve / reject / execute / reconcile HTTP surfaces require X-Admin-Token (403 without it, constant-time compare) | ✓ VERIFIED | `internal/server/external_auth.go` — 5 gated handlers (`:105/:135/:164/:194/:228`), each runs `isAdminRequest(c)` FIRST then the nil-manager 503, each with the CHECK ORDER comment; `isAdminRequest` (`internal/server/audit.go:114-126`) uses `subtle.ConstantTimeCompare` + fail-closed on empty token. `TestExternalAuth_AdminGateOrderedBeforeDegraded` asserts 403 (not 503) on all 5 surfaces with a nil manager — the leak-invariant is locked, not just narrated |
| 6 | eval.LoadGolden parses eval/frozen/external-auth.jsonl into 10 cases — exactly 3 refusal-required — categories cover all 7 DATA requirements (04/05/06/10/11/13/14) | ✓ VERIFIED | Read the file: 10 cases, distribution 04×1 / 05×2 / 06×1 / 10×2 / 11×2 / 13×1 / 14×1 = 10; refusal ids `ext-auth-005/006/008-refusal` (both 10-precompiled-view cases + the second 11-sql-preview non-select case — exactly per plan); `expected_chunks: []` everywhere; `source: phase21-cleanup-2026-09-06`; `#` header documents the unpinned state + replay command. `TestExternalAuthGoldenSet` (`internal/eval/external_auth_set_test.go:32-77`) asserts 10 cases, unique ids, all 7 DATA numbers, exactly 3 refusals, non-empty query/ground_truth; `TestExternalAuthGoldenSetUnpinned` locks that no lock sidecar exists. internal/eval green |
| 7 | README carries ONE three-moats set named identically everywhere (zero old-vocabulary remnants) + an External Authorized Channel section flagging the un-gated /api/external-auth/request; REQUIREMENTS DATA-04..14 rows reflect Phase 21 delivery | ✓ VERIFIED | README greps: `Go-native trust evaluator` 0, `Skill-as-platform` 0; `approve-and-remember` 6, `trust-contract kernel` 4, `CI-integrated trust gate` 6; `External Authorized Channel` section present with the flow list, the ✅/⚠️/❌ boundary table explicitly flagging "POST /api/external-auth/request is intentionally agent-facing and un-gated in v1.1.1" (T-W62-06), and the EXTERNAL_AUTH_ENABLED=1 gate disclosure. REQUIREMENTS: 7 rows `[x]` with "v1.1 (Phase 21; wired + agent tool in v1.1.1)"; DATA-07 reads "both modes complete"; DATA-08/09/12 untouched. CHANGELOG `## [v1.1.1] — 2026-09-07` present with Added/Fixed/Docs |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/externalauth/memstore.go` | exported in-process Store (fakeStore → alias) | ✓ VERIFIED | `func NewMemoryStore` (`:72`), all 6 Store methods, `var _ Store = (*MemoryStore)(nil)` (`:81`), FIFO pending list, AuditRowID stamp, single-write resolve guard (refuses re-resolution — a hardening beyond the plan's minimum), honest non-durable godoc. `fakeStore = MemoryStore` alias confirmed at `externalauth_test.go:21-23` |
| `internal/externalauth/views.go` | LoadViewsDir controlled-YAML bootstrap (DATA-10) | ✓ VERIFIED | `func LoadViewsDir` (`:83`), fail-closed per-entry validation (view-name pattern, cross-file duplicates, non-empty source, closed mask-kind set, mask field ∈ columns, unique columns); absent/zero-yaml dir = the single legal degraded state; errors name the file. 10 loader tests incl. `TestLoadViewsDir_ShipsReferenceView` |
| `external_views/v_user_consumption.yaml` | shipped reference view (columns + masks) | ✓ VERIFIED | 5 columns / 3 masks exactly as specified (account_no keep 4, phone keep 4, email keep 2); loads via the production loader in tests |
| `cmd/server/main.go` | 6c-5 assembly, EXTERNAL_AUTH_ENABLED gate, Deps assignment | ✓ VERIFIED | Section 6c-5 (`:462-508`) + `ExternalAuthManager: externalAuthManager` in the Deps literal (`:641`); deviation (opt-in gate) documented in the code comment as planned |
| `internal/agent/tools/external_query_tool.go` | BuildExternalQueryTool (DATA-11 preview → DATA-13 pending → masked rows) | ✓ VERIFIED | `ExternalQueryToolName = "needs_external_query"` (`:61`); structured refusals as tool results (8 reason codes); construction-time schema + mask snapshot; wired into the data-channel branch |
| `internal/skill/manifest.go` | ReadDataExternalAuthorized constant | ✓ VERIFIED | `:170` with godoc stating the two data-channel constants are the complete allowlist; both stale comments refreshed (aggregated_only's "must NOT appear" note and HasDataChannel's "unknown modes rejected" narrative) |
| `eval/frozen/external-auth.jsonl` | 10-case golden set (3 refusal-required) | ✓ VERIFIED | See truth 6 |
| `.planning/phases/21-external-auth-channel/21-SUMMARY.md` | honest post-hoc record | ✓ VERIFIED | Documents a955450 vs the v1.1.1 cleanup per commit, a full test inventory table (12 pre-existing + all new tests), boundaries carried forward verbatim, and an explicit note that no 21-VERIFICATION.md was fabricated |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| cmd/server/main.go | externalauth.NewManager(NewMemoryStore, NewMemorySource, LoadViewsDir) | 6c-5 assigns Deps.ExternalAuthManager | ✓ WIRED | `:500-501` store+source construction, `:501` NewManager, `:641` Deps field; `internal/server/server.go:357` declares the field, `:520` routes read it |
| internal/server/runner_setup.go | agent.RunnerBuildConfig.ExternalAuthManager | ChatHandler passes s.ExternalAuthManager through | ✓ WIRED | `:175` `ExternalAuthManager: s.ExternalAuthManager` inside the RunnerBuildConfig literal |
| internal/agent/runner_setup.go | agenttools.BuildExternalQueryTool | data-channel branch appends only for ReadDataExternalAuthorized, fail-fasts on nil | ✓ WIRED | `:346` slices.Contains gate, `:347-351` nil → fail-fast error naming the literal constant + "fail-closed", `:353-356` BuildExternalQueryTool with `cfg.ExternalAuthManager.Registry()` |
| internal/agent/tools/external_query_tool.go | nl2sql.Generate | pass-through preview only (DATA-09) | ✓ WIRED | `:255`; zero SQL verbs outside comments on the current tree (grep = 0) and locked by TestExternalToolNoSQLSurface |
| internal/skill/validate.go | skill.ReadDataExternalAuthorized | allowlist pre-scan accepts both data constants | ✓ WIRED | `:103` and `:115` (mixed-reads message refreshed to name both) |
| internal/eval/external_auth_set_test.go | eval/frozen/external-auth.jsonl | eval.LoadGolden loader-compatibility check | ✓ WIRED | `:33` LoadGolden on the real path; suite green under WSL |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Pure-Go suites green | `go test ./internal/externalauth/ ./internal/nl2sql/ ./internal/skill/ -count=1` (Windows) | ok × 3 packages (externalauth 0.5s, nl2sql 0.1s, skill 8.5s) | ✓ PASS |
| CGO suites + build + vet green | `make check-cgo && go build ./cmd/server && go vet ./cmd/server/ ./internal/server/ ./internal/agent/... && go test ./internal/server/ ./internal/agent/... ./internal/eval/ -count=1` (WSL) | check-cgo PASS, build PASS, vet clean, ok × 6 packages (server 22.5s, agent 5.8s, tools 0.16s, eval 0.39s) | ✓ PASS |
| memstore insert-only lock | comment-stripped grep for `UPDATE \|DELETE FROM` | 0 (require 0) | ✓ PASS |
| tool DATA-09 pass-through lock | comment-stripped grep for `SELECT \|INSERT \|UPDATE \|DELETE ` | 0 (require 0) | ✓ PASS |
| tool registration gate | grep `needs_external_query` in internal/agent/runner_setup.go | 2 (require ≥1) | ✓ PASS |
| assembly gate | grep `externalauth` in cmd/server/main.go | 6 (require ≥3) | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes are declared or conventional for this task. The T8 gate sequence (build + vet + 6 CGO packages + 3 pure-Go packages + structural greps) was re-run by the verifier in its own process — all green (see Behavioral Spot-Checks).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| DATA-04 | 260906-w62 | External authorized channel with TTL token | ✓ SATISFIED | TTL tokens minted on approve, expiry enforced (`TestManager_TokenExpiryEnforced` pre-existing, still green); wired + tool in v1.1.1 |
| DATA-05 | 260906-w62 | Field-level masking | ✓ SATISFIED | Mask kinds validated fail-closed at load; masked rows asserted in tool + HTTP tests |
| DATA-06 | 260906-w62 | Dual audit + reconcile | ✓ SATISFIED | MemoryStore appends audit rows; reconcile endpoint + `TestManager_Reconcile_PairMismatch` green |
| DATA-07 | 260906-w62 | reads allowlist both modes | ✓ SATISFIED | `ReadDataExternalAuthorized` accepted; unknown `data:*` still fails closed (`truly_unknown_data_mode_rejected`) |
| DATA-10 | 260906-w62 | Precompiled view allowlist | ✓ SATISFIED | LoadViewsDir is the only runtime registration path; unregistered-view refusals locked |
| DATA-11 | 260906-w62 | Read-only SQL preview | ✓ SATISFIED | nl2sql pass-through + RejectNonSelect gate + non-select refusal test |
| DATA-13 | 260906-w62 | Persistent pending approval | ✓ SATISFIED | Pending is a positive state in the tool and the store; no auto-resolve/timeout path exists |
| DATA-14 | 260906-w62 | Audit row schema | ✓ SATISFIED | AuditRowID stamped, insert-only discipline structurally locked; 9-field schema documented in the golden set ground_truth |

Orphaned requirements: none — the plan's `requirements: [DATA-04, DATA-05, DATA-06, DATA-07, DATA-10, DATA-11, DATA-13, DATA-14]` matches the REQUIREMENTS.md rows flipped.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| (none) | — | No TBD/FIXME/XXX/HACK, no placeholder / not-yet-implemented markers, no empty-return stubs in any file this task touched | — | — |

### Anti-Gaming Review

- **Structural locks are not vacuous.** Both locks (`TestMemoryStoreNoSQL`, `TestExternalToolNoSQLSurface`) strip comment lines before matching and target real verb tokens; both would fail on a smuggled statement string. Minor info: the verb patterns carry a trailing space (`"SELECT "`), so a statement token at exact end-of-line without a trailing space would evade — this matches the plan's own specified pattern and is low-risk (Go string literals virtually never end a line mid-statement).
- **No lock weakened to pass.** The `unknown_data_mode_rejected` → passing flip is compensated by the newly added `truly_unknown_data_mode_rejected` (wantErr true, error echoes the offending value) — the fail-closed default stayed locked, exactly as the plan required.
- **No unconditional trailing verify chains.** Plan verify commands are `&&`-chained (failure propagates) and the T7 doc verify asserts each absence (`! grep`) and presence (`grep -c … -ge`) separately rather than a single self-tripping compound.
- **Channel registration test is a documented re-derivation.** `external_channel_test.go` mirrors BuildRunner's tool-composition branch because the Runner's tool list is unexported — mild mirror-drift risk, mitigated because the same test drives the REAL `agent.BuildRunner` for the nil-manager fail-fast face and for validation.
- **No fabricated artifacts.** `external-auth.lock.json` absent (and locked absent by `TestExternalAuthGoldenSetUnpinned`); no `21-VERIFICATION.md` exists in the phase dir (correct — that is this orchestrator step); SUMMARY's 7 commit hashes all resolve to real commits with matching subjects.
- **web/ untouched.** `git diff --stat 552d32e HEAD -- web/` is empty; `git status --short web/` empty. FE checks correctly not required this round.

### Honest Boundaries Preserved

- README channel section discloses the un-gated `/api/external-auth/request` (⚠️ row, T-W62-06), the reference-impl MemorySource, the missing FE panel, and the `EXTERNAL_AUTH_ENABLED=1` gate requirement.
- 21-SUMMARY lists FE panel, reference impls, unpinned golden set, un-gated request, and the gate as explicit carried-forward boundaries.
- CHANGELOG `[v1.1.1]` section present with the disclosure line.
- Gate-unset → 503 behavior unchanged (nil Deps field → `TestExternalAuth_DegradedPath` 503 on all six routes).

### Human Verification Required

See the three items in the frontmatter `human_verification` list: (1) real process boot with/without the env gate, (2) the live-LLM tool-calling loop end to end, (3) README prose-level coherence pass. All three are behaviors hermetic tests deliberately cannot observe.

### Gaps Summary

No gaps. All 7 must-have truths are verified in the codebase with hermetic test evidence; the 6 key links are wired; all 8 artifacts are substantive and wired; requirements coverage is complete with no orphans; the trust contract (fail-closed defaults, INSERT-only audit, DATA-09 pass-through, admin gates ordered before the nil-manager 503) is each locked by a named test rather than by narration. The only residuals are the three live-process/prose items routed to human verification, plus the two info-level notes above (trailing-space verb pattern, mirrored tool-composition test). Status is `human_needed` solely because the human-verification section is non-empty — not because any automated check failed.

---

_Verified: 2026-09-07_
_Verifier: Claude (gsd-verifier)_
