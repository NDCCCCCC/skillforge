# SkillForge v1.1.0 — Release Notes (2026-09-06)

> **The lane**: execution-time trust boundaries for AI agents —
> 什么不许答, 什么必须批. We are complementary to WeKnora (查得准)
> and Semantica (为什么), not head-to-head with either.

## Three things you get in v1.1 that v1.0 didn't have

1. **External Authorized Channel** (Phase 21 / DATA-04..14) — when the
   agent needs to query a customer data warehouse, every query is
   gated by data-officer approval with the NL2SQL preview shown
   verbatim. TTL token scope-bound to (view, user). Field-level PII
   masking on every returned row. Dual audit (internal + warehouse
   query_log) reconciled by query_id.

2. **CI-Integrated Trust Gate** (Phase 22) — `.github/workflows/eval-gate.yml`
   blocks any PR that drops a Skill below faithfulness 0.9 / refusal
   accuracy 0.95. Runs the 50-case frozen compliance golden set on
   every push.

3. **Audit chain hardening** (Phase 24) — every audit session
   exports as W3C PROV-O (`GET /api/session/:id/audit/prov`); every
   audit row shows up in the FE decision-chain view
   (`GET /api/session/:id/decision-chain`); every state can be
   reconstructed for any past timestamp (`GET /api/session/:id/state?at=`)
   including revoked rules.

Plus the Phase 25 eval trust-loop closure (golden set ↔ corpus sync,
agent_error separated from refused) that made #2's headline numbers
trustworthy.

## What changed in the codebase

| Change | Type | Notes |
|---|---|---|
| `internal/auditchain` (new) | package | pure-Go decision chain + state reconstruction |
| `internal/auditprov` (new) | package | PROV-O JSON encoder (zero LLM) |
| `internal/externalauth` (new) | package | channel + manager + Source interface + MemorySource |
| `internal/nl2sql` (new) | package | schema-aware preview generator |
| `internal/llm` (decorator) | additive | `ExtraFieldsModel` for `LOCAL_LLM_THINK=false` |
| `cmd/eval --cases` / `--dump` | additive flags | harness-vs-model triage |
| `CaseResult.RefusalReason` | additive | excludes agent_error from means |
| `Report.AgentErrors` | additive | harness fault counter |
| `frozen compliance.jsonl` | source + corpus_file attribution | 50 cases traceable |
| `knowledge/authority/` (4 rewrites + 9 new) | content | 17 MISS-ing ground truths filled |
| `.github/workflows/eval-gate.yml` | new | CI gate |
| 6 `/api/external-auth/*` endpoints | new | Phase 21 channel surface |
| 3 audit-related endpoints (`/audit/prov`, `/decision-chain`, `/state`) | new | Phase 24 surface |
| `README.md` | rewritten §2 + new comparison table | Phase 23 |
| `CHANGELOG.md` | new | this release |

## Migration notes

**Breaking changes**: NONE.

All wire shapes are additive + `omitempty`; the SQLite store
auto-applies additive `ALTER TABLE` migrations on startup. v1.0
deployments upgrade in place by replacing the binary + restarting;
no schema dump / restore required.

**New optional env vars** (defaults preserve v1.0 behavior):
```
# External Authorized Channel (default OFF)
EXTERNAL_AUTH_ENABLED=1

# Self-hosted local model (default OFF)
LOCAL_BASE_URL=http://10.x.x.x:8888/v1
LOCAL_API_KEY=...
LOCAL_MODEL=Qwen3.8-27B

# Latency budgets (cloud-tuned defaults preserved)
HISTORY_LAYER_TIMEOUT=2s
VERDICT_STREAM_TIMEOUT=60s
EVAL_CASE_TIMEOUT=90s
```

**Operator runbook update**: see
[`C:\Users\CPIC\.claude\projects\D--code-ClaudeCode-ai-agent\memory\env-wsl-backend-runbook.md`](C:\Users\CPIC\.claude\projects\D--code-ClaudeCode-ai-agent\memory\env-wsl-backend-runbook.md)
for the verified startup command set (Windows + WSL both covered).

## What we **don't** ship

- A **standalone evaluation platform** — use [Ragas](https://docs.ragas.io/)
  / [DeepEval](https://deepeval.com/) / [TruLens](https://www.trulens.org/)
  directly. `internal/eval` is internal infrastructure that backs the
  Skill promotion gate, nothing more.
- A **decision-trace platform** — use [Semantica](https://github.com/semantica-agi/semantica).
  We export PROV-O so you can pipe our audit to their tooling, but
  causal-graph + time-travel are not our lane.
- A **retrieval-quality benchmark** — use [WeKnora](https://github.com/Tencent/WeKnora)
  if recall is your bottleneck. Their hybrid retrieval + multi-tenant
  RBAC is the production-tested RAG stack.

## Quick taste

```bash
# Install + run (Docker Compose)
git clone https://github.com/<owner>/skillforge && cd skillforge
docker compose up -d
# Browse http://localhost:8080

# Or run the v1.1 frozen replay + gate locally
go test ./internal/externalauth/    # 11 + 1 UAT test
go run ./cmd/eval \
    --frozen compliance \
    --skills-dir examples/skills \
    --report eval/reports/ci/compliance.json
go run ./cmd/eval --gate-check eval/reports/ci/compliance.json --skills-dir examples/skills
```

## Trust posture — be honest

| Metric | v1.0 | v1.1 (local model baseline, 27B Q4) | v1.1 (pinned GLM-5.2) |
|---|---|---|---|
| Adversarial refusal rate | 100% (12/12 in v1.0 UAT) | 100% (20/20 in Phase 25 closed-loop) | (PENDING GLM quota — see Phase 14-10) |
| Faithfulness ≥ 0.9 | ✅ v1.0 | ✅ Phase 25 closed-loop on local | ✅ v1.0 baseline |
| Eval CI gate | ❌ manual | ✅ `.github/workflows/eval-gate.yml` | ✅ same |
| Audit chain PROV-O export | ❌ | ✅ Phase 24-01 | ✅ same |
| Approval gate primitive | ✅ Phase 11 | ✅ Phase 11 (unchanged) | ✅ same |
| External auth channel | ❌ | ✅ Phase 21 | ✅ same |

Local-model baselines are tracked in
[`.planning/uat/local-model-baselines-2026-09-06.md`](.planning/uat/local-model-baselines-2026-09-06.md).

## License

Apache-2.0 — see [`LICENSE`](LICENSE).
