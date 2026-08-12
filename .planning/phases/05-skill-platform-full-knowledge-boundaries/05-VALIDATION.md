---
phase: 5
slug: skill-platform-full-knowledge-boundaries
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-12
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `05-RESEARCH.md` § Validation Architecture. Task-level rows (Per-Task Verification Map) are filled during `/gsd:plan-phase`.

---

## Test Infrastructure

Full-stack phase — Go backend + React/Vitest frontend. Both must stay green; the wire drift alarm (`tsc --noEmit`) is the cheap early signal for the D-04/D12/D15 transport-swap invariant.

| Property | Value |
|----------|-------|
| **Framework (BE)** | Go `testing` + `testify` v1.11.1 (`[VERIFIED: go.mod]`) |
| **Framework (FE)** | vitest + @testing-library/react (`[VERIFIED: Phase 4 — 84/84 green]`) |
| **Config file (BE)** | none — standard `go test` |
| **Config file (FE)** | `web/vitest.config.*` (Phase-4 established) |
| **Quick run command (BE)** | `go test ./internal/skill/... ./internal/wire/... ./internal/retriever/... -count=1` |
| **Quick run command (FE)** | `cd web && npx tsc --noEmit` (D15 drift alarm — catches wire-break class early) |
| **Full suite command (BE)** | `go test ./...` |
| **Full suite command (FE)** | `cd web && npx vitest run && npx tsc --noEmit` |
| **Estimated runtime** | BE quick ~15s · FE quick ~8s · full ~120s |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/skill/... ./internal/wire/... ./internal/retriever/... -count=1` + `cd web && npx tsc --noEmit`
- **After every plan wave:** `go test ./...` + `cd web && npx vitest run`
- **Before `/gsd:verify-work`:** Full suite green + manual SC-walk (8 SCs). The Phase-4 `transport-swap.test.tsx` MUST stay green after every wire change.
- **Max feedback latency:** ~15s (quick commands above)

---

## Per-Task Verification Map

> Filled during planning. One row per task. Every task MUST have an `<automated>` verify or a Wave 0 dependency. Sampling continuity rule: no 3 consecutive tasks without automated verify.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| _(filled at plan time)_ | — | — | — | — | — | — | — | — | ⬜ pending |

*Requirement→test seed (from RESEARCH.md, expands into task rows above):*

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SYSK-04 | fsnotify reload after 1s debounce; in-flight pinned | unit + integration | `go test ./internal/skill -run TestRegistry_Watch -count=1` | ❌ Wave 0 |
| SYSK-05 | cosine routing picks right Skill; trace carries score | unit | `go test ./internal/skill -run TestRouter -count=1` | ❌ Wave 0 |
| SYSK-06 | wizard output passes `skill.Validate` (7-field) | unit | `go test ./internal/skill -run TestWizard_GeneratesValid -count=1` | ❌ Wave 0 |
| AGNT-06 | inspector renders data-decision + tool/reasoning tree | unit (FE) | `cd web && npx vitest run inspector` | ❌ Wave 0 |
| KNOW-06 | graph browser shows tier→doc→chunk; click reveals chunk | unit (FE) | `cd web && npx vitest run graph` | ❌ Wave 0 |
| CMPL-06 | counter-evidence 历史 chunks emit as annotation; never grounded | unit | `go test ./internal/server -run TestCounterEvidence_NotGrounded -count=1` | ❌ Wave 0 |
| CMPL-07 | redline emits only on 不合规/需补充; `data-redline` wire type | unit | `go test ./internal/server -run TestRedline_Conditional` + `go test ./internal/wire -run TestValidTypes_DataRedline` | ❌ Wave 0 |
| CMPL-08 | effective_to PRE-filter: expired → 0 hits; sentinel 0 → valid | unit | `go test ./internal/retriever/... -run TestEffectiveFilter -count=1` | ❌ Wave 0 |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

This phase adds 5 new BE files + 2 new FE component dirs — Wave 0 test scaffolding is mandatory (NOT covered by existing infrastructure).

- [ ] `internal/skill/registry_test.go` — SYSK-04 (atomic snapshot + 1s debounce + validation-failure-keeps-old D5-08)
- [ ] `internal/skill/router_test.go` — SYSK-05 (cosine routing + ambiguity threshold D5-03)
- [ ] `internal/skill/wizard_test.go` — SYSK-06 (output passes `Validate`; temperature-by-type D5-12)
- [ ] `internal/wire/chunk_test.go` — extend for `data-redline` (18th type) + `DecisionPayload` routing fields
- [ ] `internal/retriever/milvus/filter_test.go` + `internal/retriever/mongo/mongo_test.go` — extend for effective_to PRE-filter (sentinel 0 + expiry)
- [ ] `internal/server/annotate_test.go` — CMPL-06 (counter-evidence NEVER in `GroundingChecker`'s `chunksByID` map)
- [ ] `web/src/types/wire-types.ts` — D15 mirror (`data-redline` variant + `DecisionPayload` fields); `npx tsc --noEmit` is the gate
- [ ] `web/src/components/inspector/*` + `web/src/components/graph/*` — new component tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Ambiguous query routes to right Skill; trace shows score + selected Skill (live) | SYSK-05 (SC-1) | End-to-end semantic judgment + visual trace in UI | Install 2 Skills (compliance + general QA); send an ambiguous query; confirm the inspector shows `skill_scores[]` + `selected` |
| fsnotify hot reload — next query after 1s debounce uses new behavior (live) | SYSK-04 (SC-2) | Real filesystem watch + timing | Edit an installed `SKILL.md`'s description; wait >1s; confirm next query routes/behaves per new version without restart |
| Skill Wizard guided interview → valid SKILL.md (live) | SYSK-06 (SC-3) | Multi-turn NL interaction quality | Drive the wizard with "审批流合规审计 Skill"; confirm emitted file passes `validate.go` and registers via fsnotify |
| Counter-evidence shown next to verdict (live UI) | CMPL-06 (SC-6) | Visual annotation placement | Trigger a 权威 vs 历史 conflict; confirm counter-evidence chunks render beside the verdict, NOT as a grounding source |
| Redline rewrite proposed on policy gap (live UI) | CMPL-07 (SC-7) | NL rewrite quality + conditional firing | Submit a query whose verdict is 不合规/需补充; confirm `data-redline` chunk renders original_clause + proposed_rewrite |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (8 test files above)
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
