# Phase 26 UAT — Live Vector-Library Migration + Admin-Console Walkthrough (Operator)

**Phase:** 26 知识库注册表地基 (KB Registry Foundation)
**Plan:** 26-10 (checkpoint task)
**Status:** OPERATOR-PENDING — the hermetic layer is fully green; the live migration needs an
operator on the intranet stack. This document is the complete runbook.
**Authored:** 2026-09-07 (Plan 26-10)

---

## 0. Why this is a human step

Phase 26 switched the knowledge layer's tier vocabulary from CN-char values (权威/历史/个人/草稿)
to registry slugs (authority/historical/personal/draft). Existing rows in the live Milvus
collection still carry the CN-char values, so after the switch **every** tier query matches zero
rows. The system is not broken — D-09 fail-closed refusal is exactly right — but it is
functionally dead until the one-shot re-seed runs. The failure is *silent* (refusal metrics can
look healthy), which is precisely why acceptance is an explicit operator checklist and not a
hermetic gate (Pitfall 1 / T-26-04 in `docs/threat-model/STRIDE-26.md`).

## 1. Preconditions

| # | Precondition | How to check |
|---|--------------|--------------|
| 1 | Milvus reachable at `MILVUS_ADDRESS` (gRPC) | `timeout 5 bash -c 'cat < /dev/null > /dev/tcp/10.62.10.34/19530' && echo MILVUS-REACHABLE` |
| 2 | BGE-M3 reachable at `BGE_M3_URL` (HTTP) | `curl -s -o /dev/null -w '%{http_code}\n' --connect-timeout 5 -X POST "$BGE_M3_URL" -H 'Content-Type: application/json' -d '{"input":["probe"],"model":"bge-m3"}'` — any HTTP status (200/400/404) proves the endpoint is up; a connection error proves it is not |
| 3 | WSL Go toolchain (backend build/run is WSL-only on this host — memory: env-wsl-backend-runbook) | `wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && make check-cgo"` |
| 4 | Structural gate green before you start | `bash scripts/verify-phase-26.sh` → exit 0 |

Probe each endpoint **once**. Do not loop or retry-flake your way to green (memory:
env-intranet-service-flakiness).

## 2. Execution steps (one-shot re-seed)

```bash
# 1. Set the one-shot re-seed flag in .env (literal "1" only — anything else stays closed)
echo 'SKILLFORGE_MILVUS_RECREATE=1' >> .env

# 2. Start the server (WSL build chain) and WATCH THE BOOT LOG
wsl bash -lc "cd /mnt/d/code/ClaudeCode/ai-agent && go run ./cmd/server"
#    Expect, in order:
#      [WARN] ... SKILLFORGE_MILVUS_RECREATE=1 — dropping and re-creating collection
#             skillforge_knowledge (full re-embed cost: every corpus doc re-embedded via BGE-M3)
#      ... the boot-time index re-seeds the empty collection (chunk + embed + insert per doc)
#      ... registry/audit boot lines (kb.Open over sqlite; audit hook bound)

# 3. AFTER a successful boot: REMOVE the flag (one-shot semantics — leaving it set
#    re-embeds the whole corpus on every future boot) and restart clean.
#    Edit .env to delete the SKILLFORGE_MILVUS_RECREATE line, then stop + re-run step 2.
```

Notes:
- `milvus.RecreateCollection` = drop → `EnsureCollection` (schema + IVF_FLAT index rebuilt).
  There is no upsert-by-PK path, so drop+re-embed is the cheapest correct migration.
- The switch survives admin deactivations: the seed never resurrects an operator's off switch
  (T-26-03), and an admin-created base survives the re-seed too.
- Mongo backend deployments (`RETRIEVER_BACKEND=mongo`) use `mongo.Reset()` + re-seed instead of
  the Milvus door.

## 3. Acceptance criteria (all five, no partial credit)

| # | Criterion | How to check | Pass condition |
|---|-----------|--------------|----------------|
| ① | Retrieval returns non-zero hits (no more blanket refusal) | Any tier-channel query through the chat UI or a Skill run | hits > 0; refusal (if any) is query-justified, not `no_authorized_source` on every ask |
| ② | `/eval` face is not 100% refusal | Run the eval face / a frozen-style pass against the live stack | refusal rate clearly below 100%; a 100% refusal rate = the migration did not take |
| ③ | Knowledge-graph nodes have chunks | Graph view / `GET` the graph endpoint | every node's `chunkCount` > 0 |
| ④ | Admin console round-trip | `/admin` → Knowledge Bases: create a base (slug form `^[a-z][a-z0-9-]{0,19}$`), deactivate it, restart the server, re-open the view | created base visible; deactivated base still listed but inactive after restart (not resurrected by the seed); its `knowledge/<slug>/` data untouched |
| ⑤ | Structural gate still green on the live tree | `bash scripts/verify-phase-26.sh` | exit 0 |

## 4. Blocked-with-notes discipline (unreachable stack)

If step 1's probes fail: **do not retry-loop, do not re-run hoping the network healed, do not
mark any criterion pass on the hermetic evidence alone.** Record the blocker with notes — the
established 14-08 precedent:

- Add a STATE Blockers line: what was probed (endpoint + address), the probe output, and the
  timestamp.
- The plan's checkpoint stays `blocked-with-notes`; the five criteria above remain open.
- Re-run this runbook once the stack is reachable. A recovered BGE/Milvus + a later green run
  closes the checkpoint; nothing else does.

## 5. Residue carried for the phase verifier (not in this runbook's path)

- `web/src/components/layout/KnowledgeModal.tsx:40` — `TIER_ORDER: Tier[] = ['权威','历史','个人','草稿']`
  is a leftover CN display-order map flagged by 26-08 as out of its scope. Phase 26's gates scan
  Go only (`--include='*.go'`), so this FE file is outside them. It is a display-order constant,
  not a data value — listed here so the phase verifier decides its fate deliberately.
