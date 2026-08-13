# Phase 6: Eval Dashboard & Production Hardening - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning
**Requirements:** GEVL-06, GEVL-07 (2 v1 requirements) — 6 Success Criteria (SC-1..6)

<domain>
## Phase Boundary

Turn the Phase-3-shipped Go-native trust evaluator (`internal/eval` + `cmd/eval`) from a batch CLI emitting `eval/report.json` + `eval/calibration.json` into a **CI promotion gate + visible dashboard + production-grade privacy posture**. This is the closing piece of SkillForge's "可衡量效果" (measurable outcomes) moat — the OSS personal-brand differentiator over a pure interview project. Six SCs:

1. **SC-1 — Per-Skill metrics dashboard with trend (GEVL-06).** A web dashboard surfaces per-Skill metrics — faithfulness, citation precision, refusal accuracy — with a trend line across runs.
2. **SC-2 — CI gate blocks promotion (GEVL-07).** Opening a PR that drops a Skill's faithfulness below its threshold fails the CI gate; the PR comment names the Skill and the threshold.
3. **SC-3 — Queryable audit UI with row-level redaction.** Audit records (from Phase 3 `store.Store.PutAudit`) are queryable via a UI table filterable by user / skill / time range / `refusal_reason`, with row-level redaction toggled by access policy.
4. **SC-4 — Session isolation proven by 50×5 concurrency.** 50 sessions × 5 turns each see only their own conversation history (no cross-session bleed).
5. **SC-5 — Retention / redaction policy enforced.** Queries past the retention window have `query` and `answer` fields redacted in storage; access requires an explicit access policy.
6. **SC-6 — Eval runs reproducible.** Re-running against a frozen eval set + pinned `model_ver` + pinned `prompt_hash` returns numerically identical scores within tolerance.

**In scope:** GEVL-06, GEVL-07 (the only two v1 requirements traced to Phase 6 — see REQUIREMENTS.md traceability), PLUS the 4 SCs that have no dedicated requirement (SC-3 audit UI, SC-4 concurrency, SC-5 retention/redaction, SC-6 reproducibility) since the ROADMAP-authoritative SC list governs phase scope.
**Out of scope:** DATA-* financial data layer (deferred — still owns its own phase/v2), open-source packaging (DIST-* Phase 7), real multi-role RBAC / SSO (v2 SCALE-*), live dashboard push (manual refresh + 30s poll — see D6-03), the `0.3`→`recommended_threshold` manual ops swap (already documented in `eval/README.md` — not Phase 6 work).

**UI hint:** yes — 3 new frontend surfaces (Dashboard full-page route + Trend panel + Audit sub-route) PLUS the existing three-panel layout stays untouched for the chat product.

**Sequencing note:** Phase 6 depends on Phase 5 (shipped UI substrate — LeftSidebar nav, TopBar, store contract `item.manifest.*`) AND Phase 3 (`internal/eval` + `cmd/eval` + `store.Store` + `AuditRow` + `TestRunnerReproducibility`). All wire invariants (D-04/D12/D15/D16) MUST be preserved so the shipped chat UI keeps rendering. Phase 6 adds a NEW route + new API endpoints; it does not modify the chat SSE wire.

</domain>

<decisions>
## Implementation Decisions

### Dashboard UI surface (SC-1, GEVL-06) — D6-01..04
- **D6-01: `/eval` is an independent full-screen route (admin mode).** The dashboard hides the locked three-panel layout (260/flex/420) entirely — entering `/eval` enters "admin mode". Trend graphs + audit tables need horizontal space; a full-screen canvas is the correct substrate. Navigation: `LeftSidebar` (Phase 5) gains an "Eval Dashboard" entry that switches routes; returning to `/` restores the chat three-panel. **Rejected:** embedding the dashboard inside RightChat (420px too narrow for trend graphs); modal/overlay (confined canvas). The three-panel D-04/D12 transport-swap invariant is untouched — `/eval` is a separate route with its own transport (no SSE), so the chat wire is not at risk.
- **D6-02: Trend history source = `eval/reports/*.json` + `eval/reports/INDEX.json` manifest (file-based, git-friendly).** `cmd/eval` gains an `--archive` flag: each run appends `eval/reports/<iso-ts>.json` (the full report) AND updates `eval/reports/INDEX.json` (a lightweight array of `{ts, skill, judge_model, faithfulness, context_precision, answer_relevance, context_recall, refusal_accuracy, frozen_version}`). The Dashboard reads `INDEX.json` for the trend line + lazy-loads individual reports on demand. **Rationale:** file-based, zero extra dependency, OSS contributors can read history in git diff, plays naturally with the existing `eval/report.json` convention. **Rejected:** new sqlite `EvalRun` table (eval metadata ≠ audit — pollutes the audit-store semantic boundary; also INSERT-only rule does not apply to eval metadata); GitHub API pull of Actions history (needs network + rate limits + conflicts with local-first data philosophy).
- **D6-03: Refresh model = manual Refresh button + 30s low-frequency poll.** Dashboard top-bar has a "Refresh" button; the page polls `INDEX.json` + `QueryAudit` every 30s when mounted. Eval runs are minute-scale (5-10 min on CI), so 30s poll is more than sufficient; SSE push of eval-completion would force a second SSE channel alongside the chat stream and complicate the client transport. **Rejected:** SSE push of eval-completion events (mixed transport, not worth it); manual-only (multi-window staleness).
- **D6-04: Dashboard = Skills table + Trend panel; Audit table lives on `/eval/audit` sub-route.** Main `/eval` page = Skills table (one row per Skill: faithfulness / context_precision / answer_relevance / context_recall + derived `refusal_accuracy` from the `refusal-required` golden category + latest-run timestamp + ❌/✅ vs threshold badge) + Trend panel (select a Skill, see its score line over runs). The audit query UI (SC-3) is a separate `/eval/audit` sub-route (reached from a "View Audit Log" link in the sidebar) — keeps the dashboard main page focused on metrics + trend, while still delivering SC-3's audit UI within the same admin mode. **Rejected:** embedding the audit table on the main dashboard page (information-dense, excessive scroll); skipping the audit UI entirely (violates SC-3).

### Audit query + redaction (SC-3 + SC-5) — D6-05..08
- **D6-05: `/api/audit` = parameterized `store.QueryAudit` + LIMIT/OFFSET pagination.** The endpoint accepts query params `user`, `skill`, `since`, `until`, `refusal_reason`, `limit`, `offset` and maps them to the existing `store.AuditFilter` (already 100% `?`-parameterized, T-3-SQLI regression-tested in Plan 03-02). **Rationale:** directly reuses the Phase-3-shipped SQLi-safe filter; zero new SQL surface. **Rejected:** RESTful cursor + server-side sort index (over-engineered for ~50-row dev datasets); GraphQL (extra dep + learning curve for an OSS personal-brand project).
- **D6-06: Retention + redaction = `RetentionDays` parameter + `X-Admin-Token` access split.** `store.QueryAudit` (or a thin server-side wrapper) gains a `RetentionDays` param (default 90, overridable via `SKILLFORGE_RETENTION_DAYS` env). Returned rows past `row.Timestamp + retention` have their `Query`/`Answer`/`VerdictJSON` fields replaced with `"[REDACTED:<reason>]"`. Access policy is a binary admin/non-admin split: requests carrying a valid `X-Admin-Token` (value = `SKILLFORGE_ADMIN_TOKEN` env, default unset → no admin access) receive RAW rows regardless of retention; requests without the token receive retention-redacted rows. **Rationale:** the simplest defensible reading of "access requires an explicit access policy" for a single-developer OSS project; a real RBAC/ABAC layer is v2 (SCALE-*). **Rejected:** background TTL delete task (loses the meta-record "this happened" which matters for compliance audit); per-field encryption-at-rest (loses audit traceability); per-row `access_policy` column (schema change violates T-3-AUDIT INSERT-only invariant + needs admin UI to set).
- **D6-07: Frontend redaction UX = visible `[REDACTED]` badge + hover detail.** The audit table renders redacted `query`/`answer` cells as a gray badge `REDACTED (90d+)` with a hover tooltip showing the row timestamp + retention boundary. Raw content is never expandable from the UI without the admin token. **Rejected:** plain-text `[REDACTED]` (opaque — user can't tell why); hiding expired rows entirely (loses the meta-visibility audit wants).
- **D6-08: Access policy = admin vs non-admin binary (no ACL in v1).** v1 ships a single admin role (the `X-Admin-Token` holder). No per-row `access_policy` column, no per-user ABAC. A real multi-role ACL is explicitly v2 SCALE-* scope — noted in `<deferred>`. **Rationale:** Phase 6 has no user table; building ABAC here would require standing up an auth system that belongs to a later phase.

### CI gate integration (SC-2, GEVL-07) — D6-09..12
- **D6-09: GitHub Actions workflow triggers on `pull_request` only (not on `push` to main).** New `.github/workflows/eval-gate.yml` triggers on `pull_request` events; path-filtered to run when `skills/*/SKILL.md` OR `eval/golden/*.jsonl` OR `eval/frozen/*` OR `internal/eval/**` OR `internal/agent/**` change. `push` to main does NOT re-run eval (avoids polluting history + avoids exposing `GLM_API_KEY` secret on the default branch). **Rejected:** PR + push-main both run (secret exposure on fork contributor path, report pollution); scheduled weekly run (redundant alerts + GLM cost).
- **D6-10: Per-Skill threshold via `.claude/skills/<skill>/eval-gate.json`, default 0.9.** Each Skill MAY carry an `eval-gate.json` declaring its faithfulness threshold (e.g. compliance → `0.9`, a future creative Skill → `0.7`). Skills without the file default to `0.9`. The CI gate reads each Skill's threshold and fails the gate if ANY Skill's `overall.faithfulness < threshold`. Phase 6 ships exactly ONE Skill (compliance) → one threshold = `0.9`. **Rejected:** uniform 0.9 for all Skills (different Skill types have different quality bars — compliance vs creative); dynamic threshold from `eval/calibration.json` (D2-15's `recommended_threshold` is the *retrieval* threshold, not faithfulness — semantic mismatch).
- **D6-11: PR feedback = `gh pr comment` with markdown table + uploaded `report.json`/`calibration.json` artifacts.** A final workflow step parses `eval/reports/<ts>.json`, builds a markdown table (per-Skill: faithfulness / threshold / ❌ or ✅ / refusal_accuracy), and posts it via `gh pr comment`. `report.json` + `calibration.json` are uploaded as workflow artifacts for post-failure debugging. **Rejected:** status-check red-X only (SC-2 explicitly requires a PR comment naming the Skill); Slack/Discord webhook (extra secret + overkill for OSS personal-brand).
- **D6-12: Cost control = all PRs run; manual override deferred to Phase 7.** Phase 6 runs the eval gate on every PR (GLM Coding Plan rates are acceptable). Manual approval / `[skip-eval]` label / `workflow_dispatch` override is added in Phase 7 once fork-contributor frequency is observed. **Rejected:** `[skip-eval]` label from day one (adds yaml complexity before the contributor pattern is known); main-branch-only gate (violates SC-2's "PR-stage block" requirement).

### Reproducibility + cross-eval (SC-6 + Pitfall 8) — D6-13..16
- **D6-13: Frozen eval sets = `eval/frozen/<skill>.jsonl` + sidecar `.lock.json`.** Each Skill ships a frozen eval set (extending the existing `eval/golden/compliance.jsonl` + `internal/eval/testdata/frozen_5.jsonl` pattern to the full N=50 set). Each frozen set has a sidecar `eval/frozen/<skill>.lock.json` pinning `model_ver`, `prompt_hash`, `seed`, `eval_set_version`. `cmd/eval --frozen=<skill>` reads the frozen set + lock, asserts the pin holds, runs metrics. **Rationale:** file-based, diffable, no binary embedding, no LFS overhead. **Rejected:** `go:embed` into the binary (inflexible — change requires rebuild); Git LFS (repo complexity overkill for ~50-case sets).
- **D6-14: Cross-eval = `cmd/eval --judge-models <a,b>` dual-report diff.** `cmd/eval` accepts `--judge-models glm-5.2,deepseek-chat` (a comma list); it runs the eval once per judge model, writing `eval/reports/<judge>/<ts>.json` each. The Dashboard gains a "Cross-eval" tab that diffs the two reports' per-Skill faithfulness + answer_relevance and flags any Skill with a `>5%` faithfulness delta (the Pitfall 8 self-eval-bias signal documented in `eval/README.md`). **Rejected:** dashboard-triggered async cross-eval run (needs service-ification — out of Phase 6 scope); docs-only JUDGE_MODEL manual switching (no dashboard landing point for the conclusion).
- **D6-15: Reproducibility tolerance = layered — StubJudge `1e-6` (strict) + live judge `1e-3` (jitter-tolerant).** The existing `TestRunnerReproducibility` (StubJudge + deterministic embedder + `frozen_5.jsonl`, asserts `abs(run1-run2) < 1e-9`) is RETAINED and strengthened to `1e-6` (the new prod-style tolerance gate). A NEW live reproducibility path is added: `cmd/eval --frozen` with a real judge runs the frozen set twice and asserts `abs(run1-run2) < 1e-3` (tolerating GLM API retry jitter). Both layers gate Phase 6 exit. **Rejected:** StubJudge-only (SC-6's "re-running against a frozen eval set + pinned model_ver + pinned prompt_hash" explicitly wants live-frozen); uniform `1e-9` (unachievable against a live LLM API).
- **D6-16: Trend panel overlays frozen-set baseline as a gray dashed reference line + "diff vs baseline" action.** The Dashboard Trend panel plots live-run history AND overlays the frozen-set baseline (gray dashed line) as a reference. Each future live run offers a "diff vs baseline" button that surfaces per-metric drift. **Rationale:** unifies SC-1 (trend) and SC-6 (reproducibility) on one visual; OSS contributors (Phase 7 DIST) can immediately see whether a change drifted from the published baseline. **Rejected:** trend shows live history only (cross-validation needs manual work); trend fully decoupled from baseline (weakens the reproducibility narrative).

### Claude's Discretion
- **"Citation precision" semantics (SC-1 dashboard metric list):** the SC-1 text lists "faithfulness, citation precision, refusal accuracy" — but `internal/eval` ships `faithfulness`, `context_precision`, `answer_relevance`, `context_recall` (no "citation precision" / no "refusal accuracy"). Researcher resolves: most likely "citation precision" = `context_precision` (Ragas-aligned naming), and "refusal accuracy" = a derived rate from the `refusal-required` golden category (correct-refusals / total-refusal-required-cases). If a genuine new metric is wanted (e.g. verdict-citation vs ground-truth-citation precision using the existing `verdict.Citation{ChunkID,Span,Tier}`), researcher scopes it; the lean is to NOT introduce a new metric in Phase 6 unless the golden set can compute it.
- **SC-4 (50×5 concurrency test) positioning:** Phase 3 shipped `internal/server/chat_session_test.go` (a 2-session isolation test). Researcher/planner decides whether SC-4's 50×5 is (a) a parameterized expansion of the existing test, or (b) a new integration test. The invariant is unchanged (row-level `session_id` partition — D18/SC-9); the work is scaling the test, not new enforcement.
- **Per-Skill golden-set seeding for future Skills:** Phase 6 ships ONE Skill (compliance, 50-case golden set already exists). When Phase 5's wizard generates new Skills, who creates their golden set? OUT of Phase 6 scope (Phase 6 only proves the multi-Skill dashboard architecture with the single compliance Skill). Recommend: the wizard gains an "author golden set" step in a later phase. Noted in `<deferred>`.
- **`/api/eval/*` endpoint design** (serving the dashboard): `/api/eval/reports` (serve `INDEX.json` + per-report fetch), `/api/eval/frozen` (serve frozen-set list + lock metadata), `/api/audit` (D6-05). Exact route shape + handler package placement (`internal/server` extension vs new `internal/evalapi`) → planner decides; lean toward extending `internal/server` (the existing HTTP surface) to avoid a second Hertz server.
- **`cmd/eval` flag taxonomy + the `--archive` / `--frozen` / `--judge-models` interactions:** planner designs the flag matrix; ensure no conflicting-flag combos (e.g. `--frozen` + `--judge-models` is the live-reproducibility path; `--frozen` + StubJudge is the strict gate).
- **Admin token absence behavior:** when `SKILLFORGE_ADMIN_TOKEN` is unset (the OSS default), is `/api/audit` (a) 401 for all, or (b) returns retention-redacted rows with no token needed? Researcher/planner picks — lean toward (b) so the demo works out-of-the-box, with raw access gated by setting the env var.

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` (todo.match-phase score ~0.4-0.6, recurring keyword false-match) — reviewed across Phases 2-5; SYSK-08 (temperature required, `WithTemperature`) was implemented in Phase 2's 7-field validator. For Phase 6 it is only a constraint on any newly generated eval set (frozen sets pin `prompt_hash` which already captures the rendered temperature). Not Phase 6 work.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level (locked decisions that constrain this phase)
- `.planning/PROJECT.md` — Core Value (Skill-driven platform, OSS personal-brand "measurable outcomes") + Key Decisions: **D6 reversed** (Go-native evaluator, no Python sidecar), **D7** (Milvus tier PRE-filter), **D8** (citation = chunk_id + span), **D9** (refusal-by-construction), **D11** (multi-backend abstraction — the pattern Phase 6 reuses for `/api/audit`), **D18** (store.Store interface-first + SQLite default), **D19** (Go-native trust evaluator detail)
- `.planning/REQUIREMENTS.md` — Phase 6 requirements: **GEVL-06** (eval dashboard per-Skill metrics + trend), **GEVL-07** (CI gating faithfulness < 0.9). Traceability table shows Phase 6 = 2 requirements (the SC list carries the other 4 SCs).
- `.planning/ROADMAP.md` — Phase 6 goal + 6 Success Criteria (SC-1..6) + pitfalls addressed (P-RAG-2 monitoring/drift, P-COMPL-4 audit integrity, P-EINO-2 session isolation, P-OSS-5 CI hygiene)
- `.planning/STATE.md` — open question #7 (audit retention + redaction + access control → resolved by D6-06/D6-08); research flag "Phase 6: audit privacy (retention/redaction/access control), reproducibility, CI evaluation cost"; D3-07/D2-15 carry-forward (the eval + calibration vehicles this phase surfaces)

### Eval + store (the Phase-3 substrate Phase 6 builds on — MUST read)
- `eval/README.md` — the Go-native evaluator's operational doc: the `cmd/eval` CLI flow, `report.json` + `calibration.json` schemas, **Pitfall 8 three counter-measures** (Go grounding floor / human spot-check / `JUDGE_MODEL` cross-eval slot), the `0.3`→`recommended_threshold` manual ops procedure (NOT Phase 6 work), and the SC-6 reproducibility gate (`TestRunnerReproducibility`)
- `eval/NOTICE` — Apache-2.0 attribution for the ported Ragas v0.1.21 faithfulness prompt (D6 reversed)
- `internal/eval/` — the library: `metrics.go` (4 metrics), `faithfulness.go` (Ragas-ported prompt), `judge.go` (`NewJudge(cm)`, `JUDGE_MODEL` slot), `dataset.go` (`LoadGolden`), `calibration.go` (`Calibrate()`), `eval_test.go` (`TestRunnerReproducibility` — the SC-6 gate Phase 6 strengthens)
- `cmd/eval/main.go` — the CLI entry: flag set (`--golden`, `--report`, `--calibration`, `--limit`, `--topk`), bootstrap (mirrors `cmd/server` env wiring), per-case flow (embed → search → agent.Run → 4 metrics → backfill `store.UpdateAuditMetrics`). Phase 6 extends this with `--archive` / `--frozen` / `--judge-models`.
- `internal/store/store.go` — `Store` interface + `AuditRow` schema (`Faithfulness*float64` 4 metric columns NULLable + `PromptHash` + `ModelVersion` + `SessionID` + `RefusalReason` + `ChunkIDs`) + `AuditFilter` (the parameterized filter `/api/audit` maps to) + `QueryAudit`/`UpdateAuditMetrics`
- `internal/store/sqlite/` — the pure-Go zero-ops default impl; `RetentionDays` redaction wrapper (D6-06) lives at the store or server layer

### Prior phase context (carry-forward — MUST read)
- `.planning/phases/03-agentic-retrieval-trust-evaluation/03-CONTEXT.md` — **D3-08** (CLI batch + JSON report, NOT always-on HTTP — Phase 6 turns it into the CI gate), **D3-07** (`JUDGE_MODEL` slot for cross-eval — Phase 6 surfaces it as D6-14), **D3-09/D18** (store.Store + SQLite), **D2-15** (calibration vehicle → `eval/calibration.json`), the `<deferred>` list = exactly Phase 6's pickup list (CI gate GEVL-07, dashboard GEVL-06, retention/redaction SC-5, reproducibility, real-user-sampled golden-set)
- `.planning/phases/05-skill-platform-full-knowledge-boundaries/05-CONTEXT.md` — LeftSidebar nav evolution (D6-01 "Eval Dashboard" entry slots here), `GET /api/skills` shape `[]{id,name,version,status,error?,manifest}` (BLOCKING CONSTRAINT — read `item.manifest.*`), store contract `sessions[]`/`skills[]`/`activeSessionId` (D14 — no `messages` field; the dashboard's own state must obey this)
- `.planning/phases/04-streaming-trust-ui/04-CONTEXT.md` — D-04/D12 transport-swap invariant + D15 wire drift alarm: the `/eval` route has its own transport (no SSE), so these invariants are preserved by isolation, not modification

### Existing code (reuse targets — see code_context)
- `internal/server/server.go` — the Hertz HTTP surface; `/api/audit` + `/api/eval/*` endpoints extend this (or a sibling handler file)
- `internal/server/skills.go` — `GET /api/skills` shape contract (read `item.manifest.*`) — the dashboard's Skills table reuses this data source for the Skill list
- `internal/server/chat_session_test.go` — the Phase-3 2-session isolation test; SC-4's 50×5 is a scaled expansion of this
- `web/src/components/{graph,inspector,status}/` — Phase-5 React component patterns (data-driven panels) the Dashboard reuses for layout + theming
- `web/src/store/app-store.ts` — Zustand store (D14 — no `messages` field); the dashboard's own slice (eval state) lives here
- `.github/workflows/` — existing CI (if any); `eval-gate.yml` joins it (researcher confirms current CI layout)

### Research (stack + pitfalls)
- `.planning/research/PITFALLS.md` — P-RAG-2 (monitoring + drift detection), P-COMPL-4 (audit trail integrity), P-EINO-2 (multi-tenant session isolation), P-OSS-5 (CI hygiene)
- `.planning/research/STACK.md` + `.planning/research/ARCHITECTURE.md` — locked stack (Go-native eval, SQLite, React 19 + Vite + Zustand + Vercel AI SDK), 6-layer architecture

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/eval/` library + `cmd/eval` CLI** — the entire Go-native trust evaluator is Phase-3-shipped and tested. Phase 6 EXTENDS it (new flags `--archive`/`--frozen`/`--judge-models`), does not rewrite it. The 4 metrics, `Judge`, `LoadGolden`, `Calibrate`, and `TestRunnerReproducibility` are all reusable as-is.
- **`store.QueryAudit(ctx, AuditFilter)` + `AuditFilter`** — already 100% `?`-parameterized (T-3-SQLI regression-tested), already supports `UserID`/`SkillID`/`RefusalReason`/`SessionID`/`Since`/`Until`/`Limit`. The `/api/audit` endpoint (D6-05) is a 1:1 HTTP mapping. The `RetentionDays` redaction (D6-06) is a thin wrapper applied post-query (or via a `View`-style projection) BEFORE returning rows.
- **`store.UpdateAuditMetrics`** — the eval CLI already backfills the 4 metric columns on matching audit rows; the dashboard reads these via `QueryAudit` for the "audit row with metrics" view.
- **`eval/golden/compliance.jsonl` + `eval/golden/README.md`** — the 50-case synthetic golden set (GEVL-05). The frozen set `eval/frozen/compliance.jsonl` (D6-13) is a pinned copy with a `.lock.json` sidecar — same schema.
- **`internal/eval/testdata/frozen_5.jsonl` + `TestRunnerReproducibility`** — the SC-6 seed. Phase 6 scales `frozen_5` → frozen-N per Skill + adds the live-reproducibility path (D6-15).
- **Phase-5 React substrate (`web/src/components/{graph,inspector,status}`, `app-store.ts`, `LeftSidebar`)** — the Dashboard route + Audit sub-route are new surfaces built on the established component/theming/state patterns. The `GET /api/skills` data source (read `item.manifest.*`) feeds the Skills table.

### Established Patterns
- **D11 / D18 interface-first multi-backend** — the `/api/audit` handler depends on `store.Store` (the interface), not the SQLite impl; PG-swap is unchanged. The Dashboard's data fetchers depend on `internal/eval` outputs (files), not on the CLI process.
- **D6 Go-native (no Python sidecar)** — Phase 6 STAYS single-language Go. Cross-eval (D6-14) swaps `judge_model` within Go, never reintroduces Python.
- **D-04/D12 transport-swap invariant + D15 wire drift alarm** — preserved by ISOLATION: the `/eval` route uses its own (non-SSE) transport, so the chat wire is untouched. No new `SkillForgeChunk` types are added in Phase 6 (the wire budget set aside in Phase 5 for `data-redline` stays unused here).
- **D14 Zustand-bans-`messages`** — the Dashboard's own state slice (eval reports, audit rows, filters) lives in `app-store.ts` WITHOUT a `messages` field; the chat `useChat` stream is on a different route.
- **D2-15 / D3-08 calibration → CI-gate handoff** — Phase 3 emits `eval/report.json` + `eval/calibration.json`; Phase 6 turns `report.json` into the CI gate input (D6-09..12) and `calibration.json` stays a manual-ops concern (documented in `eval/README.md`, NOT automated in Phase 6 — the threshold governs the refusal floor, a human signs off).
- **T-3-AUDIT INSERT-only + T-3-SQLI parameterization** — the audit store's anti-tampering + injection guarantees are preserved; `RetentionDays` redaction is a READ-time projection, not a write (no UPDATE/DELETE path added).

### Integration Points
- **New `/eval` + `/eval/audit` routes (frontend)** — registered in the Vite/React router; `LeftSidebar` gains an "Eval Dashboard" nav entry (D6-01). The three-panel `/` route is untouched.
- **New `/api/audit` + `/api/eval/*` endpoints (backend)** — extend `internal/server`; `/api/audit` maps to `store.QueryAudit` (D6-05); `/api/eval/reports` serves `INDEX.json` + per-report fetch; `/api/eval/frozen` serves frozen-set metadata. `X-Admin-Token` middleware gates raw access (D6-06/D6-08).
- **`cmd/eval` flag extension** — `--archive` (append to `eval/reports/` + `INDEX.json`), `--frozen=<skill>` (read `eval/frozen/<skill>.jsonl` + `.lock.json`, assert pin), `--judge-models <a,b>` (dual-report for cross-eval). Existing flags unchanged.
- **New `.github/workflows/eval-gate.yml`** — `pull_request` trigger, path-filtered; runs `cmd/eval`, parses report, `gh pr comment`, uploads artifacts.
- **`TestRunnerReproducibility` strengthening + new live-repro path** — SC-6 gate (D6-15).

### Creative Options
- The "Cross-eval" tab (D6-14) is the Dashboard's Pitfall-8 landing point — it turns the `JUDGE_MODEL` config slot (documented but invisible) into a visible bias-detection tool. This is a strong OSS-personal-brand narrative beat ("we don't just ship a number, we show you how much it could be lying").
- Trend + frozen baseline overlay (D6-16) unifies SC-1 and SC-6 on one graph — drift from the published baseline is immediately visible, which is the OSS contributor's most-asked question ("did my change make it worse?").
- File-based trend history (D6-02) means `eval/reports/` IS the published eval-results artifact that Phase 7 DIST-09 ("README shows published per-Skill faithfulness %") reads from — Phase 6 sets up Phase 7's data.

</code_context>

<specifics>
## Specific Ideas

- **"Admin mode" is a deliberate UX boundary, not just a route.** Entering `/eval` hides the chat three-panel; the Dashboard is a separate cognitive surface (metrics + audit, not conversation). This keeps the chat product's trust-visualization narrative (Phase 4) uncluttered by operator tooling.
- **Retention redaction is a READ-time projection, never a write.** The audit row stays immutable (T-3-AUDIT); redaction happens between `QueryAudit` and the HTTP response. This preserves "we can prove what happened" (the meta-record survives) while satisfying "we don't keep raw PII past retention" (the content is masked). This dual property is the compliance-audit crux.
- **The CI gate's per-Skill threshold file (`.claude/skills/<skill>/eval-gate.json`) is co-located with the Skill it gates** — same directory as `SKILL.md`. This mirrors the D2-07 file-driven-registration philosophy: a Skill's quality bar travels with the Skill.
- **Cross-eval's `>5%` delta flag (D6-14) is the exact numeric threshold `eval/README.md` already documents** as the Pitfall 8 self-eval-bias signal — Phase 6 operationalizes the doc into a visible alert.
- **SC-6's two-layer tolerance (StubJudge 1e-6 / live 1e-3)** directly addresses the tension between "deterministic reproducibility" (the project's semantic-determinism philosophy) and "real LLM APIs have jitter" — the strict layer proves the pipeline is deterministic; the jitter layer proves it is stable in practice.

</specifics>

<deferred>
## Deferred Ideas

- **Real multi-role RBAC / SSO / per-row `access_policy` column (D6-08 binary split)** — v2 SCALE-02 (K8s/Helm/enterprise SSO). Phase 6 ships the `X-Admin-Token` binary split; a real ACL needs a user table that does not exist yet.
- **DATA-* financial data layer (DATA-01..14)** — still deferred from Phase 5 (user-confirmed out-of-scope). Own phase or v2. ⚠️ Documentation drift note (from 05-CONTEXT.md): REQUIREMENTS.md traceability still maps DATA-* to Phase 5 and the ROADMAP Coverage table shows 55 vs 70 inconsistency — these are docs drift, fix opportunistically but NOT Phase 6 scope.
- **Per-Skill golden-set seeding for wizard-generated Skills** — Phase 6 proves the multi-Skill dashboard architecture with the single compliance Skill. When Phase 5's wizard generates new Skills, an "author golden set" wizard step is the natural home (later phase). Phase 6 only assumes "a Skill has a frozen set if `eval/frozen/<skill>.jsonl` exists".
- **Real-user-sampled golden-set augmentation** — Phase 6 ships synthetic (Phase 3's D3-06 choice); real-user sampling deferred (D3-06 `<deferred>`).
- **`/api/skills/reload` rescan route** — explicitly NOT added (Phase 5 D5-IA-05); not Phase 6 work.
- **PostgreSQL production store backend** — D3-09/D18 leaves the `store.Store` PG-ready slot; SQLite ships now. PG impl is a later production-hardening step (could be a quick-task, not Phase 6 scope).
- **Live SSE push of eval-completion to the Dashboard** — D6-03 chose manual refresh + 30s poll; SSE push deferred (mixed-transport cost not worth it for minute-scale runs).
- **`[skip-eval]` PR label / `workflow_dispatch` manual override** — D6-12 defers to Phase 7 (observe fork-contributor frequency first).
- **Open-source packaging (DIST-*)** — Phase 7.

### Reviewed Todos (not folded)
- `spike-temperature-field-validation.md` — recurring keyword false-match; SYSK-08 implemented in Phase 2. For Phase 6 it is only an implicit constraint on frozen-set pinning (`prompt_hash` captures the rendered temperature). Not folded.

</deferred>

---

*Phase: 6-Eval-Dashboard-Production-Hardening*
*Context gathered: 2026-08-13*
