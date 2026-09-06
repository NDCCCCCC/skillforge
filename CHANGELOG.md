# Changelog

All notable changes to SkillForge will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.1.0] — 2026-09-06

v1.1 is a **trust-architecture maturation** release. No breaking changes —
all additions are backward-compatible wire shapes (additive fields,
omitempty) and additive store tables (DDL migrations auto-apply).

### Added — Trust Contract Hardening (Phase 24, audit chain)

- **PROV-O export endpoint** `GET /api/session/:id/audit/prov` — INSERT-only
  audit rows are now mapped to W3C PROV-JSON (Entity / Activity / Agent
  + wasGeneratedBy / used / wasDerivedFrom). Zero LLM, deterministic,
  passes through the existing retention redaction. Read by Semantica
  users via standard PROV tooling.
- **Decision-chain endpoint** `GET /api/session/:id/decision-chain?limit&at` —
  every audit row for the session, classified into a chronological
  Link list (verdict / pending_approval / approved / rejected /
  rule_created / rule_revoked). FE AuditTimelineTab renders this as a
  vertical timeline.
- **Time-travel state endpoint** `GET /api/session/:id/state?at=RFC3339` —
  reconstructs the per-session T-point snapshot (visible turns +
  effective approval-rules). Revoked rules are rebuilt from the
  `action_rule_revoked` row's `frozen_args` column.
- **New package `internal/auditchain`** — pure Go reconstruction
  helpers (6 unit tests covering verdict / approval / create / revoke
  / skill-scope / session-scope / empty-session / ordering invariant).
- **New package `internal/auditprov`** — PROV-JSON encoder.

### Added — External Authorized Channel (Phase 21)

New `internal/externalauth` package + 4 HTTP endpoints — implements
**DATA-04/05/06/10/11/13/14** from `REQUIREMENTS.md`:

| Need | Implementation |
|---|---|
| DATA-04 TTL token | `externalauth.TTLToken` + `Manager.ExecuteWithToken` + 5-min default lifetime + `token.Valid()` guard |
| DATA-05 二级脱敏 | `MaskSpec` (account last-N / phone / email / generic) + `View.Mask` + `ApplyMask` |
| DATA-06 双重审计 | `Source.AppendExternalAudit` + `Manager.Reconcile` + `query_id` pair matching |
| DATA-10 预编译视图 | `ViewRegistry` whitelist + `ViewNamePattern` + `ErrViewNotAllowed` |
| DATA-11 NL2SQL 预览 | `internal/nl2sql.Generate` + `RejectNonSelect` AST gate (rejects INSERT/UPDATE/DELETE/DDL/CALL) |
| DATA-13 持续挂起 | `pending` state is permanent without explicit Approve / Reject; **no auto-reject, no auto-timeout** |
| DATA-14 audit row schema | `external_auth_audit` 9-field schema with `MaskApplied` column |

Endpoints:
- `POST /api/external-auth/request` — create approval
- `GET /api/external-auth/pending` — data-officer dashboard
- `POST /api/external-auth/approve` / `/reject` — explicit human resolution
- `POST /api/external-auth/execute` — agent's post-approval query
- `GET /api/external-auth/audit/reconcile` — dual-audit pair mismatch

7-step end-to-end release-gate test (`TestUAT_FullChannelFlow`)
covers all 7 DATA invariants. UAT doc:
`.planning/uat/phase21-external-auth-2026-09-06.md`.

### Added — CI-Integrated Trust Gate (Phase 22)

- `.github/workflows/eval-gate.yml` — every PR + main push runs
  `cmd/eval --frozen compliance --gate-check` (50-case frozen replay
  against `examples/skills/<skill>/eval-gate.json` thresholds). Fail-closed
  exit code; no auto-merge.
- `eval/frozen/compliance.jsonl` — 50 cases now carry `source` annotation
  (spike-anchor / closed-loop-2026-09-06 / negative-test) and
  `corpus_file` attribution back to `knowledge/authority/*.md`. The
  3 spike-anchor cases (adv-001/002/003) had GBK mojibake from a
  historical terminal encoding bug; rewritten as UTF-8 verbatim copies.
- **README §3 "评测定位声明"** — SkillForge does NOT ship an independent
  evaluation platform. `internal/eval` is internal infrastructure that
  backs the Skill promotion gate. If you want a standalone RAG
  evaluation suite, use [Ragas](https://docs.ragas.io/) directly.

### Added — Eval Trust-Loop Closure (Phase 25)

- 13 ground-truth entries added to `knowledge/authority/` (4 rewrites +
  9 new topical docs: annual_leave carryover / overtime_pay / sick_leave /
  maternity_leave / probation / supplier_management / travel_meals /
  travel_flight_class / travel_personal_delay / expense_documents).
  All 30 should-answer questions now ≥ 50% shingle coverage.
- `cmd/eval`:
  - `--cases <id,id>` — diagnostic subset mode
  - `--dump <path>` — per-case raw verdict dump (query / answer /
    refusal / chunk_scores) for offline harness-vs-model dispute triage
- `CaseResult.RefusalReason` additive + `Report.AgentErrors` counter —
  harness faults (agent_error) are excluded from means and counted
  separately. No more 2/50 infrastructure fault masquerading as
  refusal collapse.

### Added — Local Model Baseline

- New env triple `LOCAL_BASE_URL` + `LOCAL_API_KEY` + `LOCAL_MODEL` —
  switch the chat model + judge to a self-hosted OpenAI-compatible
  endpoint. Precedence `LOCAL_*` > `GLM_*`; if neither is set, the
  default Coding Plan endpoint is used.
- `internal/llm.ExtraFieldsModel` decorator — `LOCAL_LLM_THINK=false`
  injects ollama's `think:false` body field on every Generate /
  Stream call. Reaches the ReAct agent loop (flowagent has no chat-
  model-option passthrough in eino v0.9.14).
- Budget tunables: `HISTORY_LAYER_TIMEOUT` (rewrite + classifier
  per-call), `VERDICT_STREAM_TIMEOUT` (verdict stream), `EVAL_CASE_TIMEOUT`
  (cmd/eval per case). Cloud-tuned defaults preserved.

### Changed — README 差异化叙事重排 (Phase 23)

- README §2 "Why SkillForge — The Three Moats" rewritten from
  tier-filter / Skill-as-Platform / Go-eval to **approve-and-remember
  primitive / trust-contract kernel / CI-Integrated Trust Gate**.
- New §"SkillForge vs WeKnora vs Semantica — three lanes" comparison
  table positioning SkillForge as the execution-time trust boundary
  alongside (not against) the other two.
- New opening tagline + Skill de-confusion note — distinguishes
  SkillForge's `SKILL.md` (domain adaptation contract) from WeKnora's
  Skill (sandboxed skill plugin) and Anthropic Agent Skills (MCP tool).
- "Go 企业 RAG 平台" / "4 信任指标" / "审计链" → demoted to **support
  capabilities**, not headline differentiation.

### Migration notes

No data migration required. New env vars (all optional, defaults
preserve v1.0 behavior):
```
LOCAL_BASE_URL=http://10.x.x.x:8888/v1   # optional
LOCAL_API_KEY=...                         # optional
LOCAL_MODEL=Qwen3.8-27B                    # optional
LOCAL_LLM_THINK=false                      # optional (ollama dialect)
HISTORY_LAYER_TIMEOUT=2s                   # optional
VERDICT_STREAM_TIMEOUT=60s                 # optional
EVAL_CASE_TIMEOUT=90s                     # optional
EXTERNAL_AUTH_ENABLED=1                    # optional (default: disabled)
```

Schema migrations are auto-applied at server start (additive ALTER
TABLE only).

### Dependencies

- `cloudwego/eino@v0.9.14` (locked 2026-08-17) — unchanged
- `cloudwego/eino-ext` (latest) — unchanged
- Go 1.25 — unchanged

---

## [v1.0.0] — 2026-08-30

Initial public release. v1.0 ships:

- 4-tier knowledge isolation (权威 / 历史 / 个人 / 草稿)
- Skill-as-platform (SKILL.md hot-reload, intent routing)
- 3 example Skills (simple-qa, calculator, compliance)
- 4 RAG-trust metrics (Ragas v0.1.21 verbatim port)
- HITL approve-and-remember (Phase 11)
- Admin console (Phase 20)
- KPI pipeline (Phase 17)
- Static report portal (Phase 19)
- 439 vitest + Go test suites green at v1.0 UAT (`.planning/uat/v1.0-uat-2026-09-06.md`)
