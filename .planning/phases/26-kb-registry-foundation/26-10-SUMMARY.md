---
phase: 26-kb-registry-foundation
plan: 10
subsystem: docs + structural-gates + operator-checkpoint
tags: [kbm-03, hyg-01, stride-26, structural-gate, todo-zero, red-green, operator-checkpoint, milvus-recreate, d14-b4-red-line]
requires:
  - "kb.Registry / CitableRole / Roles() contract (26-01/26-06)"
  - "admin KB endpoints + audit actions (26-05)"
  - "seed 'structure belongs to config, switch belongs to ops' merge (26-02, verified by 26-09 boot tests)"
  - "SKILLFORGE_MILVUS_RECREATE one-shot door (26-09)"
  - "verify-phase-16.sh run_stage form (Phase 16 gate precedent)"
provides:
  - "README 'Knowledge Bases' section — the user-facing canonical statement of the four KBM-03 role semantics"
  - "internal/kb godoc — the code-level canonical statement, dual-anchored to the README section"
  - "docs/threat-model/STRIDE-26.md — 5-threat register (backdoor / traversal / seed resurrection / CN blind spot / non-authority evidence) + phase close-out controls"
  - "scripts/verify-phase-26.sh + make verify-phase-26 — three structural locks (TODO-zero / TierEnum-zero / CN-tier-literal scoped ban) that every later phase inherits"
  - "repo-wide TODO/FIXME count == 0 in non-test Go, with the MEM-05 truth recorded where the TODO used to lie"
  - ".planning/phases/26-kb-registry-foundation/26-UAT.md — the operator runbook + 5 live acceptance criteria"
affects:
  - "Phase 27+ (document upload pipeline): inherits verify-phase-26 as a standing hygiene gate"
  - "Operator: the live re-seed is the only open item; 26-UAT.md is the executable runbook"
  - "Phase verifier: one listed FE residue (KnowledgeModal.tsx TIER_ORDER) to disposition"
tech-stack:
  added: [] # zero new dependencies
  patterns:
    - "structural gate proven RED on the pre-cleanup tree before the cleanup commit (D8-04 negative-proof discipline)"
    - "gate 3 targets exact quoted CN tier literals in non-comment Go — data values banned, prose exempt — so natural-language false positives are structurally excluded (Pitfall 5)"
    - "whitelist entries carry a reason + review date (复核 2027-03) so an unjustified exemption is visible in the diff"
    - "doc/code dual anchor for semantics: README section + kb package godoc cross-reference and must change together"
key-files:
  created:
    - docs/threat-model/STRIDE-26.md
    - scripts/verify-phase-26.sh
    - .planning/phases/26-kb-registry-foundation/26-UAT.md
  modified:
    - README.md
    - CONTRIBUTING.md
    - internal/kb/kb.go
    - Makefile
    - cmd/spike-history/long.go
    - cmd/spike-history/runner.go
    - cmd/spike-hitl/main.go
decisions:
  - "D26-10-A: gate 3 bans CN tier words only as EXACT quoted literals on non-comment lines, instead of the plan's any-line grep + 2-path whitelist — the plan's form measured 25+ hits (comment prose predating Phase 26: wire/chunk.go D5-13 comments, verdict/citation.go docs, retriever Hit doc comments) and could never go green without a 14-file whitelist that would have gutted the gate; the tightened predicate flags exactly the data-value hazard and only the plan's two intended prose sites need exemption"
  - "D26-10-B: the four real CN data values gate 3 exposed (spike-history dry-run hits + citations, spike-hitl smoke frozen_args anchor, spike-hitl smoke manifest resolved tiers) were migrated to the authority slug rather than whitelisted — whitelisting live legacy values would have exempted exactly the hazard the gate exists to catch (Pitfall 1 in miniature)"
  - "D26-10-C: MEM-05 disposal is a factual statement, not a promise — the shipped long-conversation escape is D10-B3 TruncateToCharBudget (PRIMARY and only window), the summarization branch never happened, and the record points at STATE D9-V2 + phases/09-* so no future reader infers middleware exists"
metrics:
  duration: ~19 min
  completed: 2026-09-07
  tasks: 3 (hermetic) + 1 checkpoint (operator-pending)
  commits: 3 (+1 docs closeout)
  files: 10 (3 created, 7 modified)
  insertions: 470
  deletions: 11
  gate_red_proofs: 4 (gate 1 on long.go:54; probes for gates 1, 2, 3)
status: complete — live re-seed operator-pending (stack reachable, authorization required)
---

# Phase 26 Plan 10: KB Registry Closeout Summary

**One-liner:** the four KBM-03 role semantics now live as dual anchors (README "Knowledge Bases" section + `internal/kb` godoc, with the citation whitelist already code-enforced at `kb.CitableRole`), `scripts/verify-phase-26.sh` carries three structural locks proven RED on the pre-cleanup tree and GREEN after it, repo-wide non-test Go TODO/FIXME count is zero with the MEM-05 truth replacing the stale promise, and the one remaining live step — the Milvus drop+re-seed — is handed to the operator as a five-criterion runbook with the stack probe-verified reachable.

## What Was Built

| Task | Deliverable | Commit |
|------|-------------|--------|
| 1 | README "Knowledge Bases" section (authority = 唯一可引用证据源 / historical / personal / draft, registry-driven "Adding a knowledge base" workflow, TierEnum mentions → 0) + CONTRIBUTING reads two-form + deactivated-KB fail-closed gotcha + `internal/kb/kb.go` canonical godoc (comment-only) + `docs/threat-model/STRIDE-26.md` (5 threats) | `140e3c4` |
| 2 | `scripts/verify-phase-26.sh` (run_stage form, 3 stages) + `make verify-phase-26` — **proven RED** on the pre-cleanup tree: exit 1 at Stage 01 naming `cmd/spike-history/long.go:54` (the only TODO), plus probe injections turning gates 1/2/3 red individually | `495dafe` |
| 3 | TODO-zero: `long.go` TODO rewritten as a factual D10-B3/MEM-05 statement (comment-only diff, verified) + 4 pre-migration CN tier data values → `authority` slug + `.planning/.../26-UAT.md` operator runbook → **gate exit 0** (RED→GREEN loop closed) | `3e2c599` |
| 4 (checkpoint) | Pre-flight probe executed once; live re-seed **NOT** run — handed to the operator (see Checkpoint Report below) | — |

### Task 1 — KBM-03 documentation (D14-B4 red line held)

- README gains a **Knowledge Bases** section: authority is the *only* citable evidence source
  (唯一可引用证据源) with the whitelist anchored at `internal/kb.CitableRole`; historical is a
  superseded revision out of default retrieval and reachable only via explicit `reads` (or an
  explicit point-in-time query); personal is user-private and excluded unless explicitly
  declared; draft is never retrievable regardless of the entry switch. Non-authority hits reach
  the model only through the counter-evidence annotation channel. The old
  "Adding a knowledge tier (the `TierEnum` + `knowledge/` convention)" bullet is replaced by the
  registry workflow (seed PR or `POST /api/admin/kb`, plus `knowledge/<slug>/`). `grep -c TierEnum README.md` == 0.
- CONTRIBUTING's `reads` row already carried the two legal forms from 26-07; this plan added the
  missing gotcha: a `reads` pattern naming an unknown or **deactivated** KB is a fail-closed
  load-time error listing active slugs — the data is untouched, the Skill just stops loading.
- `internal/kb/kb.go`: package doc + `Roles()` + `CitableRole` godoc now state the four role
  semantics as the canonical code-level contract, cross-reference the README section, and require
  the two to change together. Comment-only — `go build ./internal/kb/` and the kb suite stay green.
- `git diff --quiet internal/platformfloor/platform_floor.go` exit 0 **across the whole plan**
  (re-asserted at closeout): the prompt floor is byte-untouched, so the D14-B4 frozen-set re-run
  was never triggered. The reason KBM-03 landed in docs + godoc instead of the floor is recorded
  in STRIDE-26 §4.

### Task 2 — structural gates (RED proven first)

Gate design (see D26-10-A for the one plan deviation):

| Stage | Lock | Anti-false-positive design |
|-------|------|---------------------------|
| 01 | `TODO\(\|TODO:\|FIXME\(\|FIXME:` over non-test Go | only bracketed/colon TODOs count; the eino type name `deep.TODO` measures **0** by construction — and the stage asserts that live (`deep_hits != 0` = gate bug, not a whitelisted file) |
| 02 | `TierEnum` identifier zero over non-test Go | the grep-level complement to the enum deletion; tests exempt so the zero-assertion itself can name the identifier |
| 03 | CN tier word as an **exact quoted literal** on a **non-comment** line | comments and natural-language strings (e.g. `[历史结论不可解析]`) never match; the two plan-mandated prose whitelists (`platform_floor.go` prompt prose, `verdict_renderer.go` display sentinel) stay as explicit annotated filters with 复核 2027-03 dates |

**RED→GREEN evidence (D8-04):**

1. Whole-gate run on the pre-cleanup tree: `[STAGE-01-FAIL]`, exit **1**, hit list naming
   `cmd/spike-history/long.go:54` — the only real TODO.
2. Probe `// TODO(x):` in `internal/kb/gate26_probe_tmp.go` → gate 1 flagged the probe file
   alongside long.go:54; `deep.TODO` false-positive count during the probe run: **0**.
3. Probe `var TierEnumProbe = 0` → gate 2 flagged 2 hits; probe removed → 0.
4. Probe `var gateProbeCN = "权威"` → gate 3 flagged it **plus the 4 genuine pre-existing CN
   data-value sites** (`cmd/spike-history/runner.go:684,705`, `cmd/spike-hitl/main.go:94,287`);
   probe removed → back to exactly those 4.
5. After Task 3: gate exit **0**, all three stages PASS.

`make verify-phase-26` resolves to the same script (`make -n` under WSL; `mingw32-make` run
produced the identical STAGE-01 FAIL on the pre-cleanup tree).

### Task 3 — TODO-zero + data-value cleanup + runbook

- `cmd/spike-history/long.go`: the ruling-3 TODO is now a factual statement — Phase 10's shipped
  long-conversation escape is the **D10-B3 char-budget window** (`internal/history/projection.go`
  `TruncateToCharBudget`, PRIMARY and only window; legacy 6-message pre-cap removed), the
  "replace B3 with a summarization middleware" branch **never happened** (MEM-05 unimplemented —
  every `summariz*` hit in the repo is this spike's own `--summarize` reporting), and the record
  points at `.planning/phases/09-*` + STATE D9-V2. `git diff` on the file is comment-only
  (verified: 0 changed lines outside `//`); `grep -c TODO` on the file == 0;
  `go test ./cmd/spike-history/ -count=1` green.
- Deviation D26-10-B: the four pre-migration CN tier data values gate 3 exposed were migrated to
  the `authority` slug — spike-history's dry-run `retriever.Hit.Tier` and dry-run
  `verdict.Citation.Tier`, spike-hitl's `smokeToolArgs` frozen-args anchor (its sha256 test
  recomputes from the constant, so byte-identity within the run is preserved), and the smoke
  manifest's `SetResolvedTiersForTest`. Pre-migration these values no longer resolve against any
  registered KB — the exact silent-outage shape of Pitfall 1, caught by the new gate on day one.
- `26-UAT.md`: probe-once preconditions, the `SKILLFORGE_MILVUS_RECREATE=1` one-shot re-seed
  steps (set → boot → watch WARN + re-index → **remove the env** → restart), five acceptance
  criteria, and the blocked-with-notes discipline.

## Checkpoint Report (Task 4 — operator)

**Type:** human-verify / authorization gate (`gate="blocking"`)
**Status:** AWAITING OPERATOR — the live re-seed was **not** executed; per protocol even a
healthy stack does not authorize an autonomous destructive migration.

### Pre-flight probe (single pass, 2026-09-07T03:39Z — no retries)

| Endpoint | Target | Result |
|----------|--------|--------|
| Milvus (`MILVUS_ADDRESS`) | `10.62.10.34:19530` (gRPC/TCP) | **REACHABLE** (TCP connect OK, < 6 s) |
| BGE-M3 (`BGE_M3_URL`) | `http://10.62.0.123:11434/v1/embeddings` | **REACHABLE** — HTTP 200 in 3.17 s with a real embedding payload |

The intranet stack is **up** — this is the unexpected branch of the plan's protocol, so the
outcome is "ready, awaiting authorization", not "blocked-with-notes".

### Operator runbook (full text: `.planning/phases/26-kb-registry-foundation/26-UAT.md`)

1. Probe once (commands in UAT §1) — already green as of the timestamp above; re-probe at run time.
2. `echo 'SKILLFORGE_MILVUS_RECREATE=1' >> .env`, then boot under WSL:
   `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go run ./cmd/server"`.
3. Watch the boot log for the RECREATE WARN (names the collection + the re-embed cost) and the
   boot-time re-index lines.
4. **Remove the env line** and restart clean — one-shot semantics; leaving it set re-embeds the
   corpus on every boot.
5. Acceptance (all five, no partial credit): ① a tier query returns non-zero hits; ② `/eval`
   face is not 100% refusal; ③ graph nodes have `chunkCount > 0`; ④ `/admin` → Knowledge Bases:
   create + deactivate + restart shows the deactivation survives (seed does not resurrect); ⑤
   `bash scripts/verify-phase-26.sh` exit 0.
6. If the stack goes down mid-run: stop, record blocked-with-notes in STATE Blockers (14-08
   precedent), do not retry-loop or score-chase.

### Resume signal

Reply **"approved"** once the five criteria pass (or describe the failure); if the stack is
unreachable at run time, reply **"blocked-with-notes"** with the probe output.

## Deviations from Plan

**1. [Rule 3 - Blocking + Decision] Gate 3 form: exact-quoted-literal + comment-strip instead of any-line grep + 2-path whitelist**
- **Found during:** Task 2 (baseline measurement before writing the gate)
- **Issue:** the plan's `grep -rnIE '权威|历史|个人|草稿'` whitelist design measured **25+ hits**
  across ~14 files on the real tree — overwhelmingly *comment prose predating Phase 26*
  (`internal/wire/chunk.go` D5-13 comments, `internal/verdict/citation.go` doc comments,
  `internal/retriever/{retriever,mongo/rank}.go` Hit doc comments, `internal/store` doc/DDL
  comments). Going green would have required a 14-entry whitelist — the gate-fatigue shape
  Pitfall 5 exists to prevent — and the gate could never have reached GREEN as specified.
- **Fix:** gate 3 strips comment lines (`//`, `*`, `/*`, `--`) and requires the CN tier word to
  be an **exact whole string literal** — i.e. a data value. The plan's two intended prose sites
  remain as explicit, annotated, dated whitelist filters; they are now its only exemptions.
- **Files modified:** scripts/verify-phase-26.sh
- **Commit:** `495dafe`

**2. [Rule 1 - Bug] Four pre-migration CN tier data values in spike harnesses**
- **Found during:** Task 2 probe proof (gate 3 flagged them alongside the injected probe)
- **Issue:** `cmd/spike-history/runner.go:684,705` (dry-run `retriever.Hit.Tier` and
  `verdict.Citation.Tier` = `"权威"`) and `cmd/spike-hitl/main.go:94,287` (smoke frozen-args
  anchor + smoke manifest resolved tiers) still carried CN tier values that resolve against **no**
  registered KB after the slug switch — the silent-zero-hit shape of Pitfall 1, in miniature,
  live in the tree.
- **Fix:** migrated to the `authority` slug. `smokeToolArgs`'s HITL-07 byte-identity contract is
  within-run (the sha256 test recomputes from the constant), so the anchor stays valid.
- **Files modified:** cmd/spike-history/runner.go, cmd/spike-hitl/main.go
- **Commit:** `3e2c599`

**3. [Decision] CONTRIBUTING already carried the 26-07 reads-form text**
- The plan asked CONTRIBUTING to add the two `reads` forms + draft-role placeholder notes;
  26-07/26-08 had already landed both (`:71`, `:117`, `:198`). The genuinely missing piece — the
  deactivated-KB fail-closed gotcha — was added as gotcha 3. No duplication introduced.

## Verification

| Check | Result |
|-------|--------|
| `bash scripts/verify-phase-26.sh` | exit **0** — 3/3 stages PASS (GREEN after cleanup) |
| RED proof before cleanup | exit **1** at Stage 01 naming `cmd/spike-history/long.go:54` |
| Probe proofs (gates 1/2/3) | each turned red on injection; tree restored to pre-probe state after removal |
| `deep.TODO` false-positive count | **0** (asserted live inside Stage 01) |
| Repo-wide non-test Go TODO/FIXME | **0** |
| `grep -c TierEnum README.md` / non-test Go | **0** / **0** |
| `grep -c "唯一可引用证据源" README.md` | 1 (authority section) |
| STRIDE-26.md T-26-xx register entries | 5/5, each with disposition + code anchor + verification gate |
| `git diff --quiet internal/platformfloor/platform_floor.go` (whole plan) | exit **0** — D14-B4 red line held |
| `go test ./cmd/spike-history/ ./cmd/spike-hitl/ -count=1` | both ok; `go vet` clean |
| `go build ./internal/kb/` + kb suite | ok (comment-only godoc change) |
| 26-09 WSL full gate re-run | `make check-cgo` + `go build ./...` + `go vet ./internal/... ./cmd/...` OK; `go test ./... -count=1` exit 0, 40 packages ok, **0 FAIL** |
| `make verify-phase-26` | resolves to the script (`make -n` WSL); `mingw32-make` run reproduced the identical gate output |

## Threat Model Compliance

| Threat | Mitigation delivered |
|--------|----------------------|
| T-26-10-1 (gate false-positive fatigue) | gate 1 matches only bracketed/colon TODOs (`deep.TODO` = 0, asserted live); gate 3 targets data values only; whitelists carry reasons + 复核 2027-03; every gate proven RED before GREEN |
| T-26-10-2 (frozen eval baseline) | platform floor byte-identical across the plan (`git diff --quiet` exit 0); KBM-03 landed in README + kb godoc; rationale recorded in STRIDE-26 §4 |
| T-26-10-3 (live migration on a failing stack) | probe executed once before anything destructive; stack healthy → re-seed left to operator authorization; blocked-with-notes discipline written into 26-UAT §4 |
| T-26-10-4 (narrative lying about MEM-05) | long.go now states D10-B3 TruncateToCharBudget as the shipped window and records MEM-05 as never implemented, with pointers to STATE D9-V2 + phases/09-* |
| T-26-SC (dependency install) | accepted — zero new dependencies |

## Known Stubs

None. The re-seed door is real, hermetically tested code (26-09) whose live execution is
deliberately an operator act, not a stub.

## Known Residue (for the phase verifier)

- `web/src/components/layout/KnowledgeModal.tsx:40` — `TIER_ORDER: Tier[] = ['权威','历史','个人','草稿']`,
  the FE display-order map 26-08 flagged as out of its scope. Phase 26's gates scan Go only, so
  this file is outside them; it is a display-order constant, not a data value. Listed in 26-UAT §5
  so the verifier dispositions it deliberately (Phase 28's GraphCanvas dynamic work is the natural
  owner).

## Self-Check: PASSED

- Files: `docs/threat-model/STRIDE-26.md`, `scripts/verify-phase-26.sh`,
  `.planning/phases/26-kb-registry-foundation/26-UAT.md` exist (created); `README.md`,
  `CONTRIBUTING.md`, `internal/kb/kb.go`, `Makefile`, `cmd/spike-history/long.go`,
  `cmd/spike-history/runner.go`, `cmd/spike-hitl/main.go` modified — all present in
  `git diff --stat ff72072..HEAD` (10 files, +470/−11, zero tracked-file deletions).
- Commits: `140e3c4`, `495dafe`, `3e2c599` — all present in `git log`.
- Gates: verify-phase-26 exit 0; WSL check-cgo/build/vet/test exit 0; platform_floor.go
  zero-byte assertion exit 0.
