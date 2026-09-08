# Phase 28 — Threat Model (STRIDE-28)

**Phase:** 28 — 知识图谱动态化 + 关系层 (Dynamic Knowledge Graph)
**Authored:** 2026-09-07 (Plan 28-07)
**Status:** active — locks the trust surface for Phase 28

---

## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| client → GET /api/knowledge/graph | Public read; no admin token |
| client → GET /api/knowledge/graph/neighbors | Public read |
| client → GET /api/knowledge/graph/path | Public read |
| retriever → EdgeGraph (derived) | Deterministic, zero LLM |
| cache → invalidation hook | Cross-phase callback (Phase 27 → Phase 28) |
| kb.Citable (single predicate) | Query-time citability gate (Phase 26 contract; UNCHANGED) |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-28-01 | Tampering | graph edges | mitigate | Pure derivation from DocumentMeta + audit rows; deterministic; same input → byte-identical output |
| T-28-02 | Information Disclosure | citation co-occurrence edges | mitigate | AuditQuery yields only verdict citations; no new disclosure path beyond what FE already sees |
| T-28-03 | Denial of Service | graph endpoint | mitigate | 4 SKILLFORGE_GRAPH_* env-driven caps (MAX_NODES, MAX_EDGES, DEPTH, HOPS); degraded response shape with explicit reason |
| T-28-04 | Tampering | cache stale | mitigate | 5s TTL + invalidation hook on Phase 27 upload success (max staleness 5s) |
| T-28-05 | Tampering | kb.Citable weakened | mitigate | graph_edges.go does NOT call kb.Citable; query-time gate stays single-source at Phase 26 |
| T-28-06 | Information Disclosure | neighbor expansion (depth bomb) | mitigate | SKILLFORGE_GRAPH_NEIGHBOR_DEPTH_LIMIT default 2; depth > limit is hard-capped |
| T-28-SC | Supply Chain | zero new Go packages | mitigate | Phase 28 has no new Go deps (zero-deps standard) |
| T-28-LLM | Tampering | LLM in graph edges | mitigate | Explicitly forbidden: edges are deterministic algorithms (BFS, Jaccard, sort); no LLM calls anywhere in graph_edges.go |

## Mitigation Summary

- **T-28-01**: ComputeEdges is pure; tested for determinism (RED→GREEN in 28-01 T2).
- **T-28-02**: Citation co-occurrence audit query is the existing QueryAudit API; no new disclosure.
- **T-28-03**: 4 SKILLFORGE_GRAPH_* env vars + skeleton-degraded response shape with explicit reason string.
- **T-28-04**: 5s TTL cache + cross-phase invalidation hook (wiring near main.go:766).
- **T-28-05**: graph_edges.go verified to NOT call kb.Citable; verify-phase-28.sh Stage 2b enforces.
- **T-28-06**: depth cap enforced in handlers; > cap → cap.
- **T-28-SC**: Phase 28 standards (zero new deps).
- **T-28-LLM**: zero LLM calls; deterministic algorithms only.

---

_See also:_ `docs/threat-model/STRIDE-27.md` (Phase 27 trust contract), `verify-phase-28.sh` (structural gate), `28-RESEARCH.md` §Trust-contract surface.
