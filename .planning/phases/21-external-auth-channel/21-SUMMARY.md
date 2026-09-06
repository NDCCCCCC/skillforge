---
phase: 21
plan: post-hoc backfill (v1.1.1 cleanup 260906-w62)
status: complete
subsystem: external-auth-channel
tags: [externalauth, nl2sql, agent-tools, http-surface, golden-set]
requires: [internal/store (absent — see boundaries), eino tool utils, nl2sql]
provides: [externalauth.Manager wiring, needs_external_query tool, data:external:authorized reads mode, external-auth golden set, admin-token-gated channel surfaces]
affects: [cmd/server, internal/server, internal/agent, internal/skill, eval/frozen]
tech-stack:
  added: [gopkg.in/yaml.v3 controlled view files (existing dep)]
  patterns: [controlled-file loader (D16-08-D posture), construction-time tool snapshot (kpi_tool pattern), comment-stripped structural locks, fail-closed assembly guard]
key-files:
  created:
    - internal/externalauth/memstore.go
    - internal/externalauth/views.go
    - internal/externalauth/views_test.go
    - internal/externalauth/memstore_test.go
    - external_views/v_user_consumption.yaml
    - internal/agent/tools/external_query_tool.go
    - internal/agent/tools/external_query_tool_test.go
    - internal/agent/external_channel_test.go
    - internal/server/external_auth_test.go
    - internal/eval/external_auth_set_test.go
    - eval/frozen/external-auth.jsonl
    - examples/skills/external-lookup/SKILL.md
  modified:
    - internal/externalauth/types.go
    - internal/externalauth/manager.go
    - internal/externalauth/externalauth_test.go
    - cmd/server/main.go
    - internal/server/external_auth.go
    - internal/server/server.go
    - internal/server/runner_setup.go
    - internal/agent/runner_setup.go
    - internal/skill/manifest.go
    - internal/skill/validate.go
    - internal/skill/validate_test.go
    - internal/skill/examples_validate_test.go
    - README.md
    - .planning/REQUIREMENTS.md
    - CHANGELOG.md
decisions:
  - EXTERNAL_AUTH_ENABLED=1 explicit operator opt-in (fail-closed default-off) instead of unconditional-on wiring — the channel brokers individual-level PII and the route table already declared that gate as the contract
  - MemoryStore is the v1.1 reference Store (promoted from the test-only fake) — sqlite/vault-backed Store is a v1.2 candidate
  - Manager.Registry() accessor added so the agent tool derives its nl2sql schema + mask list from the same registry the manager enforces
metrics:
  duration: single quick-task execution (2026-09-07)
  tasks: 8
  date: 2026-09-07
---

# Phase 21: External Authorized Channel — Summary (post-hoc backfill)

**One-liner:** the DATA-04..14 external authorized channel — approve-before-query with NL2SQL preview, 5-minute TTL tokens, field-level masking, dual audit + reconcile — shipped in a955450 as a tested library + HTTP surface, and was made **reachable end-to-end** by the v1.1.1 cleanup (wiring + agent tool + reads allowlist + golden set + admin gates).

> **Why this file exists.** Phase 21 was executed outside the normal GSD
> per-phase artifact flow and never produced a SUMMARY. This document was
> backfilled (2026-09-07) by the v1.1.1 cleanup so the planning record
> matches the code. `.planning/reports/MILESTONE_SUMMARY-v1.1.md` §6 Gap 1
> is the gap analysis that triggered it.

## What shipped in a955450 (Phase 21, 2026-09-06)

- `internal/externalauth/{types,source,manager}.go` — the trust pipeline:
  `TTLToken` (DATA-04), `PrecompiledView` + `ViewRegistry` + field masks
  (DATA-05/DATA-10), `AuditRow` + `Reconcile` (DATA-06/DATA-14),
  persistent-pending approval (DATA-13), `RejectNonSelect` (DATA-11).
- `internal/nl2sql/generate.go` — NL2SQL **preview only**, behind the
  SELECT-only gate (DATA-09 relaxed solely inside the DATA-10..14 stack).
- `internal/server/external_auth.go` + six routes in `server.go`:
  request / pending / approve / reject / execute / audit-reconcile.
- 12 package tests, including `TestUAT_FullChannelFlow` — a 7-step
  end-to-end release gate covering all seven DATA requirements. The live
  curl UAT is recorded in `.planning/uat/phase21-external-auth-2026-09-06.md`.

**What it did NOT ship:** any construction site for `Manager` (the only
`Store` implementation lived inside a `_test.go` file), the
`needs_external_query` agent tool, a `reads` allowlist entry for the
channel, and the Wave-5 golden set. The channel was library-correct and
deployment-dead (MILESTONE_SUMMARY-v1.1 Gap 1, HIGH).

## What shipped in the v1.1.1 cleanup (260906-w62, 2026-09-07)

| Commit | Deliverable |
|--------|-------------|
| `6cda7d4` | `MemoryStore` — exported in-process `Store` (the test fake promoted to production semantics + FIFO pending list + `AuditRowID` stamp + single-write resolve guard); `fakeStore` became a 2-line alias so the existing 12 tests compile unchanged |
| `3aa602c` | `LoadViewsDir` — controlled-file view registry bootstrap (DATA-10) + `PrecompiledView.Columns` + the shipped reference view `external_views/v_user_consumption.yaml` |
| `c3a5da5` | cmd/server assembly section 6c-5 behind `EXTERNAL_AUTH_ENABLED=1` (fail-closed default-off) + `X-Admin-Token` gates on the five sensitive surfaces, admin check ordered before the nil-manager 503 |
| `14dc153` | `needs_external_query` agent tool (pass-through preview, structured refusals, fail-closed registration) + `RunnerBuildConfig.ExternalAuthManager` + ChatHandler pass-through + `Manager.Registry()` accessor |
| `a3761f1` | `reads: [data:external:authorized]` validates (DATA-07 second half); unknown `data:*` still fails closed; published `examples/skills/external-lookup` |
| `a4dbb7f` | `eval/frozen/external-auth.jsonl` — 10 cases across the 7 DATA requirements, exactly 3 refusal-required, **unpinned** (no lock sidecar) |
| (this commit) | README moat-vocabulary reconciliation + new channel section; REQUIREMENTS DATA rows flipped; CHANGELOG v1.1.1; this file |

## Test inventory

Pre-existing (a955450, all still green unchanged):

| Test | Locks |
|------|-------|
| `TestMask_AccountLast4` | DATA-05 account last-4 |
| `TestMask_EmailKeepsFirstTwo` | DATA-05 email local-part first 2 |
| `TestMask_AccountShortFullMask` | input shorter than KeepLen → full mask |
| `TestMask_UnknownKindFullMask` | unknown kind → conservative full mask |
| `TestRejectNonSelect` | DATA-11 SELECT-only gate (14-row verb table) |
| `TestManager_FullApprovalAndExecute` | full flow + DATA-04/05/06 |
| `TestManager_RejectDoesNotMintToken` | reject path mints no token |
| `TestManager_NonSelectRejected` | DATA-11 interception in RequestApproval |
| `TestManager_UnregisteredViewRejected` | DATA-10 allowlist |
| `TestManager_TokenExpiryEnforced` | DATA-04 TTL expiry (frozen clock) |
| `TestManager_Reconcile_PairMismatch` | DATA-06 one-sided row is surfaced |
| `TestUAT_FullChannelFlow` | 7-step end-to-end release gate |

Added by the v1.1.1 cleanup:

| Test | Locks |
|------|-------|
| `TestMemoryStore_Lifecycle` | issue → get → FIFO pending list → approve (token persisted + `AuditRowID` stamped) → reject (no token) → single-write resolve guard |
| `TestMemoryStoreNoSQL` | comment-stripped: zero mutation verbs in `memstore.go` (insert-only discipline, structural) |
| `TestLoadViewsDir_*` (10) | valid load w/ columns+masks; **the shipped `external_views/` file loads**; malformed yaml; unknown mask kind; mask field not in columns; duplicate view name; absent dir; zero-yaml dir; duplicate column; bad view name |
| `TestExternalAuth_WiredPath` | wired channel: 201 + `xreq-`, pending 403/200, approve 403/200 + `xtok-`, reconcile 200, execute 200 |
| `TestExternalAuth_DegradedPath` | nil manager → 503 on all six routes even with a correct admin token |
| `TestExternalAuth_AdminGateOrderedBeforeDegraded` | 403 before 503 — a 403 never leaks channel state (T-W62-01) |
| `TestExternalAuth_EndToEndMasking` | request → approve → execute over the real route table; no raw account digits beyond the last 4; DATA-14 `mask_applied` |
| `TestExternalTool_FreshCallPending` | preview path returns pending + `xreq-` + the nl2sql preview verbatim (DATA-09 pass-through) |
| `TestExternalTool_ApproveThenExecuteMaskedRows` | pending → approve → executed with masked rows |
| `TestExternalTool_PendingStaysPending` | DATA-13: pending is a positive state, no auto-resolve |
| `TestExternalTool_UnknownQueryIDRefused` | fabricated `query_id` → `request_not_found` |
| `TestExternalTool_NonSelectPreviewRefused` | DATA-11: mutating model output → `sql_preview_rejected` |
| `TestExternalTool_MissingQueryRefused` | empty query → `invalid_arguments` |
| `TestExternalTool_NilManagerRefused` | defense in depth: un-wired tool refuses, never panics |
| `TestExternalToolNoSQLSurface` | comment-stripped: zero statement verbs in the tool source (the tool never authors SQL — DATA-09) |
| `TestExternalChannelToolRegistration` (3 sub) | external:authorized + manager → tool present; nil manager → fail-fast naming the tool; aggregated_only → never present (DATA-07) |
| `external_authorized_data_mode_passes` / `truly_unknown_data_mode_rejected` | validate_test.go: the new mode validates; unknown modes still fail closed |
| `TestExamplesValidate` (6→7) | the published external-lookup example loads through the production Loader |
| `TestExternalAuthGoldenSet` | the golden set parses via the production `LoadGolden`; 10 cases; unique ids; all seven DATA numbers covered; exactly 3 refusal-required |
| `TestExternalAuthGoldenSetUnpinned` | no lock sidecar exists until an authorized replay mints one (D14-B4) |

## Known boundaries carried forward

Verbatim from the Phase 21 UAT closure report
(`.planning/uat/phase21-external-auth-2026-09-06.md`) and the v1.1.1
cleanup, still true after this cleanup:

- **FE panel deferred** — no `ExternalChannelPanel`; the HTTP surfaces +
  the agent tool are the interface. Left for v1.2 or when a user need
  materializes.
- **MemorySource / MemoryStore are v1.1 reference implementations** —
  in-process and non-durable (a restart loses pending requests, tokens,
  and audit rows). Real warehouse drivers and a durable token/audit store
  (sqlite or vault) land in v1.2 behind the unchanged `Source` / `Store`
  interfaces.
- **The golden set is unpinned** until an operator runs an authorized
  live replay; no `external-auth.lock.json` was fabricated (D14-B4:
  regenerating a lock = tampering with the baseline).
- **`POST /api/external-auth/request` is intentionally un-gated in
  v1.1.1** — the agent's in-process path mirrored as an observable /
  audit surface, bounded by the DATA-10 view allowlist; request-flood
  hardening (shared secret + rate limit) is queued for v1.2 (T-W62-06).
- **`EXTERNAL_AUTH_ENABLED=1` is required** — the channel is fail-closed
  off by default; unset gate = every `/api/external-auth/*` route 503s
  and no Skill sees `needs_external_query`.

## Verification note

No `21-VERIFICATION.md` exists and none was fabricated: per-phase
verification is a separate orchestrator step that was never run for
Phase 21 (the milestone was executed outside the GSD per-phase artifact
flow). The v1.1.1 cleanup's own gates — WSL build + vet + server/agent/
eval suites, Windows pure-Go suites, and the comment-stripped structural
greps — are recorded in `260906-w62-SUMMARY.md`.
