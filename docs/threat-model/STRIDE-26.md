# STRIDE-26 — Phase 26 (知识库注册表地基 — KB Registry Foundation) Threat Model

**Phase:** 26 (TierEnum 4-value enum → registry-driven knowledge bases, KBM-01..06 + HYG-01)
**Version:** 1.0
**Last updated:** 2026-09-07
**Owner:** Platform Engineering + Internal Audit Specialist
**Status:** Accepted at the Phase 26 hermetic gate (`scripts/verify-phase-26.sh`). The live
vector-library migration is an operator checkpoint (`26-UAT.md`) and closes with an explicit
operator verdict — it is NOT closed by this register.

---

## 1. Scope & Trust Boundaries

Phase 26 replaces the compile-time 4-value tier enumeration with a runtime **knowledge-base
registry**: a controlled seed file (`knowledge_bases.yaml`) + a persisted sqlite table
(`knowledge_bases`), consumed through the `kb.Registry` interface by the skill, knowledge,
server and data packages. The trust boundary crossings introduced are:

| Boundary | Description |
|----------|-------------|
| `POST/GET /api/admin/kb*` (admin mutation surface) | The FIRST admin surface in the repo with create/deactivate verbs on a trust-bearing registry. Gated by `X-Admin-Token` (constant-time compare), every mutation lands an INSERT-only audit row, and no delete verb exists. |
| `kb.Registry` interface (`internal/kb`) | The single source of truth for slug identity + semantic role + deactivation switch. `Lookup` is fail-closed: deactivated ≡ unregistered ≡ absent. |
| `kb.ValidateSlug` → filesystem path segment | A slug becomes `knowledge/<slug>/` (directory walk) and a Milvus filter value. The slug whitelist is the only gate before a path join. |
| Seed file vs runtime state | The seed declares STRUCTURE; the retrieval switch belongs to operations. A seed re-load must never resurrect an admin deactivation. |
| Citation whitelist (`kb.Citable` / `CitableRole`) | The insertion points where a retrieved chunk may enter the grounding closure. Only authority-role chunks may become `[ref-N]` evidence. |
| Live vector library (Milvus/Mongo `tier` column) | Legacy rows carry CN-char tier values. Until the one-shot re-seed runs, every tier query fail-closes to zero hits — a functional outage that LOOKS like a security pass. |
| Structural gates (`scripts/verify-phase-26.sh`) | The gate is editable text; a loosened regex or an unjustified whitelist entry silently voids the hygiene guarantee it certifies. |

Phase 26 inherits the trust semantics of Phases 11–16 unchanged (KNOW-02 fail-closed,
D-09 refusal-by-construction, D5-13 annotation channel, D14-B4 frozen-eval discipline) and
adds the five new threats registered below.

---

## 2. STRIDE Register — Phase 26 New Threats

Each threat names its code anchor and its verification gate.

| Threat ID | Category | Component | Disposition | Mitigation | Verification Gate |
|-----------|----------|-----------|-------------|------------|-------------------|
| **T-26-01** | Elevation of Privilege / Repudiation | KB registry backdoor — an HTTP surface that creates/deactivates knowledge bases without review, or a delete verb that destroys evidence | mitigate | The ONLY mutation surface is `POST /api/admin/kb` + `POST /api/admin/kb/:slug/deactivate` (`internal/server/kb_admin.go:167,238`), both behind `isAdminRequest` (`crypto/subtle` constant-time, `internal/server/audit.go:114-129`, unset env ⇒ 403 fail-closed — CHECK ORDER locked by `TestKBAdminCheckOrder` / `TestKBAdminUnsetEnv403`). Every mutation writes an INSERT-only audit row (`action_kb_created` / `action_kb_deactivated`, `putKBAdminAuditRow`). No DELETE route exists (`internal/server/server.go:482-488`) and `kb.Registry` deliberately exposes no delete verb — deactivation hides, it never destroys (KBM-02). Structure-level changes to the baseline set go through the controlled seed file (PR review only). | `go test ./internal/server/ -run TestKBAdmin -count=1`; route-table review (no DELETE verb) |
| **T-26-02** | Tampering | Slug path traversal / filter injection — a slug that escapes `knowledge/` or breaks the Milvus filter expression | mitigate | DOUBLE gate (mirrors `internal/server/knowledge.go:340-369`): (1) `kb.ValidateSlug` whitelist regex `^[a-z][a-z0-9-]{0,19}$` with a reserved-word set (`data` + the four role names — `internal/kb/slug.go`), so a traversal-shaped value never reaches the registry; (2) the slug must exist as an ACTIVE registry entry (`Registry.Lookup` fail-closed) before it is used. The document path joins also keep the legacy `filepath.Base(file) == file` + backslash-reject guard. Because slugs are `[a-z0-9-]` only, the `%q`-quoted Milvus filter value stays injection-safe. | `go test ./internal/kb/ -run TestValidate -count=1`; `TestKBAdminCreateSlugValidation`; docs-serving traversal tests in `internal/server/` |
| **T-26-03** | Tampering / Repudiation | Deactivated knowledge base resurrected by the seed on restart — an admin's hide decision silently undone | mitigate | "结构归配置，开关归运营" (structure belongs to config, the switch belongs to ops): the seed UPSERTS declared structure fields but the persisted `retrieval_enabled` value wins (`internal/kb/seed.go` package doc + `kb.Open` merge). Locked by `TestOpenDeactivationSurvivesSeedReload` (`internal/kb/open_test.go:146-179`), the cmd/server boot-order twin (`cmd/server/main_test.go:155` "deactivation survives a reopen"), and `TestOpenAdminCreatedRowSurvives` (the seed is not a garbage collector either). | `go test ./internal/kb/ ./cmd/server/ -count=1` |
| **T-26-04** | Denial of Service | CN-tier residue blind spot — legacy vector rows keep CN-char tier values, every tier query matches 0 rows, the system fail-closes to 100% refusal and LOOKS secure while being functionally dead | mitigate | The one-shot re-seed door: `SKILLFORGE_MILVUS_RECREATE=1` (literal `"1"` only — `milvus.ShouldRecreate`, 16 closed values tested) drops + re-creates the collection and lets the boot index re-seed it, with a WARN naming the collection and the re-embed cost. Acceptance is OPERATOR-EXECUTED against five live criteria (`26-UAT.md`: non-zero retrieval hits, /eval not 100% refusal, graph chunkCount > 0, admin create/deactivate round-trip, gate exit 0). When the intranet stack is unreachable the discipline is blocked-with-notes (no score-chasing retries) — the outage must stay visible. | Operator checkpoint (26-10 / `26-UAT.md`); `TestRecreateEnvGateParsing`; `bash scripts/verify-phase-26.sh` |
| **T-26-05** | Elevation of Privilege | Non-authority chunk promoted into citation evidence — historical/personal/draft material backed by a `[ref-N]` | mitigate | The whitelist is enforced at the chunk-insertion points, not at the retriever: `kb.Citable(reg, tier)` (`internal/kb/kb.go:235`) is the single predicate, called by the agent-side accumulator gate (`internal/agent/trust_floor.go`) and the server-side verdict pipeline (`internal/server/pipeline_tier_test.go TestCitableChunksByID`). Non-authority hits flow only through the D5-13 counter-evidence ANNOTATION channel (`internal/server/annotate.go`, resolved from registry roles — `TestCounterEvidenceUsesRegistryRoles`, `TestCounterEvidenceEmptyRoleZeroHit`). A nil registry is the explicit legacy posture (tests/degraded paths), never a production assembly state. | `go test ./internal/agent/ ./internal/server/ -run 'TestCitable|TestCounterEvidence' -count=1` |

---

## 3. Threat Notes

### T-26-01 — Registry Backdoor

**Stakes.** A knowledge base is a trust boundary: whoever registers one decides what a
whole class of Skills may read, and whoever flips a switch decides what disappears from
retrieval. An unauthenticated or unaudited create/deactivate path would let an attacker
plant an authority-role base (instantly citable evidence) or blind a compliance Skill —
the same class of backdoor the KPI registry's immutability was built to prevent (D16-08-E).

**Why the KPI posture does not transfer.** KPIs are load-bearing SQL definitions, so their
registry is immutable by construction. Knowledge bases are operational content, and KBM-02
deliberately requires a runtime create/deactivate surface — so the control moves from
"no mutation" to "authenticated + audited + non-destructive": token gate, INSERT-only audit
row per mutation, and no delete verb anywhere in the interface or the route table.

### T-26-03 — Seed Resurrection

**Stakes.** A seed that always upserts the retrieval switch turns every restart into a
rollback of admin decisions (Pitfall 6a). The inverse failure — seed changes never reaching
existing deployments — is handled by upserting structure while deferring the switch, so a
PR that adds or renames a base still lands on existing deployments without overwriting
operational state.

### T-26-04 — The Blind-Spot Refusal

**Stakes.** After the slug switch, un-migrated vector rows match nothing. D-09 does its job
(fail-closed refusal), which means the failure mode is silent: refusal metrics can even
look healthy. The mitigation is procedural as much as technical — a hermetic gate cannot
prove a live corpus migrated, so the phase ships an explicit operator runbook with five
acceptance criteria and a blocked-with-notes discipline instead of letting the milestone
close on an unverified corpus.

---

## 4. Phase 26 Close-Out Controls (Plan 26-10)

| Control | Threat it addresses | Mechanism |
|---------|--------------------|-----------|
| Structural hygiene gates | Gate self-threat (false-positive fatigue, T-26-10-1) | `scripts/verify-phase-26.sh`: gate 1 matches only `TODO(` / `TODO:` / `FIXME(` / `FIXME:` (the eino type name `deep.TODO` measures zero hits); gate 2 greps `TierEnum` to zero; gate 3 bans CN tier literals **as exact quoted data values in non-comment code**, with the two natural-language sites (`internal/platformfloor/platform_floor.go`, `internal/history/verdict_renderer.go`) explicitly exempted and annotated with a review date. Each gate was proven RED on the pre-cleanup tree before the cleanup commit (D8-04 negative-proof discipline). |
| Frozen-eval discipline | Prompt-floor drift (T-26-10-2) | `internal/platformfloor/platform_floor.go` is byte-unchanged across Phase 26 (`git diff --quiet` asserted in 26-10): KBM-03 semantics were placed in the README + `internal/kb` godoc, NOT in the prompt floor, so the D14-B4 frozen-set re-run was never triggered. |
| Factual TODO disposal | Narrative lying about MEM-05 (T-26-10-4) | `cmd/spike-history/long.go` states the actual Phase 10 escape hatch — the D10-B3 char-budget window (`internal/history/projection.go` `TruncateToCharBudget`) — and records that the MEM-05 summarization middleware was never implemented. Zero TODO vocabulary remains. |

---

## 5. Cross-References

- `docs/threat-model/STRIDE-13.md` — inherited fan-out/tier-closure threats (T-P13-1 tier
  boundary leak remains the parent invariant of KBM-04's reads resolution).
- `docs/threat-model/STRIDE-11.md`, `STRIDE-12.md`, `STRIDE-14.md` — HITL/approval and
  data-channel threats unchanged by Phase 26.
- `.planning/phases/26-kb-registry-foundation/26-UAT.md` — the live operator runbook and
  the five acceptance criteria for T-26-04.
- `REQUIREMENTS.md` KBM-01..06 / HYG-01 — the requirement contract this register closes.
