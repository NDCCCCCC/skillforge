---
phase: 02-compliance-vertical-slice
plan: 01
subsystem: compliance-grading
tags: [go, graduation, grounding, spike, eino-adk, hertz, milvus]

# Dependency graph
requires:
  - phase: 01-integration-spike-trust-contract
    provides: "Phase 1 spike (internal/spike/) + Verdict schema + UIMessageChunk + Milvus client + Hertz SSE"
provides:
  - "Three production-grade internal packages graduated from internal/spike/: internal/verdict, internal/wire, internal/retriever"
  - "D2-06 Verdict.Citations upgraded from []string to []Citation{{ChunkID, Span, Tier}} — programmatic grounding"
  - "D2-01 GroundingChecker with substring-contains check after width.Narrow + whitespace normalization (D2-05)"
  - "D2-02 ApplyDowngrade policy: Status=需补充, Confidence cap 0.89, ReviewRequired=true"
  - "integration/contract test pinned to the new []Citation schema + new TestContractCitationGrounding"
affects: [02-02, 02-03, 02-04, 03-agentic-retrieval-trust-evaluation]

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/text (width.Narrow for D2-05 normalization — promoted to direct dep)"
  patterns:
    - "Graduation by MOVE (git rename), not copy — single source of truth per concern"
    - "Pure predicate + mutating policy split: GroundingCheck (read) vs ApplyDowngrade (mutate)"
    - "Idempotent downgrade: ApplyDowngrade can be called multiple times without progressive degradation"
    - "T-1-16 compile-time SSOT: every consumer imports internal/verdict, drift caught at compile time"

key-files:
  created:
    - internal/verdict/verdict.go
    - internal/verdict/citation.go
    - internal/verdict/validate.go
    - internal/verdict/verdict_test.go
    - internal/verdict/grounding.go
    - internal/verdict/grounding_test.go
    - internal/wire/chunk.go
    - internal/wire/sse.go
    - internal/retriever/client.go
    - internal/retriever/filter.go
    - internal/retriever/search.go
    - internal/retriever/embed.go
    - internal/retriever/filter_test.go
    - .planning/phases/02-compliance-vertical-slice/02-01-SUMMARY.md
  modified:
    - cmd/spiked/e2e.go
    - cmd/spiked/e2e_test.go
    - cmd/spiked/milvus.go
    - cmd/spiked/milvus_test.go
    - cmd/spiked/sse.go
    - cmd/spiked/sse_test.go
    - integration/contract/contract_test.go
  deleted:
    - internal/spike/verdict.go
    - internal/spike/verdict_test.go
    - internal/spike/uimsg.go
    - internal/spike/hertz_sse.go
    - internal/spike/milvus_client.go
    - internal/spike/milvus_client_test.go

key-decisions:
  - "Graduation by MOVE — git renames preserve history; no two definitions of Verdict exist anywhere"
  - "D2-06 Citations is []Citation{{ChunkID, Span, Tier}} — not []string — so the grounding checker can verify each span"
  - "Split GroundingCheck (pure read) from ApplyDowngrade (mutating policy) — pure predicate is trivially unit-testable + reusable by Ragas eval pipeline"
  - "GroundingChecker is the canonical pipeline entry point (combines check + downgrade); GroundingCheck remains exported for audit/eval paths"
  - "width.Narrow is idempotent on already-halfwidth runes — D2-05 normalization handles 全角↔半角 drift between LLM Span and chunk text"
  - "ConfidenceFloor = 0.89 + DowngradeStatus = 需补充 — forced D2-02 policy constants exported so the audit trail is reproducible"
  - "ApplyDowngrade is idempotent — first call caps Confidence at 0.89 + sets Status=需补充; second call is a no-op (does not progressively lower Confidence)"

patterns-established:
  - "Pattern: Package split by responsibility — verdict (schema + grading), wire (Hertz SSE primitives), retriever (Milvus + embedding)"
  - "Pattern: Single-source-of-truth via import graph — internal/verdict is the ONLY place Verdict is declared; rename there breaks compile in every consumer"
  - "Pattern: Adversarial unit tests pin the failure modes the moat is designed to catch (fabricated span, novel claim, idempotence, etc.)"

requirements-completed: [CMPL-02, CMPL-03, ARAG-04]

# Metrics
duration: 35min
completed: 2026-08-11
---

# Phase 2 Plan 1: Graduate Spike + Grounding Moat Summary

**Three production-grade packages graduated from internal/spike/ + D2-01 programmatic grounding checker (Normalize + substring + D2-02 downgrade) + D2-06 Verdict.Citations upgraded to []Citation**

## Performance

- **Duration:** 35 min
- **Started:** 2026-08-11T15:00:00Z
- **Completed:** 2026-08-11T15:35:00Z
- **Tasks:** 3 (Task 1 = prior agent commit 59aad41; Task 2 = 31428f7; Task 3 = d5514b9)
- **Files modified:** 27 (13 created + 7 modified + 6 deleted + 1 SUMMARY)

## Accomplishments

- **Graduation by MOVE** — internal/spike/{verdict,uimsg,hertz_sse,milvus_client} moved into three production-grade packages: internal/verdict/, internal/wire/, internal/retriever/. Git history preserves the rename (87-91% similarity); exactly ONE Verdict definition exists in the repo. A field rename in internal/verdict/verdict.go breaks compile in every consumer at the same time — runtime drift is structurally impossible (T-1-16 SSOT).
- **D2-06 schema evolution** — Verdict.Citations graduated from `[]string` (clause IDs) to `[]Citation{{ChunkID, Span, Tier}}` so each citation is programmatically verifiable by the grounding checker. The integration contract test (TestContractStructuredOutputSchema) pins the new schema.
- **D2-01 grounding moat** — `verdict.GroundingChecker(v, chunksByID)` is the canonical pipeline entry point: it calls `GroundingCheck` (pure predicate) then `ApplyDowngrade` (mutating policy) if any citation's Span does not appear in its chunk text after normalization. Downgrade sets Status=需补充, caps Confidence at 0.89, forces ReviewRequired=true (D2-02).
- **D2-05 normalization** — `verdict.Normalize(s)` applies width.Narrow (全角→半角) + whitespace collapse + trim. Pure function, exported so Phase 4 UI can reuse. Adversarial test confirms `全角，` and halfwidth `,` collapse identically.
- **5 adversarial tests** pin the four failure modes + idempotence:
  - TestGroundingCheckFabricatedSpan (LLM cites non-existent text → fail)
  - TestGroundingCheckVerbatimSpan (happy path)
  - TestGroundingCheckFullwidthHalfwidthMix (D2-05 width.Narrow)
  - TestGroundingCheckNovelClaim (ChunkID not retrieved → fail)
  - TestApplyDowngradeIdempotent (double-apply is no-op)
- **Cross-package contract test** — TestContractCitationGrounding in integration/contract asserts that fabricated spans fail and verbatim spans pass (fabricated/verbatim span tests live there, full coverage in internal/verdict/grounding_test.go).

## Task Commits

Each task was committed atomically:

1. **Task 1: Promote golang.org/x/text direct dep** - `59aad41` (chore — prior agent)
2. **Task 2: Graduate spike + D2-06 schema + contract test update** - `31428f7` (feat)
3. **Task 3: D2-01 programmatic grounding checker + 5 adversarial tests** - `d5514b9` (feat)

**Plan metadata:** This SUMMARY.md (committed separately per the REQUIRED ORDER: Write → commit → narrate)

_Note: Plan also has a 4th commit for the tracking files (STATE.md + ROADMAP.md) per the execution plan._

## Files Created/Modified

### Created (13 files)
- `internal/verdict/verdict.go` — Verdict struct (Status/Confidence/Citations/ReviewRequired) — the SSOT
- `internal/verdict/citation.go` — Citation + Refusal types + RefusalReason* constants (D2-04 / ARAG-05)
- `internal/verdict/validate.go` — ValidStatuses enum gate + Validate function (T-1-16 SSOT gate)
- `internal/verdict/verdict_test.go` — 5 tests pinning the schema + status enum
- `internal/verdict/grounding.go` — Normalize + GroundingCheck + ApplyDowngrade + GroundingChecker (D2-01/D2-02/D2-05)
- `internal/verdict/grounding_test.go` — 5 adversarial + 2 normalize unit tests
- `internal/wire/chunk.go` — UIMessageChunk + ValidTypes + IsValidType (T-1-11 wire gate)
- `internal/wire/sse.go` — EmitChunk + StreamChunks + ContentTypeIsSSE (Hertz SSE writer)
- `internal/retriever/client.go` — MilvusConfig + NewMilvusClient + EnsureCollection + SeedDocuments
- `internal/retriever/filter.go` — BuildMilvusFilter (default-deny on empty tiers — D-09)
- `internal/retriever/search.go` — SearchWithTierFilter + TierSearchResult + Document (PRE-similarity tier filter)
- `internal/retriever/embed.go` — EmbedQuery (BGE-M3) + EmbedQueryDeterministic (spike-only fallback)
- `internal/retriever/filter_test.go` — Table-driven BuildMilvusFilter tests + hostile-char escape tests

### Modified (7 files)
- `cmd/spiked/e2e.go` — internal/spike → internal/verdict + internal/wire
- `cmd/spiked/e2e_test.go` — internal/spike → internal/verdict; D2-06 []Citation schema
- `cmd/spiked/milvus.go` — internal/spike → internal/retriever
- `cmd/spiked/milvus_test.go` — internal/spike → internal/retriever
- `cmd/spiked/sse.go` — internal/spike → internal/wire (EmitChunk + UIMessageChunk)
- `cmd/spiked/sse_test.go` — internal/spike → internal/wire
- `integration/contract/contract_test.go` — All three packages imported; D2-06 schema in TestContractStructuredOutputSchema; new TestContractCitationGrounding

### Deleted (6 files)
- `internal/spike/verdict.go`, `verdict_test.go` — moved to internal/verdict/
- `internal/spike/uimsg.go` — moved to internal/wire/chunk.go
- `internal/spike/hertz_sse.go` — moved to internal/wire/sse.go
- `internal/spike/milvus_client.go`, `milvus_client_test.go` — moved to internal/retriever/

## Decisions Made

- **Pure check vs mutating downgrade split.** GroundingCheck is exported as a pure predicate that returns GroundingReport. ApplyDowngrade is exported as the mutation function. GroundingChecker is the canonical convenience wrapper that combines both for the 02-03 pipeline. This split lets eval pipelines (Ragas Faithfulness scorer) call GroundingCheck directly without needing to manage verdict mutation.
- **DowngradeStatus + ConfidenceFloor exported.** The constants `"需补充"` and `0.89` are exported so audit/eval paths can reference the policy by symbol, not magic-number repetition.
- **GroundingCheck returns UngroundedCitations in input order.** This preserves the citation-index correlation for audit trails — the pipeline trace can list "citation #2 (auth_999) was ungrounded" without re-walking the input.
- **D2-05 width.Narrow scope.** `width.Narrow.String` is applied to both Span and chunk text in GroundingCheck so both sides canonicalize. The pure `Normalize` function does the same; tests confirm `全角，` and `,` collapse identically.
- **Contract test assertion (TestContractCitationGrounding) is intentionally minimal.** A fabricated span must fail, a verbatim span must pass. The full adversarial coverage (全角/半角, novel claim, idempotence) lives in internal/verdict/grounding_test.go. The contract test pins the cross-package reachability + a smoke check.

## Deviations from Plan

None - plan executed as specified.

## Issues Encountered

- **Pre-existing half-done state on main tree.** The previous agent had started graduation on the main tree (`D:\code\ClaudeCode\ai-agent\`) but was interrupted by 429 before commit. The work was replicated to this isolated worktree (`worktree-agent-aa38814ab10b81b38`) so commits could be made against the `worktree-agent-aa38814ab10b81b38` branch. The user/orchestrator will need to merge this worktree branch back to master. The main tree's uncommitted changes were not modified by this agent (the worktree isolation prevented touching the main tree's git state).
- **Plan referenced `GroundingChecker` function name as a single entry point.** Implementation splits this into GroundingCheck (pure) + ApplyDowngrade (policy) + GroundingChecker (convenience wrapper). The convenience wrapper provides the exact behavior the plan called for; the split is an additional refinement that makes the package more testable and eval-friendly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 2 Plan 2 (skill package + corpus) is unblocked:
- internal/verdict is ready as the type the Skill spec emits into.
- internal/wire is ready for Skill-bound SSE emission.
- internal/retriever is ready for Skill.reads → BuildMilvusFilter integration.
- D2-06 []Citation schema is the contract 02-02 SKILL.md files should target.

02-03 (verdict pipeline + server) is unblocked:
- GroundingChecker is the canonical pipeline integration point.
- The contract test pins the wire shape — any drift in the pipeline breaks compile.

No concerns or blockers.

---
*Phase: 02-compliance-vertical-slice*
*Plan: 01*
*Completed: 2026-08-11*
