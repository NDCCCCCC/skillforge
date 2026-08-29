---
phase: 18-skill-p10-4
fixed_at: 2026-08-28T00:00:00Z
review_path: .planning/phases/18-skill-p10-4/18-REVIEW.md
iteration: 1
findings_in_scope: 16
fixed: 16
skipped: 0
status: all_fixed
---

# Phase 18: Code Review Fix Report

**Fixed at:** 2026-08-28
**Source review:** `.planning/phases/18-skill-p10-4/18-REVIEW.md`
**Iteration:** 1
**Scope:** Critical (6) + Warning (10). All 11 Info findings deferred per scope.

**Summary:**
- Findings in scope: 16
- Fixed: 16
- Skipped: 0

**Verification matrix:**
- `go build ./internal/agent/... ./internal/report/... ./internal/wire/... ./internal/store/... ./internal/skill/... ./internal/runtime/... ./internal/eval/... ./internal/server/`: green (known CGO-broken: `internal/data`, `internal/duckdb`, `internal/kpi/wsl`, `cmd/server`, `cmd/eval`).
- `go test ./internal/skill/... ./internal/agent/... ./internal/report/... ./internal/wire/... ./internal/store/sqlite/... ./internal/eval/...`: green. Pre-existing failure: `internal/eval.TestPlatformFloorLockSelfConsistent` (CRLF/hash mismatch in platform-floor lock — same class as the WR-05 fix, but for a different lock; not in scope).
- `cd web && npx tsc --noEmit`: green.
- `internal/server` non-test build (`go build ./internal/server/`): green. The `go test ./internal/server/` link failure (static duckdb) is pre-existing — confirmed by reproducing at the base commit before any fix.

## Fixed Issues

### CR-01: report_publish approve path can never publish

**Files modified:** `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/hitl_report_publish_test.go` (new), `internal/skill/manifest.go`
**Commit:** `186b441`
**Applied fix:** `publishReport` now re-encodes the JSON-round-trip `args.Template` (the actual post-boundary shape — never `*report.ReportTemplate`) into the typed struct via `report.ParseTemplate`, then validates fail-closed before any publish side effect. New `hitl_report_publish_test.go` covers Pass 1 interrupt + Pass 2 publish from the JSON-round-trip shape + invalid-template rejection. The `manifest.go:73` comment that asserted a direct `*report.Template` assertion is corrected.

### CR-02: Data-channel fan-out tool is dead code

**Files modified:** `internal/agent/fanout_tools.go`, `internal/agent/fanout_tools_test.go`, `internal/agent/runner_setup.go`
**Commit:** `82aa1d9`
**Applied fix:** `BuildRunner`'s data-channel branch now wires `BuildFanOutDispatchToolDataChannel(titles, kpis, cfg.KPIExecutor, cfg.SessionID, cfg.SkillID)` (titles + KPIs extracted via the new `sectionTitlesAndKPIs` helper) instead of the Phase-13 tier-channel tool whose `ResolvedTiers()` validation rejected every data-channel call. The unused `period` constructor param was dropped. An interface seam (`KPICaller`) on the data-channel tool's executor field makes the tool hermetic-testable; new tests cover `sectionTitlesAndKPIs` (extraction + index alignment + defensive shapes), V2 `fan_out_success` (with per-section per-KPI call counts), and every D18-C4 fallback path (no executor touch).

### CR-03: request_change third state rejected by ApprovalDecision.Validate

**Files modified:** `internal/agent/hitl/approval_decision.go`, `internal/agent/hitl/audit_writer.go`, `internal/agent/hitl/registry_test.go`, `internal/store/store.go`, `internal/store/sqlite/audit.go`, `internal/server/approval_api.go`, `internal/server/plan_api.go`
**Commit:** `f3d6f19`
**Applied fix:** `validDecisions` includes `request_change`; `Validate()` requires a non-empty reason for `deny` AND `request_change`. New `store.Store.MarkChangeRequested` interface method + `SQLiteStore.MarkChangeRequested` implementation (writes `action_change_requested` row, re-checks reason required) + `AuditWriter.MarkChangeRequested` companion. Both `approvalPostHandler` and `planApproveHandler` now branch three-way (deny → MarkRejected, request_change → MarkChangeRequested, default → MarkApproved); `approvalAlreadyResolved` + the GET resolved map include the new action. The resolution SSE event for `request_change` is the existing `wire.DecisionPlanChangeRequested` value. `TestApprovalDecisionValidate` extended with valid request_change + reason-required rejection cases.

### CR-04: report.Publish never writes the reports DB row

**Files modified:** `internal/report/publish.go`, `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/hitl_report_publish_test.go`
**Commit:** `a1b1481`
**Applied fix:** `Publish` now accepts a `ReportAuditSink` (narrow `PutReport` interface); after the file write succeeds it appends the reports row (period/skill/version/title/file_path/approval_audit_id/kpi_call_ids/template_version/result_digest). Ordering contract: file FIRST, then DB row; a failed audit write returns an error carrying the file path so the failure is recoverable, never silent. The previous `_ =` suppression block is removed; `ctx`, `skillID`, `approvalAuditID`, `kpiCallIDs` are now real inputs. `PublishResult` carries the template version + digest so the WR-02 emit hook + the audit row stay field-aligned. The HITL tool refuses to publish when the sink is missing (CR-04 fail-closed). Tests cover: the row is appended on success, the fail-closed refusal, the recoverable failure (error carries the file path).

### CR-05: Path traversal in report.Publish — period used unvalidated

**Files modified:** `internal/report/publish.go`, `internal/report/publish_test.go` (new), `internal/agent/hitl/hitl_report_publish.go`
**Commit:** `c2efefc`
**Applied fix:** New exported `report.IsValidPeriod` (regex `^[A-Za-z0-9_-]+$`) enforced at BOTH boundaries — the HITL tool Pass 1 (fail-closed before the interrupt) AND `report.Publish` (defense in depth before any `filepath.Join`/`MkdirAll`/`WriteFile`). The HITL tool returns an error naming the invalid period; `Publish` rejects it before touching the filesystem. New tests cover: a table of valid periods (production YYYY-MM, ISO quarters, plain names) and invalid forms (`../`, `..`, path separators, absolute, drive fragments, dot, encoded traversal, empty, newline escape); a publish test that a traversal period leaves the filesystem untouched.

### CR-06: Publish called with a nil Snapshot — nil-interface panic

**Files modified:** `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/hitl_report_publish_test.go`, `internal/report/publish.go`, `internal/report/render.go`
**Commit:** `2b0c183`
**Applied fix:** `ReportPublishArgs` gains `Snapshot map[string]any json:"snapshot,omitempty"` (kpi_call-produced values keyed by KPI name). The `ReportPublishArgs` Info() schema advertises `snapshot` as optional. New `report.MapSnapshot` adapter implements `report.Snapshot.Get`. `publishReport` binds the adapter; a template that declares KPIs without any snapshot is a hard error (`insufficient_kpi_data` — the Skill's required_refusal_phrases vocabulary), never a panic. `buildMarkdown` adds the nil-snap fail-safe (`if len(sec.KPIs) > 0 && snap != nil`). New tests: binding a populated snapshot renders the KPI value; missing-snapshot with a KPI-declaring template fails closed (no file written).

### WR-01: HITL tool audit writer is never injected in production

**Files modified:** `internal/agent/hitl/registry.go`, `internal/agent/hitl/hitl_plan_approve.go`, `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/hitl_report_publish_test.go`, `internal/agent/runner_setup.go`, `internal/server/runner_setup.go`, `internal/server/approval_api.go`, `internal/server/plan_api.go`
**Commit:** `1e3a8bd`
**Applied fix:** New exported `hitl.InjectDeps(audit, reports, emit)` iterates the registry and sets the unexported `audit`/`reports`/`emit` fields on handlers that implement the matching binder interfaces (setAudit, setReportSink, setEmit). `runner_setup.go` (agent) gains `RunnerBuildConfig.AuditWriter *hitl.AuditWriter` + `ReportAudit report.ReportAuditSink`; BuildRunner calls `InjectDeps` when any non-nil. The three server call sites (ChatHandler + approvalPostHandler + planApproveHandler) build a PER-REQUEST, session-bound `*hitl.AuditWriter` via `NewAuditWriter(s.Store, agent.PlatformFloorVersion).WithIdentity(sessionID, "", activeSkillSnapshot(manifest))` — a fresh writer (cheap struct) avoids mutating the shared `Deps.AuditWriter`'s ambient identity, which the concurrent approval endpoints rely on. Same wiring path passes `s.Store` as the CR-04 reports audit sink. Regression test `TestInjectDepsPass1PersistsPendingApprovalRow` drives a Pass-1 interrupt through the registry singleton after a real `InjectDeps` call against a real in-memory SQLite store and asserts a `action_pending_approval` row lands with the verbatim frozen args + target + D11-C2 snapshot.

### WR-02: BE/FE ReportCard contract broken three ways

**Files modified:** `internal/wire/chunk.go`, `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/registry.go`, `internal/agent/hitl/hitl_report_publish_test.go`, `internal/agent/runner_setup.go`, `internal/server/runner_setup.go`, `internal/server/approval_api.go`, `web/src/types/wire-types.ts`, `web/src/components/chat/MessageBubble.tsx`
**Commit:** `f22546f`
**Applied fix:** The tool now emits `wire.DecisionPayload{decision: "report_published", reportCard: {...}}` via an injected `hitl.ReportPublishEmitFunc` closure, hooked through `InjectDeps` (third binder `setEmit`) at BuildRunner construction; the server passes a closure using `runnerEmitDecision` for both ChatHandler and approvalPostHandler (the two places where a Pass-2 publish can run). BE `wire.ReportCard` drops `VFHref` and now carries `{title, period, version, url}`; the duplicated `PointReference` comment is fixed and the doc says "carried inside DecisionPayload.ReportCard" (not DataRef). TS `wire-types.ts` drops the never-emittable `{type:'report_published'}` union member, adds `reportCard?: ReportCard` + the two new decision literals (`report_published`, `plan_change_requested`) to `DecisionPayload`, and aligns the `ReportCard` field set. `MessageBubble.tsx` branches on `data-decision{decision:'report_published', reportCard}`, extracting a `ReportCardView` component that also gates the href to same-origin `/reports/...` (the WR-10 safe-link behavior — the only way to render the link).

### WR-03: ApprovalCard report structure reads nonexistent fields

**Files modified:** `internal/report/publish.go`, `internal/agent/hitl/hitl_report_publish.go`, `internal/agent/hitl/hitl_report_publish_test.go`, `web/src/components/ApprovalCard.tsx`
**Commit:** `e83ac7d`
**Applied fix:** New `report.ParseTemplate` consolidates the JSON-round-trip template conversion + Validate (CR-01 + WR-06) in one seam. New `report.TemplateHash` is the stable content hash (titles + KPI name sets) — exported so the HITL tool can freeze the SAME identifier the reports row carries into the frozen args the operator reviews. Pass 1 in the HITL tool now re-marshals the args with `templateVersion` injected and uses the REWRITTEN bytes for both the audit row AND `StatefulInterrupt` (the D11 byte-equality invariant is preserved — all three copies of the args agree). `ReportPublishArgs` gains `templateVersion string json:"templateVersion,omitempty"` (NOT an LLM-visible schema param — server-derived). TS `ApprovalCard.tsx` gets a narrowed `ReportPublishFrozenArgs` interface (replaces `as any` casts) and reads `template.sections` with `Array.isArray` guards + renders the `templateVersion` from the BE-computed hash.

### WR-04: CheckNarrativeConsistency is a stub + markers stripped

**Files modified:** `internal/report/format.go`, `internal/report/format_test.go` (new), `internal/report/render.go`
**Commit:** `1e8ab6b`
**Applied fix:** `CheckNarrativeConsistency` now accepts the `*ReportTemplate` (signature change — the previous stub discarded the template). Implementation: for each section, candidate values come from THIS section's KPI snapshot (numeric only — non-numeric KPIs silently dropped as candidates); each number in the slot text is checked against the candidates within ±0.5% relative tolerance, including the ×100 form (a "1.23%" narration matches a 0.0123 snapshot value). Per-number allowed-expression exemption (an "环比"/"同比" keyword within 24 bytes before the number — wider window than the prior `12` because 2-rune CJK keywords got truncated at 12). ISO `YYYY-MM` period stamps stripped before number scanning (no false positives on "2026"/"08"). 4-digit years skipped. `nil` snapshot / template → Clean (Render's fail-safe). Markers are now MARKDOWN BLOCKQUOTES (`> ⚠️ ...`); goldmark renders `<blockquote>` which bluemonday UGCPolicy allows — they survive the pipeline AND are visible to the operator (the stated "标记" semantics). Tests cover: mismatch detection, percent ×100 matching, allowed-expression per-number skip, nil-snapshot clean, marker text reaches the published HTML, old HTML-comment marker absent.

### WR-05: Frozen eval lock integrity broken

**Files modified:** `eval/frozen/fanout.lock.json`
**Commit:** `020f610`
**Applied fix:** `eval_set_sha256` now matches the LF-normalized golden-file blob (`cd0e54efcb8695601eae72102333c97c6bee97361a82b8317daaecafe84eaf32` — computed via `git cat-file blob HEAD:eval/golden/fanout.jsonl | sha256sum`); the previous `bfe8bbff…` matched neither the LF blob nor the CRLF working-tree copy. `metrics.monthly_report_fanout_rate_actual: null` + a `_monthly_report_re_measure_required` annotation noting the CR-02 wiring fix and that an operator re-run on the wired path is required before those rows count as gate evidence. `_lock_sha256_convention` documents the byte-exact algorithm (strip the `lock_sha256` line, sha256 the remainder). New `lock_sha256 = 68a3f7863f354a9743a21e696488e2ba1631ffde96009c95ed65859e21c4fd91` (recomputed; the old `f3566e52…` cannot be reproduced because the file body itself changed). `gate_verdict` now annotates the MONTHLY_REPORT rows as NOT gate evidence. Operator signature dates preserved (I cannot re-sign for the operator).

### WR-06: Template validation is fail-open at load

**Files modified:** `internal/skill/loader.go`, `internal/skill/loader_kpi_template_test.go` (new)
**Commit:** `af43277`
**Applied fix:** `validateTemplateKPIs` is now fail-closed: ONLY a nil template is skipped (a Skill without a template is fine); every structural surprise — non-map template, missing `sections`, non-list `sections`, non-map section entry, non-list `kpis`, non-string KPI name, unknown KPI name — rejects the skill load at boot. The previously skipped "section entry not a map" and "kpi entry not a string" cases now error. Sections may legitimately declare no KPIs (absent OR `null`) — preserved. Existing `TestValidateAgainstKPI` (uses the real `kpis/` corpus) still passes. New `TestValidateTemplateKPIs_StructuralSurprises` covers every rejection path; `TestValidateTemplateKPIs_HappyPath` + `_SectionWithoutKPIsIsOK` cover the green paths.

### WR-07: mergeSnapshots returns the first snapshot and discards the rest

**Files modified:** `internal/agent/fanout_tools.go`, `internal/agent/fanout_tools_test.go`
**Commit:** `a4bdb6e`
**Applied fix:** `mergeSnapshots(snap []*kpi.Snapshot) []*kpi.Snapshot` now returns the FULL slice (nil entries filtered — a refused kpi_call leaves no snapshot behind, that is the refusal posture, not data loss). The tool's `results` becomes `[][]*kpi.Snapshot` (per-section array of per-KPI snapshots); the success-path JSON surfaces every KPI the section produced (`section_results[i]` is the slice, not a single snapshot). Tests: `TestMergeSnapshotsPreservesAll` (3 inputs → 3 outputs, nil filter) + `TestDataChannelFanOutV2SectionResultsPreserveAllKPIs` (section 0 has 2 entries from its 2 KPIs).

### WR-08: Data-channel fan-out KPI calls are unattributable

**Files modified:** `internal/agent/fanout_tools.go`, `internal/agent/fanout_tools_test.go`, `internal/agent/runner_setup.go`
**Commit:** `c2c0afb`
**Applied fix:** `BuildFanOutDispatchToolDataChannel(sectionTitles, sectionKPIs, executor, sessionID, skillID)` — the new `sessionID` + `skillID` params are stored on the tool and threaded into every `kpi.CallRequest` (the previous `SessionID: ""` / `SkillID: ""` orphaned every kpi_calls audit row). `runner_setup.go` passes `cfg.SessionID, cfg.SkillID` from the production wiring path. The CR-02 test `TestDataChannelFanOutV2Success` now also asserts every captured `Call` carries the wired session + skill IDs.

### WR-09: vN version race — concurrent publishes overwrite each other

**Files modified:** `internal/store/sqlite/schema.go`, `internal/store/sqlite/reports_test.go` (new)
**Commit:** `3226aa8`
**Applied fix:** New `CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_period_version ON reports(period, version)` registered in the `migrate()` tail (same idempotent convention as the other reports indexes — `CREATE INDEX IF NOT EXISTS` against the fresh-DB schema; a migrated DB picks it up via the tail exec). The DB now rejects the duplicate PutReport at the boundary; `report.Publish`'s existing file-first / recoverable-error-with-path posture (CR-04) surfaces the failure. New tests: `TestReportsPeriodVersionUnique` (duplicate → error), `TestReportsPeriodVersionAllowsDifferentVersion` (v1+v2 coexist on same period — the index is (period, version), not period alone), `TestReportsIndexPresentAfterMigrate` (sqlite_master check — the index lands on a fresh DB without a separate ALTER pass).

### WR-10: Hardening gaps on the report surface

**Files modified:** `internal/server/server.go`, `web/src/components/chat/MessageBubble.tsx`
**Commit:** `d88bb2d`
**Applied fix:** `reportsRoute` now sets `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'`, `Cache-Control: no-cache` (a published vN is mutable — re-publish replaces the file), and `X-Content-Type-Options: nosniff` on every served HTML. The FE href gate lands in WR-02's `MessageBubble.tsx ReportCardView` (`card.url.startsWith('/reports/')` — anything else renders as text, not as a clickable link); the same hygiene applies to the `<a target="_blank" rel="noopener noreferrer">`.

---

_Fixed: 2026-08-28_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_