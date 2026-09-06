# Milestone v1.1 — Project Summary

**Generated:** 2026-09-06
**Milestone:** v1.1 — 信任架构成熟 + OSS 收官 (Trust Architecture Maturity + OSS Close-out)
**Status:** SHIPPED 2026-09-06 (5/5 phases closed; no `v1.1` git tag yet — tagging is user-driven)
**Purpose:** Team onboarding and project review

> **Artifact caveat — read this first.** v1.1 was executed *outside* the normal GSD
> per-phase artifact flow. Phases 21/22/23/24 have only `CONTEXT.md` +
> `PLAN-OUTLINE.md`; **no `*-SUMMARY.md`, no `*-VERIFICATION.md`, no `*-PLAN.md`**
> exist, and **Phase 25 has no directory at all**. This summary is therefore
> reconstructed from: ROADMAP.md, the four closure reports in `.planning/uat/`,
> the phase CONTEXT files, git history, and **direct verification against the
> code on disk**. Where the planning record and the code disagree, the code wins
> and the discrepancy is flagged in §6.

---

## 1. Project Overview

**SkillForge** is an enterprise-facing, interactive web application that
demonstrates LLM agent engineering. Backend: **Go + Eino (CloudWeGo) + Eino ADK +
Hertz**. Frontend: **React 19 + Vite + TypeScript + Vercel AI SDK**.

**Core value:** *One agent kernel, retargeted to any enterprise domain by editing
a pluggable Skill file (`SKILL.md`) — no code changes.* This is the moat that
turns an "application" into a "platform", and the explicit defence against the
dead "AI wrapper" category (Height.app closed 2025.9).

**Where v1.1 sits in the arc:**

| Milestone | Theme | Phases | Shipped |
|-----------|-------|--------|---------|
| v0.7.7 | 可信问答内核 (trustworthy QA kernel) | 1–7 | 2026-08-13 |
| v0.8 | Harness 可信×行动融合 (trust × action) | 8–13 | 2026-08-21 |
| v1.0 | 业务数据支撑 (business data support) | 14–20 | 2026-08-30 |
| **v1.1** | **信任架构成熟 + OSS 收官** | **21–25** | **2026-09-06** |

v1.1 is a **maturity-and-positioning milestone**, not a feature-expansion one. It
was reshaped mid-flight by two rounds of competitive verification (2026-09-05 and
2026-09-06) against **WeKnora** (Tencent, Go enterprise RAG, MIT) and
**Semantica** (W3C PROV-O decision provenance, MIT, GitHub Trending #1 in
2026-08). That analysis forced an honest re-read of what is actually
differentiated, and produced two brand-new phases (24 and 25) plus a full README
narrative reset.

The resulting three-lane positioning — now the project's official story:

- **WeKnora** answers *"how do I retrieve accurately?"* (RAG + Wiki + multi-tenant RBAC)
- **Semantica** answers *"why was this decided?"* (PROV-O provenance + causal graphs)
- **SkillForge** answers *"what may not be answered, and what must be approved?"*
  (runtime trust boundaries: tier isolation, refusal-by-construction, approval
  gates, eval gating)

---

## 2. Architecture & Technical Decisions

### What v1.1 added to the architecture

| Component | Purpose | Phase |
|-----------|---------|-------|
| `internal/auditprov` | Maps INSERT-only audit rows → **W3C PROV-JSON** (zero LLM, fully deterministic) | 24 |
| `internal/server/audit_prov.go` | `GET /api/session/:id/audit/prov` | 24 |
| `internal/server/audit_chain.go` | `GET /api/session/:id/decision-chain` + `GET /api/session/:id/state?at=` (time-travel) | 24 |
| `web/src/components/AuditTimelineTab.tsx` | FE chain view over the decision chain | 24 |
| `internal/externalauth` | External authorized channel: TTL tokens, field masking, precompiled-view registry, dual audit + reconcile | 21 |
| `internal/nl2sql` | NL→SQL **preview** generation behind a SELECT-only AST gate | 21 |
| `internal/server/external_auth.go` | 4 endpoints: request / pending / approve+reject / reconcile | 21 |
| `.github/workflows/eval-gate.yml` | CI-enforced frozen-replay trust gate | 22 |
| `CHANGELOG.md`, `RELEASE-NOTES.md` | New OSS release artifacts | 23 |

### Key technical decisions and rationale

- **Decision: The audit path stays 100% deterministic — zero LLM.**
  - **Why:** Same input ⇒ byte-identical output, so exports are hash-reproducible.
    This is deliberately the same philosophy as Semantica; an LLM anywhere in the
    provenance path would make the audit chain unfalsifiable.
  - **Phase:** 24 (locked as discipline #3 in `24-CONTEXT.md`)

- **Decision: Adopt W3C PROV-JSON rather than defend the bespoke audit table.**
  - **Why:** Competitive verification showed a standards-based export is the
    industry norm, and SkillForge's self-rolled INSERT-only table had *no*
    standard export. Rather than argue, v1.1 added the export and **demoted the
    audit chain from headline differentiator to supporting capability**.
  - **Phase:** 24 → consumed by 23's README rewrite

- **Decision: Additive-migration-only, INSERT-only invariant never relaxed.**
  - **Why:** Every new v1.1 audit capability (PROV export, decision chain,
    time-travel) is **read-only over existing rows**. Revoked approval rules are
    reconstructed from `action_rule_revoked.frozen_args` — which is the ultimate
    validation of the original T-3-AUDIT "the audit row *is* the evidence" design.
    Index creation stays a separate const after the ALTER loop (D14-E1).
  - **Phase:** 24

- **Decision: Raw enterprise data never enters SkillForge; the agent gets masked rows from precompiled views.**
  - **Why:** ~80% of real customer individual-identification queries need the
    enterprise warehouse, where PII truth lives. Handing over a DB connection
    violates least privilege. Instead: read-only DB user + TTL token
    (5 min, scope-bound to `(view_name, user_id)`) + precompiled views (never raw
    tables) + field-level masking + explicit human approval.
  - **Phase:** 21

- **Decision: Approval is *continuously pending* — there is no auto-reject and no auto-timeout path.**
  - **Why:** This is the milestone's sharpest trust stance. Treating "the officer
    forgot" as either an implicit deny *or* an implicit allow is a lie about human
    intent. Only an explicit human act resolves state. Locked by test, not convention.
  - **Phase:** 21 (DATA-13)

- **Decision: NL2SQL is allowed *only* as a preview, behind a SELECT-only AST gate.**
  - **Why:** DATA-09 previously banned NL2SQL in every form ("one misjudgement =
    one leak"). v1.1 admits it *only* wrapped in the full DATA-10..14 protection
    stack, and the generated SQL is an artifact for a human to audit before
    execution — never a direct query path.
  - **Phase:** 21

- **Decision: The CI gate must not soften any threshold.**
  - **Why:** `cmd/eval --gate-check` is the single ground truth; CI is a thin
    wrapper. Threshold drift *is* trust-contract breakage. Demonstrated in
    practice: the Wave 5 dry run **failed at 0.8996 vs a 0.9000 threshold** and
    was recorded as a correct fail-closed result rather than rounded up.
  - **Phase:** 22

- **Decision: Publicly declare the evaluator is *not* a differentiator.**
  - **Why:** The 4 metrics are a verbatim Ragas v0.1.21 port and are functionally
    identical to Ragas/DeepEval/TruLens. README now states this as fact and even
    says "if you want a standalone eval platform, use Ragas." Honest positioning
    beats an inflated claim a reviewer can trivially puncture.
  - **Phase:** 22 → 23

- **Decision: De-confuse the word "Skill" on the first screen of the README.**
  - **Why:** WeKnora's "Skill" = a sandboxed plugin capability (Anthropic Agent
    Skills lineage). SkillForge's Skill = a **domain adaptation contract**
    (`reads` knowledge boundary + `writes` targets + refusal guardrails + prompt
    body). Same word, different meaning; a reader must not discover this halfway
    down.
  - **Phase:** 23

---

## 3. Phases Delivered

Executed in dependency order **25 → 24 → 22 → 21 → 23** (not numeric order —
Phase 25 supplies the golden-set data that Phase 22's CI gate depends on, and
Phase 24's PROV export is what lets Phase 23's README flip a weakness into a
feature).

| Phase | Name | Status | One-liner |
|-------|------|--------|-----------|
| 25 | 评测信任环闭环 (Eval Trust-Loop Closure) | ✅ Complete (4/4) | Re-synced the golden set with the authority corpus and split `agent_error` out of refusal metrics — turning a misread "model failure" into measurable truth |
| 24 | 审计链强化 (Audit Chain Hardening) | ✅ Complete (4/4) | PROV-JSON export + decision-causal-chain query + point-in-time state rebuild, all deterministic and read-only |
| 22 | Eval CI Wiring + Live UAT | ✅ Complete (4/4) | Bound the frozen compliance replay to GitHub Actions as a hard gate, and published an honest evaluator-positioning statement |
| 21 | External Authorized Channel | ⚠️ Complete-with-gap (6/6 planned) | Landed the full DATA-04..14 trust stack as a tested library + HTTP surface — **but it is not wired into the running server** (see §6) |
| 23 | Phase 7 completion (Open-Source Release) | ✅ Complete (4/4) | README narrative reset to the three-lane story + CHANGELOG + RELEASE-NOTES + CONTRIBUTING positioning guide |

### Phase 25 — Eval Trust-Loop Closure

The most intellectually valuable phase of the milestone: it discovered that a
local-model baseline had been **misread**, and fixed the measurement instrument
rather than the score.

- **Root finding:** 17 of 20 `adversarial` cases had `ground_truth` with **no
  corresponding clause in `knowledge/authority/`** (4-gram shingle coverage
  0–44%). Those "refusals" were *correct fail-closed behaviour* being scored as
  model failure.
- **Fixes:** 13 GT items backfilled into the authority corpus (4 rewrites + 9 new
  docs) so every answerable case reaches ≥50% coverage;
  `CaseResult.RefusalReason` added; `Report.AgentErrors` counted separately so
  `exceeds max steps` / LLM 5xx no longer pollute refusal metrics.
- **Closed-loop result (Qwen3.8-27B, self-eval):** faithfulness 0.900,
  context_recall 0.823, context_precision 0.600, answer_relevance 0.522;
  **refusal-required 20/20**; 0 `agent_error`.
- **Corrected conclusion:** the real weakness is a **48% fabrication rate when
  the corpus has no basis**, not a refusal failure. The 1-bit UD-IQ1_M
  comparison (0/20 refusal) confirmed 1-bit quantization is unusable.
- **Commits:** `9b08f26`, `b5e4e72`, `23f1483`, `77071fe`

### Phase 24 — Audit Chain Hardening

- `internal/auditprov` maps audit rows into PROV-JSON with the `sf:` namespace:
  turns → `prov:Activity`, query/verdict/chunk → `prov:Entity`,
  user/skill/model/floor → `prov:Agent`, related by
  `wasGeneratedBy` / `used` / `wasDerivedFrom` / `wasAttributedTo`.
- Determinism guaranteed via `encoding/json` key ordering; redaction sentinels
  (`[REDACTED:...]`) pass through verbatim and are never decoded.
- Session isolation enforced (only the session's own rows; nil store → 503;
  empty session → empty document, not 404).
- **Verified on disk:** `internal/auditprov/{prov.go,prov_test.go}`,
  `internal/server/{audit_prov.go,audit_prov_test.go,audit_chain.go}`,
  `web/src/components/AuditTimelineTab.tsx`.
- **Commits:** `30c466b` (24-01), `7b27b31` (24-02 + 24-03)

### Phase 22 — Eval CI Wiring

- All 50 frozen compliance cases annotated with `source`
  (`spike-anchor` 5 / `closed-loop-2026-09-06` 26 / `negative-test-2026-09-06` 19);
  30 adv/mh cases carry `corpus_file` traceability.
- Fixed historic **GBK mojibake** in the three spike-anchor ground truths.
- `.github/workflows/eval-gate.yml`: PR + main-push + manual dispatch, 6 steps,
  90-minute timeout, 14-day artifact retention, **no auto-merge**.
- README §3 rewritten from "Go-Native Trust Evaluator" to "CI-Integrated Trust Gate".
- **Commit:** `f31f8e0`

### Phase 21 — External Authorized Channel

- Delivered: `internal/externalauth/{types,source,manager}.go` +
  `internal/nl2sql/generate.go` + `internal/server/external_auth.go`.
- 12 package tests including `TestUAT_FullChannelFlow`, a 7-step end-to-end gate
  covering all 7 DATA requirements (request→pending, pending-invariance,
  approve→token, execute→masked rows, reconcile, reject path, non-SELECT reject).
- **Gap:** the manager is never constructed in `cmd/server/main.go`, and the
  planned `needs_external_query` agent tool does not exist. See §6.
- **Commit:** `a955450`

### Phase 23 — Open-Source Release

- README §2 rewritten to the three moats (**approve-and-remember primitive /
  trust-contract kernel / CI-integrated trust gate**), plus a
  SkillForge-vs-WeKnora-vs-Semantica three-lane table and the Skill
  de-confusion section.
- New `CHANGELOG.md` (v1.1.0) and `RELEASE-NOTES.md`; `CONTRIBUTING.md` gained a
  "how to position SkillForge in PRs, docs, and talks" guide with ✅/⚠️/❌
  phrasing rules.
- **Commit:** `56703bf`

---

## 4. Requirements Coverage

v1.1's only formally numbered requirements are the **7 DATA items** deferred from
v1.0. Phases 22/23/24/25 were scope added mid-milestone from competitive analysis
and carry **no requirement IDs** — they are tracked only in ROADMAP.md.

| Requirement | Implementation | Test | Status |
|-------------|----------------|------|--------|
| **DATA-04** external channel, read-only DB user + TTL token | `TTLToken` + `Manager.ExecuteWithToken`, 5-min default | `TestManager_TokenExpiryEnforced` | ⚠️ Implemented, not reachable |
| **DATA-05** field-level second-layer masking + pre-authorization | `MaskSpec` + `ApplyMask` + `View.Mask` (account/phone/email/generic) | 4 mask tests | ⚠️ Implemented, not reachable |
| **DATA-06** dual audit, reconcilable by query id + timestamp | `Source.AppendExternalAudit` + `Manager.Reconcile` | `TestManager_Reconcile_PairMismatch` | ⚠️ Implemented, not reachable |
| **DATA-10** individual channel via precompiled views | `ViewRegistry` + `ErrViewNotAllowed` | `TestManager_UnregisteredViewRejected` | ⚠️ Implemented, not reachable |
| **DATA-11** SQL preview before approval flow | `nl2sql.Generate` + `RejectNonSelect` AST gate | `TestRejectNonSelect`, `TestManager_NonSelectRejected` | ⚠️ Implemented, not reachable |
| **DATA-13** approval continuously pending (no auto-reject/timeout) | `StatePending` immutable; explicit resolve only | `TestUAT_FullChannelFlow` step 2 | ⚠️ Implemented, not reachable |
| **DATA-14** approval audit-row schema (9 fields) | `AuditRow` + `MaskApplied` | `TestUAT_FullChannelFlow` step 5 | ⚠️ Implemented, not reachable |
| **DATA-07** `reads: data:external:authorized` mode | ❌ **Not implemented** — `internal/skill/validate.go:104` still fail-closes this value as "unknown data channel mode" | — | ❌ Open |

**"Implemented, not reachable"** means: the logic exists and is covered by
passing package tests, but no live request can reach it (§6, Gap 1). All seven
are library-correct; none is deployment-live.

**Bookkeeping drift:** `.planning/REQUIREMENTS.md` still shows DATA-04/05/06/10/
11/13/14 as unchecked `[ ]` with "Deferred v1.1" in the status table (lines
97–107, 340–350), despite Phase 21 having landed them. No milestone audit
(`v1.1-MILESTONE-AUDIT.md`) was produced.

---

## 5. Key Decisions Log

| ID | Decision | Phase | Rationale |
|----|----------|-------|-----------|
| v1.1-D1 | Audit path is zero-LLM and deterministic | 24 | Hash-reproducible provenance; an LLM would make the chain unfalsifiable |
| v1.1-D2 | Adopt W3C PROV-JSON; demote audit chain to supporting capability | 24 → 23 | Standards beat bespoke; concede the point and ship the export instead of arguing |
| v1.1-D3 | Additive migration only; INSERT-only invariant never relaxed | 24 | All new reads are read-only over existing rows; D14-E1 index discipline holds |
| v1.1-D4 | Revoked rules reconstructed from `action_rule_revoked.frozen_args` | 24 | Ultimate proof of the T-3-AUDIT "audit row is the evidence" design |
| v1.1-D5 | Raw warehouse data never enters SkillForge; masked rows from precompiled views only | 21 | Least privilege — the agent must never hold a schema-wide connection |
| v1.1-D6 | TTL token 5 min, scope-bound to `(view_name, user_id)` | 21 | Short-lived + narrowly scoped limits blast radius of a leaked token |
| v1.1-D7 | Approval is continuously pending — no auto-reject, no auto-timeout | 21 | Neither implicit deny nor implicit allow may impersonate a human decision |
| v1.1-D8 | NL2SQL permitted only as a human-audited preview behind a SELECT-only AST gate | 21 | Relaxes the DATA-09 ban only inside the full DATA-10..14 protection stack |
| v1.1-D9 | Dual-audit mismatch = security event (fail-closed) | 21 | A missing row on either side must never be silently tolerated |
| v1.1-D10 | Real warehouse drivers deferred behind a `Source` interface | 21 | v1.1 proves the trust surface with `MemorySource`; v1.2 swaps drivers with zero caller change |
| v1.1-D11 | CI wraps `--gate-check`; thresholds must never be softened | 22 | Threshold drift is trust-contract breakage; the 0.8996 fail was recorded, not rounded |
| v1.1-D12 | No new eval metrics without a new milestone | 22 | Prevents the "SkillForge has a homegrown eval platform" misread |
| v1.1-D13 | No nightly auto-replay; no auto-merge | 22 | GLM quota sensitivity + a human reviewer must read CI state |
| v1.1-D14 | Publicly declare the evaluator commodity, not differentiator | 22 → 23 | The 4 metrics are a verbatim Ragas port; honesty is more defensible than inflation |
| v1.1-D15 | Fix the measurement instrument, not the score | 25 | Golden-set/corpus desync was mis-scoring correct fail-closed refusals as failures |
| v1.1-D16 | `agent_error` counted separately from `refused` | 25 | Runtime errors must not inflate or deflate refusal accuracy |
| v1.1-D17 | Skill de-confusion required on README first screen | 23 | WeKnora's Skill ≠ SkillForge's Skill; the collision must be resolved immediately |
| v1.1-D18 | Every README claim carries a competitor contrast | 23 | Structural guard against relapsing into self-congratulatory "Go RAG platform" narrative |

---

## 6. Tech Debt & Deferred Items

### Gap 1 — Phase 21's external channel is not reachable in a running server ⚠️ **HIGH**

Verified directly against the code:

- `grep externalauth cmd/` → **no matches.** `Deps.ExternalAuthManager`
  (`internal/server/server.go:353`) is never assigned in `cmd/server/main.go`, so
  it stays `nil` and all four `/api/external-auth/*` endpoints return **503** in
  any real deployment.
- `grep needs_external_query internal/` → **no matches.** The agent-side tool
  specified in `21-CONTEXT.md` and Wave 3 of `21-PLAN-OUTLINE.md` was never built,
  so no agent turn can initiate the flow.
- `internal/skill/validate.go:104` still **rejects** `reads:
  ["data:external:authorized"]` as an unknown channel mode — so no Skill can
  declare use of the channel either (DATA-07's second half).

**Interpretation:** Phase 21 is a well-tested, well-designed *library plus HTTP
handlers*, not a live feature. Its own closure report is honest (it claims Go
package tests and explicitly defers the FE panel), but the milestone-level claim
"DATA-04/05/06/10/11/13/14 channel landed" overstates reachability. Wiring is
believed small (construct the manager in `main.go`, register the agent tool,
extend the `reads` allowlist) but it is **not done**.

### Gap 2 — Planned Phase 21 artifacts never produced

| Planned in | Item | On disk |
|---|---|---|
| `21-PLAN-OUTLINE.md` Wave 4 | `web/src/components/ExternalChannelPanel.tsx` | ❌ absent (deferred explicitly in the UAT report — acknowledged) |
| `21-PLAN-OUTLINE.md` Wave 5 | `eval/frozen/external-auth.jsonl` (10-case golden set) | ❌ absent (**not** acknowledged anywhere) |
| `21-CONTEXT.md` release gate | README section "External Authorized Channel" | ❌ absent (`grep -i external README.md` finds only infra/BGE/GLM mentions) |

### Gap 3 — README internal narrative inconsistency

`README.md:85` (Architecture) still describes the **old v1.0** three moats —
"真源分离 tier-filter, Skill-as-platform, and the Go-native trust evaluator" —
while the new §"Why SkillForge? — The Three Moats" (line 115) presents the
v1.1 set (approve-and-remember / trust-contract kernel / CI-integrated trust
gate). Phase 23 rewrote §2 but missed the architecture paragraph. A first-time
reader meets two different "three moats" in one document.

### Gap 4 — GSD artifact flow was bypassed for the whole milestone

No `*-SUMMARY.md`, `*-VERIFICATION.md`, or `*-PLAN.md` for phases 21–24; no
Phase 25 directory; no `v1.1-MILESTONE-AUDIT.md`; v1.1 not archived to
`.planning/milestones/`. The `.planning/uat/*.md` closure reports are good and
were the only reason this summary could be reconstructed — but no independent
verifier ever checked the phases, which is plausibly *why* Gap 1 went unnoticed.

### Deferred by explicit decision

| Item | Owner note |
|------|-----------|
| Judge human-calibration set (Phase 22-02) | Contract fully specified in `22-JUDGE-CALIBRATION.md` (agreement ≥0.85, miss rate <0.10, adversarial ≥0.70); blocked only on the user labelling 30–50 cases |
| Real warehouse drivers (Postgres/MySQL/Oracle) | `Source` interface designed for zero-change swap → v1.2 |
| TTL token vault/IAM integration | Currently in-memory `fakeStore` → v1.2 |
| Automatic SQL risk tiering | v1.1 mandates human approval by design → v1.2 |
| Pinned glm-5.2 formal re-sign (Phase 14-10 Task 2) | Awaiting GLM quota; independent of v1.1 |
| `adv-001/002/003` GT-vs-corpus wording drift | 7 weak-faith cases concentrate here; spike-era GT phrasing |
| ReAct step-cap tuning (2 → 4) | Occasional `exceeds max steps` on thinking models; decide alongside the GLM comparison |
| Single-judge noise ±0.1 | `ref-009` flipped verdict between runs; cross-judge + human spot-check should be standing gates (Pitfall 8) |

### Inherited from v1.0 (still open)

BGE-M3 endpoint restore + compliance replay re-sign · Docker live build+run
verification · Phase 18 live E2E + FAN-06 operator rerun · MSYS2 gcc emutls
linking (backend must be built in WSL).

---

## 7. Getting Started

### Run the project

```bash
# 1. Configure
cp .env.example .env      # fill GLM_API_KEY, BGE_M3_URL, MILVUS_ADDRESS

# 2. Start the embedding endpoint (ollama / bge-m3), then the stack
docker compose up

# 3. Open
#    http://localhost:8080
```

The repo ships `skills/simple-qa` as the live default. To swap the demo domain:

```bash
cp -r examples/skills/compliance skills/ && rm -r skills/simple-qa
```

Hot-reload picks it up — that swap *is* the platform thesis in one command.

> ⚠️ **Build on WSL, not Windows.** CGO (go-duckdb + mattn/go-sqlite3) fails to
> link under this host's MSYS2 gcc (emutls ABI). Go does not read `.env`; watch
> for CRLF in `.env`.
> ```bash
> wsl -d Ubuntu-22.04 -- bash -c 'cd /mnt/d/code/ClaudeCode/ai-agent && \
>   /usr/local/go/bin/go build -o skillforge-server ./cmd/server'
> ```

### Tests

```bash
make test-unit          # Go unit tests
make test-frontend      # vitest (439/439 green as of v1.0 UAT)
make verify-phase-NN    # hermetic per-phase gates (11..17)
make check-cgo          # CGO toolchain precondition

# v1.1 packages specifically
go test ./internal/externalauth/ ./internal/nl2sql/ ./internal/auditprov/
go test -v -run TestUAT ./internal/externalauth/     # 7-step channel gate
```

### Eval gate

```bash
go run ./cmd/eval --frozen=compliance --archive        # full 50-case replay (~80 min)
go run ./cmd/eval --gate-check <report.json> --skills-dir examples/skills
go run ./cmd/eval --cases ref-009 --dump /tmp/dump.json   # single-case diagnosis
```

### Where to look first

| Goal | Start here |
|------|-----------|
| The trust contract (the actual moat) | `internal/platformfloor/` — the 7 immutable data rules prepended to every prompt |
| Refusal-by-construction | `internal/verdict/`, `internal/knowledge/grounding.go` |
| Skill contract | `internal/skill/{manifest,validate}.go` + `skills/*/SKILL.md` + `.planning/SKILL_SCHEMA.md` |
| Approval gate (headline differentiator) | `internal/agent/hitl_tools.go` + `approval_rules` table |
| v1.1 audit chain | `internal/auditprov/prov.go`, `internal/server/audit_chain.go` |
| v1.1 external channel | `internal/externalauth/manager.go` (⚠️ not wired — see §6) |
| Eval | `internal/eval/`, `cmd/eval/`, `eval/frozen/compliance.jsonl` |
| HTTP surface | `internal/server/server.go` (route table) |
| Frontend | `web/src/components/` (`AuditTimelineTab.tsx` is the newest) |

### Key directories

```
cmd/          server, eval, spike binaries
internal/     all Go packages (agent, skill, knowledge, eval, store, server, data, kpi, externalauth, nl2sql, auditprov)
web/          React 19 + Vite + TS frontend
skills/       live Skills (hot-reloaded)
examples/skills/   ready-to-use templates (simple-qa, calculator, compliance)
knowledge/authority/   tier-isolated authority corpus
eval/frozen/  frozen golden sets + .lock.json pins
kpis/         declarative KPI definitions (YAML)
datarules/    desensitization rules
.planning/    GSD planning state (ROADMAP, STATE, REQUIREMENTS, phases/, uat/, reports/)
```

---

## Stats

- **Timeline:** 2026-08-31 → 2026-09-06 (7 days)
- **Phases:** 5 / 5 complete (21, 22, 23, 24, 25) — 1 with a material integration gap
- **Commits:** 29 (since v1.0 archive `f5f4912`)
- **Files changed:** 91 (+6,493 / −499)
- **Contributors:** ninedrunk
- **Tag:** none yet — `v1.0` is the latest tag; `v1.1.0` tagging is user-driven

### Per-phase footprint

| Phase | Commits | Files | Lines |
|-------|---------|-------|-------|
| 21 External Authorized Channel | `a955450` | 12 | +2,242 |
| 24 Audit Chain Hardening | `30c466b`, `7b27b31` | 15 | +1,693 / −28 |
| 23 OSS Release | `56703bf` | 7 | +506 / −12 |
| 22 Eval CI Wiring | `f31f8e0` | 7 | +420 / −52 |
| 25 Eval Trust-Loop Closure | `23f1483`, `77071fe` (+ `9b08f26`, `b5e4e72`) | 17 | +154 / −16 |

### Milestone verdict

**Substantively shipped, with one honest asterisk.** Phases 22/23/24/25 are
complete and verifiable on disk. Phase 21 delivered a well-tested trust library
and HTTP surface that **is not yet connected to the running server** — so the 7
DATA requirements are library-satisfied but not deployment-live. The highest-value
next action is closing Gap 1 (three small wiring changes), then reconciling
REQUIREMENTS.md and the README architecture paragraph.
