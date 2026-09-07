# Phase 28 LIVE Acceptance Runbook

**Phase:** 28 — 知识图谱动态化 + 关系层
**Operator checkpoint:** HUMAN VERIFY (mirrors Phase 27 27-UAT.md)

## Prerequisites

- Server running with `SKILLFORGE_ADMIN_TOKEN` set
- At least one active KB slug in the registry
- Phase 27 upload pipeline verified (operator runbook 27-UAT.md PASSED)

---

### Section A: Graph API Verification

| Step | Action | Expected Result | Validation |
|------|--------|-----------------|------------|
| 1 | `curl -s http://localhost:8888/api/knowledge/graph \| jq '.tiers\|length'` | Returns N≥1 (one tier per active KB) | tiers count matches knowledge_bases.yaml entries |
| 2 | `curl -s http://localhost:8888/api/knowledge/graph \| jq '.degraded'` | Returns false (or absent) when corpus is small | Within SKILLFORGE_GRAPH_MAX_NODES limit |
| 3 | `curl -s "http://localhost:8888/api/knowledge/graph/neighbors?chunk_id=1&depth=1"` | Returns JSON with nodes + edges arrays | edges[].edge_type in {section_adjacent, version_chain, keyword_cooccur, citation_cooccur} |
| 4 | `curl -s "http://localhost:8888/api/knowledge/graph/path?from=1&to=3&max_hops=4"` | Returns JSON with hops ≥ 0 or hops=-1 + degraded=true | BFS path found or explicit disconnected reason |
| 5 | Inspect GraphCanvas in browser | Tier nodes visible; doc nodes fan out; edges rendered | Click a doc node → center view opens |
| 6 | Click a citation in InspectorRetrieval | "常被一起引用" panel appears | Edges listed with Chinese labels |
| 7 | Upload a new doc via Phase 27 upload | Wait 6 seconds | `GET /api/knowledge/graph` shows new doc under the correct tier |

---

### Section B: Phase 27 → Phase 28 Joint Acceptance (5 steps from research §6)

| Step | Action | Expected Result | Validation |
|------|--------|-----------------|------------|
| 1 | Upload `test-compliance-policy.pdf` to authority KB via `POST /api/admin/kb/authority/upload` | 200; action_doc_uploaded audit row written | audit table has new row for action='action_doc_uploaded' |
| 2 | `GET /api/knowledge/graph` | authority tier has new DocNode | New doc filename appears under authority tier |
| 3 | Click doc node in GraphCanvas | DocViewer opens with full markdown source | Content renders |
| 4 | `GET /api/knowledge/graph/neighbors?chunk_id=<new_chunk_id>&depth=1` | Returns at least section-adjacent edges | edge_type in {section_adjacent, version_chain} |
| 5 | Trigger a chat query requiring content from uploaded doc | Verdict cites [ref-N] pointing to new chunk | Citation appears in InspectorRetrieval |

---

### Section C: Degraded Mode Verification

| Test | Action | Expected Result |
|------|--------|-----------------|
| Node limit exceeded | Add enough KBs to exceed SKILLFORGE_GRAPH_MAX_NODES=50 | graphHandler returns degraded=true + Chinese reason |
| Edge limit exceeded | Set SKILLFORGE_GRAPH_MAX_EDGES=10 and query neighbors on dense corpus | neighbors returns degraded=true |
| Depth limit exceeded | Query neighbors?depth=10 (cap is 2) | depth capped at 2, degraded=true |

---

### Section D: Cache Invalidation Verification

| Test | Action | Expected Result |
|------|--------|-----------------|
| Upload invalidation | Upload a doc; query /graph within 5s; query again after 6s | Second query (after TTL) shows updated node count |
| Manual invalidation | Call `knowledge.InvalidateCache()` (or restart server) | Cache cleared; next /graph request recomputes edges |
| Citation co-occur after verdict | After step 5 of Section B, wait 5s TTL and refetch neighbors | citation_cooccur edges present for cited chunks |

---

## Pass Criteria

All 18 verification steps (Section A: 7, Section B: 5, Section C: 3, Section D: 3) must return the expected result. Any deviation is a Phase 28 regression — record it in `.planning/STATE.md` Blockers.
