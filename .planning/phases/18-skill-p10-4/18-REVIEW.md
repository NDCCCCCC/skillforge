---
phase: 18-skill-p10-4
reviewed: 2026-08-28T00:00:00Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - eval/frozen/fanout.lock.json
  - eval/golden/fanout.jsonl
  - go.mod
  - go.sum
  - internal/agent/fanout_tools.go
  - internal/agent/hitl/hitl_plan_approve.go
  - internal/agent/hitl/hitl_report_publish.go
  - internal/agent/hitl/registry.go
  - internal/agent/hitl/registry_test.go
  - internal/agent/hitl/threat_mitigations_test.go
  - internal/agent/runner_setup.go
  - internal/agent/tools/report_publish_tool.go
  - internal/report/format.go
  - internal/report/publish.go
  - internal/report/render.go
  - internal/report/template.go
  - internal/runtime/deep_agent.go
  - internal/runtime/factory.go
  - internal/server/server.go
  - internal/skill/examples_validate_test.go
  - internal/skill/loader.go
  - internal/skill/manifest.go
  - internal/skill/validate.go
  - internal/skill/validate_test.go
  - internal/store/sqlite/reports.go
  - internal/store/sqlite/schema.go
  - internal/store/store.go
  - internal/wire/chunk.go
  - web/src/components/ApprovalCard.tsx
  - web/src/components/PlanApprovalCard.tsx
  - web/src/components/chat/ChatInput.tsx
  - web/src/components/chat/MessageBubble.tsx
  - web/src/store/app-store.ts
  - web/src/types/wire-types.ts
findings:
  critical: 6
  warning: 10
  info: 11
  total: 27
fix_iteration: 1
fix_applied_at: 2026-08-28T00:00:00Z
fixed_critical: 6
fixed_warning: 10
skipped_critical: 0
skipped_warning: 0
status: fixed
---

# Phase 18: Code Review Report

**Reviewed:** 2026-08-28
**Depth:** standard (with cross-file tracing of the report:publish / fan-out call chains)
**Files Reviewed:** 33
**Status:** issues_found

## Summary

Phase 18 ships the monthly-report Skill surface: three-guard conditional allow, `internal/report` render/publish sandbox, `report:publish` HITL handler, reports SQLite table + static route, data-channel fan-out variant, and the FE report card / request-change / pre-fill UI.

The **rendering sandbox itself is sound**: goldmark is configured WITHOUT `html.WithUnsafe()` (render.go:34-38), bluemonday UGCPolicy+tables sanitizes LAST in the pipeline (render.go:68), slot text and KPI cells are escaped before markdown assembly, and the `/reports/*` allowlist regex (`^[A-Za-z0-9_-]+/v[0-9]+\.html$`) constructively rejects `../`, `%2e%2e` (decoded or not), backslashes, and every other traversal form. SQL in reports.go is fully parameterized. The Go build, Go tests, and `tsc --noEmit` all pass.

However, tracing the actual call chains shows that **the phase's headline features are not reachable end-to-end**:

1. The approve path of `report:publish` **cannot ever succeed** (a `map[string]any` can never type-assert to `*report.ReportTemplate` after a JSON round-trip), and would nil-panic on the KPI snapshot even if it did.
2. The data-channel fan-out tool that all of 18-03 built (`BuildFanOutDispatchToolDataChannel`) has **zero callers** — runner_setup.go wires the Phase-13 tier-channel orchestrator tool instead.
3. The `request_change` third state is **rejected by `ApprovalDecision.Validate()`** (valid decisions are only `approve`/`deny`), so the FE button always gets a 422.
4. `report.Publish` **never writes the reports DB row** — the store layer, schema, and indexes are dead code, and the digest/templateVersion/approvalAuditID are explicitly discarded.
5. The BE never emits the `report_published` / ReportCard SSE payload the FE renders.

Pattern across the phase: unit-level pieces were built and tested in isolation, but the wiring between them was never completed, and no integration test covers the two-pass publish flow (there is no `hitl_report_publish_test.go` at all, and `internal/report` has zero test files). The summaries' "Self-Check: PASSED" grep-based verifications masked all of this.

## Critical Issues

### CR-01: report_publish approve path can never publish — type assertion on JSON-unmarshaled `any` always fails

**File:** `internal/agent/hitl/hitl_report_publish.go:206-213`

**Issue:** `ReportPublishArgs.Template` is `any` (line 54). In Pass 1 the args arrive as LLM JSON and `json.Unmarshal` produces `map[string]any` for `template`; `tool.StatefulInterrupt(ctx, info, args)` stores the raw JSON string. In Pass 2 the stored args are unmarshaled again (line 179), so `args.Template` is again `map[string]any` — never `*report.ReportTemplate`. The assertion at line 208 therefore always fails:

```go
tmpl, ok := args.Template.(*report.ReportTemplate)
if !ok {
    return nil, errors.New("report_publish: template is not a *report.Template")
}
```

Consequence: after the operator clicks 批准, Pass 2 always returns `report_publish: publish failed: report_publish: template is not a *report.Template`. There is no code path — none — in which a `*report.ReportTemplate` can occupy this field, because the value always crosses a JSON boundary. The entire two-pass HITL publish feature is inoperative. (`manifest.go:73` carries the same wrong comment — "callers type-assert to *report.Template after yaml.Unmarshal" — YAML also yields `map[string]any`.)

**Fix:** Convert the generic map into the typed struct instead of asserting:

```go
raw, err := json.Marshal(args.Template)
if err != nil {
    return nil, fmt.Errorf("report_publish: re-encode template: %w", err)
}
var tmpl report.ReportTemplate
if err := json.Unmarshal(raw, &tmpl); err != nil {
    return nil, fmt.Errorf("report_publish: decode template: %w", err)
}
if err := tmpl.Validate(); err != nil { // see WR-06
    return nil, fmt.Errorf("report_publish: template invalid: %w", err)
}
```

Add a Pass-1→Pass-2 integration test (`hitl_report_publish_test.go` does not exist) that drives both passes through `tool.GetInterruptState`/`GetResumeContext` with JSON args.

### CR-02: Data-channel fan-out tool is dead code — runner_setup wires the Phase-13 tier-channel tool instead

**File:** `internal/agent/runner_setup.go:264-267`

**Issue:** For data-channel Skills with a template, BuildRunner appends `BuildFanOutDispatchTool(cfg.Manifest)` — the Phase-13 orchestrator tool whose schema is `sub_queries[{id,query,tier_filter}]` and whose validation rejects every tier not in `parentManifest.ResolvedTiers()`. For a data-channel Skill, `resolvedTiers` is empty (validate.go:134-136 deliberately skips ResolveTiers), so *any* V1-shaped call errors, and the LLM — prompted by `deepArchetypePrefix` (deep_agent.go:193-201) with the V2 schema `{should_fan_out, num_subtasks, task_boundaries}` — guarantees a schema mismatch that V1 rejects with `at least one sub-query required`. Fan-out is broken in both directions.

Meanwhile `BuildFanOutDispatchToolDataChannel` (fanout_tools.go:244) — the entire subject of 18-03 Task 1 and the 4 golden cases `rpt-fanout-01..04` — has **zero callers** in the repo (verified by grep). The `monthly_report_fanout_rate` eval metric therefore measures an unreachable path.

**Fix:** In the data-channel branch, extract section titles/KPIs from the manifest template and build the data-channel tool:

```go
if cfg.Manifest.Template != nil {
    titles, kpis := sectionTitlesAndKPIs(cfg.Manifest.Template) // extract from map[string]any
    tools = append(tools, BuildFanOutDispatchToolDataChannel(
        titles, currentPeriod(cfg), kpis, cfg.KPIExecutor))
}
```

(Also note the data-channel tool currently has no source for `period` or per-section KPIs — that extraction must be added — and add a wiring test asserting the tool registered for `monthly-report` responds to V2 args with `fan_out_success`.)

### CR-03: `request_change` third state is rejected by ApprovalDecision.Validate — D18-D4 broken end-to-end

**File:** `internal/agent/hitl/hitl_report_publish.go:173,190-193` (root cause: `internal/agent/hitl/approval_decision.go:49-52,157-165`)

**Issue:** `validDecisions` is only `{"approve", "deny"}`; `ApprovalDecision.Validate()` returns `field decision: must be one of [approve deny], got "request_change"`. The approval endpoints (approval_api.go:338, plan_api.go:274) validate BEFORE resume, so the FE's `handleRequestChange` POST (ApprovalCard.tsx:237-242) always receives HTTP 422 and the card settles to `failed` ("操作失败：HTTP 422"). The `request_change` branch at hitl_report_publish.go:190-193 is **unreachable dead code** — `data.Validate()` at line 173 already rejected the decision. The `DecisionPlanChangeRequested` wire constant (chunk.go:366) has no emitter anywhere.

**Fix:** Add `request_change` to `validDecisions` (require a non-empty reason for it, mirroring the deny rule), teach `approvalPostHandler` a third audit outcome (e.g. a new `action_change_requested` row — the current code path would MarkApproved any non-deny), then update `TestApprovalDecisionValidate`. The tool-side branch (line 190) then becomes reachable and must also verify the endpoint actually marks — not approves — a request_change.

### CR-04: report.Publish never writes the reports DB row — audit trail lost, store layer is dead code

**File:** `internal/report/publish.go:72-84`

**Issue:** Publish renders and writes the HTML file, then explicitly discards every audit input:

```go
// Suppress unused-variable warnings
_ = templateVersion
_ = digest
_ = skillID
_ = approvalAuditID
_ = kpiCallIDs
_ = ctx
```

`store.PutReport` is never called anywhere in the repo (verified by grep); `ListReports`/`GetReport`/`GetLatestReport`/`ListReportsBySkill` likewise have zero production callers. The `reports` table, its DDL, and both indexes (schema.go:301-313,441-448) will remain forever empty. Consequences: (a) no append-only record ties a published report to its approval row or its KPI call IDs — the exact audit linkage the 18-02 summary claims (T-18-B2/T-18-B3 "mitigated"); (b) `PublishResult` cannot return the DB-backed metadata; (c) the `_ =` block is a red flag that the integration was stubbed and never finished. This is a broken invariant / audit-trail gap on a compliance-facing artifact.

**Fix:** Accept a `store.Store` (or a narrow `ReportAuditWriter` interface) in `Publish` (or in `reportPublishTool`), and after the file write succeeds:

```go
row := store.ReportRow{TS: time.Now().UTC(), Period: period, SkillID: skillID,
    Version: version, Title: tmpl.Title, FilePath: filePath,
    ApprovalAuditID: approvalAuditID, KPICallIDs: string(kpiJSON),
    TemplateVersion: templateVersion, ResultDigest: digest}
if err := store.PutReport(ctx, row); err != nil {
    return nil, fmt.Errorf("report: audit row write failed (file=%s): %w", filePath, err)
}
```

Note the ordering: file first, then DB row, and return the file path in the error so a failed DB write is recoverable (T-18-B3 posture).

### CR-05: Path traversal in report.Publish — `period` from LLM tool args is used unvalidated in filepath.Join

**File:** `internal/report/publish.go:61-70,93-97` (validation gap: `internal/agent/hitl/hitl_report_publish.go:132-134`)

**Issue:** Pass 1 validates only `parsed.Period == ""`. `Publish` then does `dir := filepath.Join("reports", period)` and `os.MkdirAll(dir, 0755)` + `os.WriteFile`. A prompt-injected LLM can emit `period: "../scheduled-tasks/x"` (or any `..`-containing, absolute, or Windows-drive form) and the publish writes `v<N>.html` outside the reports directory, creating directories along the way. The GET route blocks traversal (server.go:78-96) but the *write* path — the more dangerous direction — does not. The only mitigations are the approval Reason string (weak: an operator can miss `../` in "Report publication request for period ../x") and the fixed `v%d.html` basename.

**Fix:** Validate `period` against the same character class the serve path uses, in BOTH the HITL tool and `Publish` (defense in depth):

```go
var periodRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
if !periodRE.MatchString(period) {
    return nil, fmt.Errorf("report: invalid period %q", period)
}
```

### CR-06: Publish is called with a nil Snapshot — nil-interface panic on the first KPI

**File:** `internal/agent/hitl/hitl_report_publish.go:212` → `internal/report/render.go:98`

**Issue:** `report.Publish(ctx, tmpl, args.Slots, args.Period, nil, "", args.KPICallIDs, args.SkillID)` passes literal `nil` for the `Snapshot` parameter. `buildMarkdown` calls `snap.Get(kpi)` for every section KPI (render.go:98) — a nil interface method call panics. Every monthly-report section declares KPIs, so even after fixing CR-01, the approve path deterministically panics mid-run (taking down the request goroutine / the resumed agent run). The frozen args also carry no KPI values, so there is currently no data source to bind.

**Fix:** Capture the KPI snapshot at Pass 1 (kpi_call results already exist in the run) and persist it in the frozen args (e.g. `Snapshot map[string]any` keyed by KPI name), or re-execute the template's KPIs via `KPIExecutor` in Pass 2 and pass a `mapSnapshot` adapter implementing `report.Snapshot`. Guard `buildMarkdown` with `if snap == nil { continue }` as a fail-safe, and surface "no KPI data" as an error, not a panic.

## Warnings

### WR-01: HITL tool audit writer is never injected in production — `InsertPendingApproval` is a no-op for report:publish and plan_approve

**File:** `internal/agent/tools/report_publish_tool.go:28-34`, `internal/agent/hitl/registry.go:102-108`, `internal/agent/runner_setup.go:252-262`

**Issue:** `BuildReportPublishTool()` returns the registry singleton `&reportPublishTool{}` whose `audit` field is nil; `auditOrDefault()` then returns `noopAuditWriter` (hitl_report_publish.go:71-76) whose `InsertPendingApproval` returns nil without writing. The same is true for `hitl.LookupHandler("plan_approve")` in the data-channel branch. No production code assigns the `audit` field (verified by grep — only test files construct tools with `audit:`). The approval endpoints list pending approvals exclusively from `action_pending_approval` audit rows (approval_api.go:198-202), so with a noop writer the operator-facing approval surface can never see the request and the flow dead-ends. (This is a pre-existing pattern since Phase 11 for memory:facts/review_report — Phase 18 extends it to its new surface, and the D11-C3 "fail-closed INSERT" invariant is silently inverted to fail-silent.)

**Fix:** Give the registry tools a constructor or `WithContext`-style setter for the audit hook and wire `deps.AuditWriter` in at BuildRunner construction; add a regression test that a Pass-1 interrupt produces a visible `action_pending_approval` row via the real `hitl.AuditWriter`.

### WR-02: BE/FE ReportCard contract broken three ways — the FE report card is dead UI

**File:** `web/src/types/wire-types.ts:164-172,405-408`; `internal/wire/chunk.go:171-184,356-367`

**Issue:** (a) Nothing in `internal/` ever emits a chunk carrying `wire.ReportCard` or `Decision="report_published"` (grep-verified) — the struct and constants are declared, never used. (b) The FE listens for a *chunk type* `'report_published'` (wire-types.ts:408, MessageBubble.tsx:124), which the BE neither emits nor permits — `wire.ValidTypes` contains no `report_published`, so the T-1-11 pre-write gate would reject it; the BE design comment says the card "rides the existing data-decision channel". (c) Field mismatch: BE `ReportCard{title, period, version, url, vFhref}` vs TS `ReportCard{period, version, url, sections[]}`; the TS `DecisionPayload` mirror lacks the `reportCard` field and its decision union lacks `report_published`/`plan_change_requested`. The D15 lockstep invariant is violated in both directions. `tsc --noEmit` passes only because the drift alarm cannot see semantic wire mismatch.

**Fix:** Pick one channel and align both sides: emit `data-decision` with `decision:"report_published"` + `reportCard` from the approve branch (e.g. via the decision-emit hook the chatHandler already provides), add `reportCard?: ReportCard` + the two decision literals to the TS `DecisionPayload`, change MessageBubble to branch on `data-decision`, and reconcile the field set (drop `vFhref`, decide whether `sections` is sent).

### WR-03: ApprovalCard report:publish structure summary reads fields that don't exist in frozen args

**File:** `web/src/components/ApprovalCard.tsx:300-320`

**Issue:** The card renders `(approval.frozenArgs as any).sections` and `.templateVersion`, but frozen args are `ReportPublishArgs` JSON: `{period, template, slots, skillId, sessionId, kpiCallIds}` — there is no `sections` key (sections live at `frozenArgs.template.sections`) and no `templateVersion` at all. The structure summary block therefore always renders empty, so the operator approves a report publication without seeing section titles or KPI names — undermining the stated purpose of the D18-D4 summary and the HITL review itself. The `as any` casts hide this from the compiler.

**Fix:** Read `frozenArgs.template?.sections` (with `Array.isArray` guards), derive the template hash on the BE and include `templateVersion` in the frozen args, and replace `as any` with a narrowed `ReportPublishFrozenArgs` interface.

### WR-04: CheckNarrativeConsistency is a stub, and its markers are stripped by both goldmark and bluemonday — D18-A3 "标记" is non-functional

**File:** `internal/report/format.go:112-135`; `internal/report/render.go:55-59,71-79`

**Issue:** Two independent defects: (1) `CheckNarrativeConsistency` extracts numbers but performs no comparison — `_ = fmtFn; _ = snap; _ = si; _ = slotIdx` — and always returns `Clean: true`, so `Mismatches` is never populated; (2) even if it were implemented, `appendConsistencyMarkers` injects `<!-- ... -->` HTML comments into the markdown body, but goldmark without `html.WithUnsafe()` replaces raw HTML blocks with `<!-- raw HTML omitted -->`, and bluemonday's UGCPolicy strips remaining comments (no `AllowComments`). The markers can never appear in the published HTML. The consistency feature is advertised in the summary as "BLOCKER-3 fixed"; it is a no-op end to end.

**Fix:** Implement the comparison (numbers in slot text vs `snap.Get` within ±0.5% tolerance, respecting `allowedExpressions` per-number not per-text), and emit markers as visible sanitizable HTML — e.g. append a `<p class="consistency-flag">…</p>` paragraph (bluemonday allows `p`) or a `data-` attribute on the section — rather than HTML comments.

### WR-05: Frozen eval lock integrity broken — eval_set_sha256 no longer matches the golden file

**File:** `eval/frozen/fanout.lock.json:10,13,27,43,47`

**Issue:** After appending the 4 `rpt-fanout-*` cases, `eval_set_sha256` (`bfe8bbff…`) no longer matches the actual sha256 of `eval/golden/fanout.jsonl` (`cd0e54ef…`, computed during this review). `lock_sha256` (body hash) and `created_at`/`operator_signed_at` (`2026-08-21`, pre-dating the 2026-08-28 cases) are equally stale, while `monthly_report_fanout_rate_actual: 1.0` claims a measured pass for cases that exercise the unreachable wiring (CR-02). Any harness that verifies the lock against the set will fail; if none does, the lock silently misdescribes what it gates — exactly the frozen-set discipline the project's eval gates exist to enforce.

**Fix:** Re-freeze: recompute `eval_set_sha256` + `lock_sha256`, re-run the 4 monthly-report cases against the *wired* path (after CR-02), update the metric or remove it, and re-sign with a current date. Until then the monthly_report threshold rows should not be treated as gate evidence.

### WR-06: Template validation is fail-open at load and absent at publish

**File:** `internal/skill/loader.go:197-235`; `internal/report/template.go:50-68`

**Issue:** `validateTemplateKPIs` silently `return nil` whenever the YAML shape differs from its expectation (template not a map, `sections` missing, `kpis` not a list, entry not a string). A `template:` block with any structural surprise skips KPI cross-validation entirely — the opposite of the stated fail-closed posture (unknown KPIs must reject the skill load). Additionally, `ReportTemplate.Validate()` has zero callers in the repo (grep-verified): neither the loader nor the publish path ever checks section count, empty titles, or slot-type enum, so an LLM-supplied template with arbitrary slot types/structure flows straight into render on approve.

**Fix:** In `validateTemplateKPIs`, return a typed error when `template` is a non-nil map whose `sections` is absent/malformed (only skip for nil template); call `tmpl.Validate()` after converting the frozen template (see CR-01 fix) and surface failures as publish errors.

### WR-07: mergeSnapshots returns the first snapshot and discards the rest — KPI data loss per section

**File:** `internal/agent/fanout_tools.go:341,363-370`

**Issue:** Despite its name and comment ("merges multiple KPI snapshots into one per section"), `mergeSnapshots` returns the first non-nil snapshot only. A section with 3 KPIs loses 2 of 3 results; the fan-out response then reports `fan_out_success` with silently incomplete data. (Moot until CR-02 is fixed, but the function is wrong and will be inherited by the fix.)

**Fix:** Merge properly — concatenate rows/points under a `map[string]any` keyed by KPI name, or return `[]*kpi.Snapshot` per section and let the renderer iterate; add a table test with 2+ KPIs per section.

### WR-08: Data-channel fan-out KPI calls are unattributable in the audit trail

**File:** `internal/agent/fanout_tools.go:324-329`

**Issue:** Every `kpi.Executor.Call` issued by the fan-out tool passes `SessionID: ""` and `SkillID: ""`, so all `kpi_calls` audit rows produced during fan-out are orphaned — they cannot be linked to the session, the skill, or (via CR-04's `kpi_call_ids`) the published report. This defeats the KPI-02 replay/audit guarantee for the monthly-report path.

**Fix:** Thread `sessionID`/`skillID` into `dataChannelFanOutTool` (extend `BuildFanOutDispatchToolDataChannel`) and pass them through every `Call`.

### WR-09: vN version race — concurrent publishes of the same period overwrite each other, and the schema has no UNIQUE(period, version)

**File:** `internal/report/publish.go:55,68,93-123`; `internal/store/sqlite/schema.go:301-313,441-448`

**Issue:** `getNextVersion` scans the directory (TOCTOU), then `os.WriteFile` truncates — two approved publishes for the same period racing between scan and write produce the same `vN` and silently overwrite one report. The 18-02 summary claims a "Unique index: (period, version)", but the shipped DDL creates only non-unique `idx_reports_period`/`idx_reports_skill_id` — the DB-level backstop does not exist (and CR-04 means no rows are written anyway).

**Fix:** Add `CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_period_version ON reports(period, version)`; when PutReport is wired (CR-04), treat a UNIQUE violation as "retry with version+1" or fail loudly; optionally hold a per-period file lock around scan+write.

### WR-10: Hardening gaps on the report surface — no CSP on served HTML, unvalidated href from wire data

**File:** `internal/server/server.go:82-96`; `web/src/components/chat/MessageBubble.tsx:170-183`

**Issue:** (a) `/reports/*` serves `text/html` with no `Content-Security-Policy` header; bluemonday containment is good but a CSP (`default-src 'none'; style-src 'unsafe-inline'`) is cheap defense-in-depth for operator-facing artifacts. (b) MessageBubble renders `<a href={card.url}>` from a wire payload without checking it is a same-origin `/reports/…` path; React blocks `javascript:` URLs in modern versions, but a relative/protocol-relative or off-site URL would still render as a clickable link inside a trusted card. No Cache-Control on mutable vN content is minor but worth a line.

**Fix:** In `reportsRoute`, set `Content-Security-Policy` (and `Cache-Control: no-cache`); in MessageBubble, gate the link: `const safe = card.url.startsWith('/reports/') ? card.url : null`.

## Info

### IN-01: Direct dependencies misclassified as `// indirect`

**File:** `go.mod:65,87,94` — `bluemonday`, `goldmark`, and `golang.org/x/sync` are direct imports of `internal/report` / `internal/agent` but sit in the indirect block. Run `go mod tidy`.

### IN-02: Zero tests for the security-critical report package and the new HITL tool

**File:** `internal/report/` (no `*_test.go`), `internal/agent/hitl/hitl_report_publish.go` (no test file). `go test ./internal/report/...` "passing" is vacuous. The render sandbox (goldmark config, bluemonday policy, cell escaping) and the two-pass contract need golden-output tests; the 18-01/18-02 self-checks cited that vacuous pass as verification.

### IN-03: Dead store surface + import suppression

**File:** `internal/store/sqlite/reports.go:213` — `var _ = strings.TrimSpace` masks an unused import; all five report store methods currently have no caller (see CR-04). Once wired, delete the suppression; if not wired this phase, delete the methods rather than ship unreachable surface.

### IN-04: Stale registry comments and formatting drift

**File:** `internal/agent/hitl/registry.go:103-108,118-121` — comments say "currently 2 entries"/"Phase 12 may grow to 3-4" (now 5 targets); the `hitlHandlers` map literal has inconsistent alignment (would be normalized by gofmt).

### IN-05: Stale line-range comment on the MaxIteration pin

**File:** `internal/agent/hitl/threat_mitigations_test.go:89-93` — the constant's comment still says "lines 92-97" while the call site (line 339) correctly hashes 106-111. Comments and hashes should move together or the next rotation will be mis-diagnosed.

### IN-06: examples gate does not pin the new skill's name

**File:** `internal/skill/examples_validate_test.go:90-96` — the count assertion requires 6 manifests but the name list omits `monthly-report`; a renamed/misnamed monthly-report directory would pass. Add it to the `want` slice.

### IN-07: wire/chunk.go comment and tag defects

**File:** `internal/wire/chunk.go:174-184,186-189` — the ReportCard godoc says "carried inside DecisionPayload.DataRef" but the field is `DecisionPayload.ReportCard`; the `// PointReference is one data point…` comment block is duplicated verbatim; `VFHref`/`json:"vFhref"` is anomalous casing for this codebase; BE ReportCard diverges from the 18-02 summary shape (`versions`, `regenerateHint` absent). Clean these up while fixing WR-02.

### IN-08: ChatInput pendingDraft can go stale and pop up later

**File:** `web/src/components/chat/ChatInput.tsx:135-140` — when `text` is non-empty, the pre-fill is neither applied nor consumed; after the user sends/clears, the stale draft suddenly fills the composer. Consume-or-drop explicitly: `if (pendingDraft) { if (!text) setText(pendingDraft); setPendingDraft(''); }`.

### IN-09: fan-out fallback loses diagnostics; empty-boundary success

**File:** `internal/agent/fanout_tools.go:299-304,307-309,312-356` — the offending boundary returned by `validateTaskBoundaries` is discarded (detail is a hardcoded string), which will hamper LLM self-correction; `task_boundaries: []` + `num_subtasks: 0` passes validation and returns `fan_out_success` with zero work. Require `len(boundaries) >= 1` and include the offending value in `detail` (JSON-escaped).

### IN-10: Formatter doc/impl mismatches

**File:** `internal/report/format.go:26-60,137-144` — `DefaultValueFormatter` documents "thousand-separator + 2 decimal places" but `formatFloat` applies no grouping (only `formatInt` does); `isAllowedExpression` ignores its `numStr` argument, so any number in a text containing 第一/环比 passes unexamined; `int64(v)` overflows for |v| ≥ 2^63 (marginal for KPI values, but the cast is unchecked).

### IN-11: D18-D2 prompt-injection surface and duplicated PrevRunContext type

**File:** `internal/runtime/deep_agent.go:88-98`; `internal/agent/hitl/hitl_plan_approve.go:84-96`; `internal/runtime/factory.go:121-133` — operator/LLM-supplied `ModNotes` are concatenated into the system prompt with no delimiting/escaping guidance ("treat the following as data, not instructions" is absent), and two structurally-similar `PrevRunContext` types now exist in `runtime` and `hitl` with different JSON tags (`PrevVersion` vs `prev_version`) — a drift trap. Document the trust boundary and keep one canonical type (hitl already imports nothing from runtime; a small shared package or a documented conversion would do).

---

_Reviewed: 2026-08-28_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
